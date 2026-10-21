package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLoadEnvConfig_Table(t *testing.T) {
	type want struct {
		err        error
		apiKey     string
		question   string
		model      string
		effort     string
		showAll    bool
		hasShowAll bool
		timeout    time.Duration
		hasTimeout bool
	}

	tests := []struct {
		name string
		env  map[string]string
		want want
	}{
		{
			name: "missing_api_key_returns_error",
			env: map[string]string{
				"OPENAI_API_KEY": "",
			},
			want: want{
				err: ErrNoAPIKey,
			},
		},
		{
			name: "show_all_true_parsed",
			env: map[string]string{
				"OPENAI_API_KEY": "k",
				"SHOW_ALL":       "true",
			},
			want: want{
				apiKey:     "k",
				showAll:    true,
				hasShowAll: true,
			},
		},
		{
			name: "show_all_false_parsed",
			env: map[string]string{
				"OPENAI_API_KEY": "k",
				"SHOW_ALL":       "false",
			},
			want: want{
				apiKey:     "k",
				showAll:    false,
				hasShowAll: true,
			},
		},
		{
			name: "show_all_invalid_not_set",
			env: map[string]string{
				"OPENAI_API_KEY": "k",
				"SHOW_ALL":       "not-a-bool",
			},
			want: want{
				apiKey:     "k",
				showAll:    false,
				hasShowAll: false,
			},
		},
		{
			name: "timeout_valid_parsed",
			env: map[string]string{
				"OPENAI_API_KEY": "k",
				"TIMEOUT":        "150ms",
			},
			want: want{
				apiKey:     "k",
				timeout:    150 * time.Millisecond,
				hasTimeout: true,
			},
		},
		{
			name: "timeout_invalid_not_set",
			env: map[string]string{
				"OPENAI_API_KEY": "k",
				"TIMEOUT":        "not-a-duration",
			},
			want: want{
				apiKey:     "k",
				timeout:    0,
				hasTimeout: false,
			},
		},
		{
			name: "question_model_effort_read_through",
			env: map[string]string{
				"OPENAI_API_KEY": "k",
				"QUESTION":       "What is up?",
				"MODEL":          "gpt-5-mini",
				"EFFORT":         "high",
			},
			want: want{
				apiKey:   "k",
				question: "What is up?",
				model:    "gpt-5-mini",
				effort:   "high",
			},
		},
	}

	clearAll := func(t *testing.T) {
		t.Helper()
		// Clear all vars read by loadEnvConfig to keep tests hermetic.
		t.Setenv("OPENAI_API_KEY", "")
		t.Setenv("PROVIDER", "")
		t.Setenv("QUESTION", "")
		t.Setenv("MODEL", "")
		t.Setenv("EFFORT", "")
		t.Setenv("SHOW_ALL", "")
		t.Setenv("TIMEOUT", "")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearAll(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			got, err := loadEnvConfig("")

			if tt.want.err != nil {
				if !errors.Is(err, tt.want.err) {
					t.Fatalf("loadEnvConfig error = %v, want %v", err, tt.want.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("loadEnvConfig unexpected error: %v", err)
			}

			if got.APIKey != tt.want.apiKey {
				t.Errorf("APIKey = %q, want %q", got.APIKey, tt.want.apiKey)
			}
			if got.Question != tt.want.question {
				t.Errorf("Question = %q, want %q", got.Question, tt.want.question)
			}
			if got.Model != tt.want.model {
				t.Errorf("Model = %q, want %q", got.Model, tt.want.model)
			}
			if got.Effort != tt.want.effort {
				t.Errorf("Effort = %q, want %q", got.Effort, tt.want.effort)
			}
			if got.ShowAll != tt.want.showAll {
				t.Errorf("ShowAll = %v, want %v", got.ShowAll, tt.want.showAll)
			}
			if got.HasShowAll != tt.want.hasShowAll {
				t.Errorf("HasShowAll = %v, want %v", got.HasShowAll, tt.want.hasShowAll)
			}
			if got.Timeout != tt.want.timeout {
				t.Errorf("Timeout = %v, want %v", got.Timeout, tt.want.timeout)
			}
			if got.HasTimeout != tt.want.hasTimeout {
				t.Errorf("HasTimeout = %v, want %v", got.HasTimeout, tt.want.hasTimeout)
			}
		})
	}
}

func TestParseMCPConfig_Defaults(t *testing.T) {
	t.Parallel()

	got := parseMCPConfig(MCPConfig{})

	if got.Provider != providerOpenAI {
		t.Errorf("Provider = %q, want %q", got.Provider, providerOpenAI)
	}
	if got.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", got.APIKey)
	}
	if got.BaseURL != defaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, defaultBaseURL)
	}
	if got.Transport != "stdio" {
		t.Errorf("Transport = %q, want %q", got.Transport, "stdio")
	}
	if got.Port != "8080" {
		t.Errorf("Port = %q, want %q", got.Port, "8080")
	}
	if got.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want %q", got.Host, "127.0.0.1")
	}
	if got.Verbose != false {
		t.Errorf("Verbose = %v, want %v", got.Verbose, false)
	}
}

