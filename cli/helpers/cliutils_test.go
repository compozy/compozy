package helpers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/compozy/compozy/cli/tui/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCliError(t *testing.T) {
	t.Run("Should create error with code and message", func(t *testing.T) {
		err := NewCliError("TEST_ERROR", "Test message")
		assert.Equal(t, "TEST_ERROR", err.Code)
		assert.Equal(t, "Test message", err.Message)
		assert.Empty(t, err.Details)
		assert.NotNil(t, err.Context)
	})

	t.Run("Should create error with details", func(t *testing.T) {
		err := NewCliError("TEST_ERROR", "Test message", "Additional details")
		assert.Equal(t, "TEST_ERROR", err.Code)
		assert.Equal(t, "Test message", err.Message)
		assert.Equal(t, "Additional details", err.Details)
	})

	t.Run("Should implement error interface", func(t *testing.T) {
		err := NewCliError("TEST_ERROR", "Test message")
		assert.Equal(t, "TEST_ERROR: Test message", err.Error())

		errWithDetails := NewCliError("TEST_ERROR", "Test message", "Details")
		assert.Equal(t, "TEST_ERROR: Test message (Details)", errWithDetails.Error())
	})
}

func TestWithContext(t *testing.T) {
	t.Run("Should add context to error", func(t *testing.T) {
		err := NewCliError("TEST_ERROR", "Test message")
		err.WithContext("user_id", "123")
		err.WithContext("action", "test")

		assert.Equal(t, "123", err.Context["user_id"])
		assert.Equal(t, "test", err.Context["action"])
	})
}

func TestIsTimeoutError(t *testing.T) {
	t.Run("Should detect timeout errors", func(t *testing.T) {
		// Test context deadline exceeded
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		time.Sleep(2 * time.Millisecond)
		assert.True(t, IsTimeoutError(ctx.Err()))

		// Test custom timeout errors
		assert.True(t, IsTimeoutError(NewCliError("TIMEOUT", "operation timeout")))
		assert.True(t, IsTimeoutError(NewCliError("TIMEOUT", "request timed out")))

		// Test non-timeout errors
		assert.False(t, IsTimeoutError(nil))
		assert.False(t, IsTimeoutError(NewCliError("OTHER", "not a time related error")))
	})
}

func TestIsNetworkError(t *testing.T) {
	t.Run("Should detect network errors", func(t *testing.T) {
		networkErrors := []string{
			"connection refused",
			"connection timeout",
			"no route to host",
			"dns resolution failed",
		}

		for _, errMsg := range networkErrors {
			err := NewCliError("NETWORK_ERROR", errMsg)
			assert.True(t, IsNetworkError(err), "Should detect network error: %s", errMsg)
		}

		assert.False(t, IsNetworkError(nil))
		assert.False(t, IsNetworkError(NewCliError("OTHER", "Not a network error")))
	})
}

func TestIsAuthError(t *testing.T) {
	t.Run("Should detect authentication errors", func(t *testing.T) {
		authErrors := []string{
			"unauthorized",
			"authentication failed",
			"invalid token",
			"permission denied",
			"forbidden",
		}

		for _, errMsg := range authErrors {
			err := NewCliError("AUTH_ERROR", errMsg)
			assert.True(t, IsAuthError(err), "Should detect auth error: %s", errMsg)
		}

		assert.False(t, IsAuthError(nil))
		assert.False(t, IsAuthError(NewCliError("OTHER", "Not an auth error")))
	})
}

func TestFormatError(t *testing.T) {
	t.Run("Should format error for JSON mode", func(t *testing.T) {
		err := NewCliError("TEST_ERROR", "Test message", "Test details")
		formatted := FormatError(err, models.ModeJSON)
		assert.Contains(t, formatted, "TEST_ERROR")
		assert.Contains(t, formatted, "Test message")
		assert.Contains(t, formatted, "Test details")
	})

	t.Run("Should format error for TUI mode", func(t *testing.T) {
		err := NewCliError("TEST_ERROR", "Test message")
		formatted := FormatError(err, models.ModeTUI)
		assert.Contains(t, formatted, "❌")
		assert.Contains(t, formatted, "Test message")
	})

	t.Run("Should handle nil error", func(t *testing.T) {
		formatted := FormatError(nil, models.ModeJSON)
		assert.Empty(t, formatted)
	})
}

