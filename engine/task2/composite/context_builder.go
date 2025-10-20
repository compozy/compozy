package composite

import (
	"context"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

// ContextBuilder builds contexts for composite tasks
type ContextBuilder struct {
	*shared.BaseContextBuilder
}

// NewContextBuilder creates a new composite task context builder
func NewContextBuilder(ctx context.Context) *ContextBuilder {
	return &ContextBuilder{
		BaseContextBuilder: shared.NewBaseContextBuilder(ctx),
	}
}

// TaskType returns the type of task this builder handles
func (b *ContextBuilder) TaskType() task.Type {
	return task.TaskTypeComposite
}

// BuildContext creates a normalization context for composite tasks
func (b *ContextBuilder) BuildContext(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) *shared.NormalizationContext {
	// Start with base context
	normCtx := b.BaseContextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
	// Composite tasks execute sub-tasks sequentially
	// Each sub-task will get its own context when normalized
	return normCtx
}

// EnrichContext adds composite-specific data to an existing context
func (b *ContextBuilder) EnrichContext(ctx *shared.NormalizationContext, taskState *task.State) error {
	// Use base enrichment - composite tasks don't need special enrichment
	return b.BaseContextBuilder.EnrichContext(ctx, taskState)
}

// ValidateContext ensures the context has all required fields for composite tasks
func (b *ContextBuilder) ValidateContext(ctx *shared.NormalizationContext) error {
	// Use base validation - composite tasks don't have special validation requirements
	return b.BaseContextBuilder.ValidateContext(ctx)
}
