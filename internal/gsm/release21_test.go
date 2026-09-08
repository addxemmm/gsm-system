package gsm

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRadioCompatibilityBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, field, value string
		valid              bool
	}{
		{"single carrier", "arfcns", "1", true}, {"multiple carriers", "arfcns", "2", false},
		{"DCS low", "c0", "512", true}, {"DCS high", "c0", "885", true}, {"wrong band", "c0", "55", false},
		{"LAC zero", "lac", "0", false}, {"LAC max product", "lac", "65279", true}, {"LAC outside profile", "lac", "65280", false},
		{"CI zero", "ci", "0", true}, {"CI max", "ci", "65535", true}, {"CI overflow", "ci", "65536", false},
		{"iface malformed", "network", "eth0:bad", false}, {"iface long", "network", strings.Repeat("x", 16), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := validStartParams()
			switch tc.field {
			case "arfcns":
				p.ARFCNs = tc.value
			case "c0":
				p.C0 = tc.value
			case "lac":
				p.LAC = tc.value
			case "ci":
				p.CI = tc.value
			case "network":
				p.Network = tc.value
			}
			if got := p.Validate() == nil; got != tc.valid {
				t.Fatalf("valid=%v want=%v: %v", got, tc.valid, p.Validate())
			}
		})
	}
	for _, c0 := range []string{"0", "124", "975", "1023"} {
		p := validStartParams()
		p.Band = "900"
		p.C0 = c0
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}
func TestStatusClassification(t *testing.T) {
	cases := []struct {
		s     Status
		state string
		ready bool
	}{
		{Status{}, "stopped", false}, {Status{OpenBTS: true}, "degraded", false},
		{Status{OpenBTS: true, Transc: true, SipAuth: true, Smqueue: true, Asterisk: true}, "running", true},
		{Status{Transitioning: true}, "transitioning", false},
	}
	for _, tc := range cases {
		tc.s.classify()
		if tc.s.State != tc.state || tc.s.Ready != tc.ready {
			t.Fatalf("bad status %+v", tc.s)
		}
	}
}
func TestStatusDoesNotWaitForLifecycleLock(t *testing.T) {
	m := New(testConfig(t, t.TempDir(), "sqlite3"))
	m.mu.Lock()
	defer m.mu.Unlock()
	done := make(chan Status, 1)
	go func() { done <- m.IsRunning() }()
	select {
	case st := <-done:
		if !st.Transitioning {
			t.Fatal("missing transition state")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("health blocked on operation lock")
	}
}
func TestConcurrentProfileWritesAreComplete(t *testing.T) {
	cfg := testConfig(t, t.TempDir(), "sqlite3")
	m := New(cfg)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- m.SaveProfile(validStartParams()) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := m.LoadProfile(); !ok {
		t.Fatal("invalid final profile")
	}
	tmp, _ := filepath.Glob(filepath.Join(cfg.DataDir, ".last-start-*.tmp"))
	if len(tmp) != 0 {
		t.Fatal("temporary profiles leaked")
	}
}
func TestConfigBatchValidatesCombinationAndRejectsUnknownKeys(t *testing.T) {
	sqlite, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 required")
	}
	cfg := testConfig(t, t.TempDir(), sqlite)
	m := New(cfg)
	initSQL := "CREATE TABLE CONFIG(KEYSTRING TEXT PRIMARY KEY,VALUESTRING TEXT);"
	vals := []string{"1", "540", "1800", "001", "01", "4420", "41240", "test"}
	for i, key := range radioConfigKeys {
		initSQL += "INSERT INTO CONFIG VALUES('" + key + "','" + vals[i] + "');"
	}
	if out, err := exec.Command(sqlite, cfg.OpenBTSDbPath, initSQL).CombinedOutput(); err != nil {
		t.Fatalf("init: %s %v", out, err)
	}
	if err := m.UpdateConfig(map[string]string{"GSM.Radio.C0": "55"}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("accepted incompatible channel: %v", err)
	}
	if err := m.UpdateConfig(map[string]string{"GSM.Radio.Band": "900", "GSM.Radio.C0": "55"}); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateConfig(map[string]string{"Arbitrary.Key": "x"}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatal("unknown key accepted")
	}
	out, err := exec.Command(sqlite, cfg.OpenBTSDbPath, "SELECT VALUESTRING FROM CONFIG WHERE KEYSTRING='GSM.Radio.Band';").Output()
	if err != nil || strings.TrimSpace(string(out)) != "900" {
		t.Fatalf("batch: %q %v", out, err)
	}
	if out, err := exec.Command(sqlite, cfg.OpenBTSDbPath, "CREATE TRIGGER reject_channel BEFORE UPDATE ON CONFIG WHEN OLD.KEYSTRING='GSM.Radio.C0' BEGIN SELECT RAISE(ABORT,'fixture failure'); END;").CombinedOutput(); err != nil {
		t.Fatalf("trigger: %s %v", out, err)
	}
	if err := m.UpdateConfig(map[string]string{"GSM.Radio.Band": "1800", "GSM.Radio.C0": "540"}); err == nil {
		t.Fatal("trigger failure accepted")
	}
	out, err = exec.Command(sqlite, cfg.OpenBTSDbPath, "SELECT VALUESTRING FROM CONFIG WHERE KEYSTRING='GSM.Radio.Band';").Output()
	if err != nil || strings.TrimSpace(string(out)) != "900" {
		t.Fatalf("partial configuration committed: %q %v", out, err)
	}
}
func TestManagedStopAllowsGracefulFlush(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("POSIX signal fixture")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "flushed")
	ready := filepath.Join(dir, "ready")
	bin := writeShell(t, dir, "graceful", "trap 'echo flushed > "+shellQuote(marker)+"; exit 0' TERM\necho ready > "+shellQuote(ready)+"\nwhile :; do sleep 0.1; done\n")
	child, err := startDetached("fixture", bin)
	if err != nil {
		t.Fatal(err)
	}
	defer stopManaged(child, time.Second)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	stopManaged(child, time.Second)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("TERM handler did not flush: %v", err)
	}
}
