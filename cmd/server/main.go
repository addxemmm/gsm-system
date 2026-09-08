package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/addxemmm/gsm-system/internal/api"
	"github.com/addxemmm/gsm-system/internal/config"
	"github.com/addxemmm/gsm-system/internal/gsm"
	"github.com/addxemmm/gsm-system/internal/healthcheck"
	"github.com/addxemmm/gsm-system/internal/logsink"
)

var version = "2.1.0"
var revision = "unknown"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit / 显示版本后退出")
	probe := flag.Bool("healthcheck", false, "check management and cell readiness without RF / 只读检查管理及小区状态")
	flag.Parse()
	if *showVersion {
		fmt.Printf("gsm-system %s (%s)\n", version, revision)
		return
	}
	cfgPath := os.Getenv("GSM_CONFIG")
	if cfgPath == "" {
		for _, cand := range []string{"/app/configs/app.yaml", "configs/app.yaml", "/data/app.yaml"} {
			if _, err := os.Stat(cand); err == nil {
				cfgPath = cand
				break
			}
		}
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	cfg.Version, cfg.Revision = version, revision
	if *probe {
		if err := healthcheck.Check(context.Background(), cfg.ListenAddr, os.Getenv("GSM_API_TOKEN")); err != nil {
			log.Printf("healthcheck: %v", err)
			os.Exit(1)
		}
		return
	}
	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("ensure dirs: %v", err)
	}
	sink, err := logsink.StartWithSmqueueLog(cfg.SyslogSocket, cfg.LogDir, cfg.SmqueueLogName)
	if err != nil {
		log.Fatalf("start native log receiver: %v", err)
	}
	defer sink.Close()
	mgr := gsm.New(cfg)
	if strings.TrimSpace(os.Getenv("GSM_API_TOKEN")) == "" {
		log.Printf("WARNING: GSM_API_TOKEN unset, API is open (LAN-only deployment required)")
	}
	requestContext, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	srv := &http.Server{
		BaseContext:       func(net.Listener) context.Context { return requestContext },
		Addr:              cfg.ListenAddr,
		Handler:           api.New(cfg, mgr).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		log.Printf("gsm-system listening on %s (data=%s)", cfg.ListenAddr, cfg.DataDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancelRequests()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	mgr.Stop()
}
