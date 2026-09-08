package logsink

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRouting(t *testing.T) {
	for line, want := range map[string]string{
		"<30>Sep 8 12:01:00 smqueue[42]: delivery event": "smqueue.log",
		"<30>Sep 8 12:01:00 OpenBTS[43]: system ready":   "openbts-syslog.log",
		"<30>Sep 8 12:01:00 asterisk[44]: call event":    "system.log",
	} {
		if got := logName(line); got != want {
			t.Fatalf("got %s want %s", got, want)
		}
	}
}

func TestCustomSmqueueFilenameIsUsedWithoutChangingOtherRoutes(t *testing.T) {
	dir := t.TempDir()
	s, err := StartWithSmqueueLog("", dir, "custom-sms.log")
	if err != nil {
		t.Fatal(err)
	}
	for line, filename := range map[string]string{
		"<30>Sep 8 12:01:00 smqueue[42]: fixture SMS":   "custom-sms.log",
		"<30>Sep 8 12:01:00 OpenBTS[43]: fixture BTS":   "openbts-syslog.log",
		"<30>Sep 8 12:01:00 asterisk[44]: fixture call": "system.log",
	} {
		if err := s.appendMessage(dir, line); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(dir, filename))
		if err != nil || string(got) != line+"\n" {
			t.Fatalf("route %s: content=%q error=%v", filename, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "smqueue.log")); !os.IsNotExist(err) {
		t.Fatalf("default SMS file should not be written: %v", err)
	}
	legacy, err := Start("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.appendMessage(dir, "smqueue: legacy fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "smqueue.log")); err != nil {
		t.Fatalf("legacy Start lost the default filename: %v", err)
	}
}

func TestCustomSmqueueFilenameRejectsPaths(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../sms.log", `..\sms.log`, "nested/sms.log"} {
		if _, err := StartWithSmqueueLog("", t.TempDir(), name); err == nil {
			t.Errorf("accepted non-basename %q", name)
		}
	}
}

func TestRotationAndPermissions(t *testing.T) {
	p := filepath.Join(t.TempDir(), "smqueue.log")
	if err := appendLog(p, []byte("first\n"), 10); err != nil {
		t.Fatal(err)
	}
	if err := appendLog(p, []byte("second\n"), 10); err != nil {
		t.Fatal(err)
	}
	old, _ := os.ReadFile(p + ".1")
	current, _ := os.ReadFile(p)
	if string(old) != "first\n" || string(current) != "second\n" {
		t.Fatalf("rotation %q / %q", old, current)
	}
	if err := appendLog(p, []byte(strings.Repeat("x", 11)), 10); err == nil {
		t.Fatal("oversized record accepted")
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(p)
		if info.Mode().Perm() != 0600 {
			t.Fatal("private logs must be mode 0600")
		}
	}
}
func TestDisabledAndExistingFile(t *testing.T) {
	s, err := Start("", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "devlog")
	if err := os.WriteFile(p, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(p, dir); err == nil {
		t.Fatal("overwrote regular file")
	}
	data, _ := os.ReadFile(p)
	if string(data) != "keep" {
		t.Fatal("existing file changed")
	}
}
func TestUnixReceiverAndActiveSocketProtection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix datagram runtime test")
	}
	dir, err := os.MkdirTemp("", "gsm-log-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "socket")
	s, err := Start(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := Start(p, dir); err == nil {
		t.Fatal("replaced active socket")
	}
	client, err := net.Dial("unixgram", p)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.Write([]byte("<30>Sep 8 12:01:00 smqueue[42]: fixture\x00")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b, _ := os.ReadFile(filepath.Join(dir, "smqueue.log"))
		if strings.Contains(string(b), "fixture\n") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("native log was not collected")
}
