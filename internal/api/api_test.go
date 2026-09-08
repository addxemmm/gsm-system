package api

import (
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/addxemmm/gsm-system/internal/config"
	"github.com/addxemmm/gsm-system/internal/gsm"
)

func testServer(t *testing.T) (config.Config, *Server) {
	t.Helper()
	t.Setenv("GSM_API_TOKEN", "")
	dir := t.TempDir()
	cfg := config.Default()
	cfg.DataDir = dir
	cfg.ConfDir = filepath.Join(dir, "conf")
	cfg.LogDir = filepath.Join(dir, "log")
	cfg.OpenBTSDbPath = filepath.Join(dir, "openbts.db")
	cfg.AsteriskDbPath = filepath.Join(dir, "asterisk.db")
	cfg.TMSITablePath = filepath.Join(dir, "tmsi.db")
	cfg.AsteriskCDRPath = filepath.Join(dir, "Master.csv")
	if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return cfg, New(cfg, gsm.New(cfg))
}

func serve(t *testing.T, server *Server, method, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	return recorder
}

func TestOnlyV1RoutesAndUnknownPathsUseEnvelope(t *testing.T) {
	_, server := testServer(t)
	paths := []string{
		"/start", "/stop", "/ueinfo", "/smsinfo", "/sendsms", "/setphonenumber",
		"/config", "/getconfig", "/allconfig", "/iptables", "/healthz", "/status",
		"/profile", "/api/v1/nope", "/api/v1/cell/", "/api/v1/subscribers/",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			recorder := serve(t, server, http.MethodGet, path, "", "")
			assertCode(t, recorder, http.StatusNotFound, CodeNotFound)
			assertEnvelope(t, recorder)
		})
	}
}

func TestMethodAllowContract(t *testing.T) {
	_, server := testServer(t)
	tests := []struct {
		method, path, allow string
	}{
		{http.MethodPost, "/api/v1/subscribers", "GET"},
		{http.MethodPost, "/api/v1/network", "GET, PUT"},
		{http.MethodGet, "/api/v1/subscribers/001010123456780/number", "PUT, DELETE"},
		{http.MethodPost, "/api/v1/health", "GET"},
		{http.MethodPatch, "/api/v1/cell", "GET, POST, DELETE"},
	}
	for _, test := range tests {
		t.Run(test.method+test.path, func(t *testing.T) {
			recorder := serve(t, server, test.method, test.path, "application/json", `{}`)
			assertCode(t, recorder, http.StatusMethodNotAllowed, CodeMethod)
			if got := recorder.Header().Get("Allow"); got != test.allow {
				t.Fatalf("Allow=%q, want %q", got, test.allow)
			}
		})
	}
}

func TestStrictJSONMediaTypeShapeNamesAndTail(t *testing.T) {
	_, server := testServer(t)
	tests := []struct {
		name, contentType, body string
		status, code            int
	}{
		{"missing media type", "", `{}`, http.StatusUnsupportedMediaType, CodeMediaType},
		{"wrong media type", "text/plain", `{}`, http.StatusUnsupportedMediaType, CodeMediaType},
		{"media type parameter", "application/json; charset=utf-8", `{"iface":"eth0","extra":1}`, http.StatusBadRequest, CodeMalformed},
		{"null", "application/json", `null`, http.StatusBadRequest, CodeMalformed},
		{"array", "application/json", `[]`, http.StatusBadRequest, CodeMalformed},
		{"trailing JSON", "application/json", `{} {}`, http.StatusBadRequest, CodeMalformed},
		{"duplicate field", "application/json", `{"iface":"eth0","iface":"eth1"}`, http.StatusBadRequest, CodeMalformed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := serve(t, server, http.MethodPut, "/api/v1/network", test.contentType, test.body)
			assertCode(t, recorder, test.status, test.code)
		})
	}

	large := `{"iface":"eth0"}` + strings.Repeat(" ", 64<<10)
	recorder := serve(t, server, http.MethodPut, "/api/v1/network", "application/json", large)
	assertCode(t, recorder, http.StatusRequestEntityTooLarge, CodeTooLarge)
}

