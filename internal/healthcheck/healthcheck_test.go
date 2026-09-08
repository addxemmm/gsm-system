package healthcheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadOnlyProbe(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		good       bool
	}{
		{"stopped", `{"code":0,"data":{"state":"stopped"}}`, 200, true},
		{"running", `{"code":0,"data":{"state":"running","ready":true,"running":true}}`, 200, true},
		{"degraded200", `{"code":0,"data":{"state":"degraded","running":true}}`, 200, false},
		{"transition", `{"code":0,"data":{"state":"stopped","transitioning":true}}`, 200, false},
		{"missingCode", `{"data":{"state":"stopped"}}`, 200, false},
		{"empty", `{}`, 200, false},
		{"malformed", `not json`, 200, false},
		{"unauthorized", `{}`, 401, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/api/v1/cell" || r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("unexpected probe")
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			err := Check(context.Background(), strings.TrimPrefix(srv.URL, "http://"), " fixture ")
			if (err == nil) != tc.good {
				t.Fatalf("Check=%v, good=%v", err, tc.good)
			}
		})
	}
}
