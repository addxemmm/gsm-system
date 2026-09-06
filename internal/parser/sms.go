// Package parser converts OpenBTS/smqueue outputs into structured data.
// Pure functions (no exec) so unit tests run on Windows without hardware.
package parser

import (
	"regexp"
	"strings"
)

// SMS is one decoded short message ("" = missing, JSON null in API layer).
type SMS struct {
	Time           string
	Text           string
	SenderNumber   string
	SenderIMSI     string
	ReceiverNumber string
	ReceiverIMSI   string
}

var (
	reTime   = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d)`)
	reText   = regexp.MustCompile(`Decoded text:\s*(.*)`)
	reFrom   = regexp.MustCompile(`(?m)^From:\s*(\S+)`)
	reTo     = regexp.MustCompile(`(?m)^To:\s*(\S+)`)
	reContact = regexp.MustCompile(`Contact:\s*<sip:IMSI(\d{15})`)
	reMsgTo   = regexp.MustCompile(`MESSAGE\s+sip:IMSI(\d{15})`)
	reFromIMSI = regexp.MustCompile(`From:\s*<sip:IMSI(\d{15})`)
	reToIMSI   = regexp.MustCompile(`To:\s*<sip:IMSI(\d{15})`)
	reCantSend = regexp.MustCompile(`Can't send your SMS to\s+(\S+)`)
)

// ParseSmqueueLog pairs `get_text: Decoded text` lines with the following
// `Deliver message:` SIP blocks. It tolerates color escapes and version drift
// (the legacy code used fixed line indexes; we use regex + prefix search).
func ParseSmqueueLog(log string) []SMS {
	texts := extractTexts(log)
	blocks := extractDeliverBlocks(log)
	n := len(texts)
	if len(blocks) < n {
		n = len(blocks)
	}
	var out []SMS
	for i := 0; i < n; i++ {
		s := SMS{Time: texts[i].time, Text: texts[i].text}
		parseDeliverInto(&s, blocks[i])
		out = append(out, s)
	}
	return out
}

type decodedLine struct {
	time, text string
}

func extractTexts(log string) []decodedLine {
	var out []decodedLine
	for _, line := range strings.Split(log, "\n") {
		if !strings.Contains(line, "get_text:") || !strings.Contains(line, "Decoded text") {
			continue
		}
		t := ""
		if m := reTime.FindStringSubmatch(line); m != nil {
			t = m[1]
		}
		txt := ""
		if m := reText.FindStringSubmatch(stripANSI(line)); m != nil {
			txt = strings.TrimSpace(m[1])
		}
		out = append(out, decodedLine{time: t, text: txt})
	}
	return out
}

func extractDeliverBlocks(log string) []string {
	clean := stripANSI(log)
	parts := strings.Split(clean, "Deliver message:")
	var out []string
	for _, p := range parts[1:] {
		lines := strings.Split(p, "\n")
		if len(lines) > 14 {
			lines = lines[:14]
		}
		out = append(out, strings.Join(lines, "\n"))
	}
	return out
}

func parseDeliverInto(s *SMS, block string) {
	// Receiver IMSI: MESSAGE sip:IMSI... (device-to-device path).
	if m := reMsgTo.FindStringSubmatch(block); m != nil {
		s.ReceiverIMSI = m[1]
	}
	// Sender IMSI: Contact: <sip:IMSI...> preferred, else From: IMSI.
	if m := reContact.FindStringSubmatch(block); m != nil {
		s.SenderIMSI = m[1]
	} else if m := reFromIMSI.FindStringSubmatch(block); m != nil {
		s.SenderIMSI = m[1]
	}
	// Numbers from From:/To: headers (may be IMSI-form -> treat as missing).
	if m := reFrom.FindStringSubmatch(block); m != nil {
		v := cleanNumber(m[1])
		if !strings.HasPrefix(v, "IMSI") && v != "" {
			s.SenderNumber = v
		}
	}
	if m := reTo.FindStringSubmatch(block); m != nil {
		v := cleanNumber(m[1])
		if !strings.HasPrefix(v, "IMSI") && v != "" {
			s.ReceiverNumber = v
		}
	}
	// Undeliverable path: "Can't send your SMS to <num>".
	if m := reCantSend.FindStringSubmatch(block); m != nil {
		v := strings.Trim(m[1], " ,.;\"'")
		if !strings.HasPrefix(v, "IMSI") {
			s.ReceiverNumber = v
		}
		s.ReceiverIMSI = ""
	}
}

func cleanNumber(raw string) string {
	// Forms: `10000001`, `<sip:10000001@...>`, `10000001 <sip:...>`.
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "<") {
		raw = strings.Trim(raw, "<>")
		if i := strings.Index(raw, ":"); i >= 0 {
			raw = raw[i+1:]
		}
		if i := strings.Index(raw, "@"); i >= 0 {
			raw = raw[:i]
		}
	} else {
		if f := strings.Fields(raw); len(f) > 0 {
			raw = f[0]
		}
		raw = strings.Trim(raw, "<>")
	}
	return strings.TrimSpace(raw)
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }
