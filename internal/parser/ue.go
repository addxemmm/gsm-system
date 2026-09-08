package parser

import (
	"net"
	"strconv"
	"strings"
)

// UE is one observed TMSI record. A record may be rejected or stale and does
// not, by itself, mean that a terminal is currently attached.
type UE struct {
	IMSI   string
	IMEI   string
	Number string
	IP     string // "" = none/unassigned
	// Auth and RejectCode are raw TMSI-table diagnostics. nil means that the
	// column was absent or could not be parsed; neither value implies liveness.
	Auth       *int
	RejectCode *int
}

// ParseSGSN parses `/OpenBTS/OpenBTSCLI -c sgsn list` lines into imsi->ip.
// Real output uses imsi=... and IPs=... (plural); older builds may use IP=.
// We search labels instead of fixed token indexes.
func ParseSGSN(out string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var imsi, ip string
		for _, tok := range strings.Fields(line) {
			key, value, ok := strings.Cut(tok, "=")
			if !ok {
				continue
			}
			switch strings.ToLower(key) {
			case "imsi":
				imsi = strings.Trim(value, " ,;()[]<>")
			case "ip", "ips":
				ip = strings.Trim(value, " ,;()[]<>")
			}
		}
		if imsi != "" {
			ip = firstValidIP(ip)
			// An IMSI can occur in more than one diagnostic context. Preserve an
			// assigned address rather than replacing it with a later `IPs=none`.
			if _, exists := m[imsi]; !exists || ip != "" {
				m[imsi] = ip
			}
		}
	}
	return m
}

func firstValidIP(value string) string {
	if strings.EqualFold(value, "none") {
		return ""
	}
	for _, candidate := range strings.Split(value, ",") {
		candidate = strings.TrimSpace(candidate)
		if parsed := net.ParseIP(candidate); parsed != nil {
			return parsed.String()
		}
	}
	return ""
}

// ParseTMSIs parses `/OpenBTS/OpenBTSCLI -c tmsis -l` table rows.
// The output is a fixed-width table. Parsing by strings.Fields is unsafe because
// empty ASSOCIATED_URI/ASSERTED_IDENTITY cells collapse, shifting WELCOME_SENT
// into the apparent URI position.
func ParseTMSIs(out string, sgsn map[string]string) []UE {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	headerIndex := -1
	var columns map[string]columnBounds
	for i, line := range lines {
		if c, ok := tmsiColumns(strings.TrimSuffix(line, "\r")); ok {
			headerIndex, columns = i, c
			break
		}
	}
	var ues []UE
	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if i <= headerIndex || strings.TrimSpace(line) == "" {
			continue
		}
		var imsi, imei string
		var auth, rejectCode *int
		if headerIndex >= 0 {
			imsi = fixedColumn(line, columns["IMSI"])
			imei = fixedColumn(line, columns["IMEI"])
			auth = optionalInt(fixedColumn(line, columns["AUTH"]))
			rejectCode = optionalInt(fixedColumn(line, columns["REJECT_CODE"]))
		} else {
			// Compatibility fallback for old/minimal output without a recognizable
			// header. Only the non-optional leading columns are positional.
			f := strings.Fields(line)
			if len(f) < 3 {
				continue
			}
			imsi, imei = f[0], f[2]
		}
		if !decimalWithLength(imsi, 15) {
			continue
		}
		if !decimalWithLength(imei, 15) {
			imei = ""
		}
		number := ""
		if headerIndex >= 0 {
			number = phoneIdentity(fixedColumn(line, columns["ASSOCIATED_URI"]))
			if number == "" {
				number = phoneIdentity(fixedColumn(line, columns["ASSERTED_IDENTITY"]))
			}
		} else {
			number = phoneIdentity(line)
		}
		ip := ""
		if sgsn != nil {
			ip = sgsn[imsi]
		}
		if strings.EqualFold(ip, "none") {
			ip = ""
		}
		ues = append(ues, UE{
			IMSI: imsi, IMEI: imei, Number: number, IP: ip,
			Auth: auth, RejectCode: rejectCode,
		})
	}
	return ues
}

type columnBounds struct{ start, end int }

func tmsiColumns(header string) (map[string]columnBounds, bool) {
	names := []string{
		"IMSI", "TMSI", "IMEI", "AUTH", "CREATED", "ACCESSED",
		"TMSI_ASSIGNED", "PTMSI_ASSIGNED", "AUTH_EXPIRY", "REJECT_CODE",
		"ASSOCIATED_URI", "ASSERTED_IDENTITY", "WELCOME_SENT",
	}
	starts := make([]int, len(names))
	searchFrom := 0
	for i, name := range names {
		relative := strings.Index(header[searchFrom:], name)
		if relative < 0 {
			return nil, false
		}
		starts[i] = searchFrom + relative
		searchFrom = starts[i] + len(name)
	}
	columns := make(map[string]columnBounds, len(names))
	for i, name := range names {
		end := -1
		if i+1 < len(names) {
			end = starts[i+1]
		}
		columns[name] = columnBounds{start: starts[i], end: end}
	}
	return columns, true
}

func fixedColumn(line string, bounds columnBounds) string {
	if bounds.start < 0 || bounds.start >= len(line) {
		return ""
	}
	end := bounds.end
	if end < 0 || end > len(line) {
		end = len(line)
	}
	if end < bounds.start {
		return ""
	}
	return strings.TrimSpace(line[bounds.start:end])
}

func optionalInt(s string) *int {
	value, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &value
}

func decimalWithLength(s string, length int) bool {
	if len(s) != length {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func phoneIdentity(s string) string {
	lower := strings.ToLower(s)
	if start := strings.Index(lower, "tel:"); start >= 0 {
		return leadingDigits(s[start+len("tel:"):])
	}
	if start := strings.Index(lower, "sip:"); start >= 0 {
		rest := s[start+len("sip:"):]
		at := strings.IndexByte(rest, '@')
		if at < 0 {
			return ""
		}
		user := strings.TrimPrefix(rest[:at], "+")
		// A SIP user is only unambiguously a phone number when marked as such
		// or written in global-number form. Do not mistake a numeric IMSI user
		// for a subscriber number.
		if strings.HasPrefix(rest[:at], "+") || strings.Contains(strings.ToLower(rest[at:]), "user=phone") {
			if decimalWithLength(user, len(user)) && user != "" {
				return user
			}
		}
	}
	return ""
}

func leadingDigits(s string) string {
	s = strings.TrimPrefix(s, "+")
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	return s[:end]
}
