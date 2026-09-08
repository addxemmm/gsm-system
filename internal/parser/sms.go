// Package parser converts OpenBTS/smqueue outputs into structured data.
// Pure functions (no exec) so unit tests run on Windows without hardware.
package parser

import (
	"encoding/hex"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
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
	reTime       = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?)`)
	reNativeLine = regexp.MustCompile(`^(?:<\d+>.*?\bsmqueue:\s+)?(?:ALERT|CRIT|ERR|WARNING|NOTICE|INFO|DEBUG)\s+\d+:\d+\s+` + reTime.String() + `\s+([^:\s]+)(?::\d+)?:([^:\s]+):\s?(.*)$`)
	reText       = regexp.MustCompile(`^Decoded text:\s*(.*)$`)
	reKeyedText  = regexp.MustCompile(`^Decoded text\s+qtag(?:=|\s+)'?([^':\s]+)'?\s*:\s*(.*)$`)
	reStructured = regexp.MustCompile(`^GSM_SMS_V1 qtag_hex=(\S*) from_hex=(\S*) to_hex=(\S*) text_hex=(\S*)\s*$`)
	reGotSMS     = regexp.MustCompile(`^Got SMS rqst qtag '([^'\r\n]+)' from (\S+) for (\S+)\s*$`)
	reDelivery   = regexp.MustCompile(`^Request Message Delivery for\s+(\S+)\s*$`)
	reFrom       = regexp.MustCompile(`(?m)^From:\s*(\S+)`)
	reTo         = regexp.MustCompile(`(?m)^To:\s*(\S+)`)
	reContact    = regexp.MustCompile(`Contact:\s*<sip:IMSI(\d{15})`)
	reMsgTo      = regexp.MustCompile(`MESSAGE\s+sip:IMSI(\d{15})`)
	reFromIMSI   = regexp.MustCompile(`From:\s*<sip:IMSI(\d{15})`)
	reToIMSI     = regexp.MustCompile(`To:\s*<sip:IMSI(\d{15})`)
	reCantSend   = regexp.MustCompile(`Can't send your SMS to\s+(\S+)`)
	reIMSI       = regexp.MustCompile(`^IMSI(\d{15})$`)
	reNumber     = regexp.MustCompile(`^\+?[0-9]{2,15}$`)
	reCSeq       = regexp.MustCompile(`(?m)^CSeq:\s*(\d+)\s+MESSAGE\s*$`)
	reFromTag    = regexp.MustCompile(`(?mi)^From:.*;tag=([^;>\s]+)`)
)

// ParseSmqueueLog reads both the production NOTICE receive events and legacy
// INFO/DEBUG delivery blocks. Events are deduplicated by smqueue qtag within a
// process generation, never by message text. Unkeyed production Decoded-text
// lines are never paired because concurrent requests make that association
// unsafe; keyed native records are authoritative.
func ParseSmqueueLog(log string) []SMS {
	clean := strings.ReplaceAll(stripANSI(log), "\r\n", "\n")
	restarts := restartPositions(clean)
	candidates := parseNoticeEvents(clean, restarts)
	candidates = append(candidates, parseDeliveryEvents(clean, restarts)...)
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].position < candidates[j].position })

	out := make([]SMS, 0, len(candidates))
	structured := make([]bool, 0, len(candidates))
	byIdentity := make(map[string]int, len(candidates))
	for _, candidate := range candidates {
		if index, ok := byIdentity[candidate.identity]; ok {
			if candidate.structured {
				if !structured[index] {
					applyStructuredSMS(&out[index], candidate.sms)
					structured[index] = true
				}
			} else if !structured[index] {
				mergeSMS(&out[index], candidate.sms)
			}
			continue
		}
		byIdentity[candidate.identity] = len(out)
		out = append(out, candidate.sms)
		structured = append(structured, candidate.structured)
	}
	return out
}

type decodedLine struct {
	qtag, time, text string
}

type smsCandidate struct {
	identity   string
	position   int
	sms        SMS
	structured bool
}

type nativeMessage struct {
	time, source, function, text string
}

func parseNativeMessage(line string) (nativeMessage, bool) {
	match := reNativeLine.FindStringSubmatch(line)
	if match == nil {
		return nativeMessage{}, false
	}
	return nativeMessage{
		time:     match[1],
		source:   match[2],
		function: match[3],
		text:     match[4],
	}, true
}

