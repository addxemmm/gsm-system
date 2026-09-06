// Helpers: subprocess + sqlite + iptables, all argv-based (no shell).
package api

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/addxemmm/gsm-system/internal/config"
)

func runCLI(bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	return string(out), err
}

func applyIptables(bin, iface string) error {
	if bin == "" {
		bin = "iptables"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Add MASQUERADE for the GSM data subnet (legacy 192.168.99.0/24).
	if out, err := exec.CommandContext(ctx, bin, "-t", "nat", "-A", "POSTROUTING",
		"-s", "192.168.99.0/24", "-o", iface, "-j", "MASQUERADE").CombinedOutput(); err != nil {
		return fmt.Errorf("iptables: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// setSubscriberNumber mirrors legacy set_phone_number: update TMSITable +
// asterisk sip_buddies/dialdata. Returns legacy message_id (1/3/4).
func setSubscriberNumber(cfg config.Config, imsi, number string) (int, string, error) {
	for name, path := range map[string]string{
		"TMSI": cfg.TMSITablePath, "Asterisk": cfg.AsteriskDbPath,
	} {
		if _, err := os.Stat(path); err != nil {
			return 0, "False", fmt.Errorf("%s database: %w", name, err)
		}
	}
	exists, err := sqliteExists(cfg.Sqlite3Bin, cfg.TMSITablePath,
		"SELECT 1 FROM tmsi_table WHERE IMSI='"+sqliteEsc(imsi)+"' LIMIT 1;")
	if err != nil {
		return 0, "False", err
	}
	if !exists {
		return 3, "This imsi is not existed.", nil
	}
	astExists, err := sqliteExists(cfg.Sqlite3Bin, cfg.AsteriskDbPath,
		"SELECT 1 FROM sip_buddies WHERE name='IMSI"+sqliteEsc(imsi)+"' LIMIT 1;")
	if err != nil {
		return 0, "False", err
	}
	if !astExists {
		err := sqliteExec(cfg.Sqlite3Bin, cfg.TMSITablePath,
			"UPDATE tmsi_table SET ASSOCIATED_URI='<tel:"+sqliteEsc(number)+">' WHERE IMSI='"+sqliteEsc(imsi)+"';")
		if err != nil {
			return 0, "False", err
		}
		// Preserve the legacy side effect: TMSITable is updated even when
		// Asterisk has no matching row, while the response remains id 4.
		return 4, "This imsi is not existed in asterisk.", nil
	}
	// One sqlite3 connection + ATTACH keeps the three related writes in one
	// transaction. The TEMP CHECK turns a missing/duplicate target row into an
	// error; -bail then prevents COMMIT and connection close rolls back writes.
	sql := fmt.Sprintf(`ATTACH DATABASE '%s' AS asterisk;
CREATE TEMP TABLE api_update_guard(affected INTEGER CHECK(affected = 1));
BEGIN IMMEDIATE;
UPDATE main.tmsi_table SET ASSOCIATED_URI='<tel:%s>' WHERE IMSI='%s';
INSERT INTO api_update_guard VALUES(changes());
DELETE FROM api_update_guard;
UPDATE asterisk.sip_buddies SET callerid='%s' WHERE name='IMSI%s';
INSERT INTO api_update_guard VALUES(changes());
DELETE FROM api_update_guard;
UPDATE asterisk.dialdata_table SET exten='%s' WHERE dial='IMSI%s';
INSERT INTO api_update_guard VALUES(changes());
COMMIT;`, sqliteEsc(cfg.AsteriskDbPath), sqliteEsc(number), sqliteEsc(imsi),
		sqliteEsc(number), sqliteEsc(imsi), sqliteEsc(number), sqliteEsc(imsi))
	if err := sqliteExec(cfg.Sqlite3Bin, cfg.TMSITablePath, sql); err != nil {
		return 0, "False", err
	}
	return 1, "Success", nil
}

func listSubscribers(cfg config.Config) ([]map[string]string, error) {
	if _, err := os.Stat(cfg.AsteriskDbPath); err != nil {
		return nil, fmt.Errorf("Asterisk database: %w", err)
	}
	rows, err := sqliteQuery(cfg.Sqlite3Bin, cfg.AsteriskDbPath,
		"SELECT name,callerid FROM sip_buddies WHERE name LIKE 'IMSI%';")
	if err != nil {
		return nil, err
	}
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		imsi := strings.TrimPrefix(r[0], "IMSI")
		out = append(out, map[string]string{"imsi": imsi, "number": r[1]})
	}
	return out, nil
}

func listSubscribersBestEffort(cfg config.Config) []map[string]string {
	items, err := listSubscribers(cfg)
	if err != nil {
		return []map[string]string{}
	}
	return items
}

func sqliteEsc(s string) string { return strings.ReplaceAll(s, "'", "''") }

func sqliteExec(bin, db, sql string) error {
	if bin == "" {
		bin = "sqlite3"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "-bail", db, sql).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func sqliteExists(bin, db, sql string) (bool, error) {
	if bin == "" {
		bin = "sqlite3"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "-bail", db, sql).Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func sqliteQuery(bin, db, sql string) ([][2]string, error) {
	if bin == "" {
		bin = "sqlite3"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "-bail", "-separator", "\x1f", db, sql).Output()
	if err != nil {
		return nil, err
	}
	var rows [][2]string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		p := strings.SplitN(line, "\x1f", 2)
		if len(p) != 2 {
			continue
		}
		rows = append(rows, [2]string{p[0], p[1]})
	}
	return rows, nil
}
