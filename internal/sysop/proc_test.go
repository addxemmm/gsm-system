package sysop

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestKillPIDFallsBackToFreshKILLContext(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux process-signal regression test")
	}
	if os.Getenv("SYSOP_IGNORE_TERM_HELPER") == "1" {
		signal.Ignore(syscall.SIGTERM)
		_, _ = os.Stdout.WriteString("READY\n")
		closeAfter := time.NewTimer(30 * time.Second)
		defer closeAfter.Stop()
		<-closeAfter.C
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestKillPIDFallsBackToFreshKILLContext")
	cmd.Env = append(os.Environ(), "SYSOP_IGNORE_TERM_HELPER=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || ready != "READY\n" {
		t.Fatalf("helper readiness handshake: ready=%q err=%v", ready, err)
	}
	if !killPID(cmd.Process.Pid, 100*time.Millisecond) {
		t.Fatal("TERM-ignoring process was not killed by the KILL fallback")
	}
	select {
	case waitErr := <-done:
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			t.Fatalf("want signal exit, got %v", waitErr)
		}
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if !ok || status.Signal() != syscall.SIGKILL {
			t.Fatalf("want SIGKILL exit, got %v", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("helper process was not reaped")
	}
}

func TestPIDAliveExcludesZombie(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux zombie regression test")
	}
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Wait() }()
	time.Sleep(100 * time.Millisecond)
	if pidAlive(cmd.Process.Pid) {
		t.Fatal("zombie child reported alive")
	}
}
