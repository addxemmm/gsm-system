package webui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/addxemmm/gsm-system/internal/webui"
)

func TestReadOnlyWebProbe(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		good       bool
	}{
		{name: "ready", body: `{"version":"2.1.0","revision":"fixture","token_required":true}`, status: http.StatusOK, good: true},
		{name: "missing version", body: `{"revision":"fixture"}`, status: http.StatusOK},
		{name: "malformed", body: `{`, status: http.StatusOK},
		{name: "unavailable", body: `{}`, status: http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/web-meta.json" || r.Header.Get("Authorization") != "" {
					t.Error("unexpected web probe")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			err := webui.Check(context.Background(), strings.TrimPrefix(srv.URL, "http://"))
			if (err == nil) != tc.good {
				t.Fatalf("Check=%v good=%v", err, tc.good)
			}
		})
	}
}
