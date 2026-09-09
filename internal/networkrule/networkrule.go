// Package networkrule owns the single project-managed GSM data NAT rule.
// Commands are executed directly with argv and never through a shell.
package networkrule

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const managedDataCIDR = "192.168.99.0/24"

var commandContext = exec.CommandContext

// Present reports whether the exact project-managed MASQUERADE rule exists.
func Present(ctx context.Context, bin, iface string) (bool, error) {
	if bin == "" {
		bin = "iptables"
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := commandContext(cmdCtx, bin, "-t", "nat", "-C", "POSTROUTING",
		"-s", managedDataCIDR, "-o", iface, "-j", "MASQUERADE").CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("iptables check: %w: %s", err, strings.TrimSpace(string(out)))
}

// Ensure adds the exact project-managed rule when it is missing. The returned
// bool reports whether this call changed the NAT table.
func Ensure(ctx context.Context, bin, iface string) (bool, error) {
	present, err := Present(ctx, bin, iface)
	if err != nil || present {
		return false, err
	}
	if bin == "" {
		bin = "iptables"
	}
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := commandContext(cmdCtx, bin, "-t", "nat", "-A", "POSTROUTING",
		"-s", managedDataCIDR, "-o", iface, "-j", "MASQUERADE").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("iptables add: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return true, nil
}
