package config

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoader_Load(t *testing.T) {
	t.Run("Should load default configuration when no sources provided", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		loader := NewService()

		// Act
		cfg, err := loader.Load(ctx)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Should have default values
		assert.Equal(t, "0.0.0.0", cfg.Server.Host)
		assert.Equal(t, 5001, cfg.Server.Port)
		assert.Equal(t, "development", cfg.Runtime.Environment)
	})

	t.Run("Should apply sources in precedence order", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		loader := NewService()

		// Create mock sources with different values
		// Note: SourceEnv is now handled natively by koanf, so we use SourceYAML
		source1 := &mockSource{
			data: map[string]any{
				"server": map[string]any{
					"host": "source1.example.com",
					"port": 9001,
				},
			},
			sourceType: SourceYAML,
		}

		source2 := &mockSource{
			data: map[string]any{
				"server": map[string]any{
					"host": "source2.example.com",
					// Port not overridden, should keep source1 value
				},
			},
			sourceType: SourceCLI,
		}

		// Act
		cfg, err := loader.Load(ctx, source1, source2)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// CLI (source2) should override YAML (source1) for host
		assert.Equal(t, "source2.example.com", cfg.Server.Host)
		// Port should retain source1 value since source2 didn't override
		assert.Equal(t, 9001, cfg.Server.Port)
	})

	t.Run("Should validate configuration after loading", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		loader := NewService()

		// Create source with invalid port
		source := &mockSource{
			data: map[string]any{
				"server": map[string]any{
					"port": 99999, // Invalid port
				},
			},
			sourceType: SourceYAML,
		}

		// Act
		cfg, err := loader.Load(ctx, source)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
		assert.Nil(t, cfg)
	})

	t.Run("Should handle nil sources gracefully", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		loader := NewService()

		validSource := &mockSource{
			data: map[string]any{
				"server": map[string]any{
					"host": "valid.example.com",
				},
			},
			sourceType: SourceCLI,
		}

		// Act
		cfg, err := loader.Load(ctx, nil, validSource, nil)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "valid.example.com", cfg.Server.Host)
	})

	t.Run("Should handle source loading errors", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		loader := NewService()

		source := &mockSource{
			loadErr:    assert.AnError,
			sourceType: SourceCLI,
		}

		// Act
		cfg, err := loader.Load(ctx, source)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load from source")
		assert.Nil(t, cfg)
	})
}

func TestLoader_Validate(t *testing.T) {
	t.Run("Should accept valid configuration", func(t *testing.T) {
		// Arrange
		loader := NewService()
		cfg := Default()

		// Act
		err := loader.Validate(cfg)

		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should reject nil configuration", func(t *testing.T) {
		// Arrange
		loader := NewService()

		// Act
		err := loader.Validate(nil)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "configuration cannot be nil")
	})

	t.Run("Should reject invalid struct tag validation", func(t *testing.T) {
		// Arrange
		loader := NewService()
		cfg := Default()
		cfg.Server.Port = 0 // Invalid port

		// Act
		err := loader.Validate(cfg)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("Should reject invalid custom validation", func(t *testing.T) {
		// Arrange
		loader := NewService()
		cfg := Default()
		cfg.Runtime.DispatcherHeartbeatInterval = 60 * time.Second
		cfg.Runtime.DispatcherHeartbeatTTL = 30 * time.Second // TTL less than interval

		// Act
		err := loader.Validate(cfg)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dispatcher heartbeat TTL must be greater than heartbeat interval")
	})
}

func TestLoader_GetSource(t *testing.T) {
	t.Run("Should return SourceDefault for backward compatibility", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		loader := NewService()

		source := &mockSource{
			data: map[string]any{
				"server": map[string]any{
					"host": "tracked.example.com",
				},
			},
			sourceType: SourceCLI,
		}

		// Act
		_, err := loader.Load(ctx, source)
		require.NoError(t, err)

		// Assert - GetSource now always returns SourceDefault
		// as source tracking is handled internally by koanf
		assert.Equal(t, SourceDefault, loader.GetSource("server"))
		assert.Equal(t, SourceDefault, loader.GetSource("database"))
		assert.Equal(t, SourceDefault, loader.GetSource("nonexistent"))
	})
}

func TestLoader_Watch(t *testing.T) {
	t.Run("Should accept watch callbacks", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		loader := NewService()
		called := false
		callback := func(*Config) {
			called = true
		}

		// Act
		err := loader.Watch(ctx, callback)

		// Assert
		assert.NoError(t, err)
		// Note: Hot-reload not implemented yet, so callback won't be called
		assert.False(t, called)
	})

	t.Run("Should reject nil callback", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		loader := NewService()

		// Act
		err := loader.Watch(ctx, nil)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "callback cannot be nil")
	})
}

// mockSource is a test implementation of the Source interface
type mockSource struct {
	data       map[string]any
	sourceType SourceType
	loadErr    error
}

func (m *mockSource) Load() (map[string]any, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.data, nil
}

func (m *mockSource) Watch(_ context.Context, _ func()) error {
	return nil
}

func (m *mockSource) Type() SourceType {
	return m.sourceType
}

func (m *mockSource) Close() error {
	return nil
}

func TestTransformEnvKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Should handle standard environment variable",
			input:    "LIMITS_MAX_NESTING_DEPTH",
			expected: "limits.max_nesting_depth",
		},
		{
			name:     "Should handle single part",
			input:    "PORT",
			expected: "port",
		},
		{
			name:     "Should handle empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Should handle double underscore",
			input:    "FOO__BAR",
			expected: "foo.bar",
		},
		{
			name:     "Should handle leading underscore",
			input:    "_FOO_BAR",
			expected: "foo.bar",
		},
		{
			name:     "Should handle trailing underscore",
			input:    "FOO_BAR_",
			expected: "foo.bar",
		},
		{
			name:     "Should handle multiple consecutive underscores",
			input:    "FOO___BAR",
			expected: "foo.bar",
		},
		{
			name:     "Should handle only underscores",
			input:    "___",
			expected: "",
		},
		{
			name:     "Should preserve underscores in nested parts",
			input:    "SERVER_MAX_REQUEST_SIZE",
			expected: "server.max_request_size",
		},
		{
			name:     "Should handle mixed case",
			input:    "MiXeD_CaSe_VaR",
			expected: "mixed.case_var",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformEnvKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