func parseNoticeEvents(log string, restarts []int) []smsCandidate {
	var candidates []smsCandidate
	position := 0
	for _, lineWithNL := range strings.SplitAfter(log, "\n") {
		line := strings.TrimSuffix(lineWithNL, "\n")
		if isRestartLine(line) {
			position += len(lineWithNL)
			continue
		}
		native, ok := parseNativeMessage(line)
		if !ok {
			position += len(lineWithNL)
			continue
		}
		generation := generationAt(restarts, position)
		if native.source == "smqueue.cpp" && native.function == "main_loop" {
			match := reGotSMS.FindStringSubmatch(native.text)
			if match == nil {
				position += len(lineWithNL)
				continue
			}
			identity := scopedIdentity(generation, match[1])
			sms := SMS{Time: native.time}
			parseNoticeParty(&sms, match[2], true)
			parseNoticeParty(&sms, strings.TrimRight(match[3], ".,;"), false)
			candidates = append(candidates, smsCandidate{identity: identity, position: position, sms: sms})
			position += len(lineWithNL)
			continue
		}

		if native.source == "smsc.cpp" && native.function == "submitSMS" {
			structured, valid := parseStructuredSMS(native.text, native.time)
			if !valid {
				position += len(lineWithNL)
				continue
			}
			identity := scopedIdentity(generation, structured.qtag)
			candidates = append(candidates, smsCandidate{
				identity:   identity,
				position:   position,
				sms:        structured.sms,
				structured: true,
			})
		} else if native.source == "smqueue.h" && native.function == "get_text" {
			keyed := reKeyedText.FindStringSubmatch(native.text)
			if keyed == nil {
				// Unkeyed decoded text has no safe association under concurrent
				// processing. Keep any Got event, but leave its text missing.
				position += len(lineWithNL)
				continue
			}
			identity := scopedIdentity(generation, keyed[1])
			candidates = append(candidates, smsCandidate{
				identity: identity,
				position: position,
				sms:      SMS{Time: native.time, Text: strings.TrimSpace(keyed[2])},
			})
		}
		position += len(lineWithNL)
	}
	return candidates
}

type structuredSMS struct {
	qtag string
	sms  SMS
}

func parseStructuredSMS(message, eventTime string) (structuredSMS, bool) {
	match := reStructured.FindStringSubmatch(message)
	if match == nil {
		return structuredSMS{}, false
	}
	limits := []int{512, 512, 512, 8192} // Native byte caps multiplied by two for hex.
	values := make([]string, 4)
	for index, encoded := range match[1:] {
		if len(encoded) > limits[index] {
			return structuredSMS{}, false
		}
		decoded, err := hex.DecodeString(encoded)
		if err != nil || !utf8.Valid(decoded) {
			return structuredSMS{}, false
		}
		values[index] = string(decoded)
	}
	if values[0] == "" {
		return structuredSMS{}, false
	}
	sms := SMS{Time: eventTime, Text: values[3]}
	parseNoticeParty(&sms, values[1], true)
	parseNoticeParty(&sms, values[2], false)
	return structuredSMS{qtag: values[0], sms: sms}, true
}

type positionedLine struct {
	start, end int
	text       string
}

func logLines(log string) []positionedLine {
	lines := make([]positionedLine, 0, strings.Count(log, "\n")+1)
	position := 0
	for _, withNL := range strings.SplitAfter(log, "\n") {
		if withNL == "" {
			continue
		}
		lines = append(lines, positionedLine{
			start: position,
			end:   position + len(withNL),
			text:  strings.TrimSuffix(withNL, "\n"),
		})
		position += len(withNL)
	}
	return lines
}

func isDeliveryMarker(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "Deliver message:" || trimmed == "--Deliver message:" {
		return true
	}
	native, ok := parseNativeMessage(line)
	return ok && native.source == "smnet.cpp" && native.function == "deliver_msg_datagram" &&
		(native.text == "Deliver message:" || native.text == "--Deliver message:")
}

func parseDeliveryEvents(log string, restarts []int) []smsCandidate {
	lines := logLines(log)
	var candidates []smsCandidate
	previousMarkerEnd := 0
	for index, line := range lines {
		if !isDeliveryMarker(line.text) {
			continue
		}
		blockEnd := index + 19
		if blockEnd > len(lines) {
			blockEnd = len(lines)
		}
		var blockBuilder strings.Builder
		for _, blockLine := range lines[index+1 : blockEnd] {
			blockBuilder.WriteString(blockLine.text)
			blockBuilder.WriteByte('\n')
		}
		block := blockBuilder.String()
		decoded, ok := decodedForDelivery(log[previousMarkerEnd:line.start], block)
		if ok {
			sms := SMS{Time: decoded.time, Text: decoded.text}
			parseDeliverInto(&sms, block)
			candidates = append(candidates, smsCandidate{
				identity: scopedIdentity(generationAt(restarts, line.start), decoded.qtag),
				position: line.start,
				sms:      sms,
			})
		}
		previousMarkerEnd = line.end
	}
	return candidates
}

type legacyPreludeEvent struct {
	kind, qtag, time, text string
}

