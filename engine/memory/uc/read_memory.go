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

// NewReadMemory creates a new read memory use case with dependency injection
func NewReadMemory(
	manager *memory.Manager,
	worker *worker.Worker,
	svc service.MemoryOperationsService,
) *ReadMemory {
	if svc == nil && manager != nil {
		var err error
		svc, err = service.NewMemoryOperationsService(manager, nil, nil, nil, nil)
		if err != nil {
			panic("failed to create memory operations service: " + err.Error())
		}
	}
	return &ReadMemory{
		Manager: manager,
		Worker:  worker,
		Service: svc,
	}
}

// Execute reads memory content with efficient pagination
func (uc *ReadMemory) Execute(ctx context.Context, input ReadMemoryInput) (*ReadMemoryOutput, error) {
	// Apply pagination defaults
	input = uc.applyPaginationDefaults(input)

	// Use centralized service for paginated reading to prevent memory exhaustion
	resp, err := uc.Service.ReadPaginated(ctx, &service.ReadPaginatedRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: input.MemoryRef,
			Key:       input.Key,
		},
		Offset: input.Offset,
		Limit:  input.Limit,
	})
	if err != nil {
		return nil, err // Service already provides typed errors with context
	}

	// Service now returns typed messages directly
	messages := resp.Messages

	// Return paginated result directly from service
	return &ReadMemoryOutput{
		Messages:   messages,
		TotalCount: resp.TotalCount,
		HasMore:    resp.HasMore,
	}, nil
}

func (uc *ReadMemory) applyPaginationDefaults(input ReadMemoryInput) ReadMemoryInput {
	input.Offset, input.Limit = NormalizePagination(input.Offset, input.Limit, DefaultPaginationLimits)
	return input
}
