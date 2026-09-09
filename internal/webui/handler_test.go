package webui_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/addxemmm/gsm-system/internal/api"
	"github.com/addxemmm/gsm-system/internal/config"
	"github.com/addxemmm/gsm-system/internal/gsm"
	"github.com/addxemmm/gsm-system/internal/webui"
)

func TestEmbeddedAssetsAndStrictPaths(t *testing.T) {
	h := webui.New(http.NotFoundHandler(), "2.1.0", "fixture", false, "")
	for _, tc := range []struct {
		path        string
		status      int
		contentType string
	}{
		{path: "/", status: http.StatusOK, contentType: "text/html"},
		{path: "/favicon.svg", status: http.StatusOK, contentType: "image/svg+xml"},
		{path: "/missing.js", status: http.StatusNotFound},
		{path: "/health", status: http.StatusNotFound},
		{path: "/cell", status: http.StatusNotFound},
		{path: "/dashboard", status: http.StatusNotFound},
		{path: "/api/v10/cell", status: http.StatusNotFound},
		{path: "/assets/", status: http.StatusNotFound},
		{path: "/%2e%2e/go.mod", status: http.StatusNotFound},
		{path: "/assets%5c..%5cindex.html", status: http.StatusNotFound},
	} {
		t.Run(tc.path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "http://example.test"+tc.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
			}
			if tc.contentType != "" && !strings.HasPrefix(w.Header().Get("Content-Type"), tc.contentType) {
				t.Fatalf("Content-Type=%q", w.Header().Get("Content-Type"))
			}
			if w.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'self'") {
				t.Fatalf("missing security headers: %v", w.Header())
			}
		})
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "http://example.test/", nil))
	if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("static POST status=%d headers=%v", w.Code, w.Header())
	}
}

func TestMetadataContainsNoCredential(t *testing.T) {
	const secret = "do-not-transport-this-token"
	t.Setenv("GSM_API_TOKEN", secret)
	h := webui.New(http.NotFoundHandler(), "2.1.0", "abc123", true, "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://example.test/web-meta.json", nil))
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), secret) {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
	var got webui.Metadata
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != "2.1.0" || got.Revision != "abc123" || !got.TokenRequired {
		t.Fatalf("metadata=%+v", got)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("metadata cache policy=%q", w.Header().Get("Cache-Control"))
	}

	head := httptest.NewRecorder()
	h.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "http://example.test/web-meta.json", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD status=%d body=%q", head.Code, head.Body.String())
	}
}

func TestCrossOriginBrowserWritesRejected(t *testing.T) {
	calls := 0
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})
	h := webui.New(apiHandler, "2.1.0", "fixture", false, "")

	request := func(method, origin string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "http://console.test/api/v1/cell", nil)
		r.Host = "console.test"
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	for _, origin := range []string{"http://evil.test", "https://console.test", "null", "http://console.test:81"} {
		if w := request(http.MethodPost, origin); w.Code != http.StatusForbidden || w.Header().Get("X-Request-ID") == "" || !strings.Contains(w.Body.String(), `"request_id"`) {
			t.Fatalf("origin=%q status=%d body=%q", origin, w.Code, w.Body.String())
		}
	}
	if calls != 0 {
		t.Fatalf("cross-origin writes reached API %d times", calls)
	}
	for _, origin := range []string{"", "http://console.test", "http://console.test:80"} {
		if w := request(http.MethodPost, origin); w.Code != http.StatusNoContent {
			t.Fatalf("origin=%q status=%d", origin, w.Code)
		}
	}
	// Origin checks protect writes, not harmless cross-origin status reads.
	if w := request(http.MethodGet, "http://evil.test"); w.Code != http.StatusNoContent {
		t.Fatalf("cross-origin GET status=%d", w.Code)
	}
	if calls != 4 {
		t.Fatalf("allowed API calls=%d", calls)
	}
}

func TestConfiguredPublicOriginSupportsTLSTerminatingProxy(t *testing.T) {
	calls := 0
	h := webui.New(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}), "2.1.0", "fixture", false, "https://console.example")

	for _, tc := range []struct {
		origin string
		want   int
	}{
		{origin: "https://console.example", want: http.StatusNoContent},
		{origin: "https://console.example:443", want: http.StatusNoContent},
		{origin: "http://console.example", want: http.StatusForbidden},
		{origin: "https://internal-proxy:8080", want: http.StatusForbidden},
		{origin: "https://console.example/path", want: http.StatusForbidden},
	} {
		r := httptest.NewRequest(http.MethodPatch, "http://internal-proxy:8080/api/v1/config", nil)
		r.Header.Set("Origin", tc.origin)
		// X-Forwarded headers are untrusted and have no effect; the configured
		// public origin is the only proxy-mode comparison value.
		r.Header.Set("X-Forwarded-Proto", "https")
		r.Header.Set("X-Forwarded-Host", "attacker.example")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("origin=%q status=%d body=%q", tc.origin, w.Code, w.Body.String())
		}
	}
	if calls != 2 {
		t.Fatalf("allowed calls=%d", calls)
	}
}

func TestStandaloneAPIProtectionUsesSameOriginGuard(t *testing.T) {
	calls := 0
	h := webui.ProtectAPI(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}), "")
	r := httptest.NewRequest(http.MethodDelete, "http://api.test/api/v1/cell", nil)
	r.Header.Set("Origin", "http://evil.test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden || calls != 0 || w.Header().Get("X-Request-ID") == "" {
		t.Fatalf("status=%d calls=%d headers=%v", w.Code, calls, w.Header())
	}
}

func TestWebAPIRouteUsesIdenticalBearerAuthentication(t *testing.T) {
	t.Setenv("GSM_API_TOKEN", "fixture-token")
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	direct := api.New(cfg, gsm.New(cfg)).Handler()
	web := webui.New(direct, cfg.Version, cfg.Revision, true, "")

	for _, tc := range []struct {
		name, authorization string
		wantStatus          int
		wantCode            int
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized, wantCode: api.CodeUnauthorized},
		{name: "wrong", authorization: "Bearer wrong", wantStatus: http.StatusUnauthorized, wantCode: api.CodeUnauthorized},
		{name: "correct", authorization: "Bearer fixture-token", wantStatus: http.StatusOK, wantCode: api.CodeOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for name, h := range map[string]http.Handler{"api": direct, "web": web} {
				r := httptest.NewRequest(http.MethodGet, "http://console.test/api/v1/cell", nil)
				r.Header.Set("Authorization", tc.authorization)
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				var envelope api.Envelope
				if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
					t.Fatalf("%s invalid response: %v body=%q", name, err, w.Body.String())
				}
				if w.Code != tc.wantStatus || envelope.Code != tc.wantCode {
					t.Fatalf("%s status=%d code=%d", name, w.Code, envelope.Code)
				}
			}
		})
	}
}
