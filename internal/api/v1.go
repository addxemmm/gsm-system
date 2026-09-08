package api

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/addxemmm/gsm-system/internal/gsm"
	"github.com/addxemmm/gsm-system/internal/parser"
	"github.com/addxemmm/gsm-system/internal/subscriber"
	"github.com/addxemmm/gsm-system/internal/telephony"
)

func (s *Server) serveV1(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v1/cell":
		if !allow(w, r, http.MethodGet, http.MethodPost, http.MethodDelete) || !requireNoQuery(w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeV1(w, r, CodeOK, "ok", s.mgr.IsRunning())
		case http.MethodPost:
			s.handleCellStart(w, r)
		case http.MethodDelete:
			s.handleCellStop(w, r)
		}
	case "/api/v1/config":
		if !allow(w, r, http.MethodGet, http.MethodPatch) || !requireNoQuery(w, r) {
			return
		}
		if r.Method == http.MethodGet {
			s.handleConfigGet(w, r)
		} else {
			s.handleConfigPatch(w, r)
		}
	case "/api/v1/profile":
		if allow(w, r, http.MethodGet) && requireNoQuery(w, r) {
			s.handleProfileGet(w, r)
		}
	case "/api/v1/health":
		if allow(w, r, http.MethodGet) && requireNoQuery(w, r) {
			s.handleHealth(w, r)
		}
	case "/api/v1/connections":
		if allow(w, r, http.MethodGet) {
			s.handleConnections(w, r)
		}
	case "/api/v1/subscribers":
		if allow(w, r, http.MethodGet) {
			s.handleSubscribers(w, r)
		}
	case "/api/v1/sms":
		if !allow(w, r, http.MethodGet, http.MethodPost) {
			return
		}
		if r.Method == http.MethodGet {
			s.handleSMSList(w, r)
		} else if requireNoQuery(w, r) {
			s.handleSMSSend(w, r)
		}
	case "/api/v1/calls":
		if allow(w, r, http.MethodGet) {
			s.handleCalls(w, r)
		}
	case "/api/v1/calls/history":
		if allow(w, r, http.MethodGet) {
			s.handleCallHistory(w, r)
		}
	case "/api/v1/network":
		if !allow(w, r, http.MethodGet, http.MethodPut) {
			return
		}
		if r.Method == http.MethodGet {
			s.handleNetworkGet(w, r)
		} else if requireNoQuery(w, r) {
			s.handleNetworkPut(w, r)
		}
	default:
		if strings.HasPrefix(r.URL.Path, "/api/v1/subscribers/") && s.routeSubscriber(w, r) {
			return
		}
		writeV1(w, r, CodeNotFound, "not found", nil)
	}
}

func allow(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	for _, method := range methods {
		if r.Method == method {
			return true
		}
	}
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeV1(w, r, CodeMethod, "method not allowed", nil)
	return false
}

func (s *Server) handleCellStart(w http.ResponseWriter, r *http.Request) {
	var params gsm.StartParams
	if code := decodeV1JSON(w, r, &params, 1<<20, true); code != CodeOK {
		writeV1DecodeError(w, r, code)
		return
	}
	if params == (gsm.StartParams{}) {
		s.mgr.OverlayProfile(&params)
	}
	if issues := params.ValidateDetailed(); len(issues) > 0 {
		fieldErrors := make([]FieldError, 0, len(issues))
		for _, issue := range issues {
			fieldErrors = append(fieldErrors, FieldError{Field: issue.Field, Reason: issue.Reason})
		}
		writeValidation(w, r, fieldErrors)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	if err := s.mgr.Start(ctx, params); err != nil {
		switch {
		case errors.Is(err, gsm.ErrCellRunning):
			writeV1(w, r, CodeConflict, "cell already running or transitioning", nil)
		case strings.Contains(err.Error(), "not connected"):
			writeV1(w, r, CodeNoHardware, "no SDR device attached", nil)
		default:
			log.Printf("rid=%s cell start failed: %v", RequestID(r), err)
			writeV1(w, r, CodeInternal, "cell start failed", nil)
		}
		return
	}
	writeV1(w, r, CodeOK, "cell started", map[string]any{
		"band": params.Band, "mcc": params.MCC, "mnc": params.MNC, "short_name": params.ShortName,
	})
}

func (s *Server) handleCellStop(w http.ResponseWriter, r *http.Request) {
	before := s.mgr.IsRunning()
	if before.State == "stopped" {
		writeV1(w, r, CodeOK, "already stopped", map[string]any{"stopped": false})
		return
	}
	s.mgr.Stop()
	after := s.mgr.IsRunning()
	if after.State == "stopped" {
		writeV1(w, r, CodeOK, "cell stopped", map[string]any{"stopped": true})
		return
	}
	writeV1(w, r, CodeInternal, "cell stop incomplete", map[string]any{"cell": after})
}

func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	rows, err := s.mgr.GetAllConfig()
	if err != nil {
		if errors.Is(err, gsm.ErrDatabaseNotFound) {
			writeV1(w, r, CodeNotFound, "OpenBTS database not found", nil)
			return
		}
		log.Printf("rid=%s config query failed: %v", RequestID(r), err)
		writeV1(w, r, CodeInternal, "config query failed", nil)
		return
	}
	items := make([]map[string]string, 0, len(rows))
	for _, keyValue := range rows {
		items = append(items, map[string]string{"key": keyValue[0], "value": keyValue[1]})
	}
	sort.Slice(items, func(i, j int) bool { return items[i]["key"] < items[j]["key"] })
	writeV1(w, r, CodeOK, "ok", map[string]any{"config": items})
}

