package uc

import (
	"context"

	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/memory/service"
)

// DeleteMemory use case for deleting memory content
type DeleteMemory struct {
	manager   *memory.Manager
	memoryRef string
	key       string
	service   service.MemoryOperationsService
}

// NewDeleteMemory creates a new delete memory use case
func NewDeleteMemory(manager *memory.Manager, memoryRef, key string) *DeleteMemory {
	memService := service.NewMemoryOperationsService(manager, nil, nil)
	return &DeleteMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
		service:   memService,
	}
}

// Execute deletes memory content
func (uc *DeleteMemory) Execute(ctx context.Context) (*service.DeleteResponse, error) {
	// Use centralized service for deleting (validation handled by service)
	return uc.service.Delete(ctx, &service.DeleteRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: uc.memoryRef,
			Key:       uc.key,
		},
	})
}
