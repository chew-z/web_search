package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Helper: start a JSON httptest server with a provided handler.
func newJSONServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(func() {
		srv.Close()
	})
	return srv, srv.URL
}

// Helper: set multiple env vars for the duration of the test.
func withEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
}

// Helper: JSON write utility for handlers.
func writeJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func TestCallAPI_Success_WebSearchTrue_VerifyMethodPathAndBody(t *testing.T) {
	withEnv(t, map[string]string{"OPENAI_API_KEY": "test-key"})

	handler := func(w http.ResponseWriter, r *http.Request) {
		// Verify method and path
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/test" {
			t.Errorf("expected path /v1/test, got %s", r.URL.Path)
		}

		// Verify headers
		if got := r.Header.Get("Authorization"); got != "Bearer "+os.Getenv("OPENAI_API_KEY") {
			http.Error(w, "missing or invalid auth", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "missing content-type", http.StatusBadRequest)
			return
		}

		// Verify request body
		var reqBody requestBody
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "failed to decode request body", http.StatusBadRequest)
			return
		}
		if reqBody.Input != "test query" {
			t.Errorf("expected input 'test query', got %s", reqBody.Input)
		}
		if reqBody.Model != "test-model" {
			t.Errorf("expected model 'test-model', got %s", reqBody.Model)
		}
		if reqBody.Reasoning.Effort != "test-effort" {
			t.Errorf("expected effort 'test-effort', got %s", reqBody.Reasoning.Effort)
		}
		if reqBody.Text.Verbosity != "test-verbosity" {
			t.Errorf("expected verbosity 'test-verbosity', got %s", reqBody.Text.Verbosity)
		}
		if reqBody.PreviousResponseID != "test-prev-id" {
			t.Errorf("expected previous response ID 'test-prev-id', got %s", reqBody.PreviousResponseID)
		}
		if len(reqBody.Tools) != 1 || reqBody.Tools[0].Type != "web_search_preview" {
			t.Errorf("expected web search tool, got %+v", reqBody.Tools)
		}

		// Respond with valid JSON
		writeJSON(t, w, http.StatusOK, map[string]any{
			"output": []map[string]any{
				{"type": "message", "content": []map[string]any{{"type": "output_text", "text": "test response"}}},
			},
			"model": "test-model",
			"id":    "test-id",
			"reasoning": map[string]any{
				"effort": "test-effort",
			},
		})
	}

	_, base := newJSONServer(t, handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	apiResp, err := CallAPI(ctx, os.Getenv("OPENAI_API_KEY"), base+"/v1/test", "test query", "test-model", "test-effort", "test-verbosity", "test-prev-id", 2*time.Second, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiResp == nil {
		t.Fatal("expected non-nil apiResponse, got nil")
	}
	if apiResp.Model != "test-model" {
		t.Errorf("expected model 'test-model', got %s", apiResp.Model)
	}
	if apiResp.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got %s", apiResp.ID)
	}
	if apiResp.Reasoning.Effort != "test-effort" {
		t.Errorf("expected effort 'test-effort', got %s", apiResp.Reasoning.Effort)
	}
	if len(apiResp.Output) == 0 || len(apiResp.Output[0].Content) == 0 || apiResp.Output[0].Content[0].Text != "test response" {
		t.Errorf("expected output text 'test response', got %+v", apiResp.Output)
	}
}

func TestCallAPI_Success_WebSearchFalse_OmitsTools(t *testing.T) {
	withEnv(t, map[string]string{"OPENAI_API_KEY": "test-key"})

	handler := func(w http.ResponseWriter, r *http.Request) {
		// Verify method and path
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/omit-tools" {
			t.Errorf("expected path /v1/omit-tools, got %s", r.URL.Path)
		}

		var reqBody requestBody
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "failed to decode request body", http.StatusBadRequest)
			return
		}
		// When useWebSearch=false, Tools should not be populated.
		// After decoding, we should see a nil/empty slice.
		if len(reqBody.Tools) != 0 {
			t.Errorf("expected no tools, got %+v", reqBody.Tools)
		}

		writeJSON(t, w, http.StatusOK, map[string]any{
			"output": []map[string]any{
				{"type": "message", "content": []map[string]any{{"type": "output_text", "text": "ok"}}},
			},
			"model": "m",
			"id":    "id",
			"reasoning": map[string]any{
				"effort": "e",
			},
		})
	}

	_, base := newJSONServer(t, handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	apiResp, err := CallAPI(ctx, os.Getenv("OPENAI_API_KEY"), base+"/v1/omit-tools", "q", "m", "e", "v", "", 2*time.Second, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if apiResp == nil {
		t.Fatal("expected non-nil apiResponse, got nil")
	}
}

func TestCallAPI_MalformedJSONResponse(t *testing.T) {
	withEnv(t, map[string]string{"OPENAI_API_KEY": "test-key"})

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is not valid json"))
	}

	_, base := newJSONServer(t, handler)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := CallAPI(ctx, os.Getenv("OPENAI_API_KEY"), base+"/v1/bad-json", "q", "m", "e", "v", "", 2*time.Second, true)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parse json:") {
		t.Fatalf("expected error to contain 'parse json:', got %v", err)
	}
}

