// Package api exposes the versioned GSM System HTTP API.
package api

import (
	"net/http"

	"github.com/addxemmm/gsm-system/internal/config"
	"github.com/addxemmm/gsm-system/internal/gsm"
)

type Server struct {
	cfg config.Config
	mgr *gsm.Manager
	mux *http.ServeMux
}

// New builds the /api/v1-only router. Removed legacy root handlers are not
// aliases: callers receive the standard 404 envelope and must migrate.
func New(cfg config.Config, mgr *gsm.Manager) *Server {
	s := &Server{cfg: cfg, mgr: mgr, mux: http.NewServeMux()}
	s.mux.HandleFunc("/", s.serveV1)
	return s
}

func (s *Server) Handler() http.Handler { return chain(s.mux) }

func nilStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}
