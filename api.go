package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// CallAPI makes the actual API call - reusable for both CLI and MCP
func CallAPI(ctx context.Context, apiKey, baseURL, query, model, effort, verbosity, previousResponseID string, timeout time.Duration, useWebSearch bool) (*apiResponse, error) {
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}
	body := requestBody{
		Model: model,
		Input: query,
		Reasoning: reqReasoning{
			Effort: effort,
		},
		Text: reqText{
			Verbosity: verbosity,
		},
		PreviousResponseID: previousResponseID,
	}

	// Conditionally add web search tool
	if useWebSearch {
		body.Tools = []reqTool{
			{Type: "web_search_preview"},
		}
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
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

// ExtractAnswer extracts the answer text from the API response
func ExtractAnswer(apiResp *apiResponse) string {
	if apiResp == nil {
		return ""
	}
	var answers []string
	for _, item := range apiResp.Output {
		if item.Type != "message" {
			continue
		}
		for _, content := range item.Content {
			if content.Type == "output_text" && content.Text != "" {
				answers = append(answers, content.Text)
			}
		}
	}

	// Join all text content into a single answer
	if len(answers) > 0 {
		// If multiple text segments, join them with space
		answer := ""
		for i, text := range answers {
			if i > 0 {
				answer += " "
			}
			answer += text
		}
		return answer
	}
	return ""
}

// HandleWebSearch handles web search requests for the MCP server
func HandleWebSearch(ctx context.Context, apiKey, baseURL string, args map[string]interface{}) (*WebSearchResult, error) {
	// Extract optional previous response id first for consistent population
	previousResponseID, _ := args["previous_response_id"].(string) //nolint:errcheck

	// Extract parameters
	query, ok := args["query"].(string)
	if !ok || query == "" {
		errMsg := "Please provide a query to search for"
		logToClient(ctx, mcp.LoggingLevelError, "api_handler", errMsg)
		return &WebSearchResult{
				Success:            false,
				Error:              errMsg,
				Query:              query,
				WebSearchUsed:      false,
				PreviousResponseID: previousResponseID,
			},
			nil
	}

	model, _ := args["model"].(string) //nolint:errcheck // Type assertion ok to ignore
	if model == "" {
		model = defaultModel
	}

	effort, _ := args["reasoning_effort"].(string) //nolint:errcheck // Type assertion ok to ignore
	effort = validateEffort(effort)

	verbosity, _ := args["verbosity"].(string) //nolint:errcheck // Type assertion ok to ignore
	verbosity = validateVerbosity(verbosity)

	// Extract web search parameter (defaults to true)
	useWebSearch := true
	if webSearchVal, exists := args["web_search"]; exists {
		if webSearchBool, ok := webSearchVal.(bool); ok {
			useWebSearch = webSearchBool
		}
	}

	// Use effort-based timeout
	timeout := getTimeoutForEffort(effort)

	// Make API call with determined web search setting
	apiResp, err := CallAPI(ctx, apiKey, baseURL, query, model, effort, verbosity, previousResponseID, timeout, useWebSearch)
	if err != nil {
		return nil, err
	}

	// Extract answer from response
	answer := ExtractAnswer(apiResp)
	if answer == "" {
		errMsg := "No answer found in response"
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
	logToClient(ctx, mcp.LoggingLevelDebug, "api_handler", fmt.Sprintf("Search completed successfully, answer length: %d characters", len(answer)))

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
