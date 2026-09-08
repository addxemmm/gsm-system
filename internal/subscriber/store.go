// Package subscriber owns the persistent IMSI-to-number registry projection.
// Asterisk is authoritative; OpenBTS' TMSI table is only a volatile cache.
package subscriber

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound     = errors.New("subscriber registration not found")
	ErrConflict     = errors.New("number is already bound")
	ErrInconsistent = errors.New("subscriber registry is inconsistent")
	mutationMu      sync.Mutex
)

// Subscriber deliberately contains no authentication material such as Ki.
type Subscriber struct {
	IMSI       string  `json:"imsi"`
	Number     *string `json:"number"`
	Consistent bool    `json:"binding_consistent"`
}

// Result reports the non-authoritative TMSI cache projection outcome.
type Result struct {
	Subscriber Subscriber `json:"subscriber"`
	Projection string     `json:"tmsi_projection"`
}

type Store struct {
	SQLiteBin  string
	AsteriskDB string
	TMSIDB     string
}

func (s Store) List(ctx context.Context) ([]Subscriber, error) {
	if err := requireFile(s.AsteriskDB); err != nil {
		return nil, err
	}
	// One compound SELECT observes both authoritative tables in one SQLite
	// statement snapshot; two CLI queries could otherwise straddle a native
	// registration update and manufacture an inconsistent view.
	rows, err := s.query(ctx, s.AsteriskDB, `
SELECT 'S',substr(name,5),COALESCE(callerid,'') FROM sip_buddies WHERE name LIKE 'IMSI%'
UNION ALL
SELECT 'D',substr(dial,5),exten FROM dialdata_table WHERE dial LIKE 'IMSI%'
ORDER BY 2,1;`)
	if err != nil {
		return nil, err
	}
	return subscribersFromSnapshot(rows)
}

func (s Store) Get(ctx context.Context, imsi string) (Subscriber, error) {
	if err := requireFile(s.AsteriskDB); err != nil {
		return Subscriber{}, err
	}
	identity := "IMSI" + imsi
	rows, err := s.query(ctx, s.AsteriskDB, fmt.Sprintf(`
SELECT 'S','%s',COALESCE(callerid,'') FROM sip_buddies WHERE name='%s'
UNION ALL
SELECT 'D','%s',exten FROM dialdata_table WHERE dial='%s'
ORDER BY 1;`, quote(imsi), quote(identity), quote(imsi), quote(identity)))
	if err != nil {
		return Subscriber{}, err
	}
	items, err := subscribersFromSnapshot(rows)
	if err != nil {
		return Subscriber{}, err
	}
	if len(items) == 0 {
		return Subscriber{}, ErrNotFound
	}
	if len(items) != 1 {
		return Subscriber{}, ErrInconsistent
	}
	return items[0], nil
}

func subscribersFromSnapshot(rows [][]string) ([]Subscriber, error) {
	sips := make(map[string]string)
	dials := make(map[string]string)
	for _, row := range rows {
		if len(row) != 3 || row[1] == "" || (row[0] != "S" && row[0] != "D") {
			return nil, fmt.Errorf("%w: malformed registry row", ErrInconsistent)
		}
		target := sips
		label := "SIP"
		if row[0] == "D" {
			target = dials
			label = "dial"
		}
		if _, duplicate := target[row[1]]; duplicate {
			return nil, fmt.Errorf("%w: duplicate %s rows for IMSI%s", ErrInconsistent, label, row[1])
		}
		target[row[1]] = row[2]
	}
	identities := make([]string, 0, len(sips))
	for imsi := range sips {
		identities = append(identities, imsi)
	}
	sort.Strings(identities)
	out := make([]Subscriber, 0, len(identities))
	for _, imsi := range identities {
		dial, hasDial := dials[imsi]
		out = append(out, subscriberFromParts(imsi, sips[imsi], dial, hasDial))
		delete(dials, imsi)
	}
	if len(dials) != 0 {
		return nil, fmt.Errorf("%w: orphan dial rows", ErrInconsistent)
	}
	return out, nil
}

