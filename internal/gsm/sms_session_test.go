package gsm

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/addxemmm/gsm-system/internal/config"
)

func smsTestManager(t *testing.T, historical string) (*Manager, string) {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.ConfDir = filepath.Join(cfg.DataDir, "conf")
	cfg.LogDir = t.TempDir()
	path := cfg.LogPath(cfg.SmqueueLogName)
	if err := os.WriteFile(path, []byte(historical), 0600); err != nil {
		t.Fatal(err)
	}
	return New(cfg), path
}

func appendSMSFixture(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}

func beginSMSFixture(t *testing.T, m *Manager) {
	t.Helper()
	if err := m.beginSMSSession(); err != nil {
		t.Fatal(err)
	}
	m.commitSMSSession()
}

func assertSMSFixture(t *testing.T, m *Manager, want, state string) SMSWindow {
	t.Helper()
	b, w, err := m.ReadCurrentSMS(1024)
	if err != nil || string(b) != want || w.State != state || w.Bytes != int64(len(b)) || w.Scope != "current_start" {
		t.Fatalf("got %q / %+v / %v, want %q / %s", b, w, err, want, state)
	}
	return w
}

func TestSMSNoSessionNeverReadsPersistentHistory(t *testing.T) {
	m, path := smsTestManager(t, "historical\n")
	w := assertSMSFixture(t, m, "", "none")
	if w.SessionID != "" || w.StartedAt != nil {
		t.Fatal("manager reconstruction fabricated a session")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	assertSMSFixture(t, m, "", "none")
	beginSMSFixture(t, m)
	appendSMSFixture(t, path, "fresh\n")
	assertSMSFixture(t, m, "fresh\n", "running")
	assertSMSFixture(t, New(m.cfg), "", "none")
}

func TestSMSLaunchBoundaryStopAndNextLaunch(t *testing.T) {
	m, path := smsTestManager(t, "historical\n")
	beginSMSFixture(t, m)
	first := assertSMSFixture(t, m, "", "running")
	appendSMSFixture(t, path, "first\n")
	assertSMSFixture(t, m, "first\n", "running")
	m.endSMSSession()
	appendSMSFixture(t, path, "outside\n")
	stopped := assertSMSFixture(t, m, "first\n", "stopped")
	if stopped.EndedAt == nil || stopped.SessionID != first.SessionID {
		t.Fatal("stop did not preserve the launch metadata")
	}
	m.endSMSSession()
	assertSMSFixture(t, m, "first\n", "stopped")
	beginSMSFixture(t, m)
	appendSMSFixture(t, path, "second\n")
	second := assertSMSFixture(t, m, "second\n", "running")
	if second.SessionID == first.SessionID || second.EndedAt != nil {
		t.Fatal("next launch did not replace the previous boundary")
	}
	m.clearSMSSession()
	assertSMSFixture(t, m, "", "none")
}

func TestSMSPartialLinesAndBoundedRead(t *testing.T) {
	m, path := smsTestManager(t, "old incomplete")
	beginSMSFixture(t, m)
	appendSMSFixture(t, path, " continuation\nnew\npartial")
	assertSMSFixture(t, m, "new\n", "running")
	appendSMSFixture(t, path, " completed\n")
	assertSMSFixture(t, m, "new\npartial completed\n", "running")
	b, w, err := m.ReadCurrentSMS(20)
	if err != nil || string(b) != "partial completed\n" || !w.Truncated || w.Bytes > 20 {
		t.Fatalf("bounded read: %q %+v %v", b, w, err)
	}
	if _, _, err := m.ReadCurrentSMS(0); err == nil {
		t.Fatal("zero limit accepted")
	}
}

func TestSMSRotationKeepsOnlySessionBytes(t *testing.T) {
	m, path := smsTestManager(t, "historical\n")
	beginSMSFixture(t, m)
	appendSMSFixture(t, path, "one\n")
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	appendSMSFixture(t, path, "two\n")
	assertSMSFixture(t, m, "one\ntwo\n", "running")
	m.endSMSSession()
	appendSMSFixture(t, path, "not in stopped launch\n")
	assertSMSFixture(t, m, "one\ntwo\n", "stopped")
	if err := os.Remove(path + ".1"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	appendSMSFixture(t, path, "newest\n")
	w := assertSMSFixture(t, m, "", "boundary_lost")
	if !w.Truncated {
		t.Fatal("lost rotation boundary not marked truncated")
	}
}

func TestSMSTruncateRewriteAndReplacementFailClosed(t *testing.T) {
	for _, mode := range []string{"truncate", "regrow", "replace"} {
		t.Run(mode, func(t *testing.T) {
			m, path := smsTestManager(t, "historical\n")
			beginSMSFixture(t, m)
			if mode == "replace" {
				if err := os.Rename(path, filepath.Join(filepath.Dir(path), "outside.txt")); err != nil {
					t.Fatal(err)
				}
			}
			text := ""
			if mode != "truncate" {
				text = "unrelated rewritten history much longer\n"
			}
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			assertSMSFixture(t, m, "", "boundary_lost")
			appendSMSFixture(t, path, "must remain closed\n")
			assertSMSFixture(t, m, "", "boundary_lost")
		})
	}
}

func TestSMSReadDoesNotWaitForLifecycleMutex(t *testing.T) {
	m, path := smsTestManager(t, "old\n")
	beginSMSFixture(t, m)
	appendSMSFixture(t, path, "new\n")
	m.mu.Lock()
	defer m.mu.Unlock()
	assertSMSFixture(t, m, "new\n", "running")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				m.ReadCurrentSMS(1024)
			}
		}()
	}
	m.endSMSSession()
	wg.Wait()
}

