package core

import (
	"context"
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
	ctx context.Context,
	output *core.Output,
	outputsConfig *core.Input,
	normCtx *shared.NormalizationContext,
	taskConfig *task.Config,
) (*core.Output, error) {
	if outputsConfig == nil || output == nil {
		return output, nil
	}
	transformCtx := normCtx.BuildTemplateContext()
	if taskConfig.Type == task.TaskTypeCollection || taskConfig.Type == task.TaskTypeParallel {
		if nestedOutputs, ok := (*output)["outputs"]; ok {
			transformCtx["output"] = nestedOutputs
		} else {
			transformCtx["output"] = make(map[string]any)
		}
		if normCtx.WorkflowState != nil && normCtx.WorkflowState.Tasks != nil {
			if taskState, exists := normCtx.WorkflowState.Tasks[taskConfig.ID]; exists {
				if taskState.CanHaveChildren() && normCtx.ChildrenIndex != nil {
					transformCtx["children"] = buildChildrenContext(ctx, taskState, normCtx)
				}
			}
		}
	} else {
		transformCtx["output"] = output
	}
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
func buildChildrenContext(
	ctx context.Context,
	parentState *task.State,
	normCtx *shared.NormalizationContext,
) map[string]any {
	childrenBuilder := shared.NewChildrenIndexBuilder()
	taskOutputBuilder := shared.NewTaskOutputBuilder(ctx)
	return childrenBuilder.BuildChildrenContext(
		ctx,
		parentState,
		normCtx.WorkflowState,
		normCtx.ChildrenIndex,
		normCtx.TaskConfigs,
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
	// NOTE: Iterate deterministically so Temporal workers produce stable payloads.
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
