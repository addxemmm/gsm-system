package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Check verifies that the independently bound browser listener is answering.
// It is read-only and never calls the RF API.
func Check(ctx context.Context, listen string) error {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("invalid web listen address: %w", err)
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	} else if host == "::" {
		host = "::1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/web-meta.json", nil)
	if err != nil {
		return err
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("web probe failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("web HTTP status %d", resp.StatusCode)
	}
	var metadata Metadata
	dec := json.NewDecoder(io.LimitReader(resp.Body, 16<<10))
	if err := dec.Decode(&metadata); err != nil || strings.TrimSpace(metadata.Version) == "" {
		return fmt.Errorf("invalid web metadata response")
	}
	return nil
}
