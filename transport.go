package main

import (
	"fmt"

	"github.com/mark3labs/mcp-go/server"
)

// RunStdioTransport runs the MCP server using STDIO transport
func RunStdioTransport(mcpServer *server.MCPServer) error {
	Info("Starting STDIO transport")
	return server.ServeStdio(mcpServer)
}

// RunHTTPTransport runs the MCP server using HTTP transport
func RunHTTPTransport(mcpServer *server.MCPServer, host, port string) error {
	httpServer := server.NewStreamableHTTPServer(mcpServer)

	addr := fmt.Sprintf("%s:%s", host, port)
	Info("Starting HTTP server", "addr", addr)
	Info("MCP endpoint", "url", fmt.Sprintf("http://%s:%s/", host, port))

	return httpServer.Start(addr)
}
