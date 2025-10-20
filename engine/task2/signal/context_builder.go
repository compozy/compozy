package signal

import (
	"context"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

// ContextBuilder builds contexts for signal tasks
type ContextBuilder struct {
	*shared.BaseContextBuilder
}

// NewContextBuilder creates a new signal task context builder
func NewContextBuilder(ctx context.Context) *ContextBuilder {
	return &ContextBuilder{
		BaseContextBuilder: shared.NewBaseContextBuilder(ctx),
	}
}

// TaskType returns the type of task this builder handles
func (b *ContextBuilder) TaskType() task.Type {
	return task.TaskTypeSignal
}

// BuildContext creates a normalization context for signal tasks
func (b *ContextBuilder) BuildContext(
	ctx context.Context,
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) *shared.NormalizationContext {
	return b.BaseContextBuilder.BuildContext(ctx, workflowState, workflowConfig, taskConfig)
}
