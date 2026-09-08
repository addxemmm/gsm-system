// Package healthcheck implements a read-only container probe; it never starts RF.
package healthcheck

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

// Check accepts an intentionally stopped cell or a fully ready running cell.
// HTTP 200 alone is insufficient: /cell is a status query even when degraded.
func Check(ctx context.Context, listen, token string) error {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if host == "::" {
		host = "::1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/api/v1/cell", nil)
	if err != nil {
		return err
	}
	if token = strings.TrimSpace(token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 25 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("management probe failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("management HTTP status %d", resp.StatusCode)
	}
	var result struct {
		Code *int `json:"code"`
		Data struct {
			State         string `json:"state"`
			Ready         bool   `json:"ready"`
			Running       bool   `json:"running"`
			Transitioning bool   `json:"transitioning"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&result); err != nil {
		return fmt.Errorf("invalid management response")
	}
	if result.Code == nil || *result.Code != 0 || result.Data.Transitioning {
		return fmt.Errorf("management not ready")
	}
	if result.Data.State == "stopped" && !result.Data.Running {
		return nil
	}
	if result.Data.State == "running" && result.Data.Running && result.Data.Ready {
		return nil
	}
	return fmt.Errorf("cell is not ready")
}
