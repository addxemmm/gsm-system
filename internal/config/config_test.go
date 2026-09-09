package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStrictConfig(t *testing.T) {
	t.Setenv("TZ", "Asia/Shanghai")
	t.Setenv("GSM_DATA_DIR", t.TempDir())
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", "max_history_bytes: 8388608\nsyslog_socket: ''\n", true},
		{"typo", "max_hstory_bytes: 8388608\n", false},
		{"removed legacy", "default_band: '900'\n", false},
		{"extra document", "{}\n---\n{}\n", false},
		{"tiny bound", "max_history_bytes: 1\n", false},
		{"oversized bound", "max_history_bytes: 67108865\n", false},
		{"path traversal", "smqueue_log_name: '../private'\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(p, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(p)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("explicit missing configuration silently ignored")
	}
}
func TestEnvironmentOverlayAndPrivateSocketDisable(t *testing.T) {
	t.Setenv("TZ", "Asia/Shanghai")
	dir := t.TempDir()
	t.Setenv("GSM_DATA_DIR", dir)
	t.Setenv("GSM_SYSLOG_SOCKET", "")
	t.Setenv("GSM_LISTEN", "127.0.0.1:8082")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SyslogSocket != "" || cfg.LogDir != filepath.Join(dir, "log") || cfg.ListenAddr != "127.0.0.1:8082" {
		t.Fatalf("bad overlay: %+v", cfg)
	}
}

func TestWebListenerDefaultsAndEnvironment(t *testing.T) {
	defaults := Default()
	if defaults.ListenAddr != ":8082" || defaults.WebListen != ":18082" || !defaults.WebEnabled {
		t.Fatalf("unexpected default API/Web listeners: %+v", defaults)
	}
	t.Setenv("TZ", "Asia/Shanghai")
	t.Setenv("GSM_WEB_LISTEN", "127.0.0.1:9090")
	t.Setenv("GSM_WEB_ENABLED", "false")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WebEnabled || cfg.WebListen != "127.0.0.1:9090" {
		t.Fatalf("bad web overlay: %+v", cfg)
	}

	t.Setenv("GSM_WEB_ENABLED", "true")
	cfg, err = Load("")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.WebEnabled || cfg.WebListen != "127.0.0.1:9090" {
		t.Fatalf("bad enabled web overlay: %+v", cfg)
	}
}

func TestWebPublicOriginValidationAndEnvironment(t *testing.T) {
	t.Setenv("TZ", "Asia/Shanghai")
	for _, tc := range []struct {
		origin string
		valid  bool
	}{
		{origin: "", valid: true},
		{origin: "http://console.example", valid: true},
		{origin: "https://console.example:8443", valid: true},
		{origin: "https://[2001:db8::1]", valid: true},
		{origin: "console.example"},
		{origin: "ftp://console.example"},
		{origin: "https://user@console.example"},
		{origin: "https://console.example/"},
		{origin: "https://console.example/path"},
		{origin: "https://console.example?q=1"},
		{origin: "https://console.example#fragment"},
		{origin: " https://console.example"},
	} {
		t.Run(tc.origin, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.yaml")
			body := "web_public_origin: " + strconv.Quote(tc.origin) + "\n"
			if err := os.WriteFile(p, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(p)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}

	t.Setenv("GSM_WEB_PUBLIC_ORIGIN", "https://public.example")
	cfg, err := Load("")
	if err != nil || cfg.WebPublicOrigin != "https://public.example" {
		t.Fatalf("origin=%q err=%v", cfg.WebPublicOrigin, err)
	}
}

func TestListenerConfigurationValidation(t *testing.T) {
	t.Setenv("TZ", "Asia/Shanghai")
	for _, tc := range []struct {
		name, api, web, enabled string
		valid                   bool
	}{
		{name: "valid", api: "127.0.0.1:0", web: "[::1]:8080", enabled: "true", valid: true},
		{name: "invalid api", api: "localhost", web: ":8080", enabled: "true"},
		{name: "invalid api port", api: ":70000", web: ":8080", enabled: "true"},
		{name: "invalid web", api: ":8082", web: "localhost", enabled: "true"},
		{name: "empty web while enabled", api: ":8082", web: "", enabled: "true"},
		{name: "disabled ignores web", api: ":8082", web: "not-an-address", enabled: "false", valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "listen_addr: " + strconv.Quote(tc.api) + "\nweb_listen: " + strconv.Quote(tc.web) + "\nweb_enabled: " + tc.enabled + "\n"
			p := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(p, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(p)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}

	t.Setenv("GSM_WEB_ENABLED", "sometimes")
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "GSM_WEB_ENABLED") {
		t.Fatalf("expected boolean environment error, got %v", err)
	}
}

func TestTimezoneDefaultAndEnvironment(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"", "Asia/Shanghai"}, {"Asia/Shanghai", "Asia/Shanghai"},
		{"Asia/Hong_Kong", "Asia/Hong_Kong"}, {"UTC", "UTC"},
		{"America/New_York", "America/New_York"},
	} {
		t.Run(tc.want+tc.name, func(t *testing.T) {
			t.Setenv("TZ", tc.name)
			cfg, err := Load("")
			if err != nil {
				t.Fatal(err)
			}
			loc, err := cfg.Location()
			if err != nil || cfg.Timezone != tc.want || loc.String() != tc.want {
				t.Fatalf("timezone=%q loc=%v err=%v", cfg.Timezone, loc, err)
			}
		})
	}
	loc, err := Default().Location()
	if err != nil {
		t.Fatal(err)
	}
	if got := time.Date(2026, 9, 8, 1, 2, 3, 0, time.UTC).In(loc).Format(time.RFC3339); got != "2026-09-08T09:02:03+08:00" {
		t.Fatalf("default timezone conversion=%s", got)
	}
}

func TestTimezoneYAMLAndEnvironmentPrecedence(t *testing.T) {
	// Remove rather than blank TZ to exercise YAML / 未设 TZ 时使用 YAML。
	old, exists := os.LookupEnv("TZ")
	if err := os.Unsetenv("TZ"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv("TZ", old)
		} else {
			_ = os.Unsetenv("TZ")
		}
	})
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte("timezone: America/New_York\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil || cfg.Timezone != "America/New_York" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
	t.Setenv("TZ", "UTC")
	cfg, err = Load(p)
	if err != nil || cfg.Timezone != "UTC" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}

func TestTimezoneRejectsInvalidConfiguration(t *testing.T) {
	for _, value := range []string{"not/a-zone", "Local", "/etc/localtime", "../UTC", "Asia\\Shanghai", ":UTC", " Asia/Shanghai "} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("TZ", value)
			if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "timezone/TZ") {
				t.Fatalf("expected clear timezone error, got %v", err)
			}
		})
	}
}

func TestTimezoneSummerAndWinterOffsets(t *testing.T) {
	loc, err := (Config{Timezone: "America/New_York"}).Location()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		month time.Month
		want  int
	}{{time.January, -5 * 3600}, {time.July, -4 * 3600}} {
		_, got := time.Date(2026, tc.month, 8, 12, 0, 0, 0, time.UTC).In(loc).Zone()
		if got != tc.want {
			t.Fatalf("month=%s offset=%d want=%d", tc.month, got, tc.want)
		}
	}
}
