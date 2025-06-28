package shared

import (
	"fmt"

	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

// ContextBuilderAdapter adapts TaskContextBuilder to work with the existing ContextBuilder
type ContextBuilderAdapter struct {
	*ContextBuilder
	taskContextBuilder TaskContextBuilder
}

// NewContextBuilderAdapter creates a new adapter
func NewContextBuilderAdapter(taskContextBuilder TaskContextBuilder) (*ContextBuilderAdapter, error) {
	// Create the base context builder
	baseBuilder, err := NewContextBuilder()
	if err != nil {
		return nil, fmt.Errorf("failed to create base context builder: %w", err)
	}
	return &ContextBuilderAdapter{
		ContextBuilder:     baseBuilder,
		taskContextBuilder: taskContextBuilder,
	}, nil
}

// BuildContext delegates to the task-specific context builder
func (a *ContextBuilderAdapter) BuildContext(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) *NormalizationContext {
	// Use task-specific context builder if available
	if a.taskContextBuilder != nil {
		return a.taskContextBuilder.BuildContext(workflowState, workflowConfig, taskConfig)
	}
	// Fall back to base implementation
	return a.ContextBuilder.BuildContext(workflowState, workflowConfig, taskConfig)
}

// EnrichContext delegates to the task-specific context builder
func (a *ContextBuilderAdapter) EnrichContext(ctx *NormalizationContext, taskState *task.State) error {
	// Use task-specific context builder if available
	if a.taskContextBuilder != nil {
		return a.taskContextBuilder.EnrichContext(ctx, taskState)
	}
	// Fall back to base implementation (no-op in base ContextBuilder)
	return nil
}

// ValidateContext delegates to the task-specific context builder
func (a *ContextBuilderAdapter) ValidateContext(ctx *NormalizationContext) error {
	// Use task-specific context builder if available
	if a.taskContextBuilder != nil {
		return a.taskContextBuilder.ValidateContext(ctx)
	}
	// Fall back to base implementation (no validation in base ContextBuilder)
	return nil
}
