// Package api exposes the versioned GSM System HTTP API.
package api

import (
	"net/http"
	"time"

	"github.com/addxemmm/gsm-system/internal/config"
	"github.com/addxemmm/gsm-system/internal/gsm"
)

type Server struct {
	cfg            config.Config
	mgr            *gsm.Manager
	mux            *http.ServeMux
	location       *time.Location
	readCurrentSMS func(int64) ([]byte, gsm.SMSWindow, error)
}

// New builds the /api/v1-only router. Removed legacy root handlers are not
// aliases: callers receive the standard 404 envelope and must migrate.
func New(cfg config.Config, mgr *gsm.Manager) *Server {
	loc, err := cfg.Location()
	if err != nil {
		panic("invalid server timezone: " + err.Error()) // Config.Load validates before startup.
	}
	s := &Server{cfg: cfg, mgr: mgr, mux: http.NewServeMux(), location: loc, readCurrentSMS: mgr.ReadCurrentSMS}
	s.mux.HandleFunc("/", s.serveV1)
	return s
}

func (s *Server) cellStatus() gsm.Status {
	status := s.mgr.IsRunning()
	if status.StartedAt != nil {
		local := status.StartedAt.In(s.location)
		status.StartedAt = &local
	}
	return status
}

func (s *Server) Handler() http.Handler { return chain(s.mux) }

func nilStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}
