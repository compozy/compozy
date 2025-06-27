package uc

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/worker"
)

// StatsMemoryResult represents the result of getting memory stats
type StatsMemoryResult struct {
	Key              string            `json:"key"`
	MessageCount     int               `json:"message_count"`
	TokenCount       int               `json:"token_count"`
	AverageTokens    float64           `json:"average_tokens_per_message"`
	RoleDistribution map[string]int    `json:"role_distribution"`
	MemoryHealth     *MemoryHealthInfo `json:"memory_health"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
}

// MemoryHealthInfo represents memory health information
type MemoryHealthInfo struct {
	Healthy       bool   `json:"healthy"`
	FlushStrategy string `json:"flush_strategy"`
	LastFlush     string `json:"last_flush,omitempty"`
	TokenUsage    struct {
		Current    int     `json:"current"`
		Threshold  int     `json:"threshold"`
		Percentage float64 `json:"percentage"`
	} `json:"token_usage"`
}

// StatsMemory use case for getting memory statistics
type StatsMemory struct {
	Manager *memory.Manager
	Worker  *worker.Worker
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

// NewStatsMemory creates a new stats memory use case
func NewStatsMemory(manager *memory.Manager, worker *worker.Worker) *StatsMemory {
	return &StatsMemory{
		Manager: manager,
		Worker:  worker,
	}
}

// Execute gets memory statistics
func (uc *StatsMemory) Execute(ctx context.Context, input StatsMemoryInput) (*StatsMemoryOutput, error) {
	// Validate inputs
	if err := uc.validateInput(input); err != nil {
		return nil, err
	}

	// Normalize pagination parameters
	input = uc.normalizePagination(input)

	// Get memory manager
	manager, err := uc.getManager()
	if err != nil {
		return nil, NewErrorContext(err, "stats_memory", input.MemoryRef, input.Key)
	}

	// Get memory instance
	instance, err := uc.getMemoryInstance(ctx, manager, input)
	if err != nil {
		return nil, err
	}

	// Collect stats
	stats, err := uc.collectStats(ctx, instance)
	if err != nil {
		return nil, err
	}

	// Calculate role distribution
	roleDistribution, paginationInfo, err := uc.calculateRoleDistribution(ctx, instance, input)
	if err != nil {
		return nil, err
	}

	// Build output
	return &StatsMemoryOutput{
		Key:               input.Key,
		MessageCount:      stats.messageCount,
		ContextWindowUsed: stats.contextWindowUsed,
		TokenCount:        stats.tokenCount,
		TokenLimit:        stats.tokenLimit,
		TokenUtilization:  stats.tokenUtilization,
		RoleDistribution:  roleDistribution,
		PaginationInfo:    paginationInfo,
	}, nil
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
	if input.Limit <= 0 {
		input.Limit = 100 // Default limit for stats
	}
	if input.Limit > 10000 {
		input.Limit = 10000 // Max limit for stats
	}
	if input.Offset < 0 {
		input.Offset = 0
	}
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

// getMemoryInstance retrieves the memory instance
func (uc *StatsMemory) getMemoryInstance(
	ctx context.Context,
	manager *memory.Manager,
	input StatsMemoryInput,
) (memcore.Memory, error) {
	memRef := core.MemoryReference{
		ID:  input.MemoryRef,
		Key: input.Key,
	}

	workflowContext := map[string]any{
		"api_operation": "stats",
		"key":           input.Key,
	}

	instance, err := manager.GetInstance(ctx, memRef, workflowContext)
	if err != nil {
		if errors.Is(err, memcore.ErrMemoryNotFound) {
			return nil, NewErrorContext(ErrMemoryNotFound, "stats_memory", input.MemoryRef, input.Key)
		}
		return nil, NewErrorContext(
			fmt.Errorf("failed to get memory: %w", err),
			"stats_memory", input.MemoryRef, input.Key,
		).WithDetail("error_type", "memory_access")
	}

	// Check context cancellation before potentially long operation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("operation canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	return instance, nil
}

// statsInfo holds collected statistics
type statsInfo struct {
	messageCount      int
	tokenCount        int
	contextWindowUsed int
	tokenLimit        int
	tokenUtilization  float64
}

// collectStats gathers statistics from the memory instance
func (uc *StatsMemory) collectStats(ctx context.Context, instance memcore.Memory) (*statsInfo, error) {
	stats := &statsInfo{}

	// Get message count
	messageCount, err := instance.Len(ctx)
	if err != nil {
		return nil, NewErrorContext(
			fmt.Errorf("failed to get message count: %w", err),
			"stats_memory", "", "",
		).WithDetail("error_type", "count_error")
	}
	stats.messageCount = messageCount

	// Get token count
	tokenCount, err := instance.GetTokenCount(ctx)
	if err != nil {
		return nil, NewErrorContext(
			fmt.Errorf("failed to get token count: %w", err),
			"stats_memory", "", "",
		).WithDetail("error_type", "token_error")
	}
	stats.tokenCount = tokenCount

	// Get memory health (for future use, currently we use defaults)
	_, err = instance.GetMemoryHealth(ctx)
	if err != nil {
		return nil, NewErrorContext(
			fmt.Errorf("failed to get memory health: %w", err),
			"stats_memory", "", "",
		).WithDetail("error_type", "health_error")
	}

	// Calculate token utilization (using 128K as default limit)
	stats.tokenLimit = 128000 // Default context window
	stats.contextWindowUsed = messageCount
	if stats.tokenLimit > 0 {
		stats.tokenUtilization = float64(tokenCount) / float64(stats.tokenLimit)
	}

	return stats, nil
}

// calculateRoleDistribution calculates role distribution with pagination
func (uc *StatsMemory) calculateRoleDistribution(
	ctx context.Context,
	instance memcore.Memory,
	input StatsMemoryInput,
) (
	map[string]int,
	*PaginationInfo,
	error,
) {
	// Check context cancellation before potentially long operation
	select {
	case <-ctx.Done():
		return nil, nil, fmt.Errorf("operation canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	// Read messages
	messages, err := instance.Read(ctx)
	if err != nil {
		return nil, nil, NewErrorContext(
			fmt.Errorf("failed to read memory for stats: %w", err),
			"stats_memory", input.MemoryRef, input.Key,
		).WithDetail("error_type", "memory_read")
	}

	// Apply pagination if needed
	totalMessages := len(messages)
	var paginationInfo *PaginationInfo

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
		roleDistribution[string(msg.Role)]++
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
