package logger

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromContext(t *testing.T) {
	t.Run("Should return logger from context when present", func(t *testing.T) {
		expectedLogger := NewLogger(TestConfig())
		ctx := ContextWithLogger(t.Context(), expectedLogger)

		actualLogger := FromContext(ctx)

		require.NotNil(t, actualLogger)
		assert.Equal(t, expectedLogger, actualLogger)
	})

	t.Run("Should return default logger when no logger in context", func(t *testing.T) {
		ctx := t.Context()

		logger := FromContext(ctx)

		require.NotNil(t, logger)
		logger.Info("test message from default logger")
	})

	t.Run("Should return default logger when wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), LoggerCtxKey, "not a logger")

		logger := FromContext(ctx)

		require.NotNil(t, logger)
		logger.Info("test message from fallback logger")
	})

	t.Run("Should return default logger when nil logger in context", func(t *testing.T) {
		ctx := context.WithValue(t.Context(), LoggerCtxKey, (Logger)(nil))

		logger := FromContext(ctx)

		require.NotNil(t, logger)
		logger.Info("test message from fallback logger")
	})
}

func TestLogLevel_ToCharmlogLevel(t *testing.T) {
	t.Run("Should convert all log levels to charm log levels correctly", func(t *testing.T) {
		testCases := []struct {
			level    LogLevel
			expected int // Using int for comparison since charmlog.Level is int
		}{
			{DebugLevel, -4},         // charmlog.DebugLevel
			{InfoLevel, 0},           // charmlog.InfoLevel
			{WarnLevel, 4},           // charmlog.WarnLevel
			{ErrorLevel, 8},          // charmlog.ErrorLevel
			{DisabledLevel, 1000},    // High level to disable
			{LogLevel("unknown"), 0}, // Default to InfoLevel
		}

		for _, tc := range testCases {
			actual := tc.level.ToCharmlogLevel()
			assert.Equal(
				t,
				tc.expected,
				int(actual),
				"LogLevel %s should convert to charm level %d",
				tc.level,
				tc.expected,
			)
		}
	})
}

func TestNewLogger(t *testing.T) {
	t.Run("Should create logger with provided config", func(t *testing.T) {
		var buf bytes.Buffer
		config := &Config{
			Level:      InfoLevel,
			Output:     &buf,
			JSON:       false,
			AddSource:  false,
			TimeFormat: "15:04:05",
		}

		logger := NewLogger(config)
		logger.Info("test message")

		require.NotNil(t, logger)
		output := buf.String()
		assert.Contains(t, output, "test message")
	})

	t.Run("Should use default config when nil config provided in non-test environment", func(t *testing.T) {
		// This test assumes we're not in a test environment detection scenario
		logger := NewLogger(nil)

		require.NotNil(t, logger)
		// Verify it uses the default behavior (this is functional testing)
		logger.Info("test default config")
	})

	t.Run("Should create logger with JSON formatting when enabled", func(t *testing.T) {
		var buf bytes.Buffer
		config := &Config{
			Level:      InfoLevel,
			Output:     &buf,
			JSON:       true,
			AddSource:  false,
			TimeFormat: "15:04:05",
		}

		logger := NewLogger(config)
		logger.Info("test message")

		output := buf.String()
		// JSON output should contain structured fields
		assert.Contains(t, output, "test message")
		// Basic JSON structure validation
		assert.True(t, strings.Contains(output, "{") && strings.Contains(output, "}"))
	})
}

func TestLogger_With(t *testing.T) {
	t.Run("Should create logger with additional context fields", func(t *testing.T) {
		var buf bytes.Buffer
		baseLogger := NewLogger(&Config{
			Level:      InfoLevel,
			Output:     &buf,
			JSON:       false,
			AddSource:  false,
			TimeFormat: "15:04:05",
		})

		contextLogger := baseLogger.With("component", "test", "operation", "validate")
		contextLogger.Info("operation completed")

		output := buf.String()
		assert.Contains(t, output, "component")
		assert.Contains(t, output, "test")
		assert.Contains(t, output, "operation")
		assert.Contains(t, output, "validate")
		assert.Contains(t, output, "operation completed")
	})
}

func TestConfigDefaults(t *testing.T) {
	t.Run("Should provide correct default configuration", func(t *testing.T) {
		config := DefaultConfig()

		assert.Equal(t, InfoLevel, config.Level)
		assert.Equal(t, os.Stdout, config.Output)
		assert.False(t, config.JSON)
		assert.False(t, config.AddSource)
		assert.Equal(t, "15:04:05", config.TimeFormat)
	})

	t.Run("Should provide correct test configuration", func(t *testing.T) {
		config := TestConfig()

		assert.Equal(t, DisabledLevel, config.Level)
		assert.Equal(t, io.Discard, config.Output)
		assert.False(t, config.JSON)
		assert.False(t, config.AddSource)
		assert.Equal(t, "15:04:05", config.TimeFormat)
	})
}

func TestIsTestEnvironment(t *testing.T) {
	t.Run("Should detect test environment correctly", func(t *testing.T) {
		// This should return true since we're running under go test
		isTest := IsTestEnvironment()

		assert.True(t, isTest, "Should detect we're running in a test environment")
	})
}

func TestLoggerLevels(t *testing.T) {
	t.Run("Should respect log level filtering", func(t *testing.T) {
		var buf bytes.Buffer

		// Create logger with WARN level - should only log WARN and ERROR
		config := &Config{
			Level:      WarnLevel,
			Output:     &buf,
			JSON:       false,
			AddSource:  false,
			TimeFormat: "15:04:05",
		}

		logger := NewLogger(config)

		// These should not appear in output
		logger.Debug("debug message")
		logger.Info("info message")

		// These should appear in output
		logger.Warn("warn message")
		logger.Error("error message")

		output := buf.String()

		// Debug and Info should be filtered out
		assert.NotContains(t, output, "debug message")
		assert.NotContains(t, output, "info message")

		// Warn and Error should be present
		assert.Contains(t, output, "warn message")
		assert.Contains(t, output, "error message")
	})

	t.Run("Should disable all logging when DisabledLevel is used", func(t *testing.T) {
		var buf bytes.Buffer
		config := &Config{
			Level:      DisabledLevel,
			Output:     &buf,
			JSON:       false,
			AddSource:  false,
			TimeFormat: "15:04:05",
		}

		logger := NewLogger(config)
		logger.Debug("debug message")
		logger.Info("info message")
		logger.Warn("warn message")
		logger.Error("error message")

		output := buf.String()

		// All messages should be filtered out when disabled
		assert.Empty(t, output, "No output should be generated when logging is disabled")
	})
}
