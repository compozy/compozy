package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory"
)

// AppendMemoryInput represents the input for appending to memory
type AppendMemoryInput struct {
	Messages []map[string]any `json:"messages"`
}

// AppendMemoryResult represents the result of appending to memory
type AppendMemoryResult struct {
	Success    bool   `json:"success"`
	Appended   int    `json:"appended"`
	TotalCount int    `json:"total_count"`
	Key        string `json:"key"`
}

// AppendMemory use case for appending to memory content
type AppendMemory struct {
	manager   *memory.Manager
	memoryRef string
	key       string
	input     *AppendMemoryInput
}

// NewAppendMemory creates a new append memory use case
func NewAppendMemory(manager *memory.Manager, memoryRef, key string, input *AppendMemoryInput) *AppendMemory {
	return &AppendMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
		input:     input,
	}
}

// Execute appends messages to memory
func (uc *AppendMemory) Execute(ctx context.Context) (*AppendMemoryResult, error) {
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
	if uc.input == nil {
		return nil, ErrInvalidPayload
	}
	if err := ValidateRawMessages(uc.input.Messages); err != nil {
		return nil, err
	}

	// Create a memory reference
	memRef := core.MemoryReference{
		ID:  uc.memoryRef,
		Key: uc.key,
	}
	// Create workflow context for API operations
	workflowContext := map[string]any{
		"api_operation": "append",
		"key":           uc.key,
	}
	// Get memory instance
	instance, err := uc.manager.GetInstance(ctx, memRef, workflowContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory instance: %w", err)
	}
	// Convert input messages to LLM messages
	messages, err := uc.convertToLLMMessages(uc.input.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}
	// Append messages
	for _, msg := range messages {
		if err := instance.Append(ctx, msg); err != nil {
			return nil, fmt.Errorf("failed to append message: %w", err)
		}
	}
	// Get new total count
	totalCount, err := instance.Len(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get message count: %w", err)
	}
	return &AppendMemoryResult{
		Success:    true,
		Appended:   len(messages),
		TotalCount: totalCount,
		Key:        uc.key,
	}, nil
}

// convertToLLMMessages converts generic message format to LLM messages
func (uc *AppendMemory) convertToLLMMessages(messages []map[string]any) ([]llm.Message, error) {
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