func TestParseMCPConfig_NonDefaults(t *testing.T) {
	t.Parallel()

	want := MCPConfig{
		Provider:      providerDeepSeek,
		APIKey:        "k",
		BaseURL:       "http://example.local",
		Transport:     "http",
		Port:          "9090",
		Host:          "0.0.0.0",
		Verbose:       true,
		AuthEnabled:   true,
		AuthSecretKey: "secret",
		Heartbeat:     15 * time.Second,
	}

	got := parseMCPConfig(want)

	if got != want {
		t.Errorf("parseMCPConfig = %+v, want %+v", got, want)
	}
}

func TestParseMCPConfig_DeepSeekBaseURLDefault(t *testing.T) {
	t.Parallel()

	got := parseMCPConfig(MCPConfig{Provider: providerDeepSeek})

	if got.BaseURL != deepseekBaseURL {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, deepseekBaseURL)
	}
}

func TestGetTimeoutForEffort(t *testing.T) {
	tests := []struct {
		effort string
		want   time.Duration
	}{
		{"none", timeoutNone},
		{"low", timeoutLow},
		{"medium", timeoutMedium},
		{"high", timeoutHigh},
		{"xhigh", timeoutXHigh},
		{"", timeoutLow},        // empty maps to low per implementation
		{"unknown", timeoutLow}, // unknown maps to low per implementation
	}

	for _, tt := range tests {
		t.Run(tt.effort, func(t *testing.T) {
			t.Parallel()
			if got := getTimeoutForEffort(tt.effort); got != tt.want {
				t.Errorf("getTimeoutForEffort(%q) = %v, want %v", tt.effort, got, tt.want)
			}
		})
	}
}

