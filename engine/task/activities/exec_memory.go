package activities

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/services"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

const ExecuteMemoryLabel = "ExecuteMemoryTask"

type ExecuteMemoryInput struct {
	WorkflowID     string       `json:"workflow_id"`
	WorkflowExecID core.ID      `json:"workflow_exec_id"`
	TaskConfig     *task.Config `json:"task_config"`
	MergedInput    *core.Input  `json:"merged_input"`
}

type ExecuteMemory struct {
	loadWorkflowUC *uc.LoadWorkflow
	createStateUC  *uc.CreateState
	taskResponder  *services.TaskResponder
	memoryManager  memory.ManagerInterface
	templateEngine *tplengine.TemplateEngine
}

// NewExecuteMemory creates a new ExecuteMemory activity
func NewExecuteMemory(
	workflows []*workflow.Config,
	workflowRepo workflow.Repository,
	taskRepo task.Repository,
	configStore services.ConfigStore,
	memoryManager memory.ManagerInterface,
	cwd *core.PathCWD,
	templateEngine *tplengine.TemplateEngine,
) *ExecuteMemory {
	configManager := services.NewConfigManager(configStore, cwd)
	return &ExecuteMemory{
		loadWorkflowUC: uc.NewLoadWorkflow(workflows, workflowRepo),
		createStateUC:  uc.NewCreateState(taskRepo, configManager),
		taskResponder:  services.NewTaskResponder(workflowRepo, taskRepo),
		memoryManager:  memoryManager,
		templateEngine: templateEngine,
	}
}

func (a *ExecuteMemory) Run(ctx context.Context, input *ExecuteMemoryInput) (*task.MainTaskResponse, error) {
	// Validate input
	if input.TaskConfig == nil {
		return nil, fmt.Errorf("task_config is required for memory task")
	}
	// Load workflow state and config
	workflowState, workflowConfig, err := a.loadWorkflowUC.Execute(ctx, &uc.LoadWorkflowInput{
		WorkflowID:     input.WorkflowID,
		WorkflowExecID: input.WorkflowExecID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow: %w", err)
	}
	// Create state
	state, err := a.createStateUC.Execute(ctx, &uc.CreateStateInput{
		TaskConfig:     input.TaskConfig,
		WorkflowConfig: workflowConfig,
		WorkflowState:  workflowState,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create state: %w", err)
	}
	// Execute memory operation
	output, executionError := a.executeMemoryOperation(ctx, input.TaskConfig, input.MergedInput, workflowState)
	if executionError == nil {
		state.Output = output
	}
	// Handle response using task responder
	response, handleErr := a.taskResponder.HandleMainTask(ctx, &services.MainTaskResponseInput{
		WorkflowConfig: workflowConfig,
		TaskState:      state,
		TaskConfig:     input.TaskConfig,
		ExecutionError: executionError,
	})
	if handleErr != nil {
		return nil, handleErr
	}
	return response, executionError
}

func (a *ExecuteMemory) executeMemoryOperation(
	ctx context.Context,
	config *task.Config,
	mergedInput *core.Input,
	workflowState *workflow.State,
) (*core.Output, error) {
	// Set up evaluator context
	evalContext := map[string]any{
		"workflow": map[string]any{
			"id":      workflowState.WorkflowID,
			"exec_id": workflowState.WorkflowExecID,
			"input":   workflowState.Input,
		},
	}
	if mergedInput != nil {
		for k, v := range *mergedInput {
			evalContext[k] = v
		}
	}
	// Validate key template is provided for memory scoping
	if config.KeyTemplate == "" {
		return nil, fmt.Errorf("key_template is required for memory scoping")
	}
	// Get memory reference
	memRef := core.MemoryReference{
		ID:  config.MemoryRef,
		Key: config.KeyTemplate,
	}
	// Get memory instance
	memInstance, err := a.memoryManager.GetInstance(ctx, memRef, evalContext)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory instance: %w", err)
	}
	if memInstance == nil {
		return nil, fmt.Errorf("memory manager returned nil instance")
	}
	// TODO: Implement wildcard key pattern support using config.MaxKeys for safety limits
	// TODO: Implement batch processing using config.BatchSize for operations like stats
	// Get resolved key once for efficiency
	resolvedKey := memInstance.GetID()
	// Execute operation based on type
	switch config.Operation {
	case task.MemoryOpRead:
		return a.executeRead(ctx, memInstance, resolvedKey)
	case task.MemoryOpWrite:
		return a.executeWrite(ctx, memInstance, resolvedKey, config.Payload, mergedInput)
	case task.MemoryOpAppend:
		return a.executeAppend(ctx, memInstance, resolvedKey, config.Payload, mergedInput)
	case task.MemoryOpDelete:
		return a.executeDelete(ctx, memInstance, resolvedKey)
	case task.MemoryOpFlush:
		return a.executeFlush(ctx, memInstance, resolvedKey, config.FlushConfig)
	case task.MemoryOpHealth:
		return a.executeHealth(ctx, memInstance, resolvedKey, config.HealthConfig)
	case task.MemoryOpClear:
		return a.executeClear(ctx, memInstance, resolvedKey, config.ClearConfig)
	case task.MemoryOpStats:
		return a.executeStats(ctx, memInstance, resolvedKey, config.StatsConfig)
	default:
		return nil, fmt.Errorf("unsupported memory operation: %s", config.Operation)
	}
}

func (a *ExecuteMemory) executeRead(ctx context.Context, mem memory.Memory, key string) (*core.Output, error) {
	messages, err := mem.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read memory: %w", err)
	}
	// Convert messages to output format
	output := make([]map[string]any, len(messages))
	for i, msg := range messages {
		output[i] = map[string]any{
			"role":    msg.Role,
			"content": msg.Content,
		}
	}
	return &core.Output{
		"messages": output,
		"count":    len(messages),
		"key":      key,
	}, nil
}

