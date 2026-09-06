// v1 implements the standard REST API (/api/v1/*). See docs/API.md and
// docs/api/openapi.yaml. Differences from frozen legacy: proper HTTP status,
// {"code","message","data","request_id"} envelope, per-field 422, explicit
// subscriber semantics, idempotent stop.
package api

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/addxemmm/gsm-system/internal/gsm"
	"github.com/addxemmm/gsm-system/internal/parser"
	"github.com/addxemmm/gsm-system/internal/sysop"
)

// v1Route documents one endpoint (kept in sync with serveV1 + openapi.yaml).
type v1Route struct {
	method  string
	path    string
	summary string
}

var v1Routes = []v1Route{
	{http.MethodPost, "/api/v1/cell", "start cell ({} reuses profile)"},
	{http.MethodGet, "/api/v1/cell", "cell status"},
	{http.MethodDelete, "/api/v1/cell", "stop cell (idempotent)"},
	{http.MethodGet, "/api/v1/ue", "attached UE list"},
	{http.MethodGet, "/api/v1/sms", "SMS list from smqueue log"},
	{http.MethodPost, "/api/v1/sms", "send SMS via OpenBTS"},
	{http.MethodGet, "/api/v1/subscribers", "explicit subscriber registry"},
	{http.MethodPost, "/api/v1/subscribers", "set subscriber number (explicit)"},
	{http.MethodGet, "/api/v1/config", "full OpenBTS config"},
	{http.MethodPatch, "/api/v1/config", "update single config key (strict)"},
	{http.MethodPost, "/api/v1/network", "set uplink iface MASQUERADE"},
	{http.MethodGet, "/api/v1/profile", "saved profile + subscribers"},
	{http.MethodGet, "/api/v1/health", "health + SDR detection"},
}

func (s *Server) serveV1(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	for _, rt := range v1Routes {
		if rt.path == path {
			switch {
			case path == "/api/v1/cell" && r.Method == http.MethodPost:
				s.handleV1Start(w, r)
			case path == "/api/v1/cell" && r.Method == http.MethodGet:
				writeV1(w, r, CodeOK, "ok", s.mgr.IsRunning())
			case path == "/api/v1/cell" && r.Method == http.MethodDelete:
				s.handleV1Stop(w, r)
			case path == "/api/v1/ue" && r.Method == http.MethodGet:
				s.handleV1UE(w, r)
			case path == "/api/v1/sms" && r.Method == http.MethodGet:
				s.handleV1SmsList(w, r)
			case path == "/api/v1/sms" && r.Method == http.MethodPost:
				s.handleV1SmsSend(w, r)
			case path == "/api/v1/subscribers" && r.Method == http.MethodGet:
				s.handleV1SubsList(w, r)
			case path == "/api/v1/subscribers" && r.Method == http.MethodPost:
				s.handleV1SubsSet(w, r)
			case path == "/api/v1/config" && r.Method == http.MethodGet:
				s.handleV1ConfigGet(w, r)
			case path == "/api/v1/config" && r.Method == http.MethodPatch:
				s.handleV1ConfigPatch(w, r)
			case path == "/api/v1/network" && r.Method == http.MethodPost:
				s.handleV1Network(w, r)
			case path == "/api/v1/profile" && r.Method == http.MethodGet:
				s.handleProfile(w, r)
			case path == "/api/v1/health" && r.Method == http.MethodGet:
				s.handleV1Health(w, r)
			default:
				writeV1(w, r, CodeMethod, "method not allowed for "+path, nil)
			}
			return
		}
	}
	writeV1(w, r, CodeNotFound, "not found: "+path, nil)
}

