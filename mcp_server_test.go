package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

// newStatelessMCPHandler builds a stateless streamable-HTTP handler with a
// custom OpenAI baseURL injected. Stateless mode means no Mcp-Session-Id
// management is required across initialize/tools-list/tools-call calls, which
// keeps the JSON-RPC test client simple.
func newStatelessMCPHandler(t *testing.T, baseURL string) http.Handler {
	t.Helper()
	cfg := parseMCPConfig(MCPConfigParams{
		APIKey:    "test-key",
		BaseURL:   baseURL,
		Transport: "http",
		Port:      "0",
		Host:      "127.0.0.1",
	})
	mcpServer := NewMCPServer(cfg)
	httpServer := server.NewStreamableHTTPServer(mcpServer, server.WithStateLess(true))
	if h, ok := any(httpServer).(http.Handler); ok {
		return h
	}
	t.Fatalf("Streamable HTTP server does not implement http.Handler")
	return nil
}

// jsonrpcCall posts a JSON-RPC request to the given URL and returns the parsed
// response. It accepts both application/json and text/event-stream replies and
// extracts the JSON payload from the first SSE `data:` line if needed.
func jsonrpcCall(t *testing.T, url, method string, id int, params any) map[string]any {
	t.Helper()
	body := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"id":      id,
	}
	if params != nil {
		body["params"] = params
	}
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal jsonrpc body: %v", err)
	}

	ctx, cancel := withTimeout(t, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http do: %v", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("jsonrpc %s: status %d, body=%s", method, resp.StatusCode, raw)
	}

	payload := raw
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
		// Pull the first `data:` line out of the SSE stream.
		var found []byte
		for _, line := range bytes.Split(raw, []byte("\n")) {
			line = bytes.TrimRight(line, "\r")
			if bytes.HasPrefix(line, []byte("data:")) {
				found = bytes.TrimSpace(line[len("data:"):])
				break
			}
		}
		if found == nil {
			t.Fatalf("no SSE data frame in response: %s", raw)
		}
		payload = found
	}

	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("unmarshal jsonrpc response: %v; body=%s", err, payload)
	}
	return out
}

// jsonrpcResult extracts the result object, failing if the response carries an
// error. For tools/call results that report `isError:true`, the result object
// is still returned — caller decides what to do with it.
func jsonrpcResult(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	if e, ok := resp["error"]; ok && e != nil {
		t.Fatalf("jsonrpc error: %v", e)
	}
	res, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("jsonrpc response missing result: %v", resp)
	}
	return res
}

func TestMCPServer_Initialize_HasTitleAndWebsiteURL(t *testing.T) {
	t.Parallel()

	handler := newStatelessMCPHandler(t, defaultBaseURL)
	srv, baseURL := newHTTPServerFromHandler(t, handler)
	_ = srv

	resp := jsonrpcCall(t, baseURL+"/", "initialize", 1, map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0.1"},
	})
	res := jsonrpcResult(t, resp)

	info, ok := res["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("missing serverInfo in result: %v", res)
	}
	if got := info["title"]; got != "GPT Web Search" {
		t.Errorf("serverInfo.title: got %v, want %q", got, "GPT Web Search")
	}
	if got := info["websiteUrl"]; got != "https://github.com/chew-z/web_search" {
		t.Errorf("serverInfo.websiteUrl: got %v, want %q", got, "https://github.com/chew-z/web_search")
	}
	if got := info["name"]; got != serverName {
		t.Errorf("serverInfo.name: got %v, want %q", got, serverName)
	}
}

func TestGptWebsearch_ToolsList_SchemaShape(t *testing.T) {
	t.Parallel()

	handler := newStatelessMCPHandler(t, defaultBaseURL)
	srv, baseURL := newHTTPServerFromHandler(t, handler)
	_ = srv

	resp := jsonrpcCall(t, baseURL+"/", "tools/list", 1, map[string]any{})
	res := jsonrpcResult(t, resp)

	tools, ok := res["tools"].([]any)
	if !ok {
		t.Fatalf("tools/list result missing tools array: %v", res)
	}

	var tool map[string]any
	for _, raw := range tools {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if entry["name"] == "gpt_websearch" {
			tool = entry
			break
		}
	}
	if tool == nil {
		t.Fatalf("gpt_websearch not found in tools/list output")
	}

	in, ok := tool["inputSchema"].(map[string]any)
	if !ok {
		t.Fatalf("gpt_websearch inputSchema missing or wrong type")
	}

	if got := in["additionalProperties"]; got != false {
		t.Errorf("inputSchema.additionalProperties: got %v, want false", got)
	}

	props, ok := in["properties"].(map[string]any)
	if !ok {
		t.Fatalf("inputSchema.properties missing")
	}

	checkEnum := func(field string, want []string) {
		t.Helper()
		f, ok := props[field].(map[string]any)
		if !ok {
			t.Errorf("inputSchema.properties.%s missing", field)
			return
		}
		enumRaw, ok := f["enum"].([]any)
		if !ok {
			t.Errorf("inputSchema.properties.%s.enum missing", field)
			return
		}
		got := make([]string, 0, len(enumRaw))
		for _, v := range enumRaw {
			if s, ok := v.(string); ok {
				got = append(got, s)
			}
		}
		if len(got) != len(want) {
			t.Errorf("%s.enum length: got %v, want %v", field, got, want)
			return
		}
		for i, v := range want {
			if got[i] != v {
				t.Errorf("%s.enum[%d]: got %q, want %q", field, i, got[i], v)
			}
		}
	}
	checkEnum("reasoning_effort", []string{"none", "low", "medium", "high", "xhigh"})
	checkEnum("verbosity", []string{"low", "medium", "high"})

	required, ok := in["required"].([]any)
	if !ok {
		t.Fatalf("inputSchema.required missing")
	}
	hasQuery := false
	for _, r := range required {
		if r == "query" {
			hasQuery = true
			break
		}
	}
	if !hasQuery {
		t.Errorf("inputSchema.required does not contain \"query\": %v", required)
	}

	out, ok := tool["outputSchema"].(map[string]any)
	if !ok || out == nil {
		t.Fatalf("gpt_websearch outputSchema missing")
	}
	outProps, ok := out["properties"].(map[string]any)
	if !ok {
		t.Fatalf("outputSchema.properties missing")
	}
	if ans, ok := outProps["answer"].(map[string]any); !ok || ans["type"] != "string" {
		t.Errorf("outputSchema.properties.answer.type: got %v, want string", outProps["answer"])
	}
	if succ, ok := outProps["success"].(map[string]any); !ok || succ["type"] != "boolean" {
		t.Errorf("outputSchema.properties.success.type: got %v, want boolean", outProps["success"])
	}
}

