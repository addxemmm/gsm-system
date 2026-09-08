package gsm

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/addxemmm/gsm-system/internal/logsink"
)

func TestProcessLogIgnoresOldReadinessAndPreservesHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openbts.log")
	const previous = "old session\nsystem ready\n"
	if err := os.WriteFile(path, []byte(previous), 0600); err != nil {
		t.Fatal(err)
	}
	output, err := logsink.NewWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	w := newProcessLog(output)
	defer w.Close()
	done := make(chan struct{})
	if err := waitForReady(context.Background(), w.ready, done, 10*time.Millisecond); err == nil {
		t.Fatal("historical READY was accepted")
	}
	for _, data := range []string{"new session\nsystem re", "ady\n"} {
		if _, err := w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := waitForReady(context.Background(), w.ready, done, time.Second); err != nil {
		t.Fatalf("current split READY was not accepted: %v", err)
	}
	// Repeated readiness markers must not close the ready channel twice.
	if _, err := w.Write([]byte("system ready\n")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(data), previous+"new session\n") {
		t.Fatalf("prior session evidence lost: %q err=%v", data, err)
	}
	close(done)
	if err := waitForReady(context.Background(), w.ready, done, time.Second); err == nil {
		t.Fatal("exited process accepted despite ready marker")
	}
}

func TestProcessLogWaitIsBoundedWhenDescendantKeepsPipe(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("OpenBTS child-pipe lifecycle fixture is Linux-only")
	}
	switch os.Getenv("GSM_LOG_PIPE_HELPER") {
	case "descendant":
		// Keep the pipe open well beyond the assertion deadline. The test
		// cleanup kills this process group instead of leaving a sleeping helper.
		time.Sleep(10 * time.Second)
		os.Exit(0)
	case "parent":
		child := exec.Command(os.Args[0], "-test.run=^TestProcessLogWaitIsBoundedWhenDescendantKeepsPipe$")
		child.Env = append(os.Environ(), "GSM_LOG_PIPE_HELPER=descendant")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(9)
		}
		os.Exit(0)
	}
	output, err := logsink.NewWriter(filepath.Join(t.TempDir(), "openbts.log"))
	if err != nil {
		t.Fatal(err)
	}
	w := newProcessLog(output)
	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessLogWaitIsBoundedWhenDescendantKeepsPipe$")
	cmd.Env = append(os.Environ(), "GSM_LOG_PIPE_HELPER=parent")
	cmd.Stdout, cmd.Stderr = w, w
	cmd.WaitDelay = 20 * time.Millisecond
	configureProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		w.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = killProcessGroup(cmd) })
	p := watchProcess("fixture-pipe-parent", cmd, w)
	select {
	case <-p.done:
		if cmd.ProcessState.ExitCode() != 0 {
			t.Fatalf("fixture parent failed: %v", cmd.ProcessState)
		}
	// Race-instrumented helpers normally delay exit by one second. Allow that
	// delay plus scheduling headroom, but less than the descendant's lifetime.
	// race 子进程默认延迟一秒退出；留足调度余量，仍短于后代持管道时间。
	case <-time.After(5 * time.Second):
		t.Fatal("inherited output pipe prevented observing parent exit")
	}
}
