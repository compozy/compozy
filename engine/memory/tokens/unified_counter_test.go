package tokens

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTokenCounter implements TokenCounter interface for testing
type mockTokenCounter struct {
	countFunc    func(ctx context.Context, text string) (int, error)
	encodingFunc func() string
}

func (m *mockTokenCounter) CountTokens(ctx context.Context, text string) (int, error) {
	if m.countFunc != nil {
		return m.countFunc(ctx, text)
	}
	return len(text), nil // Default: return text length
}

func (m *mockTokenCounter) GetEncoding() string {
	if m.encodingFunc != nil {
		return m.encodingFunc()
	}
	return "mock-encoding"
}

func TestNewUnifiedTokenCounter(t *testing.T) {
	t.Run("Should create with valid parameters", func(t *testing.T) {
		fallback := &mockTokenCounter{}
		config := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "test-key",
		}
		counter, err := NewUnifiedTokenCounter(config, fallback)
		require.NoError(t, err)
		assert.NotNil(t, counter)
		assert.Equal(t, config, counter.GetProviderConfig())
	})
	t.Run("Should fail with nil fallback counter", func(t *testing.T) {
		config := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "test-key",
		}
		counter, err := NewUnifiedTokenCounter(config, nil)
		assert.Error(t, err)
		assert.Nil(t, counter)
		assert.Contains(t, err.Error(), "fallback counter cannot be nil")
	})
	t.Run("Should create with nil provider config", func(t *testing.T) {
		fallback := &mockTokenCounter{}
		counter, err := NewUnifiedTokenCounter(nil, fallback)
		require.NoError(t, err)
		assert.NotNil(t, counter)
		assert.Nil(t, counter.GetProviderConfig())
	})
}

func TestUnifiedTokenCounter_CountTokens(t *testing.T) {
	t.Run("Should use fallback when no provider config", func(t *testing.T) {
		fallback := &mockTokenCounter{
			countFunc: func(_ context.Context, _ string) (int, error) {
				return 42, nil
			},
		}
		counter, err := NewUnifiedTokenCounter(nil, fallback)
		require.NoError(t, err)
		ctx := t.Context()
		count, err := counter.CountTokens(ctx, "test text")
		require.NoError(t, err)
		assert.Equal(t, 42, count)
	})
	t.Run("Should use fallback when no API key", func(t *testing.T) {
		fallback := &mockTokenCounter{
			countFunc: func(_ context.Context, _ string) (int, error) {
				return 84, nil
			},
		}
		config := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "", // No API key
		}
		counter, err := NewUnifiedTokenCounter(config, fallback)
		require.NoError(t, err)
		ctx := t.Context()
		count, err := counter.CountTokens(ctx, "test text")
		require.NoError(t, err)
		assert.Equal(t, 84, count)
	})
	t.Run("Should use fallback when API returns zero", func(t *testing.T) {
		fallback := &mockTokenCounter{
			countFunc: func(_ context.Context, _ string) (int, error) {
				return 126, nil
			},
		}
		config := &ProviderConfig{
			Provider: "UnsupportedProvider", // This will return 0 from alembica
			Model:    "test-model",
			APIKey:   "test-key",
		}
		counter, err := NewUnifiedTokenCounter(config, fallback)
		require.NoError(t, err)
		ctx := t.Context()
		count, err := counter.CountTokens(ctx, "test text")
		require.NoError(t, err)
		assert.Equal(t, 126, count) // Should fallback
	})
	t.Run("Should handle timeout with fallback", func(t *testing.T) {
		fallback := &mockTokenCounter{
			countFunc: func(_ context.Context, _ string) (int, error) {
				return 999, nil
			},
		}
		config := &ProviderConfig{
			Provider: "SlowProvider", // This would trigger timeout in real scenario
			Model:    "slow-model",
			APIKey:   "test-key",
		}
		counter, err := NewUnifiedTokenCounter(config, fallback)
		require.NoError(t, err)
		// Create a context that's already canceled to simulate timeout
		ctx, cancel := context.WithTimeout(t.Context(), 1*time.Millisecond)
		defer cancel()
		time.Sleep(2 * time.Millisecond) // Ensure timeout
		count, err := counter.CountTokens(ctx, "test text")
		require.NoError(t, err)
		assert.Equal(t, 999, count) // Should use fallback due to timeout
	})
}

