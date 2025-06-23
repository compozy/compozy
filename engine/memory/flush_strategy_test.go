package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRuleBasedSummarizer_SummarizeMessages(t *testing.T) {
	ctx := context.Background()
	mockCounter, _ := NewTiktokenCounter(defaultEncoding)

	t.Run("Should not summarize if not enough messages", func(t *testing.T) {
		summarizer := NewRuleBasedSummarizer(mockCounter, 1, 1) // Keep 1 first, 1 last
		messages := []MessageWithTokens{
			{Message: llm.Message{Content: "Msg1"}, TokenCount: 1},
			{Message: llm.Message{Content: "Msg2"}, TokenCount: 1},
		} // Total 2 messages, less than or equal to KeepFirst (1) + KeepLast (1) = 2

		summaryMsg, kept, summarizedOriginal, err := summarizer.SummarizeMessages(ctx, messages, 100)
		require.NoError(t, err)
		assert.Empty(t, summaryMsg.Content, "Summary message should be empty")
		assert.Equal(t, messages, kept, "All original messages should be kept")
		assert.Nil(t, summarizedOriginal, "No messages should be marked as summarized originals")
	})

	t.Run("Should summarize middle messages", func(t *testing.T) {
		summarizer := NewRuleBasedSummarizer(mockCounter, 1, 1) // Keep 1 first, 1 last
		messages := []MessageWithTokens{
			{Message: llm.Message{Role: "user", Content: "First Message (kept)"}, TokenCount: 4},
			{Message: llm.Message{Role: "assistant", Content: "Middle Message 1 (summarized)"}, TokenCount: 5},
			{Message: llm.Message{Role: "user", Content: "Middle Message 2 (summarized)"}, TokenCount: 5},
			{Message: llm.Message{Role: "assistant", Content: "Last Message (kept)"}, TokenCount: 4},
		} // Total 4 messages. Keep first 1, last 1. Summarize middle 2.

		summaryMsg, kept, summarizedOriginal, err := summarizer.SummarizeMessages(ctx, messages, 100)
		require.NoError(t, err)

		assert.NotEmpty(t, summaryMsg.Content)
		assert.Contains(t, summaryMsg.Content, "Summary of 2 messages:")
		assert.Contains(t, summaryMsg.Content, "[assistant]: Middle Message 1 (summarized)")
		assert.Contains(t, summaryMsg.Content, " ... ")
		assert.Contains(t, summaryMsg.Content, "[user]: Middle Message 2 (summarized)")
		assert.Equal(t, llm.MessageRole("system"), summaryMsg.Role)

		require.Len(t, kept, 2)
		assert.Equal(t, "First Message (kept)", kept[0].Content)
		assert.Equal(t, "Last Message (kept)", kept[1].Content)

		require.Len(t, summarizedOriginal, 2)
		assert.Equal(t, "Middle Message 1 (summarized)", summarizedOriginal[0].Content)
		assert.Equal(t, "Middle Message 2 (summarized)", summarizedOriginal[1].Content)
	})

	t.Run("Should handle summarization with KeepFirst=0", func(t *testing.T) {
		summarizer := NewRuleBasedSummarizer(mockCounter, 0, 1) // Keep 0 first, 1 last
		messages := []MessageWithTokens{
			{Message: llm.Message{Role: "user", Content: "Msg1 (summarized)"}, TokenCount: 3},
			{Message: llm.Message{Role: "assistant", Content: "Msg2 (summarized)"}, TokenCount: 3},
			{Message: llm.Message{Role: "user", Content: "Msg3 (kept)"}, TokenCount: 3},
		}

		summaryMsg, kept, summarizedOriginal, err := summarizer.SummarizeMessages(ctx, messages, 100)
		require.NoError(t, err)
		assert.NotEmpty(t, summaryMsg.Content)
		assert.Contains(t, summaryMsg.Content, "Summary of 2 messages")

		require.Len(t, kept, 1)
		assert.Equal(t, "Msg3 (kept)", kept[0].Content)

		require.Len(t, summarizedOriginal, 2)
		assert.Equal(t, "Msg1 (summarized)", summarizedOriginal[0].Content)
	})

	t.Run("Should truncate summary content if it exceeds targetSummaryTokenCount", func(t *testing.T) {
		summarizer := NewRuleBasedSummarizer(mockCounter, 0, 0) // Summarize all
		messages := []MessageWithTokens{
			// Each "word" is 1 token with defaultEncoding
			{
				Message: llm.Message{
					Role:    "user",
					Content: "one two three four five six seven eight nine ten eleven",
				},
				TokenCount: 11,
			},
		}
		// Target summary tokens: 5. "Summary of 1 messages: [user]: one two three four five..."
		// "Summary of 1 messages: " = 5 tokens
		// "[user]: " = 2 tokens
		// Content "one two three four five" = 5 tokens
		// Total without truncation would be 5 + 2 + content_tokens.
		// The current crude truncation logic might not be precise.

		summaryMsg, _, _, err := summarizer.SummarizeMessages(ctx, messages, 5) // Target 5 tokens for summary
		require.NoError(t, err)
		assert.NotEmpty(t, summaryMsg.Content)

		summaryTokens, _ := mockCounter.CountTokens(ctx, summaryMsg.Content)
		assert.LessOrEqual(t, summaryTokens, 5, "Summary tokens should be less than or equal to target")
		assert.Contains(t, summaryMsg.Content, "...") // Indicates truncation
	})
}

