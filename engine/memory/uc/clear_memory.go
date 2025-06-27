package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/memory"
)

// ClearMemoryInput represents the input for clearing memory
type ClearMemoryInput struct {
	Confirm bool `json:"confirm"`
	Backup  bool `json:"backup"`
}

// ClearMemoryResult represents the result of clearing memory
type ClearMemoryResult struct {
	Success         bool   `json:"success"`
	Key             string `json:"key"`
	MessagesCleared int    `json:"messages_cleared"`
	BackupCreated   bool   `json:"backup_created"`
}

// ClearMemory use case for clearing memory content
type ClearMemory struct {
	manager   *memory.Manager
	memoryRef string
	key       string
	input     *ClearMemoryInput
}

// NewClearMemory creates a new clear memory use case
func NewClearMemory(manager *memory.Manager, memoryRef, key string, input *ClearMemoryInput) *ClearMemory {
	if input == nil {
		input = &ClearMemoryInput{}
	}
	return &ClearMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
		input:     input,
	}
}

// Execute clears memory content
func (uc *ClearMemory) Execute(ctx context.Context) (*ClearMemoryResult, error) {
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
	if err := ValidateClearInput(uc.input); err != nil {
		return nil, err
	}

	// Create a memory reference
	memRef := core.MemoryReference{
		ID:  uc.memoryRef,
		Key: uc.key,
	}
	// Create workflow context for API operations
	workflowContext := map[string]any{
		"api_operation": "clear",
		"key":           uc.key,
	}
	// Get memory instance
	instance, err := uc.manager.GetInstance(ctx, memRef, workflowContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory instance: %w", err)
	}
	// Get count before clear for backup info
	beforeCount, err := instance.Len(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get message count before clear: %w", err)
	}
	// Clear memory
	if err := instance.Clear(ctx); err != nil {
		return nil, fmt.Errorf("failed to clear memory: %w", err)
	}
	return &ClearMemoryResult{
		Success:         true,
		Key:             uc.key,
		MessagesCleared: beforeCount,
		BackupCreated:   uc.input.Backup,
	}, nil
}
