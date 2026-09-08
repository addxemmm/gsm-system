//go:build !linux

package gsm

import "os/exec"

func terminateProcessGroup(cmd *exec.Cmd) error { return killProcessGroup(cmd) }
func syncProfileDirectory(_ string) error       { return nil }

func configureProcessGroup(_ *exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
