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
	"sort"
	"strings"
	"time"
)

// configKeys is the public 2.1 radio configuration allowlist.
var configKeys = map[string]bool{
	"GSM.Radio.ARFCNs": true, "GSM.Radio.C0": true, "GSM.Radio.Band": true,
	"GSM.Identity.MCC": true, "GSM.Identity.MNC": true, "GSM.Identity.LAC": true,
	"GSM.Identity.CI": true, "GSM.Identity.ShortName": true,
}

var radioConfigKeys = []string{
	"GSM.Radio.ARFCNs",
	"GSM.Radio.C0",
	"GSM.Radio.Band",
	"GSM.Identity.MCC",
	"GSM.Identity.MNC",
	"GSM.Identity.LAC",
	"GSM.Identity.CI",
	"GSM.Identity.ShortName",
}

// applyConfigLocked writes all eight validated radio configuration keys.
func (m *Manager) applyConfigLocked(p StartParams) error {
	kvs := map[string]string{
		"GSM.Radio.ARFCNs": p.ARFCNs, "GSM.Radio.C0": p.C0, "GSM.Radio.Band": p.Band,
		"GSM.Identity.MCC": p.MCC, "GSM.Identity.MNC": p.MNC,
		"GSM.Identity.LAC": p.LAC, "GSM.Identity.CI": p.CI,
		"GSM.Identity.ShortName": p.ShortName,
	}
	updates := make([][2]string, 0, len(radioConfigKeys))
	for _, key := range radioConfigKeys {
		updates = append(updates, [2]string{key, kvs[key]})
	}
	if err := m.updateConfigTransaction(updates); err != nil {
		return fmt.Errorf("apply radio config: %w", err)
	}
	return nil
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

// SetSingleConfig delegates to the same validated atomic batch path.
func (m *Manager) SetSingleConfig(name, value string) error {
	return m.UpdateConfig(map[string]string{name: value})
}

// UpdateConfig validates the resulting full radio configuration, then commits
// all requested values together. Band and C0 can change in a single request.
// 校验合并后的完整配置，频段与信道可原子更新；运行中禁止配置修改。
func (m *Manager) UpdateConfig(values map[string]string) error {
	if len(values) == 0 {
		return fmt.Errorf("%w: values must not be empty", ErrInvalidConfig)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if !configKeys[key] {
			return fmt.Errorf("%w: unsupported key %s", ErrInvalidConfig, key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cellRunningLocked() {
		return ErrCellRunning
	}
	rows, err := m.sqliteQuery2("SELECT KEYSTRING,VALUESTRING FROM CONFIG;")
	if err != nil {
		return err
	}
	merged := make(map[string]string)
	for _, row := range rows {
		merged[row[0]] = row[1]
	}
	for k, v := range values {
		merged[k] = v
	}
	p := StartParams{ARFCNs: merged["GSM.Radio.ARFCNs"], C0: merged["GSM.Radio.C0"], Band: merged["GSM.Radio.Band"],
		MCC: merged["GSM.Identity.MCC"], MNC: merged["GSM.Identity.MNC"], LAC: merged["GSM.Identity.LAC"],
		CI: merged["GSM.Identity.CI"], ShortName: merged["GSM.Identity.ShortName"], Network: "lo"}
	if err := p.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	updates := make([][2]string, 0, len(keys))
	for _, k := range keys {
		updates = append(updates, [2]string{k, values[k]})
	}
	return m.updateConfigTransaction(updates)
}
func sqliteEscape(s string) string { return strings.ReplaceAll(s, "'", "''") }

// updateConfigTransaction applies every update in one sqlite process and one
// BEGIN IMMEDIATE transaction. The guard CHECK turns a missing/duplicate key
// into a sqlite error; -bail then exits and the uncommitted transaction rolls
// back instead of silently applying a partial configuration.
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
		line = strings.TrimSuffix(line, "\r")
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
