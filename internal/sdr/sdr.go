// Package sdr detects attached SDR hardware without shell injection.
package sdr

import (
	"context"
	"strings"
	"time"

	"os/exec"
)

// Detection is the machine-readable SDR presence report.
type Detection struct {
	UHD_B210 bool   `json:"uhd_b210"`
	Raw      string `json:"uhd_raw,omitempty"`
}

// Detect runs uhd_find_devices and reports B210 presence.
// Missing binary or no device => UHD_B210 false (not an error).
func Detect() Detection {
	return DetectWith("uhd_find_devices")
}

// DetectWith allows tests to inject a fake binary name.
func DetectWith(bin string) Detection {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin).CombinedOutput()
	if err != nil && len(out) == 0 {
		return Detection{}
	}
	s := string(out)
	return Detection{UHD_B210: strings.Contains(s, "B210"), Raw: firstLines(s, 5)}
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
