package gsm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/addxemmm/gsm-system/internal/config"
)

type countingCloser struct{ calls atomic.Int32 }

func (c *countingCloser) Close() error {
	c.calls.Add(1)
	return nil
}

func TestManagedProcessHasOneWaitOwner(t *testing.T) {
	if os.Getenv("GSM_QUICK_HELPER") == "1" {
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestManagedProcessHasOneWaitOwner")
	cmd.Env = append(os.Environ(), "GSM_QUICK_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	closer := &countingCloser{}
	p := watchProcess("test", cmd, closer)
	select {
	case <-p.done:
	case <-time.After(3 * time.Second):
		t.Fatal("Wait owner did not publish process exit")
	}
	if p.alive() {
		t.Fatal("completed managed process still reports alive")
	}
	stopManaged(p, 100*time.Millisecond)
	stopManaged(p, 100*time.Millisecond)
	if got := closer.calls.Load(); got != 1 {
		t.Fatalf("closer called %d times, want exactly once", got)
	}
}

func TestWaitForReadyRespondsToContextAndProcessExit(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "openbts.log")
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForReady(ctx, logPath, done, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("want context cancellation, got %v", err)
	}

	exited := make(chan struct{})
	close(exited)
	if err := waitForReady(context.Background(), logPath, exited, time.Second); err == nil || !strings.Contains(err.Error(), "process exited") {
		t.Fatalf("want process-exited error, got %v", err)
	}

	if err := os.WriteFile(logPath, []byte("noise\nsystem ready\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := waitForReady(context.Background(), logPath, done, time.Second); err != nil {
		t.Fatalf("ready log rejected: %v", err)
	}
}

func TestConfigUpdateUsesOneGuardedTransactionAndConfiguredBinary(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("shell fixture is Linux-only")
	}
	dir := t.TempDir()
	capture := filepath.Join(dir, "sql.txt")
	argsCapture := filepath.Join(dir, "args.txt")
	sqlite := writeShell(t, dir, "sqlite-fixture", fmt.Sprintf(
		"printf '%%s\\n' \"$@\" > %s\ncat > %s\n", shellQuote(argsCapture), shellQuote(capture)))
	cfg := testConfig(t, dir, sqlite)
	m := New(cfg)
	if err := m.applyConfigLocked(validStartParams()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)
	for _, want := range []string{"PRAGMA busy_timeout=5000", "BEGIN IMMEDIATE", "COMMIT"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("transaction SQL missing %q:\n%s", want, sql)
		}
	}
	if got := strings.Count(sql, "UPDATE CONFIG"); got != 8 {
		t.Fatalf("got %d UPDATEs in transaction, want 8", got)
	}
	if got := strings.Count(sql, "INSERT INTO _gsm_config_guard VALUES(changes())"); got != 8 {
		t.Fatalf("got %d missing-key guards, want 8", got)
	}
	args, err := os.ReadFile(argsCapture)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "-bail") || !strings.Contains(string(args), cfg.OpenBTSDbPath) {
		t.Fatalf("configured sqlite invocation missing -bail or db path: %q", args)
	}
}

func TestConfigTransactionRollsBackWhenPresetKeyMissing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("real sqlite regression test is Linux-only")
	}
	sqlite, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 is not installed")
	}
	dir := t.TempDir()
	cfg := testConfig(t, dir, sqlite)
	var initSQL strings.Builder
	initSQL.WriteString("CREATE TABLE CONFIG(KEYSTRING TEXT PRIMARY KEY, VALUESTRING TEXT);\n")
	for _, key := range radioConfigKeys[:len(radioConfigKeys)-1] {
		fmt.Fprintf(&initSQL, "INSERT INTO CONFIG VALUES('%s','old');\n", sqliteEscape(key))
	}
	if out, err := exec.Command(sqlite, cfg.OpenBTSDbPath, initSQL.String()).CombinedOutput(); err != nil {
		t.Fatalf("create sqlite fixture: %v: %s", err, out)
	}
	m := New(cfg)
	if err := m.applyConfigLocked(validStartParams()); err == nil {
		t.Fatal("missing preset key silently succeeded")
	}
	out, err := exec.Command(sqlite, cfg.OpenBTSDbPath,
		"SELECT count(*) FROM CONFIG WHERE VALUESTRING='old';").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "7" {
		t.Fatalf("partial preset update escaped rollback; unchanged rows=%q", out)
	}
}

