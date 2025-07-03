package parallel

import (
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

// ContextBuilder builds contexts for parallel tasks
type ContextBuilder struct {
	*shared.BaseContextBuilder
}

// NewContextBuilder creates a new parallel task context builder
func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{
		BaseContextBuilder: shared.NewBaseContextBuilder(),
	}
}

// TaskType returns the type of task this builder handles
func (b *ContextBuilder) TaskType() task.Type {
	return task.TaskTypeParallel
}

// BuildContext creates a normalization context for parallel tasks
func (b *ContextBuilder) BuildContext(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) *shared.NormalizationContext {
	// Start with base context
	ctx := b.BaseContextBuilder.BuildContext(workflowState, workflowConfig, taskConfig)
	// Parallel tasks don't need special context modifications
	// Each sub-task will get its own context when normalized
	return ctx
}

// EnrichContext adds parallel-specific data to an existing context
func (b *ContextBuilder) EnrichContext(ctx *shared.NormalizationContext, taskState *task.State) error {
	// Use base enrichment - parallel tasks don't need special enrichment
	return b.BaseContextBuilder.EnrichContext(ctx, taskState)
}

// ValidateContext ensures the context has all required fields for parallel tasks
func (b *ContextBuilder) ValidateContext(ctx *shared.NormalizationContext) error {
	// Use base validation - parallel tasks don't have special validation requirements
	return b.BaseContextBuilder.ValidateContext(ctx)
}