func TestHybridFlushingStrategy_ShouldFlush(t *testing.T) {
	ctx := context.Background()
	mockCounter, _ := NewTiktokenCounter(defaultEncoding)
	memCfg := &Resource{Type: TokenBasedMemory, MaxTokens: 100}
	tm, _ := NewTokenMemoryManager(memCfg, mockCounter, logger.NewForTests())

	flushCfg := &FlushingStrategyConfig{Type: HybridSummaryFlushing, SummarizeThreshold: 0.8}
	hfs, _ := NewHybridFlushingStrategy(flushCfg, NewRuleBasedSummarizer(mockCounter, 1, 1), tm)

	t.Run("Should not flush if below threshold", func(t *testing.T) {
		messages := make([]MessageWithTokens, 79) // 79 tokens, MaxTokens 100, Threshold 0.8 (80 tokens)
		for i := range messages {
			messages[i] = MessageWithTokens{TokenCount: 1}
		}
		assert.False(t, hfs.ShouldFlush(ctx, messages, 79))
	})

	t.Run("Should flush if at or above threshold", func(t *testing.T) {
		messages := make([]MessageWithTokens, 80) // 80 tokens
		for i := range messages {
			messages[i] = MessageWithTokens{TokenCount: 1}
		}
		assert.True(t, hfs.ShouldFlush(ctx, messages, 80))

		messagesMore := make([]MessageWithTokens, 90) // 90 tokens
		for i := range messagesMore {
			messagesMore[i] = MessageWithTokens{TokenCount: 1}
		}
		assert.True(t, hfs.ShouldFlush(ctx, messagesMore, 90))
	})

	t.Run("Should use MaxContextRatio if MaxTokens is zero", func(t *testing.T) {
		memCfgRatio := &Resource{
			Type:             TokenBasedMemory,
			MaxContextRatio:  0.01,
			ModelContextSize: 4096,
		} // 0.01 * 4096 = ~41 tokens
		tmRatio, _ := NewTokenMemoryManager(memCfgRatio, mockCounter, logger.NewForTests())
		hfsRatio, _ := NewHybridFlushingStrategy(flushCfg, NewRuleBasedSummarizer(mockCounter, 1, 1), tmRatio)

		// Threshold is 0.8 * 41 = ~32.8. So flush at 33 tokens.
		// But 0.01 * 4096 = 40.96, which rounds to 40 in integer
		// So 0.8 * 40 = 32.0 exactly
		messages := make([]MessageWithTokens, 31) // 31 tokens
		for i := range messages {
			messages[i] = MessageWithTokens{TokenCount: 1}
		}
		assert.False(t, hfsRatio.ShouldFlush(ctx, messages, 31))

		messagesOver := make([]MessageWithTokens, 32) // 32 tokens
		for i := range messagesOver {
			messagesOver[i] = MessageWithTokens{TokenCount: 1}
		}
		assert.True(t, hfsRatio.ShouldFlush(ctx, messagesOver, 32))
	})
}

