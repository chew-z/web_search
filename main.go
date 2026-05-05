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
		transport   = mcpFlags.String("t", "stdio", "Transport type (stdio or http)")
		port        = mcpFlags.String("port", "8080", "HTTP server port")
		host        = mcpFlags.String("host", "127.0.0.1", "HTTP server host (default: 127.0.0.1)")
		baseURL     = mcpFlags.String("base", defaultBaseURL, "API base URL")
		verbose     = mcpFlags.Bool("verbose", false, "Enable verbose logging")
		authEnabled = mcpFlags.Bool("auth-enabled", false, "Enable JWT authentication for HTTP transport (requires GEMINI_AUTH_SECRET_KEY env var)")
		heartbeat   = mcpFlags.Duration("heartbeat", 30*time.Second,
			"SSE heartbeat interval for HTTP transport (0 to disable); keeps long-running requests alive through proxies")
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

	// Read auth secret from environment (same variable as GeminiMCP for interoperability)
	authSecretKey := os.Getenv("GEMINI_AUTH_SECRET_KEY")
	if *authEnabled && authSecretKey == "" {
		Error("GEMINI_AUTH_SECRET_KEY must be set when --auth-enabled is used")
		os.Exit(1)
	}

	// Create server configuration using the config helper
	cfg := parseMCPConfig(MCPConfigParams{
		APIKey:        envCfg.APIKey,
		BaseURL:       *baseURL,
		Transport:     *transport,
		Port:          *port,
		Host:          *host,
		Verbose:       *verbose,
		AuthEnabled:   *authEnabled,
		AuthSecretKey: authSecretKey,
		Heartbeat:     *heartbeat,
	})

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
		if err := RunHTTPTransport(mcpServer, cfg); err != nil {
			Error("HTTP transport error", "error", err)
			os.Exit(1)
		}
	default:
		Error("Unknown transport (use 'stdio' or 'http')", "transport", cfg.Transport)
		os.Exit(1)
	}
}

// cliArgs holds the resolved command-line + environment configuration for runCLI.
type cliArgs struct {
	baseURL      string
	model        string
	effort       string
	verbosity    string
	question     string
	timeout      time.Duration
	useWebSearch bool
	showAll      bool
}

func parseCLIArgs(envCfg EnvConfig) cliArgs {
	defaultModelVal := defaultModel
	if envCfg.Model != "" {
		defaultModelVal = envCfg.Model
	}
	defaultEffortVal := defaultEffort
	if envCfg.Effort != "" {
		defaultEffortVal = envCfg.Effort
	}

	baseURL := flag.String("base", defaultBaseURL, "API endpoint")
	model := flag.String("model", defaultModelVal, "model (env MODEL)")
	effort := flag.String("effort", defaultEffortVal, "effort (env EFFORT)")
	verbosity := flag.String("verbosity", defaultVerbosity, "response verbosity (low, medium, high)")
	webSearch := flag.Bool("web-search", true, "use web search (default: true)")
	defaultTimeout := getTimeoutForEffort(defaultEffortVal)
	if envCfg.HasTimeout {
		defaultTimeout = envCfg.Timeout
	}
	timeout := flag.Duration("timeout", defaultTimeout, "HTTP timeout (env TIMEOUT)")
	showAll := flag.Bool("show-all", envCfg.HasShowAll && envCfg.ShowAll, "print raw JSON response (env SHOW_ALL)")

	var questionVal string
	flag.StringVar(&questionVal, "q", envCfg.Question, "question prompt (env QUESTION)")
	flag.StringVar(&questionVal, "question", envCfg.Question, "same as -q (env QUESTION)")
	flag.Parse()

	q := resolveQuestion(questionVal)
	*effort = validateEffort(*effort)
	*verbosity = validateVerbosity(*verbosity)
	if !envCfg.HasTimeout && !flagWasSet("timeout") {
		*timeout = getTimeoutForEffort(*effort)
	}

	return cliArgs{
		baseURL:      *baseURL,
		model:        *model,
		effort:       *effort,
		verbosity:    *verbosity,
		question:     q,
		timeout:      *timeout,
		useWebSearch: *webSearch,
		showAll:      *showAll,
	}
}

func resolveQuestion(questionVal string) string {
	if flagWasSet("q") || flagWasSet("question") {
		return questionVal
	}
	if flag.NArg() > 0 {
		return flag.Arg(0)
	}
	return questionVal
}

func flagWasSet(name string) bool {
	var set bool
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func runCLI() {
	envCfg, err := loadEnvConfig()
	if err != nil {
		fail(2, err.Error())
	}

	args := parseCLIArgs(envCfg)
	if args.question == "" {
		fail(2, "please provide a question to ask (use -q flag or positional argument)")
	}

	ctx := context.Background()
	apiResp, err := CallAPI(ctx, CallAPIParams{
		APIKey:       envCfg.APIKey,
		BaseURL:      args.baseURL,
		Query:        args.question,
		Model:        args.model,
		Effort:       args.effort,
		Verbosity:    args.verbosity,
		Timeout:      args.timeout,
		UseWebSearch: args.useWebSearch,
	})
	if err != nil {
		fail(2, err.Error())
	}

	if args.showAll {
		raw, _ := json.MarshalIndent(apiResp, "", "  ") //nolint:errcheck // Debug output, error ok to ignore
		fmt.Println(string(raw))
		return
	}

	answer := ExtractAnswer(apiResp)
	if answer == "" {
		fail(3, "no answer found in response")
	}
	fmt.Println(answer)
}
