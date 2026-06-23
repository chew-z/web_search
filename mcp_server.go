package main

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// logToClient centralizes logging to MCP clients and stderr on failure
func logToClient(ctx context.Context, level mcp.LoggingLevel, source, message string) {
	mcpServer := server.ServerFromContext(ctx)
	if mcpServer == nil {
		return
	}
	if err := mcpServer.SendLogMessageToClient(ctx, mcp.NewLoggingMessageNotification(level, source, message)); err != nil {
		fmt.Fprintf(os.Stderr, "failed to send log message: %v\n", err)
	}
}

// NewMCPServer creates and configures an MCP server with tools, resources, and prompts
func NewMCPServer(cfg MCPConfig) *server.MCPServer {
	// Create MCP server with capabilities
	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithLogging(),
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false),
		server.WithPromptCapabilities(true),
		server.WithTitle(serverTitle),
		server.WithWebsiteURL(serverWebsiteURL),
		server.WithRecovery(),
		server.WithInputSchemaValidation(),
		server.WithOutputSchemaValidation(),
	)

	// Add web search tool
	mcpServer.AddTool(newGptWebsearchTool(), webSearchHandler(cfg.APIKey, cfg.BaseURL))

	// Add server info resource
	mcpServer.AddResource(
		mcp.NewResource(
			"server://info",
			"Server Information",
			mcp.WithResourceDescription("Information about the GPT Web Search MCP server"),
			mcp.WithMIMEType("text/plain"),
		),
		serverInfoHandler(cfg.BaseURL),
	)

	// Add models list resource
	mcpServer.AddResource(
		mcp.NewResource(
			"models://list",
			"Available Models",
			mcp.WithResourceDescription("List of available GPT models with use cases and recommended parameters"),
			mcp.WithMIMEType("application/json"),
		),
		modelsHandler(),
	)

	// Add intelligent web search prompt
	mcpServer.AddPrompt(
		mcp.NewPrompt("web_search",
			mcp.WithPromptDescription("Use the gpt_websearch tool to answer user questions based on web searching"),
			mcp.WithArgument("user_question",
				mcp.RequiredArgument(),
				mcp.ArgumentDescription("The question, task, problem, or instructions from the user that requires web search"),
			),
		),
		webSearchPromptHandler(),
	)

	return mcpServer
}

// newGptWebsearchTool builds the gpt_websearch tool definition with input
// validation (additionalProperties:false, enum constraints) and a structured
// output schema derived from WebSearchResult.
func newGptWebsearchTool() mcp.Tool {
	return mcp.NewTool("gpt_websearch",
		mcp.WithDescription("Search the web using OpenAI's GPT model with web search capabilities"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query or question to ask"),
		),
		mcp.WithString("model",
			mcp.DefaultString(defaultModel),
			mcp.Description("The GPT model to use (default: gpt-5.4-mini)"),
		),
		mcp.WithString("reasoning_effort",
			mcp.DefaultString(defaultEffort),
			mcp.Description(effortDescription()),
			mcp.Enum(effortEnumValues()...),
		),
		mcp.WithString("verbosity",
			mcp.DefaultString(defaultVerbosity),
			mcp.Description("Response verbosity level: low (concise), medium (balanced), or high (detailed with explanations)"),
			mcp.Enum(verbosityLevels...),
		),
		mcp.WithString("previous_response_id",
			mcp.Description("Optional: Previous response ID for conversation continuity - improves performance by avoiding re-reasoning"),
		),
		mcp.WithString("prompt_cache_key",
			mcp.Description("Optional: OpenAI prompt_cache_key. Requests sharing the same prefix and key "+
				"reuse the same cache shard. Leave empty to use the server default (per-user when "+
				"authenticated, otherwise server-wide).")),
		mcp.WithBoolean("web_search",
			mcp.DefaultBool(true),
			mcp.Description("Use web search (default: true)"),
		),
		mcp.WithSchemaAdditionalProperties(false),
		mcp.WithOutputSchema[WebSearchResult](),
	)
}

