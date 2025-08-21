package strategies

import (
	"context"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
)

// Constants for FIFO strategy configuration
const (
	defaultThresholdPercent = 0.8  // Default threshold for triggering flush
	targetCapacityPercent   = 0.5  // Target capacity after flush (50%)
	minFlushPercent         = 0.05 // Minimum percentage of messages to flush (5%)
	maxFlushPercent         = 0.75 // Maximum percentage of messages to flush (75%)
	defaultRemovalRatio     = 0.25 // Default removal ratio when no max configured (25%)
)

// FIFOStrategy implements a simple First-In-First-Out flushing strategy
type FIFOStrategy struct {
	thresholdPercent float64           // Percentage of max capacity that triggers flush
	tokenCounter     core.TokenCounter // Token counter for accurate counting
}

// NewFIFOStrategy creates a new FIFO flushing strategy
func NewFIFOStrategy(thresholdPercent float64) *FIFOStrategy {
	if thresholdPercent <= 0 || thresholdPercent > 1 {
		thresholdPercent = defaultThresholdPercent
	}
	// Use a simple token counter adapter for backward compatibility
	return &FIFOStrategy{
		thresholdPercent: thresholdPercent,
		tokenCounter:     NewSimpleTokenCounterAdapter(),
	}
}

// NewFIFOStrategyWithTokenCounter creates a new FIFO flushing strategy with custom token counter
func NewFIFOStrategyWithTokenCounter(thresholdPercent float64, tokenCounter core.TokenCounter) *FIFOStrategy {
	if thresholdPercent <= 0 || thresholdPercent > 1 {
		thresholdPercent = defaultThresholdPercent
	}
	if tokenCounter == nil {
		tokenCounter = NewSimpleTokenCounterAdapter()
	}
	return &FIFOStrategy{
		thresholdPercent: thresholdPercent,
		tokenCounter:     tokenCounter,
	}
}

// ShouldFlush determines if a flush should be triggered based on current state
func (s *FIFOStrategy) ShouldFlush(tokenCount, messageCount int, config *core.Resource) bool {
	// Check token-based threshold
	if config.Type == core.TokenBasedMemory && config.MaxTokens > 0 {
		threshold := float64(config.MaxTokens) * s.thresholdPercent
		return float64(tokenCount) >= threshold
	}

	// Check message-based threshold
	if config.Type == core.MessageCountBasedMemory && config.MaxMessages > 0 {
		threshold := float64(config.MaxMessages) * s.thresholdPercent
		return float64(messageCount) >= threshold
	}

	// For buffer memory, check both limits
	if config.Type == core.BufferMemory {
		if config.MaxTokens > 0 {
			tokenThreshold := float64(config.MaxTokens) * s.thresholdPercent
			if float64(tokenCount) >= tokenThreshold {
				return true
			}
		}
		if config.MaxMessages > 0 {
			messageThreshold := float64(config.MaxMessages) * s.thresholdPercent
			if float64(messageCount) >= messageThreshold {
				return true
			}
		}
	}

	return false
}

// PerformFlush executes the flush operation
func (s *FIFOStrategy) PerformFlush(
	ctx context.Context,
	messages []llm.Message,
	config *core.Resource,
) (*core.FlushMemoryActivityOutput, error) {
	if len(messages) == 0 {
		return &core.FlushMemoryActivityOutput{
			Success:          true,
			SummaryGenerated: false,
			MessageCount:     0,
			TokenCount:       0,
		}, nil
	}

	// Calculate how many messages to remove
	messagesToRemove := s.calculateMessagesToRemove(len(messages), config)
	if messagesToRemove == 0 {
		return &core.FlushMemoryActivityOutput{
			Success:          true,
			SummaryGenerated: false,
			MessageCount:     len(messages),
			TokenCount:       0,
		}, nil
	}

	// Optimize token counting - only count tokens for messages we need
	tokensFlushed := 0
	remainingTokens := 0

	// Count tokens for messages being removed
	for i := range messagesToRemove {
		count, err := s.tokenCounter.CountTokens(ctx, messages[i].Content)
		if err != nil {
			// Fall back to estimation on error
			count = len(messages[i].Content) / 4
		}
		tokensFlushed += count
	}

	// Count tokens for remaining messages
	for i := messagesToRemove; i < len(messages); i++ {
		count, err := s.tokenCounter.CountTokens(ctx, messages[i].Content)
		if err != nil {
			// Fall back to estimation on error
			count = len(messages[i].Content) / 4
		}
		remainingTokens += count
	}

	return &core.FlushMemoryActivityOutput{
		Success:          true,
		SummaryGenerated: false,
		MessageCount:     len(messages) - messagesToRemove,
		TokenCount:       remainingTokens, // Fixed: returns remaining tokens, not flushed tokens
	}, nil
}

// GetType returns the strategy type
func (s *FIFOStrategy) GetType() core.FlushingStrategyType {
	return core.SimpleFIFOFlushing
}

// calculateMessagesToRemove determines how many messages to remove
func (s *FIFOStrategy) calculateMessagesToRemove(currentCount int, config *core.Resource) int {
	// Remove enough messages to get back to target capacity
	if config.MaxMessages > 0 {
		targetCount := int(float64(config.MaxMessages) * targetCapacityPercent)
		if currentCount > targetCount {
			return currentCount - targetCount
		}
		// If already under target, don't remove any
		return 0
	}

	// Default: remove default percentage of messages
	return int(float64(currentCount) * defaultRemovalRatio)
}

// GetMinMaxToFlush returns the min/max number of messages to flush for this strategy
func (s *FIFOStrategy) GetMinMaxToFlush(
	_ context.Context, // ctx - unused
	totalMsgs int,
	currentTokens int,
	maxTokens int,
) (minFlush, maxFlush int) {
	// Validate inputs
	if totalMsgs <= 0 || maxTokens <= 0 || currentTokens < 0 {
		return 0, 0
	}

	// Calculate minimum flush to avoid micro-flushes
	minFlush = int(float64(totalMsgs) * minFlushPercent)
	if minFlush < 1 && totalMsgs > 0 {
		minFlush = 1
	}

	// Calculate maximum flush based on target capacity
	if currentTokens > maxTokens {
		// Need to flush to get under limit
		targetTokens := int(float64(maxTokens) * targetCapacityPercent)
		tokensToFlush := currentTokens - targetTokens

		// Estimate messages to flush based on average token count
		avgTokensPerMsg := currentTokens / totalMsgs
		if avgTokensPerMsg > 0 {
			maxFlush = tokensToFlush / avgTokensPerMsg
		} else {
			maxFlush = int(float64(totalMsgs) * defaultRemovalRatio)
		}

		// Ensure we don't flush more than max allowed percentage
		maxAllowed := int(float64(totalMsgs) * maxFlushPercent)
		if maxFlush > maxAllowed {
			maxFlush = maxAllowed
		}
	} else {
		// Not over limit, but prepare for a moderate flush
		maxFlush = int(float64(totalMsgs) * defaultRemovalRatio)
	}

	// Ensure min <= max
	if minFlush > maxFlush {
		minFlush = maxFlush
	}

	return minFlush, maxFlush
}
