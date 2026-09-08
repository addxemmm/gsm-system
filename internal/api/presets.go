package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/addxemmm/gsm-system/internal/gsm"
)

const maxPresetRequestBytes = 16 << 10

type createPresetRequest struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description optionalJSONstring `json:"description"`
	Params      gsm.StartParams    `json:"params"`
}

type updatePresetRequest struct {
	Name        string             `json:"name"`
	Description optionalJSONstring `json:"description"`
	Params      gsm.StartParams    `json:"params"`
}

func (s *Server) handlePresets(w http.ResponseWriter, r *http.Request) {
	if !allow(w, r, http.MethodGet, http.MethodPost) || !requireNoQuery(w, r) {
		return
	}
	if r.Method == http.MethodGet {
		items, err := s.mgr.ListPresets()
		if err != nil {
			s.writePresetStoreError(w, r, "list", err)
			return
		}
		writeV1(w, r, CodeOK, "ok", map[string]any{"items": items})
		return
	}
	var body createPresetRequest
	if code := decodeV1JSON(w, r, &body, maxPresetRequestBytes, true); code != CodeOK {
		writeV1DecodeError(w, r, code)
		return
	}
	preset := gsm.Preset{ID: body.ID, Name: body.Name, Description: body.Description.Value, Params: body.Params}
	if fieldErrors := validatePresetRequest(preset, true); len(fieldErrors) > 0 {
		writeValidation(w, r, fieldErrors)
		return
	}
	if err := s.mgr.CreatePreset(preset); err != nil {
		s.writePresetMutationError(w, r, "create", err)
		return
	}
	w.Header().Set("Location", "/api/v1/presets/"+preset.ID)
	writeV1Status(w, r, http.StatusCreated, CodeOK, "preset created", preset)
}

func (s *Server) routePreset(w http.ResponseWriter, r *http.Request) bool {
	const prefix = "/api/v1/presets/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	id := strings.TrimPrefix(r.URL.Path, prefix)
	if id == "" || strings.Contains(id, "/") {
		writeV1(w, r, CodeNotFound, "not found", nil)
		return true
	}
	if !gsm.ValidPresetID(id) {
		writeValidation(w, r, []FieldError{{Field: "id", Reason: "must match [a-z0-9][a-z0-9_-]{0,63}"}})
		return true
	}
	if !allow(w, r, http.MethodGet, http.MethodPut, http.MethodDelete) || !requireNoQuery(w, r) {
		return true
	}
	switch r.Method {
	case http.MethodGet:
		preset, err := s.mgr.GetPreset(id)
		if err != nil {
			s.writePresetReadError(w, r, "get", err)
			return true
		}
		writeV1(w, r, CodeOK, "ok", preset)
	case http.MethodPut:
		var body updatePresetRequest
		if code := decodeV1JSON(w, r, &body, maxPresetRequestBytes, true); code != CodeOK {
			writeV1DecodeError(w, r, code)
			return true
		}
		preset := gsm.Preset{ID: id, Name: body.Name, Description: body.Description.Value, Params: body.Params}
		if fieldErrors := validatePresetRequest(preset, false); len(fieldErrors) > 0 {
			writeValidation(w, r, fieldErrors)
			return true
		}
		if err := s.mgr.UpdatePreset(id, preset); err != nil {
			s.writePresetMutationError(w, r, "update", err)
			return true
		}
		writeV1(w, r, CodeOK, "preset updated", preset)
	case http.MethodDelete:
		if err := s.mgr.DeletePreset(id); err != nil {
			s.writePresetMutationError(w, r, "delete", err)
			return true
		}
		writeV1(w, r, CodeOK, "preset deleted", map[string]any{"deleted": true})
	}
	return true
}

