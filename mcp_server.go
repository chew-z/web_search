package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RunMCPServer initializes and runs the MCP server
func RunMCPServer() {
	// Create a new flag set for MCP subcommand
	mcpFlags := flag.NewFlagSet("mcp", flag.ExitOnError)

	var (
		transport     = mcpFlags.String("t", "stdio", "Transport type")
		transportLong = mcpFlags.String("transport", "", "Transport type (overrides -t)")
		port          = mcpFlags.String("port", "8080", "HTTP server port")
		baseURL       = mcpFlags.String("base", defaultBaseURL, "API base URL")
		verbose       = mcpFlags.Bool("verbose", false, "Enable verbose logging")
	)

	// Parse MCP-specific flags (skip "answer mcp" args)
	mcpFlags.Parse(os.Args[2:]) //nolint:errcheck // Flag parsing error handling done by FlagSet

	// Use long form if provided
	if *transportLong != "" {
		*transport = *transportLong
	}

	// Configure logging
	if !*verbose {
		log.SetOutput(os.Stderr)
	}

	// Load environment config
	envCfg, err := loadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create MCP server
	mcpServer := CreateMCPServer(envCfg.APIKey, *baseURL)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	switch *transport {
	case "stdio":
		if err := RunStdioTransport(ctx, mcpServer, sigChan); err != nil {
			log.Fatalf("STDIO transport error: %v", err)
		}
	case "http":
		if err := RunHTTPTransport(ctx, mcpServer, *port, sigChan); err != nil {
			log.Fatalf("HTTP transport error: %v", err)
		}
	default:
		log.Fatalf("Unknown transport: %s (use 'stdio' or 'http')", *transport)
	}
}

// CreateMCPServer creates and configures the MCP server with tools, resources, and prompts
func CreateMCPServer(apiKey, baseURL string) *server.MCPServer {
	// Create MCP server with capabilities
	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false),
		server.WithPromptCapabilities(true),
	)

	// Add web search tool using fluent API
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
				mcp.Description("Reasoning effort level: low (3min), medium (5min), or high (10min timeout)"),
				mcp.Enum("low", "medium", "high"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Use proper extraction methods
			query, err := request.RequireString("query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			model := request.GetString("model", defaultModel)
			effort := request.GetString("reasoning_effort", defaultEffort)

			// Call handler with properly extracted values
			args := map[string]interface{}{
				"query":            query,
				"model":            model,
				"reasoning_effort": effort,
			}

			result, err := HandleWebSearch(ctx, apiKey, baseURL, args)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Convert result to JSON string for text content
			resultJSON, _ := json.Marshal(result) //nolint:errcheck // JSON marshal for simple types, error ok to ignore
			return mcp.NewToolResultText(string(resultJSON)), nil
		},
	)

	// Add server info resource using fluent API
	mcpServer.AddResource(
		mcp.NewResource(
			"server://info",
			"Server Information",
			mcp.WithResourceDescription("Information about the GPT Web Search MCP server"),
			mcp.WithMIMEType("text/plain"),
		),
		func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			info := fmt.Sprintf("GPT Web Search MCP Server\nVersion: %s\nEndpoint: %s\n", serverVersion, baseURL)
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      request.Params.URI,
					MIMEType: "text/plain",
					Text:     info,
				},
			}, nil
		},
	)

	// Add web search prompt using fluent API
	mcpServer.AddPrompt(
		mcp.NewPrompt("intelligent_web_search",
			mcp.WithPromptDescription("Use the gpt_websearch tool to answer user questions based on web searching"),
			mcp.WithArgument("user_question",
				mcp.RequiredArgument(),
				mcp.ArgumentDescription("The question, task, problem, or instructions from the user that requires web search"),
			),
		),
		func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			userQuestion := request.Params.Arguments["user_question"]
			if userQuestion == "" {
				return nil, fmt.Errorf("user_question parameter is required")
			}

			// System instructions for using the web search tool
			systemPrompt := `You have access to the gpt_websearch tool that performs web searches using OpenAI's GPT models. This tool searches the web, gathers sources, reads them, and provides a single comprehensive answer.

CRITICAL: You MUST use the gpt_websearch tool to answer the user's question. Do not rely on your training data alone.

## Model Selection (choose cost-effectively):
- gpt-5-nano: Simple facts, definitions, quick lookups, basic summaries
- gpt-5-mini: Well-defined research tasks, comparisons, specific topics with clear scope
- gpt-5: Complex analysis, coding questions, multi-faceted problems, reasoning tasks

## Reasoning Effort Selection:
- low: Factual queries, simple definitions, straightforward questions (3 min timeout)
- medium: Research requiring synthesis, comparisons, moderate complexity (5 min timeout)
- high: Complex analysis, multi-part questions, deep research (10 min timeout)

## Search Strategy:
1. ANALYZE the user's question in the context of our conversation
2. FORMULATE detailed, specific search queries (expand beyond the original question with context and specifics)
3. DECIDE on search approach:
   - Single comprehensive search: When question can be fully addressed in one query
   - Sequential searches: When answers build on each other or need follow-up
   - Parallel searches: When covering different aspects of the same topic
4. SELECT appropriate model and reasoning_effort for each search
5. SYNTHESIZE results into a comprehensive, coherent answer

## Query Formulation Guidelines:
- Expand user questions with conversation context and specifics
- Include relevant constraints (timeframe, geographic scope, domain)
- Make queries specific enough to get focused, useful results
- Consider breaking complex questions into focused sub-queries

## Important Notes:
- The tool returns comprehensive answers, not citations or links to extract
- Be cost-conscious: use the simplest model that can handle the complexity
- You may need multiple searches for comprehensive coverage
- Always address the original user question completely

Now use the gpt_websearch tool strategically to answer the user's question.`

			// Return properly structured messages with system and user roles
			messages := []mcp.PromptMessage{
				{
					Role: "system",
					Content: mcp.TextContent{
						Type: "text",
						Text: systemPrompt,
					},
				},
				{
					Role: "user",
					Content: mcp.TextContent{
						Type: "text",
						Text: userQuestion,
					},
				},
			}

			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		})

	return mcpServer
}
