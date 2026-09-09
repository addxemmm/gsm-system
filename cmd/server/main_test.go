package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/addxemmm/gsm-system/internal/config"
)

func TestDualListenersServeInOneGroupAndShutdown(t *testing.T) {
	cfg := config.Default()
	cfg.ListenAddr = "127.0.0.1:0"
	cfg.WebListen = "127.0.0.1:0"
	cfg.WebEnabled = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	group, err := bindHTTPServers(ctx, cfg,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "api") }),
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "web") }),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer group.close()
	if len(group.bound) != 2 {
		t.Fatalf("listeners=%d", len(group.bound))
	}
	_ = group.serve()
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	for i, want := range []string{"api", "web"} {
		resp, err := client.Get("http://" + group.bound[i].listener.Addr().String() + "/")
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil || resp.StatusCode != http.StatusOK || string(body) != want {
			t.Fatalf("listener=%d status=%d body=%q err=%v", i, resp.StatusCode, body, readErr)
		}
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	if err := group.shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
}

func TestSecondListenerConflictClosesFirst(t *testing.T) {
	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reserved.Addr().String()
	_ = reserved.Close()

	cfg := config.Default()
	cfg.ListenAddr = address
	cfg.WebListen = address
	cfg.WebEnabled = true
	group, err := bindHTTPServers(context.Background(), cfg, http.NotFoundHandler(), http.NotFoundHandler())
	if err == nil || group != nil {
		if group != nil {
			group.close()
		}
		t.Fatalf("expected port conflict, group=%v err=%v", group, err)
	}
	// The API listener acquired immediately before the Web conflict must have
	// been closed, so another process can bind without waiting for cleanup.
	rebound, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("first listener leaked after startup failure: %v", err)
	}
	_ = rebound.Close()
}

func TestExplicitWebDisableBindsAPIOnly(t *testing.T) {
	cfg := config.Default()
	cfg.ListenAddr = "127.0.0.1:0"
	cfg.WebListen = "not-a-listen-address"
	cfg.WebEnabled = false
	group, err := bindHTTPServers(context.Background(), cfg, http.NotFoundHandler(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer group.close()
	if len(group.bound) != 1 || group.bound[0].name != "API" {
		t.Fatalf("bound=%+v", group.bound)
	}
}