func TestHybridFlushingStrategy_ShouldFlushByCount(t *testing.T) {
	ctx := context.Background()
	mockCounter, _ := NewTiktokenCounter(defaultEncoding)
	memCfg := &Resource{Type: TokenBasedMemory, MaxTokens: 100}
	tm, _ := NewTokenMemoryManager(memCfg, mockCounter, logger.NewForTests())

	flushCfg := &FlushingStrategyConfig{Type: HybridSummaryFlushing, SummarizeThreshold: 0.8}
	hfs, _ := NewHybridFlushingStrategy(flushCfg, NewRuleBasedSummarizer(mockCounter, 1, 1), tm)

	t.Run("Should not flush if below threshold", func(t *testing.T) {
		// 79 tokens, MaxTokens 100, Threshold 0.8 (80 tokens)
		assert.False(t, hfs.ShouldFlushByCount(ctx, 79, 79))
	})

	t.Run("Should flush if at or above threshold", func(t *testing.T) {
		assert.True(t, hfs.ShouldFlushByCount(ctx, 80, 80))
		assert.True(t, hfs.ShouldFlushByCount(ctx, 90, 90))
	})

	t.Run("Should use MaxContextRatio if MaxTokens is zero", func(t *testing.T) {
		memCfgRatio := &Resource{
			Type:             TokenBasedMemory,
			MaxContextRatio:  0.01,
			ModelContextSize: 4096,
		} // 0.01 * 4096 = ~41 tokens
		tmRatio, _ := NewTokenMemoryManager(memCfgRatio, mockCounter, logger.NewForTests())
		hfsRatio, _ := NewHybridFlushingStrategy(flushCfg, NewRuleBasedSummarizer(mockCounter, 1, 1), tmRatio)

		// Threshold is 0.8 * 40 = 32.0 exactly
		assert.False(t, hfsRatio.ShouldFlushByCount(ctx, 31, 31))
		assert.True(t, hfsRatio.ShouldFlushByCount(ctx, 32, 32))
	})

	t.Run("Should return false if config is nil", func(t *testing.T) {
		hfsNilCfg := &HybridFlushingStrategy{
			config:       nil,
			summarizer:   NewRuleBasedSummarizer(mockCounter, 1, 1),
			tokenManager: tm,
		}
		assert.False(t, hfsNilCfg.ShouldFlushByCount(ctx, 100, 100))
	})

	t.Run("Should behave identically to ShouldFlush", func(t *testing.T) {
		// Test that both methods return the same results for various inputs
		testCases := []struct {
			tokenCount   int
			messageCount int
		}{
			{tokenCount: 50, messageCount: 10},
			{tokenCount: 79, messageCount: 20},
			{tokenCount: 80, messageCount: 30},
			{tokenCount: 100, messageCount: 40},
		}

		for _, tc := range testCases {
			// Create dummy messages for ShouldFlush
			messages := make([]MessageWithTokens, tc.messageCount)

			shouldFlushResult := hfs.ShouldFlush(ctx, messages, tc.tokenCount)
			shouldFlushByCountResult := hfs.ShouldFlushByCount(ctx, tc.messageCount, tc.tokenCount)

			assert.Equal(t, shouldFlushResult, shouldFlushByCountResult,
				"ShouldFlush and ShouldFlushByCount should return same result for tokens=%d, messages=%d",
				tc.tokenCount, tc.messageCount)
		}
	})
}

