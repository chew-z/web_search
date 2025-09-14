package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	// Check if this is MCP server mode
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		runMCPMode()
		return
	}

	// Original CLI mode
	runCLI()
}

func runMCPMode() {
	// Create a new flag set for MCP subcommand
	mcpFlags := flag.NewFlagSet("mcp", flag.ExitOnError)

	var (
		transport = mcpFlags.String("t", "stdio", "Transport type (stdio or http)")
		port      = mcpFlags.String("port", "8080", "HTTP server port")
		host      = mcpFlags.String("host", "127.0.0.1", "HTTP server host (default: 127.0.0.1)")
		baseURL   = mcpFlags.String("base", defaultBaseURL, "API base URL")
		verbose   = mcpFlags.Bool("verbose", false, "Enable verbose logging")
	)

	// Also support long form for transport
	transportLong := mcpFlags.String("transport", "", "Transport type (overrides -t)")

	// Initialize logger early with default level (info). Adjust after parsing.
	initLogger(false)

	// Parse MCP-specific flags (skip "answer mcp" args)
	if err := mcpFlags.Parse(os.Args[2:]); err != nil {
		Error("Error parsing flags", "error", err)
		os.Exit(1)
	}

	// Use long form if provided
	if *transportLong != "" {
		*transport = *transportLong
	}

	// Honor -verbose for logger level
	setVerbose(*verbose)

	// Load environment config
	envCfg, err := loadEnvConfig()
	if err != nil {
		Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Create server configuration using the config helper
	cfg := parseMCPConfig(envCfg.APIKey, *baseURL, *transport, *port, *host, *verbose)

	// Create and run MCP server
	mcpServer := NewMCPServer(cfg)

	// Run with appropriate transport
	switch cfg.Transport {
	case "stdio":
		if err := RunStdioTransport(mcpServer); err != nil {
			Error("STDIO transport error", "error", err)
			os.Exit(1)
		}
	case "http":
		if err := RunHTTPTransport(mcpServer, cfg.Host, cfg.Port); err != nil {
			Error("HTTP transport error", "error", err)
			os.Exit(1)
		}
	default:
		Error("Unknown transport (use 'stdio' or 'http')", "transport", cfg.Transport)
		os.Exit(1)
	}
}

func runCLI() {
	// Load environment-backed configuration
	envCfg, err := loadEnvConfig()
	if err != nil {
		fail(2, err.Error())
	}

	// Parse CLI flags
	var (
		baseURL = flag.String("base", defaultBaseURL, "API endpoint")
		model   = flag.String("model", func() string {
			if envCfg.Model != "" {
				return envCfg.Model
			}
			return defaultModel
		}(), "model (env MODEL)")
		effort = flag.String("effort", func() string {
			if envCfg.Effort != "" {
				return envCfg.Effort
			}
			return defaultEffort
		}(), "effort (env EFFORT)")
		verbosity   = flag.String("verbosity", defaultVerbosity, "response verbosity (low, medium, high)")
		webSearch   = flag.Bool("web-search", true, "use web search (default: true)")
		questionVal string
		timeout     = flag.Duration("timeout", func() time.Duration {
			if envCfg.HasTimeout {
				return envCfg.Timeout
			}
			return getTimeoutForEffort(*effort)
		}(), "HTTP timeout (env TIMEOUT)")
		showAll = flag.Bool("show-all", func() bool {
			if envCfg.HasShowAll {
				return envCfg.ShowAll
			}
			return false
		}(), "print raw JSON response (env SHOW_ALL)")
	)
	flag.StringVar(&questionVal, "q", envCfg.Question, "question prompt (env QUESTION)")
	flag.StringVar(&questionVal, "question", envCfg.Question, "same as -q (env QUESTION)")
	flag.Parse()

	// Determine the final question value
	q := questionVal
	var questionFlagSet bool
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "q" || f.Name == "question" {
			questionFlagSet = true
		}
	})
	if !questionFlagSet {
		if flag.NArg() > 0 {
			q = flag.Arg(0)
		}
	}
	if q == "" {
		fail(2, "please provide a question to ask (use -q flag or positional argument)")
	}

	// Validate effort and verbosity parameters
	*effort = validateEffort(*effort)
	*verbosity = validateVerbosity(*verbosity)

	// Use web search flag directly
	useWebSearch := *webSearch

	// Only override timeout if neither env nor CLI provided it
	var timeoutFlagSet bool
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "timeout" {
			timeoutFlagSet = true
		}
	})
	if !envCfg.HasTimeout && !timeoutFlagSet {
		*timeout = getTimeoutForEffort(*effort)
	}

	// Make API call with determined web search setting
	ctx := context.Background()
	apiResp, err := CallAPI(ctx, envCfg.APIKey, *baseURL, q, *model, *effort, *verbosity, "", *timeout, useWebSearch)
	if err != nil {
		fail(2, err.Error())
	}

	if *showAll {
		// Print the full raw JSON when requested
		raw, _ := json.MarshalIndent(apiResp, "", "  ") //nolint:errcheck // Debug output, error ok to ignore
		fmt.Println(string(raw))
		return
	}

	// Extract and print the answer
	answer := ExtractAnswer(apiResp)
	if answer == "" {
		fail(3, "no answer found in response")
	}
	fmt.Println(answer)
}
