package subscriber

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const fixtureIMSI = "001010123456780"

func TestBindCreatesMissingDialRowAndProjectsAfterCommit(t *testing.T) {
	store := newFixtureStore(t, false)
	result, err := store.Bind(context.Background(), fixtureIMSI, "10001")
	if err != nil {
		t.Fatal(err)
	}
	if result.Subscriber.Number == nil || *result.Subscriber.Number != "10001" ||
		!result.Subscriber.Consistent || result.Projection != "updated" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := sqliteValue(t, store.SQLiteBin, store.AsteriskDB,
		`SELECT callerid FROM sip_buddies WHERE name='IMSI001010123456780';`); got != "10001" {
		t.Fatalf("callerid=%q", got)
	}
	if got := sqliteValue(t, store.SQLiteBin, store.AsteriskDB,
		`SELECT exten FROM dialdata_table WHERE dial='IMSI001010123456780';`); got != "10001" {
		t.Fatalf("dial exten=%q", got)
	}
	if got := sqliteValue(t, store.SQLiteBin, store.TMSIDB,
		`SELECT ASSOCIATED_URI FROM tmsi_table WHERE IMSI='001010123456780';`); got != "<tel:10001>" {
		t.Fatalf("projection=%q", got)
	}
}

func TestBindConflictDoesNotPartiallyWrite(t *testing.T) {
	store := newFixtureStore(t, true)
	runSQLite(t, store.SQLiteBin, store.AsteriskDB,
		`INSERT INTO sip_buddies(name,callerid,ki) VALUES('IMSI001010123456781','20002','other-secret');`+
			`INSERT INTO dialdata_table(exten,dial) VALUES('20002','IMSI001010123456781');`)
	if _, err := store.Bind(context.Background(), fixtureIMSI, "20002"); !errors.Is(err, ErrConflict) {
		t.Fatalf("error=%v", err)
	}
	if got := sqliteValue(t, store.SQLiteBin, store.AsteriskDB,
		`SELECT callerid FROM sip_buddies WHERE name='IMSI001010123456780';`); got != "old" {
		t.Fatalf("conflict partially changed callerid=%q", got)
	}
	if got := sqliteValue(t, store.SQLiteBin, store.AsteriskDB,
		`SELECT exten FROM dialdata_table WHERE dial='IMSI001010123456780';`); got != "old" {
		t.Fatalf("conflict partially changed dial=%q", got)
	}
	if got := sqliteValue(t, store.SQLiteBin, store.TMSIDB,
		`SELECT ASSOCIATED_URI FROM tmsi_table WHERE IMSI='001010123456780';`); got != "<tel:old>" {
		t.Fatalf("conflict changed projection=%q", got)
	}
}

func TestBindTransactionFailureRollsBackBothAuthoritativeRows(t *testing.T) {
	store := newFixtureStore(t, true)
	runSQLite(t, store.SQLiteBin, store.AsteriskDB,
		`CREATE TRIGGER reject_dial_update BEFORE UPDATE ON dialdata_table `+
			`BEGIN SELECT RAISE(ABORT,'fixture failure'); END;`)
	if _, err := store.Bind(context.Background(), fixtureIMSI, "10001"); err == nil ||
		errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrInconsistent) {
		t.Fatalf("unexpected error classification: %v", err)
	}
	if got := sqliteValue(t, store.SQLiteBin, store.AsteriskDB,
		`SELECT callerid FROM sip_buddies WHERE name='IMSI001010123456780';`); got != "old" {
		t.Fatalf("failed transaction partially changed callerid=%q", got)
	}
	if got := sqliteValue(t, store.SQLiteBin, store.AsteriskDB,
		`SELECT exten FROM dialdata_table WHERE dial='IMSI001010123456780';`); got != "old" {
		t.Fatalf("failed transaction partially changed dial=%q", got)
	}
	if got := sqliteValue(t, store.SQLiteBin, store.TMSIDB,
		`SELECT ASSOCIATED_URI FROM tmsi_table WHERE IMSI='001010123456780';`); got != "<tel:old>" {
		t.Fatalf("failed authority transaction changed projection=%q", got)
	}
}

