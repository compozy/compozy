package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/memory/service"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

type ExecuteMemoryOperationInput struct {
	TaskConfig    *task.Config    `json:"task_config"`
	MergedInput   *core.Input     `json:"merged_input"`
	WorkflowState *workflow.State `json:"workflow_state"`
}

type ExecuteMemoryOperation struct {
	memoryManager  memcore.ManagerInterface
	templateEngine *tplengine.TemplateEngine
	memoryService  service.MemoryOperationsService
}

func NewExecuteMemoryOperation(
	memoryManager memcore.ManagerInterface,
	templateEngine *tplengine.TemplateEngine,
) *ExecuteMemoryOperation {
	memoryService := service.NewMemoryOperationsService(memoryManager, templateEngine, nil)
	return &ExecuteMemoryOperation{
		memoryManager:  memoryManager,
		templateEngine: templateEngine,
		memoryService:  memoryService,
	}
}

func (uc *ExecuteMemoryOperation) Execute(
	ctx context.Context,
	input *ExecuteMemoryOperationInput,
) (*core.Output, error) {
	// Set up evaluator context following project standards
	evalContext := map[string]any{
		"workflow": map[string]any{
			"id":      input.WorkflowState.WorkflowID,
			"exec_id": input.WorkflowState.WorkflowExecID,
			"input":   input.WorkflowState.Input,
		},
		"tasks": input.WorkflowState.Tasks,
	}
	// Add merged input as "input" at top level for task context
	if input.MergedInput != nil {
		evalContext["input"] = input.MergedInput
	}
	// Validate key template is provided for memory scoping
	if input.TaskConfig.KeyTemplate == "" {
		return nil, fmt.Errorf("key_template is required for memory scoping")
	}
	// Get memory reference
	memRef := core.MemoryReference{
		ID:  input.TaskConfig.MemoryRef,
		Key: input.TaskConfig.KeyTemplate,
	}
	// Get memory instance
	memInstance, err := uc.memoryManager.GetInstance(ctx, memRef, evalContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory instance: %w", err)
	}
	if memInstance == nil {
		return nil, fmt.Errorf("memory manager returned nil instance")
	}
	// Note: Wildcard key patterns and batch processing are planned features
	// that will enhance performance for large-scale memory operations
	// Get resolved key once for efficiency
	resolvedKey := memInstance.GetID()
	// Execute operation based on type using centralized service
	switch input.TaskConfig.Operation {
	case task.MemoryOpRead:
		return uc.executeRead(ctx, memRef.ID, resolvedKey)
	case task.MemoryOpWrite:
		return uc.executeWrite(
			ctx,
			memRef.ID,
			resolvedKey,
			input.TaskConfig.Payload,
			input.MergedInput,
			input.WorkflowState,
		)
	case task.MemoryOpAppend:
		return uc.executeAppend(
			ctx,
			memRef.ID,
			resolvedKey,
			input.TaskConfig.Payload,
			input.MergedInput,
			input.WorkflowState,
		)
	case task.MemoryOpDelete:
		return uc.executeDelete(ctx, memRef.ID, resolvedKey)
	case task.MemoryOpFlush:
		return uc.executeFlush(ctx, memRef.ID, resolvedKey, input.TaskConfig.FlushConfig)
	case task.MemoryOpHealth:
		return uc.executeHealth(ctx, memRef.ID, resolvedKey, input.TaskConfig.HealthConfig)
	case task.MemoryOpClear:
		return uc.executeClear(ctx, memRef.ID, resolvedKey, input.TaskConfig.ClearConfig)
	case task.MemoryOpStats:
		return uc.executeStats(ctx, memRef.ID, resolvedKey, input.TaskConfig.StatsConfig)
	default:
		return nil, fmt.Errorf("unsupported memory operation: %s", input.TaskConfig.Operation)
	}
}

func (uc *ExecuteMemoryOperation) executeRead(
	ctx context.Context,
	memoryRef string,
	key string,
) (*core.Output, error) {
	resp, err := uc.memoryService.Read(ctx, &service.ReadRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: memoryRef,
			Key:       key,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read memory: %w", err)
	}
	return &core.Output{
		"messages": resp.Messages,
		"count":    len(resp.Messages),
		"key":      key,
	}, nil
}

func (uc *ExecuteMemoryOperation) executeWrite(
	ctx context.Context,
	memoryRef string,
	key string,
	payload any,
	mergedInput *core.Input,
	workflowState *workflow.State,
) (*core.Output, error) {
	// Let the service handle template resolution
	resp, err := uc.memoryService.Write(ctx, &service.WriteRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: memoryRef,
			Key:       key,
		},
		Payload:       payload,
		MergedInput:   mergedInput,
		WorkflowState: workflowState,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to write memory: %w", err)
	}
	return &core.Output{
		"success": resp.Success,
		"count":   resp.Count,
		"key":     key,
	}, nil
}

