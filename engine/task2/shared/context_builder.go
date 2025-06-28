package shared

import (
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

// ContextBuilderInterface defines the interface for building normalization contexts
type ContextBuilderInterface interface {
	// BuildContext creates a normalization context from workflow and task data
	BuildContext(
		workflowState *workflow.State,
		workflowConfig *workflow.Config,
		taskConfig *task.Config,
	) *NormalizationContext
	// EnrichContext adds additional data to an existing context
	EnrichContext(ctx *NormalizationContext, taskState *task.State) error
	// ValidateContext ensures the context has all required fields
	ValidateContext(ctx *NormalizationContext) error
}

// TaskContextBuilder is the interface for task-specific context builders
type TaskContextBuilder interface {
	ContextBuilderInterface
	// TaskType returns the type of task this builder handles
	TaskType() task.Type
}
