// Helpers for bounded native commands, pagination, and tail reads.
// All commands use argv; none invoke a shell.
package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/addxemmm/gsm-system/internal/gsm"
)

const (
	defaultPageLimit = 100
	maxPageLimit     = 500
	maxPageOffset    = 1_000_000
)

type pageRequest struct {
	Limit  int
	Offset int
}

type tailWindow struct {
	Bytes     int64 `json:"bytes"`
	MaxBytes  int64 `json:"max_bytes"`
	Truncated bool  `json:"truncated"`
}

func runCLIContext(parent context.Context, bin string, args ...string) (string, error) {
	if strings.TrimSpace(bin) == "" {
		return "", errors.New("native command is not configured")
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("native command: %w: %s", err, strings.TrimSpace(string(out)))
	}
	lower := strings.ToLower(string(out))
	for _, marker := range []string{"no such command", "unable to connect", "command not found"} {
		if strings.Contains(lower, marker) {
			return string(out), fmt.Errorf("native command reported %q", marker)
		}
	}
	return string(out), nil
}

func parsePageQuery(values url.Values) (pageRequest, []FieldError) {
	page := pageRequest{Limit: defaultPageLimit}
	var fieldErrors []FieldError
	for key, entries := range values {
		if key != "limit" && key != "offset" {
			fieldErrors = append(fieldErrors, FieldError{Field: key, Reason: "unknown query parameter"})
			continue
		}
		if len(entries) != 1 {
			fieldErrors = append(fieldErrors, FieldError{Field: key, Reason: "must be specified once"})
		}
	}
	if entries, ok := values["limit"]; ok && len(entries) == 1 {
		n, err := strconv.Atoi(entries[0])
		if err != nil || n < 1 || n > maxPageLimit {
			fieldErrors = append(fieldErrors, FieldError{Field: "limit", Reason: "must be an integer from 1 to 500"})
		} else {
			page.Limit = n
		}
	}
	if entries, ok := values["offset"]; ok && len(entries) == 1 {
		n, err := strconv.Atoi(entries[0])
		if err != nil || n < 0 || n > maxPageOffset {
			fieldErrors = append(fieldErrors, FieldError{Field: "offset", Reason: "must be an integer from 0 to 1000000"})
		} else {
			page.Offset = n
		}
	}
	sort.Slice(fieldErrors, func(i, j int) bool { return fieldErrors[i].Field < fieldErrors[j].Field })
	return page, fieldErrors
}

func pageSlice[T any](items []T, page pageRequest) []T {
	if page.Offset >= len(items) {
		return []T{}
	}
	end := page.Offset + page.Limit
	if end > len(items) {
		end = len(items)
	}
	return items[page.Offset:end]
}

func requireNoQuery(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.RawQuery == "" {
		return true
	}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeValidation(w, r, []FieldError{{Field: "query", Reason: "malformed query string"}})
		return false
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	errs := make([]FieldError, 0, len(keys))
	for _, key := range keys {
		errs = append(errs, FieldError{Field: key, Reason: "unknown query parameter"})
	}
	writeValidation(w, r, errs)
	return false
}

// readTail reads at most maxBytes and discards the first partial physical line
// when seeking into a file. smqueue events are line-oriented and the parser
// independently requires a complete event marker before accepting a message.
func readTail(path string, maxBytes int64) ([]byte, tailWindow, error) {
	if maxBytes <= 0 {
		return nil, tailWindow{}, errors.New("tail size must be positive")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, tailWindow{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, tailWindow{}, err
	}
	start := int64(0)
	truncated := info.Size() > maxBytes
	if truncated {
		start = info.Size() - maxBytes
		if _, err := f.Seek(start, io.SeekStart); err != nil {
			return nil, tailWindow{}, err
		}
	}
	b, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, tailWindow{}, err
	}
	if int64(len(b)) > maxBytes {
		b = b[:maxBytes]
		truncated = true
	}
	if truncated {
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			b = b[i+1:]
		} else {
			b = nil
		}
	}
	return b, tailWindow{Bytes: int64(len(b)), MaxBytes: maxBytes, Truncated: truncated}, nil
}

func validateIface(iface string) error {
	if !gsm.ValidInterfaceName(strings.TrimSpace(iface)) {
		return errors.New("must be a safe Linux interface name of 1-15 characters")
	}
	return nil
}

// ipv4Forwarding reports this process's network-namespace state without
// changing it. Under the production Docker bridge, Compose enables this
// namespaced sysctl; nil means procfs is absent or unreadable.
func ipv4Forwarding() *bool {
	b, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return nil
	}
	var enabled bool
	switch strings.TrimSpace(string(b)) {
	case "0":
		enabled = false
	case "1":
		enabled = true
	default:
		return nil
	}
	return &enabled
}
