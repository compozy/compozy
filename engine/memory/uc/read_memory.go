package uc

import (
	"context"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/memory/service"
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
	Service service.MemoryOperationsService
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
	memService := service.NewMemoryOperationsService(manager, nil, nil)
	return &ReadMemory{
		Manager: manager,
		Worker:  worker,
		Service: memService,
	}
}

// Execute reads memory content
func (uc *ReadMemory) Execute(ctx context.Context, input ReadMemoryInput) (*ReadMemoryOutput, error) {
	// Apply pagination defaults
	input = uc.applyPaginationDefaults(input)

	// Use centralized service for reading (validation handled by service)
	resp, err := uc.Service.Read(ctx, &service.ReadRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: input.MemoryRef,
			Key:       input.Key,
		},
	})
	if err != nil {
		return nil, err // Service already provides typed errors with context
	}

	// Convert service response to LLM messages for pagination
	messages := make([]llm.Message, len(resp.Messages))
	for i, msgMap := range resp.Messages {
		role, ok := msgMap["role"].(string)
		if !ok {
			role = "unknown"
		}
		content, ok := msgMap["content"].(string)
		if !ok {
			content = ""
		}
		messages[i] = llm.Message{
			Role:    llm.MessageRole(role),
			Content: content,
		}
	}

	// Apply pagination to messages
	return uc.applyPagination(messages, input), nil
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

func (uc *ReadMemory) applyPagination(messages []llm.Message, input ReadMemoryInput) *ReadMemoryOutput {
	// Handle empty memory
	if len(messages) == 0 {
		return &ReadMemoryOutput{
			Messages:   []llm.Message{},
			TotalCount: 0,
			HasMore:    false,
		}
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
		}
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
	}
}
