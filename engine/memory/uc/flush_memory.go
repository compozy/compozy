package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// FlushMemoryInput represents the input for flushing memory
type FlushMemoryInput struct {
	Force    bool   `json:"force"`
	DryRun   bool   `json:"dry_run"`
	MaxKeys  int    `json:"max_keys,omitempty"`
	Strategy string `json:"strategy,omitempty"`
}

// FlushMemoryResult represents the result of flushing memory
type FlushMemoryResult struct {
	Success          bool   `json:"success"`
	Key              string `json:"key"`
	SummaryGenerated bool   `json:"summary_generated"`
	MessageCount     int    `json:"message_count"`
	TokenCount       int    `json:"token_count"`
	DryRun           bool   `json:"dry_run,omitempty"`
	WouldFlush       bool   `json:"would_flush,omitempty"`
	FlushStrategy    string `json:"flush_strategy,omitempty"`
	Error            string `json:"error,omitempty"`
}

// FlushMemory use case for flushing memory content
type FlushMemory struct {
	manager   *memory.Manager
	memoryRef string
	key       string
	input     *FlushMemoryInput
}

// NewFlushMemory creates a new flush memory use case
func NewFlushMemory(manager *memory.Manager, memoryRef, key string, input *FlushMemoryInput) *FlushMemory {
	if input == nil {
		input = &FlushMemoryInput{}
	}
	return &FlushMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
		input:     input,
	}
}

// Execute flushes memory content
func (uc *FlushMemory) Execute(ctx context.Context) (*FlushMemoryResult, error) {
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
	if err := ValidateFlushInput(uc.input); err != nil {
		return nil, err
	}

	// Create a memory reference
	memRef := core.MemoryReference{
		ID:  uc.memoryRef,
		Key: uc.key,
	}
	// Create workflow context for API operations
	workflowContext := map[string]any{
		"api_operation": "flush",
		"key":           uc.key,
	}
	// Get memory instance
	instance, err := uc.manager.GetInstance(ctx, memRef, workflowContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory instance: %w", err)
	}
	// Check if memory implements FlushableMemory interface
	flushableMem, ok := instance.(memcore.FlushableMemory)
	if !ok {
		return nil, fmt.Errorf("memory instance does not support flush operations")
	}
	// Check if we're in dry-run mode
	if uc.input.DryRun {
		// Get memory health to simulate what would happen
		health, err := instance.GetMemoryHealth(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get memory health for dry run: %w", err)
		}
		return &FlushMemoryResult{
			Success:       true,
			DryRun:        true,
			Key:           uc.key,
			WouldFlush:    health.TokenCount > 0, // Simplified check
			TokenCount:    health.TokenCount,
			MessageCount:  health.MessageCount,
			FlushStrategy: health.FlushStrategy,
		}, nil
	}
	// Perform the flush operation
	result, err := flushableMem.PerformFlush(ctx)
	if err != nil {
		return nil, fmt.Errorf("flush operation failed: %w", err)
	}
	// Convert flush result to output
	flushResult := &FlushMemoryResult{
		Success:          result.Success,
		Key:              uc.key,
		SummaryGenerated: result.SummaryGenerated,
		MessageCount:     result.MessageCount,
		TokenCount:       result.TokenCount,
	}
	// Add error to output if present
	if result.Error != "" {
		flushResult.Error = result.Error
		return flushResult, fmt.Errorf("flush completed with error: %s", result.Error)
	}
	return flushResult, nil
}
