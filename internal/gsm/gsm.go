// Package gsm owns the OpenBTS cell lifecycle + radio params.
// Go control plane: no shell string concatenation,
// no `ps | grep` self-match, explicit validation, atomic profile.
package gsm

import (
	"fmt"
	"strconv"
	"strings"
)

// StartParams mirrors the new POST /api/v1/cell (explicit GSM radio params).
// Empty object {} reuses the saved profile (last_start.json), like lte-system.
type StartParams struct {
	ARFCNs    string `json:"arfcns"`
	C0        string `json:"c0"`
	Band      string `json:"band"` // "900" or "1800" only
	MCC       string `json:"mcc"`  // 3 digits
	MNC       string `json:"mnc"`  // 2-3 digits
	LAC       string `json:"lac"`
	CI        string `json:"ci"`
	ShortName string `json:"short_name"`
	Network   string `json:"network"` // uplink iface for iptables MASQUERADE
}

// Validate checks required fields + strict formats. No silent fallback:
// unknown band / bad digits => error (caller maps to 422 / message_id 0).
func (p StartParams) Validate() error {
	if issues := p.ValidateDetailed(); len(issues) > 0 {
		return fmt.Errorf("%s: %s", issues[0].Field, issues[0].Reason)
	}
	return nil
}

// FieldIssue is one rejected field for 422 responses.
type FieldIssue struct {
	Field  string
	Reason string
}

// ValidateDetailed returns per-field issues (for v1 422 data.errors).
func (p StartParams) ValidateDetailed() []FieldIssue {
	var out []FieldIssue
	add := func(f, r string) { out = append(out, FieldIssue{Field: f, Reason: r}) }
	if strings.TrimSpace(p.ARFCNs) == "" {
		add("arfcns", "required")
	} else if !isDigits(p.ARFCNs) {
		add("arfcns", "must be digits")
	} else if p.ARFCNs != "1" {
		add("arfcns", "this UHD transceiver supports one carrier only")
	}
	if strings.TrimSpace(p.C0) == "" {
		add("c0", "required")
	} else if !isDigits(p.C0) {
		add("c0", "must be digits")
	} else {
		n, err := strconv.Atoi(p.C0)
		if err != nil || (p.Band == "900" && !((n >= 0 && n <= 124) || (n >= 975 && n <= 1023))) ||
			(p.Band == "1800" && !(n >= 512 && n <= 885)) {
			add("c0", "outside the selected GSM band channel range")
		}
	}
	if p.Band != "900" && p.Band != "1800" {
		add("band", "must be 900 or 1800")
	}
	if len(p.MCC) != 3 || !isDigits(p.MCC) {
		add("mcc", "must be 3 digits")
	}
	if !(len(p.MNC) == 2 || len(p.MNC) == 3) || !isDigits(p.MNC) {
		add("mnc", "must be 2 or 3 digits")
	}
	if strings.TrimSpace(p.LAC) == "" {
		add("lac", "required")
	} else if !isDigits(p.LAC) {
		add("lac", "must be digits")
	} else if n, err := strconv.Atoi(p.LAC); err != nil || n < 1 || n > 65279 {
		add("lac", "must be 1-65279 for this OpenBTS compatibility profile")
	}
	if strings.TrimSpace(p.CI) == "" {
		add("ci", "required")
	} else if !isDigits(p.CI) {
		add("ci", "must be digits")
	} else if n, err := strconv.Atoi(p.CI); err != nil || n < 0 || n > 65535 {
		add("ci", "must be 0-65535")
	}
	if err := validateShortName(p.ShortName); err != nil {
		add("short_name", err.Error())
	}
	if strings.TrimSpace(p.Network) == "" {
		add("network", "required")
	} else if !ValidInterfaceName(p.Network) {
		add("network", "illegal characters")
	}
	return out
}

// ValidInterfaceName accepts Linux interface names without CLI metacharacters.
func ValidInterfaceName(name string) bool {
	if len(name) < 1 || len(name) > 15 || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}

func validateShortName(s string) error {
	if s == "" {
		return fmt.Errorf("required")
	}
	if len(s) > 32 {
		return fmt.Errorf("must be 1-32 chars")
	}
	for _, r := range s {
		if r < 0x20 || r > 0x7e || strings.ContainsRune("\"';#$`\\", r) {
			return fmt.Errorf("illegal character %q", r)
		}
	}
	return nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ValidateIMSI validates a 15-digit IMSI (subscriber APIs).
func ValidateIMSI(imsi string) error {
	if len(imsi) != 15 || !isDigits(imsi) {
		return fmt.Errorf("imsi must be 15 digits")
	}
	return nil
}

// ValidateNumber validates a 2-15 digit phone number.
func ValidateNumber(n string) error {
	if len(n) < 2 || len(n) > 15 || !isDigits(n) {
		return fmt.Errorf("number must be 2-15 digits")
	}
	return nil
}
