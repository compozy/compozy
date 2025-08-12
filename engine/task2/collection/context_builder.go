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
func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{
		BaseContextBuilder: shared.NewBaseContextBuilder(),
	}
}

// TaskType returns the type of task this builder handles
func (b *ContextBuilder) TaskType() task.Type {
	return task.TaskTypeCollection
}

// BuildContext creates a normalization context for collection tasks
func (b *ContextBuilder) BuildContext(
	workflowState *workflow.State,
	workflowConfig *workflow.Config,
	taskConfig *task.Config,
) *shared.NormalizationContext {
	// Start with base context
	ctx := b.BaseContextBuilder.BuildContext(workflowState, workflowConfig, taskConfig)
	// Collection tasks will have item and index added during iteration
	// This is just the base context for the collection itself
	return ctx
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
	// Deep copy tasks map to ensure isolation between collection children
	if tasks, ok := iterCtx.Variables["tasks"].(map[string]any); ok {
		iterCtx.Variables["tasks"] = deepCopyTasksMap(tasks)
	}
	// Add item and index to variables
	iterCtx.Variables[shared.ItemKey] = item
	iterCtx.Variables[shared.IndexKey] = index
	// Create current input by merging parent context with item and index
	currentInput := make(core.Input)
	// First, copy parent's current input if it exists
	if baseContext.CurrentInput != nil {
		for k, v := range *baseContext.CurrentInput {
			currentInput[k] = v
		}
	}
	// Then add/override with item and index
	currentInput[shared.ItemKey] = item
	currentInput[shared.IndexKey] = index
	iterCtx.CurrentInput = &currentInput
	// Also add to input variable
	iterCtx.Variables["input"] = &currentInput
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

// deepCopyTasksMap creates a deep copy of the tasks map to ensure isolation
func deepCopyTasksMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		switch t := v.(type) {
		case core.Output:
			// core.Output is an alias for map[string]any
			copied := deepCopyGenericMap(map[string]any(t))
			dst[k] = core.Output(copied)
		case *core.Output:
			if t != nil {
				copied := deepCopyGenericMap(map[string]any(*t))
				output := core.Output(copied)
				dst[k] = &output
			}
		case map[string]any:
			// Handle nested maps
			dst[k] = deepCopyGenericMap(t)
		default:
			// For other types, direct assignment is safe
			dst[k] = v
		}
	}
	return dst
}

// deepCopyGenericMap performs a deep copy of a map[string]any to ensure isolation
func deepCopyGenericMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case map[string]any:
			dst[k] = deepCopyGenericMap(val)
		case *map[string]any:
			if val != nil {
				dst[k] = deepCopyGenericMap(*val)
			} else {
				dst[k] = nil
			}
		case core.Input:
			dst[k] = deepCopyGenericMap(map[string]any(val))
		case *core.Input:
			if val != nil {
				dst[k] = deepCopyGenericMap(map[string]any(*val))
			} else {
				dst[k] = nil
			}
		case core.Output:
			dst[k] = deepCopyGenericMap(map[string]any(val))
		case *core.Output:
			if val != nil {
				dst[k] = deepCopyGenericMap(map[string]any(*val))
			} else {
				dst[k] = nil
			}
		case []any:
			dst[k] = deepCopyGenericSlice(val)
		case []map[string]any:
			newSlice := make([]map[string]any, len(val))
			for i, m := range val {
				newSlice[i] = deepCopyGenericMap(m)
			}
			dst[k] = newSlice
		default:
			dst[k] = val
		}
	}
	return dst
}

// deepCopyGenericSlice performs a deep copy of a []any
func deepCopyGenericSlice(src []any) []any {
	if src == nil {
		return nil
	}
	dst := make([]any, len(src))
	for i, v := range src {
		switch val := v.(type) {
		case map[string]any:
			dst[i] = deepCopyGenericMap(val)
		case *map[string]any:
			if val != nil {
				dst[i] = deepCopyGenericMap(*val)
			} else {
				dst[i] = nil
			}
		case []any:
			dst[i] = deepCopyGenericSlice(val)
		default:
			dst[i] = val
		}
	}
	return dst
}
