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
	"regexp"
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
	if got := canonical(readMarkdownRoutes(t)); !reflect.DeepEqual(got, want) {
		t.Fatalf("API.md route drift\n got: %#v\nwant: %#v", got, want)
	}
	assertCellStartInputChecks(t, collection.Item)
	for _, key := range []string{"enable_mutations", "enable_rf_start"} {
		if strings.Contains(string(mustRead(t, filepath.Join(root(t), "postman", "gsm-system.postman_collection.json"))), key) {
			t.Errorf("Postman collection must not require retired client switch %s", key)
		}
	}
	assertPostmanDoesNotMutateDefaults(t, collection.Item)
	if got := collectionVariable(collection, "verify_factory_defaults"); got != "false" {
		t.Errorf("Postman collection verify_factory_defaults=%q, want false", got)
	}
}

func TestOpenAPIOperationsDeclareCommonMiddlewareErrors(t *testing.T) {
	type operation struct {
		Responses map[string]any `yaml:"responses"`
	}
	var doc struct {
		Paths map[string]struct {
			Get    *operation `yaml:"get"`
			Post   *operation `yaml:"post"`
			Put    *operation `yaml:"put"`
			Patch  *operation `yaml:"patch"`
			Delete *operation `yaml:"delete"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(mustRead(t, filepath.Join(root(t), "docs", "api", "openapi.yaml")), &doc); err != nil {
		t.Fatal(err)
	}
	for path, methods := range routes {
		item := doc.Paths[path]
		for _, method := range methods {
			var op *operation
			switch method {
			case http.MethodGet:
				op = item.Get
			case http.MethodPost:
				op = item.Post
			case http.MethodPut:
				op = item.Put
			case http.MethodPatch:
				op = item.Patch
			case http.MethodDelete:
				op = item.Delete
			}
			if op == nil {
				t.Errorf("OpenAPI is missing %s %s", method, path)
				continue
			}
			for _, status := range []string{"401", "422"} {
				if _, ok := op.Responses[status]; !ok {
					t.Errorf("OpenAPI %s %s must declare common %s response", method, path, status)
				}
			}
		}
	}
}

func TestConfigPatchOpenAPIUsesExactPublicAllowlist(t *testing.T) {
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					AdditionalProperties any `yaml:"additionalProperties"`
					Properties           map[string]struct {
						Type string `yaml:"type"`
					} `yaml:"properties"`
				} `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(mustRead(t, filepath.Join(root(t), "docs", "api", "openapi.yaml")), &doc); err != nil {
		t.Fatal(err)
	}
	values := doc.Components.Schemas["ConfigPatch"].Properties["values"]
	if values.AdditionalProperties != false {
		t.Fatalf("ConfigPatch.values must reject non-allowlisted keys, got %#v", values.AdditionalProperties)
	}
	got := make([]string, 0, len(values.Properties))
	for key, property := range values.Properties {
		got = append(got, key)
		if property.Type != "string" {
			t.Errorf("ConfigPatch.values.%s must be a string", key)
		}
	}
	sort.Strings(got)
	want := []string{
		"GSM.Identity.CI", "GSM.Identity.LAC", "GSM.Identity.MCC", "GSM.Identity.MNC",
		"GSM.Identity.ShortName", "GSM.Radio.ARFCNs", "GSM.Radio.Band", "GSM.Radio.C0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ConfigPatch allowlist drifted: got %v, want %v", got, want)
	}
	collection := string(mustRead(t, filepath.Join(root(t), "postman", "gsm-system.postman_collection.json")))
	for _, marker := range []string{"Config rejects non-public native key", "Control.LUR.OpenRegistration.Message", "HTTP 422 unsupported config key"} {
		if !strings.Contains(collection, marker) {
			t.Errorf("Postman config allowlist negative case is missing %q", marker)
		}
	}
}

func TestPostmanExampleIsPlaceholderOnlyWithoutEnableSwitches(t *testing.T) {
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
	for _, key := range []string{"enable_mutations", "enable_rf_start"} {
		if _, exists := values[key]; exists {
			t.Errorf("example environment must omit retired client switch %s", key)
		}
	}
	if values["verify_factory_defaults"] != "false" {
		t.Fatalf("example environment must not assume an unmodified preset store, got verify_factory_defaults=%q", values["verify_factory_defaults"])
	}
	for key, want := range map[string]string{
		"imsi": "001010000000000", "number": "10000", "iface": "eth0", "preset_id": "lab-900", "default_preset_id": "0", "start_preset_id": "0", "token": "",
		"arfcns": "1", "band": "1800", "short_name": "addx",
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
		`\"preset_id\": \"{{start_preset_id}}\"`,
		`"default_preset_id"`,
		`"verify_factory_defaults"`,
		`preset list structure`,
		`=== 'true'`,
		`GSM SENDING`,
	} {
		if !strings.Contains(collection, marker) {
			t.Errorf("Postman preset fixture is missing %q", marker)
		}
	}
	if strings.Contains(collection, `\"network\": \"IFACE\"`) {
		t.Error("Postman cell examples must use the container interface variable, not a host-interface placeholder")
	}
}

func TestConnectionRegistrationDiagnosticsContract(t *testing.T) {
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Required   []string `yaml:"required"`
				Properties map[string]struct {
					Type     string   `yaml:"type"`
					Nullable bool     `yaml:"nullable"`
					Enum     []string `yaml:"enum"`
				} `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(mustRead(t, filepath.Join(root(t), "docs", "api", "openapi.yaml")), &doc); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"auth", "reject_code"} {
		property, exists := doc.Components.Schemas["Connection"].Properties[field]
		if !exists || property.Type != "integer" || !property.Nullable {
			t.Errorf("Connection.%s must expose nullable raw integer diagnostics", field)
		}
	}
	numberSource, exists := doc.Components.Schemas["Connection"].Properties["number_source"]
	wantSources := []string{"inconsistent_registry", "openbts_tmsi", "subscriber_registry"}
	sort.Strings(numberSource.Enum)
	if !exists || numberSource.Type != "string" || !numberSource.Nullable || !reflect.DeepEqual(numberSource.Enum, wantSources) {
		t.Errorf("Connection.number_source must be optional, nullable, and enumerate all number provenances: %+v", numberSource)
	}
	for _, required := range doc.Components.Schemas["Connection"].Required {
		if required == "number_source" {
			t.Error("Connection.number_source must remain optional during rolling upgrades")
		}
	}
	collection, _ := readPostman(t)
	found := false
	var walk func([]postmanItem)
	walk = func(items []postmanItem) {
		for _, item := range items {
			walk(item.Item)
			if item.Request == nil || item.Request.Method != http.MethodGet {
				continue
			}
			var url string
			_ = json.Unmarshal(item.Request.URL, &url)
			if !strings.HasPrefix(url, "{{baseUrl}}/api/v1/connections") {
				continue
			}
			found = true
			var checks []string
			for _, event := range item.Event {
				if event.Listen == "test" {
					checks = append(checks, event.Script.Exec...)
				}
			}
			joined := strings.Join(checks, "\n")
			for _, field := range []string{"auth", "reject_code", "number_source"} {
				if !strings.Contains(joined, field) {
					t.Errorf("Postman connection checks must cover %s", field)
				}
			}
			for _, marker := range []string{"subscriber_registry", "openbts_tmsi", "inconsistent_registry", "c.number).to.eql(null)"} {
				if !strings.Contains(joined, marker) {
					t.Errorf("Postman connection checks are missing number provenance rule %q", marker)
				}
			}
		}
	}
	walk(collection.Item)
	if !found {
		t.Fatal("Postman is missing connections query")
	}
}

func TestSMSObservationSchemaAndPostmanContract(t *testing.T) {
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Required   []string `yaml:"required"`
				Properties map[string]struct {
					Type     string `yaml:"type"`
					Nullable bool   `yaml:"nullable"`
					Ref      string `yaml:"$ref"`
				} `yaml:"properties"`
				Enum []string `yaml:"enum"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(mustRead(t, filepath.Join(root(t), "docs", "api", "openapi.yaml")), &doc); err != nil {
		t.Fatal(err)
	}
	wantObservationFields := []string{"receiver_imsi", "receiver_number", "sender_imsi", "sender_number", "text", "time"}
	wantFields := append(append([]string{}, wantObservationFields...), "identity_resolution")
	sort.Strings(wantFields)
	schema := doc.Components.Schemas["SMSMessage"]
	sort.Strings(schema.Required)
	if !reflect.DeepEqual(schema.Required, wantFields) {
		t.Fatalf("SMSMessage required fields drifted: %v", schema.Required)
	}
	gotFields := make([]string, 0, len(schema.Properties))
	for field, property := range schema.Properties {
		gotFields = append(gotFields, field)
		if field == "identity_resolution" {
			if property.Ref != "#/components/schemas/SMSIdentityResolution" {
				t.Errorf("SMSMessage.identity_resolution ref=%q", property.Ref)
			}
			continue
		}
		if property.Type != "string" || !property.Nullable {
			t.Errorf("SMSMessage.%s must remain a nullable string", field)
		}
	}
	sort.Strings(gotFields)
	if !reflect.DeepEqual(gotFields, wantFields) {
		t.Fatalf("SMSMessage properties drifted: %v", gotFields)
	}
	identityFields := []string{"receiver_imsi", "receiver_number", "sender_imsi", "sender_number"}
	resolution := doc.Components.Schemas["SMSIdentityResolution"]
	sort.Strings(resolution.Required)
	if !reflect.DeepEqual(resolution.Required, identityFields) {
		t.Fatalf("SMSIdentityResolution required fields drifted: %v", resolution.Required)
	}
	for _, field := range identityFields {
		if resolution.Properties[field].Ref != "#/components/schemas/SMSIdentitySource" {
			t.Errorf("SMSIdentityResolution.%s must reference SMSIdentitySource", field)
		}
	}
	wantSources := []string{"current_subscriber_binding", "log_observation", "unknown"}
	sources := doc.Components.Schemas["SMSIdentitySource"].Enum
	sort.Strings(sources)
	if !reflect.DeepEqual(sources, wantSources) {
		t.Fatalf("SMSIdentitySource enum drifted: %v", sources)
	}

	collection, _ := readPostman(t)
	found := false
	var walk func([]postmanItem)
	walk = func(items []postmanItem) {
		for _, item := range items {
			walk(item.Item)
			if item.Request == nil || item.Request.Method != http.MethodGet {
				continue
			}
			var url string
			_ = json.Unmarshal(item.Request.URL, &url)
			if !strings.HasPrefix(url, "{{baseUrl}}/api/v1/sms") {
				continue
			}
			found = true
			var checks []string
			for _, event := range item.Event {
				if event.Listen == "test" {
					checks = append(checks, event.Script.Exec...)
				}
			}
			joined := strings.Join(checks, "\n")
			for _, marker := range append(wantFields, "d.count", "d.sms.length", "typeof m[k]", "d.limit", "d.offset", "d.window.bytes", "d.window.max_bytes", "d.truncated", "d.window.truncated", "log_observation", "current_subscriber_binding", "unknown", "d.scope", "current_start", "d.timezone", "d.session", "session.id", "session.started_at", "session.ended_at", "session.state", "boundary_lost", "Date.parse", "isOffsetTime(m.time)", "['bytes','max_bytes','truncated']") {
				if !strings.Contains(joined, marker) {
					t.Errorf("Postman SMS checks are missing %q", marker)
				}
			}
			for _, marker := range []string{"GSM_SMS_V1", "qtag_hex", "message identity/qtag", "never by content", "unkeyed text remains null", "never associated", "identity_resolution", "current_subscriber_binding", "101/411", "not historical facts", "delivery receipts", "current_start", "session=null", "Asia/Shanghai", "boundary_lost", "RFC3339Nano", "no all-history switch"} {
				if !strings.Contains(item.Request.Description, marker) {
					t.Errorf("Postman SMS description is missing identity/observation rule %q", marker)
				}
			}
		}
	}
	walk(collection.Item)
	if !found {
		t.Fatal("Postman is missing SMS observation query")
	}
}

func TestSMSSubmissionExampleAndPostmanContract(t *testing.T) {
	var doc struct {
		Components struct {
			Responses map[string]struct {
				Content map[string]struct {
					Example struct {
						Code      int    `yaml:"code"`
						Message   string `yaml:"message"`
						RequestID string `yaml:"request_id"`
						Data      struct {
							Status string `yaml:"status"`
							IMSI   string `yaml:"imsi"`
						} `yaml:"data"`
					} `yaml:"example"`
				} `yaml:"content"`
			} `yaml:"responses"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(mustRead(t, filepath.Join(root(t), "docs", "api", "openapi.yaml")), &doc); err != nil {
		t.Fatal(err)
	}
	example := doc.Components.Responses["Accepted"].Content["application/json"].Example
	if example.Code != 0 || example.Message != "sms submitted" || example.Data.Status != "submitted" ||
		example.Data.IMSI != "IMSI" || example.RequestID != "REQUEST_ID" {
		t.Fatalf("OpenAPI Accepted SMS example drifted: %+v", example)
	}

	collection, _ := readPostman(t)
	found := false
	var walk func([]postmanItem)
	walk = func(items []postmanItem) {
		for _, item := range items {
			walk(item.Item)
			if item.Request == nil || item.Request.Method != http.MethodPost {
				continue
			}
			var url string
			_ = json.Unmarshal(item.Request.URL, &url)
			if url != "{{baseUrl}}/api/v1/sms" || item.Name != "POST /api/v1/sms" {
				continue
			}
			found = true
			var checks []string
			for _, event := range item.Event {
				if event.Listen == "test" {
					checks = append(checks, event.Script.Exec...)
				}
			}
			joined := strings.Join(checks, "\n")
			for _, marker := range []string{"sms submitted", "data.status", "data.imsi", "{{imsi}}"} {
				if !strings.Contains(joined, marker) {
					t.Errorf("Postman SMS submission checks are missing %q", marker)
				}
			}
		}
	}
	walk(collection.Item)
	if !found {
		t.Fatal("Postman is missing SMS submission request")
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

func readMarkdownRoutes(t *testing.T) map[string][]string {
	t.Helper()
	markdown := string(mustRead(t, filepath.Join(root(t), "docs", "API.md")))
	heading := regexp.MustCompile("(?m)^### `(?P<method>GET|POST|PUT|PATCH|DELETE) (?P<path>/[^`\\[]+)")
	out := map[string][]string{}
	for _, match := range heading.FindAllStringSubmatch(markdown, -1) {
		out["/api/v1"+match[2]] = append(out["/api/v1"+match[2]], match[1])
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
	Method      string          `json:"method"`
	URL         json.RawMessage `json:"url"`
	Description string          `json:"description"`
	Body        *struct {
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

func assertCellStartInputChecks(t *testing.T, items []postmanItem) {
	t.Helper()
	for _, item := range items {
		assertCellStartInputChecks(t, item.Item)
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
		for _, marker := range []string{"console.error", "GSM NOT SENT", "pm.execution.skipRequest", "pm.variables.replaceIn", "JSON.parse", "GSM SENDING"} {
			if !strings.Contains(strings.Join(pre, "\n"), marker) {
				t.Errorf("Postman cell start %q is missing %s diagnostics/validation", item.Name, marker)
			}
		}
	}
}

func TestPostmanContainsIndependentPresetAndCustomStarts(t *testing.T) {
	collection, _ := readPostman(t)
	counts := map[string]int{}
	var visit func([]postmanItem)
	visit = func(items []postmanItem) {
		for _, item := range items {
			visit(item.Item)
			if item.Request == nil || item.Request.Method != http.MethodPost {
				continue
			}
			var url string
			_ = json.Unmarshal(item.Request.URL, &url)
			if url != "{{baseUrl}}/api/v1/cell" {
				continue
			}
			if item.Request.Body == nil {
				t.Fatalf("cell start %s requires an explicit body", item.Name)
			}
			var body map[string]string
			if err := json.Unmarshal([]byte(item.Request.Body.Raw), &body); err != nil {
				t.Fatal(err)
			}
			if id, ok := body["preset_id"]; ok {
				counts["preset"]++
				if id != "{{start_preset_id}}" || len(body) != 1 || !strings.Contains(item.Name, "按预设启动") {
					t.Errorf("preset start must use its own selector and no overrides: %v", body)
				}
				continue
			}
			counts["custom"]++
			fields := []string{"arfcns", "c0", "band", "mcc", "mnc", "lac", "ci", "short_name", "network"}
			if len(body) != len(fields) || !strings.Contains(item.Name, "自定义参数启动") {
				t.Errorf("custom start must contain all nine fields: %v", body)
			}
			resolved := map[string]string{}
			for _, key := range fields {
				variable := key
				if key == "network" {
					variable = "iface"
				}
				if body[key] != "{{"+variable+"}}" {
					t.Errorf("custom field %s must be configurable, got %q", key, body[key])
				}
				resolved[key] = collectionVariable(collection, variable)
			}
			encoded, _ := json.Marshal(resolved)
			var params gsm.StartParams
			_ = json.Unmarshal(encoded, &params)
			if err := params.Validate(); err != nil {
				t.Errorf("custom default variables are invalid: %v", err)
			}
		}
	}
	visit(collection.Item)
	if !reflect.DeepEqual(counts, map[string]int{"preset": 1, "custom": 1}) {
		t.Fatalf("want exactly one preset and one custom start, got %v", counts)
	}
	for key, want := range map[string]string{"start_preset_id": "0"} {
		if got := collectionVariable(collection, key); got != want {
			t.Errorf("collection %s=%q, want %q", key, got, want)
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
