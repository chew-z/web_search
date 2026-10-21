package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/chew-z/web_search/internal/oauth"
	"golang.org/x/term"
)

// defaultTokenTTL is the lifetime of CLI-minted static tokens (31 days).
const defaultTokenTTL = 744 * time.Hour

// readPassword reads one line from stdin, suppressing terminal echo when
// stdin is a TTY. It is a package-level var to provide a test seam.
var readPassword = func(stdin io.Reader, stderr io.Writer) (string, error) {
	if file, ok := stdin.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		value, err := term.ReadPassword(int(file.Fd()))
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		_, _ = fmt.Fprintln(stderr)
		return string(value), nil
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read password: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// runCredentialCommand dispatches the one-shot -generate-token and
// -hash-password flags. handled reports whether the process should exit after
// this call; err carries the failure to log when it should not exit silently.
func runCredentialCommand(generateToken, hashPassword bool, username string, ttl time.Duration) (handled bool, err error) {
	if !generateToken && !hashPassword {
		return false, nil
	}
	if generateToken && hashPassword {
		return true, errors.New("-generate-token and -hash-password cannot be used together")
	}
	if generateToken {
		return true, runGenerateToken(username, ttl, os.Stdout)
	}
	return true, runHashPassword(os.Stdin, os.Stdout, os.Stderr)
}

// runGenerateToken signs a static bearer token with GEMINI_AUTH_SECRET_KEY and
// writes it to stdout.
func runGenerateToken(username string, ttl time.Duration, stdout io.Writer) error {
	secret := os.Getenv("GEMINI_AUTH_SECRET_KEY")
	if secret == "" {
		return errors.New("GEMINI_AUTH_SECRET_KEY environment variable is required for token generation")
	}
	token, err := GenerateStaticToken(secret, username, ttl)
	if err != nil {
		return fmt.Errorf("generate static token: %w", err)
	}
	_, _ = fmt.Fprintln(stdout, token)
	return nil
}

// runHashPassword reads a password and prints its PBKDF2 hash — the format
// ANSWER_MCP_LOGIN_PASSWORD_HASH expects — on stdout. The prompt goes to
// stderr so stdout stays pipeable.
func runHashPassword(stdin io.Reader, stdout, stderr io.Writer) error {
	_, _ = fmt.Fprint(stderr, "Password: ")
	password, err := readPassword(stdin, stderr)
	if err != nil {
		return err
	}
	if password == "" {
		return errors.New("password must not be empty")
	}
	hash, err := oauth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, _ = fmt.Fprintln(stdout, hash)
	return nil
}
