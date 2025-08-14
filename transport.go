package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

// RunStdioTransport runs the MCP server using STDIO transport
func RunStdioTransport(ctx context.Context, mcpServer *server.MCPServer, sigChan chan os.Signal) error {
	// The server package provides a simple ServeStdio function
	// It handles all the complexity internally
	log.Println("Starting STDIO transport...")

	// Note: ServeStdio blocks and handles signals internally
	// The ctx and sigChan parameters are kept for interface compatibility
	// but the library manages its own lifecycle
	return server.ServeStdio(mcpServer)
}

// RunHTTPTransport runs the MCP server using StreamableHTTP transport
func RunHTTPTransport(ctx context.Context, mcpServer *server.MCPServer, port string, sigChan chan os.Signal) error {
	// Create StreamableHTTP server - this is the proper HTTP transport for MCP
	httpServer := server.NewStreamableHTTPServer(mcpServer)

	// Start the server on the specified port
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting StreamableHTTP server on %s", addr)
	log.Printf("MCP endpoint: http://localhost:%s/", port)

	// Note: Start blocks and handles shutdown internally
	// The ctx and sigChan parameters are kept for interface compatibility
	// but the library manages its own lifecycle
	return httpServer.Start(addr)
}
