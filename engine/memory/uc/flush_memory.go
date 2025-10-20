package uc

import (
	"context"

	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/memory/service"
)

// FlushMemoryInput represents the input for flushing memory
type FlushMemoryInput struct {
	Force    bool   `json:"force"`
	DryRun   bool   `json:"dry_run"`
	MaxKeys  int    `json:"max_keys,omitempty"`
	Strategy string `json:"strategy,omitempty"`
}

// FlushMemoryResult represents the result of flushing memory
type FlushMemoryResult struct {
	Success          bool   `json:"success"`
	Key              string `json:"key"`
	SummaryGenerated bool   `json:"summary_generated"`
	MessageCount     int    `json:"message_count"`
	TokenCount       int    `json:"token_count"`
	DryRun           bool   `json:"dry_run,omitempty"`
	WouldFlush       bool   `json:"would_flush,omitempty"`
	ActualStrategy   string `json:"actual_strategy"`
	Error            string `json:"error,omitempty"`
}

// FlushMemory use case for flushing memory content
type FlushMemory struct {
	manager   *memory.Manager
	memoryRef string
	key       string
	input     *FlushMemoryInput
	service   service.MemoryOperationsService
}

// NewFlushMemory creates a new flush memory use case with dependency injection
func NewFlushMemory(
	manager *memory.Manager,
	memoryRef, key string,
	input *FlushMemoryInput,
	svc service.MemoryOperationsService,
) (*FlushMemory, error) {
	if input == nil {
		input = &FlushMemoryInput{}
	}
	if svc == nil {
		var err error
		svc, err = service.NewMemoryOperationsService(manager, nil, nil, nil, nil)
		if err != nil {
			// Log error but continue with nil service
			return nil, err
		}
	}
	return &FlushMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
		input:     input,
		service:   svc,
	}, nil
}

// Execute flushes memory content
func (uc *FlushMemory) Execute(ctx context.Context) (*FlushMemoryResult, error) {
	// Validate inputs
	if err := uc.validate(); err != nil {
		return nil, err
	}
	// Use centralized service for flushing
	resp, err := uc.service.Flush(ctx, &service.FlushRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: uc.memoryRef,
			Key:       uc.key,
		},
		Config: &service.FlushConfig{
			Strategy: uc.input.Strategy,
			MaxKeys:  uc.input.MaxKeys,
			DryRun:   uc.input.DryRun,
			Force:    uc.input.Force,
		},
	})
	if err != nil {
		return nil, NewErrorContext(err, "flush_memory", uc.memoryRef, uc.key)
	}
	return &FlushMemoryResult{
		Success:          resp.Success,
		Key:              resp.Key,
		SummaryGenerated: resp.SummaryGenerated,
		MessageCount:     resp.MessageCount,
		TokenCount:       resp.TokenCount,
		DryRun:           resp.DryRun,
		WouldFlush:       resp.WouldFlush,
		ActualStrategy:   resp.ActualStrategy,
		Error:            resp.Error,
	}, nil
}

// validate performs input validation
func (uc *FlushMemory) validate() error {
	if uc.manager == nil {
		return ErrMemoryManagerNotAvailable
	}
	if err := ValidateMemoryRef(uc.memoryRef); err != nil {
		return err
	}
	if err := ValidateKey(uc.key); err != nil {
		return err
	}
	if err := ValidateFlushInput(uc.input); err != nil {
		return err
	}
	return nil
}
