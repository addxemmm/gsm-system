// Package webui serves the embedded browser UI and mounts the exact API
// handler used by the private API listener.
package webui

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// Embedding the directory (rather than enumerating its current build output)
// keeps hashed asset names and nested asset directories deployable.
//
//go:embed static
var embeddedAssets embed.FS

// Metadata is intentionally safe to expose before API authentication. It
// tells the UI whether to ask for a token, but never transports that token.
type Metadata struct {
	Version       string `json:"version"`
	Revision      string `json:"revision"`
	TokenRequired bool   `json:"token_required"`
}

type handler struct {
	api      http.Handler
	assets   fs.FS
	metadata []byte
	// publicOrigin is empty for direct serving and set when a trusted proxy
	// terminates TLS. Forwarded headers remain intentionally ignored.
	publicOrigin string
}

// New returns a handler for the browser listener. apiHandler must be the same
// already-constructed handler installed on the API listener; no credentials
// are added or auth decisions duplicated here.
func New(apiHandler http.Handler, version, revision string, tokenRequired bool, publicOrigin string) http.Handler {
	assets, err := fs.Sub(embeddedAssets, "static")
	if err != nil {
		panic("embedded web assets unavailable: " + err.Error())
	}
	metadata, err := json.Marshal(Metadata{
		Version: version, Revision: revision, TokenRequired: tokenRequired,
	})
	if err != nil {
		panic("marshal web metadata: " + err.Error())
	}
	metadata = append(metadata, '\n')
	return &handler{api: apiHandler, assets: assets, metadata: metadata, publicOrigin: publicOrigin}
}

// ProtectAPI applies the browser-origin write guard to the standalone API
// listener as well. The wrapped handler remains the sole owner of bearer auth
// and API routing.
func ProtectAPI(apiHandler http.Handler, publicOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w.Header())
		serveAPI(w, r, apiHandler, publicOrigin)
	})
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w.Header())

	if isAPIRoute(r.URL.Path) {
		serveAPI(w, r, h.api, h.publicOrigin)
		return
	}

	if r.URL.Path == "/web-meta.json" {
		h.serveMetadata(w, r)
		return
	}
	h.serveAsset(w, r)
}

func serveAPI(w http.ResponseWriter, r *http.Request, apiHandler http.Handler, publicOrigin string) {
	if isWriteMethod(r.Method) && !allowsOrigin(r, publicOrigin) {
		writeCrossOriginError(w)
		return
	}
	apiHandler.ServeHTTP(w, r)
}

func setSecurityHeaders(header http.Header) {
	header.Set("Content-Security-Policy", "default-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
}

func isAPIRoute(requestPath string) bool {
	return requestPath == "/api/v1" || strings.HasPrefix(requestPath, "/api/v1/")
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

// allowsOrigin permits non-browser clients with no Origin header and browser
// writes from this listener's exact origin. It deliberately does not trust
// X-Forwarded-* headers supplied by a client.
func allowsOrigin(r *http.Request, publicOrigin string) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	originScheme, originHost, originPort, ok := splitOrigin(origin)
	if !ok {
		return false
	}
	if publicOrigin != "" {
		publicScheme, publicHost, publicPort, ok := splitOrigin(publicOrigin)
		return ok && strings.EqualFold(originScheme, publicScheme) && strings.EqualFold(originHost, publicHost) && originPort == publicPort
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if !strings.EqualFold(originScheme, scheme) {
		return false
	}
	requestHost, requestPort, ok := splitAuthority(r.Host, scheme)
	return ok && strings.EqualFold(originHost, requestHost) && originPort == requestPort
}

func splitOrigin(value string) (scheme, host, port string, ok bool) {
	u, err := url.Parse(value)
	if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", "", "", false
	}
	host, port, ok = splitAuthority(u.Host, u.Scheme)
	return u.Scheme, host, port, ok
}

func splitAuthority(authority, scheme string) (host, port string, ok bool) {
	u, err := url.Parse("//" + authority)
	if err != nil || u.User != nil || u.Hostname() == "" {
		return "", "", false
	}
	host, port = u.Hostname(), u.Port()
	if port == "" {
		if scheme == "http" {
			port = "80"
		} else if scheme == "https" {
			port = "443"
		}
	} else if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return "", "", false
	}
	return host, port, true
}

func writeCrossOriginError(w http.ResponseWriter) {
	requestID := newRequestID()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		Data      any    `json:"data,omitempty"`
		RequestID string `json:"request_id"`
	}{Code: 40301, Message: "cross-origin write rejected", RequestID: requestID})
}

func newRequestID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "web-origin-check"
	}
	return hex.EncodeToString(value[:])
}

func (h *handler) serveMetadata(w http.ResponseWriter, r *http.Request) {
	if !allowStaticMethod(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(h.metadata)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(h.metadata)
	}
}

func (h *handler) serveAsset(w http.ResponseWriter, r *http.Request) {
	if !allowStaticMethod(w, r) {
		return
	}
	name, ok := assetName(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(h.assets, name)
	if err != nil {
		// Unknown application and legacy API paths are real 404s. The UI uses
		// fragment navigation, so serving index.html here would only hide bugs.
		http.NotFound(w, r)
		return
	}
	info, err := fs.Stat(h.assets, name)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	if strings.EqualFold(path.Ext(name), ".html") {
		w.Header().Set("Cache-Control", "no-store, max-age=0")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, name, info.ModTime(), bytes.NewReader(data))
}

func assetName(requestPath string) (string, bool) {
	if requestPath == "/" {
		return "index.html", true
	}
	if requestPath == "" || requestPath[0] != '/' || strings.HasSuffix(requestPath, "/") || strings.ContainsAny(requestPath, "\\\x00") {
		return "", false
	}
	name := strings.TrimPrefix(requestPath, "/")
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", false
		}
	}
	if !fs.ValidPath(name) || path.Clean(name) != name {
		return "", false
	}
	return name, true
}

func allowStaticMethod(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	w.Header().Set("Allow", "GET, HEAD")
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}