func (s *Server) handleV1Start(w http.ResponseWriter, r *http.Request) {
	var p gsm.StartParams
	if code := decodeV1JSON(w, r, &p, 1<<20, true); code != CodeOK {
		writeV1DecodeError(w, r, code)
		return
	}
	s.mgr.OverlayProfile(&p)
	if issues := p.ValidateDetailed(); len(issues) > 0 {
		errs := make([]map[string]string, 0, len(issues))
		for _, is := range issues {
			errs = append(errs, map[string]string{"field": is.Field, "reason": is.Reason})
		}
		writeV1(w, r, CodeInvalid, "validation failed", map[string]any{"errors": errs})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	if err := s.mgr.Start(ctx, p); err != nil {
		switch {
		case err.Error() == "is running":
			writeV1(w, r, CodeConflict, "cell already running", nil)
		case strings.Contains(err.Error(), "not connected"):
			writeV1(w, r, CodeNoHardware, "no SDR device attached", map[string]any{"detail": err.Error()})
		case strings.Contains(err.Error(), "unknown network interface"):
			writeV1(w, r, CodeInvalid, err.Error(), map[string]any{"errors": []map[string]string{{"field": "network", "reason": err.Error()}}})
		default:
			writeV1(w, r, CodeInternal, "start failed: "+err.Error(), nil)
		}
		return
	}
	writeV1(w, r, CodeOK, "cell started", map[string]any{
		"band": p.Band, "mcc": p.MCC, "mnc": p.MNC, "short_name": p.ShortName,
	})
}

func (s *Server) handleV1Stop(w http.ResponseWriter, r *http.Request) {
	if !anyCellServiceRunning(sysop.Running) {
		writeV1(w, r, CodeOK, "already stopped", map[string]any{"stopped": false})
		return
	}
	s.mgr.Stop()
	if !anyCellServiceRunning(sysop.Running) {
		writeV1(w, r, CodeOK, "cell stopped", map[string]any{"stopped": true})
		return
	}
	writeV1(w, r, CodeInternal, "stop failed, inspect logs", nil)
}

func anyCellServiceRunning(running func(string) bool) bool {
	for _, name := range []string{"OpenBTS", "transceiver", "sipauthserve", "smqueue", "asterisk"} {
		if running(name) {
			return true
		}
	}
	return false
}

func (s *Server) handleV1UE(w http.ResponseWriter, r *http.Request) {
	if !sysop.Running("OpenBTS") {
		writeV1(w, r, CodePrecondition, "cell not running", nil)
		return
	}
	sgsnOut, _ := runCLI(s.cfg.OpenBTSCLIBin, "-c", "sgsn list")
	tmsiOut, _ := runCLI(s.cfg.OpenBTSCLIBin, "-c", "tmsis -l")
	ues := parser.ParseTMSIs(tmsiOut, parser.ParseSGSN(sgsnOut))
	if len(ues) == 0 {
		writeV1(w, r, CodeNotFound, "no UE attached", nil)
		return
	}
	items := make([]map[string]any, 0, len(ues))
	for _, u := range ues {
		items = append(items, map[string]any{
			"imsi": u.IMSI, "imei": nilStr(u.IMEI),
			"number": nilStr(u.Number), "ip": nilStr(u.IP),
		})
	}
	writeV1(w, r, CodeOK, "ok", map[string]any{"ues": items, "count": len(items)})
}

func (s *Server) handleV1SmsList(w http.ResponseWriter, r *http.Request) {
	b, err := os.ReadFile(s.cfg.LogPath(s.cfg.SmqueueLogName))
	if err != nil {
		writeV1(w, r, CodeNotFound, "smqueue log not found", nil)
		return
	}
	sms := parser.ParseSmqueueLog(string(b))
	if len(sms) == 0 {
		writeV1(w, r, CodeNotFound, "no SMS yet", nil)
		return
	}
	items := make([]map[string]any, 0, len(sms))
	for _, m := range sms {
		items = append(items, map[string]any{
			"time": nilStr(m.Time), "text": nilStr(m.Text),
			"sender_number": nilStr(m.SenderNumber), "sender_imsi": nilStr(m.SenderIMSI),
			"receiver_number": nilStr(m.ReceiverNumber), "receiver_imsi": nilStr(m.ReceiverIMSI),
		})
	}
	writeV1(w, r, CodeOK, "ok", map[string]any{"sms": items, "count": len(items)})
}

func (s *Server) handleV1SmsSend(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IMSI   string `json:"imsi"`
		Sender string `json:"sender"`
		Text   string `json:"text"`
		// Legacy alias: smsmessage.
		SMSMessage string `json:"smsmessage"`
	}
	if code := decodeV1JSON(w, r, &body, 64*1024, false); code != CodeOK {
		writeV1DecodeError(w, r, code)
		return
	}
	text := body.Text
	if text == "" {
		text = body.SMSMessage
	}
	var errs []map[string]string
	if err := gsm.ValidateIMSI(body.IMSI); err != nil {
		errs = append(errs, map[string]string{"field": "imsi", "reason": err.Error()})
	}
	if body.Sender == "" {
		errs = append(errs, map[string]string{"field": "sender", "reason": "required"})
	}
	if text == "" {
		errs = append(errs, map[string]string{"field": "text", "reason": "required"})
	} else if !isGSM7(text) {
		errs = append(errs, map[string]string{"field": "text", "reason": "non-GSM7 characters (Chinese not supported, see docs)"})
	}
	if len(errs) > 0 {
		writeV1(w, r, CodeInvalid, "validation failed", map[string]any{"errors": errs})
		return
	}
	if !sysop.Running("OpenBTS") {
		writeV1(w, r, CodePrecondition, "cell not running", nil)
		return
	}
	out, _ := runCLI(s.cfg.OpenBTSCLIBin, "-c",
		"sendsms "+body.IMSI+" "+body.Sender+" "+text)
	if strings.Contains(out, "message submitted for delivery") {
		writeV1(w, r, CodeOK, "sms submitted", map[string]any{"imsi": body.IMSI})
		return
	}
	writeV1(w, r, CodeInternal, "send failed, inspect openbts log", nil)
}

