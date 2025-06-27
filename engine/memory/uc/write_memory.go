package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
)

// WriteMemoryInput represents the input for writing to memory
type WriteMemoryInput struct {
	Messages []map[string]any `json:"messages"`
}

// WriteMemoryResult represents the result of writing to memory
type WriteMemoryResult struct {
	Success bool   `json:"success"`
	Count   int    `json:"count"`
	Key     string `json:"key"`
}

// WriteMemory use case for writing/replacing memory content
type WriteMemory struct {
	manager   *memory.Manager
	memoryRef string
	key       string
	input     *WriteMemoryInput
}

// NewWriteMemory creates a new write memory use case
func NewWriteMemory(manager *memory.Manager, memoryRef, key string, input *WriteMemoryInput) *WriteMemory {
	return &WriteMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
		input:     input,
	}
}

// Execute writes/replaces memory content
func (uc *WriteMemory) Execute(ctx context.Context) (*WriteMemoryResult, error) {
	// Validate inputs
	if err := uc.validate(); err != nil {
		return nil, err
	}

	// Get memory instance
	instance, err := uc.getMemoryInstance(ctx)
	if err != nil {
		return nil, err
	}

	// Convert messages
	messages, err := uc.convertToLLMMessages(uc.input.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	// Perform write operation
	if err := uc.performWrite(ctx, instance, messages); err != nil {
		return nil, err
	}

	return &WriteMemoryResult{
		Success: true,
		Count:   len(messages),
		Key:     uc.key,
	}, nil
}

// validate performs input validation
func (uc *WriteMemory) validate() error {
	if uc.manager == nil {
		return ErrMemoryManagerNotAvailable
	}
	if err := ValidateMemoryRef(uc.memoryRef); err != nil {
		return err
	}
	if err := ValidateKey(uc.key); err != nil {
		return err
	}
	if uc.input == nil {
		return ErrInvalidPayload
	}
	if err := ValidateRawMessages(uc.input.Messages); err != nil {
		return err
	}
	return nil
}

// getMemoryInstance retrieves the memory instance
func (uc *WriteMemory) getMemoryInstance(ctx context.Context) (memcore.Memory, error) {
	memRef := core.MemoryReference{
		ID:  uc.memoryRef,
		Key: uc.key,
	}
	workflowContext := map[string]any{
		"api_operation": "write",
		"key":           uc.key,
	}
	instance, err := uc.manager.GetInstance(ctx, memRef, workflowContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory instance: %w", err)
	}
	return instance, nil
}

// performWrite performs the actual write operation
func (uc *WriteMemory) performWrite(ctx context.Context, instance memcore.Memory, messages []llm.Message) error {
	// Check if instance supports atomic operations
	if atomicInstance, ok := instance.(memcore.AtomicOperations); ok {
		return uc.performAtomicWrite(ctx, atomicInstance, messages)
	}
	return uc.performNonAtomicWrite(ctx, instance, messages)
}

// performAtomicWrite performs atomic write operation
func (uc *WriteMemory) performAtomicWrite(
	ctx context.Context,
	instance memcore.AtomicOperations,
	messages []llm.Message,
) error {
	// Calculate total tokens (approximate - actual token count would require tokenizer)
	totalTokens := 0
	for _, msg := range messages {
		// Rough estimate: ~4 characters per token
		totalTokens += len(msg.Content) / 4
	}

	if err := instance.ReplaceMessagesWithMetadata(ctx, uc.key, messages, totalTokens); err != nil {
		return fmt.Errorf("failed to replace messages atomically: %w", err)
	}
	return nil
}

// performNonAtomicWrite performs non-atomic write with best-effort rollback
func (uc *WriteMemory) performNonAtomicWrite(
	ctx context.Context,
	instance memcore.Memory,
	messages []llm.Message,
) error {
	// Read current messages to enable rollback
	currentMessages, readErr := instance.Read(ctx)

	// Clear existing content
	if err := instance.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear memory: %w", err)
	}

	// Try to write all messages
	for i, msg := range messages {
		if err := instance.Append(ctx, msg); err != nil {
			// Try to restore original messages on failure
			if readErr == nil {
				uc.restoreMessages(ctx, instance, currentMessages)
			}
			return fmt.Errorf("failed to write message %d: %w", i, err)
		}
	}

	return nil
}

// restoreMessages attempts to restore messages after a failed write
func (uc *WriteMemory) restoreMessages(ctx context.Context, instance memcore.Memory, messages []llm.Message) {
	// Best effort restore - we don't return errors from here
	// Log errors if available but continue trying to restore
	for _, msg := range messages {
		if err := instance.Append(ctx, msg); err != nil {
			// In production, we would log this error
			// logger.Error("failed to restore message during rollback", "error", err)
			_ = err // Acknowledge error for linter
		}
	}
}

// convertToLLMMessages converts generic message format to LLM messages
func (uc *WriteMemory) convertToLLMMessages(messages []map[string]any) ([]llm.Message, error) {
	result := make([]llm.Message, 0, len(messages))
	for i, msg := range messages {
		role, ok := msg["role"].(string)
		if !ok || role == "" {
			role = "user" // Default to user role
		}
		content, ok := msg["content"].(string)
		if !ok || content == "" {
			return nil, fmt.Errorf("message[%d] content is required", i)
		}
		// Validate role
		if err := ValidateMessageRole(role); err != nil {
			return nil, fmt.Errorf("message[%d]: %w", i, err)
		}
		result = append(result, llm.Message{
			Role:    llm.MessageRole(role),
			Content: content,
		})
	}
	return result, nil
}
