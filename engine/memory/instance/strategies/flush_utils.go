package strategies

import "github.com/compozy/compozy/engine/memory/core"

// FlushDecisionEngine contains common logic for determining when to flush memory
type FlushDecisionEngine struct {
	thresholdPercent float64
}

// NewFlushDecisionEngine creates a new flush decision engine with the given threshold
func NewFlushDecisionEngine(thresholdPercent float64) *FlushDecisionEngine {
	if thresholdPercent <= 0 || thresholdPercent > 1 {
		thresholdPercent = 0.8 // Default to 80%
	}
	return &FlushDecisionEngine{
		thresholdPercent: thresholdPercent,
	}
}

// ShouldFlush determines if a flush should be triggered based on current state and thresholds
func (fde *FlushDecisionEngine) ShouldFlush(tokenCount, messageCount int, config *core.Resource) bool {
	switch config.Type {
	case core.TokenBasedMemory:
		return fde.shouldFlushTokenBased(tokenCount, config)
	case core.MessageCountBasedMemory:
		return fde.shouldFlushMessageBased(messageCount, config)
	case core.BufferMemory:
		return fde.shouldFlushBuffer(tokenCount, messageCount, config)
	default:
		return false
	}
}

// shouldFlushTokenBased checks token-based threshold
func (fde *FlushDecisionEngine) shouldFlushTokenBased(tokenCount int, config *core.Resource) bool {
	if config.MaxTokens <= 0 {
		return false
	}
	threshold := float64(config.MaxTokens) * fde.thresholdPercent
	return float64(tokenCount) >= threshold
}

// shouldFlushMessageBased checks message-based threshold
func (fde *FlushDecisionEngine) shouldFlushMessageBased(messageCount int, config *core.Resource) bool {
	if config.MaxMessages <= 0 {
		return false
	}
	threshold := float64(config.MaxMessages) * fde.thresholdPercent
	return float64(messageCount) >= threshold
}

// shouldFlushBuffer checks both limits for buffer memory
func (fde *FlushDecisionEngine) shouldFlushBuffer(tokenCount, messageCount int, config *core.Resource) bool {
	if config.MaxTokens > 0 {
		tokenThreshold := float64(config.MaxTokens) * fde.thresholdPercent
		if float64(tokenCount) >= tokenThreshold {
			return true
		}
	}
	if config.MaxMessages > 0 {
		messageThreshold := float64(config.MaxMessages) * fde.thresholdPercent
		if float64(messageCount) >= messageThreshold {
			return true
		}
	}
	return false
}

// GetThreshold returns the current threshold percentage
func (fde *FlushDecisionEngine) GetThreshold() float64 {
	return fde.thresholdPercent
}

// SetThreshold updates the threshold percentage
func (fde *FlushDecisionEngine) SetThreshold(threshold float64) {
	if threshold > 0 && threshold <= 1 {
		fde.thresholdPercent = threshold
	}
}
