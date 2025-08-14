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

	// Add a helpful prompt template
	prompt := mcp.Prompt{
		Name:        "web_search",
		Description: "Template for web search queries",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "topic",
				Description: "The topic to search for",
				Required:    true,
			},
		},
	}
	mcpServer.AddPrompt(prompt, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		query, ok := request.Params.Arguments["topic"]
		if !ok || query == "" {
			return nil, fmt.Errorf("topic parameter is required")
		}

		messages := []mcp.PromptMessage{
			{
				Role: "user",
				Content: mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Search the web for: %s", query),
				},
			},
		}

		return &mcp.GetPromptResult{
			Messages: messages,
		}, nil
	})

	return mcpServer
}
