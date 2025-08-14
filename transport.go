package main

import (
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/server"
)

// RunStdioTransport runs the MCP server using STDIO transport
func RunStdioTransport(mcpServer *server.MCPServer) error {
	log.Println("Starting STDIO transport...")
	return server.ServeStdio(mcpServer)
}

// RunHTTPTransport runs the MCP server using HTTP/SSE transport
func RunHTTPTransport(mcpServer *server.MCPServer, port string) error {
	httpServer := server.NewStreamableHTTPServer(mcpServer)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting HTTP/SSE server on %s", addr)
	log.Printf("MCP endpoint: http://localhost:%s/", port)

	return httpServer.Start(addr)
}
