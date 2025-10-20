package collection

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

// ContextBuilder builds contexts for collection tasks
type ContextBuilder struct {
	*shared.BaseContextBuilder
}

// NewContextBuilder creates a new collection task context builder
func NewContextBuilder(ctx context.Context) *ContextBuilder {
	return &ContextBuilder{
		BaseContextBuilder: shared.NewBaseContextBuilder(ctx),
	}
}

// TaskType returns the type of task this builder handles
func (b *ContextBuilder) TaskType() task.Type {
	return task.TaskTypeCollection
}

// BuildContext creates a normalization context for collection tasks
func (b *ContextBuilder) BuildContext(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) *shared.NormalizationContext {
	normCtx := b.BaseContextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	return normCtx
}

// BuildIterationContext creates a context for a specific iteration in a collection
func (b *ContextBuilder) BuildIterationContext(
	baseContext *shared.NormalizationContext,
	item any,
	index int,
) (*shared.NormalizationContext, error) {
	iterCtx := &shared.NormalizationContext{
		WorkflowState:  baseContext.WorkflowState,
		WorkflowConfig: baseContext.WorkflowConfig,
		TaskConfig:     baseContext.TaskConfig,
		ParentTask:     baseContext.ParentTask,
		TaskConfigs:    baseContext.TaskConfigs,
		ChildrenIndex:  baseContext.ChildrenIndex,
		MergedEnv:      baseContext.MergedEnv,
	}
	if baseContext.Variables != nil {
		vars, err := core.DeepCopy(baseContext.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to deep copy variables: %w", err)
		}
		iterCtx.Variables = vars
	} else {
		iterCtx.Variables = make(map[string]any)
	}
	iterCtx.Variables[shared.ItemKey] = item
	iterCtx.Variables[shared.IndexKey] = index
	var currentInput core.Input
	if baseContext.CurrentInput != nil {
		copied, err := core.DeepCopy(*baseContext.CurrentInput)
		if err != nil {
			return nil, fmt.Errorf("failed to deep copy current input: %w", err)
		}
		currentInput = copied
	} else {
		currentInput = make(core.Input)
	}
	currentInput[shared.ItemKey] = item
	currentInput[shared.IndexKey] = index
	iterCtx.CurrentInput = &currentInput
	iterCtx.Variables[shared.InputKey] = &currentInput
	return iterCtx, nil
}

// BuildIterationContextWithProgress creates a context for a specific iteration including progress information
func (b *ContextBuilder) BuildIterationContextWithProgress(
	ctx context.Context,
	baseContext *shared.NormalizationContext,
	item any,
	index int,
	progressState *task.ProgressState,
) (*shared.NormalizationContext, error) {
	iterCtx, err := b.BuildIterationContext(baseContext, item, index)
	if err != nil {
		return nil, err
	}
	if progressState != nil {
		progressCtx := shared.BuildProgressContext(ctx, progressState)
		iterCtx.Variables["progress"] = progressCtx
	}
	return iterCtx, nil
}

// EnrichContext adds collection-specific data to an existing context
func (b *ContextBuilder) EnrichContext(ctx *shared.NormalizationContext, taskState *task.State) error {
	if err := b.BaseContextBuilder.EnrichContext(ctx, taskState); err != nil {
		return err
	}
	return nil
}

// ValidateContext ensures the context has all required fields for collection tasks
func (b *ContextBuilder) ValidateContext(ctx *shared.NormalizationContext) error {
	if err := b.BaseContextBuilder.ValidateContext(ctx); err != nil {
		return err
	}
	if ctx.TaskConfig != nil && ctx.TaskConfig.Type == task.TaskTypeCollection {
		if ctx.TaskConfig.Items == "" {
			return fmt.Errorf("collection task config missing items field")
		}
	}
	return nil
}
