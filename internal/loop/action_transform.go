package loop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
)

// TransformActionExecutor evaluates pure in-daemon transform maps.
type TransformActionExecutor struct{}

// Execute evaluates params.map without side effects.
func (TransformActionExecutor) Execute(
	_ context.Context,
	node dsl.Node,
	in ActionExecutionInput,
) (ActionRawResult, error) {
	params, err := normalizeNodeParams(node.Params)
	if err != nil {
		return ActionRawResult{}, err
	}
	var spec dsl.TransformParams
	if err := dsl.NodeParams(params).Decode(&spec); err != nil {
		return ActionRawResult{}, fmt.Errorf("decode transform params: %w", err)
	}
	output := make(map[string]any, len(spec.Map))
	for key, mapping := range spec.Map {
		value, err := evaluateTransformMapping(key, mapping, in.Namespace)
		if err != nil {
			return ActionRawResult{}, err
		}
		output[key] = value
	}
	structured, err := json.Marshal(output)
	if err != nil {
		return ActionRawResult{}, fmt.Errorf("marshal transform output: %w", err)
	}
	return ActionRawResult{Structured: structured, Value: output}, nil
}

// Harvest returns the transform output as-is.
func (TransformActionExecutor) Harvest(_ context.Context, raw ActionRawResult, node dsl.Node) (ActionOutput, error) {
	if err := validateSyncHarvest(node); err != nil {
		return ActionOutput{}, err
	}
	return outputFromRaw(raw)
}
