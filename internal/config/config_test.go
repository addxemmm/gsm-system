package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStrictConfig(t *testing.T) {
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