func (s *Server) handleConfigPatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Values map[string]string `json:"values"`
	}
	if code := decodeV1JSON(w, r, &body, 64<<10, true); code != CodeOK {
		writeV1DecodeError(w, r, code)
		return
	}
	if len(body.Values) == 0 {
		writeValidation(w, r, []FieldError{{Field: "values", Reason: "must contain at least one key"}})
		return
	}
	if err := s.mgr.UpdateConfig(body.Values); err != nil {
		switch {
		case errors.Is(err, gsm.ErrCellRunning):
			writeV1(w, r, CodeConflict, "stop the cell before changing config", nil)
		case errors.Is(err, gsm.ErrDatabaseNotFound):
			writeV1(w, r, CodeNotFound, "OpenBTS database not found", nil)
		case errors.Is(err, gsm.ErrInvalidConfig):
			writeValidation(w, r, []FieldError{{Field: "values", Reason: err.Error()}})
		default:
			log.Printf("rid=%s config update failed: %v", RequestID(r), err)
			writeV1(w, r, CodeInternal, "config update failed", nil)
		}
		return
	}
	updated := make([]string, 0, len(body.Values))
	for key := range body.Values {
		updated = append(updated, key)
	}
	sort.Strings(updated)
	writeV1(w, r, CodeOK, "config updated", map[string]any{"updated": updated})
}

func (s *Server) handleProfileGet(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.mgr.LoadProfile()
	data := map[string]any{"has_profile": ok}
	if ok {
		data["profile"] = profile
	}
	writeV1(w, r, CodeOK, "ok", data)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeV1(w, r, CodeOK, "ok", map[string]any{
		"ok": true, "version": s.cfg.Version, "revision": s.cfg.Revision,
		"cell": s.mgr.IsRunning(), "time": time.Now().UTC().Format(time.RFC3339),
	})
}

func pageForRequest(w http.ResponseWriter, r *http.Request) (pageRequest, bool) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeValidation(w, r, []FieldError{{Field: "query", Reason: "malformed query string"}})
		return pageRequest{}, false
	}
	page, fieldErrors := parsePageQuery(query)
	if len(fieldErrors) != 0 {
		writeValidation(w, r, fieldErrors)
		return pageRequest{}, false
	}
	return page, true
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	page, ok := pageForRequest(w, r)
	if !ok {
		return
	}
	if !s.mgr.IsRunning().OpenBTS {
		writeV1(w, r, CodePrecondition, "OpenBTS is not running", nil)
		return
	}
	sgsnOutput, err := runCLIContext(r.Context(), s.cfg.OpenBTSCLIBin, "-c", "sgsn list")
	if err != nil {
		log.Printf("rid=%s sgsn query failed: %v", RequestID(r), err)
		writeV1(w, r, CodeInternal, "connection query failed", nil)
		return
	}
	tmsiOutput, err := runCLIContext(r.Context(), s.cfg.OpenBTSCLIBin, "-c", "tmsis -l")
	if err != nil {
		log.Printf("rid=%s tmsi query failed: %v", RequestID(r), err)
		writeV1(w, r, CodeInternal, "connection query failed", nil)
		return
	}
	connections := parser.ParseTMSIs(tmsiOutput, parser.ParseSGSN(sgsnOutput))
	items := make([]map[string]any, 0, len(connections))
	for _, connection := range connections {
		items = append(items, map[string]any{
			"imsi": connection.IMSI, "imei": nilStr(connection.IMEI),
			"number": nilStr(connection.Number), "ip": nilStr(connection.IP),
		})
	}
	paged := pageSlice(items, page)
	writeV1(w, r, CodeOK, "ok", map[string]any{
		"connections": paged, "count": len(paged), "total": len(items),
		"limit": page.Limit, "offset": page.Offset, "source": "openbts_tmsi_sgsn",
		"window": "current_snapshot", "truncated": false,
	})
}

