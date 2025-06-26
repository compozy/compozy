package tokens

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTiktokenCounter_NewTiktokenCounter(t *testing.T) {
	t.Run("Should create counter with default encoding", func(t *testing.T) {
		counter, err := NewTiktokenCounter("")
		require.NoError(t, err)
		assert.NotNil(t, counter)
		assert.Equal(t, defaultEncoding, counter.GetEncoding())
	})

	t.Run("Should create counter with specific encoding", func(t *testing.T) {
		counter, err := NewTiktokenCounter("cl100k_base")
		require.NoError(t, err)
		assert.NotNil(t, counter)
		assert.Equal(t, "cl100k_base", counter.GetEncoding())
	})

	t.Run("Should create counter for GPT-4 model", func(t *testing.T) {
		counter, err := NewTiktokenCounter("gpt-4")
		require.NoError(t, err)
		assert.NotNil(t, counter)
		assert.Equal(t, "cl100k_base", counter.GetEncoding())
	})

	t.Run("Should fallback to default for unknown model", func(t *testing.T) {
		counter, err := NewTiktokenCounter("unknown-model")
		require.NoError(t, err)
		assert.NotNil(t, counter)
		assert.Equal(t, defaultEncoding, counter.GetEncoding())
	})
}

func TestTiktokenCounter_CountTokens(t *testing.T) {
	counter, err := NewTiktokenCounter("gpt-4")
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("Should count tokens for simple text", func(t *testing.T) {
		count, err := counter.CountTokens(ctx, "Hello, world!")
		require.NoError(t, err)
		assert.Greater(t, count, 0)
		assert.LessOrEqual(t, count, 10) // Reasonable upper bound for simple text
	})

	t.Run("Should return zero for empty text", func(t *testing.T) {
		count, err := counter.CountTokens(ctx, "")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Should count tokens for longer text", func(t *testing.T) {
		longText := "This is a longer piece of text that should result in more tokens when processed by the tokenizer."
		count, err := counter.CountTokens(ctx, longText)
		require.NoError(t, err)
		assert.Greater(t, count, 10)
		assert.LessOrEqual(t, count, 50) // Reasonable upper bound
	})

	t.Run("Should handle special characters", func(t *testing.T) {
		specialText := "Special chars: !@#$%^&*()_+-=[]{}|;:'\",.<>?/~`"
		count, err := counter.CountTokens(ctx, specialText)
		require.NoError(t, err)
		assert.Greater(t, count, 0)
	})

	t.Run("Should handle unicode characters", func(t *testing.T) {
		unicodeText := "Unicode: 🌟🚀💡 こんにちは 您好 Здравствуйте"
		count, err := counter.CountTokens(ctx, unicodeText)
		require.NoError(t, err)
		assert.Greater(t, count, 0)
	})
}

func TestTiktokenCounter_GetEncoding(t *testing.T) {
	t.Run("Should return correct encoding name", func(t *testing.T) {
		counter, err := NewTiktokenCounter("cl100k_base")
		require.NoError(t, err)

		encoding := counter.GetEncoding()
		assert.Equal(t, "cl100k_base", encoding)
	})
}

func TestDefaultTokenCounter(t *testing.T) {
	t.Run("Should create default token counter", func(t *testing.T) {
		counter, err := DefaultTokenCounter()
		require.NoError(t, err)
		assert.NotNil(t, counter)

		// Test that it can count tokens
		ctx := context.Background()
		count, err := counter.CountTokens(ctx, "Test message")
		require.NoError(t, err)
		assert.Greater(t, count, 0)
	})
}

func TestGetEncodingNameForModel(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"gpt-4", "cl100k_base"},
		{"gpt-4-0314", "cl100k_base"},
		{"gpt-3.5-turbo", "cl100k_base"},
		{"text-davinci-003", "p50k_base"},
		{"unknown-model", defaultEncoding},
		{"", defaultEncoding},
	}

	for _, test := range tests {
		t.Run("Should return correct encoding for "+test.model, func(t *testing.T) {
			result := getEncodingNameForModel(test.model)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestTiktokenCounter_ConcurrentAccess(t *testing.T) {
	counter, err := NewTiktokenCounter("gpt-4")
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("Should handle concurrent token counting", func(t *testing.T) {
		const numGoroutines = 10
		results := make(chan int, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(text string) {
				count, err := counter.CountTokens(ctx, text)
				require.NoError(t, err)
				results <- count
			}("Test message for goroutine")
		}

		// Collect results
		counts := make([]int, numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			counts[i] = <-results
		}

		// All counts should be the same for the same input
		for i := 1; i < numGoroutines; i++ {
			assert.Equal(t, counts[0], counts[i])
		}
	})
}
