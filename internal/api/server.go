// Server wires frozen legacy routes + /api/v1. Legacy responses keep
// run.py verbatim: HTTP 200 always, {status,message_id,message,...}.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/addxemmm/gsm-system/internal/config"
	"github.com/addxemmm/gsm-system/internal/gsm"
	"github.com/addxemmm/gsm-system/internal/parser"
	"github.com/addxemmm/gsm-system/internal/sdr"
	"github.com/addxemmm/gsm-system/internal/sysop"
)

// Server wires handlers to a Manager.
type Server struct {
	cfg config.Config
	mgr *gsm.Manager
	mux *http.ServeMux
}

// New builds routes.
func New(cfg config.Config, mgr *gsm.Manager) *Server {
	s := &Server{cfg: cfg, mgr: mgr, mux: http.NewServeMux()}
	s.mux.HandleFunc("/start", s.handleStart)
	s.mux.HandleFunc("/stop", s.handleStop)
	s.mux.HandleFunc("/config", s.handleConfig)
	s.mux.HandleFunc("/getconfig", s.handleGetConfig)
	s.mux.HandleFunc("/allconfig", s.handleAllConfig)
	s.mux.HandleFunc("/iptables", s.handleIptables)
	s.mux.HandleFunc("/smsinfo", s.handleSmsInfo)
	s.mux.HandleFunc("/ueinfo", s.handleUeInfo)
	s.mux.HandleFunc("/setphonenumber", s.handleSetPhone)
	s.mux.HandleFunc("/sendsms", s.handleSendSms)
	s.mux.HandleFunc("/healthz", s.handleHealthz)
	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/profile", s.handleProfile)
	s.mux.HandleFunc("/api/", s.serveV1)
	s.mux.HandleFunc("/", s.handleNotFound)
	return s
}

// Handler returns the mux wrapped in the middleware chain.
func (s *Server) Handler() http.Handler { return chain(s.mux) }

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeV1(w, r, CodeNotFound, "not found: "+r.URL.Path, nil)
		return
	}
	http.NotFound(w, r)
}

// ---- legacy envelope (frozen: status/message_id/message, HTTP 200) ----

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func resp(status bool, id int, msg string, extras ...map[string]any) map[string]any {
	m := map[string]any{"status": status, "message_id": id, "message": msg}
	for _, extra := range extras {
		for k, v := range extra {
			m[k] = v
		}
	}
	return m
}

func nilStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

// ---- POST /start (legacy, frozen) ----

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, resp(false, 0, "Start Failed"))
		return
	}
	if sysop.Running("OpenBTS") {
		writeJSON(w, resp(false, 2, "is running"))
		return
	}
	// Reuse saved profile when present (empty body => last_start.json).
	var p gsm.StartParams
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p)
	s.mgr.OverlayProfile(&p)
	if p.Band == "" && p.MCC == "" {
		// No profile and no body: fall back to defaults (id=0 equivalent)
		// so bare POST still works like legacy no-arg start.
		p = gsm.StartParams{ARFCNs: s.cfg.DefaultARFCNs, C0: s.cfg.DefaultC0,
			Band: s.cfg.DefaultBand, MCC: s.cfg.DefaultMCC, MNC: s.cfg.DefaultMNC,
			LAC: s.cfg.DefaultLAC, CI: s.cfg.DefaultCI, ShortName: s.cfg.DefaultShortName}
	}
	if err := p.Validate(); err != nil {
		// Legacy has no 422: incomplete => message_id 0 Start Failed,
		// except USRP-missing which keeps id 3.
		if strings.Contains(err.Error(), "incomplete") {
			writeJSON(w, resp(false, 0, "Start Failed"))
			return
		}
		writeJSON(w, resp(false, 0, "Start Failed: "+err.Error()))
		return
	}
	if !sdr.Detect().UHD_B210 {
		// Allow unit-test env without SDR only when explicitly loopback?
		// Keep frozen: report not connected.
		writeJSON(w, resp(false, 3, "device is not connected, please connect usrp device."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	if err := s.mgr.Start(ctx, p); err != nil {
		switch {
		case err.Error() == "is running":
			writeJSON(w, resp(false, 2, "is running"))
		case strings.Contains(err.Error(), "not connected"):
			writeJSON(w, resp(false, 3, "device is not connected, please connect usrp device."))
		default:
			writeJSON(w, resp(false, 0, "Start Failed"))
		}
		return
	}
	writeJSON(w, resp(true, 1, "Start successfully"))
}

// ---- POST /stop ----

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, resp(false, 0, "Stop failed."))
		return
	}
	if !sysop.Running("OpenBTS") && !sysop.Running("sipauthserve") &&
		!sysop.Running("smqueue") && !sysop.Running("asterisk") && !sysop.Running("transceiver") {
		writeJSON(w, resp(false, 2, "Not running."))
		return
	}
	if s.mgr.Stop() {
		writeJSON(w, resp(true, 1, "Stop successfully."))
		return
	}
	if !sysop.Running("OpenBTS") && !sysop.Running("transceiver") {
		writeJSON(w, resp(true, 1, "Stop successfully."))
		return
	}
	writeJSON(w, resp(false, 0, "Stop failed."))
}

