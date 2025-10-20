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
	if err := uc.validateInput(input); err != nil {
		return nil, err
	}
	if _, err := uc.getManager(); err != nil {
		return nil, NewErrorContext(err, "stats_memory", input.MemoryRef, input.Key)
	}
	statsResp, err := uc.fetchStats(ctx, input)
	if err != nil {
		return nil, err
	}
	tokenLimit, err := uc.getTokenLimit(ctx, input.MemoryRef)
	if err != nil {
		return nil, NewErrorContext(err, "stats_memory", input.MemoryRef, input.Key)
	}
	output := uc.buildStatsOutput(input.Key, statsResp, tokenLimit)
	roleDistribution, paginationInfo, err := uc.calculateRoleDistribution(ctx, input)
	if err != nil {
		return nil, err
	}
	output.RoleDistribution = roleDistribution
	output.PaginationInfo = paginationInfo
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
func (uc *StatsMemory) getTokenLimit(ctx context.Context, memoryRef string) (int, error) {
	manager, err := uc.getManager()
	if err != nil {
		return 0, err
	}
	// Get memory resource configuration from manager
	resource, err := manager.GetMemoryConfig(ctx, memoryRef)
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
	input StatsMemoryInput,
) (
	map[string]int,
	*PaginationInfo,
	error,
) {
	normalized := uc.normalizePagination(input)
	messages, err := uc.readMessages(ctx, normalized)
	if err != nil {
		return nil, nil, err
	}
	paginated, pagination := uc.applyPagination(messages, normalized)
	return uc.countRoles(paginated), pagination, nil
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

func (uc *StatsMemory) fetchStats(ctx context.Context, input StatsMemoryInput) (*service.StatsResponse, error) {
	resp, err := uc.service.Stats(ctx, &service.StatsRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: input.MemoryRef,
			Key:       input.Key,
		},
		Config: &service.StatsConfig{IncludeContent: true},
	})
	if err != nil {
		return nil, NewErrorContext(err, "stats_memory", input.MemoryRef, input.Key)
	}
	return resp, nil
}

func (uc *StatsMemory) buildStatsOutput(key string, resp *service.StatsResponse, tokenLimit int) *StatsMemoryOutput {
	output := &StatsMemoryOutput{
		Key:               key,
		MessageCount:      resp.MessageCount,
		ContextWindowUsed: resp.MessageCount,
		TokenCount:        resp.TokenCount,
		TokenLimit:        tokenLimit,
	}
	if tokenLimit > 0 {
		output.TokenUtilization = float64(resp.TokenCount) / float64(tokenLimit)
	}
	return output
}

func (uc *StatsMemory) readMessages(ctx context.Context, input StatsMemoryInput) ([]llm.Message, error) {
	resp, err := uc.service.Read(ctx, &service.ReadRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: input.MemoryRef,
			Key:       input.Key,
		},
	})
	if err != nil {
		return nil, NewErrorContext(err, "stats_memory", input.MemoryRef, input.Key)
	}
	return resp.Messages, nil
}

func (uc *StatsMemory) applyPagination(
	messages []llm.Message,
	input StatsMemoryInput,
) ([]llm.Message, *PaginationInfo) {
	total := len(messages)
	if input.Limit <= 0 || total <= input.Limit {
		return messages, nil
	}
	info := &PaginationInfo{
		Limit:      input.Limit,
		Offset:     input.Offset,
		TotalCount: total,
		HasMore:    (input.Offset + input.Limit) < total,
	}
	return uc.paginateMessages(messages, input.Offset, input.Limit), info
}

func (uc *StatsMemory) countRoles(messages []llm.Message) map[string]int {
	distribution := make(map[string]int)
	for _, msg := range messages {
		role := string(msg.Role)
		if role == "" {
			role = "unknown"
		}
		distribution[role]++
	}
	return distribution
}
