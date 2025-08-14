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
		RunMCPServer()
		return
	}

	// Original CLI mode
	runCLI()
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

	// If timeout wasn't explicitly set, use effort-based timeout
	if !envCfg.HasTimeout {
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "timeout" {
				return // User set it explicitly
			}
		})
		*timeout = getTimeoutForEffort(*effort)
	}

	// Make API call
	ctx := context.Background()
	apiResp, err := CallAPI(ctx, envCfg.APIKey, *baseURL, q, *model, *effort, *timeout)
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