func validatePresetRequest(preset gsm.Preset, includeID bool) []FieldError {
	var fieldErrors []FieldError
	if includeID && !gsm.ValidPresetID(preset.ID) {
		fieldErrors = append(fieldErrors, FieldError{Field: "id", Reason: "must match [a-z0-9][a-z0-9_-]{0,63}"})
	}
	if strings.TrimSpace(preset.Name) == "" {
		fieldErrors = append(fieldErrors, FieldError{Field: "name", Reason: "required"})
	} else if len(preset.Name) > 128 {
		fieldErrors = append(fieldErrors, FieldError{Field: "name", Reason: "must contain at most 128 bytes"})
	}
	if len(preset.Description) > 2048 {
		fieldErrors = append(fieldErrors, FieldError{Field: "description", Reason: "must contain at most 2048 bytes"})
	}
	for _, issue := range preset.Params.ValidateDetailed() {
		fieldErrors = append(fieldErrors, FieldError{Field: "params." + issue.Field, Reason: issue.Reason})
	}
	return fieldErrors
}

// optionalJSONstring distinguishes omission (the empty default) from an
// explicitly invalid null while keeping description optional for clients.
type optionalJSONstring struct {
	Value string
}

func (v *optionalJSONstring) UnmarshalJSON(b []byte) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return errors.New("must be a string")
	}
	if err := json.Unmarshal(b, &v.Value); err != nil {
		return fmt.Errorf("must be a string: %w", err)
	}
	return nil
}

func (s *Server) writePresetReadError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	if errors.Is(err, gsm.ErrPresetNotFound) {
		writeV1(w, r, CodeNotFound, "preset not found", nil)
		return
	}
	s.writePresetStoreError(w, r, operation, err)
}

func (s *Server) writePresetMutationError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, gsm.ErrPresetNotFound):
		writeV1(w, r, CodeNotFound, "preset not found", nil)
	case errors.Is(err, gsm.ErrPresetExists):
		writeV1(w, r, CodeConflict, "preset already exists", nil)
	case errors.Is(err, gsm.ErrPresetInvalid):
		writeValidation(w, r, []FieldError{{Field: "preset", Reason: err.Error()}})
	default:
		s.writePresetStoreError(w, r, operation, err)
	}
}

func (s *Server) writePresetStoreError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	log.Printf("rid=%s preset %s failed: %v", RequestID(r), operation, err)
	writeV1(w, r, CodeInternal, "preset store unavailable", nil)
}

// presentString retains whether a JSON name was supplied, including when its
// value is empty, so preset_id cannot be mixed with even one explicit field.
type presentString struct {
	Value string
	Set   bool
}

func (v *presentString) UnmarshalJSON(b []byte) error {
	v.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		v.Value = ""
		return nil
	}
	if err := json.Unmarshal(b, &v.Value); err != nil {
		return fmt.Errorf("must be a string: %w", err)
	}
	return nil
}

type cellStartRequest struct {
	PresetID  presentString `json:"preset_id"`
	ARFCNs    presentString `json:"arfcns"`
	C0        presentString `json:"c0"`
	Band      presentString `json:"band"`
	MCC       presentString `json:"mcc"`
	MNC       presentString `json:"mnc"`
	LAC       presentString `json:"lac"`
	CI        presentString `json:"ci"`
	ShortName presentString `json:"short_name"`
	Network   presentString `json:"network"`
}

func (body cellStartRequest) hasExplicitParams() bool {
	return body.ARFCNs.Set || body.C0.Set || body.Band.Set || body.MCC.Set || body.MNC.Set ||
		body.LAC.Set || body.CI.Set || body.ShortName.Set || body.Network.Set
}

func (body cellStartRequest) params() gsm.StartParams {
	return gsm.StartParams{
		ARFCNs: body.ARFCNs.Value, C0: body.C0.Value, Band: body.Band.Value,
		MCC: body.MCC.Value, MNC: body.MNC.Value, LAC: body.LAC.Value,
		CI: body.CI.Value, ShortName: body.ShortName.Value, Network: body.Network.Value,
	}
}
