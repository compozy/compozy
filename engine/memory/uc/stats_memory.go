package uc

import (
	"context"

	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/memory/service"
	"github.com/compozy/compozy/engine/worker"
)

// StatsMemory use case for getting memory statistics
type StatsMemory struct {
	Manager *memory.Manager
	Worker  *worker.Worker
	service service.MemoryOperationsService
}

type StatsMemoryInput struct {
	MemoryRef string
	Key       string
	Limit     int // Limit for role distribution calculation
	Offset    int // Offset for role distribution calculation
}

type StatsMemoryOutput struct {
	Key               string          `json:"key"`
	MessageCount      int             `json:"message_count"`
	ContextWindowUsed int             `json:"context_window_used"`
	TokenCount        int             `json:"token_count"`
	TokenLimit        int             `json:"token_limit"`
	TokenUtilization  float64         `json:"token_utilization"`
	RoleDistribution  map[string]int  `json:"role_distribution"`
	PaginationInfo    *PaginationInfo `json:"pagination_info,omitempty"`
}

type PaginationInfo struct {
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	TotalCount int  `json:"total_count"`
	HasMore    bool `json:"has_more"`
}

// NewStatsMemory creates a new stats memory use case with dependency injection
func NewStatsMemory(
	manager *memory.Manager,
	worker *worker.Worker,
	svc service.MemoryOperationsService,
) (*StatsMemory, error) {
	if svc == nil && manager != nil {
		var err error
		svc, err = service.NewMemoryOperationsService(manager, nil, nil, nil, nil)
		if err != nil {
			// Log error but continue with nil service
			return nil, err
		}
	}
	return &StatsMemory{
		Manager: manager,
		Worker:  worker,
		service: svc,
	}, nil
}

// Execute gets memory statistics
func (uc *StatsMemory) Execute(ctx context.Context, input StatsMemoryInput) (*StatsMemoryOutput, error) {
	// Validate inputs
	if err := uc.validateInput(input); err != nil {
		return nil, err
	}

	// Get memory manager
	manager, err := uc.getManager()
	if err != nil {
		return nil, NewErrorContext(err, "stats_memory", input.MemoryRef, input.Key)
	}

	// Use centralized service for stats
	resp, err := uc.service.Stats(ctx, &service.StatsRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: input.MemoryRef,
			Key:       input.Key,
		},
		Config: &service.StatsConfig{
			IncludeContent: true, // Always include content for role distribution
		},
	})
	if err != nil {
		return nil, NewErrorContext(err, "stats_memory", input.MemoryRef, input.Key)
	}

	// Get token limit from memory configuration
	tokenLimit, err := uc.getTokenLimit(ctx, input.MemoryRef)
	if err != nil {
		return nil, NewErrorContext(err, "stats_memory", input.MemoryRef, input.Key)
	}

	// Build basic output from service response
	output := &StatsMemoryOutput{
		Key:              input.Key,
		MessageCount:     resp.MessageCount,
		TokenCount:       resp.TokenCount,
		TokenLimit:       tokenLimit,
		TokenUtilization: float64(resp.TokenCount) / float64(tokenLimit),
	}

	// Calculate role distribution separately for compatibility
	roleDistribution, paginationInfo, err := uc.calculateRoleDistribution(ctx, manager, input)
	if err != nil {
		return nil, err
	}

	output.RoleDistribution = roleDistribution
	output.PaginationInfo = paginationInfo
	output.ContextWindowUsed = resp.MessageCount

	return output, nil
}

// validateInput validates the input parameters
func (uc *StatsMemory) validateInput(input StatsMemoryInput) error {
	if err := ValidateMemoryRef(input.MemoryRef); err != nil {
		return NewErrorContext(err, "stats_memory", input.MemoryRef, input.Key)
	}
	if err := ValidateKey(input.Key); err != nil {
		return NewErrorContext(err, "stats_memory", input.MemoryRef, input.Key)
	}
	return nil
}

// normalizePagination applies defaults and limits to pagination parameters
func (uc *StatsMemory) normalizePagination(input StatsMemoryInput) StatsMemoryInput {
	input.Offset, input.Limit = NormalizePagination(input.Offset, input.Limit, StatsPaginationLimits)
	return input
}

// getManager returns the memory manager
func (uc *StatsMemory) getManager() (*memory.Manager, error) {
	manager := uc.Manager
	if manager == nil && uc.Worker != nil {
		manager = uc.Worker.GetMemoryManager()
	}
	if manager == nil {
		return nil, ErrMemoryManagerNotAvailable
	}
	return manager, nil
}

// getTokenLimit retrieves the token limit from memory configuration
func (uc *StatsMemory) getTokenLimit(_ context.Context, memoryRef string) (int, error) {
	manager, err := uc.getManager()
	if err != nil {
		return 0, err
	}
	// Get memory resource configuration from manager
	resource, err := manager.GetMemoryConfig(memoryRef)
	if err != nil {
		return 0, NewErrorContext(err, "get_memory_config", memoryRef, "")
	}
	// Use MaxTokens from configuration, with fallback to default context window
	if resource.MaxTokens > 0 {
		return resource.MaxTokens, nil
	}
	// Default fallback for when MaxTokens is not configured
	return 128000, nil
}

// calculateRoleDistribution calculates role distribution with pagination using memory manager
func (uc *StatsMemory) calculateRoleDistribution(
	ctx context.Context,
	_ *memory.Manager,
	input StatsMemoryInput,
) (
	map[string]int,
	*PaginationInfo,
	error,
) {
	// Normalize pagination parameters
	input = uc.normalizePagination(input)

	// Use centralized service to read messages
	resp, err := uc.service.Read(ctx, &service.ReadRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: input.MemoryRef,
			Key:       input.Key,
		},
	})
	if err != nil {
		return nil, nil, NewErrorContext(err, "stats_memory", input.MemoryRef, input.Key)
	}

	// Apply pagination if needed
	totalMessages := len(resp.Messages)
	var paginationInfo *PaginationInfo
	messages := resp.Messages

	if input.Limit > 0 && totalMessages > input.Limit {
		paginationInfo = &PaginationInfo{
			Limit:      input.Limit,
			Offset:     input.Offset,
			TotalCount: totalMessages,
			HasMore:    (input.Offset + input.Limit) < totalMessages,
		}

		// Apply pagination to messages
		messages = uc.paginateMessages(messages, input.Offset, input.Limit)
	}

	// Calculate role distribution
	roleDistribution := make(map[string]int)
	for _, msg := range messages {
		role := string(msg.Role)
		if role == "" {
			role = "unknown"
		}
		roleDistribution[role]++
	}

	return roleDistribution, paginationInfo, nil
}

// paginateMessages applies pagination to message slice
func (uc *StatsMemory) paginateMessages(messages []llm.Message, offset, limit int) []llm.Message {
	start := offset
	end := offset + limit

	if start >= len(messages) {
		return []llm.Message{}
	}

	if end > len(messages) {
		end = len(messages)
	}

	return messages[start:end]
}
