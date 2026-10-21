package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestRunGenerateToken(t *testing.T) {
	t.Run("missing_secret_errors", func(t *testing.T) {
		t.Setenv("GEMINI_AUTH_SECRET_KEY", "")
		var out bytes.Buffer
		err := runGenerateToken("alice", time.Hour, &out)
		if err == nil || !strings.Contains(err.Error(), "GEMINI_AUTH_SECRET_KEY") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("token_passes_the_guard_parse", func(t *testing.T) {
		t.Setenv("GEMINI_AUTH_SECRET_KEY", testSecret)
		var out bytes.Buffer
		if err := runGenerateToken("alice", 24*time.Hour, &out); err != nil {
			t.Fatalf("runGenerateToken() error = %v", err)
		}
		token := strings.TrimSpace(out.String())
		if token == "" {
			t.Fatal("no token printed")
		}
		claims, err := parseBearerToken(testSecret, bearerRequest(t, "Bearer "+token))
		if err != nil {
			t.Fatalf("parseBearerToken() error = %v", err)
		}
		if claims.Username != "alice" || claims.Issuer != staticIssuer || !hasAudience(claims.Audience, staticAudience) {
			t.Fatalf("claims = %+v", claims)
		}
	})

	t.Run("nonpositive_ttl_errors", func(t *testing.T) {
		t.Setenv("GEMINI_AUTH_SECRET_KEY", testSecret)
		var out bytes.Buffer
		if err := runGenerateToken("alice", 0, &out); err == nil {
			t.Fatal("expected error for zero TTL")
		}
	})
}

func TestRunHashPassword(t *testing.T) {
	t.Run("hashes_piped_password", func(t *testing.T) {
		var out, errOut bytes.Buffer
		if err := runHashPassword(strings.NewReader("pw\n"), &out, &errOut); err != nil {
			t.Fatalf("runHashPassword() error = %v", err)
		}
		hash := strings.TrimSpace(out.String())
		if !strings.HasPrefix(hash, "pbkdf2-sha256.") {
			t.Fatalf("hash = %q", hash)
		}
		if len(strings.Split(hash, ".")) != 4 {
			t.Fatalf("hash is not four dot-separated fields: %q", hash)
		}
		if !strings.Contains(errOut.String(), "Password:") {
			t.Fatalf("prompt missing from stderr: %q", errOut.String())
		}
	})

	t.Run("empty_password_errors", func(t *testing.T) {
		var out, errOut bytes.Buffer
		err := runHashPassword(strings.NewReader("\n"), &out, &errOut)
		if err == nil || !strings.Contains(err.Error(), "empty") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("read_errors_propagate", func(t *testing.T) {
		var out, errOut bytes.Buffer
		if err := runHashPassword(errReader{}, &out, &errOut); err == nil {
			t.Fatal("expected read error")
		}
	})
}

func TestRunCredentialCommand(t *testing.T) {
	t.Run("no_flags_not_handled", func(t *testing.T) {
		handled, err := runCredentialCommand(false, false, "admin", time.Hour)
		if handled || err != nil {
			t.Fatalf("handled = %v, err = %v", handled, err)
		}
	})

	t.Run("both_flags_is_an_error", func(t *testing.T) {
		handled, err := runCredentialCommand(true, true, "admin", time.Hour)
		if !handled || err == nil || !strings.Contains(err.Error(), "cannot be used together") {
			t.Fatalf("handled = %v, err = %v", handled, err)
		}
	})

	t.Run("generate_token_handled", func(t *testing.T) {
		t.Setenv("GEMINI_AUTH_SECRET_KEY", testSecret)
		handled, err := runCredentialCommand(true, false, "alice", time.Hour)
		if !handled || err != nil {
			t.Fatalf("handled = %v, err = %v", handled, err)
		}
	})
}

// errReader always fails, exercising the readPassword error path.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