// ---- POST /config {id} ----

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, resp(false, 0, "Failed."))
		return
	}
	var body struct {
		ID *int `json:"id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&body); err != nil || body.ID == nil {
		writeJSON(w, resp(false, 4, "The config id is not existed."))
		return
	}
	if *body.ID < 0 || *body.ID >= len(gsm.Presets) {
		writeJSON(w, resp(false, 4, "The config id is not existed."))
		return
	}
	if _, err := os.Stat(s.cfg.OpenBTSDbPath); err != nil {
		writeJSON(w, resp(false, 3, "Can not find database file."))
		return
	}
	// Legacy stops the cell first; stop failure (id 0) => message_id 2.
	if sysop.Running("OpenBTS") && !s.mgr.Stop() && sysop.Running("OpenBTS") {
		writeJSON(w, resp(false, 2, "Stop failed, please stop manually."))
		return
	}
	p, _ := gsm.PresetParams(*body.ID, "")
	// Preserve network from profile (config presets carry no iface).
	if saved, ok := s.mgr.LoadProfile(); ok {
		p.Network = saved.Network
	}
	// Apply via manager helper (validates + sqlite argv, no concat).
	if err := applyPresetDB(s.cfg.OpenBTSDbPath, *body.ID); err != nil {
		if strings.Contains(err.Error(), "database") {
			writeJSON(w, resp(false, 3, "Can not find database file."))
			return
		}
		writeJSON(w, resp(false, 0, "Failed."))
		return
	}
	_ = p
	writeJSON(w, resp(true, 1, "Success, please start system manually."))
}

// ---- POST /getconfig ----

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, resp(false, 0, "Failed."))
		return
	}
	rows, err := s.mgr.GetAllConfig()
	if err != nil {
		writeJSON(w, resp(false, 3, "Can not find database file."))
		return
	}
	data := make([][2]string, 0, len(rows))
	data = append(data, rows...)
	writeJSON(w, resp(true, 1, "Success", map[string]any{"data": data}))
}

// ---- POST /allconfig {name,value} ----

func (s *Server) handleAllConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, resp(false, 0, "Failed."))
		return
	}
	var body struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&body); err != nil ||
		body.Name == "" {
		writeJSON(w, resp(false, 0, "Failed."))
		return
	}
	if _, err := os.Stat(s.cfg.OpenBTSDbPath); err != nil {
		writeJSON(w, resp(false, 2, "Can not find database file."))
		return
	}
	if sysop.Running("OpenBTS") && !s.mgr.Stop() && sysop.Running("OpenBTS") {
		writeJSON(w, resp(false, 2, "Stop failed, please stop manually."))
		return
	}
	if err := s.mgr.SetSingleConfig(body.Name, body.Value); err != nil {
		if strings.Contains(err.Error(), "database") {
			writeJSON(w, resp(false, 2, "Can not find database file."))
			return
		}
		writeJSON(w, resp(false, 0, "Failed."))
		return
	}
	writeJSON(w, resp(true, 1, "Success"))
}

// ---- POST /iptables {iface} ----

func (s *Server) handleIptables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, resp(false, 0, "Failed"))
		return
	}
	var body struct {
		Iface string `json:"iface"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&body); err != nil ||
		body.Iface == "" || strings.ContainsAny(body.Iface, " \t\n\r\"';&|<>$`\\") {
		writeJSON(w, resp(false, 0, "Failed"))
		return
	}
	if err := applyIptables(s.cfg.IptablesBin, body.Iface); err != nil {
		writeJSON(w, resp(false, 0, "Failed"))
		return
	}
	writeJSON(w, resp(true, 1, "Success"))
}

// ---- POST /smsinfo ----

func (s *Server) handleSmsInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, resp(false, 0, "Failed"))
		return
	}
	path := s.cfg.LogPath(s.cfg.SmqueueLogName)
	b, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, resp(false, 2, "Can not find /var/log/smqueue.log"))
		return
	}
	sms := parser.ParseSmqueueLog(string(b))
	if len(sms) == 0 {
		writeJSON(w, resp(false, 3, "SMS message not fount."))
		return
	}
	infos := make([][]any, 0, len(sms))
	for _, m := range sms {
		infos = append(infos, []any{
			nilStr(m.Time), nilStr(m.Text), nilStr(m.SenderNumber),
			nilStr(m.SenderIMSI), nilStr(m.ReceiverNumber), nilStr(m.ReceiverIMSI),
		})
	}
	writeJSON(w, resp(true, 1, "Success", map[string]any{"infos": infos}))
}