func subscriberFromParts(imsi, callerID, dial string, hasDial bool) Subscriber {
	callerID = strings.TrimSpace(callerID)
	dial = strings.TrimSpace(dial)
	var number *string
	if hasDial && dial != "" {
		copy := dial
		number = &copy
	}
	consistent := callerID == dial
	if !hasDial {
		consistent = callerID == ""
	}
	return Subscriber{IMSI: imsi, Number: number, Consistent: consistent}
}

func (s Store) Bind(ctx context.Context, imsi, number string) (Result, error) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	state, err := s.bindingState(ctx, imsi, number)
	if err != nil {
		return Result{}, err
	}
	if state.sipCount == 0 {
		return Result{}, ErrNotFound
	}
	if state.sipCount != 1 || state.dialCount > 1 {
		return Result{}, ErrInconsistent
	}
	if state.conflicts != 0 {
		return Result{}, ErrConflict
	}

	identity := "IMSI" + imsi
	sql := fmt.Sprintf(`PRAGMA busy_timeout=5000;
CREATE TEMP TABLE api_guard(v INTEGER CHECK(v=1));
BEGIN IMMEDIATE;
INSERT INTO api_guard SELECT CASE WHEN count(*)=1 THEN 1 ELSE 0 END FROM sip_buddies WHERE name='%s'; DELETE FROM api_guard;
INSERT INTO api_guard SELECT CASE WHEN count(*)<=1 THEN 1 ELSE 0 END FROM dialdata_table WHERE dial='%s'; DELETE FROM api_guard;
INSERT INTO api_guard SELECT CASE WHEN NOT EXISTS(
 SELECT 1 FROM sip_buddies WHERE callerid='%s' AND name<>'%s'
 UNION ALL SELECT 1 FROM dialdata_table WHERE exten='%s' AND dial<>'%s'
) THEN 1 ELSE 0 END; DELETE FROM api_guard;
UPDATE sip_buddies SET callerid='%s' WHERE name='%s';
INSERT INTO api_guard VALUES(changes()); DELETE FROM api_guard;
INSERT INTO dialdata_table(exten,dial) SELECT '%s','%s' WHERE NOT EXISTS(SELECT 1 FROM dialdata_table WHERE dial='%s');
UPDATE dialdata_table SET exten='%s' WHERE dial='%s';
INSERT INTO api_guard VALUES(changes()); DELETE FROM api_guard;
COMMIT;`, quote(identity), quote(identity), quote(number), quote(identity),
		quote(number), quote(identity), quote(number), quote(identity), quote(number), quote(identity),
		quote(identity), quote(number), quote(identity))
	if err := s.exec(ctx, s.AsteriskDB, sql); err != nil {
		if latest, queryErr := s.bindingState(ctx, imsi, number); queryErr == nil {
			switch {
			case latest.sipCount == 0:
				return Result{}, ErrNotFound
			case latest.sipCount != 1 || latest.dialCount > 1:
				return Result{}, ErrInconsistent
			case latest.conflicts != 0:
				return Result{}, ErrConflict
			}
		}
		return Result{}, err
	}
	current, err := s.Get(ctx, imsi)
	if err != nil || current.Number == nil || *current.Number != number || !current.Consistent {
		if err == nil {
			err = ErrInconsistent
		}
		return Result{}, err
	}
	return Result{Subscriber: current, Projection: s.projectTMSI(ctx, imsi, "<tel:"+number+">")}, nil
}

func (s Store) Unbind(ctx context.Context, imsi string) (Result, error) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	current, err := s.Get(ctx, imsi)
	if err != nil {
		return Result{}, err
	}
	identity := "IMSI" + imsi
	sql := fmt.Sprintf(`PRAGMA busy_timeout=5000;
CREATE TEMP TABLE api_guard(v INTEGER CHECK(v=1));
BEGIN IMMEDIATE;
INSERT INTO api_guard SELECT CASE WHEN count(*)=1 THEN 1 ELSE 0 END FROM sip_buddies WHERE name='%s'; DELETE FROM api_guard;
INSERT INTO api_guard SELECT CASE WHEN count(*)<=1 THEN 1 ELSE 0 END FROM dialdata_table WHERE dial='%s'; DELETE FROM api_guard;
UPDATE sip_buddies SET callerid='' WHERE name='%s';
INSERT INTO api_guard VALUES(changes()); DELETE FROM api_guard;
UPDATE dialdata_table SET exten='' WHERE dial='%s';
COMMIT;`, quote(identity), quote(identity), quote(identity), quote(identity))
	if err := s.exec(ctx, s.AsteriskDB, sql); err != nil {
		return Result{}, err
	}
	current, err = s.Get(ctx, imsi)
	if err != nil || current.Number != nil || !current.Consistent {
		if err == nil {
			err = ErrInconsistent
		}
		return Result{}, err
	}
	return Result{Subscriber: current, Projection: s.projectTMSI(ctx, imsi, "")}, nil
}

