// Package telephony reads observable Asterisk call state. It never originates
// calls or infers delivery from submission.
package telephony

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ActiveCall struct {
	Channel         string  `json:"channel"`
	Context         string  `json:"context"`
	Extension       string  `json:"extension"`
	State           string  `json:"state"`
	Application     string  `json:"application"`
	CallerID        string  `json:"caller_id"`
	DurationSeconds int     `json:"duration_seconds"`
	BridgeID        *string `json:"bridge_id"`
	UniqueID        *string `json:"unique_id"`
}

// HistoryCall represents all 18 columns emitted by cdr_csv when
// loguniqueid=yes and loguserfield=yes. Times are normalized to RFC3339 UTC.
type HistoryCall struct {
	AccountCode        string  `json:"account_code"`
	Source             string  `json:"source"`
	Destination        string  `json:"destination"`
	DestinationContext string  `json:"destination_context"`
	CallerID           string  `json:"caller_id"`
	Channel            string  `json:"channel"`
	DestinationChannel string  `json:"destination_channel"`
	LastApplication    string  `json:"last_application"`
	LastData           string  `json:"last_data"`
	StartedAt          string  `json:"started_at"`
	AnsweredAt         *string `json:"answered_at"`
	EndedAt            string  `json:"ended_at"`
	DurationSeconds    int     `json:"duration_seconds"`
	BilledSeconds      int     `json:"billed_seconds"`
	Disposition        string  `json:"disposition"`
	AMAFlags           string  `json:"ama_flags"`
	UniqueID           *string `json:"unique_id"`
	UserField          *string `json:"user_field"`
}

type HistoryWindow struct {
	Bytes     int64 `json:"bytes"`
	MaxBytes  int64 `json:"max_bytes"`
	Truncated bool  `json:"truncated"`
}

const maxHistoryWindowBytes = int64(64 << 20)

func Active(ctx context.Context, asteriskBin string) ([]ActiveCall, error) {
	if asteriskBin == "" {
		asteriskBin = "asterisk"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Asterisk can emit unrelated startup warnings on stderr even when the CLI
	// request succeeds (for example, no ethernet interface for global EID
	// seeding). Parse stdout only; stderr must never be mistaken for a channel
	// row. Keep failures generic so native diagnostics cannot leak call data.
	out, err := exec.CommandContext(ctx, asteriskBin, "-rx", "core show channels concise").Output()
	if err != nil {
		return nil, fmt.Errorf("asterisk active channel query: %w", err)
	}
	return ParseConcise(string(out))
}

// ParseConcise parses Asterisk 18's 14 non-empty concise fields. The final
// values are duration, bridge id, and unique id (indexes 11, 12, and 13).
func ParseConcise(output string) ([]ActiveCall, error) {
	calls := make([]ActiveCall, 0)
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		// Asterisk 18 remote CLI emits this fixed bootstrap notice on stdout
		// before the concise response when an isolated host has no Ethernet.
		// Ignore only this known notice; arbitrary command errors still fail.
		if line == "No ethernet interface found for seeding global EID. You will have to set it manually." {
			continue
		}
		if line == "" {
			continue
		}
		if !strings.Contains(line, "!") {
			if conciseSummary.MatchString(line) {
				continue
			}
			return nil, fmt.Errorf("unexpected concise output %q", line)
		}
		fields := strings.Split(line, "!")
		if len(fields) != 14 {
			return nil, fmt.Errorf("malformed concise channel row: want 14 fields")
		}
		duration, err := strconv.Atoi(strings.TrimSpace(fields[11]))
		if err != nil || duration < 0 {
			return nil, fmt.Errorf("invalid channel duration %q", fields[11])
		}
		calls = append(calls, ActiveCall{
			Channel:         fields[0],
			Context:         fields[1],
			Extension:       fields[2],
			State:           fields[4],
			Application:     fields[5],
			CallerID:        fields[7],
			DurationSeconds: duration,
			BridgeID:        nullableNative(fields[12]),
			UniqueID:        nullableNative(fields[13]),
		})
	}
	return calls, nil
}

var conciseSummary = regexp.MustCompile(`^[0-9]+ active (?:channels?|calls?)$`)

