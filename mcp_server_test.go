package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// newTestMCPHandler spins up an MCP server instance and returns an http.Handler.
// Fails with guidance if the underlying HTTP server does not implement http.Handler.
func newTestMCPHandler(t *testing.T) http.Handler {
	t.Helper()

	// Minimal MCP config; API key not required for /health.
	cfg := parseMCPConfig(MCPConfigParams{
		APIKey:    "test-key",
		BaseURL:   defaultBaseURL,
		Transport: "http",
		Port:      "0",
		Host:      "127.0.0.1",
	})
	mcpServer := NewMCPServer(cfg)

	httpServer := server.NewStreamableHTTPServer(mcpServer)

	// Prefer direct handler use if exposed by the HTTP server type.
	if h, ok := any(httpServer).(http.Handler); ok {
		return h
	}

	// If this hits, add a small adapter in transport.go such as:
	//   func HTTPHandler(m *server.MCPServer) http.Handler { return server.NewStreamableHTTPServer(m) }
	t.Fatalf("Streamable HTTP server does not implement http.Handler; add an adapter returning http.Handler for tests")
	return nil
}

// newHTTPServerFromHandler creates an httptest server around a provided handler.
func newHTTPServerFromHandler(t *testing.T, h http.Handler) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(func() { srv.Close() })
	return srv, srv.URL
}

// withTimeout builds a context with timeout for HTTP requests.
func withTimeout(t *testing.T, d time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), d)
}

func TestMCPServer_HTTP_HealthRoute(t *testing.T) {
	handler := newTestMCPHandler(t)
	server, baseURL := newHTTPServerFromHandler(t, handler)
	_ = server // cleaned up by t.Cleanup

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "health_ok", method: http.MethodGet, path: "/health", wantStatus: http.StatusOK},
		// Optionally verify root docs/handshake page if exposed:
		// {name: "root_docs", method: http.MethodGet, path: "/", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := withTimeout(t, 400*time.Millisecond)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, tt.method, baseURL+tt.path, nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("http do: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("unexpected status for %s %s: got %d, want %d", tt.method, tt.path, resp.StatusCode, tt.wantStatus)
			}
		})
	}
}

// TestMCPServer_HTTP_ProxiedPaths verifies the MCP handler responds to POST
// initialize at any path — critical for reverse proxies (like nginx with
// variable proxy_pass) that forward the original URI instead of rewriting it.
func TestMCPServer_HTTP_ProxiedPaths(t *testing.T) {
	t.Parallel()

	handler := newTestMCPHandler(t)
	srv, baseURL := newHTTPServerFromHandler(t, handler)
	_ = srv

	initBody := `{"jsonrpc":"2.0","method":"initialize","id":1,"params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}`

	tests := []struct {
		name string
		path string
	}{
		{name: "direct /mcp", path: "/mcp"},
		{name: "proxied /answer/mcp", path: "/answer/mcp"},
		{name: "proxied /prefix/deep/mcp", path: "/prefix/deep/mcp"},
		{name: "root /", path: "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := withTimeout(t, 2*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+tt.path, strings.NewReader(initBody))
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("http do: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("POST %s: got %d, want 200, body: %s", tt.path, resp.StatusCode, body)
			}
		})
	}
}