func TestDuplicateNestedConfigKeyIsMalformed(t *testing.T) {
	_, server := testServer(t)
	recorder := serve(t, server, http.MethodPatch, "/api/v1/config", "application/json",
		`{"values":{"GSM.Radio.C0":"55","GSM.Radio.C0":"56"}}`)
	assertCode(t, recorder, http.StatusBadRequest, CodeMalformed)
}

func TestQueryValidationAndPaginationBounds(t *testing.T) {
	_, server := testServer(t)
	for _, path := range []string{
		"/api/v1/sms?limit=1&limit=2", "/api/v1/subscribers?unknown=1",
		"/api/v1/connections?limit=0", "/api/v1/calls?offset=-1",
		"/api/v1/calls/history?limit=501", "/api/v1/health?verbose=1",
		"/api/v1/network?iface=eth0&iface=eth1",
	} {
		recorder := serve(t, server, http.MethodGet, path, "", "")
		assertCode(t, recorder, http.StatusUnprocessableEntity, CodeInvalid)
	}
}

func TestHealthUsesBuildIdentityWithoutSDRProbe(t *testing.T) {
	cfg, _ := testServer(t)
	cfg.Version = "2.1.test"
	cfg.Revision = "revision-test"
	cfg.UHDFindBin = filepath.Join(t.TempDir(), "must-not-run")
	server := New(cfg, gsm.New(cfg))
	recorder := serve(t, server, http.MethodGet, "/api/v1/health", "", "")
	assertCode(t, recorder, http.StatusOK, CodeOK)
	var envelope struct {
		Data struct {
			Version  string `json:"version"`
			Revision string `json:"revision"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Version != cfg.Version || envelope.Data.Revision != cfg.Revision {
		t.Fatalf("unexpected identity: %+v", envelope.Data)
	}
}

func TestConfigBatchWithRealSQLiteAndErrorClasses(t *testing.T) {
	sqlite := sqliteBinary(t)
	cfg, _ := testServer(t)
	cfg.Sqlite3Bin = sqlite
	createOpenBTSFixture(t, sqlite, cfg.OpenBTSDbPath)
	server := New(cfg, gsm.New(cfg))

	recorder := serve(t, server, http.MethodPatch, "/api/v1/config", "application/json",
		`{"values":{"GSM.Radio.Band":"900","GSM.Radio.C0":"55"}}`)
	assertCode(t, recorder, http.StatusOK, CodeOK)
	if got := sqliteValue(t, sqlite, cfg.OpenBTSDbPath,
		`SELECT VALUESTRING FROM CONFIG WHERE KEYSTRING='GSM.Radio.Band';`); got != "900" {
		t.Fatalf("band=%q", got)
	}

	recorder = serve(t, server, http.MethodPatch, "/api/v1/config", "application/json",
		`{"values":{"GSM.Radio.C0":"540"}}`)
	assertCode(t, recorder, http.StatusUnprocessableEntity, CodeInvalid)
	if got := sqliteValue(t, sqlite, cfg.OpenBTSDbPath,
		`SELECT VALUESTRING FROM CONFIG WHERE KEYSTRING='GSM.Radio.C0';`); got != "55" {
		t.Fatalf("invalid update partially committed: %q", got)
	}

	missing := cfg
	missing.OpenBTSDbPath = filepath.Join(t.TempDir(), "missing.db")
	recorder = serve(t, New(missing, gsm.New(missing)), http.MethodPatch, "/api/v1/config", "application/json",
		`{"values":{"GSM.Radio.C0":"55"}}`)
	assertCode(t, recorder, http.StatusNotFound, CodeNotFound)

	broken := cfg
	broken.Sqlite3Bin = filepath.Join(t.TempDir(), "missing-sqlite")
	recorder = serve(t, New(broken, gsm.New(broken)), http.MethodGet, "/api/v1/config", "", "")
	assertCode(t, recorder, http.StatusInternalServerError, CodeInternal)
}

func TestSubscriberPaginationAndNoKiExposure(t *testing.T) {
	sqlite := sqliteBinary(t)
	cfg, _ := testServer(t)
	cfg.Sqlite3Bin = sqlite
	createSubscriberFixture(t, sqlite, cfg.AsteriskDbPath, cfg.TMSITablePath)
	server := New(cfg, gsm.New(cfg))
	recorder := serve(t, server, http.MethodGet, "/api/v1/subscribers?limit=1&offset=1", "", "")
	assertCode(t, recorder, http.StatusOK, CodeOK)
	if strings.Contains(strings.ToLower(recorder.Body.String()), "secret-ki") || strings.Contains(strings.ToLower(recorder.Body.String()), `"ki"`) {
		t.Fatalf("authentication material exposed: %s", recorder.Body.String())
	}
	var envelope struct {
		Data struct {
			Count int `json:"count"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Count != 1 || envelope.Data.Total != 2 {
		t.Fatalf("pagination=%+v", envelope.Data)
	}
}

func TestSubscriberConflictAndIntegrityClassification(t *testing.T) {
	sqlite := sqliteBinary(t)
	cfg, _ := testServer(t)
	cfg.Sqlite3Bin = sqlite
	createSubscriberFixture(t, sqlite, cfg.AsteriskDbPath, cfg.TMSITablePath)
	server := New(cfg, gsm.New(cfg))
	const firstIMSI = "001010123456780"

	recorder := serve(t, server, http.MethodPut, "/api/v1/subscribers/"+firstIMSI+"/number",
		"application/json", `{"number":"10002"}`)
	assertCode(t, recorder, http.StatusConflict, CodeConflict)
	if got := sqliteValue(t, sqlite, cfg.AsteriskDbPath,
		`SELECT callerid FROM sip_buddies WHERE name='IMSI001010123456780';`); got != "10001" {
		t.Fatalf("conflict partially wrote callerid=%q", got)
	}

	runSQLite(t, sqlite, cfg.AsteriskDbPath,
		`INSERT INTO dialdata_table(exten,dial) VALUES('','IMSI001010123456780');`)
	recorder = serve(t, server, http.MethodGet, "/api/v1/subscribers/"+firstIMSI, "", "")
	assertCode(t, recorder, http.StatusConflict, CodeConflict)
}

func TestSubscriberBindingRejectsReservedServiceNumbers(t *testing.T) {
	_, server := testServer(t)
	for _, number := range []string{"111", "112", "911"} {
		recorder := serve(t, server, http.MethodPut,
			"/api/v1/subscribers/001010123456780/number", "application/json",
			`{"number":"`+number+`"}`)
		assertCode(t, recorder, http.StatusUnprocessableEntity, CodeInvalid)
	}
}

func TestSMSHistoryPaginationAndTailMetadata(t *testing.T) {
	cfg, _ := testServer(t)
	logText := smsEvent("10--tag-a", "001010123456780", "same text") +
		smsEvent("10--tag-a", "001010123456780", "same text") +
		smsEvent("11--tag-b", "001010123456781", "same text")
	if err := os.WriteFile(cfg.LogPath(cfg.SmqueueLogName), []byte(logText), 0o600); err != nil {
		t.Fatal(err)
	}
	recorder := serve(t, New(cfg, gsm.New(cfg)), http.MethodGet, "/api/v1/sms?limit=1&offset=1", "", "")
	assertCode(t, recorder, http.StatusOK, CodeOK)
	var envelope struct {
		Data struct {
			SMS []struct {
				Text               string            `json:"text"`
				ReceiverIMSI       string            `json:"receiver_imsi"`
				IdentityResolution map[string]string `json:"identity_resolution"`
			} `json:"sms"`
			Count     int        `json:"count"`
			Total     int        `json:"total"`
			Source    string     `json:"source"`
			Window    tailWindow `json:"window"`
			Truncated bool       `json:"truncated"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Count != 1 || envelope.Data.Total != 2 || len(envelope.Data.SMS) != 1 ||
		envelope.Data.SMS[0].Text != "same text" || envelope.Data.SMS[0].ReceiverIMSI != "001010123456781" ||
		envelope.Data.SMS[0].IdentityResolution["receiver_imsi"] != identityLogObservation ||
		envelope.Data.SMS[0].IdentityResolution["receiver_number"] != identityLogObservation ||
		envelope.Data.Source != cfg.SmqueueLogName || envelope.Data.Truncated {
		t.Fatalf("unexpected SMS page: %+v", envelope.Data)
	}
}

func TestSMSHistoryCompletesUniqueCurrentSubscriberBindings(t *testing.T) {
	cfg, _ := testServer(t)
	sqlite := sqliteBinary(t)
	createSubscriberFixture(t, sqlite, cfg.AsteriskDbPath, cfg.TMSITablePath)
	hexValue := func(value string) string { return hex.EncodeToString([]byte(value)) }
	logText := "NOTICE 10:12 2026-09-08T01:02:03.4 smsc.cpp:320:submitSMS: GSM_SMS_V1 qtag_hex=" +
		hexValue("fixture-tag") + " from_hex=" + hexValue("IMSI001010123456780") +
		" to_hex=" + hexValue("10002") + " text_hex=" + hexValue("fixture text") + "\n"
	if err := os.WriteFile(cfg.LogPath(cfg.SmqueueLogName), []byte(logText), 0o600); err != nil {
		t.Fatal(err)
	}

	recorder := serve(t, New(cfg, gsm.New(cfg)), http.MethodGet, "/api/v1/sms", "", "")
	assertCode(t, recorder, http.StatusOK, CodeOK)
	var envelope struct {
		Data struct {
			SMS []struct {
				SenderNumber       string            `json:"sender_number"`
				ReceiverIMSI       string            `json:"receiver_imsi"`
				IdentityResolution map[string]string `json:"identity_resolution"`
			} `json:"sms"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.SMS) != 1 || envelope.Data.SMS[0].SenderNumber != "10001" ||
		envelope.Data.SMS[0].ReceiverIMSI != "001010123456781" ||
		envelope.Data.SMS[0].IdentityResolution["sender_number"] != identityCurrentBinding ||
		envelope.Data.SMS[0].IdentityResolution["receiver_imsi"] != identityCurrentBinding {
		t.Fatalf("unexpected current-binding completion: %+v", envelope.Data.SMS)
	}
}

func TestCallHistoryPaginationUsesReal18ColumnCSV(t *testing.T) {
	cfg, _ := testServer(t)
	file, err := os.Create(cfg.AsteriskCDRPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := csv.NewWriter(file)
	for _, uniqueID := range []string{"first", "second"} {
		if err := writer.Write([]string{
			"", "10001", "10002", "phones", "caller", "SIP/a", "SIP/b", "Dial", "SIP/b",
			"2026-09-08 01:02:03", "", "2026-09-08 01:02:15", "12", "0",
			"NO ANSWER", "DOCUMENTATION", uniqueID, "",
		}); err != nil {
			t.Fatal(err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	recorder := serve(t, New(cfg, gsm.New(cfg)), http.MethodGet,
		"/api/v1/calls/history?limit=1&offset=1", "", "")
	assertCode(t, recorder, http.StatusOK, CodeOK)
	var envelope struct {
		Data struct {
			Calls []struct {
				StartedAt  string  `json:"started_at"`
				AnsweredAt *string `json:"answered_at"`
				UniqueID   *string `json:"unique_id"`
			} `json:"calls"`
			Count int `json:"count"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Count != 1 || envelope.Data.Total != 2 || len(envelope.Data.Calls) != 1 ||
		envelope.Data.Calls[0].StartedAt != "2026-09-08T01:02:03Z" ||
		envelope.Data.Calls[0].AnsweredAt != nil || envelope.Data.Calls[0].UniqueID == nil ||
		*envelope.Data.Calls[0].UniqueID != "second" {
		t.Fatalf("unexpected CDR page: %+v", envelope.Data)
	}
}

func TestStoppedLiveCollectionsAndMissingHistoryErrorClasses(t *testing.T) {
	cfg, server := testServer(t)
	for _, path := range []string{"/api/v1/connections", "/api/v1/calls"} {
		recorder := serve(t, server, http.MethodGet, path, "", "")
		assertCode(t, recorder, http.StatusPreconditionFailed, CodePrecondition)
	}
	for _, path := range []string{"/api/v1/sms", "/api/v1/calls/history"} {
		recorder := serve(t, New(cfg, gsm.New(cfg)), http.MethodGet, path, "", "")
		assertCode(t, recorder, http.StatusNotFound, CodeNotFound)
	}
}

func TestPageSliceReturnsNonNullEmptyPage(t *testing.T) {
	page := pageSlice([]int{1, 2}, pageRequest{Limit: 1, Offset: 2})
	if page == nil || len(page) != 0 {
		t.Fatalf("page=%v", page)
	}
}

func TestSMSTextSafeIntersectionAndByteLimit(t *testing.T) {
	if reason := validateSMSText(strings.Repeat("A", 159)); reason != "" {
		t.Fatal(reason)
	}
	for _, text := range []string{"", strings.Repeat("A", 160), "quote'", `quote"`, "tick`", "line\n", "€", "中文", "[extension]", "é"} {
		if reason := validateSMSText(text); reason == "" {
			t.Fatalf("accepted unsafe SMS %q", text)
		}
	}
}

func TestSavedProfileSuppliesMissingNetworkIface(t *testing.T) {
	cfg, server := testServer(t)
	params := gsm.StartParams{ARFCNs: "1", C0: "540", Band: "1800", MCC: "001", MNC: "01",
		LAC: "1", CI: "0", ShortName: "test", Network: "eth0"}
	if err := server.mgr.SaveProfile(params); err != nil {
		t.Fatal(err)
	}
	if got, err := server.savedOrExplicitIface(""); err != nil || got != "eth0" {
		t.Fatalf("saved iface=%q err=%v path=%s", got, err, cfg.DataDir)
	}
	if got, err := server.savedOrExplicitIface("wlan0"); err != nil || got != "wlan0" {
		t.Fatalf("explicit iface=%q err=%v", got, err)
	}
	recorder := serve(t, server, http.MethodPut, "/api/v1/network", "application/json", `{}`)
	assertCode(t, recorder, http.StatusUnprocessableEntity, CodeInvalid)
}

func TestTailReaderStartsAtCompleteLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tail.log")
	if err := os.WriteFile(path, []byte("partial-old-line\ncomplete-one\ncomplete-two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	content, window, err := readTail(path, 28)
	if err != nil {
		t.Fatal(err)
	}
	if !window.Truncated || strings.HasPrefix(string(content), "line") || !strings.Contains(string(content), "complete-two") {
		t.Fatalf("tail=%q window=%+v", content, window)
	}
}

func TestMiddlewarePanicAndUnauthorizedAreAudited(t *testing.T) {
	var logs bytes.Buffer
	oldWriter := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(oldWriter)

	t.Run("panic", func(t *testing.T) {
		t.Setenv("GSM_API_TOKEN", "")
		recorder := serve(t, New(config.Default(), nil), http.MethodGet, "/api/v1/cell", "", "")
		assertCode(t, recorder, http.StatusInternalServerError, CodeInternal)
		rid := recorder.Header().Get("X-Request-ID")
		if rid == "" || !strings.Contains(logs.String(), "rid="+rid+" panic recovered:") {
			t.Fatalf("panic not audited: %s", logs.String())
		}
	})
	t.Run("unauthorized", func(t *testing.T) {
		t.Setenv("GSM_API_TOKEN", "secret")
		recorder := serve(t, New(config.Default(), nil), http.MethodGet, "/api/v1/health", "", "")
		assertCode(t, recorder, http.StatusUnauthorized, CodeUnauthorized)
		rid := recorder.Header().Get("X-Request-ID")
		if rid == "" || !strings.Contains(logs.String(), "rid="+rid+" GET /api/v1/health -> 401") {
			t.Fatalf("unauthorized request not audited: %s", logs.String())
		}
	})
}

func assertCode(t *testing.T, recorder *httptest.ResponseRecorder, status, code int) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status=%d want=%d body=%s", recorder.Code, status, recorder.Body.String())
	}
	var envelope Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid envelope: %v: %s", err, recorder.Body.String())
	}
	if envelope.Code != code {
		t.Fatalf("code=%d want=%d body=%s", envelope.Code, code, recorder.Body.String())
	}
}

func assertEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content type=%q", got)
	}
	var envelope Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.RequestID == "" || envelope.RequestID != recorder.Header().Get("X-Request-ID") {
		t.Fatalf("request id mismatch: header=%q body=%q", recorder.Header().Get("X-Request-ID"), envelope.RequestID)
	}
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

func createOpenBTSFixture(t *testing.T, sqlite, path string) {
	t.Helper()
	runSQLite(t, sqlite, path, `CREATE TABLE CONFIG(KEYSTRING TEXT,VALUESTRING TEXT);`+
		`INSERT INTO CONFIG VALUES('GSM.Radio.ARFCNs','1'),('GSM.Radio.C0','540'),('GSM.Radio.Band','1800'),`+
		`('GSM.Identity.MCC','001'),('GSM.Identity.MNC','01'),('GSM.Identity.LAC','1'),`+
		`('GSM.Identity.CI','0'),('GSM.Identity.ShortName','test');`)
}

func createSubscriberFixture(t *testing.T, sqlite, asteriskPath, tmsiPath string) {
	t.Helper()
	runSQLite(t, sqlite, asteriskPath,
		`CREATE TABLE sip_buddies(id INTEGER PRIMARY KEY,name TEXT NOT NULL,callerid TEXT,ki TEXT DEFAULT '');`+
			`CREATE TABLE dialdata_table(id INTEGER PRIMARY KEY,exten TEXT NOT NULL DEFAULT '',dial TEXT NOT NULL DEFAULT '');`+
			`INSERT INTO sip_buddies(name,callerid,ki) VALUES('IMSI001010123456780','10001','secret-ki-a'),`+
			`('IMSI001010123456781','10002','secret-ki-b');`+
			`INSERT INTO dialdata_table(exten,dial) VALUES('10001','IMSI001010123456780'),`+
			`('10002','IMSI001010123456781');`)
	runSQLite(t, sqlite, tmsiPath,
		`CREATE TABLE tmsi_table(IMSI TEXT,ASSOCIATED_URI TEXT);`+
			`INSERT INTO tmsi_table VALUES('001010123456780','<tel:10001>');`)
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

func smsEvent(qtag, imsi, text string) string {
	return "INFO 10:12 2026-09-08T01:02:03.4 smqueue.cpp:2400:process_timeout: Request Message Delivery for " + qtag + "\n" +
		"NOTICE 10:12 2026-09-08T01:02:03.4 smqueue.h:500:get_text: Decoded text: " + text + "\n" +
		"Deliver message:\nMESSAGE sip:IMSI" + imsi + "@127.0.0.1\n" +
		"From: 10001\nTo: 10002\nContact: <sip:IMSI" + imsi + "@127.0.0.1>\n"
}
