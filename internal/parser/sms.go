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
	reTime     = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d)`)
	reText     = regexp.MustCompile(`Decoded text:\s*(.*)`)
	reFrom     = regexp.MustCompile(`(?m)^From:\s*(\S+)`)
	reTo       = regexp.MustCompile(`(?m)^To:\s*(\S+)`)
	reContact  = regexp.MustCompile(`Contact:\s*<sip:IMSI(\d{15})`)
	reMsgTo    = regexp.MustCompile(`MESSAGE\s+sip:IMSI(\d{15})`)
	reFromIMSI = regexp.MustCompile(`From:\s*<sip:IMSI(\d{15})`)
	reToIMSI   = regexp.MustCompile(`To:\s*<sip:IMSI(\d{15})`)
	reCantSend = regexp.MustCompile(`Can't send your SMS to\s+(\S+)`)
)

// ParseSmqueueLog only pairs text and addressing found in one delivery event.
// A decoded line is accepted when it follows the event's "Request Message
// Delivery" marker immediately before the SIP block. This intentionally drops
// incomplete events rather than joining independent text/block lists by index,
// which corrupts records when smqueue retries or log lines are missing.
func ParseSmqueueLog(log string) []SMS {
	clean := stripANSI(log)
	parts := strings.Split(clean, "Deliver message:")
	var out []SMS
	for i := 1; i < len(parts); i++ {
		decoded, ok := decodedForDelivery(parts[i-1])
		if !ok {
			continue
		}
		block := deliveryBlock(parts[i])
		s := SMS{Time: decoded.time, Text: decoded.text}
		parseDeliverInto(&s, block)
		out = append(out, s)
	}
	return out
}

type decodedLine struct {
	time, text string
}

func decodedForDelivery(prelude string) (decodedLine, bool) {
	marker := strings.LastIndex(prelude, "Request Message Delivery for ")
	if marker < 0 {
		return decodedLine{}, false
	}
	window := prelude[marker:]
	idx := strings.LastIndex(window, "Decoded text:")
	if idx < 0 {
		return decodedLine{}, false
	}
	line := window[idx:]
	if nl := strings.IndexByte(line, '\n'); nl >= 0 {
		line = line[:nl]
	}
	t := ""
	// Timestamp sits before "Decoded text", so read it from the full window.
	for _, candidate := range strings.Split(window, "\n") {
		if strings.Contains(candidate, "Decoded text:") {
			if m := reTime.FindStringSubmatch(candidate); m != nil {
				t = m[1]
			}
		}
	}
	m := reText.FindStringSubmatch(line)
	if m == nil {
		return decodedLine{}, false
	}
	return decodedLine{time: t, text: strings.TrimSpace(m[1])}, true
}

func deliveryBlock(afterMarker string) string {
	lines := strings.Split(afterMarker, "\n")
	if len(lines) > 18 {
		lines = lines[:18]
	}
	return strings.Join(lines, "\n")
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
