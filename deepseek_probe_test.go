//go:build deepseek

package main

// DeepSeek Responses API probe. Excluded from normal test runs via build tag.
// Run explicitly with:
//
//	GOEXPERIMENT=jsonv2 DEEPSEEK_API_KEY=sk-... \
//	  go test -tags deepseek -run TestDeepSeek -v -timeout 30m .

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const deepseekBase = "https://api.deepseek.com/responses"

func dsKey(t *testing.T) string {
	t.Helper()
	if k := os.Getenv("DEEPSEEK_API_KEY"); k != "" {
		return k
	}
	t.Skip("DEEPSEEK_API_KEY not set")
	return ""
}

// callRaw posts an arbitrary requestBody to DeepSeek, bypassing CallAPI's
// fixed body construction (needed to probe tool-type variants).
func callRaw(ctx context.Context, key string, body requestBody) (*apiResponse, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deepseekBase, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var ar apiResponse
	if err := json.Unmarshal(raw, &ar); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return &ar, nil
}

func dsParams(key, model, effort string, useWebSearch bool) CallAPIParams {
	return CallAPIParams{
		APIKey:       key,
		BaseURL:      deepseekBase,
		Query:        "What is the capital of Luxembourg?",
		Model:        model,
		Effort:       effort,
		Verbosity:    "medium",
		Timeout:      3 * time.Minute,
		UseWebSearch: useWebSearch,
	}
}

// Probe 1: basic non-streaming call through the production CallAPI path.
// Verifies response shape (id, message items, output_text) with ExtractAnswer.
func TestDeepSeekBasic(t *testing.T) {
	resp, err := CallAPI(context.Background(), dsParams(dsKey(t), "deepseek-v4-flash", "medium", false))
	if err != nil {
		t.Fatalf("basic call failed: %v", err)
	}
	answer := ExtractAnswer(resp)
	t.Logf("id=%s model=%s answer=%q", resp.ID, resp.Model, truncate(answer, 200))
	if resp.ID == "" {
		t.Error("response missing id")
	}
	if answer == "" {
		t.Error("ExtractAnswer returned empty string")
	}
	if !strings.Contains(strings.ToLower(answer), "luxembourg") {
		t.Errorf("answer does not mention Luxembourg: %q", truncate(answer, 200))
	}
}

// Probe 2: DeepSeek's own web_search tool with a current-events query,
// so an ungrounded answer is detectable.
func TestDeepSeekWebSearch(t *testing.T) {
	key := dsKey(t)
	body := requestBody{
		Model:     "deepseek-v4-flash",
		Input:     "What major world news happened in the last 48 hours? Give two examples with dates.",
		Reasoning: reqReasoning{Effort: "medium"},
		Text:      reqText{Verbosity: "medium"},
		Tools:     []reqTool{{Type: "web_search"}},
	}
	resp, err := callRaw(context.Background(), key, body)
	if err != nil {
		t.Fatalf("web_search call failed: %v", err)
	}
	answer := ExtractAnswer(resp)
	t.Logf("answer=%q", truncate(answer, 400))
	if answer == "" {
		t.Error("ExtractAnswer returned empty string")
	}
	for _, item := range resp.Output {
		if item.Type == "web_search_call" {
			t.Log("web_search_call item present in output")
			return
		}
	}
	t.Error("no web_search_call item in output — search may not have executed")
}

// Probe 3: our current tool type (web_search_preview) against DeepSeek.
// Docs say unknown tool types are silently ignored — verify no error, no search.
func TestDeepSeekOpenAIToolTypeIgnored(t *testing.T) {
	resp, err := CallAPI(context.Background(), dsParams(dsKey(t), "deepseek-v4-flash", "low", true))
	if err != nil {
		t.Fatalf("call with web_search_preview failed (expected silent ignore): %v", err)
	}
	for _, item := range resp.Output {
		if item.Type == "web_search_call" {
			t.Error("unexpected web_search_call — web_search_preview should be ignored")
		}
	}
	t.Logf("id=%s answer=%q", resp.ID, truncate(ExtractAnswer(resp), 120))
}

// Probe 4: effort enum acceptance. Our registry spans none..xhigh; DeepSeek
// docs list none/minimal/low/medium/high/xhigh/max. Probe each value.
func TestDeepSeekEffortEnum(t *testing.T) {
	key := dsKey(t)
	for _, effort := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"} {
		t.Run(effort, func(t *testing.T) {
			p := dsParams(key, "deepseek-v4-flash", effort, false)
			p.Query = "Reply with just the word: ok"
			p.Timeout = 2 * time.Minute
			resp, err := CallAPI(context.Background(), p)
			if err != nil {
				t.Errorf("effort=%s rejected: %v", effort, err)
				return
			}
			t.Logf("effort=%s accepted, echoed effort=%q", effort, resp.Reasoning.Effort)
		})
	}
}

// Probe 5: unsupported fields silently ignored per docs.
func TestDeepSeekUnsupportedFieldsIgnored(t *testing.T) {
	p := dsParams(dsKey(t), "deepseek-v4-flash", "low", false)
	p.Query = "Reply with just the word: ok"
	p.PreviousResponseID = "resp_nonexistent_probe"
	p.PromptCacheKey = "answer-probe"
	resp, err := CallAPI(context.Background(), p)
	if err != nil {
		t.Fatalf("previous_response_id/prompt_cache_key not silently ignored: %v", err)
	}
	t.Logf("id=%s answer=%q", resp.ID, truncate(ExtractAnswer(resp), 80))
}

// Probe 6: deepseek-v4-pro availability (docs said "early August 2026").
func TestDeepSeekProModel(t *testing.T) {
	p := dsParams(dsKey(t), "deepseek-v4-pro", "medium", false)
	p.Query = "Reply with just the word: ok"
	resp, err := CallAPI(context.Background(), p)
	if err != nil {
		t.Skipf("deepseek-v4-pro not available yet: %v", err)
	}
	t.Logf("pro available: id=%s model=%s", resp.ID, resp.Model)
}

// Probe 7: dump the output item structure for a search query. Determines
// whether search-process narration arrives as separate message items
// (which ExtractAnswer currently concatenates into the answer).
func TestDeepSeekOutputItemStructure(t *testing.T) {
	key := dsKey(t)
	body := requestBody{
		Model:     "deepseek-v4-flash",
		Input:     "What major world news happened in the last 48 hours? Give two examples with dates.",
		Reasoning: reqReasoning{Effort: "medium"},
		Text:      reqText{Verbosity: "medium"},
		Tools:     []reqTool{{Type: "web_search"}},
	}
	resp, err := callRaw(context.Background(), key, body)
	if err != nil {
		t.Fatalf("web_search call failed: %v", err)
	}
	for i, item := range resp.Output {
		t.Logf("item[%d] type=%s", i, item.Type)
		for j, c := range item.Content {
			t.Logf("  content[%d] type=%s text=%q", j, c.Type, truncate(c.Text, 160))
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
