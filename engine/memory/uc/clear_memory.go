package uc

import (
	"context"

	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/memory/service"
)

// ClearMemoryInput represents the input for clearing memory
type ClearMemoryInput struct {
	Confirm bool `json:"confirm"`
	Backup  bool `json:"backup"`
}

// ClearMemoryResult represents the result of clearing memory
type ClearMemoryResult struct {
	Success         bool   `json:"success"`
	Key             string `json:"key"`
	MessagesCleared int    `json:"messages_cleared"`
	BackupCreated   bool   `json:"backup_created"`
}

// ClearMemory use case for clearing memory content
type ClearMemory struct {
	manager   *memory.Manager
	memoryRef string
	key       string
	input     *ClearMemoryInput
	service   service.MemoryOperationsService
}

// NewClearMemory creates a new clear memory use case with dependency injection
func NewClearMemory(
	manager *memory.Manager,
	memoryRef, key string,
	input *ClearMemoryInput,
	svc service.MemoryOperationsService,
) *ClearMemory {
	if input == nil {
		input = &ClearMemoryInput{}
	}
	if svc == nil {
		svc = service.NewMemoryOperationsService(manager, nil, nil)
	}
	return &ClearMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
		input:     input,
		service:   svc,
	}
}

// Execute clears memory content
func (uc *ClearMemory) Execute(ctx context.Context) (*ClearMemoryResult, error) {
	// Validate inputs
	if err := uc.validate(); err != nil {
		return nil, err
	}

	// Use centralized service for clearing
	resp, err := uc.service.Clear(ctx, &service.ClearRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: uc.memoryRef,
			Key:       uc.key,
		},
		Config: &service.ClearConfig{
			Confirm: uc.input.Confirm,
			Backup:  uc.input.Backup,
		},
	})
	if err != nil {
		return nil, NewErrorContext(err, "clear_memory", uc.memoryRef, uc.key)
	}

	return &ClearMemoryResult{
		Success:         resp.Success,
		Key:             resp.Key,
		MessagesCleared: resp.MessagesCleared,
		BackupCreated:   resp.BackupCreated,
	}, nil
}

// validate performs input validation
func (uc *ClearMemory) validate() error {
	if uc.manager == nil {
		return ErrMemoryManagerNotAvailable
	}
	if err := ValidateMemoryRef(uc.memoryRef); err != nil {
		return err
	}
	if err := ValidateKey(uc.key); err != nil {
		return err
	}
	if err := ValidateClearInput(uc.input); err != nil {
		return err
	}
	return nil
}
