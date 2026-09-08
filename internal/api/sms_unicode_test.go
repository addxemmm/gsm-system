package api

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

func TestSMSHistoryPreservesChineseStructuredText(t *testing.T) {
	cfg, _ := testServer(t)
	const text = "中文短信 AéΩ€\n第二行"
	log := "NOTICE 10:12 2026-09-08T01:02:03.4 smsc.cpp:320:submitSMS: GSM_SMS_V1 " +
		"qtag_hex=756e69636f6465 from_hex=313031 to_hex=3130303032 text_hex=" + hex.EncodeToString([]byte(text)) + "\n"
	if err := os.WriteFile(cfg.LogPath(cfg.SmqueueLogName), []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := serve(t, smsFixtureServer(t, cfg), http.MethodGet, "/api/v1/sms", "", "")
	assertCode(t, rec, http.StatusOK, CodeOK)
	var body struct {
		Data struct {
			SMS []struct {
				Text *string `json:"text"`
			} `json:"sms"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.SMS) != 1 || body.Data.SMS[0].Text == nil || *body.Data.SMS[0].Text != text {
		t.Fatalf("UTF-8 receive text did not survive API JSON: %s", rec.Body.String())
	}
}
