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

// NewAppendMemory creates a new append memory use case with dependency injection
func NewAppendMemory(
	manager *memory.Manager,
	memoryRef, key string,
	input *AppendMemoryInput,
	svc service.MemoryOperationsService,
) *AppendMemory {
	if svc == nil {
		var err error
		svc, err = service.NewMemoryOperationsService(manager, nil, nil, nil, nil)
		if err != nil {
			panic("failed to create memory operations service: " + err.Error())
		}
	}
	return &AppendMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
		input:     input,
		service:   svc,
	}
}

// Execute appends messages to memory
func (uc *AppendMemory) Execute(ctx context.Context) (*service.AppendResponse, error) {
	return uc.service.Append(ctx, &service.AppendRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: uc.memoryRef,
			Key:       uc.key,
		},
		Payload: uc.input.Messages,
	})
}
