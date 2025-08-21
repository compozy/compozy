package shared

import (
	"fmt"
	"maps"

	"github.com/compozy/compozy/engine/core"
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
	normCtx, err := n.validateAndPrepareContext(config, ctx)
	if err != nil {
		return err
	}
	if config == nil {
		return nil
	}
	// Normalize parent task configuration
	if err := n.normalizeParentConfig(config, normCtx); err != nil {
		return err
	}
	// Normalize all sub-tasks with parent context
	if err := n.normalizeSubTasks(config, normCtx); err != nil {
		return fmt.Errorf("failed to normalize %s sub-tasks: %w", n.taskTypeName, err)
	}
	return nil
}

// validateAndPrepareContext validates the context and configuration
func (n *BaseSubTaskNormalizer) validateAndPrepareContext(
	config *task.Config,
	ctx contracts.NormalizationContext,
) (*NormalizationContext, error) {
	normCtx, ok := ctx.(*NormalizationContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type: expected *NormalizationContext, got %T", ctx)
	}
	if config != nil && config.Type != n.taskType {
		return nil, fmt.Errorf("%s normalizer cannot handle task type: %s", n.taskTypeName, config.Type)
	}
	return normCtx, nil
}

// normalizeParentConfig normalizes the parent task configuration
func (n *BaseSubTaskNormalizer) normalizeParentConfig(config *task.Config, normCtx *NormalizationContext) error {
	context := normCtx.BuildTemplateContext()
	configMap, err := config.AsMap()
	if err != nil {
		return fmt.Errorf("failed to convert task config to map: %w", err)
	}
	if n.templateEngine == nil {
		return fmt.Errorf("template engine is required for normalization")
	}
	parsed, err := n.templateEngine.ParseMapWithFilter(configMap, context, n.shouldSkipField)
	if err != nil {
		return fmt.Errorf("failed to normalize %s task config: %w", n.taskTypeName, err)
	}
	if err := config.FromMap(parsed); err != nil {
		return fmt.Errorf("failed to update %s task config from normalized map: %w", n.taskTypeName, err)
	}
	return nil
}

// shouldSkipField determines if a field should be skipped during normalization
func (n *BaseSubTaskNormalizer) shouldSkipField(k string) bool {
	return k == AgentKey || k == ToolKey || k == TasksKey || k == OutputsKey || k == InputKey || k == OutputKey
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

// InheritTaskConfig is a shared utility function that copies relevant config fields from parent to child config
// InheritTaskConfig copies non-empty parent task fields into a child task when the
// child does not define them, and propagates the child's CWD to any nested tasks.
//
// If either child or parent is nil the function is a no-op. It will:
//   - copy parent.CWD to child.CWD when child.CWD is nil and then call
//     task.PropagateSingleTaskCWD to propagate that CWD to nested tasks;
//   - copy parent.FilePath to child.FilePath when child.FilePath is empty.
//
// Returns an error if CWD propagation fails.
func InheritTaskConfig(child, parent *task.Config) error {
	if child == nil || parent == nil {
		return nil // Graceful handling of nil configs
	}
	// Copy CWD from parent config to child config if not already set
	if child.CWD == nil && parent.CWD != nil {
		child.CWD = parent.CWD
		// Recursively propagate CWD to all nested tasks
		if err := task.PropagateSingleTaskCWD(child, child.CWD, "sub-task"); err != nil {
			return fmt.Errorf("failed to propagate CWD to nested tasks: %w", err)
		}
	}
	// Copy FilePath from parent config to child config if not already set
	if child.FilePath == "" && parent.FilePath != "" {
		child.FilePath = parent.FilePath
	}
	return nil
}

// normalizeSingleSubTask normalizes a single sub-task with proper context setup
func (n *BaseSubTaskNormalizer) normalizeSingleSubTask(
	subTask *task.Config,
	parentConfig *task.Config,
	ctx *NormalizationContext,
) error {
	// Apply config inheritance before template processing
	if err := InheritTaskConfig(subTask, parentConfig); err != nil {
		return err
	}

	// Prepare sub-task context
	subTaskCtx, err := n.prepareSubTaskContext(subTask, parentConfig, ctx)
	if err != nil {
		return err
	}

	// Get normalizer for sub-task type
	subNormalizer, err := n.normalizerFactory.CreateNormalizer(subTask.Type)
	if err != nil {
		return fmt.Errorf("failed to create normalizer for task type %s: %w", subTask.Type, err)
	}

	// Recursively normalize the sub-task (this handles nested tasks too)
	return subNormalizer.Normalize(subTask, subTaskCtx)
}

// prepareSubTaskContext prepares the normalization context for a sub-task
func (n *BaseSubTaskNormalizer) prepareSubTaskContext(
	subTask *task.Config,
	parentConfig *task.Config,
	ctx *NormalizationContext,
) (*NormalizationContext, error) {
	subTaskCtx, err := n.contextBuilder.BuildNormalizationSubTaskContext(ctx, parentConfig, subTask)
	if err != nil {
		return nil, fmt.Errorf("failed to build sub-task context: %w", err)
	}
	// Merge parent input with sub-task's With instead of overwriting
	// This ensures parent-provided input (from collection.with) is accessible
	if subTask.With != nil {
		if subTaskCtx.CurrentInput != nil {
			// Merge parent input with sub-task's With
			mergedInput := make(core.Input)
			// First copy parent input
			maps.Copy(mergedInput, *subTaskCtx.CurrentInput)
			// Then overlay sub-task's With
			maps.Copy(mergedInput, *subTask.With)
			subTaskCtx.CurrentInput = &mergedInput
		} else {
			subTaskCtx.CurrentInput = subTask.With
		}
	}
	if ctx.MergedEnv != nil {
		subTaskCtx.MergedEnv = ctx.MergedEnv
	}
	return subTaskCtx, nil
}
