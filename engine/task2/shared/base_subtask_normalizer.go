package shared

import (
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/pkg/tplengine"
)

// BaseSubTaskNormalizer provides common functionality for normalizers that handle sub-tasks
// This eliminates code duplication between parallel and composite normalizers
type BaseSubTaskNormalizer struct {
	templateEngine    *tplengine.TemplateEngine
	contextBuilder    *ContextBuilder
	normalizerFactory contracts.NormalizerFactory
	taskType          task.Type
	taskTypeName      string
}

// NewBaseSubTaskNormalizer creates a new base sub-task normalizer
func NewBaseSubTaskNormalizer(
	templateEngine *tplengine.TemplateEngine,
	contextBuilder *ContextBuilder,
	normalizerFactory contracts.NormalizerFactory,
	taskType task.Type,
	taskTypeName string,
) *BaseSubTaskNormalizer {
	return &BaseSubTaskNormalizer{
		templateEngine:    templateEngine,
		contextBuilder:    contextBuilder,
		normalizerFactory: normalizerFactory,
		taskType:          taskType,
		taskTypeName:      taskTypeName,
	}
}

// Type returns the task type this normalizer handles
func (n *BaseSubTaskNormalizer) Type() task.Type {
	return n.taskType
}

// Normalize applies common sub-task normalization rules
func (n *BaseSubTaskNormalizer) Normalize(config *task.Config, ctx contracts.NormalizationContext) error {
	// Type assert to get the concrete type
	normCtx, ok := ctx.(*NormalizationContext)
	if !ok {
		return fmt.Errorf("invalid context type: expected *NormalizationContext, got %T", ctx)
	}
	if config == nil {
		return nil
	}
	if config.Type != n.taskType {
		return fmt.Errorf("%s normalizer cannot handle task type: %s", n.taskTypeName, config.Type)
	}
	// Build template context
	context := normCtx.BuildTemplateContext()
	// Convert config to map for template processing
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	// First normalize the task itself (excluding the tasks field)
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, func(k string) bool {
		return k == AgentKey || k == ToolKey || k == TasksKey || k == OutputsKey || k == InputKey || k == OutputKey
	})
	if err != nil {
		return fmt.Errorf("failed to normalize %s task config: %w", n.taskTypeName, err)
	}
	// Update config from normalized map
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update %s task config from normalized map: %w", n.taskTypeName, err)
	}
	// Now normalize each sub-task with the parent task as context
	if err := n.normalizeSubTasks(config, normCtx); err != nil {
		return fmt.Errorf("failed to normalize %s sub-tasks: %w", n.taskTypeName, err)
	}
	return nil
}

// normalizeSubTasks normalizes sub-tasks within a parent task with proper parent context
func (n *BaseSubTaskNormalizer) normalizeSubTasks(parentConfig *task.Config, ctx *NormalizationContext) error {
	// Normalize each sub-task in the Tasks array
	for i := range parentConfig.Tasks {
		subTask := &parentConfig.Tasks[i]
		if err := n.normalizeSingleSubTask(subTask, parentConfig, ctx); err != nil {
			return fmt.Errorf("failed to normalize sub-task %s: %w", subTask.ID, err)
		}
		parentConfig.Tasks[i] = *subTask
	}

	// Also normalize the task reference if present
	if parentConfig.Task != nil {
		if err := n.normalizeSingleSubTask(parentConfig.Task, parentConfig, ctx); err != nil {
			return fmt.Errorf("failed to normalize task reference: %w", err)
		}
	}
	return nil
}

// normalizeSingleSubTask normalizes a single sub-task with proper context setup
func (n *BaseSubTaskNormalizer) normalizeSingleSubTask(
	subTask *task.Config,
	parentConfig *task.Config,
	ctx *NormalizationContext,
) error {
	// Create sub-task context with parent task as context
	subTaskCtx, err := n.contextBuilder.BuildNormalizationSubTaskContext(ctx, parentConfig, subTask)
	if err != nil {
		return fmt.Errorf("failed to build sub-task context: %w", err)
	}

	// Get normalizer for sub-task type
	subNormalizer, err := n.normalizerFactory.CreateNormalizer(subTask.Type)
	if err != nil {
		return fmt.Errorf("failed to create normalizer for task type %s: %w", subTask.Type, err)
	}

	// Set current input if available
	if subTask.With != nil {
		subTaskCtx.CurrentInput = subTask.With
	}

	// Merge environment
	if ctx.MergedEnv != nil {
		subTaskCtx.MergedEnv = ctx.MergedEnv
	}

	// Recursively normalize the sub-task (this handles nested tasks too)
	return subNormalizer.Normalize(subTask, subTaskCtx)
}
