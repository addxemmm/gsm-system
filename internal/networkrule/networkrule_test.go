package networkrule

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestCommandHelper(t *testing.T) {
	if os.Getenv("GSM_NETWORKRULE_HELPER") != "1" {
		return
	}
	code, _ := strconv.Atoi(os.Getenv("GSM_NETWORKRULE_EXIT"))
	if output := os.Getenv("GSM_NETWORKRULE_OUTPUT"); output != "" {
		_, _ = os.Stderr.WriteString(output)
	}
	os.Exit(code)
}

func installCommandFixture(t *testing.T, exits []int, output string) *[][]string {
	t.Helper()
	original := commandContext
	calls := make([][]string, 0, len(exits))
	commandContext = func(ctx context.Context, bin string, args ...string) *exec.Cmd {
		call := append([]string{bin}, args...)
		calls = append(calls, call)
		index := len(calls) - 1
		if index >= len(exits) {
			t.Fatalf("unexpected command call %d: %v", index+1, call)
		}
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCommandHelper$")
		cmd.Env = append(os.Environ(),
			"GSM_NETWORKRULE_HELPER=1",
			"GSM_NETWORKRULE_EXIT="+strconv.Itoa(exits[index]),
			"GSM_NETWORKRULE_OUTPUT="+output,
		)
		return cmd
	}
	t.Cleanup(func() { commandContext = original })
	return &calls
}

func TestEnsureAddsMissingRuleWithExactScope(t *testing.T) {
	calls := installCommandFixture(t, []int{1, 0}, "")
	changed, err := Ensure(context.Background(), "iptables-fixture", "eth0")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	want := [][]string{
		{"iptables-fixture", "-t", "nat", "-C", "POSTROUTING", "-s", "192.168.99.0/24", "-o", "eth0", "-j", "MASQUERADE"},
		{"iptables-fixture", "-t", "nat", "-A", "POSTROUTING", "-s", "192.168.99.0/24", "-o", "eth0", "-j", "MASQUERADE"},
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("commands=%v, want %v", *calls, want)
	}
}

func TestEnsureDoesNotDuplicateExistingRule(t *testing.T) {
	calls := installCommandFixture(t, []int{0}, "")
	changed, err := Ensure(context.Background(), "iptables-fixture", "eth0")
	if err != nil || changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if got := len(*calls); got != 1 {
		t.Fatalf("command calls=%d, want check only", got)
	}
}

func TestEnsureReportsIptablesFailure(t *testing.T) {
	calls := installCommandFixture(t, []int{1, 42}, "fixture denied")
	changed, err := Ensure(context.Background(), "iptables-fixture", "eth0")
	if changed || err == nil || !strings.Contains(err.Error(), "iptables add") || !strings.Contains(err.Error(), "fixture denied") {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if got := len(*calls); got != 2 {
		t.Fatalf("command calls=%d, want check plus add", got)
	}
}
