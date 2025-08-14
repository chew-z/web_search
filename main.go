package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

const defaultPrompt = "Who won Wimbledon 2025"

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

// fail prints to stderr and exits non-zero.
func fail(code int, msg string, args ...any) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(code)
}

// EnvConfig centralizes environment-derived configuration.
// Fields with companion Has* indicate whether the env var was set and parsed.
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

// loadEnvConfig reads environment variables, parses typed values, and validates
// required settings. It does not fail on malformed optional values; instead, it
// leaves Has* as false so callers can fall back to defaults. It validates that
// OPENAI_API_KEY is present and non-empty.
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
		return EnvConfig{}, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	return cfg, nil
}

func main() {
	// Load environment-backed configuration (dotenv auto-loads via side-effect import).
	envCfg, err := loadEnvConfig()
	if err != nil {
		fail(2, err.Error())
	}

	// Decide defaults using env when set; CLI flags override env; positional can override question if flag not set.
	var (
		baseURL = flag.String("base", "https://api.openai.com/v1/responses", "API endpoint")
		model   = flag.String("model", func() string {
			if envCfg.Model != "" {
				return envCfg.Model
			}
			return "gpt-5"
		}(), "model (env MODEL)")
		effort = flag.String("effort", func() string {
			if envCfg.Effort != "" {
				return envCfg.Effort
			}
			return "low"
		}(), "effort (env EFFORT)")
		// Support both -q and -question, seeded from env QUESTION if present.
		questionVal string
		timeout     = flag.Duration("timeout", func() time.Duration {
			if envCfg.HasTimeout {
				return envCfg.Timeout
			}
			return 300 * time.Second
		}(), "HTTP timeout (env TIMEOUT)")
		showAll = flag.Bool("show-all", func() bool {
			if envCfg.HasShowAll {
				return envCfg.ShowAll
			}
			return false
		}(), "print raw JSON response (env SHOW_ALL)")
	)
	flag.StringVar(&questionVal, "q", envCfg.Question, "question prompt (env QUESTION; default: '"+defaultPrompt+"')")
	flag.StringVar(&questionVal, "question", envCfg.Question, "same as -q (env QUESTION)")
	flag.Parse()

	key := envCfg.APIKey

	// Determine the final question value with precedence:
	// 1) If -q/-question explicitly set, use it
	// 2) Else if positional arg present, use it
	// 3) Else if env QUESTION provided (via default), it is already in questionVal
	// 4) Else fall back to defaultPrompt
	q := questionVal
	var questionFlagSet bool
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "q" || f.Name == "question" {
			questionFlagSet = true
		}
	})
	if !questionFlagSet {
		if flag.NArg() > 0 {
			q = flag.Arg(0)
		}
	}
	if q == "" {
		q = defaultPrompt
	}

	body := requestBody{
		Model: *model,
		Input: q,
		Reasoning: reqReasoning{
			Effort: *effort,
		},
		Tools: []reqTool{
			{Type: "web_search_preview"},
		},
	}

	buf, err := json.Marshal(body)
	if err != nil {
		fail(2, "marshal request: %v", err)
	}

	client := &http.Client{Timeout: *timeout}
	req, err := http.NewRequest(http.MethodPost, *baseURL, bytes.NewReader(buf))
	if err != nil {
		fail(2, "build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := client.Do(req)
	if err != nil {
		fail(2, "http request: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fail(2, "read response: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fail(2, "api error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	if *showAll {
		// Print the full raw JSON when requested and exit.
		fmt.Println(string(bodyBytes))
		return
	}

	var ar apiResponse
	if err := json.Unmarshal(bodyBytes, &ar); err != nil {
		fail(2, "parse json: %v\nraw=%s", err, string(bodyBytes))
	}

	// Extract same fields your jq pulled:
	// .output[] | select(.type=="message") | .content[] | select(.type=="output_text") | .text
	var printed int
	for _, it := range ar.Output {
		if it.Type != "message" {
			continue
		}
		for _, c := range it.Content {
			if c.Type == "output_text" && c.Text != "" {
				fmt.Println(c.Text)
				printed++
			}
		}
	}

	if printed == 0 {
		// Helpful debug if schema shifts.
		fail(3, "no output_text found in response")
	}
}