func TestUnbindIsIdempotentAndDoesNotDeleteSubscriber(t *testing.T) {
	store := newFixtureStore(t, true)
	for i := 0; i < 2; i++ {
		result, err := store.Unbind(context.Background(), fixtureIMSI)
		if err != nil {
			t.Fatal(err)
		}
		if result.Subscriber.Number != nil || !result.Subscriber.Consistent || result.Projection != "updated" {
			t.Fatalf("unexpected unbind: %+v", result)
		}
	}
	if got := sqliteValue(t, store.SQLiteBin, store.AsteriskDB,
		`SELECT count(*) FROM sip_buddies WHERE name='IMSI001010123456780';`); got != "1" {
		t.Fatalf("subscriber was deleted: %q", got)
	}
}

func TestDuplicateNativeRowsAreInconsistent(t *testing.T) {
	store := newFixtureStore(t, true)
	runSQLite(t, store.SQLiteBin, store.AsteriskDB,
		`INSERT INTO dialdata_table(exten,dial) VALUES('','IMSI001010123456780');`)
	if _, err := store.Get(context.Background(), fixtureIMSI); !errors.Is(err, ErrInconsistent) {
		t.Fatalf("Get error=%v", err)
	}
	if _, err := store.List(context.Background()); !errors.Is(err, ErrInconsistent) {
		t.Fatalf("List error=%v", err)
	}
	if _, err := store.Bind(context.Background(), fixtureIMSI, "10001"); !errors.Is(err, ErrInconsistent) {
		t.Fatalf("Bind error=%v", err)
	}
}

func TestProjectionIsOptionalAndNeverRollsBackAuthority(t *testing.T) {
	store := newFixtureStore(t, false)
	store.TMSIDB = filepath.Join(t.TempDir(), "missing-tmsi.db")
	result, err := store.Bind(context.Background(), fixtureIMSI, "10001")
	if err != nil {
		t.Fatal(err)
	}
	if result.Projection != "unavailable" || result.Subscriber.Number == nil || *result.Subscriber.Number != "10001" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func newFixtureStore(t *testing.T, withDial bool) Store {
	t.Helper()
	dir := t.TempDir()
	store := Store{SQLiteBin: sqliteBinary(t), AsteriskDB: filepath.Join(dir, "asterisk.db"), TMSIDB: filepath.Join(dir, "tmsi.db")}
	runSQLite(t, store.SQLiteBin, store.AsteriskDB,
		`CREATE TABLE sip_buddies(id INTEGER PRIMARY KEY,name TEXT NOT NULL,callerid TEXT,ki TEXT DEFAULT '');`+
			`CREATE TABLE dialdata_table(id INTEGER PRIMARY KEY,exten TEXT NOT NULL DEFAULT '',dial TEXT NOT NULL DEFAULT '');`+
			`INSERT INTO sip_buddies(name,callerid,ki) VALUES('IMSI001010123456780','old','never-expose');`)
	if withDial {
		runSQLite(t, store.SQLiteBin, store.AsteriskDB,
			`INSERT INTO dialdata_table(exten,dial) VALUES('old','IMSI001010123456780');`)
	}
	runSQLite(t, store.SQLiteBin, store.TMSIDB,
		`CREATE TABLE tmsi_table(IMSI TEXT,ASSOCIATED_URI TEXT);`+
			`INSERT INTO tmsi_table VALUES('001010123456780','<tel:old>');`)
	return store
}

func sqliteBinary(t *testing.T) string {
	t.Helper()
	if configured := os.Getenv("SQLITE3_BIN"); configured != "" {
		return configured
	}
	if path, err := exec.LookPath("sqlite3"); err == nil {
		return path
	}
	if runtime.GOOS == "windows" {
		const bundled = `C:\APP\platform-tools\sqlite3.exe`
		if _, err := os.Stat(bundled); err == nil {
			return bundled
		}
	}
	t.Skip("real sqlite3 executable is unavailable")
	return ""
}

func runSQLite(t *testing.T, binary, database, sql string) {
	t.Helper()
	if output, err := exec.Command(binary, "-batch", "-bail", database, sql).CombinedOutput(); err != nil {
		t.Fatalf("sqlite setup: %v: %s", err, output)
	}
}

func sqliteValue(t *testing.T, binary, database, sql string) string {
	t.Helper()
	output, err := exec.Command(binary, "-batch", "-bail", database, sql).CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite query: %v: %s", err, output)
	}
	return strings.TrimSpace(string(output))
}
