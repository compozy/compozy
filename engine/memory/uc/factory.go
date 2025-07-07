package uc

import (
	"fmt"

	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/memory/service"
	"github.com/compozy/compozy/engine/worker"
)

// Factory provides methods to create use case instances with proper dependency injection
type Factory struct {
	manager       *memory.Manager
	worker        *worker.Worker
	memoryService service.MemoryOperationsService
}

// NewFactory creates a new use case factory
func NewFactory(
	manager *memory.Manager,
	worker *worker.Worker,
	memoryService service.MemoryOperationsService,
) (*Factory, error) {
	// Create default service if not provided
	if memoryService == nil && manager != nil {
		var err error
		memoryService, err = service.NewMemoryOperationsService(manager, nil, nil, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create memory operations service: %w", err)
		}
	}

	return &Factory{
		manager:       manager,
		worker:        worker,
		memoryService: memoryService,
	}, nil
}

// CreateReadMemory creates a new read memory use case with injected dependencies
func (f *Factory) CreateReadMemory() *ReadMemory {
	return NewReadMemory(f.manager, f.worker, f.memoryService)
}

// CreateWriteMemory creates a new write memory use case with injected dependencies
func (f *Factory) CreateWriteMemory(memoryRef, key string, input *WriteMemoryInput) *WriteMemory {
	return NewWriteMemory(f.memoryService, memoryRef, key, input)
}

// CreateAppendMemory creates a new append memory use case with injected dependencies
func (f *Factory) CreateAppendMemory(memoryRef, key string, input *AppendMemoryInput) *AppendMemory {
	return NewAppendMemory(f.manager, memoryRef, key, input, f.memoryService)
}

// CreateDeleteMemory creates a new delete memory use case with injected dependencies
func (f *Factory) CreateDeleteMemory(memoryRef, key string) (*DeleteMemory, error) {
	return NewDeleteMemory(f.manager, memoryRef, key, f.memoryService)
}

// CreateFlushMemory creates a new flush memory use case with injected dependencies
func (f *Factory) CreateFlushMemory(memoryRef, key string, input *FlushMemoryInput) (*FlushMemory, error) {
	return NewFlushMemory(f.manager, memoryRef, key, input, f.memoryService)
}

// CreateClearMemory creates a new clear memory use case with injected dependencies
func (f *Factory) CreateClearMemory(memoryRef, key string, input *ClearMemoryInput) (*ClearMemory, error) {
	return NewClearMemory(f.manager, memoryRef, key, input, f.memoryService)
}

// CreateHealthMemory creates a new health memory use case with injected dependencies
func (f *Factory) CreateHealthMemory(memoryRef, key string, input *HealthMemoryInput) (*HealthMemory, error) {
	return NewHealthMemory(f.manager, memoryRef, key, input, f.memoryService)
}

// CreateStatsMemory creates a new stats memory use case with injected dependencies
func (f *Factory) CreateStatsMemory() (*StatsMemory, error) {
	return NewStatsMemory(f.manager, f.worker, f.memoryService)
}
