package strategies

import (
	"context"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/core"
)

// FIFOStrategy implements a simple First-In-First-Out flushing strategy
type FIFOStrategy struct {
	thresholdPercent float64 // Percentage of max capacity that triggers flush
}

// NewFIFOStrategy creates a new FIFO flushing strategy
func NewFIFOStrategy(thresholdPercent float64) *FIFOStrategy {
	if thresholdPercent <= 0 || thresholdPercent > 1 {
		thresholdPercent = 0.8 // Default to 80%
	}
	return &FIFOStrategy{
		thresholdPercent: thresholdPercent,
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
	_ context.Context, // ctx - unused
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

	// Calculate tokens in messages to be removed
	tokensFlushed := 0
	for i := 0; i < messagesToRemove; i++ {
		// Rough estimate - would use actual token counter in real implementation
		tokensFlushed += len(messages[i].Content) / 4
	}

	return &core.FlushMemoryActivityOutput{
		Success:          true,
		SummaryGenerated: false,
		MessageCount:     len(messages) - messagesToRemove,
		TokenCount:       tokensFlushed, // This should be total remaining tokens
	}, nil
}

// GetType returns the strategy type
func (s *FIFOStrategy) GetType() core.FlushingStrategyType {
	return core.SimpleFIFOFlushing
}

// calculateMessagesToRemove determines how many messages to remove
func (s *FIFOStrategy) calculateMessagesToRemove(currentCount int, config *core.Resource) int {
	// Remove enough messages to get back to 50% capacity
	targetPercent := 0.5

	if config.MaxMessages > 0 {
		targetCount := int(float64(config.MaxMessages) * targetPercent)
		if currentCount > targetCount {
			return currentCount - targetCount
		}
	}

	// Default: remove 25% of messages
	return currentCount / 4
}

// GetMinMaxToFlush returns the min/max number of messages to flush for this strategy
func (s *FIFOStrategy) GetMinMaxToFlush(
	_ context.Context, // ctx - unused
	_ int, // totalMsgs - unused
	_ int, // currentTokens - unused
	_ int, // maxTokens - unused
) (minFlush, maxFlush int) {
	// Implementation of GetMinMaxToFlush method
	return 0, 0 // Placeholder return, actual implementation needed
}
