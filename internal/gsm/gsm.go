// Package gsm owns the OpenBTS cell lifecycle + radio params.
// Intelligent rewrite of gsmsystem/run.py: no shell string concat,
// no `ps | grep` self-match, explicit validation, atomic profile.
package gsm

import (
	"fmt"
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

// Preset mirrors legacy /config id 0-4 (run.py values, the canonical set).
// [arfcns c0 band mcc mnc lac ci shortname]
var Presets = [][]string{
	{"1", "540", "1800", "001", "01", "4420", "41240", "test"},
	{"1", "55", "900", "460", "00", "4420", "41240", "ChinaMobile"},
	{"1", "540", "1800", "460", "00", "1", "0", "ChinaMobile"},
	{"1", "100", "900", "460", "01", "4420", "41240", "ChinaUnicom"},
	{"1", "540", "1800", "460", "01", "4420", "41240", "ChinaUnicom"},
}

// PresetParams expands a legacy config id into explicit params (network kept).
func PresetParams(id int, network string) (StartParams, error) {
	if id < 0 || id >= len(Presets) {
		return StartParams{}, fmt.Errorf("unknown config id %d (want 0-%d)", id, len(Presets)-1)
	}
	c := Presets[id]
	return StartParams{
		ARFCNs: c[0], C0: c[1], Band: c[2], MCC: c[3],
		MNC: c[4], LAC: c[5], CI: c[6], ShortName: c[7], Network: network,
	}, nil
}

// Validate checks required fields + strict formats. No silent fallback:
// unknown band / bad digits => error (caller maps to 422 / message_id 0).
func (p StartParams) Validate() error {
	if strings.TrimSpace(p.ARFCNs) == "" || strings.TrimSpace(p.C0) == "" ||
		strings.TrimSpace(p.Band) == "" || strings.TrimSpace(p.MCC) == "" ||
		strings.TrimSpace(p.MNC) == "" || strings.TrimSpace(p.LAC) == "" ||
		strings.TrimSpace(p.CI) == "" || strings.TrimSpace(p.ShortName) == "" ||
		strings.TrimSpace(p.Network) == "" {
		return fmt.Errorf("incomplete parameters")
	}
	if p.Band != "900" && p.Band != "1800" {
		return fmt.Errorf("band must be 900 or 1800")
	}
	for _, f := range []struct {
		name, val string
		exact    []int
		digits   bool
	}{
		{"arfcns", p.ARFCNs, nil, true},
		{"c0", p.C0, nil, true},
		{"mcc", p.MCC, []int{3}, true},
		{"mnc", p.MNC, []int{2, 3}, true},
		{"lac", p.LAC, nil, true},
		{"ci", p.CI, nil, true},
	} {
		if f.digits && !isDigits(f.val) {
			return fmt.Errorf("%s must be digits", f.name)
		}
		if len(f.exact) > 0 {
			ok := false
			for _, n := range f.exact {
				if len(f.val) == n {
					ok = true
				}
			}
			if !ok {
				return fmt.Errorf("%s has bad length", f.name)
			}
		}
	}
	if err := validateShortName(p.ShortName); err != nil {
		return err
	}
	if strings.ContainsAny(p.Network, " \t\n\r\"';&|<>$`\\") {
		return fmt.Errorf("network contains illegal characters")
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
	}
	if strings.TrimSpace(p.C0) == "" {
		add("c0", "required")
	} else if !isDigits(p.C0) {
		add("c0", "must be digits")
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
	}
	if strings.TrimSpace(p.CI) == "" {
		add("ci", "required")
	} else if !isDigits(p.CI) {
		add("ci", "must be digits")
	}
	if err := validateShortName(p.ShortName); err != nil {
		add("short_name", err.Error())
	}
	if strings.TrimSpace(p.Network) == "" {
		add("network", "required")
	} else if strings.ContainsAny(p.Network, " \t\n\r\"';&|<>$`\\") {
		add("network", "illegal characters")
	}
	return out
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
