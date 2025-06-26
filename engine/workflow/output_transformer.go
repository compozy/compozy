package workflow

import (
	"fmt"
	"sort"

	"github.com/compozy/compozy/engine/core"
)

// TemplateEngine defines interface for template processing
type TemplateEngine interface {
	Process(template string, context map[string]any) (any, error)
	ProcessMap(templateMap map[string]any, context map[string]any) (map[string]any, error)
}

// NormalizationContext defines context for normalization
type NormalizationContext interface {
	BuildTemplateContext() map[string]any
}

// OutputNormalizer handles workflow output normalization and transformation
type OutputNormalizer struct {
	templateEngine TemplateEngine
}

// NewOutputNormalizer creates a new workflow output transformer
func NewOutputNormalizer(templateEngine TemplateEngine) *OutputNormalizer {
	return &OutputNormalizer{
		templateEngine: templateEngine,
	}
}

// TransformWorkflowOutput transforms workflow output based on the outputs configuration
func (ot *OutputNormalizer) TransformWorkflowOutput(
	workflowState *State,
	outputsConfig *core.Output,
	ctx NormalizationContext,
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
func (ot *OutputNormalizer) transformOutputFields(
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
