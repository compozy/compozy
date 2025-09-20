package tool

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	fixtures "github.com/compozy/compozy/test/fixtures"
	"github.com/stretchr/testify/require"
)

func setupTestTimeout(t *testing.T, toolFile string) (*core.PathCWD, string) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	cwd, dstPath := fixtures.SetupConfigTest(t, filename)
	dstPath = filepath.Join(dstPath, toolFile)
	return cwd, dstPath
}

func TestToolConfig_GetTimeout(t *testing.T) {
	t.Run("Should return global timeout when tool timeout is empty", func(t *testing.T) {
		config := &Config{}
		globalTimeout := 60 * time.Second
		result, err := config.GetTimeout(context.Background(), globalTimeout)
		require.NoError(t, err)
		require.Equal(t, globalTimeout, result)
	})

	t.Run("Should return tool-specific timeout when valid", func(t *testing.T) {
		config := &Config{
			ID:      "test-tool",
			Timeout: "5m",
		}
		globalTimeout := 60 * time.Second
		expected := 5 * time.Minute
		result, err := config.GetTimeout(context.Background(), globalTimeout)
		require.NoError(t, err)
		require.Equal(t, expected, result)
	})

	t.Run("Should return error when tool timeout is invalid", func(t *testing.T) {
		config := &Config{
			ID:      "test-tool",
			Timeout: "invalid-timeout",
		}
		globalTimeout := 60 * time.Second
		log := logger.NewForTests()
		ctx := logger.ContextWithLogger(context.Background(), log)
		result, err := config.GetTimeout(ctx, globalTimeout)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid tool timeout")
		require.Equal(t, time.Duration(0), result)
	})

	t.Run("Should return error for zero timeout", func(t *testing.T) {
		config := &Config{
			ID:      "test-tool",
			Timeout: "0s",
		}
		globalTimeout := 60 * time.Second
		result, err := config.GetTimeout(context.Background(), globalTimeout)
		require.Error(t, err)
		require.Contains(t, err.Error(), "timeout must be positive")
		require.Equal(t, time.Duration(0), result)
	})

	t.Run("Should return error for negative timeout", func(t *testing.T) {
		config := &Config{
			ID:      "test-tool",
			Timeout: "-5s",
		}
		globalTimeout := 60 * time.Second
		result, err := config.GetTimeout(context.Background(), globalTimeout)
		require.Error(t, err)
		require.Contains(t, err.Error(), "timeout must be positive")
		require.Equal(t, time.Duration(0), result)
	})

	t.Run("Should handle various timeout formats", func(t *testing.T) {
		testCases := []struct {
			name     string
			timeout  string
			expected time.Duration
		}{
			{"seconds", "30s", 30 * time.Second},
			{"minutes", "2m", 2 * time.Minute},
			{"hours", "1h", 1 * time.Hour},
			{"mixed", "1h30m", 90 * time.Minute},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := &Config{
					ID:      "test-tool",
					Timeout: tc.timeout,
				}
				globalTimeout := 60 * time.Second
				result, err := config.GetTimeout(context.Background(), globalTimeout)
				require.NoError(t, err)
				require.Equal(t, tc.expected, result)
			})
		}
	})
}

func TestToolConfig_GetTimeout_ContextInteraction(t *testing.T) {
	t.Run("Should cap by ctx deadline when earlier than tool timeout", func(t *testing.T) {
		cfg := &Config{ID: "test-tool", Timeout: "5m"}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		got, err := cfg.GetTimeout(ctx, 10*time.Minute)
		require.NoError(t, err)
		require.LessOrEqual(t, got, 2*time.Second)
	})
	t.Run("Should error when context already expired", func(t *testing.T) {
		cfg := &Config{ID: "test-tool", Timeout: "5m"}
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		time.Sleep(time.Millisecond)
		got, err := cfg.GetTimeout(ctx, 10*time.Minute)
		require.Error(t, err)
		require.Equal(t, time.Duration(0), got)
	})
}

func TestToolConfig_ValidateTimeout(t *testing.T) {
	t.Run("Should validate valid timeout format", func(t *testing.T) {
		cwd, dstPath := setupTestTimeout(t, "basic_tool.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		config.Timeout = "5m"

		err = config.Validate()
		require.NoError(t, err)
	})

	t.Run("Should return error for invalid timeout format", func(t *testing.T) {
		cwd, dstPath := setupTestTimeout(t, "basic_tool.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		config.Timeout = "invalid-timeout"

		err = config.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid timeout format")
	})

	t.Run("Should return error for zero timeout", func(t *testing.T) {
		cwd, dstPath := setupTestTimeout(t, "basic_tool.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		config.Timeout = "0s"

		err = config.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "timeout must be positive")
	})

	t.Run("Should return error for negative timeout", func(t *testing.T) {
		cwd, dstPath := setupTestTimeout(t, "basic_tool.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		config.Timeout = "-5s"

		err = config.Validate()
		require.Error(t, err)
		require.Contains(t, err.Error(), "timeout must be positive")
	})

	t.Run("Should validate successfully when timeout is empty", func(t *testing.T) {
		cwd, dstPath := setupTestTimeout(t, "basic_tool.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)
		config.Timeout = ""

		err = config.Validate()
		require.NoError(t, err)
	})
}

func TestToolConfig_Integration(t *testing.T) {
	t.Run("Should load and parse tool with timeout configuration", func(t *testing.T) {
		cwd, dstPath := setupTestTimeout(t, "quick_tool.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)

		// Verify timeout field is loaded correctly
		require.Equal(t, "30s", config.Timeout)

		// Verify GetTimeout returns correct duration
		globalTimeout := 60 * time.Second
		toolTimeout, err := config.GetTimeout(context.Background(), globalTimeout)
		require.NoError(t, err)
		require.Equal(t, 30*time.Second, toolTimeout)

		// Verify config validates successfully
		err = config.Validate()
		require.NoError(t, err)
	})

	t.Run("Should load tool with long timeout configuration", func(t *testing.T) {
		cwd, dstPath := setupTestTimeout(t, "slow_tool.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)

		// Verify timeout field is loaded correctly
		require.Equal(t, "10m", config.Timeout)

		// Verify GetTimeout returns correct duration
		globalTimeout := 60 * time.Second
		toolTimeout, err := config.GetTimeout(context.Background(), globalTimeout)
		require.NoError(t, err)
		require.Equal(t, 10*time.Minute, toolTimeout)

		// Verify config validates successfully
		err = config.Validate()
		require.NoError(t, err)
	})

	t.Run("Should use global timeout when tool has no timeout configured", func(t *testing.T) {
		cwd, dstPath := setupTestTimeout(t, "basic_tool.yaml")
		config, err := Load(cwd, dstPath)
		require.NoError(t, err)

		// Verify timeout field is empty (not configured in basic_tool.yaml)
		require.Equal(t, "", config.Timeout)

		// Verify GetTimeout returns global timeout
		globalTimeout := 120 * time.Second
		toolTimeout, err := config.GetTimeout(context.Background(), globalTimeout)
		require.NoError(t, err)
		require.Equal(t, globalTimeout, toolTimeout)
	})
}