func (s *Server) subscriberStore() subscriber.Store {
	return subscriber.Store{SQLiteBin: s.cfg.Sqlite3Bin, AsteriskDB: s.cfg.AsteriskDbPath, TMSIDB: s.cfg.TMSITablePath}
}

func (s *Server) handleSubscribers(w http.ResponseWriter, r *http.Request) {
	page, ok := pageForRequest(w, r)
	if !ok {
		return
	}
	items, err := s.subscriberStore().List(r.Context())
	if err != nil {
		s.writeSubscriberError(w, r, err, "subscriber query failed")
		return
	}
	paged := pageSlice(items, page)
	writeV1(w, r, CodeOK, "ok", map[string]any{
		"subscribers": paged, "count": len(paged), "total": len(items),
		"limit": page.Limit, "offset": page.Offset, "source": "asterisk_registry",
		"window": "full", "truncated": false,
	})
}

func (s *Server) routeSubscriber(w http.ResponseWriter, r *http.Request) bool {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/subscribers/")
	parts := strings.Split(rest, "/")
	if len(parts) == 1 && parts[0] != "" {
		if !allow(w, r, http.MethodGet) || !requireNoQuery(w, r) {
			return true
		}
		if err := gsm.ValidateIMSI(parts[0]); err != nil {
			writeValidation(w, r, []FieldError{{Field: "imsi", Reason: err.Error()}})
			return true
		}
		item, err := s.subscriberStore().Get(r.Context(), parts[0])
		if err != nil {
			s.writeSubscriberError(w, r, err, "subscriber query failed")
			return true
		}
		writeV1(w, r, CodeOK, "ok", map[string]any{"subscriber": item})
		return true
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] == "number" {
		if !allow(w, r, http.MethodPut, http.MethodDelete) || !requireNoQuery(w, r) {
			return true
		}
		imsi := parts[0]
		if err := gsm.ValidateIMSI(imsi); err != nil {
			writeValidation(w, r, []FieldError{{Field: "imsi", Reason: err.Error()}})
			return true
		}
		if r.Method == http.MethodPut {
			s.handleSubscriberBind(w, r, imsi)
		} else {
			s.handleSubscriberUnbind(w, r, imsi)
		}
		return true
	}
	return false
}

func (s *Server) handleSubscriberBind(w http.ResponseWriter, r *http.Request, imsi string) {
	var body struct {
		Number string `json:"number"`
	}
	if code := decodeV1JSON(w, r, &body, 64<<10, true); code != CodeOK {
		writeV1DecodeError(w, r, code)
		return
	}
	if err := validateBindingNumber(body.Number); err != nil {
		writeValidation(w, r, []FieldError{{Field: "number", Reason: err.Error()}})
		return
	}
	result, err := s.subscriberStore().Bind(r.Context(), imsi, body.Number)
	if err != nil {
		s.writeSubscriberError(w, r, err, "subscriber update failed")
		return
	}
	writeV1(w, r, CodeOK, "subscriber number bound", result)
}

func validateBindingNumber(number string) error {
	if err := gsm.ValidateNumber(number); err != nil {
		return err
	}
	switch number {
	case "111", "112", "911":
		return errors.New("reserved service number")
	default:
		return nil
	}
}

func (s *Server) handleSubscriberUnbind(w http.ResponseWriter, r *http.Request, imsi string) {
	result, err := s.subscriberStore().Unbind(r.Context(), imsi)
	if err != nil {
		s.writeSubscriberError(w, r, err, "subscriber update failed")
		return
	}
	writeV1(w, r, CodeOK, "subscriber number unbound", result)
}

func (s *Server) writeSubscriberError(w http.ResponseWriter, r *http.Request, err error, message string) {
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, subscriber.ErrNotFound):
		writeV1(w, r, CodeNotFound, "subscriber registration not found", nil)
	case errors.Is(err, subscriber.ErrConflict):
		writeV1(w, r, CodeConflict, "number is already bound", nil)
	case errors.Is(err, subscriber.ErrInconsistent):
		writeV1(w, r, CodeConflict, "subscriber registry is inconsistent", nil)
	default:
		log.Printf("rid=%s %s: %v", RequestID(r), message, err)
		writeV1(w, r, CodeInternal, message, nil)
	}
}

