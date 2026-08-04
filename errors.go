package main

import (
	"errors"
	"fmt"
	"os"
)

var (
	// Configuration errors
	ErrNoAPIKey        = errors.New("environment variable is required")
	ErrInvalidProvider = errors.New("invalid provider")
)

// APIError represents an error from the OpenAI API
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: status=%d body=%s", e.StatusCode, e.Body)
}

// exitError carries a process exit code alongside a message so that runCLI can
// return errors (and stay unit-testable) while main remains the single place
// that calls os.Exit. The message is the exact string shown on stderr.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// fail prints to stderr and exits non-zero. Called only from main.
func fail(code int, msg string) {
	fmt.Fprintf(os.Stderr, "%s\n", msg)
	os.Exit(code)
}
