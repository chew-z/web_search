package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

// RunStdioTransport runs the MCP server using STDIO transport
func RunStdioTransport(ctx context.Context, mcpServer *server.MCPServer, sigChan chan os.Signal) error {
	// Create stdio server
	stdioServer := server.NewStdioServer(mcpServer)

	// Create error channel
	errChan := make(chan error, 1)

	// Run server in goroutine
	go func() {
		if err := stdioServer.Listen(ctx, os.Stdin, os.Stdout); err != nil {
			errChan <- fmt.Errorf("serve error: %w", err)
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		log.Println("Shutting down stdio server...")
		return nil
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RunHTTPTransport runs the MCP server using HTTP/SSE transport
func RunHTTPTransport(ctx context.Context, mcpServer *server.MCPServer, port string, sigChan chan os.Signal) error {
	// Create SSE server for HTTP
	sseServer := server.NewSSEServer(mcpServer)

	// Setup HTTP routes
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck // HTTP response write error handled by HTTP layer
		fmt.Fprintf(w, `{"status":"healthy","server":"%s","version":"%s"}`, serverName, serverVersion)
	})

	// SSE endpoint for MCP communication
	mux.HandleFunc("/sse", sseServer.SSEHandler().ServeHTTP)

	// Message endpoint for MCP communication
	mux.HandleFunc("/message", sseServer.MessageHandler().ServeHTTP)

	// API documentation endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		//nolint:errcheck // HTTP response write error handled by HTTP layer
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>GPT Web Search MCP Server</title>
    <style>
        body { font-family: -apple-system, system-ui, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        .endpoint { background: #f5f5f5; padding: 10px; margin: 10px 0; border-radius: 5px; }
        code { background: #eee; padding: 2px 5px; border-radius: 3px; }
        .tool { background: #e8f4f8; padding: 15px; margin: 20px 0; border-radius: 5px; }
        .note { background: #fff3cd; padding: 10px; margin: 10px 0; border-radius: 5px; }
    </style>
</head>
<body>
    <h1>🔍 GPT Web Search MCP Server</h1>
    <p>Version: %s</p>
    <h2>Available Endpoints</h2>
    <div class="endpoint">
        <strong>GET /health</strong> - Health check endpoint
    </div>
    <div class="endpoint">
        <strong>GET /sse</strong> - Server-Sent Events endpoint for MCP protocol
    </div>
    <h2>Available Tools</h2>
    <div class="tool">
        <h3>gpt_websearch</h3>
        <p>Search the web using OpenAI's GPT model with web search capabilities</p>
        <h4>Parameters:</h4>
        <ul>
            <li><code>query</code> (required) - The search query or question</li>
            <li><code>model</code> (optional) - GPT model to use (default: gpt-5-mini)</li>
            <li><code>reasoning_effort</code> (optional) - Effort level with automatic timeout:
                <ul>
                    <li><code>low</code> - 3 minute timeout</li>
                    <li><code>medium</code> - 5 minute timeout</li>
                    <li><code>high</code> - 10 minute timeout</li>
                </ul>
            </li>
        </ul>
    </div>
    <div class="note">
        <strong>Note:</strong> API key must be set via <code>OPENAI_API_KEY</code> environment variable.
    </div>
</body>
</html>`, serverVersion)
	})

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting HTTP server on port %s", port)
		log.Printf("Health check: http://localhost:%s/health", port)
		log.Printf("SSE endpoint: http://localhost:%s/sse", port)
		log.Printf("Documentation: http://localhost:%s/", port)

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutting down HTTP server...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	return nil
}
