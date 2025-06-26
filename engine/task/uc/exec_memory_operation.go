package uc

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	memcore "github.com/compozy/compozy/engine/memory/core"
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
}

func NewExecuteMemoryOperation(
	memoryManager memcore.ManagerInterface,
	templateEngine *tplengine.TemplateEngine,
) *ExecuteMemoryOperation {
	return &ExecuteMemoryOperation{
		memoryManager:  memoryManager,
		templateEngine: templateEngine,
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
	// TODO: Implement wildcard key pattern support using config.MaxKeys for safety limits
	// TODO: Implement batch processing using config.BatchSize for operations like stats
	// Get resolved key once for efficiency
	resolvedKey := memInstance.GetID()
	// Execute operation based on type
	switch input.TaskConfig.Operation {
	case task.MemoryOpRead:
		return uc.executeRead(ctx, memInstance, resolvedKey)
	case task.MemoryOpWrite:
		return uc.executeWrite(
			ctx,
			memInstance,
			resolvedKey,
			input.TaskConfig.Payload,
			input.MergedInput,
			input.WorkflowState,
		)
	case task.MemoryOpAppend:
		return uc.executeAppend(
			ctx,
			memInstance,
			resolvedKey,
			input.TaskConfig.Payload,
			input.MergedInput,
			input.WorkflowState,
		)
	case task.MemoryOpDelete:
		return uc.executeDelete(ctx, memInstance, resolvedKey)
	case task.MemoryOpFlush:
		return uc.executeFlush(ctx, memInstance, resolvedKey, input.TaskConfig.FlushConfig)
	case task.MemoryOpHealth:
		return uc.executeHealth(ctx, memInstance, resolvedKey, input.TaskConfig.HealthConfig)
	case task.MemoryOpClear:
		return uc.executeClear(ctx, memInstance, resolvedKey, input.TaskConfig.ClearConfig)
	case task.MemoryOpStats:
		return uc.executeStats(ctx, memInstance, resolvedKey, input.TaskConfig.StatsConfig)
	default:
		return nil, fmt.Errorf("unsupported memory operation: %s", input.TaskConfig.Operation)
	}
}

func (uc *ExecuteMemoryOperation) executeRead(
	ctx context.Context,
	mem memcore.Memory,
	key string,
) (*core.Output, error) {
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

// MemoryTransaction provides transactional operations for memory modifications
type MemoryTransaction struct {
	mem     memcore.Memory
	backup  []llm.Message
	cleared bool
}

// NewMemoryTransaction creates a new memory transaction
func NewMemoryTransaction(mem memcore.Memory) *MemoryTransaction {
	return &MemoryTransaction{
		mem: mem,
	}
}

// Begin starts the transaction by backing up current state
func (t *MemoryTransaction) Begin(ctx context.Context) error {
	// Backup current messages
	backup, err := t.mem.Read(ctx)
	if err != nil {
		return fmt.Errorf("failed to backup messages: %w", err)
	}
	t.backup = backup
	return nil
}

// Clear clears the memory and marks it for potential rollback
func (t *MemoryTransaction) Clear(ctx context.Context) error {
	if err := t.mem.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear memory: %w", err)
	}
	t.cleared = true
	return nil
}

// Commit finalizes the transaction (no-op for successful operations)
func (t *MemoryTransaction) Commit() error {
	// Reset state
	t.backup = nil
	t.cleared = false
	return nil
}

// Rollback restores the original state
func (t *MemoryTransaction) Rollback(ctx context.Context) error {
	if !t.cleared || t.backup == nil {
		return nil // Nothing to rollback
	}

	// Clear any partial state
	if err := t.mem.Clear(ctx); err != nil {
		return fmt.Errorf("rollback clear failed: %w", err)
	}

	// Restore backup messages
	for i, msg := range t.backup {
		if err := t.mem.Append(ctx, msg); err != nil {
			return fmt.Errorf("rollback failed at message %d: %w", i, err)
		}
	}

	return nil
}

// ApplyMessages appends messages within the transaction
func (t *MemoryTransaction) ApplyMessages(ctx context.Context, messages []llm.Message) error {
	for i, msg := range messages {
		if err := t.mem.Append(ctx, msg); err != nil {
			return fmt.Errorf("failed to append message %d: %w", i, err)
		}
	}
	return nil
}

func (uc *ExecuteMemoryOperation) executeWrite(
	ctx context.Context,
	mem memcore.Memory,
	key string,
	payload any,
	mergedInput *core.Input,
	workflowState *workflow.State,
) (*core.Output, error) {
	// Resolve payload if it's a template
	resolvedPayload, err := uc.resolvePayload(payload, mergedInput, workflowState)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve payload: %w", err)
	}
	// Convert payload to messages
	messages, err := uc.payloadToMessages(resolvedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to convert payload to messages: %w", err)
	}

	// Use transaction for atomic write operation
	tx := NewMemoryTransaction(mem)

	// Begin transaction
	if err := tx.Begin(ctx); err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Clear existing messages (write replaces all)
	if err := tx.Clear(ctx); err != nil {
		return nil, fmt.Errorf("failed to clear memory: %w", err)
	}

	// Apply new messages
	if err := tx.ApplyMessages(ctx, messages); err != nil {
		// Rollback on failure
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return nil, fmt.Errorf("write failed and rollback failed: %w (original: %w)", rollbackErr, err)
		}
		return nil, fmt.Errorf("write failed, memory restored: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &core.Output{
		"success": true,
		"count":   len(messages),
		"key":     key,
	}, nil
}

