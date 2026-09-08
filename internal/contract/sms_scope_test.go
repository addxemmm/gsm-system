package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/addxemmm/gsm-system/internal/api"
	"github.com/addxemmm/gsm-system/internal/config"
	"github.com/addxemmm/gsm-system/internal/gsm"
	"gopkg.in/yaml.v3"
)

// Check the additive scope without weakening the existing identity contract.
// 新范围字段须保持原身份来源契约和 window 三键不变。
func TestSMSCurrentStartSchemaContract(t *testing.T) {
	type schemaNode struct {
		Type       string                `yaml:"type"`
		Format     string                `yaml:"format"`
		Ref        string                `yaml:"$ref"`
		Nullable   bool                  `yaml:"nullable"`
		Required   []string              `yaml:"required"`
		Enum       []string              `yaml:"enum"`
		Properties map[string]schemaNode `yaml:"properties"`
		AllOf      []schemaNode          `yaml:"allOf"`
	}
	var doc struct {
		Paths map[string]struct {
			Get struct {
				Responses map[string]any `yaml:"responses"`
			} `yaml:"get"`
		} `yaml:"paths"`
		Components struct {
			Schemas   map[string]schemaNode `yaml:"schemas"`
			Responses map[string]struct {
				Content map[string]struct {
					Schema schemaNode `yaml:"schema"`
				} `yaml:"content"`
			} `yaml:"responses"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(mustRead(t, filepath.Join(root(t), "docs", "api", "openapi.yaml")), &doc); err != nil {
		t.Fatal(err)
	}
	if _, exists := doc.Paths["/api/v1/sms"].Get.Responses["404"]; exists {
		t.Error("GET SMS missing session/log must return 200 rather than 404")
	}
	data := doc.Components.Responses["SMSListOK"].Content["application/json"].Schema.AllOf[1].Properties["data"]
	want := []string{"count", "limit", "offset", "scope", "session", "sms", "source", "timezone", "total", "truncated", "window"}
	sort.Strings(data.Required)
	if !reflect.DeepEqual(data.Required, want) || len(data.Properties) != len(want) {
		t.Fatalf("SMS data fields drifted: required=%v properties=%v", data.Required, data.Properties)
	}
	if !reflect.DeepEqual(data.Properties["scope"].Enum, []string{"current_start"}) || data.Properties["timezone"].Type != "string" || data.Properties["session"].Ref != "#/components/schemas/SMSObservationSession" {
		t.Error("SMS scope/timezone/session schema drifted")
	}
	session := doc.Components.Schemas["SMSObservationSession"]
	sort.Strings(session.Required)
	if !session.Nullable || !reflect.DeepEqual(session.Required, []string{"ended_at", "id", "started_at", "state"}) || len(session.Properties) != 4 {
		t.Fatalf("nullable session shape drifted: %+v", session)
	}
	if !reflect.DeepEqual(session.Properties["state"].Enum, []string{"starting", "running", "stopped", "boundary_lost"}) {
		t.Errorf("session states drifted: %v", session.Properties["state"].Enum)
	}
	for _, field := range []string{"started_at", "ended_at"} {
		p := session.Properties[field]
		if p.Type != "string" || p.Format != "date-time" || p.Nullable != (field == "ended_at") {
			t.Errorf("session.%s timestamp contract drifted: %+v", field, p)
		}
	}
	messageTime := doc.Components.Schemas["SMSMessage"].Properties["time"]
	if messageTime.Format != "date-time" || !messageTime.Nullable {
		t.Error("SMSMessage.time must be nullable RFC3339 date-time")
	}
	window := doc.Components.Schemas["HistoryWindow"]
	sort.Strings(window.Required)
	if !reflect.DeepEqual(window.Required, []string{"bytes", "max_bytes", "truncated"}) || len(window.Properties) != 3 {
		t.Error("HistoryWindow must retain exactly bytes/max_bytes/truncated")
	}
	markdown := string(mustRead(t, filepath.Join(root(t), "docs", "API.md")))
	for _, marker := range []string{"current_start", "session: null", "boundary_lost", "Asia/Shanghai", "RFC3339Nano", "no all-history query switch"} {
		if !strings.Contains(markdown, marker) {
			t.Errorf("API.md missing %q", marker)
		}
	}
}

// A new management process never adopts an existing SMS log as its own session.
// 管理进程重建不把磁盘旧日志当成本轮数据，也不删除日志。
func TestSMSNoKnownStartReturnsEmptyContract(t *testing.T) {
	t.Setenv("GSM_API_TOKEN", "")
	for _, existingLog := range []bool{false, true} {
		name := "missing-log"
		if existingLog {
			name = "old-log"
		}
		t.Run(name, func(t *testing.T) {
			cfg := config.Default()
			cfg.DataDir = t.TempDir()
			cfg.LogDir = cfg.DataDir
			path := cfg.LogPath(cfg.SmqueueLogName)
			const oldLog = "2026-09-01T00:00:00Z NOTICE GSM_SMS_V1 qtag_hex=6f6c64 from_hex=313031 to_hex=343131 text_hex=686973746f7279\n"
			if existingLog {
				if err := os.WriteFile(path, []byte(oldLog), 0600); err != nil {
					t.Fatal(err)
				}
			}
			h := api.New(cfg, gsm.New(cfg)).Handler()
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/sms?limit=100&offset=0", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("SMS without known start: HTTP %d, body=%s", rec.Code, rec.Body.String())
			}
			var body struct {
				Data map[string]json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			for key, want := range map[string]string{"scope": `"current_start"`, "timezone": `"Asia/Shanghai"`, "session": "null", "sms": "[]", "count": "0", "total": "0"} {
				if got := string(body.Data[key]); got != want {
					t.Errorf("data.%s=%s, want %s", key, got, want)
				}
			}
			if existingLog && string(mustRead(t, path)) != oldLog {
				t.Error("query changed preexisting log")
			}
		})
	}
}
