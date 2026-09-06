package api_test

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/addxemmm/gsm-system/internal/api"
	"github.com/addxemmm/gsm-system/internal/config"
	"github.com/addxemmm/gsm-system/internal/gsm"
)

func TestLegacyFrozenPaths(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.ConfDir = t.TempDir()
	cfg.LogDir = t.TempDir()
	mgr := gsm.New(cfg)
	srv := api.New(cfg, mgr)
	h := srv.Handler()

	// Non-POST to POST-only legacy route keeps frozen message_id 0 body.
	req := httptest.NewRequest(http.MethodGet, "/sendsms", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy must stay HTTP 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["message_id"] != float64(0) {
		t.Fatalf("frozen sendsms GET must be message_id 0, got %v", body)
	}
}

func TestV1NotFoundEnvelope(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	mgr := gsm.New(cfg)
	srv := api.New(cfg, mgr)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("v1 unknown must be 404, got %d", rec.Code)
	}
	if rid := rec.Header().Get("X-Request-ID"); rid == "" {
		t.Fatal("missing X-Request-ID")
	}
	var env struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != 40401 {
		t.Fatalf("want code 40401, got %d", env.Code)
	}
}

func TestV1CellValidation(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	mgr := gsm.New(cfg)
	srv := api.New(cfg, mgr)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cell",
		strings.NewReader(`{"band":"7"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 422 {
		t.Fatalf("bad GSM band must be 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestV1RejectsTrailingJSON(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	mgr := gsm.New(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sms",
		strings.NewReader(`{} {}`))
	rec := httptest.NewRecorder()
	api.New(cfg, mgr).Handler().ServeHTTP(rec, req)
	assertV1Code(t, rec, http.StatusBadRequest, api.CodeMalformed)
}

func TestV1RejectsNullBodyAsWrongShape(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	mgr := gsm.New(cfg)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sms", strings.NewReader(`null`))
	rec := httptest.NewRecorder()
	api.New(cfg, mgr).Handler().ServeHTTP(rec, req)
	assertV1Code(t, rec, http.StatusBadRequest, api.CodeMalformed)
}

func TestV1BodyLimitIncludesTrailingData(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	mgr := gsm.New(cfg)
	body := `{"iface":"eth0"}` + strings.Repeat(" ", 64*1024)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/network", strings.NewReader(body))
	rec := httptest.NewRecorder()
	api.New(cfg, mgr).Handler().ServeHTTP(rec, req)
	assertV1Code(t, rec, http.StatusRequestEntityTooLarge, api.CodeTooLarge)
}

func TestV1ConfigMissingDatabaseIsNotValidationError(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.OpenBTSDbPath = filepath.Join(cfg.DataDir, "missing-openbts.db")
	mgr := gsm.New(cfg)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/config",
		strings.NewReader(`{"name":"GSM.Identity.ShortName","value":"lab"}`))
	rec := httptest.NewRecorder()
	api.New(cfg, mgr).Handler().ServeHTTP(rec, req)
	assertV1Code(t, rec, http.StatusNotFound, api.CodeNotFound)
}

func TestV1ConfigQueryFailureIsInternal(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.OpenBTSDbPath = filepath.Join(cfg.DataDir, "openbts.db")
	if err := os.WriteFile(cfg.OpenBTSDbPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Sqlite3Bin = filepath.Join(cfg.DataDir, "missing-sqlite3")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	rec := httptest.NewRecorder()
	api.New(cfg, gsm.New(cfg)).Handler().ServeHTTP(rec, req)
	assertV1Code(t, rec, http.StatusInternalServerError, api.CodeInternal)
}

func TestV1SubscriberListMissingDatabaseIsNotEmptySuccess(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.AsteriskDbPath = filepath.Join(cfg.DataDir, "missing-asterisk.db")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscribers", nil)
	rec := httptest.NewRecorder()
	api.New(cfg, gsm.New(cfg)).Handler().ServeHTTP(rec, req)
	assertV1Code(t, rec, http.StatusNotFound, api.CodeNotFound)
}

func TestMiddlewarePanicPreservesEnvelopeAndAudit(t *testing.T) {
	t.Setenv("GSM_API_TOKEN", "")
	var logs bytes.Buffer
	oldWriter := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(oldWriter)

	cfg := config.Default()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cell", nil)
	rec := httptest.NewRecorder()
	api.New(cfg, nil).Handler().ServeHTTP(rec, req) // nil manager deliberately panics
	assertV1Code(t, rec, http.StatusInternalServerError, api.CodeInternal)

	var env struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.RequestID == "" || rec.Header().Get("X-Request-ID") != env.RequestID {
		t.Fatalf("panic response request id mismatch: header=%q body=%q",
			rec.Header().Get("X-Request-ID"), env.RequestID)
	}
	if got := logs.String(); !strings.Contains(got, "rid="+env.RequestID+" panic recovered:") ||
		!strings.Contains(got, "rid="+env.RequestID+" GET /api/v1/cell -> 500") {
		t.Fatalf("panic request missing audit evidence: %s", got)
	}
}

func TestUnauthorizedRequestIsAudited(t *testing.T) {
	t.Setenv("GSM_API_TOKEN", "secret")
	var logs bytes.Buffer
	oldWriter := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(oldWriter)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	api.New(config.Default(), nil).Handler().ServeHTTP(rec, req)
	assertV1Code(t, rec, http.StatusUnauthorized, api.CodeUnauthorized)
	rid := rec.Header().Get("X-Request-ID")
	if rid == "" || !strings.Contains(logs.String(), "rid="+rid+" GET /api/v1/health -> 401") {
		t.Fatalf("unauthorized request missing audit evidence: %s", logs.String())
	}
}

func TestSubscriberUpdateRollsBackOnSQLiteError(t *testing.T) {
	cfg := config.Default()
	if _, err := exec.LookPath(cfg.Sqlite3Bin); err != nil {
		t.Skipf("sqlite3 unavailable: %v", err)
	}
	dir := t.TempDir()
	cfg.DataDir = dir
	cfg.TMSITablePath = filepath.Join(dir, "tmsi.db")
	cfg.AsteriskDbPath = filepath.Join(dir, "asterisk.db")
	const imsi = "001010123456780"
	runSQLite(t, cfg.Sqlite3Bin, cfg.TMSITablePath,
		`CREATE TABLE tmsi_table(IMSI TEXT PRIMARY KEY, ASSOCIATED_URI TEXT);`+
			`INSERT INTO tmsi_table VALUES('`+imsi+`','<tel:old>');`)
	runSQLite(t, cfg.Sqlite3Bin, cfg.AsteriskDbPath,
		`CREATE TABLE sip_buddies(name TEXT PRIMARY KEY, callerid TEXT);`+
			`INSERT INTO sip_buddies VALUES('IMSI`+imsi+`','old');`)

	h := api.New(cfg, gsm.New(cfg)).Handler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscribers",
		strings.NewReader(`{"imsi":"`+imsi+`","number":"10001"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assertV1Code(t, rec, http.StatusInternalServerError, api.CodeInternal)
	if got := sqliteValue(t, cfg.Sqlite3Bin, cfg.TMSITablePath,
		"SELECT ASSOCIATED_URI FROM tmsi_table;"); got != "<tel:old>" {
		t.Fatalf("TMSI write was not rolled back: %q", got)
	}
	if got := sqliteValue(t, cfg.Sqlite3Bin, cfg.AsteriskDbPath,
		"SELECT callerid FROM sip_buddies;"); got != "old" {
		t.Fatalf("Asterisk write was not rolled back: %q", got)
	}

	runSQLite(t, cfg.Sqlite3Bin, cfg.AsteriskDbPath,
		`CREATE TABLE dialdata_table(dial TEXT PRIMARY KEY, exten TEXT);`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/subscribers",
		strings.NewReader(`{"imsi":"`+imsi+`","number":"10001"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assertV1Code(t, rec, http.StatusInternalServerError, api.CodeInternal)
	if got := sqliteValue(t, cfg.Sqlite3Bin, cfg.TMSITablePath,
		"SELECT ASSOCIATED_URI FROM tmsi_table;"); got != "<tel:old>" {
		t.Fatalf("missing dialdata row did not roll back TMSI: %q", got)
	}
	if got := sqliteValue(t, cfg.Sqlite3Bin, cfg.AsteriskDbPath,
		"SELECT callerid FROM sip_buddies;"); got != "old" {
		t.Fatalf("missing dialdata row did not roll back callerid: %q", got)
	}

	runSQLite(t, cfg.Sqlite3Bin, cfg.AsteriskDbPath,
		`INSERT INTO dialdata_table VALUES('IMSI`+imsi+`','old');`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/subscribers",
		strings.NewReader(`{"imsi":"`+imsi+`","number":"10001"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assertV1Code(t, rec, http.StatusOK, api.CodeOK)
	if got := sqliteValue(t, cfg.Sqlite3Bin, cfg.TMSITablePath,
		"SELECT ASSOCIATED_URI FROM tmsi_table;"); got != "<tel:10001>" {
		t.Fatalf("TMSI update = %q", got)
	}
	if got := sqliteValue(t, cfg.Sqlite3Bin, cfg.AsteriskDbPath,
		"SELECT callerid FROM sip_buddies;"); got != "10001" {
		t.Fatalf("sip callerid update = %q", got)
	}
	if got := sqliteValue(t, cfg.Sqlite3Bin, cfg.AsteriskDbPath,
		"SELECT exten FROM dialdata_table;"); got != "10001" {
		t.Fatalf("dialdata exten update = %q", got)
	}
}

func assertV1Code(t *testing.T, rec *httptest.ResponseRecorder, status, code int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status=%d, want %d: %s", rec.Code, status, rec.Body.String())
	}
	var env struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != code {
		t.Fatalf("code=%d, want %d: %s", env.Code, code, rec.Body.String())
	}
}

func runSQLite(t *testing.T, bin, db, sql string) {
	t.Helper()
	if out, err := exec.Command(bin, "-bail", db, sql).CombinedOutput(); err != nil {
		t.Fatalf("sqlite setup: %v: %s", err, out)
	}
}

func sqliteValue(t *testing.T, bin, db, sql string) string {
	t.Helper()
	out, err := exec.Command(bin, "-bail", db, sql).CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite query: %v: %s", err, out)
	}
	return strings.TrimSpace(string(out))
}
