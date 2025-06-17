package logger

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
)

func TestLogger_BasicFunctionality(t *testing.T) {
	ctx := t.Context()
	log := FromContext(ctx)
	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message")
}

func TestLogger_ManualTestConfig(t *testing.T) {
	ctx := t.Context()
	log := FromContext(ctx)
	log.Info("test message")
	log.Error("test error")
}

func TestLogger_WithOutput(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Level:      InfoLevel,
		Output:     &buf,
		JSON:       false,
		AddSource:  false,
		TimeFormat: "15:04:05",
	}

	log := NewLogger(cfg)
	log.Info("Test message for verification")

	output := buf.String()
	if output == "" {
		t.Error("Expected log output but got none")
	}

	if !contains(output, "Test message for verification") {
		t.Error("Expected log message not found in output")
	}
}

func TestIsTestEnvironment(t *testing.T) {
	if !IsTestEnvironment() {
		t.Error("Expected IsTestEnvironment() to return true during tests")
	}
}

func TestLogger_AutoDetectTest(t *testing.T) {
	ctx := t.Context()
	log := FromContext(ctx)

	if log == nil {
		t.Error("Expected logger to be created")
	}

	log.Info("auto-detected test logger")
}

func TestLogLevels(t *testing.T) {
	ctx := t.Context()
	log := FromContext(ctx)
	log.Debug("debug level")
	log.Info("info level")
	log.Warn("warn level")
	log.Error("error level")
}

func TestConfigDefaults(t *testing.T) {
	defaultCfg := DefaultConfig()
	if defaultCfg.Level != InfoLevel {
		t.Errorf("Expected default level to be InfoLevel, got %v", defaultCfg.Level)
	}
	if defaultCfg.Output != os.Stdout {
		t.Error("Expected default output to be os.Stdout")
	}

	testCfg := TestConfig()
	if testCfg.Level != DisabledLevel {
		t.Errorf("Expected test level to be DisabledLevel, got %v", testCfg.Level)
	}
	if testCfg.Output != io.Discard {
		t.Error("Expected test output to be io.Discard")
	}
}

func TestLogger_WithMethod(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&Config{
		Level:      InfoLevel,
		Output:     &buf,
		JSON:       false,
		AddSource:  false,
		TimeFormat: "15:04:05",
	})

	// Test the With method
	log := logger.With("component", "test")
	log.Info("message with context")
	output := buf.String()
	if !contains(output, "component") || !contains(output, "test") {
		t.Error("Expected context fields in log output")
	}
}

func TestContextWithLogger(t *testing.T) {
	// Create a log
	log := NewLogger(TestConfig())

	// Store it in context
	ctx := context.Background()
	ctx = ContextWithLogger(ctx, log)

	// Retrieve it from context
	retrievedLogger := FromContext(ctx)

	if retrievedLogger == nil {
		t.Error("Expected to retrieve logger from context")
	}

	// Test that the logger works
	retrievedLogger.Info("message from context logger")
}

func TestLoggerFromContext_WithoutLogger(t *testing.T) {
	// Test context without logger returns default
	ctx := context.Background()
	log := FromContext(ctx)

	if log == nil {
		t.Error("Expected default logger when none in context")
	}

	// Test that the default logger works
	log.Info("message from default logger")
}

func TestLoggerFromContext_WithWrongType(t *testing.T) {
	// Test context with wrong type returns default
	ctx := context.Background()
	ctx = context.WithValue(ctx, LoggerCtxKey, "not a logger")
	log := FromContext(ctx)

	if log == nil {
		t.Error("Expected default logger when wrong type in context")
	}

	// Test that the default logger works
	log.Info("message from default logger after wrong type")
}

// Helper function since strings.Contains might not be available in all contexts
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsAtPosition(s, substr))))
}

func containsAtPosition(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
