package main

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Check if this is MCP server mode
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		runMCPMode()
		return
	}

	// Original CLI mode. main is the single exit point: runCLI returns errors
	// (so it is unit-testable) and we map them back to the original exit codes.
	if err := runCLI(); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			fail(ee.code, ee.msg)
		}
		fail(1, err.Error())
	}
}

func runMCPMode() {
	// Create a new flag set for MCP subcommand
	mcpFlags := flag.NewFlagSet("mcp", flag.ExitOnError)

	var (
		transport    = mcpFlags.String("t", "stdio", "Transport type (stdio or http)")
		port         = mcpFlags.String("port", "8080", "HTTP server port")
		host         = mcpFlags.String("host", "127.0.0.1", "HTTP server host (default: 127.0.0.1)")
		baseURL      = mcpFlags.String("base", "", "API base URL (default: provider-specific)")
		providerName = mcpFlags.String("provider", "", "API provider: openai or deepseek (default: env PROVIDER, else openai)")
		verbose      = mcpFlags.Bool("verbose", false, "Enable verbose logging")
		authEnabled  = mcpFlags.Bool("auth-enabled", false, "Enable JWT authentication for HTTP transport (requires GEMINI_AUTH_SECRET_KEY env var)")
		heartbeat    = mcpFlags.Duration("heartbeat", 30*time.Second,
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

	// Load environment config (validates the provider and reads its API key)
	envCfg, err := loadEnvConfig(*providerName)
	if err != nil {
		Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Read auth secret from environment (same variable as GeminiMCP for interoperability)
	authSecretKey, err := requireAuthSecret(*authEnabled)
	if err != nil {
		Error(err.Error())
		os.Exit(1)
	}

	// DNS rebinding protection: disabled by default since production deployments
	// run behind nginx. Set ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION=false to enable.
	disableLocalhostProtection := parseEnvBool("ANSWER_HTTP_DISABLE_LOCALHOST_PROTECTION", true)

	// Create server configuration using the config helper
	cfg := parseMCPConfig(MCPConfigParams{
		Provider:                   envCfg.Provider,
		APIKey:                     envCfg.APIKey,
		BaseURL:                    *baseURL,
		Transport:                  *transport,
		Port:                       *port,
		Host:                       *host,
		Verbose:                    *verbose,
		AuthEnabled:                *authEnabled,
		AuthSecretKey:              authSecretKey,
		Heartbeat:                  *heartbeat,
		DisableLocalhostProtection: disableLocalhostProtection,
	})

	// Create and run MCP server
	mcpServer := NewMCPServer(cfg)

	logStartup(cfg)

	runTransport(mcpServer, cfg)
}

// runTransport starts the configured transport and exits on failure.
func runTransport(mcpServer *server.MCPServer, cfg MCPConfig) {
	switch cfg.Transport {
	case "stdio":
		if err := RunStdioTransport(mcpServer); err != nil {
			Error("transport failed", "protocol", "stdio", "error", err)
			os.Exit(1)
		}
	case "http":
		if err := RunHTTPTransport(mcpServer, cfg); err != nil {
			Error("transport failed", "protocol", "http", "error", err)
			os.Exit(1)
		}
	default:
		Error("unknown transport", "transport", cfg.Transport)
		os.Exit(1)
	}
}

// requireAuthSecret reads the JWT secret (same variable as GeminiMCP for
// interoperability) and enforces its presence when auth is enabled.
func requireAuthSecret(authEnabled bool) (string, error) {
	key := os.Getenv("GEMINI_AUTH_SECRET_KEY")
	if authEnabled && key == "" {
		return "", errors.New("GEMINI_AUTH_SECRET_KEY must be set when --auth-enabled is used")
	}
	return key, nil
}

// resolveEndpointAndModel applies provider defaults when the user did not
// supply an explicit -base / -model.
func resolveEndpointAndModel(args cliArgs, envCfg EnvConfig, prov provider) (baseURL, model string) {
	baseURL = args.baseURL
	if baseURL == "" {
		baseURL = prov.DefaultBaseURL
	}
	model = args.model
	if !flagWasSet("model") && envCfg.Model == "" {
		model = prov.DefaultModel
	}
	return baseURL, model
}

// cliArgs holds the resolved command-line + environment configuration for runCLI.
type cliArgs struct {
	provider       string
	baseURL        string
	model          string
	effort         string
	verbosity      string
	question       string
	promptCacheKey string
	timeout        time.Duration
	useWebSearch   bool
	showAll        bool
}

func parseCLIArgs(envCfg EnvConfig) cliArgs {
	defaultModelVal := providerOrDefault(envCfg.Provider).DefaultModel
	if envCfg.Model != "" {
		defaultModelVal = envCfg.Model
	}
	defaultEffortVal := defaultEffort
	if envCfg.Effort != "" {
		defaultEffortVal = envCfg.Effort
	}

	provider := flag.String("provider", envCfg.Provider, "API provider: openai or deepseek (env PROVIDER)")
	baseURL := flag.String("base", "", "API endpoint (default: provider-specific)")
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
	cacheKey := flag.String("cache-key", os.Getenv("PROMPT_CACHE_KEY"), "OpenAI prompt_cache_key (env PROMPT_CACHE_KEY); leave empty for server default")

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
		provider:       *provider,
		baseURL:        *baseURL,
		model:          *model,
		effort:         *effort,
		verbosity:      *verbosity,
		question:       q,
		promptCacheKey: *cacheKey,
		timeout:        *timeout,
		useWebSearch:   *webSearch,
		showAll:        *showAll,
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

func runCLI() error {
	// Advisory load for flag defaults only: its error is ignored because the
	// effective provider is unknown until flags are parsed. The authoritative
	// load below uses the post-parse provider, so `-provider deepseek` works
	// even when only DEEPSEEK_API_KEY is configured.
	envDefaults, _ := loadEnvConfig("") //nolint:errcheck // see comment above

	args := parseCLIArgs(envDefaults)

	envCfg, err := loadEnvConfig(args.provider)
	if err != nil {
		return &exitError{2, err.Error()}
	}
	prov := providerOrDefault(envCfg.Provider)

	if args.question == "" {
		return &exitError{2, "please provide a question to ask (use -q flag or positional argument)"}
	}

	baseURL, model := resolveEndpointAndModel(args, envCfg, prov)

	ctx := context.Background()

	cacheKey := ""
	if prov.SupportsContinuity {
		cacheKey = resolvePromptCacheKey(ctx, args.promptCacheKey)
	}

	apiResp, err := CallAPI(ctx, CallAPIParams{
		APIKey:         envCfg.APIKey,
		BaseURL:        baseURL,
		Query:          args.question,
		Model:          model,
		Effort:         args.effort,
		Verbosity:      args.verbosity,
		PromptCacheKey: cacheKey,
		Timeout:        args.timeout,
		UseWebSearch:   args.useWebSearch,
		WebSearchTool:  prov.WebSearchTool,
	})
	if err != nil {
		return &exitError{2, err.Error()}
	}

	if args.showAll {
		raw, _ := json.Marshal(apiResp, jsontext.WithIndent("  ")) //nolint:errcheck // Debug output, error ok to ignore
		fmt.Println(string(raw))
		return nil
	}

	answer := ExtractAnswer(apiResp)
	if answer == "" {
		return &exitError{3, "no answer found in response"}
	}
	fmt.Println(answer)
	return nil
}