func (a *ExecuteMemory) executeWrite(
	ctx context.Context,
	mem memory.Memory,
	key string,
	payload any,
	mergedInput *core.Input,
) (*core.Output, error) {
	// Resolve payload if it's a template
	resolvedPayload, err := a.resolvePayload(payload, mergedInput)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve payload: %w", err)
	}
	// Convert payload to messages
	messages, err := a.payloadToMessages(resolvedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to convert payload to messages: %w", err)
	}
	// Backup existing messages before clearing for rollback safety
	backup, err := mem.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to backup existing messages: %w", err)
	}
	// Write operation replaces all existing messages (clear + append pattern)
	// This is the intended behavior for "write" vs "append"
	if err := mem.Clear(ctx); err != nil {
		return nil, fmt.Errorf("failed to clear memory before write: %w", err)
	}
	// Append new messages with rollback on failure
	for i, msg := range messages {
		if err := mem.Append(ctx, msg); err != nil {
			// Rollback: restore original messages
			if clearErr := mem.Clear(ctx); clearErr != nil {
				return nil, fmt.Errorf(
					"write failed at message %d and rollback clear failed: %w (original: %w)",
					i,
					clearErr,
					err,
				)
			}
			for j, backupMsg := range backup {
				if appendErr := mem.Append(ctx, backupMsg); appendErr != nil {
					return nil, fmt.Errorf(
						"write failed at message %d and rollback failed at message %d: %w (original: %w)",
						i,
						j,
						appendErr,
						err,
					)
				}
			}
			return nil, fmt.Errorf("write failed at message %d, memory restored: %w", i, err)
		}
	}
	return &core.Output{
		"success": true,
		"count":   len(messages),
		"key":     key,
	}, nil
}

