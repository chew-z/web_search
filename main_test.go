package main

import (
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// resetCLIState restores the process-global flag and arg state that
// parseCLIArgs mutates, so each runCLI test case starts clean. Without this,
// the second call panics with "flag redefined: base".
func resetCLIState(t *testing.T, argv []string) {
	t.Helper()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	oldArgs := os.Args
	os.Args = argv
	t.Cleanup(func() { os.Args = oldArgs })
}

// assertExitError checks that err is an *exitError carrying the exact code and
// message previously passed to fail() at the converted call site.
func assertExitError(t *testing.T, err error, wantCode int, wantMsg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var ee *exitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *exitError, got %T: %v", err, err)
	}
	if ee.code != wantCode {
		t.Errorf("exit code = %d, want %d", ee.code, wantCode)
	}
	if ee.msg != wantMsg {
		t.Errorf("exit message = %q, want %q", ee.msg, wantMsg)
	}
}

// TestRunCLI_MissingAPIKey covers the loadEnvConfig failure path: exit code 2,
// message is ErrNoAPIKey.Error(). t.Setenv overrides the autoloaded .env.
func TestRunCLI_MissingAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	resetCLIState(t, []string{"answer", "some question"})

	err := runCLI()
	assertExitError(t, err, 2, ErrNoAPIKey.Error())
}

// TestRunCLI_NoQuestion covers the empty-question path: exit code 2 with the
// exact guidance string. Returns before any network call.
func TestRunCLI_NoQuestion(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("QUESTION", "")
	resetCLIState(t, []string{"answer"})

	err := runCLI()
	assertExitError(t, err, 2, "please provide a question to ask (use -q flag or positional argument)")
}

// TestRunCLI_CallAPIError covers the CallAPI failure path: a non-2xx upstream
// yields exit code 2 with the wrapped error's message (err.Error()). The
// message must equal the APIError's Error() string, surfaced verbatim.
func TestRunCLI_CallAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("QUESTION", "")
	resetCLIState(t, []string{"answer", "-base", srv.URL, "ping"})

	err := runCLI()
	wantMsg := (&APIError{StatusCode: http.StatusInternalServerError, Body: `{"error":"boom"}`}).Error()
	assertExitError(t, err, 2, wantMsg)
}

// TestRunCLI_EmptyAnswer covers the empty-answer path: a 200 response with no
// output_text yields exit code 3 with "no answer found in response".
func TestRunCLI_EmptyAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Valid response shape but no message/output_text → ExtractAnswer == "".
		_, _ = w.Write([]byte(`{"id":"x","model":"m","reasoning":{"effort":"low"},"output":[]}`))
	}))
	t.Cleanup(srv.Close)

	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("QUESTION", "")
	resetCLIState(t, []string{"answer", "-base", srv.URL, "ping"})

	err := runCLI()
	assertExitError(t, err, 3, "no answer found in response")
}
