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
	"/api/v1/presets":                   {http.MethodGet, http.MethodPost},
	"/api/v1/presets/{id}":              {http.MethodDelete, http.MethodGet, http.MethodPut},
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
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	h := api.New(cfg, gsm.New(cfg)).Handler()
	for path, methods := range routes {
		probe := strings.ReplaceAll(path, "{imsi}", "001010000000000")
		probe = strings.ReplaceAll(probe, "{id}", "lab-900")
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
		"/iptables", "/healthz", "/status", "/profile", "/presets", "/preset", "/api/v1/ue",
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
	assertCellStartsRequireExplicitRFAck(t, collection.Item)
	assertPostmanDoesNotMutateDefaults(t, collection.Item)
	if got := collectionVariable(collection, "verify_factory_defaults"); got != "false" {
		t.Errorf("Postman collection verify_factory_defaults=%q, want false", got)
	}
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
	if values["verify_factory_defaults"] != "false" {
		t.Fatalf("example environment must not assume an unmodified preset store, got verify_factory_defaults=%q", values["verify_factory_defaults"])
	}
	for key, want := range map[string]string{
		"imsi": "001010000000000", "number": "10000", "iface": "eth0", "preset_id": "lab-900", "default_preset_id": "0", "token": "",
	} {
		if values[key] != want {
			t.Errorf("example %s=%q, want placeholder %q", key, values[key], want)
		}
	}
}

func TestFreshPresetListExposesFiveDefaults(t *testing.T) {
	t.Setenv("GSM_API_TOKEN", "")
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	h := api.New(cfg, gsm.New(cfg)).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/presets", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET presets status=%d body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Items []gsm.Preset `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 0 {
		t.Fatalf("code=%d, want 0", envelope.Code)
	}
	if want := gsm.DefaultPresets(); !reflect.DeepEqual(envelope.Data.Items, want) {
		t.Fatalf("fresh defaults drift\n got: %#v\nwant: %#v", envelope.Data.Items, want)
	}
}

func TestPresetContractArtifacts(t *testing.T) {
	openapi := string(mustRead(t, filepath.Join(root(t), "docs", "api", "openapi.yaml")))
	for _, marker := range []string{
		"pattern: '^[a-z0-9][a-z0-9_-]{0,63}$'",
		"required: [arfcns, c0, band, mcc, mnc, lac, ci, short_name, network]",
		"required: [id, name, params]",
		"required: [name, params]",
		"preset_id:",
		"Location:",
		"items:",
		"deleted:",
	} {
		if !strings.Contains(openapi, marker) {
			t.Errorf("OpenAPI preset contract is missing %q", marker)
		}
	}

	collection := string(mustRead(t, filepath.Join(root(t), "postman", "gsm-system.postman_collection.json")))
	for _, marker := range []string{
		`"preset_id"`,
		`"method": "POST"`,
		`{{baseUrl}}/api/v1/presets`,
		`{{baseUrl}}/api/v1/presets/{{preset_id}}`,
		`\"preset_id\": \"{{default_preset_id}}\"`,
		`"default_preset_id"`,
		`"verify_factory_defaults"`,
		`preset list structure`,
		`=== 'true'`,
		`enable_rf_start`,
	} {
		if !strings.Contains(collection, marker) {
			t.Errorf("Postman preset fixture is missing %q", marker)
		}
	}
	if strings.Contains(collection, `\"network\": \"IFACE\"`) {
		t.Error("Postman cell examples must use the container interface variable, not a host-interface placeholder")
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
	Item     []postmanItem `json:"item"`
	Variable []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"variable"`
}

func collectionVariable(collection postmanCollection, key string) string {
	for _, variable := range collection.Variable {
		if variable.Key == key {
			return variable.Value
		}
	}
	return ""
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
	Body   *struct {
		Raw string `json:"raw"`
	} `json:"body"`
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
			path = strings.ReplaceAll(path, "{{preset_id}}", "{id}")
			path = strings.ReplaceAll(path, "{{default_preset_id}}", "{id}")
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

func assertCellStartsRequireExplicitRFAck(t *testing.T, items []postmanItem) {
	t.Helper()
	for _, item := range items {
		assertCellStartsRequireExplicitRFAck(t, item.Item)
		if item.Request == nil || !strings.EqualFold(item.Request.Method, http.MethodPost) {
			continue
		}
		var rawURL string
		if err := json.Unmarshal(item.Request.URL, &rawURL); err != nil || rawURL != "{{baseUrl}}/api/v1/cell" {
			continue
		}
		var pre []string
		for _, event := range item.Event {
			if event.Listen == "test" {
				t.Errorf("Postman cell start %q must not contain automatic RF tests", item.Name)
			}
			if event.Listen == "prerequest" {
				pre = append(pre, event.Script.Exec...)
			}
		}
		if !strings.Contains(strings.Join(pre, "\n"), "enable_rf_start") {
			t.Errorf("Postman cell start %q requires an explicit enable_rf_start guard", item.Name)
		}
	}
}

func assertPostmanDoesNotMutateDefaults(t *testing.T, items []postmanItem) {
	t.Helper()
	for _, item := range items {
		assertPostmanDoesNotMutateDefaults(t, item.Item)
		if item.Request == nil {
			continue
		}
		var rawURL string
		_ = json.Unmarshal(item.Request.URL, &rawURL)
		usesDefaultInBody := item.Request.Body != nil && strings.Contains(item.Request.Body.Raw, "{{default_preset_id}}")
		usesDefaultInURL := strings.Contains(rawURL, "{{default_preset_id}}")
		if usesDefaultInURL && !strings.EqualFold(item.Request.Method, http.MethodGet) {
			t.Errorf("Postman request %q must not mutate a default preset resource", item.Name)
		}
		if usesDefaultInBody && (!strings.EqualFold(item.Request.Method, http.MethodPost) || rawURL != "{{baseUrl}}/api/v1/cell") {
			t.Errorf("Postman request %q must use the default preset only as a cell-start selector", item.Name)
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