func (s *Server) handleSMSList(w http.ResponseWriter, r *http.Request) {
	page, ok := pageForRequest(w, r)
	if !ok {
		return
	}
	path := s.cfg.LogPath(s.cfg.SmqueueLogName)
	content, window, err := readTail(path, s.cfg.MaxHistoryBytes)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeV1(w, r, CodeNotFound, "smqueue log not found", nil)
		} else {
			log.Printf("rid=%s smqueue log read failed: %v", RequestID(r), err)
			writeV1(w, r, CodeInternal, "smqueue log read failed", nil)
		}
		return
	}
	messages := parser.ParseSmqueueLog(string(content))
	items := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		items = append(items, map[string]any{
			"time": nilStr(message.Time), "text": nilStr(message.Text),
			"sender_number": nilStr(message.SenderNumber), "sender_imsi": nilStr(message.SenderIMSI),
			"receiver_number": nilStr(message.ReceiverNumber), "receiver_imsi": nilStr(message.ReceiverIMSI),
		})
	}
	paged := pageSlice(items, page)
	writeV1(w, r, CodeOK, "ok", map[string]any{
		"sms": paged, "count": len(paged), "total": len(items),
		"limit": page.Limit, "offset": page.Offset, "source": filepath.Base(path),
		"window": window, "truncated": window.Truncated,
	})
}

func (s *Server) handleSMSSend(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IMSI   string `json:"imsi"`
		Sender string `json:"sender"`
		Text   string `json:"text"`
	}
	if code := decodeV1JSON(w, r, &body, 64<<10, true); code != CodeOK {
		writeV1DecodeError(w, r, code)
		return
	}
	var fieldErrors []FieldError
	if err := gsm.ValidateIMSI(body.IMSI); err != nil {
		fieldErrors = append(fieldErrors, FieldError{Field: "imsi", Reason: err.Error()})
	}
	if err := gsm.ValidateNumber(body.Sender); err != nil {
		fieldErrors = append(fieldErrors, FieldError{Field: "sender", Reason: err.Error()})
	}
	if reason := validateSMSText(body.Text); reason != "" {
		fieldErrors = append(fieldErrors, FieldError{Field: "text", Reason: reason})
	}
	if len(fieldErrors) > 0 {
		writeValidation(w, r, fieldErrors)
		return
	}
	state := s.mgr.IsRunning()
	if !state.SMSReady {
		writeV1(w, r, CodePrecondition, "SMS services are not ready", map[string]any{"cell": state})
		return
	}
	output, err := runCLIContext(r.Context(), s.cfg.OpenBTSCLIBin, "-c",
		"sendsms "+body.IMSI+" "+body.Sender+" \""+body.Text+"\"")
	if err != nil {
		// The CLI may echo the submitted command; do not copy its error/output
		// into logs or the API response because it can contain message content.
		log.Printf("rid=%s sendsms CLI failed", RequestID(r))
		writeV1(w, r, CodeInternal, "SMS submission failed", nil)
		return
	}
	if !strings.Contains(output, "message submitted for delivery") {
		log.Printf("rid=%s sendsms submission rejected", RequestID(r))
		writeV1(w, r, CodeInternal, "SMS submission rejected", nil)
		return
	}
	writeV1Status(w, r, http.StatusAccepted, CodeOK, "sms submitted",
		map[string]any{"status": "submitted", "imsi": body.IMSI})
}

func validateSMSText(text string) string {
	if text == "" {
		return "required"
	}
	if len(text) > 159 {
		return "must contain at most 159 bytes"
	}
	const safe = "@!#$%&()*+,-./0123456789:;<=>?ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz "
	for _, character := range text {
		if !strings.ContainsRune(safe, character) {
			return "contains a character outside the safe ASCII/GSM default-basic intersection"
		}
	}
	return ""
}

func (s *Server) handleCalls(w http.ResponseWriter, r *http.Request) {
	page, ok := pageForRequest(w, r)
	if !ok {
		return
	}
	if !s.mgr.IsRunning().Asterisk {
		writeV1(w, r, CodePrecondition, "Asterisk is not running", nil)
		return
	}
	calls, err := telephony.Active(r.Context(), s.cfg.AsteriskBin)
	if err != nil {
		log.Printf("rid=%s active channel query failed: %v", RequestID(r), err)
		writeV1(w, r, CodeNoHardware, "Asterisk channel query unavailable", nil)
		return
	}
	paged := pageSlice(calls, page)
	writeV1(w, r, CodeOK, "ok", map[string]any{
		"calls": paged, "count": len(paged), "total": len(calls),
		"limit": page.Limit, "offset": page.Offset, "source": "asterisk_active_channels",
		"window": "current_snapshot", "truncated": false,
	})
}