func TestHybridFlushingStrategy_FlushMessages(t *testing.T) {
	ctx := context.Background()
	mockCounter, _ := NewTiktokenCounter(defaultEncoding)
	memCfg := &Resource{Type: TokenBasedMemory, MaxTokens: 1000} // High MaxTokens to not interfere
	tm, _ := NewTokenMemoryManager(memCfg, mockCounter, logger.NewForTests())

	flushCfg := &FlushingStrategyConfig{
		Type:                   HybridSummaryFlushing,
		SummarizeOldestPercent: 0.5, // This isn't directly used by RuleBasedSummarizer's current interface
		SummaryTokens:          20,  // Target for the summary message itself
	}
	// RuleBasedSummarizer keeps 1 first, 1 last.
	summarizer := NewRuleBasedSummarizer(mockCounter, 1, 1)
	hfs, _ := NewHybridFlushingStrategy(flushCfg, summarizer, tm)

	messages := []MessageWithTokens{
		{Message: llm.Message{Role: "user", Content: "First (kept)"}, TokenCount: 3},
		{Message: llm.Message{Role: "system", Content: "Mid1 (summarized)"}, TokenCount: 3},
		{Message: llm.Message{Role: "user", Content: "Mid2 (summarized)"}, TokenCount: 3},
		{Message: llm.Message{Role: "assistant", Content: "Mid3 (summarized)"}, TokenCount: 3},
		{Message: llm.Message{Role: "user", Content: "Last (kept)"}, TokenCount: 3},
	} // 5 messages, 15 tokens total.

	newMessages, newTokens, summaryGenerated, err := hfs.FlushMessages(ctx, messages)
	require.NoError(t, err)

	assert.True(t, summaryGenerated)
	// Expected: Summary msg + First (kept) + Last (kept) = 3 messages
	assert.Len(t, newMessages, 3)

	// Check summary message is first
	assert.Equal(t, llm.MessageRole("system"), newMessages[0].Role)            // Summary role
	assert.Contains(t, newMessages[0].Content, "Summary of 3 messages")        // Mid1, Mid2, Mid3 summarized
	assert.LessOrEqual(t, newMessages[0].TokenCount, flushCfg.SummaryTokens+5) // +5 for some buffer due to content

	// Check kept messages
	assert.Equal(t, "First (kept)", newMessages[1].Content)
	assert.Equal(t, "Last (kept)", newMessages[2].Content)

	expectedTokensAfterFlush := newMessages[0].TokenCount + newMessages[1].TokenCount + newMessages[2].TokenCount
	assert.Equal(t, expectedTokensAfterFlush, newTokens)

	t.Run("Should do nothing if no summary generated", func(t *testing.T) {
		shortMessages := []MessageWithTokens{
			{Message: llm.Message{Role: "user", Content: "First (kept)"}, TokenCount: 3},
			{Message: llm.Message{Role: "user", Content: "Last (kept)"}, TokenCount: 3},
		} // Too short to trigger summarization by RuleBasedSummarizer(1,1)

		resultMsgs, resultTokens, sumGenerated, rErr := hfs.FlushMessages(ctx, shortMessages)
		require.NoError(t, rErr)
		assert.False(t, sumGenerated)
		assert.Equal(t, shortMessages, resultMsgs)
		assert.Equal(t, 6, resultTokens)
	})

	t.Run("Should handle SimpleFIFOFlushing type (no-op in this func)", func(t *testing.T) {
		fifoFlushCfg := &FlushingStrategyConfig{Type: SimpleFIFOFlushing}
		hfsFifo, _ := NewHybridFlushingStrategy(fifoFlushCfg, nil, tm) // No summarizer for FIFO

		resultMsgs, resultTokens, sumGenerated, rErr := hfsFifo.FlushMessages(ctx, messages)
		require.NoError(t, rErr) // No error, but no operation done by this specific function
		assert.False(t, sumGenerated)
		assert.Equal(t, messages, resultMsgs) // Returns original messages
		assert.Equal(t, 15, resultTokens)     // Original token count
	})
}

func TestCalculateTotalTokens(t *testing.T) {
	messages := []MessageWithTokens{
		{TokenCount: 10},
		{TokenCount: 5},
		{TokenCount: 3},
	}
	assert.Equal(t, 18, calculateTotalTokens(messages))
	assert.Equal(t, 0, calculateTotalTokens([]MessageWithTokens{}))
	assert.Equal(t, 0, calculateTotalTokens(nil))
}

func TestRuleBasedSummarizer_TokenFallbackRatio(t *testing.T) {
	ctx := context.Background()

	// Create a mock token counter that always fails
	mockCounter := &MockTokenCounter{}
	mockCounter.On("CountTokens", mock.Anything, mock.AnythingOfType("string")).
		Return(0, errors.New("token counting failed"))

	t.Run("Should use default fallback ratio of 3", func(t *testing.T) {
		summarizer := NewRuleBasedSummarizer(mockCounter, 1, 1)
		assert.Equal(t, 3, summarizer.TokenFallbackRatio)

		// Test with 60 character content should return 20 tokens (60/3)
		content := "This is a test content with exactly sixty characters long!!!"
		assert.Equal(t, 60, len(content))
		tokenCount := summarizer.getTokenCount(ctx, content)
		assert.Equal(t, 20, tokenCount)
	})

	t.Run("Should use custom fallback ratio", func(t *testing.T) {
		customRatio := 5
		summarizer := NewRuleBasedSummarizerWithOptions(mockCounter, 1, 1, customRatio)
		assert.Equal(t, customRatio, summarizer.TokenFallbackRatio)

		// Test with 50 character content should return 10 tokens (50/5)
		content := "This is test content with fifty characters ok!!!!!"
		assert.Equal(t, 50, len(content))
		tokenCount := summarizer.getTokenCount(ctx, content)
		assert.Equal(t, 10, tokenCount)
	})

	t.Run("Should use default when invalid ratio provided", func(t *testing.T) {
		summarizer := NewRuleBasedSummarizerWithOptions(mockCounter, 1, 1, 0) // Invalid ratio
		assert.Equal(t, 3, summarizer.TokenFallbackRatio)                     // Should default to 3

		summarizer2 := NewRuleBasedSummarizerWithOptions(mockCounter, 1, 1, -1) // Invalid ratio
		assert.Equal(t, 3, summarizer2.TokenFallbackRatio)                      // Should default to 3
	})
}