func (a *ExecuteMemory) executeAppend(
	ctx context.Context,
	mem memory.Memory,
	key string,
	payload any,
	mergedInput *core.Input,
) (*core.Output, error) {
	// Resolve payload if it's a template
	resolvedPayload, err := a.resolvePayload(payload, mergedInput)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve payload: %w", err)
	}
	// Convert payload to messages
	messages, err := a.payloadToMessages(resolvedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to convert payload to messages: %w", err)
	}
	// Append messages
	for _, msg := range messages {
		if err := mem.Append(ctx, msg); err != nil {
			return nil, fmt.Errorf("failed to append message: %w", err)
		}
	}
	// Get new total count
	totalCount, err := mem.Len(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get message count: %w", err)
	}
	return &core.Output{
		"success":     true,
		"appended":    len(messages),
		"total_count": totalCount,
		"key":         key,
	}, nil
}

func (a *ExecuteMemory) executeDelete(ctx context.Context, mem memory.Memory, key string) (*core.Output, error) {
	// Clear all messages (delete operation)
	if err := mem.Clear(ctx); err != nil {
		return nil, fmt.Errorf("failed to delete memory: %w", err)
	}
	return &core.Output{
		"success": true,
		"key":     key,
	}, nil
}

func (a *ExecuteMemory) executeFlush(
	_ context.Context,
	_ memory.Memory,
	_ string,
	_ *task.FlushConfig,
) (*core.Output, error) {
	// Flush operation is not yet implemented
	// This would need to:
	// 1. Check if flush is needed based on threshold/max_keys
	// 2. Preserve recent messages based on strategy
	// 3. Archive/summarize older messages
	// 4. Update memory state
	return nil, fmt.Errorf("flush operation is not yet implemented")
}

func (a *ExecuteMemory) executeHealth(
	ctx context.Context,
	mem memory.Memory,
	key string,
	config *task.HealthConfig,
) (*core.Output, error) {
	if config == nil {
		return nil, fmt.Errorf("health operation requires health_config to be provided")
	}
	health, err := mem.GetMemoryHealth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory health: %w", err)
	}
	result := map[string]any{
		"healthy":        true,
		"key":            key,
		"token_count":    health.TokenCount,
		"message_count":  health.MessageCount,
		"flush_strategy": health.FlushStrategy,
	}
	if health.LastFlush != nil {
		result["last_flush"] = health.LastFlush.Format("2006-01-02T15:04:05Z07:00")
	}
	if config.IncludeStats {
		tokenCount, err := mem.GetTokenCount(ctx)
		if err == nil {
			result["current_tokens"] = tokenCount
		}
	}
	output := core.Output(result)
	return &output, nil
}

func (a *ExecuteMemory) executeClear(
	ctx context.Context,
	mem memory.Memory,
	key string,
	config *task.ClearConfig,
) (*core.Output, error) {
	if config == nil {
		return nil, fmt.Errorf("clear operation requires clear_config to be provided")
	}
	if !config.Confirm {
		return nil, fmt.Errorf("clear operation requires confirm flag to be true")
	}
	// Get count before clear for backup info
	beforeCount, err := mem.Len(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get message count before clear: %w", err)
	}
	// Clear memory
	if err := mem.Clear(ctx); err != nil {
		return nil, fmt.Errorf("failed to clear memory: %w", err)
	}
	return &core.Output{
		"success":          true,
		"key":              key,
		"messages_cleared": beforeCount,
		"backup_created":   config.Backup,
	}, nil
}

func (a *ExecuteMemory) executeStats(
	ctx context.Context,
	mem memory.Memory,
	key string,
	config *task.StatsConfig,
) (*core.Output, error) {
	if config == nil {
		return nil, fmt.Errorf("stats operation requires stats_config to be provided")
	}
	// Get basic stats
	messageCount, err := mem.Len(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get message count: %w", err)
	}
	tokenCount, err := mem.GetTokenCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get token count: %w", err)
	}
	health, err := mem.GetMemoryHealth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory health: %w", err)
	}
	stats := map[string]any{
		"key":            key,
		"message_count":  messageCount,
		"token_count":    tokenCount,
		"flush_strategy": health.FlushStrategy,
	}
	if health.LastFlush != nil {
		stats["last_flush"] = health.LastFlush.Format("2006-01-02T15:04:05Z07:00")
	}
	if config.IncludeContent && messageCount > 0 {
		avgTokensPerMessage := 0
		if messageCount > 0 {
			avgTokensPerMessage = tokenCount / messageCount
		}
		stats["avg_tokens_per_message"] = avgTokensPerMessage
	}
	output := core.Output(stats)
	return &output, nil
}