// History reads a bounded tail. It scans backward from EOF, where CSV quote
// state is known, to discard the first possibly partial record without treating
// a newline inside a quoted multiline field as a boundary.
func History(ctx context.Context, path string, maxBytes int64) ([]HistoryCall, HistoryWindow, error) {
	if maxBytes <= 0 {
		return nil, HistoryWindow{}, errors.New("history window must be positive")
	}
	if maxBytes > maxHistoryWindowBytes {
		return nil, HistoryWindow{}, fmt.Errorf("history window exceeds %d bytes", maxHistoryWindowBytes)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, HistoryWindow{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, HistoryWindow{}, err
	}
	start := int64(0)
	window := HistoryWindow{MaxBytes: maxBytes, Truncated: info.Size() > maxBytes}
	if window.Truncated {
		start = info.Size() - maxBytes
	}
	data, err := readSectionContext(ctx, f, start, info.Size()-start)
	if err != nil {
		return nil, HistoryWindow{}, err
	}
	if window.Truncated {
		// cdr_csv terminates complete records with a newline. A truncated tail
		// captured during an in-progress append has unknown EOF quote state, so
		// fail explicitly rather than guessing backward boundaries.
		if len(data) != 0 && data[len(data)-1] != '\n' {
			return nil, HistoryWindow{}, errors.New("parse CDR: truncated tail ends with an incomplete record")
		}
		boundary, found, boundaryErr := firstSafeCSVBoundary(ctx, data)
		if boundaryErr != nil {
			return nil, HistoryWindow{}, boundaryErr
		}
		if !found {
			data = nil
		} else {
			data = data[boundary:]
		}
	}
	window.Bytes = int64(len(data))

	reader := csv.NewReader(contextReader{ctx: ctx, reader: bytes.NewReader(data)})
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true
	rows := make([]HistoryCall, 0)
	for {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, HistoryWindow{}, fmt.Errorf("parse CDR: %w", readErr)
		}
		if len(record) == 1 && strings.TrimSpace(record[0]) == "" {
			continue
		}
		if len(record) != 18 {
			return nil, HistoryWindow{}, fmt.Errorf("parse CDR: want 18 columns, got %d", len(record))
		}
		call, parseErr := parseHistoryRow(record)
		if parseErr != nil {
			return nil, HistoryWindow{}, parseErr
		}
		rows = append(rows, call)
	}
	return rows, window, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func readSectionContext(ctx context.Context, f *os.File, offset, length int64) ([]byte, error) {
	if length == 0 {
		return []byte{}, nil
	}
	reader := io.NewSectionReader(f, offset, length)
	var buffer bytes.Buffer
	buffer.Grow(int(length))
	chunk := make([]byte, 64<<10)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, err := reader.Read(chunk)
		if n > 0 {
			_, _ = buffer.Write(chunk[:n])
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return buffer.Bytes(), nil
}

// firstSafeCSVBoundary returns the first record start after the truncated
// prefix. Scanning from complete EOF makes quote state deterministic. Paired
// CSV quotes toggle twice and therefore preserve the correct state.
func firstSafeCSVBoundary(ctx context.Context, data []byte) (int, bool, error) {
	inQuotes := false
	earliest := -1
	for index := len(data) - 1; index >= 0; index-- {
		if index%(64<<10) == 0 {
			if err := ctx.Err(); err != nil {
				return 0, false, err
			}
		}
		switch data[index] {
		case '"':
			inQuotes = !inQuotes
		case '\n':
			if !inQuotes && index+1 < len(data) {
				earliest = index + 1
			}
		}
	}
	return earliest, earliest >= 0, nil
}

func parseHistoryRow(row []string) (HistoryCall, error) {
	started, err := cdrTime(row[9], false)
	if err != nil {
		return HistoryCall{}, fmt.Errorf("parse CDR start: %w", err)
	}
	answered, err := cdrTime(row[10], true)
	if err != nil {
		return HistoryCall{}, fmt.Errorf("parse CDR answer: %w", err)
	}
	ended, err := cdrTime(row[11], false)
	if err != nil {
		return HistoryCall{}, fmt.Errorf("parse CDR end: %w", err)
	}
	duration, err := nonNegativeInt(row[12])
	if err != nil {
		return HistoryCall{}, fmt.Errorf("parse CDR duration: %w", err)
	}
	billed, err := nonNegativeInt(row[13])
	if err != nil {
		return HistoryCall{}, fmt.Errorf("parse CDR billsec: %w", err)
	}
	return HistoryCall{
		AccountCode:        row[0],
		Source:             row[1],
		Destination:        row[2],
		DestinationContext: row[3],
		CallerID:           row[4],
		Channel:            row[5],
		DestinationChannel: row[6],
		LastApplication:    row[7],
		LastData:           row[8],
		StartedAt:          started,
		AnsweredAt:         nullable(answered),
		EndedAt:            ended,
		DurationSeconds:    duration,
		BilledSeconds:      billed,
		Disposition:        row[14],
		AMAFlags:           row[15],
		UniqueID:           nullable(row[16]),
		UserField:          nullable(row[17]),
	}, nil
}

func cdrTime(value string, optional bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" && optional {
		return "", nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339Nano} {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
	}
	return "", fmt.Errorf("invalid UTC timestamp %q", value)
}

func nonNegativeInt(value string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid non-negative integer %q", value)
	}
	return n, nil
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func nullableNative(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" || value == "(None)" || value == "<none>" {
		return nil
	}
	return nullable(value)
}
