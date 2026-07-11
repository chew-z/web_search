package main

import (
	"log/slog"
	"os"
	"runtime"
	"sync"
)

// Centralized structured logger using slog with dynamic level control.

var (
	logger   *slog.Logger
	levelVar slog.LevelVar
	once     sync.Once
)

// initLogger initializes the global JSON logger. Safe to call multiple times.
func initLogger(verbose bool) {
	once.Do(func() {
		if verbose {
			levelVar.Set(slog.LevelDebug)
		} else {
			levelVar.Set(slog.LevelInfo)
		}
		handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: &levelVar})
		logger = slog.New(handler)
	})
}

// setVerbose updates the log level at runtime to debug when true, otherwise info.
func setVerbose(verbose bool) {
	if verbose {
		levelVar.Set(slog.LevelDebug)
	} else {
		levelVar.Set(slog.LevelInfo)
	}
}

// logStartup emits a standardized startup banner with server identity, runtime,
// and effective configuration. Call once after all config is resolved.
func logStartup(cfg MCPConfig) {
	Info("server starting",
		"name", serverName,
		"version", serverVersion,
		"pid", os.Getpid(),
		"go", runtime.Version(),
		"gomaxprocs", runtime.GOMAXPROCS(0),
		"transport", cfg.Transport,
		"auth_enabled", cfg.AuthEnabled,
		"heartbeat", cfg.Heartbeat.String(),
		"verbose", cfg.Verbose,
		"base_url", cfg.BaseURL,
		"model_default", defaultModel,
		"effort_default", defaultEffort,
		"disable_localhost_protection", cfg.DisableLocalhostProtection,
	)
}

// Debug logs at debug level with optional structured key/value pairs.
func Debug(msg string, args ...any) {
	ensureLogger()
	logger.Debug(msg, args...)
}

// Info logs at info level with optional structured key/value pairs.
func Info(msg string, args ...any) {
	ensureLogger()
	logger.Info(msg, args...)
}

// Warn logs at warn level with optional structured key/value pairs.
func Warn(msg string, args ...any) {
	ensureLogger()
	logger.Warn(msg, args...)
}

// Error logs at error level with optional structured key/value pairs.
func Error(msg string, args ...any) {
	ensureLogger()
	logger.Error(msg, args...)
}

func ensureLogger() {
	if logger == nil {
		// Default initialize to info level if not set up explicitly.
		initLogger(false)
	}
}

// SLogger returns the underlying *slog.Logger for direct use with libraries
// that accept a standard slog.Logger (e.g. mcp-go v0.56+ WithStreamableHTTPLogger).
func SLogger() *slog.Logger {
	ensureLogger()
	return logger
}