func TestSetSingleConfigErrorsAreClassified(t *testing.T) {
	cfg := config.Default()
	cfg.OpenBTSDbPath = filepath.Join(t.TempDir(), "missing.db")
	m := New(cfg)
	if err := m.SetSingleConfig("GSM.Radio.C0", "55"); !errors.Is(err, ErrDatabaseNotFound) {
		t.Fatalf("want ErrDatabaseNotFound, got %v", err)
	}

	if os.Getenv("GSM_LONG_HELPER") == "1" {
		time.Sleep(30 * time.Second)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestSetSingleConfigErrorsAreClassified")
	cmd.Env = append(os.Environ(), "GSM_LONG_HELPER=1")
	configureProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	m.openbts = watchProcess("OpenBTS", cmd, nil)
	t.Cleanup(func() { stopManaged(m.openbts, time.Second) })
	if err := m.SetSingleConfig("GSM.Radio.C0", "55"); !errors.Is(err, ErrCellRunning) {
		t.Fatalf("want ErrCellRunning, got %v", err)
	}
}

func TestStartKeepsSuccessfulChildrenAfterRequestCancellation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("process fixture is Linux-only")
	}
	dir := t.TempDir()
	sqlite := writeShell(t, dir, "sqlite-fixture", "cat >/dev/null\n")
	cfg := testConfig(t, dir, sqlite)
	cfg.UHDFindBin = writeShell(t, dir, "uhd-fixture", "echo 'type: B210'\n")
	service := writeShell(t, dir, "service-fixture", "exec sleep 30\n")
	cfg.SipAuthServeBin = service
	cfg.SmqueueBin = service
	cfg.AsteriskBin = service
	openPIDs := filepath.Join(dir, "openbts.pids")
	cfg.OpenBTSBin = writeShell(t, dir, "OpenBTS-fixture", fmt.Sprintf(
		"sleep 30 &\necho \"$$ $!\" > %s\necho 'system ready'\nwait\n", shellQuote(openPIDs)))
	cfg.OpenBTSCLIBin = writeShell(t, dir, "cli-fixture", "exit 0\n")
	m := New(cfg)
	t.Cleanup(func() { m.Stop() })
	ctx, cancel := context.WithCancel(context.Background())
	if err := m.Start(ctx, validStartParams()); err != nil {
		t.Fatal(err)
	}
	cancel()
	time.Sleep(150 * time.Millisecond)
	if !m.openbts.alive() || !m.IsRunning().OpenBTS {
		t.Fatal("successful OpenBTS remained bound to request context or reports stopped")
	}
	if !m.Stop() {
		t.Fatal("managed processes did not stop cleanly")
	}
	if m.openbts != nil {
		t.Fatal("Stop retained managed OpenBTS handle")
	}
	assertFixturePIDsStopped(t, openPIDs)
}

func TestStartPropagatesDependencyExitAndRollsBackOwnedChildren(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("process fixture is Linux-only")
	}
	dir := t.TempDir()
	sqlite := writeShell(t, dir, "sqlite-fixture", "cat >/dev/null\n")
	cfg := testConfig(t, dir, sqlite)
	cfg.UHDFindBin = writeShell(t, dir, "uhd-fixture", "echo B210\n")
	pidFile := filepath.Join(dir, "sipauth.pids")
	cfg.SipAuthServeBin = writeShell(t, dir, "sipauth-fixture",
		fmt.Sprintf("sleep 30 &\necho \"$$ $!\" > %s\nwait\n", shellQuote(pidFile)))
	cfg.SmqueueBin = writeShell(t, dir, "smqueue-fixture", "exit 12\n")
	cfg.AsteriskBin = writeShell(t, dir, "asterisk-fixture", "exec sleep 30\n")
	m := New(cfg)
	t.Cleanup(func() { m.Stop() })
	err := m.Start(context.Background(), validStartParams())
	if err == nil || !strings.Contains(err.Error(), "smqueue exited during startup") {
		t.Fatalf("dependency exit was not propagated: %v", err)
	}
	assertFixturePIDsStopped(t, pidFile)
}

func TestStopBoundsOpenBTSCLI(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("process fixture is Linux-only")
	}
	dir := t.TempDir()
	cfg := config.Default()
	cfg.OpenBTSCLIBin = writeShell(t, dir, "stuck-cli", "exec sleep 30\n")
	cell := writeShell(t, dir, "managed-cell", "exec sleep 30\n")
	p, err := startDetached("OpenBTS", cell)
	if err != nil {
		t.Fatal(err)
	}
	m := New(cfg)
	m.openbts = p
	t.Cleanup(func() { m.Stop() })
	started := time.Now()
	if !m.Stop() {
		t.Fatal("Stop did not stop managed cell")
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("stuck OpenBTSCLI made Stop unbounded: %s", elapsed)
	}
}

func TestStartPreCanceledHasNoSideEffects(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "unexpected")
	cfg := config.Default()
	cfg.DataDir = dir
	cfg.ConfDir = filepath.Join(dir, "conf")
	cfg.LogDir = filepath.Join(dir, "log")
	cfg.OpenBTSDbPath = filepath.Join(dir, "OpenBTS.db")
	cfg.UHDFindBin = filepath.Join(dir, "missing-uhd")
	cfg.Sqlite3Bin = marker
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := New(cfg).Start(ctx, validStartParams())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("pre-canceled start reached configured tools: %v", err)
	}
}

func testConfig(t *testing.T, dir, sqlite string) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = dir
	cfg.ConfDir = filepath.Join(dir, "conf")
	cfg.LogDir = filepath.Join(dir, "log")
	cfg.OpenBTSDbPath = filepath.Join(dir, "OpenBTS.db")
	cfg.Sqlite3Bin = sqlite
	if err := os.WriteFile(cfg.OpenBTSDbPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func validStartParams() StartParams {
	return StartParams{ARFCNs: "1", C0: "540", Band: "1800", MCC: "001", MNC: "01",
		LAC: "4420", CI: "41240", ShortName: "test", Network: "lo"}
}

func writeShell(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func assertFixturePIDsStopped(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, pid := range strings.Fields(string(b)) {
		out, _ := exec.Command("ps", "-o", "stat=", "-p", pid).Output()
		stat := strings.TrimSpace(string(out))
		if stat != "" && !strings.HasPrefix(stat, "Z") {
			t.Fatalf("owned process-group pid %s survived: stat=%q", pid, stat)
		}
	}
}