func TestSMSRejectedDuplicateStartKeepsExistingSession(t *testing.T) {
	m, path := smsTestManager(t, "old\n")
	beginSMSFixture(t, m)
	appendSMSFixture(t, path, "current\n")
	// A live managed handle makes the duplicate guard deterministic without RF.
	m.openbts = &managedProcess{cmd: &exec.Cmd{Process: &os.Process{Pid: os.Getpid()}}, done: make(chan struct{})}
	before := assertSMSFixture(t, m, "current\n", "running")
	if err := m.Start(context.Background(), validStartParams()); !errors.Is(err, ErrCellRunning) {
		t.Fatalf("duplicate start: %v", err)
	}
	after := assertSMSFixture(t, m, "current\n", "running")
	if before.SessionID != after.SessionID {
		t.Fatal("rejected duplicate reset current session")
	}
}

func TestSMSFailedAcceptedStartClearsPreviousSession(t *testing.T) {
	m, path := smsTestManager(t, "old\n")
	beginSMSFixture(t, m)
	appendSMSFixture(t, path, "previous launch\n")
	m.endSMSSession()
	m.cfg.UHDFindBin = filepath.Join(t.TempDir(), "missing-detector")
	p := validStartParams()
	p.Network = "lo"
	if err := m.Start(context.Background(), p); err == nil {
		t.Fatal("missing detector unexpectedly started")
	}
	assertSMSFixture(t, m, "", "none")
}

func TestSMSStopNeverCompletesAnOldPartialLine(t *testing.T) {
	m, path := smsTestManager(t, "old\n")
	beginSMSFixture(t, m)
	appendSMSFixture(t, path, "complete\npartial")
	m.endSMSSession()
	appendSMSFixture(t, path, " completed outside launch\n")
	assertSMSFixture(t, m, "complete\n", "stopped")
}

func TestSMSByteBudgetKeepsExactLineBoundary(t *testing.T) {
	for _, rotated := range []bool{false, true} {
		m, path := smsTestManager(t, "historical\n")
		beginSMSFixture(t, m)
		appendSMSFixture(t, path, "a\n")
		if rotated {
			if err := os.Rename(path, path+".1"); err != nil {
				t.Fatal(err)
			}
		}
		appendSMSFixture(t, path, "b\n")
		b, w, err := m.ReadCurrentSMS(2)
		if err != nil || string(b) != "b\n" || !w.Truncated || w.Bytes != 2 {
			t.Fatalf("rotated=%v: %q %+v %v", rotated, b, w, err)
		}
	}
}

func TestSMSRotationBetweenFileOpensResamplesBoundary(t *testing.T) {
	m, path := smsTestManager(t, "historical\n")
	beginSMSFixture(t, m)
	appendSMSFixture(t, path, "one\n")
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	appendSMSFixture(t, path, "two\n")
	oldBackup := filepath.Join(filepath.Dir(path), "old-backup.txt")
	if err := os.WriteFile(oldBackup, []byte("unrelated older history\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Deterministically replay the file handles observed if .1 was opened
	// before rotation, but the active file was opened after rotation.
	// 注入文件打开结果复现竞态，不依赖时序或操作系统删除已打开文件的语义。
	opens := 0
	open := func(name string) (*os.File, error) {
		opens++
		if opens == 1 {
			return os.Open(oldBackup)
		}
		return os.Open(name)
	}
	b, truncated, err := readSMSSpanUsing(path, m.smsSession.start, nil, 1024, open)
	if err != nil || string(b) != "one\ntwo\n" || truncated || opens != 4 {
		t.Fatalf("resample: %q truncated=%v opens=%d err=%v", b, truncated, opens, err)
	}
}

func TestSMSFailedStopKeepsActiveWindow(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("shell process-observation fixture is Linux-only")
	}
	m, path := smsTestManager(t, "old\n")
	beginSMSFixture(t, m)
	appendSMSFixture(t, path, "before\n")
	dir := t.TempDir()
	counter := shellQuote(filepath.Join(dir, "ps-count"))
	// Twelve initial observations contain no process, so no kill is issued.
	// The final stop verification observes a newly appeared service.
	// 前十二次无进程，不发送信号；停止完成检查时服务出现，模拟未停成功。
	writeShell(t, dir, "ps", "n=0\n[ ! -f "+counter+" ] || read n < "+counter+"\nn=$((n+1))\nprintf '%s\\n' \"$n\" > "+counter+"\nif [ \"$n\" -ge 13 ]; then printf '999999 S OpenBTS\\n'; fi\n")
	t.Setenv("PATH", dir)
	m.mu.Lock()
	_, stopped := m.stopLocked()
	m.mu.Unlock()
	if stopped {
		t.Fatal("fixture expected failed stop verification")
	}
	appendSMSFixture(t, path, "after failed stop\n")
	w := assertSMSFixture(t, m, "before\nafter failed stop\n", "running")
	if w.EndedAt != nil {
		t.Fatal("failed stop froze the active session")
	}
	writeShell(t, dir, "ps", "exit 0\n")
	m.mu.Lock()
	_, stopped = m.stopLocked()
	m.mu.Unlock()
	if !stopped {
		t.Fatal("empty fixture should confirm successful stop")
	}
	appendSMSFixture(t, path, "outside successful stop\n")
	assertSMSFixture(t, m, "before\nafter failed stop\n", "stopped")
}
