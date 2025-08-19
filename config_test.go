package main

import (
	"errors"
	"testing"
	"time"
)

func TestLoadEnvConfig_Table(t *testing.T) {
	type want struct {
		err         error
		apiKey      string
		question    string
		model       string
		effort      string
		showAll     bool
		hasShowAll  bool
		timeout     time.Duration
		hasTimeout  bool
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
		t.Setenv("QUESTION", "")
		t.Setenv("MODEL", "")
		t.Setenv("EFFORT", "")
		t.Setenv("SHOW_ALL", "")
		t.Setenv("TIMEOUT", "")
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			clearAll(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			got, err := loadEnvConfig()

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

	got := parseMCPConfig("", "", "", "", "", false)

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
		APIKey:    "k",
		BaseURL:   "http://example.local",
		Transport: "http",
		Port:      "9090",
		Host:      "0.0.0.0",
		Verbose:   true,
	}

	got := parseMCPConfig(
		want.APIKey,
		want.BaseURL,
		want.Transport,
		want.Port,
		want.Host,
		want.Verbose,
	)

	if got != want {
		t.Errorf("parseMCPConfig = %+v, want %+v", got, want)
	}
}

func TestGetTimeoutForEffort(t *testing.T) {
	tests := []struct {
		effort string
		want   time.Duration
	}{
		{"minimal", timeoutMinimal},
		{"low", timeoutLow},
		{"medium", timeoutMedium},
		{"high", timeoutHigh},
		{"", timeoutLow},        // empty maps to low per implementation
		{"unknown", timeoutLow}, // unknown maps to low per implementation
	}

	for _, tt := range tests {
		tt := tt
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
		{"minimal", "minimal"},
		{"low", "low"},
		{"medium", "medium"},
		{"high", "high"},
		{"", defaultEffort},
		{"weird", defaultEffort},
	}

	for _, tt := range tests {
		tt := tt
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
		tt := tt
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			if got := validateVerbosity(tt.in); got != tt.want {
				t.Errorf("validateVerbosity(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
