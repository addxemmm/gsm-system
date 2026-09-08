// Package api implements the /api/v1 HTTP layer.
//
// v1 contract (see docs/API.md and docs/api/openapi.yaml):
//   - Proper HTTP status codes (200/400/401/404/405/409/412/413/422/500/503)
//   - JSON envelope {"code","message","data","request_id"}; code 0 = success
//   - X-Request-ID response header; per-request audit log line
//   - Optional bearer auth via GSM_API_TOKEN (when set, all routes require it)
package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"
)

// Business codes for the v1 envelope. HTTP status is derived from code/100.
const (
	CodeOK           = 0
	CodeMalformed    = 40001 // body not JSON / wrong shape (HTTP 400)
	CodeUnauthorized = 40101 // missing or bad bearer token (HTTP 401)
	CodeNotFound     = 40401 // unknown resource id (HTTP 404)
	CodeMethod       = 40501 // method not allowed (HTTP 405)
	CodeConflict     = 40901 // cell already running, subscriber exists (HTTP 409)
	CodePrecondition = 41201 // cell not running, no UE/SMS, no data (HTTP 412)
	CodeTooLarge     = 41301 // upload exceeds limit (HTTP 413)
	CodeMediaType    = 41501 // request is not application/json (HTTP 415)
	CodeInvalid      = 42201 // validation failed, see data.errors (HTTP 422)
	CodeInternal     = 50001 // unexpected failure (HTTP 500)
	CodeNoHardware   = 50301 // no SDR attached (HTTP 503)
)

// FieldError describes one rejected field for 422 responses.
type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// Envelope is the v1 response body.
type Envelope struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id"`
}

type ctxKey int

const requestIDKey ctxKey = iota

// RequestID returns the request id attached by middleware ("" when absent).
func RequestID(r *http.Request) string {
	if v, ok := r.Context().Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000")
	}
	return hex.EncodeToString(b[:])
}

// writeV1 writes a v1 envelope with the HTTP status derived from code.
func writeV1(w http.ResponseWriter, r *http.Request, code int, message string, data any) {
	status := http.StatusOK
	if code != 0 {
		status = code / 100
	}
	if status < 100 || status > 599 {
		status = http.StatusInternalServerError
	}
	writeV1Status(w, r, status, code, message, data)
}

func writeV1Status(w http.ResponseWriter, r *http.Request, status, code int, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Request-ID", RequestID(r))
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{
		Code: code, Message: message, Data: data, RequestID: RequestID(r),
	})
}

// decodeV1JSON decodes exactly one JSON object and consumes the entire body so
// MaxBytesReader also enforces the limit against otherwise-ignored tail data.
// It returns CodeOK, CodeMalformed, or CodeTooLarge.
func decodeV1JSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64, strict bool) int {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return CodeMediaType
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return jsonDecodeCode(err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return jsonDecodeCode(err)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return CodeMalformed
	}
	if err := rejectDuplicateJSONNames(raw); err != nil {
		return CodeMalformed
	}
	objectDecoder := json.NewDecoder(bytes.NewReader(raw))
	if strict {
		objectDecoder.DisallowUnknownFields()
	}
	if err := objectDecoder.Decode(dst); err != nil {
		return CodeMalformed
	}
	return CodeOK
}

// rejectDuplicateJSONNames rejects duplicate names at every object depth,
// including duplicate OpenBTS keys inside PATCH /config values. encoding/json
// otherwise silently keeps the last occurrence.
func rejectDuplicateJSONNames(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, isDelim := token.(json.Delim)
		if !isDelim {
			return nil
		}
		switch delim {
		case '{':
			seen := make(map[string]struct{})
			for decoder.More() {
				nameToken, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := nameToken.(string)
				if !ok {
					return errors.New("JSON object name is not a string")
				}
				if _, duplicate := seen[name]; duplicate {
					return fmt.Errorf("duplicate JSON name %q", name)
				}
				seen[name] = struct{}{}
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return errors.New("unexpected JSON delimiter")
		}
	}
	return walk()
}

func jsonDecodeCode(err error) int {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return CodeTooLarge
	}
	return CodeMalformed
}

func writeV1DecodeError(w http.ResponseWriter, r *http.Request, code int) {
	if code == CodeTooLarge {
		writeV1(w, r, code, "request body too large", nil)
		return
	}
	if code == CodeMediaType {
		writeV1(w, r, code, "Content-Type must be application/json", nil)
		return
	}
	writeV1(w, r, CodeMalformed, "malformed JSON body", nil)
}

// chain applies middlewares: recover -> request id + audit log -> auth.
func chain(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey, id))
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("rid=%s panic recovered: %v", id, recovered)
				// Once a handler has started a response, net/http cannot replace
				// it. Before that point, preserve the v1 envelope contract.
				if !rec.wroteHeader {
					writeV1(rec, r, CodeInternal, "internal error", nil)
				}
			}
			log.Printf("rid=%s %s %s -> %d (%s)", id, r.Method, r.URL.Path,
				rec.status, time.Since(start).Round(time.Millisecond))
		}()
		if !authorized(r) {
			writeV1(rec, r, CodeUnauthorized,
				"unauthorized: bad or missing bearer token", nil)
			return
		}
		next.ServeHTTP(rec, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.wroteHeader {
		return
	}
	s.wroteHeader = true
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(p []byte) (int, error) {
	if !s.wroteHeader {
		s.WriteHeader(http.StatusOK)
	}
	return s.ResponseWriter.Write(p)
}

// authorized checks the optional bearer token. Empty GSM_API_TOKEN = open
// LAN mode (a warning is logged once at startup by main).
func authorized(r *http.Request) bool {
	want := strings.TrimSpace(os.Getenv("GSM_API_TOKEN"))
	if want == "" {
		return true
	}
	fields := strings.Fields(r.Header.Get("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return false
	}
	gotToken := fields[1]
	wantHash := sha256.Sum256([]byte(want))
	gotHash := sha256.Sum256([]byte(gotToken))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

// methodOnly wraps a handler, enforcing one HTTP method with a 405 envelope.
func methodOnly(method string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			writeV1(w, r, CodeMethod, "method not allowed, want "+method, nil)
			return
		}
		h(w, r)
	}
}