func (uc *ExecuteMemoryOperation) executeAppend(
	ctx context.Context,
	memoryRef string,
	key string,
	payload any,
	mergedInput *core.Input,
	workflowState *workflow.State,
) (*core.Output, error) {
	// Let the service handle template resolution
	resp, err := uc.memoryService.Append(ctx, &service.AppendRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: memoryRef,
			Key:       key,
		},
		Payload:       payload,
		MergedInput:   mergedInput,
		WorkflowState: workflowState,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to append to memory: %w", err)
	}
	return &core.Output{
		"success":     resp.Success,
		"appended":    resp.Appended,
		"total_count": resp.TotalCount,
		"key":         key,
	}, nil
}

func (uc *ExecuteMemoryOperation) executeDelete(
	ctx context.Context,
	memoryRef string,
	key string,
) (*core.Output, error) {
	resp, err := uc.memoryService.Delete(ctx, &service.DeleteRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: memoryRef,
			Key:       key,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to delete memory: %w", err)
	}
	return &core.Output{
		"success": resp.Success,
		"key":     key,
	}, nil
}

func (uc *ExecuteMemoryOperation) executeFlush(
	ctx context.Context,
	memoryRef string,
	key string,
	config *task.FlushConfig,
) (*core.Output, error) {
	// Convert task flush config to service flush config
	flushConfig := &service.FlushConfig{}
	if config != nil {
		flushConfig.Force = config.Force
		flushConfig.DryRun = config.DryRun
		flushConfig.Strategy = config.Strategy
		flushConfig.MaxKeys = config.MaxKeys
		flushConfig.Threshold = config.Threshold
	}
	resp, err := uc.memoryService.Flush(ctx, &service.FlushRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: memoryRef,
			Key:       key,
		},
		Config: flushConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to flush memory: %w", err)
	}
	output := &core.Output{
		"success":           resp.Success,
		"key":               key,
		"summary_generated": resp.SummaryGenerated,
		"message_count":     resp.MessageCount,
		"token_count":       resp.TokenCount,
	}
	if config != nil && config.DryRun {
		(*output)["dry_run"] = true
		(*output)["would_flush"] = resp.WouldFlush
	}
	if resp.Error != "" {
		(*output)["error"] = resp.Error
		return output, fmt.Errorf("flush completed with error: %s", resp.Error)
	}
	return output, nil
}

func (uc *ExecuteMemoryOperation) executeHealth(
	ctx context.Context,
	memoryRef string,
	key string,
	config *task.HealthConfig,
) (*core.Output, error) {
	if config == nil {
		return nil, fmt.Errorf("health operation requires health_config to be provided")
	}
	// Convert task health config to service health config
	healthConfig := &service.HealthConfig{
		IncludeStats: config.IncludeStats,
	}
	resp, err := uc.memoryService.Health(ctx, &service.HealthRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: memoryRef,
			Key:       key,
		},
		Config: healthConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get memory health: %w", err)
	}
	result := map[string]any{
		"healthy":         resp.Healthy,
		"key":             key,
		"token_count":     resp.TokenCount,
		"message_count":   resp.MessageCount,
		"actual_strategy": resp.ActualStrategy,
	}
	if resp.LastFlush != "" {
		result["last_flush"] = resp.LastFlush
	}
	if config.IncludeStats && resp.CurrentTokens > 0 {
		result["current_tokens"] = resp.CurrentTokens
	}
	output := core.Output(result)
	return &output, nil
}

func (uc *ExecuteMemoryOperation) executeClear(
	ctx context.Context,
	memoryRef string,
	key string,
	config *task.ClearConfig,
) (*core.Output, error) {
	if config == nil {
		return nil, fmt.Errorf("clear operation requires clear_config to be provided")
	}
	// Convert task clear config to service clear config
	clearConfig := &service.ClearConfig{
		Confirm: config.Confirm,
		Backup:  config.Backup,
	}
	resp, err := uc.memoryService.Clear(ctx, &service.ClearRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: memoryRef,
			Key:       key,
		},
		Config: clearConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clear memory: %w", err)
	}
	return &core.Output{
		"success":          resp.Success,
		"key":              key,
		"messages_cleared": resp.MessagesCleared,
		"backup_created":   resp.BackupCreated,
	}, nil
}

func (uc *ExecuteMemoryOperation) executeStats(
	ctx context.Context,
	memoryRef string,
	key string,
	config *task.StatsConfig,
) (*core.Output, error) {
	if config == nil {
		return nil, fmt.Errorf("stats operation requires stats_config to be provided")
	}
	// Convert task stats config to service stats config
	statsConfig := &service.StatsConfig{
		IncludeContent: config.IncludeContent,
	}
	resp, err := uc.memoryService.Stats(ctx, &service.StatsRequest{
		BaseRequest: service.BaseRequest{
			MemoryRef: memoryRef,
			Key:       key,
		},
		Config: statsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}
	stats := map[string]any{
		"key":             key,
		"message_count":   resp.MessageCount,
		"token_count":     resp.TokenCount,
		"actual_strategy": resp.ActualStrategy,
	}
	if resp.LastFlush != "" {
		stats["last_flush"] = resp.LastFlush
	}
	if config.IncludeContent && resp.MessageCount > 0 {
		avgTokens := 0
		if resp.MessageCount > 0 {
			avgTokens = resp.TokenCount / resp.MessageCount
		}
		stats["avg_tokens_per_message"] = avgTokens
	}
	output := core.Output(stats)
	return &output, nil
}
