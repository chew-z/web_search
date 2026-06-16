package main

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

const maxResponseBodySize = 10 * 1024 * 1024 // 10 MB

var httpClient = &http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConnsPerHost:   runtime.GOMAXPROCS(0) + 1,
	},
}

// CallAPIParams groups the inputs for CallAPI to keep the signature readable.
type CallAPIParams struct {
	APIKey             string
	BaseURL            string
	Query              string
	Model              string
	Effort             string
	Verbosity          string
	PreviousResponseID string
	PromptCacheKey     string
	Timeout            time.Duration
	UseWebSearch       bool
	// HTTPClient overrides the package-level default client when non-nil.
	// Enables per-request / test injection without touching the global.
	HTTPClient *http.Client
}

// CallAPI makes the actual API call - reusable for both CLI and MCP
func CallAPI(ctx context.Context, p CallAPIParams) (*apiResponse, error) {
	if p.APIKey == "" {
		return nil, ErrNoAPIKey
	}
	body := requestBody{
		Model: p.Model,
		Input: p.Query,
		Reasoning: reqReasoning{
			Effort: p.Effort,
		},
		Text: reqText{
			Verbosity: p.Verbosity,
		},
		PreviousResponseID: p.PreviousResponseID,
		PromptCacheKey:     p.PromptCacheKey,
	}

	// Conditionally add web search tool
	if p.UseWebSearch {
		body.Tools = []reqTool{
			{Type: "web_search_preview"},
		}
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := clientOrDefault(p.HTTPClient).Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	limitedReader := io.LimitReader(resp.Body, maxResponseBodySize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(bodyBytes)}
	}

	var ar apiResponse
	if err := json.Unmarshal(bodyBytes, &ar); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	return &ar, nil
}

// clientOrDefault returns the supplied client, or the package-level default
// when nil. Kept as a helper so CallAPI stays under the gocyclo threshold.
func clientOrDefault(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return httpClient
}

// ExtractAnswer extracts the answer text from the API response
func ExtractAnswer(apiResp *apiResponse) string {
	if apiResp == nil {
		return ""
	}
	var sb strings.Builder
	for _, item := range apiResp.Output {
		if item.Type != "message" {
			continue
		}
		for _, content := range item.Content {
			if content.Type == "output_text" && content.Text != "" {
				if sb.Len() > 0 {
					sb.WriteString(" ")
				}
				sb.WriteString(content.Text)
			}
		}
	}
	return sb.String()
}

// WebSearchParams holds the typed inputs for a web-search request. It is the
// single, type-safe entry point into HandleWebSearch — no map[string]interface{}
// plumbing. The MCP handler populates it from the validated tool-call request;
// other (e.g. future non-MCP) callers can build it directly.
type WebSearchParams struct {
	Query              string
	Model              string
	Effort             string
	Verbosity          string
	PreviousResponseID string
	PromptCacheKey     string
	UseWebSearch       bool
}

// resolvePromptCacheKey picks the prompt_cache_key to send upstream:
// caller-provided value wins; otherwise a per-authenticated-user shard
// (when the request was authenticated); otherwise the server name as a
// stable global default.
func resolvePromptCacheKey(ctx context.Context, supplied string) string {
	if supplied != "" {
		return supplied
	}
	if userID, _ := getUserInfo(ctx); userID != "" {
		return serverName + ":" + userID
	}
	return serverName
}

// HandleWebSearch handles web search requests for the MCP server
func HandleWebSearch(ctx context.Context, apiKey, baseURL string, p WebSearchParams) (*WebSearchResult, error) {
	if p.Query == "" {
		errMsg := "Please provide a query to search for"
		Warn("empty query rejected")
		logToClient(ctx, mcp.LoggingLevelError, "api_handler", errMsg)
		return &WebSearchResult{
			Success:            false,
			Error:              errMsg,
			WebSearchUsed:      false,
			PreviousResponseID: p.PreviousResponseID,
		}, nil
	}

	// Keep validation defensive even though the MCP enum schema already rejects
	// bad effort/verbosity: model has no enum, and a future non-MCP caller could
	// populate WebSearchParams directly. This is the same defense the deleted
	// extractWebSearchArgs provided.
	model := p.Model
	if model == "" {
		model = defaultModel
	}
	effort := validateEffort(p.Effort)
	verbosity := validateVerbosity(p.Verbosity)

	query := p.Query
	previousResponseID, useWebSearch := p.PreviousResponseID, p.UseWebSearch
	timeout := getTimeoutForEffort(effort)
	cacheKey := resolvePromptCacheKey(ctx, p.PromptCacheKey)

	apiResp, err := CallAPI(ctx, CallAPIParams{
		APIKey:             apiKey,
		BaseURL:            baseURL,
		Query:              query,
		Model:              model,
		Effort:             effort,
		Verbosity:          verbosity,
		PreviousResponseID: previousResponseID,
		PromptCacheKey:     cacheKey,
		Timeout:            timeout,
		UseWebSearch:       useWebSearch,
	})
	if err != nil {
		return nil, err
	}

	// Extract answer from response
	answer := ExtractAnswer(apiResp)
	if answer == "" {
		errMsg := "No answer found in response"
		Warn("no answer in response", "model", model, "effort", effort, "response_id", apiResp.ID)
		logToClient(ctx, mcp.LoggingLevelWarning, "api_handler", errMsg)
		return &WebSearchResult{
			Success:            false,
			Error:              errMsg,
			Query:              query,
			RequestedModel:     model,
			RequestedEffort:    effort,
			WebSearchUsed:      useWebSearch,
			TimeoutUsed:        timeout.String(),
			PreviousResponseID: previousResponseID,
		}, nil
	}

	// Log successful completion
	Debug("search completed", "model", apiResp.Model, "effort", apiResp.Reasoning.Effort, "answer_chars", len(answer), "response_id", apiResp.ID)
	logToClient(ctx, mcp.LoggingLevelDebug, "api_handler", fmt.Sprintf("Search completed: model=%s effort=%s answer=%d chars", apiResp.Model, apiResp.Reasoning.Effort, len(answer)))

	// Return structured response
	return &WebSearchResult{
		Success:            true,
		Answer:             answer,
		Query:              query,
		Model:              apiResp.Model,
		Effort:             apiResp.Reasoning.Effort,
		TimeoutUsed:        timeout.String(),
		ID:                 apiResp.ID,
		RequestedModel:     model,
		RequestedEffort:    effort,
		WebSearchUsed:      useWebSearch,
		PreviousResponseID: previousResponseID,
	}, nil
}

// WebSearchResult defines the structured result returned to MCP clients
type WebSearchResult struct {
	Success            bool   `json:"success"`
	Answer             string `json:"answer,omitempty"`
	Query              string `json:"query"`
	Model              string `json:"model"`
	Effort             string `json:"effort"`
	TimeoutUsed        string `json:"timeout_used"`
	ID                 string `json:"id,omitempty"`
	RequestedModel     string `json:"requested_model"`
	RequestedEffort    string `json:"requested_effort"`
	WebSearchUsed      bool   `json:"web_search_used"`
	PreviousResponseID string `json:"previous_response_id,omitempty"`
	Error              string `json:"error,omitempty"`
}