// mockOpenAIServer returns an httptest server that fails the test if any
// request reaches it, plus an int counter for explicit assertions.
func mockOpenAIServerForbidden(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		t.Errorf("unexpected upstream call: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func callToolAndAssertError(t *testing.T, baseURL string, args map[string]any, mustMention string) {
	t.Helper()
	resp := jsonrpcCall(t, baseURL+"/", "tools/call", 1, map[string]any{
		"name":      "gpt_websearch",
		"arguments": args,
	})
	res := jsonrpcResult(t, resp)

	isErr, _ := res["isError"].(bool)
	if !isErr {
		t.Fatalf("expected isError=true, got result: %v", res)
	}
	if mustMention != "" {
		raw, _ := json.Marshal(res)
		if !strings.Contains(strings.ToLower(string(raw)), strings.ToLower(mustMention)) {
			t.Errorf("error result does not mention %q: %s", mustMention, raw)
		}
	}
}

func TestGptWebsearch_InputValidation_RejectsUnknownEnum(t *testing.T) {
	t.Parallel()

	upstream, hits := mockOpenAIServerForbidden(t)
	handler := newStatelessMCPHandler(t, upstream.URL)
	srv, baseURL := newHTTPServerFromHandler(t, handler)
	_ = srv

	callToolAndAssertError(t, baseURL, map[string]any{
		"query":            "x",
		"reasoning_effort": "turbo",
	}, "reasoning_effort")

	if got := hits.Load(); got != 0 {
		t.Errorf("upstream OpenAI received %d calls; want 0", got)
	}
}

func TestGptWebsearch_InputValidation_RejectsAdditionalProperty(t *testing.T) {
	t.Parallel()

	upstream, hits := mockOpenAIServerForbidden(t)
	handler := newStatelessMCPHandler(t, upstream.URL)
	srv, baseURL := newHTTPServerFromHandler(t, handler)
	_ = srv

	callToolAndAssertError(t, baseURL, map[string]any{
		"query": "x",
		"extra": true,
	}, "")

	if got := hits.Load(); got != 0 {
		t.Errorf("upstream OpenAI received %d calls; want 0", got)
	}
}

func TestGptWebsearch_OutputSchemaValidation_PassesForValidResult(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respBody := map[string]any{
			"id":    "resp_test_1",
			"model": modelMini,
			"reasoning": map[string]any{
				"effort": "medium",
			},
			"output": []any{
				map[string]any{
					"type": "message",
					"content": []any{
						map[string]any{
							"type": "output_text",
							"text": "Paris is the capital of France.",
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(respBody)
	}))
	t.Cleanup(upstream.Close)

	handler := newStatelessMCPHandler(t, upstream.URL)
	srv, baseURL := newHTTPServerFromHandler(t, handler)
	_ = srv

	resp := jsonrpcCall(t, baseURL+"/", "tools/call", 1, map[string]any{
		"name": "gpt_websearch",
		"arguments": map[string]any{
			"query":            "capital of France",
			"reasoning_effort": "medium",
			"verbosity":        "low",
			"web_search":       false,
		},
	})
	res := jsonrpcResult(t, resp)

	if isErr, _ := res["isError"].(bool); isErr {
		t.Fatalf("unexpected isError=true: %v", res)
	}
	sc, ok := res["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent missing in result: %v", res)
	}
	if succ, _ := sc["success"].(bool); !succ {
		t.Errorf("structuredContent.success: got %v, want true", sc["success"])
	}
	answer, _ := sc["answer"].(string)
	if answer == "" {
		t.Errorf("structuredContent.answer is empty")
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
