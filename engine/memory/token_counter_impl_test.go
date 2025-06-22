package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTiktokenCounter_NewTiktokenCounter(t *testing.T) {
	ctx := context.Background()
	t.Run("Should create counter with default encoding for empty input", func(t *testing.T) {
		counter, err := NewTiktokenCounter("")
		require.NoError(t, err)
		assert.NotNil(t, counter)
		assert.Equal(t, defaultEncoding, counter.GetEncoding())
		count, err := counter.CountTokens(ctx, "hello world")
		assert.NoError(t, err)
		assert.Equal(t, 2, count) // "hello world" is 2 tokens in cl100k_base
	})

	t.Run("Should create counter with specified valid encoding", func(t *testing.T) {
		counter, err := NewTiktokenCounter("p50k_base") // Use encoding name directly
		require.NoError(t, err)
		assert.NotNil(t, counter)
		assert.Equal(t, "p50k_base", counter.GetEncoding())
		count, err := counter.CountTokens(ctx, "hello world")
		assert.NoError(t, err)
		assert.Equal(t, 2, count) // "hello world" is also 2 tokens in p50k_base
	})

	t.Run("Should create counter for a specific model name", func(t *testing.T) {
		// Test with a model name that tiktoken-go recognizes
		// For example, "gpt-4" uses "cl100k_base"
		counter, err := NewTiktokenCounter("gpt-4")
		require.NoError(t, err)
		assert.NotNil(t, counter)
		assert.Equal(t, "cl100k_base", counter.GetEncoding())
		count, err := counter.CountTokens(ctx, "hello world GPT-4")
		assert.NoError(t, err)
		assert.Equal(t, 6, count) // "hello world GPT-4" is 6 tokens in cl100k_base
	})

	t.Run("Should fallback to default encoding for unknown model/encoding", func(t *testing.T) {
		counter, err := NewTiktokenCounter("unknown-model-or-encoding-123")
		require.NoError(t, err) // NewTiktokenCounter is designed to fallback, not error
		assert.NotNil(t, counter)
		assert.Equal(t, defaultEncoding, counter.GetEncoding())
		count, err := counter.CountTokens(ctx, "test string")
		assert.NoError(t, err)
		assert.Equal(t, 2, count) // "test string" is 2 tokens in cl100k_base
	})

	t.Run("Should return error if default encoding itself is invalid (highly unlikely)", func(_ *testing.T) {
		// This test is hard to make pass without changing defaultEncoding to something invalid
		// and tiktoken-go not having it. For now, this documents the expectation.
		// If NewTiktokenCounter were to error on *any* failure, this test would be different.
		// Currently, it falls back. To test the error path for default, one would need to
		// break tiktoken.GetEncoding(defaultEncoding).
	})
}

func TestTiktokenCounter_CountTokens(t *testing.T) {
	ctx := context.Background()
	counter, err := NewTiktokenCounter(defaultEncoding) // Uses cl100k_base
	require.NoError(t, err)
	require.NotNil(t, counter)

	testCases := []struct {
		name           string
		text           string
		expectedTokens int
	}{
		{"Empty string", "", 0},
		{"Simple phrase", "hello world", 2},
		{"Phrase with punctuation", "Hello, world!", 4}, // Actual token count in cl100k_base
		{"Longer text", "This is a longer test sentence to count tokens.", 10},
		{"Text with numbers", "There are 3 apples and 5 oranges.", 10},
		{"Special characters", "test &%$#@ test", 6}, // Actual token count in cl100k_base
		{"Unicode characters", "こんにちは世界", 4},         // Actual token count in cl100k_base
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := counter.CountTokens(ctx, tc.text)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedTokens, tokens)
		})
	}
}

func TestTiktokenCounter_GetEncoding(t *testing.T) {
	counter, err := NewTiktokenCounter("cl100k_base")
	require.NoError(t, err)
	assert.Equal(t, "cl100k_base", counter.GetEncoding())

	counterP50, err := NewTiktokenCounter("p50k_base")
	require.NoError(t, err)
	assert.Equal(t, "p50k_base", counterP50.GetEncoding())
}

func TestDefaultTokenCounter(t *testing.T) {
	ctx := context.Background()
	counter, err := DefaultTokenCounter()
	require.NoError(t, err)
	require.NotNil(t, counter)
	assert.Equal(t, defaultEncoding, counter.GetEncoding())
	tokens, err := counter.CountTokens(ctx, "some text")
	assert.NoError(t, err)
	assert.Equal(t, 2, tokens) // "some text" is 2 tokens in cl100k_base
}

func TestTiktokenCounter_Uninitialized(t *testing.T) {
	// Test behavior if tke somehow becomes nil (should be prevented by constructor)
	// This is more of a safeguard test.
	ctx := context.Background()
	counter := &TiktokenCounter{encodingName: "test", tke: nil} // Manually create bad state
	_, err := counter.CountTokens(ctx, "text")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tiktoken encoder is not initialized")
}
