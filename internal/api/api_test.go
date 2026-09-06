package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
