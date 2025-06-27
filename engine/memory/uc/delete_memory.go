package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/memory"
)

// DeleteMemoryResult represents the result of deleting memory
type DeleteMemoryResult struct {
	Success bool   `json:"success"`
	Key     string `json:"key"`
}

// DeleteMemory use case for deleting memory content
type DeleteMemory struct {
	manager   *memory.Manager
	memoryRef string
	key       string
}

// NewDeleteMemory creates a new delete memory use case
func NewDeleteMemory(manager *memory.Manager, memoryRef, key string) *DeleteMemory {
	return &DeleteMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
	}
}

// Execute deletes memory content
func (uc *DeleteMemory) Execute(ctx context.Context) (*DeleteMemoryResult, error) {
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
		"api_operation": "delete",
		"key":           uc.key,
	}
	// Get memory instance
	instance, err := uc.manager.GetInstance(ctx, memRef, workflowContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory instance: %w", err)
	}
	// Clear all messages (delete operation)
	if err := instance.Clear(ctx); err != nil {
		return nil, fmt.Errorf("failed to delete memory: %w", err)
	}
	return &DeleteMemoryResult{
		Success: true,
		Key:     uc.key,
	}, nil
}
