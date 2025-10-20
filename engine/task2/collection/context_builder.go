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
	// Start with base context
	normCtx := b.BaseContextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	// Collection tasks will have item and index added during iteration
	// This is just the base context for the collection itself
	return normCtx
}

// BuildIterationContext creates a context for a specific iteration in a collection
func (b *ContextBuilder) BuildIterationContext(
	baseContext *shared.NormalizationContext,
	item any,
	index int,
) (*shared.NormalizationContext, error) {
	// Create a new context for the iteration
	iterCtx := &shared.NormalizationContext{
		WorkflowState:  baseContext.WorkflowState,
		WorkflowConfig: baseContext.WorkflowConfig,
		TaskConfig:     baseContext.TaskConfig,
		ParentTask:     baseContext.ParentTask,
		TaskConfigs:    baseContext.TaskConfigs,
		ChildrenIndex:  baseContext.ChildrenIndex,
		MergedEnv:      baseContext.MergedEnv,
	}
	// Copy variables from base context
	if baseContext.Variables != nil {
		vars, err := core.DeepCopy(baseContext.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to deep copy variables: %w", err)
		}
		iterCtx.Variables = vars
	} else {
		iterCtx.Variables = make(map[string]any)
	}

	// Add item and index to variables
	iterCtx.Variables[shared.ItemKey] = item
	iterCtx.Variables[shared.IndexKey] = index
	// Create current input by deep-copying parent context, then override with item and index
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
	// Then add/override with item and index
	currentInput[shared.ItemKey] = item
	currentInput[shared.IndexKey] = index
	iterCtx.CurrentInput = &currentInput
	// Also add to input variable
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
	// First build the standard iteration context
	iterCtx, err := b.BuildIterationContext(baseContext, item, index)
	if err != nil {
		return nil, err
	}

	// Add progress context if provided
	if progressState != nil {
		progressCtx := shared.BuildProgressContext(ctx, progressState)
		iterCtx.Variables["progress"] = progressCtx
	}

	return iterCtx, nil
}

// EnrichContext adds collection-specific data to an existing context
func (b *ContextBuilder) EnrichContext(ctx *shared.NormalizationContext, taskState *task.State) error {
	// First apply base enrichment
	if err := b.BaseContextBuilder.EnrichContext(ctx, taskState); err != nil {
		return err
	}
	// Collection tasks might need special handling for aggregated outputs
	// This is handled during output transformation
	return nil
}

// ValidateContext ensures the context has all required fields for collection tasks
func (b *ContextBuilder) ValidateContext(ctx *shared.NormalizationContext) error {
	// First apply base validation
	if err := b.BaseContextBuilder.ValidateContext(ctx); err != nil {
		return err
	}
	// Validate collection-specific requirements
	if ctx.TaskConfig != nil && ctx.TaskConfig.Type == task.TaskTypeCollection {
		// Check if items field is present
		if ctx.TaskConfig.Items == "" {
			return fmt.Errorf("collection task config missing items field")
		}
	}
	return nil
}