func TestValidateID(t *testing.T) {
	t.Run("Should validate valid UUIDs", func(t *testing.T) {
		validUUIDs := []string{
			"123e4567-e89b-12d3-a456-426614174000",
			"550e8400-e29b-41d4-a716-446655440000",
			"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		}

		for _, uuid := range validUUIDs {
			err := ValidateID(uuid)
			assert.NoError(t, err, "Should validate UUID: %s", uuid)
		}
	})

	t.Run("Should reject invalid UUIDs", func(t *testing.T) {
		invalidUUIDs := []string{
			"",
			"not-a-uuid",
			"123e4567-e89b-12d3-a456-42661417400",   // too short
			"123e4567-e89b-12d3-a456-4266141740000", // too long
			"123e4567-e89b-12d3-a456-42661417400g",  // invalid character
		}

		for _, uuid := range invalidUUIDs {
			err := ValidateID(uuid)
			assert.Error(t, err, "Should reject invalid UUID: %s", uuid)
		}
	})
}

func TestValidateRequired(t *testing.T) {
	t.Run("Should validate required fields", func(t *testing.T) {
		assert.NoError(t, ValidateRequired("valid", "field"))
		assert.NoError(t, ValidateRequired("  valid  ", "field"))

		assert.Error(t, ValidateRequired("", "field"))
		assert.Error(t, ValidateRequired("   ", "field"))
	})
}

func TestValidateEnum(t *testing.T) {
	t.Run("Should validate enum values", func(t *testing.T) {
		allowed := []string{"option1", "option2", "option3"}

		assert.NoError(t, ValidateEnum("option1", allowed, "field"))
		assert.NoError(t, ValidateEnum("option2", allowed, "field"))
		assert.NoError(t, ValidateEnum("", allowed, "field")) // empty is allowed

		assert.Error(t, ValidateEnum("invalid", allowed, "field"))
	})
}

func TestContains(t *testing.T) {
	t.Run("Should perform case-insensitive substring search", func(t *testing.T) {
		assert.True(t, Contains("Hello World", "hello"))
		assert.True(t, Contains("Hello World", "WORLD"))
		assert.True(t, Contains("Hello World", "lo Wo"))
		assert.False(t, Contains("Hello World", "xyz"))
	})
}

func TestTruncate(t *testing.T) {
	t.Run("Should truncate strings correctly", func(t *testing.T) {
		assert.Equal(t, "hello", Truncate("hello", 10))
		assert.Equal(t, "hello", Truncate("hello", 5))
		assert.Equal(t, "hel...", Truncate("hello world", 6))
		assert.Equal(t, "he", Truncate("hello", 2))
	})
}

func TestLogOperation(t *testing.T) {
	t.Run("Should log operation success", func(t *testing.T) {
		ctx := context.Background()

		err := LogOperation(ctx, "test_operation", func() error {
			return nil
		})

		assert.NoError(t, err)
	})

	t.Run("Should log operation failure", func(t *testing.T) {
		ctx := context.Background()
		expectedErr := NewCliError("TEST_ERROR", "Test failure")

		err := LogOperation(ctx, "test_operation", func() error {
			return expectedErr
		})

		assert.Equal(t, expectedErr, err)
	})
}

func TestWithTimeout(t *testing.T) {
	t.Run("Should execute function within timeout", func(t *testing.T) {
		ctx := context.Background()

		err := WithTimeout(ctx, 100*time.Millisecond, func(_ context.Context) error {
			return nil
		})

		assert.NoError(t, err)
	})

	t.Run("Should timeout long-running function", func(t *testing.T) {
		ctx := context.Background()

		err := WithTimeout(ctx, 10*time.Millisecond, func(_ context.Context) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timed out")
	})

	t.Run("Should not timeout with zero duration", func(t *testing.T) {
		ctx := context.Background()

		err := WithTimeout(ctx, 0, func(_ context.Context) error {
			return nil
		})

		assert.NoError(t, err)
	})
}

func TestPluralize(t *testing.T) {
	t.Run("Should return correct singular/plural forms", func(t *testing.T) {
		assert.Equal(t, "item", Pluralize(1, "item", "items"))
		assert.Equal(t, "items", Pluralize(0, "item", "items"))
		assert.Equal(t, "items", Pluralize(2, "item", "items"))
		assert.Equal(t, "items", Pluralize(10, "item", "items"))
	})
}

func TestFormatDuration(t *testing.T) {
	t.Run("Should format durations correctly", func(t *testing.T) {
		assert.Equal(t, "500ms", FormatDuration(500*time.Millisecond))
		assert.Equal(t, "1.5s", FormatDuration(1500*time.Millisecond))
		assert.Equal(t, "2.5m", FormatDuration(150*time.Second))
		assert.Equal(t, "1.5h", FormatDuration(90*time.Minute))
	})
}

func TestSanitizeForJSON(t *testing.T) {
	t.Run("Should remove non-printable characters", func(t *testing.T) {
		input := "Hello\x00World\x1F"
		expected := "HelloWorld"
		assert.Equal(t, expected, SanitizeForJSON(input))
	})

	t.Run("Should preserve printable characters", func(t *testing.T) {
		input := "Hello World 123!@#"
		assert.Equal(t, input, SanitizeForJSON(input))
	})
}

func TestFileExists(t *testing.T) {
	t.Run("Should return false for non-existent file", func(t *testing.T) {
		assert.False(t, FileExists("/non/existent/file.txt"))
	})

	t.Run("Should return false for directory", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp("", "test_dir")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		assert.False(t, FileExists(tempDir)) // directory should return false
	})
}

func TestDirExists(t *testing.T) {
	t.Run("Should return true for existing directory", func(t *testing.T) {
		// Create a temporary directory for testing
		tempDir, err := os.MkdirTemp("", "test_dir")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		assert.True(t, DirExists(tempDir))
	})

	t.Run("Should return false for non-existent directory", func(t *testing.T) {
		assert.False(t, DirExists("/non/existent/directory"))
	})
}

func TestParseID(t *testing.T) {
	t.Run("Should parse valid UUID", func(t *testing.T) {
		uuid := "123e4567-e89b-12d3-a456-426614174000"
		id, err := ParseID(uuid)

		require.NoError(t, err)
		assert.Equal(t, uuid, string(id))
	})

	t.Run("Should reject invalid UUID", func(t *testing.T) {
		_, err := ParseID("invalid-uuid")
		assert.Error(t, err)
	})
}

func TestValidateIDOrEmpty(t *testing.T) {
	t.Run("Should allow empty ID", func(t *testing.T) {
		assert.NoError(t, ValidateIDOrEmpty(""))
	})

	t.Run("Should validate non-empty ID", func(t *testing.T) {
		assert.NoError(t, ValidateIDOrEmpty("123e4567-e89b-12d3-a456-426614174000"))
		assert.Error(t, ValidateIDOrEmpty("invalid"))
	})
}
