package main

import "fmt"

// Provider names selectable via the PROVIDER env var or the -provider flag.
const (
	providerOpenAI   = "openai"
	providerDeepSeek = "deepseek"
	defaultProvider  = providerOpenAI

	// DeepSeek endpoints and models (Responses API).
	deepseekBaseURL    = "https://api.deepseek.com/responses"
	modelDeepSeekFlash = "deepseek-v4-flash"
)

// provider describes an upstream Responses API backend. The registry is the
// single source of truth for per-provider endpoint, auth, tool and prompt
// differences; everything else looks providers up by name.
type provider struct {
	Name               string
	KeyEnv             string
	DefaultBaseURL     string
	WebSearchTool      string
	DefaultModel       string
	SupportsContinuity bool // previous_response_id / prompt_cache_key
	Models             []modelInfo
	Prompt             string
}

var providerRegistry = map[string]provider{
	providerOpenAI: {
		Name:               providerOpenAI,
		KeyEnv:             "OPENAI_API_KEY",
		DefaultBaseURL:     defaultBaseURL,
		WebSearchTool:      "web_search_preview",
		DefaultModel:       defaultModel,
		SupportsContinuity: true,
		Models:             modelRegistry,
		Prompt:             webSearchPrompt,
	},
	providerDeepSeek: {
		Name:               providerDeepSeek,
		KeyEnv:             "DEEPSEEK_API_KEY",
		DefaultBaseURL:     deepseekBaseURL,
		WebSearchTool:      "web_search",
		DefaultModel:       modelDeepSeekFlash,
		SupportsContinuity: false,
		Models: []modelInfo{
			{modelDeepSeekFlash, "Deep reasoning with server-side web search (deepseek-v4-pro coming soon)", "medium"},
		},
		Prompt: deepseekWebSearchPrompt,
	},
}

// providerByName looks up a provider by canonical name.
func providerByName(name string) (provider, bool) {
	p, ok := providerRegistry[name]
	return p, ok
}

// resolveProvider returns the provider for the given name, applying the
// default when name is empty and ErrInvalidProvider for unknown names.
func resolveProvider(name string) (provider, error) {
	if name == "" {
		name = defaultProvider
	}
	p, ok := providerByName(name)
	if !ok {
		return provider{}, fmt.Errorf("%w: %q (valid: %s, %s)", ErrInvalidProvider, name, providerOpenAI, providerDeepSeek)
	}
	return p, nil
}

// providerOrDefault returns the provider for name, falling back to the
// default provider for empty or unknown names. Use at surfaces where the
// name was already validated upstream (e.g. MCP handlers).
func providerOrDefault(name string) provider {
	p, err := resolveProvider(name)
	if err != nil {
		return providerRegistry[defaultProvider]
	}
	return p
}
