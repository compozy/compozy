package core

import (
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

// ConfigNormalizer orchestrates the entire normalization process
type ConfigNormalizer struct {
	factory        NormalizerFactory
	envMerger      *EnvMerger
	contextBuilder *shared.ContextBuilder
}

// NewConfigNormalizer creates a new config normalizer
func NewConfigNormalizer(
	factory NormalizerFactory,
	envMerger *EnvMerger,
	contextBuilder *shared.ContextBuilder,
) *ConfigNormalizer {
	return &ConfigNormalizer{
		factory:        factory,
		envMerger:      envMerger,
		contextBuilder: contextBuilder,
	}
}

// NormalizeTask normalizes a task configuration with workflow context
func (cn *ConfigNormalizer) NormalizeTask(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) error {
	// Build normalization context
	ctx := cn.contextBuilder.BuildContext(workflowState, workflowConfig, taskConfig)

	// Merge environments
	ctx.MergedEnv = cn.envMerger.MergeWorkflowToTask(workflowConfig, taskConfig)

	// Create appropriate normalizer based on task type
	normalizer, err := cn.factory.CreateNormalizer(taskConfig.Type)
	if err != nil {
		return fmt.Errorf("failed to create normalizer for task %s: %w", taskConfig.ID, err)
	}

	// Apply normalization
	if err := normalizer.Normalize(taskConfig, ctx); err != nil {
		return fmt.Errorf("failed to normalize task %s: %w", taskConfig.ID, err)
	}

	return nil
}

// NormalizeAllTasks normalizes all tasks in a workflow
func (cn *ConfigNormalizer) NormalizeAllTasks(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
) error {
	// Build task configs map for context
	taskConfigs := make(map[string]*task.Config)
	for i := range workflowConfig.Tasks {
		taskConfigs[workflowConfig.Tasks[i].ID] = &workflowConfig.Tasks[i]
	}

	// Normalize each task
	for i := range workflowConfig.Tasks {
		taskConfig := &workflowConfig.Tasks[i]

		// Build normalization context with task configs
		ctx := cn.contextBuilder.BuildContext(workflowState, workflowConfig, taskConfig)
		ctx.TaskConfigs = taskConfigs
		ctx.MergedEnv = cn.envMerger.MergeWorkflowToTask(workflowConfig, taskConfig)

		// Create appropriate normalizer
		normalizer, err := cn.factory.CreateNormalizer(taskConfig.Type)
		if err != nil {
			return fmt.Errorf("failed to create normalizer for task %s: %w", taskConfig.ID, err)
		}

		// Apply normalization
		if err := normalizer.Normalize(taskConfig, ctx); err != nil {
			return fmt.Errorf("failed to normalize task %s: %w", taskConfig.ID, err)
		}
	}

	return nil
}

// NormalizeSubTask normalizes a sub-task within a parent task context
func (cn *ConfigNormalizer) NormalizeSubTask(
	parentCtx *shared.NormalizationContext,
	parentTask *task.Config,
	subTask *task.Config,
) error {
	// Validate inputs
	if parentCtx == nil {
		return fmt.Errorf("parent context is nil")
	}
	if parentTask == nil {
		return fmt.Errorf("parent task is nil")
	}
	if subTask == nil {
		return fmt.Errorf("sub-task is nil")
	}

	// Build sub-task context
	ctx, err := cn.contextBuilder.BuildNormalizationSubTaskContext(parentCtx, parentTask, subTask)
	if err != nil {
		return fmt.Errorf("failed to build sub-task context: %w", err)
	}

	// Merge environments for sub-task
	ctx.MergedEnv = cn.envMerger.MergeThreeLevels(
		parentCtx.WorkflowConfig,
		subTask,
		nil,
	)

	// Create appropriate normalizer
	normalizer, err := cn.factory.CreateNormalizer(subTask.Type)
	if err != nil {
		return fmt.Errorf("failed to create normalizer for sub-task %s: %w", subTask.ID, err)
	}

	// Apply normalization
	if err := normalizer.Normalize(subTask, ctx); err != nil {
		return fmt.Errorf("failed to normalize sub-task %s: %w", subTask.ID, err)
	}

	return nil
}