type bindingState struct {
	sipCount, dialCount, conflicts int
}

func (s Store) bindingState(ctx context.Context, imsi, number string) (bindingState, error) {
	if err := requireFile(s.AsteriskDB); err != nil {
		return bindingState{}, err
	}
	identity := "IMSI" + imsi
	query := fmt.Sprintf(`SELECT
(SELECT count(*) FROM sip_buddies WHERE name='%s'),
(SELECT count(*) FROM dialdata_table WHERE dial='%s'),
(SELECT count(*) FROM (
 SELECT name FROM sip_buddies WHERE callerid='%s' AND name<>'%s'
 UNION ALL SELECT dial FROM dialdata_table WHERE exten='%s' AND dial<>'%s'
));`, quote(identity), quote(identity), quote(number), quote(identity), quote(number), quote(identity))
	rows, err := s.query(ctx, s.AsteriskDB, query)
	if err != nil {
		return bindingState{}, err
	}
	if len(rows) != 1 || len(rows[0]) != 3 {
		return bindingState{}, fmt.Errorf("unexpected binding query result")
	}
	counts := [3]int{}
	for i := range counts {
		counts[i], err = strconv.Atoi(rows[0][i])
		if err != nil {
			return bindingState{}, err
		}
	}
	return bindingState{sipCount: counts[0], dialCount: counts[1], conflicts: counts[2]}, nil
}

func (s Store) projectTMSI(ctx context.Context, imsi, uri string) string {
	if s.TMSIDB == "" {
		return "unavailable"
	}
	if err := requireFile(s.TMSIDB); err != nil {
		return "unavailable"
	}
	rows, err := s.query(ctx, s.TMSIDB,
		"SELECT count(*) FROM tmsi_table WHERE IMSI='"+quote(imsi)+"';")
	if err != nil || len(rows) != 1 || len(rows[0]) != 1 {
		return "failed"
	}
	if rows[0][0] == "0" {
		return "not_present"
	}
	if rows[0][0] != "1" {
		return "failed"
	}
	if err := s.exec(ctx, s.TMSIDB,
		"PRAGMA busy_timeout=5000; UPDATE tmsi_table SET ASSOCIATED_URI='"+quote(uri)+"' WHERE IMSI='"+quote(imsi)+"';"); err != nil {
		return "failed"
	}
	return "updated"
}

func requireFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return nil
}

func quote(value string) string { return strings.ReplaceAll(value, "'", "''") }

func (s Store) exec(ctx context.Context, db, sql string) error {
	ctx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	bin := s.SQLiteBin
	if bin == "" {
		bin = "sqlite3"
	}
	// Feed multi-statement transactions on stdin. Some sqlite3 CLI versions do
	// not honor -bail consistently for a command supplied as the final argv;
	// continuing after a statement-level ABORT could otherwise COMMIT earlier
	// writes. Explicit `.bail on` plus connection close guarantees rollback.
	cmd := exec.CommandContext(ctx, bin, "-batch", "-bail", db)
	cmd.Stdin = strings.NewReader(".bail on\n" + sql + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sqlite transaction: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s Store) query(ctx context.Context, db, sql string) ([][]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	bin := s.SQLiteBin
	if bin == "" {
		bin = "sqlite3"
	}
	out, err := exec.CommandContext(ctx, bin, "-batch", "-bail", "-separator", "\x1f", db, sql).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("sqlite query: %w: %s", err, strings.TrimSpace(string(out)))
	}
	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	if text == "" {
		return [][]string{}, nil
	}
	// sqlite terminates every result row with one newline. Remove exactly that
	// terminator; trimming all newlines would erase trailing rows whose only
	// selected value is an empty string.
	text = strings.TrimSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	rows := make([][]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, strings.Split(line, "\x1f"))
	}
	return rows, nil
}