func TestCallAPI_Non2xxErrors_401_403_429(t *testing.T) {
	withEnv(t, map[string]string{"OPENAI_API_KEY": "test-key"})

	type tc struct {
		name       string
		status     int
		respBody   any
		expectBody string
	}

	cases := []tc{
		{name: "401_unauthorized", status: http.StatusUnauthorized, respBody: map[string]string{"error": "unauthorized"}, expectBody: "{\"error\":\"unauthorized\"}\n"},
		{name: "403_forbidden", status: http.StatusForbidden, respBody: map[string]string{"error": "forbidden"}, expectBody: "{\"error\":\"forbidden\"}\n"},
		{name: "429_ratelimit", status: http.StatusTooManyRequests, respBody: map[string]string{"error": "too many requests"}, expectBody: "{\"error\":\"too many requests\"}\n"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			handler := func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, c.status, c.respBody)
			}

			_, base := newJSONServer(t, handler)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err := CallAPI(ctx, os.Getenv("OPENAI_API_KEY"), base+"/v1/error", "q", "m", "e", "v", "", 2*time.Second, true)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError via errors.As, got %T", err)
			}
			if apiErr.StatusCode != c.status {
				t.Errorf("expected status %d, got %d", c.status, apiErr.StatusCode)
			}
			if apiErr.Body != c.expectBody {
				t.Errorf("expected body %q, got %q", c.expectBody, apiErr.Body)
			}
		})
	}
}

func TestCallAPI_Timeout(t *testing.T) {
	withEnv(t, map[string]string{"OPENAI_API_KEY": "test-key"})

	handler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		writeJSON(t, w, http.StatusOK, map[string]any{"ok": true})
	}
	_, base := newJSONServer(t, handler)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := CallAPI(ctx, os.Getenv("OPENAI_API_KEY"), base+"/slow", "q", "m", "e", "v", "", 50*time.Millisecond, true)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// Accept either context deadline exceeded or a net.Error that timeouts.
	if !errors.Is(err, context.DeadlineExceeded) {
		var nerr net.Error
		if !(errors.As(err, &nerr) && nerr.Timeout()) {
			t.Fatalf("expected timeout (deadline exceeded or net.Error), got %v", err)
		}
	}
}

func TestCallAPI_MissingAPIKey(t *testing.T) {
	withEnv(t, map[string]string{"OPENAI_API_KEY": ""})

	// Handler will not be reached; CallAPI should fail fast.
	handler := func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, map[string]any{"ok": true})
	}
	_, base := newJSONServer(t, handler)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := CallAPI(ctx, os.Getenv("OPENAI_API_KEY"), base+"/auth-check", "q", "m", "e", "v", "", 1*time.Second, true)
	if !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("expected ErrNoAPIKey, got %v", err)
	}
}

func TestExtractAnswer(t *testing.T) {
	tests := []struct {
		name    string
		apiResp *apiResponse
		want    string
	}{
		{
			name: "single_text_segment",
			apiResp: &apiResponse{
				Output: []respItem{
					{
						Type: "message",
						Content: []respContent{
							{Type: "output_text", Text: "Hello world."},
						},
					},
				},
			},
			want: "Hello world.",
		},
		{
			name: "multiple_text_segments",
			apiResp: &apiResponse{
				Output: []respItem{
					{
						Type: "message",
						Content: []respContent{
							{Type: "output_text", Text: "First part."},
						},
					},
					{
						Type: "message",
						Content: []respContent{
							{Type: "output_text", Text: "Second part."},
						},
					},
				},
			},
			want: "First part. Second part.",
		},
		{
			name: "mixed_content_types",
			apiResp: &apiResponse{
				Output: []respItem{
					{
						Type: "message",
						Content: []respContent{
							{Type: "output_text", Text: "Text content."},
							{Type: "image", Text: "base64image"},
						},
					},
					{
						Type: "tool_code",
						Content: []respContent{
							{Type: "output_text", Text: "Should be ignored."},
						},
					},
				},
			},
			want: "Text content.",
		},
		{
			name: "empty_output",
			apiResp: &apiResponse{
				Output: []respItem{},
			},
			want: "",
		},
		{
			name:    "nil_api_response",
			apiResp: nil,
			want:    "",
		},
		{
			name: "empty_content_text",
			apiResp: &apiResponse{
				Output: []respItem{
					{
						Type: "message",
						Content: []respContent{
							{Type: "output_text", Text: ""},
						},
					},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractAnswer(tt.apiResp)
			if got != tt.want {
				t.Errorf("ExtractAnswer() got = %v, want %v", got, tt.want)
			}
		})
	}
}
