package api

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/addxemmm/gsm-system/internal/config"
	"github.com/addxemmm/gsm-system/internal/gsm"
)

type artifactState struct {
	exists bool
	data   []byte
}

func localLoopbackInterface(t *testing.T) string {
	t.Helper()
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 && gsm.ValidInterfaceName(iface.Name) {
			return iface.Name
		}
	}
	// Non-Linux test hosts skip the /sys/class/net existence check. Keep the
	// fallback valid and loopback-shaped rather than assuming Linux's eth0.
	return "lo"
}

func validCellStartParams(t *testing.T) gsm.StartParams {
	t.Helper()
	return gsm.StartParams{
		ARFCNs: "1", C0: "55", Band: "900", MCC: "001", MNC: "01",
		LAC: "1", CI: "1", ShortName: "test", Network: localLoopbackInterface(t),
	}
}

func newCellStartModeFixture(t *testing.T, profile []byte) (config.Config, *Server, []string) {
	t.Helper()
	cfg, _ := testServer(t)
	cfg.UHDFindBin = filepath.Join(t.TempDir(), "missing-uhd-find-devices")
	mgr := gsm.New(cfg)

	// Make built-in preset 0 portable on Linux CI by assigning the host's real
	// loopback interface before taking the immutability baseline.
	preset, err := mgr.GetPreset("0")
	if err != nil {
		t.Fatal(err)
	}
	preset.Params.Network = localLoopbackInterface(t)
	if err := mgr.UpdatePreset("0", preset); err != nil {
		t.Fatal(err)
	}

	if profile != nil {
		if err := os.WriteFile(mgr.ProfilePath(), profile, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	databasePaths := []string{cfg.OpenBTSDbPath, cfg.AsteriskDbPath, cfg.TMSITablePath}
	for i, path := range databasePaths {
		if err := os.WriteFile(path, []byte(fmt.Sprintf("database-marker-%d", i)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	artifacts := append([]string{mgr.ProfilePath(), filepath.Join(cfg.DataDir, "presets.json")}, databasePaths...)
	return cfg, New(cfg, mgr), artifacts
}

func snapshotArtifacts(t *testing.T, paths []string) map[string]artifactState {
	t.Helper()
	result := make(map[string]artifactState, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			result[path] = artifactState{exists: true, data: append([]byte(nil), data...)}
		case os.IsNotExist(err):
			result[path] = artifactState{}
		default:
			t.Fatal(err)
		}
	}
	return result
}

func assertArtifactsUnchanged(t *testing.T, paths []string, before map[string]artifactState) {
	t.Helper()
	after := snapshotArtifacts(t, paths)
	if !reflect.DeepEqual(after, before) {
		for _, path := range paths {
			if after[path].exists != before[path].exists || !bytes.Equal(after[path].data, before[path].data) {
				t.Errorf("start request changed %s", path)
			}
		}
	}
}

func validProfileJSON(t *testing.T) []byte {
	t.Helper()
	p := validCellStartParams(t)
	return []byte(fmt.Sprintf(
		`{"arfcns":"%s","c0":"%s","band":"%s","mcc":"%s","mnc":"%s","lac":"%s","ci":"%s","short_name":"%s","network":"%s"}`,
		p.ARFCNs, p.C0, p.Band, p.MCC, p.MNC, p.LAC, p.CI, p.ShortName, p.Network,
	))
}

func TestCellStartModesReachHardwareCheckWithoutMutatingState(t *testing.T) {
	profile := validProfileJSON(t)
	_, server, artifacts := newCellStartModeFixture(t, profile)
	params := validCellStartParams(t)
	fullCustom := fmt.Sprintf(
		`{"arfcns":"1","c0":"55","band":"900","mcc":"001","mnc":"01","lac":"1","ci":"1","short_name":"test","network":"%s"}`,
		params.Network,
	)

	for _, test := range []struct {
		name string
		body string
	}{
		{name: "saved profile", body: `{}`},
		{name: "full custom", body: fullCustom},
		{name: "preset zero", body: `{"preset_id":"0"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := snapshotArtifacts(t, artifacts)
			assertCode(t, serve(t, server, http.MethodPost, "/api/v1/cell", "application/json", test.body), http.StatusServiceUnavailable, CodeNoHardware)
			assertArtifactsUnchanged(t, artifacts, before)
		})
	}
}

func TestCellStartSavedProfileNeverFillsPartialExplicitInput(t *testing.T) {
	_, server, artifacts := newCellStartModeFixture(t, validProfileJSON(t))
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "partial", body: `{"band":"900"}`},
		{name: "empty", body: `{"network":""}`},
		{name: "null", body: `{"network":null}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := snapshotArtifacts(t, artifacts)
			assertCode(t, serve(t, server, http.MethodPost, "/api/v1/cell", "application/json", test.body), http.StatusUnprocessableEntity, CodeInvalid)
			assertArtifactsUnchanged(t, artifacts, before)
		})
	}
}

func TestCellStartPresetIDIsExclusiveByPresenceForEveryCustomField(t *testing.T) {
	_, server, artifacts := newCellStartModeFixture(t, validProfileJSON(t))
	fields := []string{"arfcns", "c0", "band", "mcc", "mnc", "lac", "ci", "short_name", "network"}
	for _, field := range fields {
		for _, value := range []string{`""`, `null`} {
			name := field + "/empty"
			if value == "null" {
				name = field + "/null"
			}
			t.Run(name, func(t *testing.T) {
				before := snapshotArtifacts(t, artifacts)
				body := fmt.Sprintf(`{"preset_id":"0","%s":%s}`, field, value)
				assertCode(t, serve(t, server, http.MethodPost, "/api/v1/cell", "application/json", body), http.StatusUnprocessableEntity, CodeInvalid)
				assertArtifactsUnchanged(t, artifacts, before)
			})
		}
	}
}

func TestCellStartRejectsEmptyAndNullPresetIDWithoutMutation(t *testing.T) {
	_, server, artifacts := newCellStartModeFixture(t, validProfileJSON(t))
	for _, body := range []string{`{"preset_id":""}`, `{"preset_id":null}`} {
		before := snapshotArtifacts(t, artifacts)
		assertCode(t, serve(t, server, http.MethodPost, "/api/v1/cell", "application/json", body), http.StatusUnprocessableEntity, CodeInvalid)
		assertArtifactsUnchanged(t, artifacts, before)
	}
}

func TestCellStartEmptyObjectRejectsMissingCorruptAndIncompleteProfiles(t *testing.T) {
	profiles := []struct {
		name string
		data []byte
	}{
		{name: "missing", data: nil},
		{name: "corrupt", data: []byte(`{"band":`)},
		{name: "semantically incomplete", data: []byte(`{"band":"900","mcc":"001","mnc":"01","network":"lo"}`)},
	}
	for _, test := range profiles {
		t.Run(test.name, func(t *testing.T) {
			_, server, artifacts := newCellStartModeFixture(t, test.data)
			before := snapshotArtifacts(t, artifacts)
			assertCode(t, serve(t, server, http.MethodPost, "/api/v1/cell", "application/json", `{}`), http.StatusUnprocessableEntity, CodeInvalid)
			assertArtifactsUnchanged(t, artifacts, before)
		})
	}
}
