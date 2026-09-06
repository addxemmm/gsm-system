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

var presetConfigKeys = []string{
	"GSM.Radio.ARFCNs",
	"GSM.Radio.C0",
	"GSM.Radio.Band",
	"GSM.Identity.MCC",
	"GSM.Identity.MNC",
	"GSM.Identity.LAC",
	"GSM.Identity.CI",
	"GSM.Identity.ShortName",
}

// applyConfigLocked writes the 8 radio keys (preset-equivalent).
func (m *Manager) applyConfigLocked(p StartParams) error {
	kvs := map[string]string{
		"GSM.Radio.ARFCNs": p.ARFCNs, "GSM.Radio.C0": p.C0, "GSM.Radio.Band": p.Band,
		"GSM.Identity.MCC": p.MCC, "GSM.Identity.MNC": p.MNC,
		"GSM.Identity.LAC": p.LAC, "GSM.Identity.CI": p.CI,
		"GSM.Identity.ShortName": p.ShortName,
	}
	updates := make([][2]string, 0, len(presetConfigKeys))
	for _, key := range presetConfigKeys {
		updates = append(updates, [2]string{key, kvs[key]})
	}
	if err := m.updateConfigTransaction(updates); err != nil {
		return fmt.Errorf("apply radio config: %w", err)
	}
	return nil
}

// ApplyPreset is the legacy /config id path (stops cell first, like run.py).
func (m *Manager) ApplyPreset(id int) error {
	p, err := PresetParams(id, "")
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, stopped := m.stopLocked()
	if !stopped {
		return ErrStopFailed
	}
	return m.applyConfigLocked(p)
}

// GetAllConfig returns every KEYSTRING,VALUESTRING row.
func (m *Manager) GetAllConfig() ([][2]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out, err := m.sqliteQuery2("SELECT KEYSTRING,VALUESTRING FROM CONFIG;")
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
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cellRunningLocked() {
		return ErrCellRunning
	}
	return m.updateConfigTransaction([][2]string{{name, value}})
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

func sqliteEscape(s string) string { return strings.ReplaceAll(s, "'", "''") }

// updateConfigTransaction applies every update in one sqlite process and one
// BEGIN IMMEDIATE transaction. The guard CHECK turns a missing/duplicate key
// into a sqlite error; -bail then exits and the uncommitted transaction rolls
// back instead of silently applying a partial preset.
func (m *Manager) updateConfigTransaction(updates [][2]string) error {
	var sql strings.Builder
	sql.WriteString("PRAGMA busy_timeout=5000;\n")
	sql.WriteString("BEGIN IMMEDIATE;\n")
	sql.WriteString("CREATE TEMP TABLE _gsm_config_guard(ok INTEGER CHECK(ok = 1));\n")
	for _, kv := range updates {
		fmt.Fprintf(&sql, "UPDATE CONFIG SET VALUESTRING='%s' WHERE KEYSTRING='%s';\n",
			sqliteEscape(kv[1]), sqliteEscape(kv[0]))
		sql.WriteString("INSERT INTO _gsm_config_guard VALUES(changes());\n")
	}
	sql.WriteString("DROP TABLE _gsm_config_guard;\nCOMMIT;\n")
	return m.sqliteExec(sql.String())
}

func (m *Manager) sqliteExec(sql string) error {
	if _, err := os.Stat(m.cfg.OpenBTSDbPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrDatabaseNotFound, m.cfg.OpenBTSDbPath)
		}
		return fmt.Errorf("stat sqlite database: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.cfg.Sqlite3Bin, "-batch", "-bail", m.cfg.OpenBTSDbPath)
	cmd.Stdin = strings.NewReader(sql)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sqlite: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (m *Manager) sqliteQuery2(sql string) ([][2]string, error) {
	if _, err := os.Stat(m.cfg.OpenBTSDbPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrDatabaseNotFound, m.cfg.OpenBTSDbPath)
		}
		return nil, fmt.Errorf("stat sqlite database: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.cfg.Sqlite3Bin, "-batch", "-bail", "-separator", "\x1f", m.cfg.OpenBTSDbPath, sql)
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
