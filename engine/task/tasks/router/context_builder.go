package router

import (
	"context"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/workflow"
)

// ContextBuilder builds contexts for router tasks
type ContextBuilder struct {
	*shared.BaseContextBuilder
}

// NewContextBuilder creates a new router task context builder
func NewContextBuilder(ctx context.Context) *ContextBuilder {
	return &ContextBuilder{
		BaseContextBuilder: shared.NewBaseContextBuilder(ctx),
	}
}

// TaskType returns the type of task this builder handles
func (b *ContextBuilder) TaskType() task.Type {
	return task.TaskTypeRouter
}

// BuildContext creates a normalization context for router tasks
func (b *ContextBuilder) BuildContext(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) *shared.NormalizationContext {
	return b.BaseContextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
}
