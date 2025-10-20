package basic

import (
	"context"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

// ContextBuilder builds contexts for basic tasks
type ContextBuilder struct {
	*shared.BaseContextBuilder
}

// NewContextBuilder creates a new basic task context builder
func NewContextBuilder(ctx context.Context) *ContextBuilder {
	return &ContextBuilder{
		BaseContextBuilder: shared.NewBaseContextBuilder(ctx),
	}
}

// TaskType returns the type of task this builder handles
func (b *ContextBuilder) TaskType() task.Type {
	return task.TaskTypeBasic
}

// BuildContext creates a normalization context for basic tasks
// Basic tasks use the default context building logic from BaseContextBuilder
func (b *ContextBuilder) BuildContext(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) *shared.NormalizationContext {
	return b.BaseContextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
}
