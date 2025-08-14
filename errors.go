package main

import (
	"errors"
	"fmt"
	"os"
)

var (
	// Configuration errors
	ErrNoAPIKey = errors.New("OPENAI_API_KEY environment variable is required")

	// API errors
	ErrNoOutputText = errors.New("no output_text found in response")
	ErrAPIRequest   = errors.New("API request failed")

	// MCP errors
	ErrQueryRequired      = errors.New("please provide a query to search for")
	ErrInvalidEffort      = errors.New("invalid reasoning effort level")
	ErrSessionNotFound    = errors.New("session not found")
	ErrNotificationFailed = errors.New("failed to send notification")
)

// APIError represents an error from the OpenAI API
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: status=%d body=%s", e.StatusCode, e.Body)
}

// fail prints to stderr and exits non-zero.
func fail(code int, msg string) {
	fmt.Fprintf(os.Stderr, "%s\n", msg)
	os.Exit(code)
}
