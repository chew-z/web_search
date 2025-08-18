package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

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
	)

	// Add web search tool
	mcpServer.AddTool(
		mcp.NewTool("gpt_websearch",
			mcp.WithDescription("Search the web using OpenAI's GPT model with web search capabilities"),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("The search query or question to ask"),
			),
			mcp.WithString("model",
				mcp.DefaultString(defaultModel),
				mcp.Description("The GPT model to use (default: gpt-5-mini)"),
			),
			mcp.WithString("reasoning_effort",
				mcp.DefaultString(defaultEffort),
				mcp.Description("Reasoning effort level: minimal (90s), low (3min), medium (5min), or high (10min timeout)"),
				mcp.Enum("minimal", "low", "medium", "high"),
			),
			mcp.WithString("verbosity",
				mcp.DefaultString(defaultVerbosity),
				mcp.Description("Response verbosity level: low (concise), medium (balanced), or high (detailed with explanations)"),
				mcp.Enum("low", "medium", "high"),
			),
			mcp.WithString("previous_response_id",
				mcp.Description("Optional: Previous response ID for conversation continuity - improves performance by avoiding re-reasoning"),
			),
		),
		webSearchHandler(cfg.APIKey, cfg.BaseURL),
	)

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

// webSearchHandler returns a handler for the web search tool
func webSearchHandler(apiKey, baseURL string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Get the server from context to send log messages
		mcpServer := server.ServerFromContext(ctx)

		// Extract parameters
		query, err := request.RequireString("query")
		if err != nil {
			if mcpServer != nil {
				_ = mcpServer.SendLogMessageToClient(ctx, mcp.NewLoggingMessageNotification(
					mcp.LoggingLevelError,
					"web_search",
					fmt.Sprintf("Failed to extract query parameter: %v", err),
				))
			}
			return mcp.NewToolResultError(err.Error()), nil
		}

		model := request.GetString("model", defaultModel)
		effort := request.GetString("reasoning_effort", defaultEffort)
		verbosity := request.GetString("verbosity", defaultVerbosity)
		previousResponseID := request.GetString("previous_response_id", "")

		// Log the search request
		if mcpServer != nil {
			_ = mcpServer.SendLogMessageToClient(ctx, mcp.NewLoggingMessageNotification(
				mcp.LoggingLevelInfo,
				"web_search",
				fmt.Sprintf("Executing web search: query='%s', model='%s', effort='%s', verbosity='%s'", query, model, effort, verbosity),
			))
		}

		// Call handler with properly extracted values
		args := map[string]interface{}{
			"query":                query,
			"model":                model,
			"reasoning_effort":     effort,
			"verbosity":            verbosity,
			"previous_response_id": previousResponseID,
		}

		result, err := HandleWebSearch(ctx, apiKey, baseURL, args)
		if err != nil {
			if mcpServer != nil {
				_ = mcpServer.SendLogMessageToClient(ctx, mcp.NewLoggingMessageNotification(
					mcp.LoggingLevelError,
					"web_search",
					fmt.Sprintf("Web search failed: %v", err),
				))
			}
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Log success
		if mcpServer != nil {
			_ = mcpServer.SendLogMessageToClient(ctx, mcp.NewLoggingMessageNotification(
				mcp.LoggingLevelInfo,
				"web_search",
				"Web search completed successfully",
			))
		}

		// Convert result to JSON string for text content
		resultJSON, err := json.Marshal(result)
		if err != nil {
			if mcpServer != nil {
				_ = mcpServer.SendLogMessageToClient(ctx, mcp.NewLoggingMessageNotification(
					mcp.LoggingLevelError,
					"web_search",
					fmt.Sprintf("Failed to marshal result: %v", err),
				))
			}
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(resultJSON)), nil
	}
}

// serverInfoHandler returns a handler for the server info resource
func serverInfoHandler(baseURL string) func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Get the server from context to send log messages
		mcpServer := server.ServerFromContext(ctx)

		// Log the resource access
		if mcpServer != nil {
			_ = mcpServer.SendLogMessageToClient(ctx, mcp.NewLoggingMessageNotification(
				mcp.LoggingLevelDebug,
				"server_info",
				fmt.Sprintf("Server info resource accessed: URI=%s", request.Params.URI),
			))
		}

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

// webSearchPromptHandler returns a handler for the intelligent web search prompt
func webSearchPromptHandler() func(context.Context, mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		// Get the server from context to send log messages
		mcpServer := server.ServerFromContext(ctx)

		userQuestion := request.Params.Arguments["user_question"]
		if userQuestion == "" {
			if mcpServer != nil {
				_ = mcpServer.SendLogMessageToClient(ctx, mcp.NewLoggingMessageNotification(
					mcp.LoggingLevelError,
					"web_search_prompt",
					"user_question parameter is required",
				))
			}
			return nil, fmt.Errorf("user_question parameter is required")
		}

		// Log the prompt request
		if mcpServer != nil {
			_ = mcpServer.SendLogMessageToClient(ctx, mcp.NewLoggingMessageNotification(
				mcp.LoggingLevelDebug,
				"web_search_prompt",
				fmt.Sprintf("Generating prompt for question: %s", userQuestion),
			))
		}

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
