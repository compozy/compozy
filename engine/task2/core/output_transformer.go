package core

import (
	"fmt"
	"sort"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/workflow"
)

// OutputTransformer handles output normalization and transformation
type OutputTransformer struct {
	templateEngine shared.TemplateEngine
}

// NewOutputTransformer creates a new output transformer
func NewOutputTransformer(templateEngine shared.TemplateEngine) *OutputTransformer {
	return &OutputTransformer{
		templateEngine: templateEngine,
	}
}

// TransformOutput transforms task output based on the outputs configuration
func (ot *OutputTransformer) TransformOutput(
	output *core.Output,
	outputsConfig *core.Input,
	ctx *shared.NormalizationContext,
	taskConfig *task.Config,
) (*core.Output, error) {
	if outputsConfig == nil || output == nil {
		return output, nil
	}
	// Build transformation context
	transformCtx := ctx.BuildTemplateContext()
	// Special handling for collection/parallel tasks
	if taskConfig.Type == task.TaskTypeCollection || taskConfig.Type == task.TaskTypeParallel {
		// Look for the nested outputs map
		if nestedOutputs, ok := (*output)["outputs"]; ok {
			// Use child outputs map as .output in template context
			transformCtx["output"] = nestedOutputs
		} else {
			// Outputs not yet aggregated, use empty map
			transformCtx["output"] = make(map[string]any)
		}
		// For parent tasks, also add children context at the top level
		if ctx.WorkflowState != nil && ctx.WorkflowState.Tasks != nil {
			if taskState, exists := ctx.WorkflowState.Tasks[taskConfig.ID]; exists {
				if taskState.CanHaveChildren() && ctx.ChildrenIndex != nil {
					transformCtx["children"] = buildChildrenContext(taskState, ctx)
				}
			}
		}
	} else {
		// For regular tasks, use the full output
		transformCtx["output"] = output
	}
	// Apply output transformation
	transformedOutput, err := ot.transformOutputFields(*outputsConfig, transformCtx, "task")
	if err != nil {
		return nil, err
	}
	result := core.Output(transformedOutput)
	return &result, nil
}

// TransformWorkflowOutput transforms workflow output based on the outputs configuration
func (ot *OutputTransformer) TransformWorkflowOutput(
	workflowState *workflow.State,
	outputsConfig *core.Output,
	ctx *shared.NormalizationContext,
) (*core.Output, error) {
	if outputsConfig == nil {
		return nil, nil
	}
	// Build transformation context
	transformCtx := ctx.BuildTemplateContext()
	// Add workflow-specific fields
	transformCtx["status"] = workflowState.Status
	transformCtx["workflow_id"] = workflowState.WorkflowID
	transformCtx["workflow_exec_id"] = workflowState.WorkflowExecID
	if workflowState.Error != nil {
		transformCtx["error"] = workflowState.Error
	}
	// Apply output transformation
	if len(*outputsConfig) == 0 {
		return &core.Output{}, nil
	}
	transformedOutput, err := ot.transformOutputFields(outputsConfig.AsMap(), transformCtx, "workflow")
	if err != nil {
		return nil, err
	}
	finalOutput := core.Output(transformedOutput)
	return &finalOutput, nil
}

// transformOutputFields applies template transformation to output fields
func (ot *OutputTransformer) transformOutputFields(
	outputsConfig map[string]any,
	transformCtx map[string]any,
	contextName string,
) (map[string]any, error) {
	// Sort keys to ensure deterministic iteration order for Temporal workflows
	keys := make([]string, 0, len(outputsConfig))
	for k := range outputsConfig {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make(map[string]any)
	for _, key := range keys {
		value := outputsConfig[key]
		// Value can be a string template or a map
		switch v := value.(type) {
		case string:
			// Process string template
			processed, err := ot.templateEngine.Process(v, transformCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to transform %s output field %s: %w", contextName, key, err)
			}
			result[key] = processed
		case map[string]any:
			// Process map recursively
			processed, err := ot.templateEngine.ProcessMap(v, transformCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to transform %s output field %s: %w", contextName, key, err)
			}
			result[key] = processed
		default:
			// Keep other types as-is
			result[key] = value
		}
	}
	return result, nil
}

// buildChildrenContext is a helper function to build children context
// This is a simplified version - the actual implementation is in ContextBuilder
func buildChildrenContext(parentState *task.State, ctx *shared.NormalizationContext) map[string]any {
	children := make(map[string]any)
	parentExecID := string(parentState.TaskExecID)
	if childTaskIDs, exists := ctx.ChildrenIndex[parentExecID]; exists {
		for _, childTaskID := range childTaskIDs {
			if childState, exists := ctx.WorkflowState.Tasks[childTaskID]; exists {
				childMap := map[string]any{
					"id":     childTaskID,
					"input":  childState.Input,
					"output": childState.Output,
					"status": childState.Status,
				}
				if childState.Error != nil {
					childMap["error"] = childState.Error
				}
				children[childTaskID] = childMap
			}
		}
	}
	return children
}
