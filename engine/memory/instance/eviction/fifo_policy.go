package eviction

import (
	"context"

	"github.com/compozy/compozy/engine/llm"
)

// FIFOEvictionPolicy implements a First-In-First-Out eviction strategy
type FIFOEvictionPolicy struct {
	name string
}

// NewFIFOEvictionPolicy creates a new FIFO eviction policy
func NewFIFOEvictionPolicy(_ context.Context) *FIFOEvictionPolicy {
	return &FIFOEvictionPolicy{
		name: "fifo",
	}
}

// SelectMessagesToEvict selects the oldest messages for eviction
func (p *FIFOEvictionPolicy) SelectMessagesToEvict(
	_ context.Context,
	messages []llm.Message,
	targetCount int,
) []llm.Message {
	if len(messages) <= targetCount || targetCount < 0 {
		return nil // No eviction needed
	}
	evictCount := len(messages) - targetCount
	return messages[:evictCount]
}

// GetType returns the policy type
func (p *FIFOEvictionPolicy) GetType() string {
	return p.name
}
