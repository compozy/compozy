package loop

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

// completedRunLoopResult is the durable result emitted by the reserved run-loop action.
type completedRunLoopResult struct {
	LoopRunID string `json:"loop_run_id"`
	Status    string `json:"status"`
}

// refreshCompletedRunLoopOutput restores durable await state from a completed run-loop task.
func refreshCompletedRunLoopOutput(
	node dsl.Node,
	output GenerationOutput,
	payload json.RawMessage,
) (GenerationOutput, bool) {
	if node.Kind != string(dsl.ActionRunLoop) {
		return output, false
	}
	if node.Class != dsl.NodeClassAction {
		return invalidCompletedRunLoopOutput(output), true
	}
	var params dsl.RunLoopParams
	if err := node.Params.Decode(&params); err != nil {
		return invalidCompletedRunLoopOutput(output), true
	}
	if params.Mode == "" {
		params.Mode = dsl.RunLoopAwait
	}
	if params.Mode == dsl.RunLoopDetach {
		return output, false
	}
	if params.Mode != dsl.RunLoopAwait {
		return invalidCompletedRunLoopOutput(output), true
	}

	var result completedRunLoopResult
	if len(bytes.TrimSpace(payload)) == 0 {
		return invalidCompletedRunLoopOutput(output), true
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		return invalidCompletedRunLoopOutput(output), true
	}
	if result.Status != generationOutputAwaitingChild || result.LoopRunID == "" ||
		strings.TrimSpace(result.LoopRunID) != result.LoopRunID {
		return invalidCompletedRunLoopOutput(output), true
	}

	output.Status = generationOutputAwaitingChild
	output.ChildLoopRunID = result.LoopRunID
	setGenerationOutputRef(&output, string(payload))
	return output, true
}

// invalidCompletedRunLoopOutput rejects a completed await result that cannot identify its child.
func invalidCompletedRunLoopOutput(output GenerationOutput) GenerationOutput {
	return invalidActionOutput(
		output,
		output.NodeID,
		"completed run-loop await result is invalid",
		"Inspect the completed run-loop result before retrying the parent Loop.",
	)
}

// invalidCompletedActionOwnerOutput rejects a result whose authored graph node disappeared.
func invalidCompletedActionOwnerOutput(output GenerationOutput) GenerationOutput {
	return invalidActionOutput(
		output,
		output.NodeID,
		"completed action owner node is missing",
		"Restore the authored graph node before retrying the parent Loop.",
	)
}

// invalidAwaitedChildBoundaryOutput rejects a child owned by another parent or workspace.
func invalidAwaitedChildBoundaryOutput(output GenerationOutput, childRunID RunID) GenerationOutput {
	return invalidActionOutput(
		output,
		string(childRunID),
		"awaited child Loop is outside the parent boundary",
		"Inspect the child workspace and parent Loop identity before retrying.",
	)
}

// invalidAwaitedChildIdentityOutput rejects a persisted child ID that requires normalization.
func invalidAwaitedChildIdentityOutput(output GenerationOutput) GenerationOutput {
	return invalidActionOutput(
		output,
		output.NodeID,
		"awaited child Loop identity is invalid",
		"Inspect the persisted child Loop identity before retrying.",
	)
}

// invalidActionOutput records a stable authoring failure and drops any untrusted child ID.
func invalidActionOutput(output GenerationOutput, target, cause, hint string) GenerationOutput {
	failure := NewActionFailure(
		string(ReasonCodeActionSchemaInvalid),
		cause,
		hint,
	)
	failure.Target = target
	output.Status = generationOutputFailed
	output.ChildLoopRunID = ""
	if ref, ok := ActionFailureOutputRef(failure); ok {
		setGenerationOutputRef(&output, ref)
	}
	return output
}
