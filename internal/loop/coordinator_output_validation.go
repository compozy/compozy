package loop

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func completedRunAgentOutputFailure(node dsl.Node, payload json.RawMessage) *ActionFailure {
	if node.Class != dsl.NodeClassAction || dsl.ActionKind(node.Kind) != dsl.ActionRunAgent {
		return nil
	}
	params, err := decodeRunAgentNodeParams(node.Params)
	if err != nil {
		failure := NewActionFailure(
			string(ReasonCodeInvalidOutput),
			fmt.Sprintf("the pinned run-agent output schema is invalid: %s", err),
			"Fix the run-agent output_schema before retrying the Loop.",
		)
		return &failure
	}
	if len(params.OutputSchema) == 0 {
		return nil
	}
	_, err = ValidateActionStructured(params.OutputSchema, ActionPromptResult{Structured: payload})
	if err == nil {
		return nil
	}
	if provider, ok := errors.AsType[SafeActionFailureProvider](err); ok {
		failure := provider.SafeActionFailure()
		return &failure
	}
	failure := NewActionFailure(
		string(ReasonCodeInvalidOutput),
		schemaInvalidCause(err),
		"Return one JSON object that satisfies every required output field, then retry the action.",
	)
	return &failure
}
