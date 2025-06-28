package uc

import (
	"context"

	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/memory/service"
)

// HealthMemoryInput represents the input for checking memory health
type HealthMemoryInput struct {
	IncludeStats bool `json:"include_stats"`
}

// HealthMemoryResult represents the result of checking memory health
type HealthMemoryResult struct {
	Healthy       bool   `json:"healthy"`
	Key           string `json:"key"`
	TokenCount    int    `json:"token_count"`
	MessageCount  int    `json:"message_count"`
	FlushStrategy string `json:"flush_strategy"`
	LastFlush     string `json:"last_flush,omitempty"`
	CurrentTokens int    `json:"current_tokens,omitempty"`
}

// HealthMemory use case for checking memory health
type HealthMemory struct {
	manager   *memory.Manager
	memoryRef string
	key       string
	input     *HealthMemoryInput
	service   service.MemoryOperationsService
}

// NewHealthMemory creates a new health memory use case
func NewHealthMemory(manager *memory.Manager, memoryRef, key string, input *HealthMemoryInput) *HealthMemory {
	if input == nil {
		input = &HealthMemoryInput{}
	}
	memService := service.NewMemoryOperationsService(manager, nil, nil)
	return &HealthMemory{
		manager:   manager,
		memoryRef: memoryRef,
		key:       key,
		input:     input,
		service:   memService,
	}
}

// Execute checks memory health
func (uc *HealthMemory) Execute(ctx context.Context) (*HealthMemoryResult, error) {
	// Validate inputs
	if err := uc.validate(); err != nil {
		return nil, err
	}

	// Use centralized service for health check
	resp, err := uc.service.Health(ctx, &service.HealthRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: uc.memoryRef,
			Key:       uc.key,
		},
		Config: &service.HealthConfig{
			IncludeStats: uc.input.IncludeStats,
		},
	})
	if err != nil {
		return nil, NewErrorContext(err, "health_memory", uc.memoryRef, uc.key)
	}

	return &HealthMemoryResult{
		Healthy:       resp.Healthy,
		Key:           resp.Key,
		TokenCount:    resp.TokenCount,
		MessageCount:  resp.MessageCount,
		FlushStrategy: resp.FlushStrategy,
		LastFlush:     resp.LastFlush,
		CurrentTokens: resp.CurrentTokens,
	}, nil
}

// validate performs input validation
func (uc *HealthMemory) validate() error {
	if uc.manager == nil {
		return ErrMemoryManagerNotAvailable
	}
	if err := ValidateMemoryRef(uc.memoryRef); err != nil {
		return err
	}
	if err := ValidateKey(uc.key); err != nil {
		return err
	}
	return nil
}
