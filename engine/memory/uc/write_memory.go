package uc

import (
	"context"

	"github.com/compozy/compozy/engine/memory/service"
)

// WriteMemoryInput represents the input for writing to memory
type WriteMemoryInput struct {
	Messages []map[string]any `json:"messages"`
}

// WriteMemory use case for writing/replacing memory content
type WriteMemory struct {
	memoryRef string
	key       string
	input     *WriteMemoryInput
	service   service.MemoryOperationsService
}

// NewWriteMemory creates a new write memory use case
func NewWriteMemory(
	memoryService service.MemoryOperationsService,
	memoryRef, key string,
	input *WriteMemoryInput,
) *WriteMemory {
	return &WriteMemory{
		memoryRef: memoryRef,
		key:       key,
		input:     input,
		service:   memoryService,
	}
}

// Execute writes/replaces memory content
func (uc *WriteMemory) Execute(ctx context.Context) (*service.WriteResponse, error) {
	// Use centralized service for writing (validation handled by service)
	return uc.service.Write(ctx, &service.WriteRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: uc.memoryRef,
			Key:       uc.key,
		},
		Payload: uc.input.Messages,
	})
}
