package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CallAPI makes the actual API call - reusable for both CLI and MCP
func CallAPI(ctx context.Context, apiKey, baseURL, query, model, effort string, timeout time.Duration) (*apiResponse, error) {
	body := requestBody{
		Model: model,
		Input: query,
		Reasoning: reqReasoning{
			Effort: effort,
		},
		Tools: []reqTool{
			{Type: "web_search_preview"},
		},
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
func HandleWebSearch(ctx context.Context, apiKey, baseURL string, args map[string]interface{}) (interface{}, error) {
	// Extract parameters
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return map[string]interface{}{
			"success": false,
			"error":   "Please provide a query to search for",
		}, nil
	}

	model, _ := args["model"].(string) //nolint:errcheck // Type assertion ok to ignore
	if model == "" {
		model = defaultModel
	}

	effort, _ := args["reasoning_effort"].(string) //nolint:errcheck // Type assertion ok to ignore
	effort = validateEffort(effort)

	// Use effort-based timeout
	timeout := getTimeoutForEffort(effort)

	// Make API call
	apiResp, err := CallAPI(ctx, apiKey, baseURL, query, model, effort, timeout)
	if err != nil {
		return nil, err
	}

	// Extract the answer from response
	answer := ExtractAnswer(apiResp)
	if answer == "" {
		return map[string]interface{}{
			"success": false,
			"error":   "No answer found in response",
		}, nil
	}

	// Return structured response
	return map[string]interface{}{
		"success":      true,
		"answer":       answer,
		"query":        query,
		"model":        model,
		"effort":       effort,
		"timeout_used": timeout.String(),
	}, nil
}
