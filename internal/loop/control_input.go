package loop

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
)

func isInputSourceNode(node dsl.Node) bool {
	return node.Class == dsl.NodeClassSource && node.Kind == string(dsl.SourceInput)
}

func evaluateInputSourceNode(
	run Run,
	output GenerationOutput,
	node dsl.Node,
) (GenerationOutput, error) {
	inputRef := strings.TrimSpace(node.InputRef)
	if inputRef == "" {
		return GenerationOutput{}, fmt.Errorf(
			"%w: input source %q input_ref is required",
			ErrValidation,
			node.ID,
		)
	}
	value, ok := run.Inputs[inputRef]
	if !ok {
		return GenerationOutput{}, fmt.Errorf(
			"%w: input source %q references missing input %q",
			ErrValidation,
			node.ID,
			inputRef,
		)
	}
	ref, err := inputSourceOutputRef(value)
	if err != nil {
		return GenerationOutput{}, err
	}
	output.Status = generationOutputSucceeded
	output.OutputRef = ref
	return output, nil
}

func inputSourceOutputRef(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("loop: marshal input source output: %w", err)
	}
	return string(encoded), nil
}
