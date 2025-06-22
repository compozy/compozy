package memory

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenMemoryManager(t *testing.T) {
	mockCounter, _ := NewTiktokenCounter(defaultEncoding)
	cfg := &Resource{Type: TokenBasedMemory, MaxTokens: 100}

	t.Run("Should create new manager with valid config and counter", func(t *testing.T) {
		tm, err := NewTokenMemoryManager(cfg, mockCounter, nil)
		assert.NoError(t, err)
		assert.NotNil(t, tm)
	})

	t.Run("Should return error if config is nil", func(t *testing.T) {
		_, err := NewTokenMemoryManager(nil, mockCounter, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "memory resource config cannot be nil")
	})

	t.Run("Should return error if counter is nil", func(t *testing.T) {
		_, err := NewTokenMemoryManager(cfg, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token counter cannot be nil")
	})

	t.Run("Should create manager even if no token limits are set (logs warning internally)", func(t *testing.T) {
		cfgNoTokenLimit := &Resource{Type: MessageCountBasedMemory, MaxMessages: 10}
		tm, err := NewTokenMemoryManager(cfgNoTokenLimit, mockCounter, nil)
		assert.NoError(t, err)
		assert.NotNil(t, tm)
		// Test that it doesn't immediately fail, internal logging is not checked here
	})
}

func TestTokenMemoryManager_CalculateMessagesWithTokens(t *testing.T) {
	ctx := context.Background()
	mockCounter, _ := NewTiktokenCounter(defaultEncoding) // cl100k_base
	tm, _ := NewTokenMemoryManager(&Resource{Type: TokenBasedMemory}, mockCounter, nil)

	messages := []llm.Message{
		{Content: "Hello world"},                          // 2 tokens
		{Content: "This is a test."},                      // 5 tokens
		{Content: "Another one for the road, my friend."}, // 9 tokens
	}

	expectedTotalTokens := 2 + 5 + 9

	processedMessages, totalTokens, err := tm.CalculateMessagesWithTokens(ctx, messages)
	require.NoError(t, err)
	assert.Len(t, processedMessages, 3)
	assert.Equal(t, expectedTotalTokens, totalTokens)

	assert.Equal(t, 2, processedMessages[0].TokenCount)
	assert.Equal(t, messages[0].Content, processedMessages[0].Content)
	assert.Equal(t, 5, processedMessages[1].TokenCount)
	assert.Equal(t, messages[1].Content, processedMessages[1].Content)
	assert.Equal(t, 9, processedMessages[2].TokenCount)
	assert.Equal(t, messages[2].Content, processedMessages[2].Content)
}

func TestTokenMemoryManager_EnforceLimits_TokenLimit(t *testing.T) {
	ctx := context.Background()
	mockCounter, _ := NewTiktokenCounter(defaultEncoding)
	cfg := &Resource{Type: TokenBasedMemory, MaxTokens: 10} // Max 10 tokens
	tm, _ := NewTokenMemoryManager(cfg, mockCounter, nil)

	// Total 16 tokens, Max 10. Should evict first message (2 tokens) -> 14 tokens.
	// Then evict second (5 tokens) -> 9 tokens. Keeps last message (9 tokens).
	messages := []MessageWithTokens{
		{Message: llm.Message{Content: "Hi"}, TokenCount: 1},                                       // Evicted
		{Message: llm.Message{Content: "Hello there"}, TokenCount: 2},                              // Evicted
		{Message: llm.Message{Content: "Test sentence"}, TokenCount: 3},                            // Evicted
		{Message: llm.Message{Content: "This is a much longer sentence to keep."}, TokenCount: 10}, // Kept
	} // Total 1+2+3+10 = 16 tokens

	currentTotalTokens := 0
	for _, m := range messages {
		currentTotalTokens += m.TokenCount
	}

	finalMessages, finalTokens, err := tm.EnforceLimits(ctx, messages, currentTotalTokens)
	require.NoError(t, err)

	assert.Len(t, finalMessages, 1)
	assert.Equal(t, 10, finalTokens)
	assert.Equal(t, "This is a much longer sentence to keep.", finalMessages[0].Content)
}

func TestTokenMemoryManager_EnforceLimits_MessageLimit(t *testing.T) {
	ctx := context.Background()
	mockCounter, _ := NewTiktokenCounter(defaultEncoding)
	// No token limit, but message limit of 2
	cfg := &Resource{Type: TokenBasedMemory, MaxMessages: 2}
	tm, _ := NewTokenMemoryManager(cfg, mockCounter, nil)

	messages := []MessageWithTokens{
		{Message: llm.Message{Content: "Msg1"}, TokenCount: 1}, // Evicted
		{Message: llm.Message{Content: "Msg2"}, TokenCount: 1}, // Kept
		{Message: llm.Message{Content: "Msg3"}, TokenCount: 1}, // Kept
	}
	currentTotalTokens := 3

	finalMessages, finalTokens, err := tm.EnforceLimits(ctx, messages, currentTotalTokens)
	require.NoError(t, err)

	assert.Len(t, finalMessages, 2)
	assert.Equal(t, 2, finalTokens) // 1+1
	assert.Equal(t, "Msg2", finalMessages[0].Content)
	assert.Equal(t, "Msg3", finalMessages[1].Content)
}

func TestTokenMemoryManager_EnforceLimits_BothLimits(t *testing.T) {
	ctx := context.Background()
	mockCounter, _ := NewTiktokenCounter(defaultEncoding)
	cfg := &Resource{Type: TokenBasedMemory, MaxTokens: 7, MaxMessages: 2}
	tm, _ := NewTokenMemoryManager(cfg, mockCounter, nil)

	// Msg1 (3t), Msg2 (3t), Msg3 (3t), Msg4 (3t) = 12 tokens, 4 messages
	// Token limit (7): Evicts Msg1, Msg2. Remaining: Msg3 (3t), Msg4 (3t) = 6 tokens, 2 messages.
	// Message limit (2): Already satisfied.
	// Result: Msg3, Msg4.
	messages := []MessageWithTokens{
		{Message: llm.Message{Content: "One two three"}, TokenCount: 3},     // Evicted by token limit
		{Message: llm.Message{Content: "Four five six"}, TokenCount: 3},     // Evicted by token limit
		{Message: llm.Message{Content: "Seven eight nine"}, TokenCount: 3},  // Kept
		{Message: llm.Message{Content: "Ten eleven twelve"}, TokenCount: 3}, // Kept
	}
	currentTotalTokens := 12

	finalMessages, finalTokens, err := tm.EnforceLimits(ctx, messages, currentTotalTokens)
	require.NoError(t, err)

	assert.Len(t, finalMessages, 2)
	assert.Equal(t, 6, finalTokens)
	assert.Equal(t, "Seven eight nine", finalMessages[0].Content)
	assert.Equal(t, "Ten eleven twelve", finalMessages[1].Content)
}

func TestTokenMemoryManager_EnforceLimits_MaxContextRatio(t *testing.T) {
	ctx := context.Background()
	mockCounter, _ := NewTiktokenCounter(defaultEncoding)
	// MaxTokens not set, MaxContextRatio 0.001 of 4096 = 4.096 -> effectively 4 tokens
	cfg := &Resource{Type: TokenBasedMemory, MaxContextRatio: 0.001} // Assuming 4096 default context
	tm, _ := NewTokenMemoryManager(cfg, mockCounter, nil)

	messages := []MessageWithTokens{
		{Message: llm.Message{Content: "One two three"}, TokenCount: 3},       // Evicted
		{Message: llm.Message{Content: "Four five six seven"}, TokenCount: 4}, // Kept
	} // Total 7 tokens. Max effective 4.
	currentTotalTokens := 7

	finalMessages, finalTokens, err := tm.EnforceLimits(ctx, messages, currentTotalTokens)
	require.NoError(t, err)

	assert.Len(t, finalMessages, 1)
	assert.Equal(t, 4, finalTokens)
	assert.Equal(t, "Four five six seven", finalMessages[0].Content)
}

func TestTokenMemoryManager_GetManagedMessages(t *testing.T) {
	ctx := context.Background()
	mockCounter, _ := NewTiktokenCounter(defaultEncoding)
	cfg := &Resource{Type: TokenBasedMemory, MaxTokens: 5}
	tm, _ := NewTokenMemoryManager(cfg, mockCounter, nil)

	inputMessages := []llm.Message{
		{Content: "Hello world"},      // 2 tokens, kept
		{Content: "Test sentence."},   // 3 tokens, kept
		{Content: "This is too long"}, // 4 tokens, would be evicted if processed first
	}
	// If processed in order:
	// 1. "Hello world" (2t) - total 2t
	// 2. "Test sentence." (3t) - total 5t
	// This test case seems to be wrong in the comment. The GetManagedMessages will calculate all first.
	// Total tokens = 2 + 3 + 4 = 9. MaxTokens = 5.
	// Evict "Hello world" (2t) -> remaining 7t.
	// Evict "Test sentence." (3t) -> remaining 4t.
	// Keeps "This is too long" (4t).

	managedMessages, finalTokens, err := tm.GetManagedMessages(ctx, inputMessages)
	require.NoError(t, err)

	assert.Len(t, managedMessages, 1)
	assert.Equal(t, 4, finalTokens)
	assert.Equal(t, "This is too long", managedMessages[0].Content)
}

func TestTokenMemoryManager_EnforceLimitsWithPriority(t *testing.T) {
	ctx := context.Background()
	mockCounter, _ := NewTiktokenCounter(defaultEncoding)
	cfg := &Resource{Type: TokenBasedMemory, MaxTokens: 7} // Max 7 tokens
	tm, _ := NewTokenMemoryManager(cfg, mockCounter, nil)

	// Messages with priorities. Total tokens = 3+3+3+3 = 12. Max = 7.
	// Need to evict 5 tokens.
	// Prio 0: Critical (keep)
	// Prio 1: Normal
	// Prio 2: Low (evict first)
	messages := []MessageWithPriorityAndTokens{
		{
			MessageWithTokens: MessageWithTokens{Message: llm.Message{Content: "Critical Sys Msg"}, TokenCount: 3},
			Priority:          0,
		}, // Keep
		{
			MessageWithTokens: MessageWithTokens{Message: llm.Message{Content: "Low Prio Old"}, TokenCount: 3},
			Priority:          2,
		}, // Evict this (3 tokens)
		{
			MessageWithTokens: MessageWithTokens{Message: llm.Message{Content: "Normal Prio"}, TokenCount: 3},
			Priority:          1,
		}, // Keep
		{
			MessageWithTokens: MessageWithTokens{Message: llm.Message{Content: "Low Prio New"}, TokenCount: 3},
			Priority:          2,
		}, // Evict this (another 2 from this one)
	}
	// Expected after sorting for eviction (Prio DESC, Original Index ASC):
	// Low Prio Old (P2, Idx1, 3t) -> Evict fully (need 5, got 3, need 2 more)
	// Low Prio New (P2, Idx3, 3t) -> Evict partially (need 2, this has 3. Evict this one)
	// Total evicted = Low Prio Old (3t) + Low Prio New (3t) = 6 tokens.
	// Kept: Critical (3t) + Normal (3t) = 6 tokens. This fits.

	currentTotalTokens := 12

	// The EnforceLimitsWithPriority sorts for eviction (lowest actual prio, oldest first).
	// So, (Low Prio Old, P2, 3t), (Low Prio New, P2, 3t), (Normal Prio, P1, 3t), (Critical Sys Msg, P0, 3t)
	// Evict Low Prio Old (3t). Tokens to evict = 5-3=2. currentTotalTokens = 12-3=9.
	// Evict Low Prio New (3t). Tokens to evict = 2-3=-1. currentTotalTokens = 9-3=6.
	// Kept: Normal Prio (3t), Critical Sys Msg (3t). Total = 6 tokens.

	finalMessages, finalTokens, err := tm.EnforceLimitsWithPriority(ctx, messages, currentTotalTokens)
	require.NoError(t, err)

	// The order of finalMessages is currently the eviction preference order of kept items.
	// It should contain "Critical Sys Msg" and "Normal Prio".
	assert.Len(t, finalMessages, 2)
	assert.Equal(t, 6, finalTokens)

	foundCritical := false
	foundNormal := false
	for _, msg := range finalMessages {
		if msg.Content == "Critical Sys Msg" {
			foundCritical = true
		}
		if msg.Content == "Normal Prio" {
			foundNormal = true
		}
	}
	assert.True(t, foundCritical, "Critical message should be kept")
	assert.True(t, foundNormal, "Normal priority message should be kept")
}
