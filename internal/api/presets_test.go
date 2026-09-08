package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/addxemmm/gsm-system/internal/gsm"
)

const validPresetJSON = `{"id":"lab-900","name":"Lab 900","description":"indoor","params":{"arfcns":"1","c0":"55","band":"900","mcc":"001","mnc":"01","lac":"1","ci":"0","short_name":"lab","network":"eth0"}}`

func TestPresetAPICompleteCRUDAndPersistence(t *testing.T) {
	cfg, server := testServer(t)
	recorder := serve(t, server, http.MethodPost, "/api/v1/presets", "application/json", validPresetJSON)
	assertCode(t, recorder, http.StatusCreated, CodeOK)
	if got := recorder.Header().Get("Location"); got != "/api/v1/presets/lab-900" {
		t.Fatalf("Location=%q", got)
	}
	recorder = serve(t, server, http.MethodPost, "/api/v1/presets", "application/json", validPresetJSON)
	assertCode(t, recorder, http.StatusConflict, CodeConflict)

	recorder = serve(t, server, http.MethodGet, "/api/v1/presets", "", "")
	assertCode(t, recorder, http.StatusOK, CodeOK)
	var list struct {
		Data struct {
			Items []gsm.Preset `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Data.Items) != 1 || list.Data.Items[0].ID != "lab-900" {
		t.Fatalf("list=%+v", list.Data.Items)
	}

	update := strings.Replace(validPresetJSON, `"id":"lab-900",`, "", 1)
	update = strings.Replace(update, `"name":"Lab 900"`, `"name":"Updated"`, 1)
	recorder = serve(t, server, http.MethodPut, "/api/v1/presets/lab-900", "application/json", update)
	assertCode(t, recorder, http.StatusOK, CodeOK)

	reloaded := New(cfg, gsm.New(cfg))
	recorder = serve(t, reloaded, http.MethodGet, "/api/v1/presets/lab-900", "", "")
	assertCode(t, recorder, http.StatusOK, CodeOK)
	if !strings.Contains(recorder.Body.String(), `"name":"Updated"`) {
		t.Fatalf("updated preset not persisted: %s", recorder.Body.String())
	}

	recorder = serve(t, reloaded, http.MethodDelete, "/api/v1/presets/lab-900", "", "")
	assertCode(t, recorder, http.StatusOK, CodeOK)
	if !strings.Contains(recorder.Body.String(), `"deleted":true`) {
		t.Fatalf("delete response=%s", recorder.Body.String())
	}
	recorder = serve(t, reloaded, http.MethodGet, "/api/v1/presets/lab-900", "", "")
	assertCode(t, recorder, http.StatusNotFound, CodeNotFound)
	recorder = serve(t, reloaded, http.MethodDelete, "/api/v1/presets/lab-900", "", "")
	assertCode(t, recorder, http.StatusNotFound, CodeNotFound)
}

func TestPresetAPIValidationStrictJSONAndNoPartialUpdate(t *testing.T) {
	_, server := testServer(t)
	assertCode(t, serve(t, server, http.MethodPost, "/api/v1/presets", "application/json", validPresetJSON), http.StatusCreated, CodeOK)
	tests := []struct {
		method string
		path   string
		body   string
		status int
		code   int
	}{
		{http.MethodPost, "/api/v1/presets", `{"id":"UPPER","name":"x","description":"","params":{}}`, http.StatusUnprocessableEntity, CodeInvalid},
		{http.MethodPost, "/api/v1/presets", strings.Replace(validPresetJSON, `"description":"indoor"`, `"description":"indoor","unknown":true`, 1), http.StatusBadRequest, CodeMalformed},
		{http.MethodPost, "/api/v1/presets", strings.Replace(validPresetJSON, `"network":"eth0"`, `"network":"eth0","extra":"x"`, 1), http.StatusBadRequest, CodeMalformed},
		{http.MethodPut, "/api/v1/presets/lab-900", strings.Replace(strings.Replace(validPresetJSON, `"id":"lab-900",`, "", 1), `"band":"900"`, `"band":"850"`, 1), http.StatusUnprocessableEntity, CodeInvalid},
		{http.MethodPut, "/api/v1/presets/lab-900", validPresetJSON, http.StatusBadRequest, CodeMalformed},
		{http.MethodGet, "/api/v1/presets/BAD", "", http.StatusUnprocessableEntity, CodeInvalid},
		{http.MethodGet, "/api/v1/presets?extra=1", "", http.StatusUnprocessableEntity, CodeInvalid},
	}
	for _, test := range tests {
		recorder := serve(t, server, test.method, test.path, contentTypeFor(test.method), test.body)
		assertCode(t, recorder, test.status, test.code)
	}
	recorder := serve(t, server, http.MethodGet, "/api/v1/presets/lab-900", "", "")
	assertCode(t, recorder, http.StatusOK, CodeOK)
	if !strings.Contains(recorder.Body.String(), `"band":"900"`) {
		t.Fatalf("invalid update changed preset: %s", recorder.Body.String())
	}
}

func TestPresetAPIDescriptionIsOptionalButNotNull(t *testing.T) {
	_, server := testServer(t)
	withoutDescription := strings.Replace(validPresetJSON, `"description":"indoor",`, "", 1)
	recorder := serve(t, server, http.MethodPost, "/api/v1/presets", "application/json", withoutDescription)
	assertCode(t, recorder, http.StatusCreated, CodeOK)
	if !strings.Contains(recorder.Body.String(), `"description":""`) {
		t.Fatalf("omitted description did not default empty: %s", recorder.Body.String())
	}
	update := strings.Replace(withoutDescription, `"id":"lab-900",`, "", 1)
	update = strings.Replace(update, `"name":"Lab 900"`, `"name":"No description"`, 1)
	recorder = serve(t, server, http.MethodPut, "/api/v1/presets/lab-900", "application/json", update)
	assertCode(t, recorder, http.StatusOK, CodeOK)
	if !strings.Contains(recorder.Body.String(), `"description":""`) {
		t.Fatalf("PUT omission did not default empty: %s", recorder.Body.String())
	}
	nullDescription := strings.Replace(withoutDescription, `"name":"Lab 900"`, `"name":"Null","description":null`, 1)
	assertCode(t, serve(t, server, http.MethodPost, "/api/v1/presets", "application/json", nullDescription), http.StatusBadRequest, CodeMalformed)
}

func TestPresetAPIStoreCorruptionIs500AndFailClosed(t *testing.T) {
	cfg, _ := testServer(t)
	path := filepath.Join(cfg.DataDir, "presets.json")
	original := []byte(`{"version":1,"items":[`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	server := New(cfg, gsm.New(cfg))
	assertCode(t, serve(t, server, http.MethodGet, "/api/v1/presets", "", ""), http.StatusInternalServerError, CodeInternal)
	assertCode(t, serve(t, server, http.MethodPost, "/api/v1/presets", "application/json", validPresetJSON), http.StatusInternalServerError, CodeInternal)
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(original) {
		t.Fatalf("corrupt source changed: %q err=%v", after, err)
	}
}

func TestCellStartPresetSelectionAndPresenceBasedExclusivity(t *testing.T) {
	cfg, server := testServer(t)
	cfg.UHDFindBin = filepath.Join(t.TempDir(), "no-rf-probe")
	server = New(cfg, gsm.New(cfg))
	assertCode(t, serve(t, server, http.MethodPost, "/api/v1/presets", "application/json", validPresetJSON), http.StatusCreated, CodeOK)

	for _, body := range []string{
		`{"preset_id":"lab-900","network":""}`,
		`{"preset_id":"lab-900","network":null}`,
		`{"preset_id":"lab-900","band":"900"}`,
	} {
		assertCode(t, serve(t, server, http.MethodPost, "/api/v1/cell", "application/json", body), http.StatusUnprocessableEntity, CodeInvalid)
	}
	assertCode(t, serve(t, server, http.MethodPost, "/api/v1/cell", "application/json", `{"preset_id":"missing"}`), http.StatusNotFound, CodeNotFound)
	// A resolved, valid preset reaches the existing Manager.Start path. The
	// deliberately absent detector keeps this test hardware-free.
	assertCode(t, serve(t, server, http.MethodPost, "/api/v1/cell", "application/json", `{"preset_id":"lab-900"}`), http.StatusServiceUnavailable, CodeNoHardware)
}

func TestPresetAPIMethodAllow(t *testing.T) {
	_, server := testServer(t)
	for _, test := range []struct{ method, path, allow string }{
		{http.MethodPatch, "/api/v1/presets", "GET, POST"},
		{http.MethodPost, "/api/v1/presets/lab-900", "GET, PUT, DELETE"},
	} {
		recorder := serve(t, server, test.method, test.path, "application/json", `{}`)
		assertCode(t, recorder, http.StatusMethodNotAllowed, CodeMethod)
		if got := recorder.Header().Get("Allow"); got != test.allow {
			t.Fatalf("Allow=%q want=%q", got, test.allow)
		}
	}
}

func contentTypeFor(method string) string {
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		return "application/json"
	}
	return ""
}