func (uc *ExecuteMemoryOperation) executeAppend(
	ctx context.Context,
	mem memcore.Memory,
	key string,
	payload any,
	mergedInput *core.Input,
	workflowState *workflow.State,
) (*core.Output, error) {
	// Resolve payload if it's a template
	resolvedPayload, err := uc.resolvePayload(payload, mergedInput, workflowState)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve payload: %w", err)
	}
	// Convert payload to messages
	messages, err := uc.payloadToMessages(resolvedPayload)
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

func (uc *ExecuteMemoryOperation) executeDelete(
	ctx context.Context,
	mem memcore.Memory,
	key string,
) (*core.Output, error) {
	// Clear all messages (delete operation)
	if err := mem.Clear(ctx); err != nil {
		return nil, fmt.Errorf("failed to delete memory: %w", err)
	}
	return &core.Output{
		"success": true,
		"key":     key,
	}, nil
}

func (uc *ExecuteMemoryOperation) executeFlush(
	ctx context.Context,
	mem memcore.Memory,
	key string,
	config *task.FlushConfig,
) (*core.Output, error) {
	// Check if memory implements FlushableMemory interface
	flushableMem, ok := mem.(memcore.FlushableMemory)
	if !ok {
		return nil, fmt.Errorf("memory instance does not support flush operations")
	}

	// Validate flush config if provided
	if config != nil {
		if config.Threshold < 0 || config.Threshold > 1 {
			return nil, fmt.Errorf("flush threshold must be between 0 and 1, got %f", config.Threshold)
		}
	}

	// Check if we're in dry-run mode
	if config != nil && config.DryRun {
		// Get memory health to simulate what would happen
		health, err := mem.GetMemoryHealth(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get memory health for dry run: %w", err)
		}

		return &core.Output{
			"success":        true,
			"dry_run":        true,
			"key":            key,
			"would_flush":    health.TokenCount > 0, // Simplified check
			"token_count":    health.TokenCount,
			"message_count":  health.MessageCount,
			"flush_strategy": health.FlushStrategy,
		}, nil
	}

	// Perform the flush operation
	// Note: PerformFlush handles its own flush pending management internally
	result, err := flushableMem.PerformFlush(ctx)
	if err != nil {
		return nil, fmt.Errorf("flush operation failed: %w", err)
	}

	// Convert flush result to output
	output := &core.Output{
		"success":           result.Success,
		"key":               key,
		"summary_generated": result.SummaryGenerated,
		"message_count":     result.MessageCount,
		"token_count":       result.TokenCount,
	}

	// Add optional flush config details to output
	if config != nil {
		(*output)["force"] = config.Force
		if config.Strategy != "" {
			(*output)["strategy"] = config.Strategy
		}
		if config.MaxKeys > 0 {
			(*output)["max_keys"] = config.MaxKeys
		}
	}

	// Add error to output if present
	if result.Error != "" {
		(*output)["error"] = result.Error
		return output, fmt.Errorf("flush completed with error: %s", result.Error)
	}

	return output, nil
}

func (uc *ExecuteMemoryOperation) executeHealth(
	ctx context.Context,
	mem memcore.Memory,
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

func (uc *ExecuteMemoryOperation) executeClear(
	ctx context.Context,
	mem memcore.Memory,
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

func (uc *ExecuteMemoryOperation) executeStats(
	ctx context.Context,
	mem memcore.Memory,
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

func (uc *ExecuteMemoryOperation) resolvePayload(
	payload any,
	mergedInput *core.Input,
	workflowState *workflow.State,
) (any, error) {
	if payload == nil {
		return nil, nil
	}
	// Build context for template evaluation following project standards
	tplCtx := map[string]any{
		"workflow": map[string]any{
			"id":      workflowState.WorkflowID,
			"exec_id": workflowState.WorkflowExecID,
			"input":   workflowState.Input,
		},
		"tasks": workflowState.Tasks,
	}
	// Add merged input as "input" at top level for task context
	if mergedInput != nil {
		tplCtx["input"] = mergedInput
	}
	return uc.resolvePayloadRecursive(payload, tplCtx)
}

func (uc *ExecuteMemoryOperation) resolvePayloadRecursive(payload any, context map[string]any) (any, error) {
	switch v := payload.(type) {
	case string:
		// Resolve string templates
		resolved, err := uc.templateEngine.RenderString(v, context)
		if err != nil {
			return nil, err
		}
		return resolved, nil
	case map[string]any:
		// Recursively resolve map values
		resolved := make(map[string]any)
		for k, val := range v {
			resolvedVal, err := uc.resolvePayloadRecursive(val, context)
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
			resolvedItem, err := uc.resolvePayloadRecursive(item, context)
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

func (uc *ExecuteMemoryOperation) validateMessageRole(role string) error {
	switch role {
	case string(llm.MessageRoleUser), string(llm.MessageRoleAssistant),
		string(llm.MessageRoleSystem), string(llm.MessageRoleTool):
		return nil
	default:
		return fmt.Errorf("invalid message role '%s', must be one of: user, assistant, system, tool", role)
	}
}

func (uc *ExecuteMemoryOperation) mapToMessage(msg map[string]any) (llm.Message, error) {
	role, ok := msg["role"].(string)
	if !ok || role == "" {
		role = "user"
	}
	content, ok := msg["content"].(string)
	if !ok || content == "" {
		return llm.Message{}, fmt.Errorf("message content is required")
	}
	// Validate role
	if err := uc.validateMessageRole(role); err != nil {
		return llm.Message{}, err
	}
	return llm.Message{Role: llm.MessageRole(role), Content: content}, nil
}

func (uc *ExecuteMemoryOperation) payloadToMessages(payload any) ([]llm.Message, error) {
	if payload == nil {
		return nil, fmt.Errorf("payload is nil")
	}
	// Handle single message
	if msg, ok := payload.(map[string]any); ok {
		message, err := uc.mapToMessage(msg)
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
				message, err := uc.mapToMessage(msg)
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
