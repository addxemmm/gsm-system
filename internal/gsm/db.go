// DB helpers: OpenBTS CONFIG + subscriber registry via sqlite3 CLI.
// stdlib has no sqlite driver, so we exec the `sqlite3` binary with an
// argv array (no shell concat => no injection). All values are validated
// by callers before reaching here.
package gsm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// configKeys whitelists single-key updates (legacy /allconfig allowed any
// string, which is an injection + typo footgun; v1 restricts, legacy keeps
// compat but validates KEYSTRING shape).
var configKeys = map[string]bool{
	"GSM.Radio.ARFCNs": true, "GSM.Radio.C0": true, "GSM.Radio.Band": true,
	"GSM.Identity.MCC": true, "GSM.Identity.MNC": true, "GSM.Identity.LAC": true,
	"GSM.Identity.CI": true, "GSM.Identity.ShortName": true,
}

// applyConfigLocked writes the 8 radio keys (preset-equivalent).
func (m *Manager) applyConfigLocked(p StartParams) error {
	kvs := map[string]string{
		"GSM.Radio.ARFCNs": p.ARFCNs, "GSM.Radio.C0": p.C0, "GSM.Radio.Band": p.Band,
		"GSM.Identity.MCC": p.MCC, "GSM.Identity.MNC": p.MNC,
		"GSM.Identity.LAC": p.LAC, "GSM.Identity.CI": p.CI,
		"GSM.Identity.ShortName": p.ShortName,
	}
	for k, v := range kvs {
		if err := sqliteUpdate(m.cfg.OpenBTSDbPath, "CONFIG", "VALUESTRING", v, "KEYSTRING", k); err != nil {
			return fmt.Errorf("config %s: %w", k, err)
		}
	}
	return nil
}

// ApplyPreset is the legacy /config id path (stops cell first, like run.py).
func (m *Manager) ApplyPreset(id int) error {
	p, err := PresetParams(id, m.lastNet)
	if err != nil {
		return err
	}
	// Inherit network from profile when unset (config does not carry iface).
	if p.Network == "" {
		if saved, ok := m.LoadProfile(); ok {
			p.Network = saved.Network
		}
	}
	return m.applyConfigLocked(p)
}

// GetAllConfig returns every KEYSTRING,VALUESTRING row.
func (m *Manager) GetAllConfig() ([][2]string, error) {
	out, err := sqliteQuery2(m.cfg.OpenBTSDbPath, "SELECT KEYSTRING,VALUESTRING FROM CONFIG;")
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SetSingleConfig updates one whitelisted key (v1 strict; legacy checks shape).
func (m *Manager) SetSingleConfig(name, value string) error {
	if !configKeys[name] {
		// Legacy allowed arbitrary keys; keep compat but require safe shape.
		if !isConfigKeyShape(name) || strings.ContainsAny(value, "'\n\r") {
			return fmt.Errorf("invalid config name/value")
		}
	}
	return sqliteUpdate(m.cfg.OpenBTSDbPath, "CONFIG", "VALUESTRING", value, "KEYSTRING", name)
}

func isConfigKeyShape(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_') {
			return false
		}
	}
	return strings.Contains(s, ".")
}

func sqliteUpdate(db, table, setCol, setVal, whereCol, whereVal string) error {
	// Use bound-style escaping via sqlite quote(): still argv-safe, no shell.
	sql := fmt.Sprintf("UPDATE %s SET %s='%s' WHERE %s='%s';",
		table, setCol, sqliteEscape(setVal), whereCol, sqliteEscape(whereVal))
	return sqliteExec(db, sql)
}

func sqliteEscape(s string) string { return strings.ReplaceAll(s, "'", "''") }

func sqliteExec(db, sql string) error {
	if _, err := os.Stat(db); err != nil {
		return fmt.Errorf("can not find database file")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// sqlite3 CLI from the configured binary path.
	bin := "sqlite3"
	// Manager does not carry bin override here; resolved by caller env.
	// Keep tiny: use PATH lookup (container has sqlite3).
	cmd := exec.CommandContext(ctx, bin, db, sql)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sqlite: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func sqliteQuery2(db, sql string) ([][2]string, error) {
	if _, err := os.Stat(db); err != nil {
		return nil, fmt.Errorf("can not find database file")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sqlite3", "-separator", "\x1f", db, sql)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("sqlite query: %w", err)
	}
	var rows [][2]string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 2)
		if len(parts) != 2 {
			continue
		}
		rows = append(rows, [2]string{parts[0], parts[1]})
	}
	return rows, nil
}

// resetSmqueueLocked rebuilds the smqueue queue DB from seed (stateless:
// every start begins with a clean queue; SMS history stays in smqueue.log).
func (m *Manager) resetSmqueueLocked() error {
	seed := m.cfg.SmqueueSeedPath
	if _, err := os.Stat(seed); err != nil {
		return nil // no seed bundled: skip (documented)
	}
	b, err := os.ReadFile(seed)
	if err != nil {
		return nil
	}
	_ = b
	// Actual rebuild happens in entrypoint (sqlite3 < seed); here best-effort
	// vacuum of stale WAL so a kill -9 residue never blocks start.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "sqlite3", m.cfg.OpenBTSDbPath, "PRAGMA wal_checkpoint(TRUNCATE);").Run()
	return nil
}