func TestUnifiedTokenCounter_GetEncoding(t *testing.T) {
	t.Run("Should return provider info when configured", func(t *testing.T) {
		fallback := &mockTokenCounter{
			encodingFunc: func() string {
				return "fallback-encoding"
			},
		}
		config := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "test-key",
		}
		counter, err := NewUnifiedTokenCounter(config, fallback)
		require.NoError(t, err)
		encoding := counter.GetEncoding()
		assert.Equal(t, "OpenAI-gpt-4", encoding)
	})
	t.Run("Should return fallback encoding when no provider config", func(t *testing.T) {
		fallback := &mockTokenCounter{
			encodingFunc: func() string {
				return "fallback-encoding"
			},
		}
		counter, err := NewUnifiedTokenCounter(nil, fallback)
		require.NoError(t, err)
		encoding := counter.GetEncoding()
		assert.Equal(t, "fallback-encoding", encoding)
	})
}

func TestUnifiedTokenCounter_UpdateProvider(t *testing.T) {
	t.Run("Should update provider configuration", func(t *testing.T) {
		fallback := &mockTokenCounter{}
		initialConfig := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "test-key",
		}
		counter, err := NewUnifiedTokenCounter(initialConfig, fallback)
		require.NoError(t, err)
		// Update configuration
		newConfig := &ProviderConfig{
			Provider: "Anthropic",
			Model:    "claude-3",
			APIKey:   "new-key",
		}
		counter.UpdateProvider(newConfig)
		assert.Equal(t, newConfig, counter.GetProviderConfig())
		// Verify encoding reflects new config
		encoding := counter.GetEncoding()
		assert.Equal(t, "Anthropic-claude-3", encoding)
	})
}

func TestUnifiedTokenCounter_IsFallbackActive(t *testing.T) {
	t.Run("Should return true when no provider config", func(t *testing.T) {
		fallback := &mockTokenCounter{}
		counter, err := NewUnifiedTokenCounter(nil, fallback)
		require.NoError(t, err)
		assert.True(t, counter.IsFallbackActive())
	})
	t.Run("Should return true when no API key", func(t *testing.T) {
		fallback := &mockTokenCounter{}
		config := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "", // No API key
		}
		counter, err := NewUnifiedTokenCounter(config, fallback)
		require.NoError(t, err)
		assert.True(t, counter.IsFallbackActive())
	})
	t.Run("Should return false when fully configured", func(t *testing.T) {
		fallback := &mockTokenCounter{}
		config := &ProviderConfig{
			Provider: "OpenAI",
			Model:    "gpt-4",
			APIKey:   "test-key",
		}
		counter, err := NewUnifiedTokenCounter(config, fallback)
		require.NoError(t, err)
		assert.False(t, counter.IsFallbackActive())
	})
}

func TestUnifiedTokenCounter_ThreadSafety(t *testing.T) {
	t.Run("Should handle concurrent access safely", func(t *testing.T) {
		fallback := &mockTokenCounter{
			countFunc: func(_ context.Context, text string) (int, error) {
				return len(text), nil
			},
		}
		counter, err := NewUnifiedTokenCounter(nil, fallback)
		require.NoError(t, err)
		// Run multiple goroutines concurrently
		ctx := t.Context()
		done := make(chan bool, 10)
		for i := range 10 {
			go func(i int) {
				// Test reading
				config := counter.GetProviderConfig()
				_ = config
				encoding := counter.GetEncoding()
				_ = encoding
				isActive := counter.IsFallbackActive()
				_ = isActive
				// Test counting
				count, err := counter.CountTokens(ctx, "test")
				assert.NoError(t, err)
				assert.True(t, count > 0) // Just verify it returns a positive count
				// Test updating (alternating configs)
				if i%2 == 0 {
					counter.UpdateProvider(&ProviderConfig{
						Provider: "OpenAI",
						Model:    "gpt-4",
						APIKey:   "key1",
					})
				} else {
					counter.UpdateProvider(&ProviderConfig{
						Provider: "Anthropic",
						Model:    "claude-3",
						APIKey:   "key2",
					})
				}
				done <- true
			}(i)
		}
		// Wait for all goroutines to complete
		for range 10 {
			<-done
		}
	})
}
