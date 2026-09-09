package main

import (
	"context"
	"errors"
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
	"github.com/addxemmm/gsm-system/internal/webui"
)

var version = "2.1.0"
var revision = "unknown"

func main() {
	if err := run(); err != nil {
		log.Printf("gsm-system: %v", err)
		os.Exit(1)
	}
}

func run() error {
	showVersion := flag.Bool("version", false, "print version and exit / 显示版本后退出")
	probe := flag.Bool("healthcheck", false, "check API, Web UI, and cell readiness without RF / 只读检查 API、Web 与小区状态")
	flag.Parse()
	if *showVersion {
		fmt.Printf("gsm-system %s (%s)\n", version, revision)
		return nil
	}

	cfgPath := os.Getenv("GSM_CONFIG")
	if cfgPath == "" {
		for _, candidate := range []string{"/app/configs/app.yaml", "configs/app.yaml", "/data/app.yaml"} {
			if _, err := os.Stat(candidate); err == nil {
				cfgPath = candidate
				break
			}
		}
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	location, err := cfg.Location()
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}
	if err := os.Setenv("TZ", cfg.Timezone); err != nil {
		return fmt.Errorf("set native timezone: %w", err)
	}
	// Set once, before goroutines and native children start / 启动前统一时区。
	time.Local = location
	cfg.Version, cfg.Revision = version, revision

	if *probe {
		if err := healthcheck.Check(context.Background(), cfg.ListenAddr, os.Getenv("GSM_API_TOKEN")); err != nil {
			return fmt.Errorf("API healthcheck: %w", err)
		}
		if cfg.WebEnabled {
			if err := webui.Check(context.Background(), cfg.WebListen); err != nil {
				return fmt.Errorf("Web healthcheck: %w", err)
			}
		}
		return nil
	}

	if err := cfg.EnsureDirs(); err != nil {
		return fmt.Errorf("ensure dirs: %w", err)
	}
	mgr := gsm.New(cfg)

	tokenRequired := strings.TrimSpace(os.Getenv("GSM_API_TOKEN")) != ""
	if !tokenRequired {
		log.Printf("WARNING: GSM_API_TOKEN unset, API is open (LAN-only deployment required)")
	}
	rawAPIHandler := api.New(cfg, mgr).Handler()
	apiHandler := webui.ProtectAPI(rawAPIHandler, cfg.WebPublicOrigin)
	webHandler := webui.New(rawAPIHandler, cfg.Version, cfg.Revision, tokenRequired, cfg.WebPublicOrigin)
	requestContext, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()

	servers, err := bindHTTPServers(requestContext, cfg, apiHandler, webHandler)
	if err != nil {
		return err
	}
	defer servers.close()

	// Bind every HTTP address before starting side receivers or accepting any
	// request. A bad second listener therefore closes the first and exits cleanly.
	sink, err := logsink.StartWithSmqueueLog(cfg.SyslogSocket, cfg.LogDir, cfg.SmqueueLogName)
	if err != nil {
		return fmt.Errorf("start native log receiver: %w", err)
	}
	defer sink.Close()
	// Only a process that reached request serving may own native children.
	// Startup failures above must not run Manager.Stop: another instance may
	// already own the configured OpenBTS processes when a port is in use.
	defer mgr.Stop()

	serveErrors := servers.serve()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	var serveErr error
	select {
	case <-stop:
	case serveErr = <-serveErrors:
	}
	cancelRequests()
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	shutdownErr := servers.shutdown(shutdownContext)
	if serveErr != nil {
		return serveErr
	}
	return shutdownErr
}

type boundHTTPServer struct {
	name     string
	server   *http.Server
	listener net.Listener
}

type httpServerGroup struct {
	bound []boundHTTPServer
}

// bindHTTPServers synchronously acquires both ports. This avoids reporting a
// healthy API-only process when the Web listener actually failed to start.
func bindHTTPServers(ctx context.Context, cfg config.Config, apiHandler, webHandler http.Handler) (*httpServerGroup, error) {
	type serverSpec struct {
		name, address string
		handler       http.Handler
	}
	specs := []serverSpec{{name: "API", address: cfg.ListenAddr, handler: apiHandler}}
	if cfg.WebEnabled {
		specs = append(specs, serverSpec{name: "Web", address: cfg.WebListen, handler: webHandler})
	}

	group := &httpServerGroup{}
	for _, spec := range specs {
		listener, err := net.Listen("tcp", spec.address)
		if err != nil {
			group.close()
			return nil, fmt.Errorf("listen %s on %s: %w", spec.name, spec.address, err)
		}
		group.bound = append(group.bound, boundHTTPServer{
			name:     spec.name,
			listener: listener,
			server: &http.Server{
				Addr:              spec.address,
				Handler:           spec.handler,
				BaseContext:       func(net.Listener) context.Context { return ctx },
				ReadHeaderTimeout: 10 * time.Second,
				ReadTimeout:       30 * time.Second,
				WriteTimeout:      120 * time.Second,
				IdleTimeout:       120 * time.Second,
			},
		})
	}
	return group, nil
}

func (g *httpServerGroup) serve() <-chan error {
	errorsOut := make(chan error, len(g.bound))
	for i := range g.bound {
		bound := &g.bound[i]
		log.Printf("gsm-system %s listening on %s", bound.name, bound.listener.Addr())
		go func() {
			if err := bound.server.Serve(bound.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errorsOut <- fmt.Errorf("serve %s: %w", bound.name, err)
			}
		}()
	}
	return errorsOut
}

func (g *httpServerGroup) shutdown(ctx context.Context) error {
	var result error
	for i := range g.bound {
		if err := g.bound[i].server.Shutdown(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("shutdown %s: %w", g.bound[i].name, err))
		}
	}
	return result
}

func (g *httpServerGroup) close() {
	for i := range g.bound {
		_ = g.bound[i].listener.Close()
	}
}
