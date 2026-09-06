// Package sysop provides process helpers without shell injection.
// Legacy used `ps -aux | grep OpenBTS` which matches itself and counts
// zombies; we use `ps -eo pid,stat,comm` exact match + Z exclusion.
// Copied from lte-system internal/sysop.
package sysop

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Running reports whether any process with the exact name is alive.
func Running(name string) bool {
	return len(PIDs(name)) > 0
}

// PIDs returns PIDs for an exact process name via `ps -eo pid,stat,comm`.
// Zombie (Z) entries are excluded.
func PIDs(name string) []int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-eo", "pid,stat,comm").Output()
	if err != nil {
		return nil
	}
	var res []int
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) != 3 {
			continue
		}
		if f[2] == name && !strings.Contains(f[1], "Z") {
			if pid, err := strconv.Atoi(f[0]); err == nil {
				res = append(res, pid)
			}
		}
	}
	return res
}

// AnyRunning reports true if any of the names is running.
func AnyRunning(names ...string) (string, bool) {
	for _, n := range names {
		if Running(n) {
			return n, true
		}
	}
	return "", false
}

// KillAll sends TERM then KILL to all PIDs of name. Returns killed count.
func KillAll(name string, timeout time.Duration) int {
	pids := PIDs(name)
	killed := 0
	for _, pid := range pids {
		if killPID(pid, timeout) {
			killed++
		}
	}
	return killed
}

func killPID(pid int, timeout time.Duration) bool {
	termCtx, termCancel := context.WithTimeout(context.Background(), timeout)
	_ = exec.CommandContext(termCtx, "kill", strconv.Itoa(pid)).Run()
	termCancel()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	// TERM's context has expired by this point. KILL must use a fresh context;
	// reusing termCtx makes CommandContext reject the command before it starts.
	killTimeout := timeout
	if killTimeout < time.Second {
		killTimeout = time.Second
	}
	killCtx, killCancel := context.WithTimeout(context.Background(), killTimeout)
	_ = exec.CommandContext(killCtx, "kill", "-9", strconv.Itoa(pid)).Run()
	killCancel()
	time.Sleep(300 * time.Millisecond)
	return !pidAlive(pid)
}

func pidAlive(pid int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	stat := strings.TrimSpace(string(out))
	return stat != "" && !strings.HasPrefix(stat, "Z")
}