func (s *Server) handleV1SubsList(w http.ResponseWriter, r *http.Request) {
	subscribers, err := listSubscribers(s.cfg)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeV1(w, r, CodeNotFound, "asterisk registry db not found", nil)
			return
		}
		log.Printf("rid=%s list subscribers failed: %v", RequestID(r), err)
		writeV1(w, r, CodeInternal, "subscriber registry query failed", nil)
		return
	}
	writeV1(w, r, CodeOK, "ok", map[string]any{
		"subscribers": subscribers,
	})
}

func (s *Server) handleV1SubsSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IMSI   string `json:"imsi"`
		Number string `json:"number"`
	}
	if code := decodeV1JSON(w, r, &body, 64*1024, false); code != CodeOK {
		writeV1DecodeError(w, r, code)
		return
	}
	var errs []map[string]string
	if err := gsm.ValidateIMSI(body.IMSI); err != nil {
		errs = append(errs, map[string]string{"field": "imsi", "reason": err.Error()})
	}
	if err := gsm.ValidateNumber(body.Number); err != nil {
		errs = append(errs, map[string]string{"field": "number", "reason": err.Error()})
	}
	if len(errs) > 0 {
		writeV1(w, r, CodeInvalid, "validation failed", map[string]any{"errors": errs})
		return
	}
	code, _, err := setSubscriberNumber(s.cfg, body.IMSI, body.Number)
	if err != nil {
		log.Printf("rid=%s subscriber update failed: %v", RequestID(r), err)
		writeV1(w, r, CodeInternal, "subscriber update failed", nil)
		return
	}
	switch code {
	case 1:
		writeV1(w, r, CodeOK, "subscriber updated", map[string]any{"imsi": body.IMSI, "number": body.Number})
	case 4:
		writeV1(w, r, CodePrecondition, "imsi not in asterisk registry", nil)
	default:
		writeV1(w, r, CodeNotFound, "imsi not found, attach UE first", nil)
	}
}

func (s *Server) handleV1ConfigGet(w http.ResponseWriter, r *http.Request) {
	rows, err := s.mgr.GetAllConfig()
	if err != nil {
		if errors.Is(err, gsm.ErrDatabaseNotFound) {
			writeV1(w, r, CodeNotFound, "openbts db not found", nil)
			return
		}
		log.Printf("rid=%s read config failed: %v", RequestID(r), err)
		writeV1(w, r, CodeInternal, "config query failed", nil)
		return
	}
	items := make([]map[string]string, 0, len(rows))
	for _, kv := range rows {
		items = append(items, map[string]string{"key": kv[0], "value": kv[1]})
	}
	writeV1(w, r, CodeOK, "ok", map[string]any{"config": items})
}

func (s *Server) handleV1ConfigPatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if code := decodeV1JSON(w, r, &body, 64*1024, false); code != CodeOK {
		writeV1DecodeError(w, r, code)
		return
	}
	if body.Name == "" {
		writeV1(w, r, CodeInvalid, "validation failed", map[string]any{
			"errors": []map[string]string{{"field": "name", "reason": "required"}}})
		return
	}
	if err := s.mgr.SetSingleConfig(body.Name, body.Value); err != nil {
		switch {
		case errors.Is(err, gsm.ErrCellRunning):
			writeV1(w, r, CodeConflict, "stop the cell before changing config", nil)
		case errors.Is(err, gsm.ErrDatabaseNotFound):
			writeV1(w, r, CodeNotFound, "openbts db not found", nil)
		case err.Error() == "invalid config name/value":
			writeV1(w, r, CodeInvalid, err.Error(), nil)
		default:
			log.Printf("rid=%s update config failed: %v", RequestID(r), err)
			writeV1(w, r, CodeInternal, "config update failed", nil)
		}
		return
	}
	writeV1(w, r, CodeOK, "config updated, start the cell to apply", map[string]any{"name": body.Name})
}

func (s *Server) handleV1Network(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Iface string `json:"iface"`
	}
	if code := decodeV1JSON(w, r, &body, 64*1024, false); code != CodeOK {
		writeV1DecodeError(w, r, code)
		return
	}
	if body.Iface == "" || strings.ContainsAny(body.Iface, " \t\n\r\"';&|<>$`\\") {
		writeV1(w, r, CodeInvalid, "validation failed", map[string]any{
			"errors": []map[string]string{{"field": "iface", "reason": "required, no shell metachars"}}})
		return
	}
	if err := applyIptables(s.cfg.IptablesBin, body.Iface); err != nil {
		writeV1(w, r, CodeInternal, "iptables failed: "+err.Error(), nil)
		return
	}
	writeV1(w, r, CodeOK, "network configured", map[string]any{"iface": body.Iface})
}

func (s *Server) handleV1Health(w http.ResponseWriter, r *http.Request) {
	det := s.mgr.DetectUSRP()
	st := s.mgr.IsRunning()
	writeV1(w, r, CodeOK, "ok", map[string]any{
		"ok": true, "running": st.Running, "cell": st,
		"sdr": det, "time": timeNow(),
	})
}

func timeNow() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func isGSM7(s string) bool {
	for _, r := range s {
		if r > 0x7F {
			return false
		}
	}
	return true
}
