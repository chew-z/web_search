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
	// Create MCP server
	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
	)

	// Create the tool definition
	tool := mcp.Tool{
		Name:        "gpt_websearch",
		Description: "Search the web using OpenAI's GPT model with web search capabilities",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query or question to ask",
				},
				"model": map[string]interface{}{
					"type":        "string",
					"description": "The GPT model to use (default: gpt-5)",
					"default":     defaultModel,
				},
				"reasoning_effort": map[string]interface{}{
					"type":        "string",
					"description": "Reasoning effort level: low (3min), medium (5min), or high (10min timeout)",
					"enum":        []string{"low", "medium", "high"},
					"default":     defaultEffort,
				},
			},
			Required: []string{"query"},
		},
	}

	// Register the tool with its handler
	mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		result, err := HandleWebSearch(ctx, apiKey, baseURL, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		// Convert result to JSON string for text content
		resultJSON, _ := json.Marshal(result) //nolint:errcheck // JSON marshal for simple types, error ok to ignore
		return mcp.NewToolResultText(string(resultJSON)), nil
	})

	// Add server info resource
	resource := mcp.Resource{
		URI:         "server-info",
		Name:        "server-info",
		Description: "Information about the MCP server",
		MIMEType:    "text/plain",
	}
	mcpServer.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		info := fmt.Sprintf("GPT Web Search MCP Server\nVersion: %s\nEndpoint: %s\n", serverVersion, baseURL)
		return []mcp.ResourceContents{
			&mcp.TextResourceContents{
				URI:      "server-info",
				MIMEType: "text/plain",
				Text:     info,
			},
		}, nil
	})

	// Add intelligent web search prompt
	prompt := mcp.Prompt{
		Name:        "web_search",
		Description: "Use the gpt_websearch tool to answer user questions based on web searching",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "user_question",
				Description: "The question, task, problem, or instructions from the user that requires web search",
				Required:    true,
			},
		},
	}
	mcpServer.AddPrompt(prompt, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		userQuestion, ok := request.Params.Arguments["user_question"]
		if !ok || userQuestion == "" {
			return nil, fmt.Errorf("user_question parameter is required")
		}

		messages := []mcp.PromptMessage{
			{
				Role: "system",
				Content: mcp.TextContent{
					Type: "text",
					Text: `You have access to the gpt_websearch tool that performs web searches using OpenAI's GPT models. ` +
						`This tool searches the web, gathers sources, reads them, and provides a single comprehensive answer.

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

Now use the gpt_websearch tool strategically to answer the user's question.`,
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
