package uc

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/worker"
)

// ReadMemoryResult represents the result of reading memory
type ReadMemoryResult struct {
	Key      string           `json:"key"`
	Messages []map[string]any `json:"messages"`
	Count    int              `json:"count"`
}

// ReadMemory use case for reading memory content
type ReadMemory struct {
	Manager *memory.Manager
	Worker  *worker.Worker
}

type ReadMemoryInput struct {
	MemoryRef string
	Key       string
	Limit     int
	Offset    int
}

type ReadMemoryOutput struct {
	Messages   []llm.Message
	TotalCount int
	HasMore    bool
}

// NewReadMemory creates a new read memory use case
func NewReadMemory(manager *memory.Manager, worker *worker.Worker) *ReadMemory {
	return &ReadMemory{
		Manager: manager,
		Worker:  worker,
	}
}

// Execute reads memory content
func (uc *ReadMemory) Execute(ctx context.Context, input ReadMemoryInput) (*ReadMemoryOutput, error) {
	// Validate inputs
	if err := uc.validateInput(input); err != nil {
		return nil, err
	}

	// Apply pagination defaults
	input = uc.applyPaginationDefaults(input)

	// Get memory manager
	manager, err := uc.getMemoryManager(input)
	if err != nil {
		return nil, err
	}

	// Get memory instance
	instance, err := uc.getMemoryInstance(ctx, manager, input)
	if err != nil {
		return nil, err
	}

	// Check context cancellation before potentially long operation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("operation canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	// Read and paginate messages
	return uc.readAndPaginateMessages(ctx, instance, input)
}

func (uc *ReadMemory) validateInput(input ReadMemoryInput) error {
	if err := ValidateMemoryRef(input.MemoryRef); err != nil {
		return NewErrorContext(err, "read_memory", input.MemoryRef, input.Key)
	}
	if err := ValidateKey(input.Key); err != nil {
		return NewErrorContext(err, "read_memory", input.MemoryRef, input.Key)
	}
	return nil
}

func (uc *ReadMemory) applyPaginationDefaults(input ReadMemoryInput) ReadMemoryInput {
	if input.Limit <= 0 {
		input.Limit = 50 // Default limit
	}
	if input.Limit > 1000 {
		input.Limit = 1000 // Max limit
	}
	if input.Offset < 0 {
		input.Offset = 0
	}
	return input
}

func (uc *ReadMemory) getMemoryManager(input ReadMemoryInput) (*memory.Manager, error) {
	if uc.Manager == nil && uc.Worker != nil {
		uc.Manager = uc.Worker.GetMemoryManager()
	}
	if uc.Manager == nil {
		return nil, NewErrorContext(ErrMemoryManagerNotAvailable, "read_memory", input.MemoryRef, input.Key)
	}
	return uc.Manager, nil
}

func (uc *ReadMemory) getMemoryInstance(
	ctx context.Context,
	manager *memory.Manager,
	input ReadMemoryInput,
) (memcore.Memory, error) {
	// Create a memory reference
	memRef := core.MemoryReference{
		ID:  input.MemoryRef,
		Key: input.Key,
	}

	// Create workflow context for API operations
	workflowContext := map[string]any{
		"api_operation": "read",
		"key":           input.Key,
	}

	// Get memory instance
	instance, err := manager.GetInstance(ctx, memRef, workflowContext)
	if err != nil {
		if errors.Is(err, memcore.ErrMemoryNotFound) {
			return nil, NewErrorContext(ErrMemoryNotFound, "read_memory", input.MemoryRef, input.Key)
		}
		return nil, fmt.Errorf("failed to get memory instance: %w", err)
	}
	return instance, nil
}

func (uc *ReadMemory) readAndPaginateMessages(
	ctx context.Context,
	instance memcore.Memory,
	input ReadMemoryInput,
) (*ReadMemoryOutput, error) {
	// Read memory content
	messages, err := instance.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read memory: %w", err)
	}

	// Handle empty memory
	if len(messages) == 0 {
		return &ReadMemoryOutput{
			Messages:   []llm.Message{},
			TotalCount: 0,
			HasMore:    false,
		}, nil
	}

	// Apply pagination
	totalCount := len(messages)
	start := input.Offset
	end := input.Offset + input.Limit

	// Validate offset
	if start >= totalCount {
		return &ReadMemoryOutput{
			Messages:   []llm.Message{},
			TotalCount: totalCount,
			HasMore:    false,
		}, nil
	}

	// Adjust end if needed
	if end > totalCount {
		end = totalCount
	}

	// Slice messages for pagination
	paginatedMessages := messages[start:end]
	hasMore := end < totalCount

	return &ReadMemoryOutput{
		Messages:   paginatedMessages,
		TotalCount: totalCount,
		HasMore:    hasMore,
	}, nil
}
