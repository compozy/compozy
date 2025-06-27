package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/memory"
)

// HealthMemoryInput represents the input for checking memory health
type HealthMemoryInput struct {
	IncludeStats bool `json:"include_stats"`
}

// HealthMemoryResult represents the result of checking memory health
type HealthMemoryResult struct {
	Healthy       bool   `json:"healthy"`
	Key           string `json:"key"`
	TokenCount    int    `json:"token_count"`
	MessageCount  int    `json:"message_count"`
	FlushStrategy string `json:"flush_strategy"`
	LastFlush     string `json:"last_flush,omitempty"`
	CurrentTokens int    `json:"current_tokens,omitempty"`
}

// HealthMemory use case for checking memory health
type HealthMemory struct {
	manager   *memory.Manager
	memoryRef string
	key       string
	input     *HealthMemoryInput
}

// NewHealthMemory creates a new health memory use case
func NewHealthMemory(manager *memory.Manager, memoryRef, key string, input *HealthMemoryInput) *HealthMemory {
	if input == nil {
		input = &HealthMemoryInput{}
	}
	return &HealthMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
		input:     input,
	}
}

// Execute checks memory health
func (uc *HealthMemory) Execute(ctx context.Context) (*HealthMemoryResult, error) {
	if uc.manager == nil {
		return nil, ErrMemoryManagerNotAvailable
	}

	// Validate inputs
	if err := ValidateMemoryRef(uc.memoryRef); err != nil {
		return nil, err
	}
	if err := ValidateKey(uc.key); err != nil {
		return nil, err
	}

	// Create a memory reference
	memRef := core.MemoryReference{
		ID:  uc.memoryRef,
		Key: uc.key,
	}
	// Create workflow context for API operations
	workflowContext := map[string]any{
		"api_operation": "health",
		"key":           uc.key,
	}
	// Get memory instance
	instance, err := uc.manager.GetInstance(ctx, memRef, workflowContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory instance: %w", err)
	}
	// Get memory health
	health, err := instance.GetMemoryHealth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory health: %w", err)
	}
	result := &HealthMemoryResult{
		Healthy:       true,
		Key:           uc.key,
		TokenCount:    health.TokenCount,
		MessageCount:  health.MessageCount,
		FlushStrategy: health.FlushStrategy,
	}
	if health.LastFlush != nil {
		result.LastFlush = health.LastFlush.Format("2006-01-02T15:04:05Z07:00")
	}
	if uc.input.IncludeStats {
		tokenCount, err := instance.GetTokenCount(ctx)
		if err == nil {
			result.CurrentTokens = tokenCount
		}
	}
	return result, nil
}
