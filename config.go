package main

import (
	"os"
	"strconv"
	"time"
)

const (
	// Default values
	defaultModel   = "gpt-5-mini"
	defaultEffort  = "low"
	defaultBaseURL = "https://api.openai.com/v1/responses"

	// Server metadata
	serverName    = "gpt-websearch-mcp"
	serverVersion = "1.0.0"

	// Timeouts based on reasoning effort
	timeoutLow    = 3 * time.Minute
	timeoutMedium = 5 * time.Minute
	timeoutHigh   = 10 * time.Minute
)

// API request/response structures
type reqReasoning struct {
	Effort string `json:"effort"`
}

type reqTool struct {
	Type string `json:"type"`
}

type requestBody struct {
	Model     string       `json:"model"`
	Input     string       `json:"input"`
	Reasoning reqReasoning `json:"reasoning"`
	Tools     []reqTool    `json:"tools"`
}

type respContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type respItem struct {
	Type    string        `json:"type"`
	Content []respContent `json:"content,omitempty"`
}

type apiResponse struct {
	Output []respItem `json:"output"`
}

// EnvConfig centralizes environment-derived configuration.
type EnvConfig struct {
	Question   string
	Model      string
	Effort     string
	ShowAll    bool
	HasShowAll bool
	Timeout    time.Duration
	HasTimeout bool
	APIKey     string
}

// MCPConfig holds configuration for the MCP server
type MCPConfig struct {
	APIKey    string
	BaseURL   string
	Transport string
	Port      string
	Verbose   bool
}

// loadEnvConfig reads environment variables
func loadEnvConfig() (EnvConfig, error) {
	cfg := EnvConfig{
		Question: os.Getenv("QUESTION"),
		Model:    os.Getenv("MODEL"),
		Effort:   os.Getenv("EFFORT"),
	}

	if v := os.Getenv("SHOW_ALL"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.ShowAll = b
			cfg.HasShowAll = true
		}
	}

	if v := os.Getenv("TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Timeout = d
			cfg.HasTimeout = true
		}
	}

	cfg.APIKey = os.Getenv("OPENAI_API_KEY")
	if cfg.APIKey == "" {
		return EnvConfig{}, ErrNoAPIKey
	}

	return cfg, nil
}

// getTimeoutForEffort returns the appropriate timeout based on reasoning effort level
func getTimeoutForEffort(effort string) time.Duration {
	switch effort {
	case "high":
		return timeoutHigh
	case "medium":
		return timeoutMedium
	case "low", "":
		return timeoutLow
	default:
		return timeoutLow
	}
}

// validateEffort ensures the effort level is valid
func validateEffort(effort string) string {
	switch effort {
	case "low", "medium", "high":
		return effort
	case "":
		return defaultEffort
	default:
		return defaultEffort
	}
}

// parseMCPConfig creates MCPConfig from environment and command line flags
func parseMCPConfig(apiKey, baseURL, transport, port string, verbose bool) MCPConfig {
	// Use defaults if not provided
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if transport == "" {
		transport = "stdio"
	}
	if port == "" {
		port = "8080"
	}

	return MCPConfig{
		APIKey:    apiKey,
		BaseURL:   baseURL,
		Transport: transport,
		Port:      port,
		Verbose:   verbose,
	}
}
