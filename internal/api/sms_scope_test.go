package api

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/addxemmm/gsm-system/internal/config"
	"github.com/addxemmm/gsm-system/internal/gsm"
)

// Parser/response tests inject an already scoped reader. Manager lifecycle
// tests separately exercise real byte boundaries; no fixture HTTP route exists.
func smsFixtureServer(t *testing.T, cfg config.Config) *Server {
	t.Helper()
	s := New(cfg, gsm.New(cfg))
	s.readCurrentSMS = func(limit int64) ([]byte, gsm.SMSWindow, error) {
		data, window, err := readTail(cfg.LogPath(cfg.SmqueueLogName), limit)
		started := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
		return data, gsm.SMSWindow{Bytes: window.Bytes, MaxBytes: window.MaxBytes,
			Truncated: window.Truncated, Scope: "current_start", SessionID: "fixture-session",
			StartedAt: &started, State: "running"}, err
	}
	return s
}

func TestSMSWithoutCurrentStartNeverReadsPersistentHistory(t *testing.T) {
	cfg, server := testServer(t)
	if err := os.WriteFile(cfg.LogPath(cfg.SmqueueLogName), []byte(smsEvent("old", "001010000000001", "old text")), 0600); err != nil {
		t.Fatal(err)
	}
	r := serve(t, server, http.MethodGet, "/api/v1/sms", "", "")
	assertCode(t, r, http.StatusOK, CodeOK)
	var envelope struct {
		Data struct {
			SMS      []any  `json:"sms"`
			Total    int    `json:"total"`
			Session  any    `json:"session"`
			Scope    string `json:"scope"`
			Timezone string `json:"timezone"`
		} `json:"data"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	d := envelope.Data
	if d.SMS == nil || len(d.SMS) != 0 || d.Total != 0 || d.Session != nil || d.Scope != "current_start" || d.Timezone != "Asia/Shanghai" {
		t.Fatalf("unexpected no-session result: %+v", d)
	}
}

func TestSMSNativeTimeUsesProjectZoneAndPreservesInstants(t *testing.T) {
	loc, err := config.Default().Location()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		raw  string
		want any
	}{
		{"2026-09-08T16:50:08.1", "2026-09-08T16:50:08.1+08:00"},
		{"2026-09-08T08:50:08.1Z", "2026-09-08T16:50:08.1+08:00"},
		{"2026-09-08T09:50:08.1+01:00", "2026-09-08T16:50:08.1+08:00"},
		{"", nil}, {"not-a-time", nil},
	} {
		if got := nativeSMSTime(tc.raw, loc); got != tc.want {
			t.Errorf("raw=%q got=%v want=%v", tc.raw, got, tc.want)
		}
	}
	cfg := config.Default()
	cfg.Timezone = "America/New_York"
	other, err := cfg.Location()
	if err != nil {
		t.Fatal(err)
	}
	if got := nativeSMSTime("2026-09-08T08:50:08Z", other); got != "2026-09-08T04:50:08-04:00" {
		t.Fatalf("custom zone ignored: %v", got)
	}
}

func TestSMSCurrentWindowMetadataAndOffsetTime(t *testing.T) {
	cfg, _ := testServer(t)
	if err := os.WriteFile(cfg.LogPath(cfg.SmqueueLogName), []byte(smsEvent("new", "001010000000001", "text")), 0600); err != nil {
		t.Fatal(err)
	}
	s := smsFixtureServer(t, cfg)
	r := serve(t, s, http.MethodGet, "/api/v1/sms", "", "")
	var envelope struct {
		Data struct {
			SMS []struct {
				Time string `json:"time"`
			} `json:"sms"`
			Session struct {
				ID, State string
				StartedAt string `json:"started_at"`
				EndedAt   any    `json:"ended_at"`
			} `json:"session"`
		} `json:"data"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	d := envelope.Data
	if len(d.SMS) != 1 || d.SMS[0].Time != "2026-09-08T01:02:03.4+08:00" || d.Session.ID != "fixture-session" || d.Session.StartedAt != "2026-09-08T08:00:00+08:00" || d.Session.EndedAt != nil {
		t.Fatalf("unexpected scope/time: %+v", d)
	}
}