// ---- POST /ueinfo ----

func (s *Server) handleUeInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, resp(false, 0, "Failed"))
		return
	}
	if !sysop.Running("OpenBTS") {
		writeJSON(w, resp(false, 2, "System is not running."))
		return
	}
	sgsnOut, _ := runCLI(s.cfg.OpenBTSCLIBin, "-c", "sgsn list")
	tmsiOut, _ := runCLI(s.cfg.OpenBTSCLIBin, "-c", "tmsis -l")
	sgsn := parser.ParseSGSN(sgsnOut)
	ues := parser.ParseTMSIs(tmsiOut, sgsn)
	if len(ues) == 0 {
		writeJSON(w, resp(false, 3, "Can not find info."))
		return
	}
	infos := make([][]any, 0, len(ues))
	for _, u := range ues {
		ip := any(nil)
		if u.IP != "" {
			ip = u.IP
		}
		infos = append(infos, []any{u.IMSI, u.IMEI, u.Number, ip})
	}
	writeJSON(w, resp(true, 1, "Success", map[string]any{"infos": infos}))
}

// ---- POST /setphonenumber {imsi,number} ----

func (s *Server) handleSetPhone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, resp(false, 0, "False"))
		return
	}
	var body struct {
		IMSI   string `json:"imsi"`
		Number string `json:"number"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&body); err != nil {
		writeJSON(w, resp(false, 0, "False"))
		return
	}
	if _, err := os.Stat(s.cfg.TMSITablePath); err != nil {
		writeJSON(w, resp(false, 2, "Can not find /var/run/TMSITable.db or /val/lib/asterisk/sqlite3dir/sqlite3.db"))
		return
	}
	if _, err := os.Stat(s.cfg.AsteriskDbPath); err != nil {
		writeJSON(w, resp(false, 2, "Can not find /var/run/TMSITable.db or /val/lib/asterisk/sqlite3dir/sqlite3.db"))
		return
	}
	if err := gsm.ValidateIMSI(body.IMSI); err != nil {
		writeJSON(w, resp(false, 3, "This imsi is not existed."))
		return
	}
	if err := gsm.ValidateNumber(body.Number); err != nil {
		writeJSON(w, resp(false, 0, "False"))
		return
	}
	code, msg := setSubscriberNumber(s.cfg, body.IMSI, body.Number)
	switch code {
	case 1:
		writeJSON(w, resp(true, 1, "Success"))
	case 4:
		writeJSON(w, resp(false, 4, "This imsi is not existed in asterisk."))
	case 3:
		writeJSON(w, resp(false, 3, "This imsi is not existed."))
	default:
		writeJSON(w, resp(false, 0, "False"))
	}
	_ = msg
}

// ---- POST /sendsms {imsi,sender,smsmessage} ----

func (s *Server) handleSendSms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, resp(false, 0, "Send failed."))
		return
	}
	var body struct {
		IMSI       string `json:"imsi"`
		Sender     string `json:"sender"`
		SMSMessage string `json:"smsmessage"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&body); err != nil {
		writeJSON(w, resp(false, 0, "Send failed."))
		return
	}
	if !sysop.Running("OpenBTS") && !sysop.Running("smqueue") {
		// Legacy checks all 5 empty => not running. Keep the OpenBTS gate
		// (cell must be up to deliver).
		if !sysop.Running("OpenBTS") {
			writeJSON(w, resp(false, 2, "Not running."))
			return
		}
	}
	if err := gsm.ValidateIMSI(body.IMSI); err != nil {
		writeJSON(w, resp(false, 0, "Send failed."))
		return
	}
	if body.Sender == "" || body.SMSMessage == "" {
		writeJSON(w, resp(false, 0, "Send failed."))
		return
	}
	out, err := runCLI(s.cfg.OpenBTSCLIBin, "-c",
		fmt.Sprintf("sendsms %s %s %s", body.IMSI, body.Sender, body.SMSMessage))
	if err == nil && strings.Contains(out, "message submitted for delivery") {
		writeJSON(w, resp(true, 1, "Send successfully."))
		return
	}
	writeJSON(w, resp(false, 0, "Send failed."))
}

// ---- GET /healthz + /status + /profile ----

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	det := sdr.Detect()
	st := s.mgr.IsRunning()
	writeJSON(w, map[string]any{
		"ok": true, "running": st.Running, "sdr": det,
		"time": fmt.Sprintf("%s", time.Now().UTC().Format("2006-01-02T15:04:05Z")),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.mgr.IsRunning())
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1(w, r, CodeMethod, "method not allowed, want GET", nil)
		return
	}
	p, ok := s.mgr.LoadProfile()
	ues := listSubscribersBestEffort(s.cfg)
	out := map[string]any{"has_profile": ok, "ues": ues}
	if ok {
		out["profile"] = p
	}
	writeV1(w, r, CodeOK, "ok", out)
}
