// Package contract prevents the Go router, OpenAPI, and Postman artifacts from
// silently describing different public APIs.
package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/addxemmm/gsm-system/internal/api"
	"github.com/addxemmm/gsm-system/internal/config"
	"github.com/addxemmm/gsm-system/internal/gsm"
	"gopkg.in/yaml.v3"
)

var routes = map[string][]string{
	"/api/v1/cell":                      {http.MethodDelete, http.MethodGet, http.MethodPost},
	"/api/v1/config":                    {http.MethodGet, http.MethodPatch},
	"/api/v1/profile":                   {http.MethodGet},
	"/api/v1/health":                    {http.MethodGet},
	"/api/v1/connections":               {http.MethodGet},
	"/api/v1/subscribers":               {http.MethodGet},
	"/api/v1/subscribers/{imsi}":        {http.MethodGet},
	"/api/v1/subscribers/{imsi}/number": {http.MethodDelete, http.MethodPut},
	"/api/v1/sms":                       {http.MethodGet, http.MethodPost},
	"/api/v1/calls":                     {http.MethodGet},
	"/api/v1/calls/history":             {http.MethodGet},
	"/api/v1/network":                   {http.MethodGet, http.MethodPut},
}

func TestRouterExposesOnlyRelease21Routes(t *testing.T) {
	t.Setenv("GSM_API_TOKEN", "")
	h := api.New(config.Default(), gsm.New(config.Default())).Handler()
	for path, methods := range routes {
		probe := strings.ReplaceAll(path, "{imsi}", "001010000000000")
		req := httptest.NewRequest(http.MethodOptions, probe, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("OPTIONS %s: got %d, want 405 for a registered route", path, rec.Code)
			continue
		}
		got := splitAllow(rec.Header().Get("Allow"))
		if !reflect.DeepEqual(got, methods) {
			t.Errorf("OPTIONS %s Allow=%v, want %v", path, got, methods)
		}
	}

	retired := []string{
		"/start", "/stop", "/ueinfo", "/smsinfo", "/sendsms",
		"/setphonenumber", "/config", "/getconfig", "/allconfig",
		"/iptables", "/healthz", "/status", "/profile", "/api/v1/ue",
	}
	for _, path := range retired {
		req := httptest.NewRequest(http.MethodOptions, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("retired route %s: got %d, want 404", path, rec.Code)
		}
	}
}

func TestOpenAPIAndPostmanMatchRouter(t *testing.T) {
	want := canonical(routes)
	if got := canonical(readOpenAPIRoutes(t)); !reflect.DeepEqual(got, want) {
		t.Fatalf("OpenAPI route drift\n got: %#v\nwant: %#v", got, want)
	}
	collection, got := readPostman(t)
	if got = canonical(got); !reflect.DeepEqual(got, want) {
		t.Fatalf("Postman route drift\n got: %#v\nwant: %#v", got, want)
	}
	assertMutationGuards(t, collection.Item)
}

func TestPostmanExampleIsReadOnlyAndPlaceholderOnly(t *testing.T) {
	b := mustRead(t, filepath.Join(root(t), "postman", "gsm-system.postman_environment.example.json"))
	var env struct {
		Values []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"values"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, value := range env.Values {
		values[value.Key] = value.Value
	}
	if values["enable_mutations"] != "false" {
		t.Fatalf("example environment must default to read-only, got %q", values["enable_mutations"])
	}
	for key, want := range map[string]string{
		"imsi": "001010000000000", "number": "10000", "iface": "IFACE", "token": "",
	} {
		if values[key] != want {
			t.Errorf("example %s=%q, want placeholder %q", key, values[key], want)
		}
	}
}

func readOpenAPIRoutes(t *testing.T) map[string][]string {
	t.Helper()
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(mustRead(t, filepath.Join(root(t), "docs", "api", "openapi.yaml")), &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for path, item := range doc.Paths {
		for key := range item {
			method := strings.ToUpper(key)
			switch method {
			case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				out[path] = append(out[path], method)
			}
		}
	}
	return out
}

type postmanCollection struct {
	Item []postmanItem `json:"item"`
}

type postmanItem struct {
	Name    string          `json:"name"`
	Item    []postmanItem   `json:"item"`
	Event   []postmanEvent  `json:"event"`
	Request *postmanRequest `json:"request"`
}

type postmanEvent struct {
	Listen string `json:"listen"`
	Script struct {
		Exec []string `json:"exec"`
	} `json:"script"`
}

type postmanRequest struct {
	Method string          `json:"method"`
	URL    json.RawMessage `json:"url"`
}

func readPostman(t *testing.T) (postmanCollection, map[string][]string) {
	t.Helper()
	var collection postmanCollection
	if err := json.Unmarshal(mustRead(t, filepath.Join(root(t), "postman", "gsm-system.postman_collection.json")), &collection); err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	var walk func([]postmanItem)
	walk = func(items []postmanItem) {
		for _, item := range items {
			walk(item.Item)
			if item.Request == nil {
				continue
			}
			var raw string
			if err := json.Unmarshal(item.Request.URL, &raw); err != nil {
				t.Fatalf("Postman request %q must use a string URL: %v", item.Name, err)
			}
			path := strings.TrimPrefix(strings.SplitN(raw, "?", 2)[0], "{{baseUrl}}")
			path = strings.ReplaceAll(path, "{{imsi}}", "{imsi}")
			out[path] = append(out[path], strings.ToUpper(item.Request.Method))
		}
	}
	walk(collection.Item)
	return collection, out
}

func assertMutationGuards(t *testing.T, items []postmanItem) {
	t.Helper()
	for _, item := range items {
		assertMutationGuards(t, item.Item)
		if item.Request == nil || strings.EqualFold(item.Request.Method, http.MethodGet) {
			continue
		}
		var scripts []string
		for _, event := range item.Event {
			if event.Listen == "prerequest" {
				scripts = append(scripts, event.Script.Exec...)
			}
		}
		joined := strings.Join(scripts, "\n")
		if !strings.Contains(joined, "enable_mutations") || !strings.Contains(joined, "pm.execution.skipRequest") {
			t.Errorf("Postman mutation %q has no independent enable_mutations skip guard", item.Name)
		}
	}
}

func canonical(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for path, methods := range in {
		set := map[string]bool{}
		for _, method := range methods {
			set[strings.ToUpper(method)] = true
		}
		for method := range set {
			out[path] = append(out[path], method)
		}
		sort.Strings(out[path])
	}
	return out
}

func splitAllow(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	sort.Strings(parts)
	return parts
}

func root(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate contract test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
