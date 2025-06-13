package logger

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestLogger_BasicFunctionality(t *testing.T) {
	// Method 1: Use SetupTestLogger helper (recommended)
	defer SetupTestLogger(t)()

	// These log calls should not produce any output
	Info("This should not appear in test output")
	Debug("Debug message should be suppressed")
	Error("Even errors should be suppressed")
}

func TestLogger_ManualTestConfig(t *testing.T) {
	// Method 2: Manually initialize with test config
	err := Init(TestConfig())
	if err != nil {
		t.Fatalf("Failed to initialize test logger: %v", err)
	}

	// These should not produce output
	Info("Test message 1")
	Warn("Test warning")
}

func TestLogger_DisableLogging(t *testing.T) {
	// Method 3: Disable logging after initialization
	err := Init(DefaultConfig())
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	DisableLogging()

	// These should not produce output
	Info("This should be disabled")
	Error("This error should not show")
}

func TestLogger_WithOutput(t *testing.T) {
	// Method 4: Capture output for testing log content
	var buf bytes.Buffer

	cfg := &Config{
		Level:      InfoLevel,
		Output:     &buf,
		JSON:       false,
		AddSource:  false,
		TimeFormat: "15:04:05",
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	Info("Test message for verification")

	output := buf.String()
	if output == "" {
		t.Error("Expected log output but got none")
	}

	if !contains(output, "Test message for verification") {
		t.Error("Expected log message not found in output")
	}
}

func TestIsTestEnvironment(t *testing.T) {
	// This should return true when running under go test
	if !IsTestEnvironment() {
		t.Error("Expected IsTestEnvironment() to return true during tests")
	}
}

func TestLogger_AutoDetectTest(t *testing.T) {
	// Test that NewLogger automatically uses test config in test environment
	logger := NewLogger(nil) // nil config should auto-detect test env

	// Verify it's using the right configuration by checking if logs are suppressed
	// We can't easily test this without capturing output, but we can at least
	// verify the logger was created successfully
	if logger == nil {
		t.Error("Expected logger to be created")
	}
}

func TestLogLevels(t *testing.T) {
	defer SetupTestLogger(t)()

	// Test all log levels work without panicking
	Debug("Debug level test")
	Info("Info level test")
	Warn("Warn level test")
	Error("Error level test")
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
