package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func validateNodeRuntimeAuthoring(
	ctx context.Context,
	catalog RuntimeCatalog,
	inputs map[string]dsl.Input,
	path string,
	params dsl.NodeParams,
) error {
	runtime, inputName, err := authoredNodeRuntime(params)
	if err != nil {
		return runtimeValidation(path, params["runtime"], "invalid_runtime_binding")
	}
	if inputName == "" {
		return validateRuntimeSpec(ctx, catalog, path, runtime)
	}
	input, exists := inputs[inputName]
	if !exists || input.Type != dsl.InputTypeRuntime {
		return runtimeValidation(path, inputName, "runtime_input_required")
	}
	return nil
}

func materializedNodeRuntime(
	params dsl.NodeParams,
	materialized RuntimeSpec,
) (RuntimeSpec, RuntimeSpec, error) {
	_, inputName, err := authoredNodeRuntime(params)
	if err != nil {
		return RuntimeSpec{}, RuntimeSpec{}, err
	}
	if inputName != "" {
		return RuntimeSpec{}, materialized, nil
	}
	return materialized, RuntimeSpec{}, nil
}

func authoredNodeRuntime(params dsl.NodeParams) (RuntimeSpec, string, error) {
	raw, exists := params["runtime"]
	if !exists || raw == nil {
		return RuntimeSpec{}, "", nil
	}
	if reference, ok := raw.(string); ok {
		path, direct := directTemplateReferencePath(reference)
		parts := strings.Split(path, ".")
		if !direct || len(parts) != 2 || parts[0] != namespaceInputsKey || strings.TrimSpace(parts[1]) == "" {
			return RuntimeSpec{}, "", fmt.Errorf(
				"runtime must be an object or an exact {{ .inputs.<name> }} reference",
			)
		}
		return RuntimeSpec{}, parts[1], nil
	}
	runtime, err := runtimeInputSpec(raw)
	if err != nil {
		return RuntimeSpec{}, "", fmt.Errorf("runtime must be an object: %w", err)
	}
	if runtimeSpecContainsTemplate(runtime) {
		return RuntimeSpec{}, "", fmt.Errorf(
			"runtime field interpolation is not supported; reference the complete runtime input",
		)
	}
	return runtime, "", nil
}

func runtimeSpecContainsTemplate(runtime RuntimeSpec) bool {
	for _, value := range []string{
		runtime.Provider,
		runtime.Model,
		runtime.Reasoning,
	} {
		if strings.Contains(value, "{{") || strings.Contains(value, "}}") {
			return true
		}
	}
	return false
}

func decodeRunAgentNodeParams(params dsl.NodeParams) (dsl.RunAgentParams, error) {
	var decoded dsl.RunAgentParams
	err := paramsWithDecodableRuntime(params).Decode(&decoded)
	return decoded, err
}

func decodeGoalNodeParams(params dsl.NodeParams) (dsl.GoalParams, error) {
	var decoded dsl.GoalParams
	err := paramsWithDecodableRuntime(params).Decode(&decoded)
	return decoded, err
}

func paramsWithDecodableRuntime(params dsl.NodeParams) dsl.NodeParams {
	if _, isReference := params["runtime"].(string); !isReference {
		return params
	}
	cloned := cloneNodeParams(params)
	cloned["runtime"] = map[string]any{}
	return cloned
}
