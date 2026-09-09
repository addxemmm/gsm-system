package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/addxemmm/gsm-system/internal/gsm"
)

func TestCellStartNetworkFailureIs503AndStopRemainsAvailable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("process fixture is Linux-only")
	}
	dir := t.TempDir()
	cfg, _ := testServer(t)
	cfg.UHDFindBin = writeAPIShell(t, dir, "uhd-fixture", "echo 'type: B210'\n")
	cfg.IptablesBin = writeAPIShell(t, dir, "iptables-failure", "if [ \"$3\" = -C ]; then exit 1; fi\necho denied >&2\nexit 42\n")
	server := New(cfg, gsm.New(cfg))
	p := validCellStartParams(t)
	body := `{"arfcns":"1","c0":"55","band":"900","mcc":"001","mnc":"01","lac":"1","ci":"1","short_name":"test","network":"` + p.Network + `"}`

	start := serve(t, server, http.MethodPost, "/api/v1/cell", "application/json", body)
	assertCode(t, start, http.StatusServiceUnavailable, CodeNoHardware)
	var envelope Envelope
	if err := json.Unmarshal(start.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Message != "network rule setup unavailable" {
		t.Fatalf("message=%q", envelope.Message)
	}

	stop := serve(t, server, http.MethodDelete, "/api/v1/cell", "", "")
	assertCode(t, stop, http.StatusOK, CodeOK)
}

func writeAPIShell(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+strings.TrimSpace(body)+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