func (s *Server) handleCallHistory(w http.ResponseWriter, r *http.Request) {
	page, ok := pageForRequest(w, r)
	if !ok {
		return
	}
	calls, window, err := telephony.History(r.Context(), s.cfg.AsteriskCDRPath, s.cfg.MaxHistoryBytes)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeV1(w, r, CodeNotFound, "Asterisk CDR not found", nil)
		} else {
			log.Printf("rid=%s CDR query failed: %v", RequestID(r), err)
			writeV1(w, r, CodeInternal, "Asterisk CDR query failed", nil)
		}
		return
	}
	paged := pageSlice(calls, page)
	writeV1(w, r, CodeOK, "ok", map[string]any{
		"calls": paged, "count": len(paged), "total": len(calls),
		"limit": page.Limit, "offset": page.Offset, "source": filepath.Base(s.cfg.AsteriskCDRPath),
		"window": window, "truncated": window.Truncated,
	})
}

func (s *Server) savedOrExplicitIface(explicit string) (string, error) {
	iface := strings.TrimSpace(explicit)
	if iface == "" {
		profile, ok := s.mgr.LoadProfile()
		if !ok {
			return "", errors.New("iface is required when no saved profile exists")
		}
		iface = strings.TrimSpace(profile.Network)
	}
	if err := validateIface(iface); err != nil {
		return "", err
	}
	return iface, nil
}

func (s *Server) handleNetworkGet(w http.ResponseWriter, r *http.Request) {
	query, queryErr := url.ParseQuery(r.URL.RawQuery)
	if queryErr != nil {
		writeValidation(w, r, []FieldError{{Field: "query", Reason: "malformed query string"}})
		return
	}
	var fieldErrors []FieldError
	for key, values := range query {
		if key != "iface" {
			fieldErrors = append(fieldErrors, FieldError{Field: key, Reason: "unknown query parameter"})
		} else if len(values) != 1 {
			fieldErrors = append(fieldErrors, FieldError{Field: key, Reason: "must be specified once"})
		}
	}
	if len(fieldErrors) != 0 {
		sort.Slice(fieldErrors, func(i, j int) bool { return fieldErrors[i].Field < fieldErrors[j].Field })
		writeValidation(w, r, fieldErrors)
		return
	}
	var iface string
	var err error
	if _, present := query["iface"]; present {
		iface = strings.TrimSpace(query.Get("iface"))
		err = validateIface(iface)
	} else {
		iface, err = s.savedOrExplicitIface("")
	}
	if err != nil {
		writeValidation(w, r, []FieldError{{Field: "iface", Reason: err.Error()}})
		return
	}
	present, err := networkRulePresent(r.Context(), s.cfg.IptablesBin, iface)
	if err != nil {
		log.Printf("rid=%s iptables check failed: %v", RequestID(r), err)
		writeV1(w, r, CodeNoHardware, "network rule query unavailable", nil)
		return
	}
	writeV1(w, r, CodeOK, "ok", map[string]any{
		"iface": iface, "rule_present": present, "persisted": false,
		"ipv4_forwarding": ipv4Forwarding(),
	})
}

func (s *Server) handleNetworkPut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Iface string `json:"iface"`
	}
	if code := decodeV1JSON(w, r, &body, 64<<10, true); code != CodeOK {
		writeV1DecodeError(w, r, code)
		return
	}
	iface := strings.TrimSpace(body.Iface)
	if err := validateIface(iface); err != nil {
		writeValidation(w, r, []FieldError{{Field: "iface", Reason: err.Error()}})
		return
	}
	var changed bool
	err := s.mgr.WithStoppedCell(func() error {
		var operationErr error
		changed, operationErr = ensureNetworkRule(r.Context(), s.cfg.IptablesBin, iface)
		return operationErr
	})
	if err != nil {
		if errors.Is(err, gsm.ErrCellRunning) {
			writeV1(w, r, CodeConflict, "stop the cell before changing network rules", nil)
		} else {
			log.Printf("rid=%s iptables update failed: %v", RequestID(r), err)
			writeV1(w, r, CodeNoHardware, "network rule update unavailable", nil)
		}
		return
	}
	writeV1(w, r, CodeOK, "network rule ready", map[string]any{
		"iface": iface, "rule_present": true, "changed": changed, "persisted": false,
		"ipv4_forwarding": ipv4Forwarding(),
	})
}

func writeValidation(w http.ResponseWriter, r *http.Request, fieldErrors []FieldError) {
	writeV1(w, r, CodeInvalid, "validation failed", map[string]any{"errors": fieldErrors})
}