func legacyPreludeEvents(prelude string) []legacyPreludeEvent {
	var events []legacyPreludeEvent
	for _, line := range logLines(prelude) {
		native, ok := parseNativeMessage(line.text)
		if !ok {
			continue
		}
		switch {
		case native.source == "smqueue.cpp" && native.function == "process_timeout":
			if match := reDelivery.FindStringSubmatch(native.text); match != nil {
				events = append(events, legacyPreludeEvent{kind: "request", qtag: strings.Trim(match[1], "'\""), time: native.time})
			}
		case native.source == "smqueue.cpp" && native.function == "main_loop" && reGotSMS.MatchString(native.text):
			events = append(events, legacyPreludeEvent{kind: "boundary"})
		case isRestartLine(line.text):
			events = append(events, legacyPreludeEvent{kind: "boundary"})
		case native.source == "smqueue.h" && native.function == "get_text":
			if match := reText.FindStringSubmatch(native.text); match != nil {
				events = append(events, legacyPreludeEvent{kind: "decoded", time: native.time, text: strings.TrimSpace(match[1])})
			}
		}
	}
	return events
}

func qtagFromDeliveryBlock(block string) string {
	cseq := reCSeq.FindStringSubmatch(block)
	fromTag := reFromTag.FindStringSubmatch(block)
	if cseq == nil || fromTag == nil {
		return ""
	}
	return cseq[1] + "--" + fromTag[1]
}

func decodedForDelivery(prelude, block string) (decodedLine, bool) {
	events := legacyPreludeEvents(prelude)
	var requestIndexes []int
	for index, event := range events {
		if event.kind == "request" {
			requestIndexes = append(requestIndexes, index)
		}
	}
	if len(requestIndexes) == 0 {
		return decodedLine{}, false
	}
	requestIndex := -1
	if len(requestIndexes) == 1 {
		requestIndex = requestIndexes[0]
	} else if blockQTag := qtagFromDeliveryBlock(block); blockQTag != "" {
		for _, index := range requestIndexes {
			if events[index].qtag == blockQTag {
				if requestIndex >= 0 {
					return decodedLine{}, false
				}
				requestIndex = index
			}
		}
	}
	if requestIndex < 0 {
		return decodedLine{}, false
	}

	request := events[requestIndex]
	result := decodedLine{qtag: request.qtag, time: request.time}
	decodedCount := 0
	for _, event := range events[requestIndex+1:] {
		if event.kind == "request" || event.kind == "boundary" {
			// A different request/restart destroys the local association.
			return result, true
		}
		if event.kind == "decoded" {
			decodedCount++
			result.time = event.time
			result.text = event.text
		}
	}
	if decodedCount != 1 {
		result.text = ""
		result.time = request.time
	}
	return result, true
}

func restartPositions(log string) []int {
	var positions []int
	position := 0
	for _, line := range strings.SplitAfter(log, "\n") {
		if isRestartLine(strings.TrimSuffix(line, "\n")) {
			positions = append(positions, position)
		}
		position += len(line)
	}
	return positions
}

func isRestartLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "Starting at ") && reTime.MatchString(strings.TrimPrefix(trimmed, "Starting at ")) {
		return true
	}
	native, ok := parseNativeMessage(line)
	return ok && native.source == "smqueue.cpp" && native.function == "main" &&
		strings.HasPrefix(native.text, "smqueue (re)starting")
}

func generationAt(restarts []int, position int) int {
	return sort.Search(len(restarts), func(i int) bool { return restarts[i] > position })
}

func scopedIdentity(generation int, qtag string) string {
	return strconv.Itoa(generation) + "\x00" + qtag
}

func parseNoticeParty(sms *SMS, value string, sender bool) {
	if match := reIMSI.FindStringSubmatch(value); match != nil {
		if sender {
			sms.SenderIMSI = match[1]
		} else {
			sms.ReceiverIMSI = match[1]
		}
		return
	}
	if !reNumber.MatchString(value) {
		return
	}
	if sender {
		sms.SenderNumber = value
	} else {
		sms.ReceiverNumber = value
	}
}

func mergeSMS(target *SMS, incoming SMS) {
	if target.Time == "" {
		target.Time = incoming.Time
	}
	if target.Text == "" {
		target.Text = incoming.Text
	}
	if target.SenderNumber == "" {
		target.SenderNumber = incoming.SenderNumber
	}
	if target.SenderIMSI == "" {
		target.SenderIMSI = incoming.SenderIMSI
	}
	if target.ReceiverNumber == "" {
		target.ReceiverNumber = incoming.ReceiverNumber
	}
	if target.ReceiverIMSI == "" {
		target.ReceiverIMSI = incoming.ReceiverIMSI
	}
}

func applyStructuredSMS(target *SMS, incoming SMS) {
	// Preserve the first observable event time while replacing all fields whose
	// values are explicitly carried by the keyed native record. Empty values are
	// authoritative too and must not be filled from a later unkeyed retry.
	if target.Time != "" {
		incoming.Time = target.Time
	}
	*target = incoming
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
