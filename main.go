package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
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

func main() {
	var (
		baseURL = flag.String("base", "https://api.openai.com/v1/responses", "API endpoint")
		model   = flag.String("model", "gpt-5", "model")
		prompt  = flag.String("q", "", "prompt (default: '"+defaultPrompt+"')")
		timeout = flag.Duration("timeout", 60*time.Second, "HTTP timeout")
	)
	flag.Parse()

	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		fail(2, "OPENAI_API_KEY is not set")
	}

	q := *prompt
	if q == "" {
		// If user passed a positional arg, use it; else default.
		if flag.NArg() > 0 {
			q = flag.Arg(0)
		} else {
			q = defaultPrompt
		}
	}

	body := requestBody{
		Model: *model,
		Input: q,
		Reasoning: reqReasoning{
			Effort: "low",
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
