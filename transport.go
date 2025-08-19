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

// RunHTTPTransport runs the MCP server using HTTP transport
func RunHTTPTransport(mcpServer *server.MCPServer, host, port string) error {
	httpServer := server.NewStreamableHTTPServer(mcpServer)

	addr := fmt.Sprintf("%s:%s", host, port)
	log.Printf("Starting HTTP server on %s", addr)
	log.Printf("MCP endpoint: http://%s:%s/", host, port)

	return httpServer.Start(addr)
}