func TestValidateEffort(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"none", "none"},
		{"low", "low"},
		{"medium", "medium"},
		{"high", "high"},
		{"xhigh", "xhigh"},
		{"minimal", defaultEffort}, // legacy 4o-only value, no longer accepted
		{"", defaultEffort},
		{"weird", defaultEffort},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if got := validateEffort(tt.in); got != tt.want {
				t.Errorf("validateEffort(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateVerbosity(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"low", "low"},
		{"medium", "medium"},
		{"high", "high"},
		{"", defaultVerbosity},
		{"invalid", defaultVerbosity},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if got := validateVerbosity(tt.in); got != tt.want {
				t.Errorf("validateVerbosity(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLoadEnvConfig_Provider(t *testing.T) {
	clear := func(t *testing.T) {
		t.Helper()
		t.Setenv("PROVIDER", "")
		t.Setenv("OPENAI_API_KEY", "")
		t.Setenv("DEEPSEEK_API_KEY", "")
	}

	t.Run("deepseek_env_reads_deepseek_key", func(t *testing.T) {
		clear(t)
		t.Setenv("PROVIDER", "deepseek")
		t.Setenv("DEEPSEEK_API_KEY", "dk")

		got, err := loadEnvConfig("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Provider != providerDeepSeek {
			t.Errorf("Provider = %q, want %q", got.Provider, providerDeepSeek)
		}
		if got.APIKey != "dk" {
			t.Errorf("APIKey = %q, want %q", got.APIKey, "dk")
		}
	})

	t.Run("override_beats_env", func(t *testing.T) {
		clear(t)
		t.Setenv("PROVIDER", "openai")
		t.Setenv("DEEPSEEK_API_KEY", "dk")

		got, err := loadEnvConfig("deepseek")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Provider != providerDeepSeek || got.APIKey != "dk" {
			t.Errorf("got Provider=%q APIKey=%q, want deepseek/dk", got.Provider, got.APIKey)
		}
	})

	t.Run("unknown_provider_errors", func(t *testing.T) {
		clear(t)
		t.Setenv("PROVIDER", "bogus")

		_, err := loadEnvConfig("")
		if !errors.Is(err, ErrInvalidProvider) {
			t.Fatalf("error = %v, want ErrInvalidProvider", err)
		}
	})

	t.Run("deepseek_missing_key_errors", func(t *testing.T) {
		clear(t)
		t.Setenv("PROVIDER", "deepseek")

		_, err := loadEnvConfig("")
		if !errors.Is(err, ErrNoAPIKey) {
			t.Fatalf("error = %v, want ErrNoAPIKey", err)
		}
	})
}

func TestResolveProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", providerOpenAI, false},
		{"openai", providerOpenAI, false},
		{"deepseek", providerDeepSeek, false},
		{"bogus", "", true},
	}

	for _, tt := range tests {
		t.Run("in="+tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := resolveProvider(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidProvider) {
					t.Fatalf("error = %v, want ErrInvalidProvider", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Name != tt.want {
				t.Errorf("resolveProvider(%q).Name = %q, want %q", tt.in, got.Name, tt.want)
			}
		})
	}
}

// testPasswordHash is a format-valid PBKDF2 fixture; Validate only parses the
// shape, it never derives the key.
const testPasswordHash = "pbkdf2-sha256.600000.00000000000000000000000000000000." +
	"0000000000000000000000000000000000000000000000000000000000000000"

func TestLoadOAuthEnv(t *testing.T) {
	t.Run("defaults_leave_oauth_off", func(t *testing.T) {
		var cfg MCPConfig
		if err := loadOAuthEnv(&cfg); err != nil {
			t.Fatalf("loadOAuthEnv() error = %v", err)
		}
		if cfg.OAuthEnabled || cfg.AuthEnabled {
			t.Fatalf("OAuthEnabled = %v, AuthEnabled = %v; want both false", cfg.OAuthEnabled, cfg.AuthEnabled)
		}
		if cfg.OAuthAccessTTL != defaultOAuthAccessTTL || cfg.OAuthRefreshTTL != defaultOAuthRefreshTTL {
			t.Fatalf("TTLs = %v / %v", cfg.OAuthAccessTTL, cfg.OAuthRefreshTTL)
		}
	})

	t.Run("enabled_forces_auth_and_reads_values", func(t *testing.T) {
		t.Setenv("ANSWER_MCP_OAUTH", "true")
		t.Setenv("ANSWER_MCP_ISSUER", "https://karma.test/answer")
		t.Setenv("ANSWER_MCP_RESOURCE", "https://karma.test/answer/mcp")
		t.Setenv("ANSWER_MCP_LOGIN_USER", "tester")
		t.Setenv("ANSWER_MCP_LOGIN_PASSWORD_HASH", testPasswordHash)
		t.Setenv("ANSWER_MCP_ACCESS_TTL", "30m")
		t.Setenv("ANSWER_MCP_REFRESH_TTL", "96h")
		var cfg MCPConfig
		if err := loadOAuthEnv(&cfg); err != nil {
			t.Fatalf("loadOAuthEnv() error = %v", err)
		}
		if !cfg.OAuthEnabled || !cfg.AuthEnabled {
			t.Fatalf("OAuthEnabled = %v, AuthEnabled = %v; want both true", cfg.OAuthEnabled, cfg.AuthEnabled)
		}
		if cfg.OAuthIssuer != "https://karma.test/answer" || cfg.OAuthResource != "https://karma.test/answer/mcp" {
			t.Fatalf("issuer/resource = %q / %q", cfg.OAuthIssuer, cfg.OAuthResource)
		}
		if cfg.OAuthLoginUser != "tester" || cfg.OAuthLoginPasswordHash != testPasswordHash {
			t.Fatalf("login user/hash not read")
		}
		if cfg.OAuthAccessTTL != 30*time.Minute || cfg.OAuthRefreshTTL != 96*time.Hour {
			t.Fatalf("TTLs = %v / %v", cfg.OAuthAccessTTL, cfg.OAuthRefreshTTL)
		}
	})

	t.Run("invalid_bool_errors", func(t *testing.T) {
		t.Setenv("ANSWER_MCP_OAUTH", "maybe")
		var cfg MCPConfig
		err := loadOAuthEnv(&cfg)
		if err == nil || !strings.Contains(err.Error(), "ANSWER_MCP_OAUTH") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("invalid_duration_errors", func(t *testing.T) {
		t.Setenv("ANSWER_MCP_ACCESS_TTL", "soon")
		var cfg MCPConfig
		err := loadOAuthEnv(&cfg)
		if err == nil || !strings.Contains(err.Error(), "ANSWER_MCP_ACCESS_TTL") {
			t.Fatalf("err = %v", err)
		}
	})
}

// validOAuthConfig returns a config that passes Validate with OAuth on.
func validOAuthConfig() MCPConfig {
	return parseMCPConfig(MCPConfig{
		APIKey:                 "k",
		AuthEnabled:            true,
		AuthSecretKey:          testSecret,
		OAuthEnabled:           true,
		OAuthIssuer:            "https://karma.test/answer",
		OAuthResource:          "https://karma.test/answer/mcp",
		OAuthLoginUser:         "tester",
		OAuthLoginPasswordHash: testPasswordHash,
		OAuthAccessTTL:         time.Hour,
		OAuthRefreshTTL:        2 * time.Hour,
	})
}

func TestMCPConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*MCPConfig)
		wantErr string // substring; empty means Validate must pass
	}{
		{
			name:   "valid_oauth_config_passes",
			mutate: func(*MCPConfig) {},
		},
		{
			name:   "disabled_passes_without_secret",
			mutate: func(c *MCPConfig) { c.OAuthEnabled, c.AuthEnabled, c.AuthSecretKey = false, false, "" },
		},
		{
			name:    "auth_requires_secret",
			mutate:  func(c *MCPConfig) { c.OAuthEnabled, c.AuthSecretKey = false, "" },
			wantErr: "ANSWER_AUTH_SECRET_KEY",
		},
		{
			name:    "oauth_requires_long_secret",
			mutate:  func(c *MCPConfig) { c.AuthSecretKey = "short" },
			wantErr: "32",
		},
		{
			name:    "oauth_requires_absolute_issuer",
			mutate:  func(c *MCPConfig) { c.OAuthIssuer = "karma.test/answer" },
			wantErr: "issuer",
		},
		{
			name:    "oauth_requires_login_user",
			mutate:  func(c *MCPConfig) { c.OAuthLoginUser = "" },
			wantErr: "LoginUser",
		},
		{
			name:    "oauth_requires_valid_hash",
			mutate:  func(c *MCPConfig) { c.OAuthLoginPasswordHash = "nope" },
			wantErr: "LoginPasswordHash",
		},
		{
			name:    "refresh_ttl_must_exceed_access_ttl",
			mutate:  func(c *MCPConfig) { c.OAuthRefreshTTL = c.OAuthAccessTTL },
			wantErr: "RefreshTokenTTL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := validOAuthConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() unexpected error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseMCPConfig_OAuthForcesAuth(t *testing.T) {
	t.Parallel()

	if got := parseMCPConfig(MCPConfig{OAuthEnabled: true}); !got.AuthEnabled {
		t.Fatal("parseMCPConfig did not force AuthEnabled when OAuthEnabled")
	}
}

func TestOAuthConfigDerivation(t *testing.T) {
	t.Parallel()

	cfg := validOAuthConfig()
	derived, err := cfg.OAuthConfig()
	if err != nil {
		t.Fatalf("OAuthConfig() error = %v", err)
	}
	if derived.Issuer != cfg.OAuthIssuer || derived.Resource != cfg.OAuthResource {
		t.Fatalf("issuer/resource = %q / %q", derived.Issuer, derived.Resource)
	}
	if derived.Scope != "answer" || derived.Realm != "answer-mcp" {
		t.Fatalf("scope/realm = %q / %q, want answer / answer-mcp", derived.Scope, derived.Realm)
	}
	signer, ok := derived.Signer.(HS256Signer)
	if !ok || signer.Secret != testSecret || signer.Issuer != cfg.OAuthIssuer || signer.Audience != cfg.OAuthResource {
		t.Fatalf("signer = %+v", derived.Signer)
	}
}
