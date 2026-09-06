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
	"github.com/addxemmm/gsm-system/internal/gsm"
)

func runCLI(bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	return string(out), err
}

func applyPresetDB(dbPath string, id int) error {
	if id < 0 || id >= len(gsm.Presets) {
		return fmt.Errorf("unknown config id")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("can not find database file")
	}
	c := gsm.Presets[id]
	keys := []string{"GSM.Radio.ARFCNs", "GSM.Radio.C0", "GSM.Radio.Band",
		"GSM.Identity.MCC", "GSM.Identity.MNC", "GSM.Identity.LAC",
		"GSM.Identity.CI", "GSM.Identity.ShortName"}
	for i, k := range keys {
		sql := fmt.Sprintf("UPDATE CONFIG SET VALUESTRING='%s' WHERE KEYSTRING='%s';",
			strings.ReplaceAll(c[i], "'", "''"), k)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		out, err := exec.CommandContext(ctx, "sqlite3", dbPath, sql).CombinedOutput()
		cancel()
		if err != nil {
			return fmt.Errorf("sqlite: %v: %s", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
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
func setSubscriberNumber(cfg config.Config, imsi, number string) (int, string) {
	exists, err := sqliteExists(cfg.TMSITablePath,
		"SELECT 1 FROM tmsi_table WHERE IMSI='"+sqliteEsc(imsi)+"' LIMIT 1;")
	if err != nil || !exists {
		return 3, "This imsi is not existed."
	}
	astExists, _ := sqliteExists(cfg.AsteriskDbPath,
		"SELECT 1 FROM sip_buddies WHERE name='IMSI"+sqliteEsc(imsi)+"' LIMIT 1;")
	if !astExists {
		_ = sqliteExec(cfg.TMSITablePath,
			"UPDATE tmsi_table SET ASSOCIATED_URI='<tel:"+sqliteEsc(number)+">' WHERE IMSI='"+sqliteEsc(imsi)+"';")
		return 4, "This imsi is not existed in asterisk."
	}
	_ = sqliteExec(cfg.TMSITablePath,
		"UPDATE tmsi_table SET ASSOCIATED_URI='<tel:"+sqliteEsc(number)+">' WHERE IMSI='"+sqliteEsc(imsi)+"';")
	_ = sqliteExec(cfg.AsteriskDbPath,
		"UPDATE sip_buddies SET callerid='"+sqliteEsc(number)+"' WHERE name='IMSI"+sqliteEsc(imsi)+"';")
	_ = sqliteExec(cfg.AsteriskDbPath,
		"UPDATE dialdata_table SET exten='"+sqliteEsc(number)+"' WHERE dial='IMSI"+sqliteEsc(imsi)+"';")
	return 1, "Success"
}

func listSubscribersBestEffort(cfg config.Config) []map[string]string {
	rows, err := sqliteQuery(cfg.AsteriskDbPath,
		"SELECT name,callerid FROM sip_buddies WHERE name LIKE 'IMSI%';")
	if err != nil {
		return []map[string]string{}
	}
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		imsi := strings.TrimPrefix(r[0], "IMSI")
		out = append(out, map[string]string{"imsi": imsi, "number": r[1]})
	}
	return out
}

func sqliteEsc(s string) string { return strings.ReplaceAll(s, "'", "''") }

func sqliteExec(db, sql string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sqlite3", db, sql).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func sqliteExists(db, sql string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sqlite3", db, sql).Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func sqliteQuery(db, sql string) ([][2]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "sqlite3", "-separator", "\x1f", db, sql).Output()
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
