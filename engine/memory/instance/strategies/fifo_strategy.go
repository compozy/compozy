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
	if config.Type == core.TokenBasedMemory && config.MaxTokens > 0 {
		threshold := float64(config.MaxTokens) * s.thresholdPercent
		return float64(tokenCount) >= threshold
	}
	if config.Type == core.MessageCountBasedMemory && config.MaxMessages > 0 {
		threshold := float64(config.MaxMessages) * s.thresholdPercent
		return float64(messageCount) >= threshold
	}
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
	messagesToRemove := s.calculateMessagesToRemove(len(messages), config)
	if messagesToRemove == 0 {
		return &core.FlushMemoryActivityOutput{
			Success:          true,
			SummaryGenerated: false,
			MessageCount:     len(messages),
			TokenCount:       0,
		}, nil
	}
	remainingTokens := s.countTokens(ctx, messages[messagesToRemove:])
	return &core.FlushMemoryActivityOutput{
		Success:          true,
		SummaryGenerated: false,
		MessageCount:     len(messages) - messagesToRemove,
		TokenCount:       remainingTokens, // Fixed: returns remaining tokens, not flushed tokens
	}, nil
}

func (s *FIFOStrategy) countTokens(ctx context.Context, messages []llm.Message) int {
	if len(messages) == 0 {
		return 0
	}
	total := 0
	for i := range messages {
		count, err := s.tokenCounter.CountTokens(ctx, messages[i].Content)
		if err != nil {
			count = len(messages[i].Content) / 4
		}
		total += count
	}
	return total
}

// GetType returns the strategy type
func (s *FIFOStrategy) GetType() core.FlushingStrategyType {
	return core.SimpleFIFOFlushing
}

// calculateMessagesToRemove determines how many messages to remove
func (s *FIFOStrategy) calculateMessagesToRemove(currentCount int, config *core.Resource) int {
	if config.MaxMessages > 0 {
		targetCount := int(float64(config.MaxMessages) * targetCapacityPercent)
		if currentCount > targetCount {
			return currentCount - targetCount
		}
		return 0
	}
	return int(float64(currentCount) * defaultRemovalRatio)
}

// GetMinMaxToFlush returns the min/max number of messages to flush for this strategy
func (s *FIFOStrategy) GetMinMaxToFlush(
	_ context.Context, // ctx - unused
	totalMsgs int,
	currentTokens int,
	maxTokens int,
) (minFlush, maxFlush int) {
	if totalMsgs <= 0 || maxTokens <= 0 || currentTokens < 0 {
		return 0, 0
	}
	minFlush = int(float64(totalMsgs) * minFlushPercent)
	if minFlush < 1 && totalMsgs > 0 {
		minFlush = 1
	}
	if currentTokens > maxTokens {
		targetTokens := int(float64(maxTokens) * targetCapacityPercent)
		tokensToFlush := currentTokens - targetTokens

		avgTokensPerMsg := currentTokens / totalMsgs
		if avgTokensPerMsg > 0 {
			maxFlush = tokensToFlush / avgTokensPerMsg
		} else {
			maxFlush = int(float64(totalMsgs) * defaultRemovalRatio)
		}

		maxAllowed := int(float64(totalMsgs) * maxFlushPercent)
		if maxFlush > maxAllowed {
			maxFlush = maxAllowed
		}
	} else {
		maxFlush = int(float64(totalMsgs) * defaultRemovalRatio)
	}
	if minFlush > maxFlush {
		minFlush = maxFlush
	}
	return minFlush, maxFlush
}