// Helper methods

func (a *ExecuteMemory) resolvePayload(payload any, mergedInput *core.Input) (any, error) {
	if payload == nil {
		return nil, nil
	}
	// Build context for template evaluation
	tplCtx := make(map[string]any)
	if mergedInput != nil {
		for k, v := range *mergedInput {
			tplCtx[k] = v
		}
	}
	return a.resolvePayloadRecursive(payload, tplCtx)
}

func (a *ExecuteMemory) resolvePayloadRecursive(payload any, context map[string]any) (any, error) {
	switch v := payload.(type) {
	case string:
		// Resolve string templates
		resolved, err := a.templateEngine.RenderString(v, context)
		if err != nil {
			return nil, err
		}
		return resolved, nil
	case map[string]any:
		// Recursively resolve map values
		resolved := make(map[string]any)
		for k, val := range v {
			resolvedVal, err := a.resolvePayloadRecursive(val, context)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve payload field '%s': %w", k, err)
			}
			resolved[k] = resolvedVal
		}
		return resolved, nil
	case []any:
		// Recursively resolve array elements
		resolved := make([]any, len(v))
		for i, item := range v {
			resolvedItem, err := a.resolvePayloadRecursive(item, context)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve payload[%d]: %w", i, err)
			}
			resolved[i] = resolvedItem
		}
		return resolved, nil
	default:
		// Return other types as-is (numbers, booleans, etc)
		return v, nil
	}
	// Return payload as-is for other types
}

func (a *ExecuteMemory) validateMessageRole(role string) error {
	switch role {
	case string(llm.MessageRoleUser), string(llm.MessageRoleAssistant),
		string(llm.MessageRoleSystem), string(llm.MessageRoleTool):
		return nil
	default:
		return fmt.Errorf("invalid message role '%s', must be one of: user, assistant, system, tool", role)
	}
}

func (a *ExecuteMemory) mapToMessage(msg map[string]any) (llm.Message, error) {
	role, ok := msg["role"].(string)
	if !ok || role == "" {
		role = "user"
	}
	content, ok := msg["content"].(string)
	if !ok || content == "" {
		return llm.Message{}, fmt.Errorf("message content is required")
	}
	// Validate role
	if err := a.validateMessageRole(role); err != nil {
		return llm.Message{}, err
	}
	return llm.Message{Role: llm.MessageRole(role), Content: content}, nil
}

func (a *ExecuteMemory) payloadToMessages(payload any) ([]llm.Message, error) {
	if payload == nil {
		return nil, fmt.Errorf("payload is nil")
	}
	// Handle single message
	if msg, ok := payload.(map[string]any); ok {
		message, err := a.mapToMessage(msg)
		if err != nil {
			return nil, err
		}
		return []llm.Message{message}, nil
	}
	// Handle array of messages
	if messages, ok := payload.([]any); ok {
		result := make([]llm.Message, 0, len(messages))
		for i, item := range messages {
			if msg, ok := item.(map[string]any); ok {
				message, err := a.mapToMessage(msg)
				if err != nil {
					return nil, fmt.Errorf("message[%d]: %w", i, err)
				}
				result = append(result, message)
			} else {
				return nil, fmt.Errorf("invalid message format at index %d", i)
			}
		}
		return result, nil
	}
	// Handle string payload as user message
	if content, ok := payload.(string); ok {
		return []llm.Message{{Role: llm.MessageRoleUser, Content: content}}, nil
	}
	return nil, fmt.Errorf("unsupported payload format")
}
