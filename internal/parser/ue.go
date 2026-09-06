package parser

import (
	"strings"
)

// UE is one attached terminal: IMSI, IMEI, assigned number, SGSN IP.
type UE struct {
	IMSI   string
	IMEI   string
	Number string
	IP     string // "" = none/unassigned
}

// ParseSGSN parses `/OpenBTS/OpenBTSCLI -c sgsn list` lines into imsi->ip.
// Expected tokens include IMSI=... and ... IP=... ; we search by prefix
// instead of fixed indexes (legacy used [2][5:] / [9][4:], brittle).
func ParseSGSN(out string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var imsi, ip string
		for _, tok := range strings.Fields(line) {
			if strings.HasPrefix(tok, "IMSI=") {
				imsi = strings.TrimPrefix(tok, "IMSI=")
			} else if strings.HasPrefix(tok, "IP=") {
				ip = strings.TrimPrefix(tok, "IP=")
			} else if strings.HasPrefix(tok, "imsi=") {
				imsi = strings.TrimPrefix(tok, "imsi=")
			} else if strings.HasPrefix(tok, "ip=") {
				ip = strings.TrimPrefix(tok, "ip=")
			}
		}
		if imsi != "" {
			m[imsi] = ip
		}
	}
	return m
}

// ParseTMSIs parses `/OpenBTS/OpenBTSCLI -c tmsis -l` table rows.
// First line is a header and skipped. Columns (legacy indexes):
// 0 imsi, 2 imei, 10 number like `...=12345)` -> digits inside.
func ParseTMSIs(out string, sgsn map[string]string) []UE {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var ues []UE
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		imsi := f[0]
		if len(imsi) != 15 {
			continue
		}
		imei := ""
		if len(f) > 2 {
			imei = f[2]
		}
		number := ""
		if len(f) > 10 {
			number = digitsOnly(f[10])
		}
		ip := ""
		if sgsn != nil {
			ip = sgsn[imsi]
		}
		if ip == "none" {
			ip = ""
		}
		ues = append(ues, UE{IMSI: imsi, IMEI: imei, Number: number, IP: ip})
	}
	return ues
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
