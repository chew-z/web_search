package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/chew-z/web_search/internal/oauth"
)

const (
	// Default values
	defaultModel = modelLuna

	// Named model constants
	modelLuna        = "gpt-6-luna"
	defaultEffort    = "medium"
	defaultVerbosity = "medium"
	defaultBaseURL   = "https://api.openai.com/v1/responses"

	// Server metadata
	serverName       = "gpt-websearch-mcp"
	serverTitle      = "GPT Web Search"
	serverVersion    = "0.3.5"
	serverWebsiteURL = "https://github.com/chew-z/web_search"

	// Timeouts based on reasoning effort.
	// Enum mirrors what gpt-6-luna accepts:
	// none, low, medium, high, xhigh. "none" disables reasoning entirely;
	// "xhigh" requests the deepest reasoning chain available.
	timeoutNone   = 90 * time.Second
	timeoutLow    = 3 * time.Minute
	timeoutMedium = 5 * time.Minute
	timeoutHigh   = 10 * time.Minute
	timeoutXHigh  = 15 * time.Minute
)

// API request/response structures
type reqReasoning struct {
	Effort string `json:"effort"`
}

type reqTool struct {
	Type string `json:"type"`
}

type reqText struct {
	Verbosity string `json:"verbosity"`
}

type requestBody struct {
	Model              string       `json:"model"`
	Input              string       `json:"input"`
	Reasoning          reqReasoning `json:"reasoning"`
	Text               reqText      `json:"text"`
	Tools              []reqTool    `json:"tools,omitempty"`
	PreviousResponseID string       `json:"previous_response_id,omitempty"`
	PromptCacheKey     string       `json:"prompt_cache_key,omitempty"`
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
	ID        string       `json:"id"`
	Model     string       `json:"model"`
	Reasoning apiReasoning `json:"reasoning"`
	Output    []respItem   `json:"output"`
}

type apiReasoning struct {
	Effort string `json:"effort"`
}

// EnvConfig centralizes environment-derived configuration.
type EnvConfig struct {
	Provider   string
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
	Provider                   string
	APIKey                     string
	BaseURL                    string
	Transport                  string
	Port                       string
	Host                       string
	Verbose                    bool
	AuthEnabled                bool
	AuthSecretKey              string
	Heartbeat                  time.Duration
	DisableLocalhostProtection bool

	// OAuth 2.1 connector auth (ANSWER_MCP_* environment variables).
	OAuthEnabled           bool
	OAuthIssuer            string
	OAuthResource          string
	OAuthLoginUser         string
	OAuthLoginPasswordHash string
	OAuthAccessTTL         time.Duration
	OAuthRefreshTTL        time.Duration
}

// parseEnvBool reads a boolean environment variable with a fallback default.
// An unset variable yields defaultVal; an unparseable value is an error —
// silently falling back on a typo in a security-relevant switch is how
// servers end up publicly open.
func parseEnvBool(key string, defaultVal bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: invalid value %q: %w", key, v, err)
	}
	return b, nil
}

// parseEnvDuration reads a duration environment variable with a fallback default.
func parseEnvDuration(key string, defaultVal time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid value %q: %w", key, v, err)
	}
	return d, nil
}

