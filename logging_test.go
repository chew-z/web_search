package main

import (
	"log/slog"
	"sync"
	"testing"
	"time"
)

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	fn()
}

func resetLoggerState() {
	logger = nil
	levelVar = slog.LevelVar{}
	once = sync.Once{}
}

func TestInitLogger(t *testing.T) {
	resetLoggerState()

	assertNoPanic(t, func() {
		initLogger(true)
	})
	if logger == nil {
		t.Fatal("logger is nil after initLogger(true)")
	}

	firstLogger := logger
	assertNoPanic(t, func() {
		initLogger(false)
		initLogger(false)
	})
	if logger != firstLogger {
		t.Fatal("logger was reinitialized on repeated initLogger calls")
	}
}

func TestSetVerbose(t *testing.T) {
	resetLoggerState()

	assertNoPanic(t, func() {
		setVerbose(true)
	})
	if levelVar.Level() != slog.LevelDebug {
		t.Fatalf("levelVar = %v, want %v", levelVar.Level(), slog.LevelDebug)
	}

	assertNoPanic(t, func() {
		setVerbose(false)
	})
	if levelVar.Level() != slog.LevelInfo {
		t.Fatalf("levelVar = %v, want %v", levelVar.Level(), slog.LevelInfo)
	}
}

func TestDebug(t *testing.T) {
	resetLoggerState()

	assertNoPanic(t, func() {
		Debug("debug test message")
		Debug("debug test message", "key", "value")
	})
}

func TestInfo(t *testing.T) {
	resetLoggerState()

	assertNoPanic(t, func() {
		Info("info test message")
		Info("info test message", "key", "value")
	})
}

func TestWarn(t *testing.T) {
	resetLoggerState()

	assertNoPanic(t, func() {
		Warn("warn test message")
		Warn("warn test message", "key", "value")
	})
}

func TestError(t *testing.T) {
	resetLoggerState()

	assertNoPanic(t, func() {
		Error("error test message")
		Error("error test message", "key", "value")
	})
}

func TestSLogger(t *testing.T) {
	resetLoggerState()

	l := SLogger()
	if l == nil {
		t.Fatal("SLogger returned nil")
	}

	assertNoPanic(t, func() {
		l.Info("slogger info message")
		l.Error("slogger error message")
	})
}

func TestLogStartup(t *testing.T) {
	resetLoggerState()

	cfg := MCPConfig{
		BaseURL:     "https://api.openai.com/v1",
		Transport:   "stdio",
		Verbose:     true,
		AuthEnabled: true,
		Heartbeat:   5 * time.Second,
	}

	assertNoPanic(t, func() {
		logStartup(cfg)
	})
}
