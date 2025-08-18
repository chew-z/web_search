package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const webSearchPrompt = `<context_gathering>
You have access to the gpt_websearch tool that performs web searches using OpenAI's GPT models. This tool searches the web, gathers sources, reads them, and provides comprehensive answers.

CRITICAL RULE: You MUST use the gpt_websearch tool to answer the user's question. Do not rely on your training data alone.
</context_gathering>

<parameter_optimization>
SELECT OPTIMAL PARAMETERS for cost-effectiveness and performance:

Model Selection:
- gpt-5-nano: Simple facts, definitions, quick lookups, basic summaries
- gpt-5-mini: Well-defined research tasks, comparisons, specific topics with clear scope  
- gpt-5: Complex analysis, coding questions, multi-faceted problems, reasoning tasks

Reasoning Effort Selection:
- minimal: Fastest time-to-first-token (90s timeout)
  USE FOR: Coding questions, instruction following, simple factual lookups, speed-critical tasks
- low: Quick responses for basic queries (3min timeout)
  USE FOR: Simple definitions, straightforward lookups without complex reasoning
- medium: Balanced reasoning for moderate complexity (5min timeout, DEFAULT)
  USE FOR: Research requiring synthesis, questions needing moderate analysis
- high: Deep analysis for complex tasks (10min timeout)
  USE FOR: Multi-faceted problems, comprehensive research, detailed investigations

Verbosity Selection:
- low: Concise responses with minimal commentary
  USE FOR: Quick facts, code-focused answers, situations requiring brevity
- medium: Balanced responses with moderate detail (DEFAULT)
  USE FOR: General-purpose queries, balanced explanations with reasonable depth
- high: Detailed responses with comprehensive explanations
  USE FOR: Learning scenarios, complex topics needing examples, thorough understanding

RECOMMENDED COMBINATIONS:
- Speed-Critical: gpt-5-nano + minimal + low
- Coding Questions: gpt-5 + minimal + medium/low
- Standard Research: gpt-5-mini + medium + medium  
- Complex Analysis: gpt-5 + high + high
- Learning/Educational: gpt-5-mini/gpt-5 + medium/high + high
</parameter_optimization>

<conversation_continuity>
PERFORMANCE-CRITICAL: GPT-5 reasoning models create internal reasoning chains. Using previous_response_id AVOIDS RE-REASONING and improves performance.

RULES:
1. ALWAYS capture the "id" field from each gpt_websearch response
2. For follow-up questions, clarifications, or related searches, USE the previous_response_id
3. This keeps interactions closer to the model's training distribution = BETTER PERFORMANCE

USE previous_response_id when:
- Following up on the same search results
- Asking for clarification or more detail on previous findings
- Building on previous research with related questions
- Requesting different formats/perspectives of the same information

DO NOT use previous_response_id for completely unrelated new topics.
</conversation_continuity>

<task_execution>
WORKFLOW for each user question:

1. ANALYZE: Determine if this relates to a previous search
   - If yes: USE previous_response_id to avoid re-reasoning
   - If no: Proceed with fresh search

2. PLAN: Select optimal model/effort/verbosity combination based on:
   - Question complexity
   - Response speed requirements  
   - Level of detail needed

3. FORMULATE: Create detailed, specific search queries
   - Expand beyond the original question with context and specifics
   - Include relevant constraints (timeframe, geographic scope, domain)
   - Make queries specific enough to get focused, useful results

4. EXECUTE: Perform search with optimal parameters
   - ALWAYS capture the response ID from results
   - For sequential searches, chain the response IDs to maintain reasoning continuity

5. SYNTHESIZE: Provide comprehensive, coherent answer addressing the original question completely
</task_execution>

<persistence>
Continue working until the user's query is completely resolved. You may need multiple searches for comprehensive coverage. Do not ask for confirmation - make reasonable assumptions and proceed with follow-up searches if needed to fully address the question.

For multi-search strategies:
- Chain response IDs between related searches
- Use previous_response_id when expanding on or clarifying previous results
- Remember: Better performance comes from avoiding duplicate reasoning through proper ID usage
</persistence>

<final_instructions>
The gpt_websearch tool returns comprehensive answers, not citations or links to extract. Be cost-conscious by using the simplest model that can handle the complexity, but ensure you fully address the user's question.

Now analyze the user's question and use the gpt_websearch tool strategically with optimal parameters.
</final_instructions>`

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