// webSearchHandler returns a handler for the web search tool.
// Authentication is enforced at the HTTP transport layer (newAuthHTTPMiddleware)
// before this handler is ever reached; no auth logic is needed here.
// User identity is logged opportunistically when present in the context
// (set by the middleware on authenticated HTTP requests).
func webSearchHandler(apiKey, baseURL string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()

		// Log authenticated user when identity is available (HTTP transport).
		if userID, username := getUserInfo(ctx); userID != "" {
			logToClient(ctx, mcp.LoggingLevelDebug, "web_search", fmt.Sprintf("authenticated user: %s (%s)", username, userID))
		}

		// Extract parameters
		query, err := request.RequireString("query")
		if err != nil {
			logToClient(ctx, mcp.LoggingLevelError, "web_search", fmt.Sprintf("missing query parameter: %v", err))
			return mcp.NewToolResultError(err.Error()), nil
		}

		model := request.GetString("model", defaultModel)
		effort := request.GetString("reasoning_effort", defaultEffort)
		verbosity := request.GetString("verbosity", defaultVerbosity)
		previousResponseID := request.GetString("previous_response_id", "")
		promptCacheKey := request.GetString("prompt_cache_key", "")
		webSearch := request.GetBool("web_search", true)

		// Log the search request
		logToClient(ctx, mcp.LoggingLevelInfo, "web_search", fmt.Sprintf(
			"request: query='%s', model='%s', effort='%s', verbosity='%s', web_search=%t",
			query, model, effort, verbosity, webSearch))

		result, err := HandleWebSearch(ctx, apiKey, baseURL, WebSearchParams{
			Query:              query,
			Model:              model,
			Effort:             effort,
			Verbosity:          verbosity,
			PreviousResponseID: previousResponseID,
			PromptCacheKey:     promptCacheKey,
			UseWebSearch:       webSearch,
		})
		if err != nil {
			duration := time.Since(start)
			Info("web search failed", "query", query, "model", model, "effort", effort, "duration", duration.String(), "error", err)
			logToClient(ctx, mcp.LoggingLevelError, "web_search", fmt.Sprintf("search failed after %s: %v", duration, err))
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Log success with duration and result metadata
		duration := time.Since(start)
		answerLen := 0
		if result != nil {
			answerLen = len(result.Answer)
		}
		Info(
			"web search completed",
			"query", query,
			"model", model,
			"effort", effort,
			"duration", duration.String(),
			"answer_chars", answerLen,
			"web_search", webSearch,
		)
		logToClient(ctx, mcp.LoggingLevelInfo, "web_search", fmt.Sprintf(
			"completed: model=%s effort=%s duration=%s answer=%d chars",
			model, effort, duration, answerLen))

		// Return structured JSON content rather than a JSON string
		return mcp.NewToolResultStructuredOnly(result), nil
	}
}

// serverInfoHandler returns a handler for the server info resource
func serverInfoHandler(baseURL string) func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Log the resource access
		logToClient(ctx, mcp.LoggingLevelDebug, "server_info", fmt.Sprintf("Server info resource accessed: URI=%s", request.Params.URI))

		info := fmt.Sprintf("GPT Web Search MCP Server\nVersion: %s\nEndpoint: %s\n", serverVersion, baseURL)
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "text/plain",
				Text:     info,
			},
		}, nil
	}
}

// modelsHandler returns a handler for the models list resource
func modelsHandler() func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	type modelEntry struct {
		Name              string `json:"name"`
		Description       string `json:"description"`
		RecommendedEffort string `json:"recommended_effort"`
		Timeout           string `json:"timeout"`
	}
	type modelsPayload struct {
		Default string       `json:"default"`
		Models  []modelEntry `json:"models"`
	}

	payload := modelsPayload{
		Default: modelMini,
		Models:  make([]modelEntry, 0, len(modelRegistry)),
	}
	for _, m := range modelRegistry {
		// DisplayTimeout is derived from the effort registry — single source,
		// zero duplication. The drift-guard test asserts every RecommendedEffort
		// resolves, so the comma-ok miss path is defensive only.
		display := ""
		if e, ok := effortByName(m.RecommendedEffort); ok {
			display = e.DisplayTimeout
		}
		payload.Models = append(payload.Models, modelEntry{
			Name:              m.Name,
			Description:       m.Description,
			RecommendedEffort: m.RecommendedEffort,
			Timeout:           display,
		})
	}

	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(`{"error":"failed to marshal models"}`)
	}
	text := string(data)

	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		logToClient(ctx, mcp.LoggingLevelDebug, "models_list", fmt.Sprintf("Models list resource accessed: URI=%s", request.Params.URI))
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "application/json",
				Text:     text,
			},
		}, nil
	}
}

// webSearchPromptHandler returns a handler for the intelligent web search prompt
func webSearchPromptHandler() func(context.Context, mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		userQuestion := request.Params.Arguments["user_question"]
		if userQuestion == "" {
			logToClient(ctx, mcp.LoggingLevelError, "web_search_prompt", "user_question parameter is required")
			return nil, fmt.Errorf("user_question parameter is required")
		}

		// Log the prompt request
		logToClient(ctx, mcp.LoggingLevelDebug, "web_search_prompt", fmt.Sprintf("Generating prompt for question: %s", userQuestion))

		// Return properly structured messages with system and user roles
		messages := []mcp.PromptMessage{
			{
				Role: "user",
				Content: mcp.TextContent{
					Type: "text",
					Text: webSearchPrompt + "\n<user_question>\n" + userQuestion + "\n</user_question>\n",
				},
			},
		}

		return &mcp.GetPromptResult{
			Messages: messages,
		}, nil
	}
}
