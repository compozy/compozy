package core

import (
	"fmt"
	"sort"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

// OutputTransformer handles output normalization and transformation
type OutputTransformer struct {
	templateEngine *tplengine.TemplateEngine
}

// NewOutputTransformer creates a new output transformer
func NewOutputTransformer(templateEngine *tplengine.TemplateEngine) *OutputTransformer {
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

// transformOutputFields applies template transformation to output fields
func (ot *OutputTransformer) transformOutputFields(
	outputsConfig map[string]any,
	transformCtx map[string]any,
	contextName string,
) (map[string]any, error) {
	return TransformOutputFields(ot.templateEngine, outputsConfig, transformCtx, contextName)
}

// buildChildrenContext delegates to the actual ContextBuilder implementation
func buildChildrenContext(parentState *task.State, ctx *shared.NormalizationContext) map[string]any {
	// Use the ChildrenIndexBuilder's implementation to avoid code duplication
	childrenBuilder := shared.NewChildrenIndexBuilder()
	taskOutputBuilder := shared.NewTaskOutputBuilder()

	return childrenBuilder.BuildChildrenContext(
		parentState,
		ctx.WorkflowState,
		ctx.ChildrenIndex,
		ctx.TaskConfigs,
		taskOutputBuilder,
		0, // depth
	)
}

func TransformOutputFields(
	templateEngine *tplengine.TemplateEngine,
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
		processed, err := templateEngine.ParseAny(value, transformCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to transform %s output field %s: %w", contextName, key, err)
		}
		result[key] = processed
	}
	return result, nil
}
