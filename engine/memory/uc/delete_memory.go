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

// NewDeleteMemory creates a new delete memory use case with dependency injection
func NewDeleteMemory(
	manager *memory.Manager,
	memoryRef, key string,
	svc service.MemoryOperationsService,
) (*DeleteMemory, error) {
	if svc == nil {
		var err error
		svc, err = service.NewMemoryOperationsService(manager, nil, nil, nil, nil)
		if err != nil {
			return nil, err
		}
	}
	return &DeleteMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
		service:   svc,
	}, nil
}

// Execute deletes memory content
func (uc *DeleteMemory) Execute(ctx context.Context) (*service.DeleteResponse, error) {
	return uc.service.Delete(ctx, &service.DeleteRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: uc.memoryRef,
			Key:       uc.key,
		},
	})
}
