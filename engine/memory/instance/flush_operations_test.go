package instance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/pkg/logger"
)

func TestFlushOperations_calculateTokenCount(t *testing.T) {
	t.Run("Should use operations component for token counting when available", func(t *testing.T) {
		ctx := context.Background()
		log := logger.NewForTests()

		// Create mock token counter
		mockTokenCounter := &flushTestTokenCounter{
			tokenCount: 42, // Will return 42 for any text
		}

		// Create operations component with mock token counter
		ops := &Operations{
			tokenCounter: mockTokenCounter,
			logger:       log,
		}

		// Create flush operations with operations component
		fo := &FlushOperations{
			operations: ops,
		}

		// Test message
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "Test message for token counting",
		}

		// Calculate token count
		count := fo.calculateTokenCount(ctx, msg)

		// Operations calculateTokenCount adds:
		// - Content tokens: 42 (from mock)
		// - Role tokens: 42 (from mock, since it's called for role too)
		// - Structure overhead: 2
		// Total: 42 + 42 + 2 = 86
		assert.Equal(t, 86, count)
		assert.True(t, mockTokenCounter.countTokensCalled)
	})

	t.Run("Should fall back to estimation when operations component is nil", func(t *testing.T) {
		ctx := context.Background()

		// Create flush operations without operations component
		fo := &FlushOperations{
			operations: nil,
		}

		// Test message with known content length
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "Test", // 4 characters
		}

		// Calculate token count
		count := fo.calculateTokenCount(ctx, msg)

		// Should use fallback estimation (len/4 = 1)
		assert.Equal(t, 1, count)
	})

	t.Run("Should handle empty content gracefully", func(t *testing.T) {
		ctx := context.Background()

		// Create flush operations without operations component
		fo := &FlushOperations{
			operations: nil,
		}

		// Test message with empty content
		msg := llm.Message{
			Role:    llm.MessageRoleUser,
			Content: "",
		}

		// Calculate token count
		count := fo.calculateTokenCount(ctx, msg)

		// Should return 0 for empty content (based on actual implementation)
		assert.Equal(t, 0, count)
	})

	t.Run("Should handle various content lengths correctly", func(t *testing.T) {
		ctx := context.Background()

		// Create flush operations without operations component
		fo := &FlushOperations{
			operations: nil,
		}

		testCases := []struct {
			content  string
			expected int
		}{
			{"", 0},            // Empty: 0 tokens
			{"Hi", 1},          // 2 chars: min 1
			{"Test", 1},        // 4 chars: 1 token
			{"Hello World", 2}, // 11 chars: 2 tokens
			{"This is a longer test message with more content", 11}, // 47 chars: 11 tokens
		}

		for _, tc := range testCases {
			msg := llm.Message{
				Role:    llm.MessageRoleUser,
				Content: tc.content,
			}

			count := fo.calculateTokenCount(ctx, msg)
			assert.Equal(t, tc.expected, count, "Content: %s", tc.content)
		}
	})
}

func TestFlushOperations_estimateTokenCount(t *testing.T) {
	t.Run("Should estimate token count correctly", func(t *testing.T) {
		fo := &FlushOperations{}

		testCases := []struct {
			content  string
			expected int
		}{
			{"", 0},                               // Empty: 0 tokens
			{"Hi", 1},                             // 2 chars: min 1
			{"Test", 1},                           // 4 chars: 1 token
			{"Hello World", 2},                    // 11 chars: 2 tokens
			{"This is a test", 3},                 // 14 chars: 3 tokens
			{"A" + string(make([]byte, 100)), 25}, // 101 chars: 25 tokens
		}

		for _, tc := range testCases {
			count := fo.estimateTokenCount(tc.content)
			assert.Equal(t, tc.expected, count, "Content length: %d", len(tc.content))
		}
	})
}

// Mock token counter for testing
type flushTestTokenCounter struct {
	tokenCount        int
	countTokensCalled bool
}

func (m *flushTestTokenCounter) CountTokens(_ context.Context, _ string) (int, error) {
	m.countTokensCalled = true
	return m.tokenCount, nil
}

func (m *flushTestTokenCounter) EncodeTokens(_ context.Context, _ string) ([]int, error) {
	// Return a simple token encoding based on content length
	return []int{1, 2, 3}, nil // Placeholder encoding
}

func (m *flushTestTokenCounter) DecodeTokens(_ context.Context, _ []int) (string, error) {
	return "decoded", nil
}

func (m *flushTestTokenCounter) GetEncoding() string {
	return "test-encoding"
}
