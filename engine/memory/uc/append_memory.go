package uc

import (
	"context"

	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/memory/service"
)

// AppendMemoryInput represents the input for appending to memory
type AppendMemoryInput struct {
	Messages []map[string]any `json:"messages"`
}

// AppendMemory use case for appending to memory content
type AppendMemory struct {
	manager   *memory.Manager
	memoryRef string
	key       string
	input     *AppendMemoryInput
	service   service.MemoryOperationsService
}

// NewAppendMemory creates a new append memory use case
func NewAppendMemory(manager *memory.Manager, memoryRef, key string, input *AppendMemoryInput) *AppendMemory {
	memService := service.NewMemoryOperationsService(manager, nil, nil)
	return &AppendMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
		input:     input,
		service:   memService,
	}
}

// Execute appends messages to memory
func (uc *AppendMemory) Execute(ctx context.Context) (*service.AppendResponse, error) {
	// Use centralized service for appending (validation handled by service)
	return uc.service.Append(ctx, &service.AppendRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: uc.memoryRef,
			Key:       uc.key,
		},
		Payload: uc.input.Messages,
	})
}