// loadEnvConfig reads environment variables. providerOverride selects the
// backend explicitly (e.g. from a -provider flag); when empty the PROVIDER
// env var decides, defaulting to openai. The API key is read from the
// resolved provider's own env var.
func loadEnvConfig(providerOverride string) (EnvConfig, error) {
	name := providerOverride
	if name == "" {
		name = os.Getenv("PROVIDER")
	}
	prov, err := resolveProvider(name)
	if err != nil {
		return EnvConfig{}, err
	}

	cfg := EnvConfig{
		Provider: prov.Name,
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

	cfg.APIKey = os.Getenv(prov.KeyEnv)
	if cfg.APIKey == "" {
		return EnvConfig{}, fmt.Errorf("%s %w", prov.KeyEnv, ErrNoAPIKey)
	}

	return cfg, nil
}

// getTimeoutForEffort returns the appropriate timeout based on reasoning effort
// level, derived from effortRegistry. Note the deliberate asymmetry with
// validateEffort: an unknown/empty effort falls back to the "low" timeout here,
// whereas validateEffort falls back to defaultEffort ("medium"). Both behaviors
// are locked in by config_test.go.
func getTimeoutForEffort(effort string) time.Duration {
	if e, ok := effortByName(effort); ok {
		return e.Timeout
	}
	return timeoutLow
}

// validateEffort ensures the effort level is valid for gpt-6-luna:
// none, low, medium, high, xhigh. The legacy "minimal" value (4o/4.1 only) is no
// longer accepted. Valid values come from effortRegistry; anything else
// (including empty) falls back to defaultEffort.
func validateEffort(effort string) string {
	if _, ok := effortByName(effort); ok {
		return effort
	}
	return defaultEffort
}

// validateVerbosity ensures the verbosity level is valid, per verbosityLevels.
func validateVerbosity(verbosity string) string {
	if slices.Contains(verbosityLevels, verbosity) {
		return verbosity
	}
	return defaultVerbosity
}

// parseMCPConfig applies defaults where the caller left a field empty/zero.
// The base URL default is provider-specific; unknown provider names fall back
// to the default provider (the name is validated upstream at flag/env parsing).
func parseMCPConfig(p MCPConfig) MCPConfig {
	prov := providerOrDefault(p.Provider)
	p.Provider = prov.Name

	// Use defaults if not provided
	if p.BaseURL == "" {
		p.BaseURL = prov.DefaultBaseURL
	}
	if p.Transport == "" {
		p.Transport = "stdio"
	}
	if p.Port == "" {
		p.Port = "8080"
	}
	if p.Host == "" {
		p.Host = "127.0.0.1"
	}
	if p.OAuthEnabled {
		// OAuth access tokens are validated by the same guard as static JWTs.
		p.AuthEnabled = true
	}

	return p
}

// OAuth defaults for the ANSWER_MCP_* environment variables.
const (
	defaultOAuthAccessTTL  = time.Hour
	defaultOAuthRefreshTTL = 720 * time.Hour
	minOAuthSecretLen      = 32
)

// loadOAuthEnv reads the OAuth connector-auth settings from ANSWER_MCP_*
// environment variables into cfg.
func loadOAuthEnv(cfg *MCPConfig) error {
	enabled, err := parseEnvBool("ANSWER_MCP_OAUTH", false)
	if err != nil {
		return err
	}
	cfg.OAuthEnabled = enabled
	if enabled {
		// OAuth access tokens are validated by the same guard as static
		// JWTs; runMCPMode reads this env after parseMCPConfig, so the
		// implication is enforced here where OAuthEnabled is discovered.
		cfg.AuthEnabled = true
	}
	cfg.OAuthIssuer = os.Getenv("ANSWER_MCP_ISSUER")
	cfg.OAuthResource = os.Getenv("ANSWER_MCP_RESOURCE")
	cfg.OAuthLoginUser = os.Getenv("ANSWER_MCP_LOGIN_USER")
	cfg.OAuthLoginPasswordHash = os.Getenv("ANSWER_MCP_LOGIN_PASSWORD_HASH")
	if cfg.OAuthAccessTTL, err = parseEnvDuration("ANSWER_MCP_ACCESS_TTL", defaultOAuthAccessTTL); err != nil {
		return err
	}
	cfg.OAuthRefreshTTL, err = parseEnvDuration("ANSWER_MCP_REFRESH_TTL", defaultOAuthRefreshTTL)
	return err
}

// OAuthConfig derives the OAuth authorization-server configuration from the
// MCP settings. Scope and realm defaults come from oauth.Config.Validate
// ("answer" / "answer-mcp").
func (c MCPConfig) OAuthConfig() (oauth.Config, error) {
	cfg := oauth.Config{
		Issuer: c.OAuthIssuer, Resource: c.OAuthResource,
		LoginUser: c.OAuthLoginUser, LoginPasswordHash: c.OAuthLoginPasswordHash,
		AccessTokenTTL: c.OAuthAccessTTL, RefreshTokenTTL: c.OAuthRefreshTTL,
		Signer: HS256Signer{Secret: c.AuthSecretKey, Issuer: c.OAuthIssuer, Audience: c.OAuthResource},
	}
	if err := cfg.Validate(); err != nil {
		return oauth.Config{}, fmt.Errorf("validate OAuth config: %w", err)
	}
	return cfg, nil
}

// Validate enforces the auth configuration rules at startup.
func (c MCPConfig) Validate() error {
	if c.AuthEnabled && c.AuthSecretKey == "" {
		return errors.New("GEMINI_AUTH_SECRET_KEY must be set when authentication is enabled")
	}
	if c.OAuthEnabled {
		if len(c.AuthSecretKey) < minOAuthSecretLen {
			return fmt.Errorf("GEMINI_AUTH_SECRET_KEY must be at least %d bytes when OAuth is enabled", minOAuthSecretLen)
		}
		if _, err := c.OAuthConfig(); err != nil {
			return fmt.Errorf("validate OAuth configuration: %w", err)
		}
	}
	return nil
}
