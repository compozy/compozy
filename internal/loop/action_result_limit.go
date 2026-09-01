package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/tools"
)

const actionResultOriginalBytesKey = "truncated_from_bytes"

// Harvest applies the registry result policy without replacing the resolved executor.
func (r *ActionRegistry) Harvest(
	ctx context.Context,
	executor ActionExecutor,
	node dsl.Node,
	raw ActionRawResult,
) (ActionOutput, error) {
	if r == nil || executor == nil {
		return ActionOutput{}, fmt.Errorf(
			"%w: action executor is required",
			ErrActionDependencyMissing,
		)
	}
	output, err := executor.Harvest(ctx, raw, node)
	if err != nil {
		return ActionOutput{}, err
	}
	if !dsl.IsReservedActionKind(node.Kind) {
		return output, nil
	}
	payload, err := actionRunResultPayload(output)
	if err != nil {
		return ActionOutput{}, err
	}
	if int64(len(payload)) > r.maxResultBytes {
		return ActionOutput{}, reservedActionResultTooLargeError(
			node,
			int64(len(payload)),
			r.maxResultBytes,
		)
	}
	return output, nil
}

func reservedActionResultTooLargeError(node dsl.Node, actualBytes int64, maxBytes int64) error {
	cause := fmt.Sprintf(
		"action node %q kind %q returned %d bytes, above the %d-byte result limit",
		node.ID,
		node.Kind,
		actualBytes,
		maxBytes,
	)
	recovery := "Return a smaller result or references, or raise tools.default_max_result_bytes."
	failure := NewActionFailure(string(ReasonCodeActionResultTooLarge), cause, recovery)
	failure.Target = string(node.ID)
	reason := reasonError(
		ReasonCodeActionResultTooLarge,
		fmt.Errorf("%w: %s", ErrActionResultTooLarge, cause),
		map[string]string{
			metadataNodeIDKey: string(node.ID),
			"action_kind":     node.Kind,
			"actual_bytes":    strconv.FormatInt(actualBytes, 10),
			"limit_bytes":     strconv.FormatInt(maxBytes, 10),
		},
	)
	return newSafeActionFailureError(reason, failure)
}

func actionResultTooLargeError(
	node dsl.Node,
	toolID tools.ToolID,
	result tools.ToolResult,
	maxBytes int64,
) error {
	actualBytes := actionResultOriginalBytes(result)
	cause := fmt.Sprintf(
		"action node %q tool %q returned %d bytes, above the %d-byte result limit",
		node.ID,
		toolID,
		actualBytes,
		maxBytes,
	)
	recovery := "Return a smaller result or references, or raise tools.default_max_result_bytes."
	failure := NewActionFailure(string(ReasonCodeActionResultTooLarge), cause, recovery)
	failure.Target = string(node.ID)
	reason := reasonError(
		ReasonCodeActionResultTooLarge,
		fmt.Errorf("%w: %s", ErrActionResultTooLarge, cause),
		map[string]string{
			metadataNodeIDKey: string(node.ID),
			"tool_id":         toolID.String(),
			"actual_bytes":    strconv.FormatInt(actualBytes, 10),
			"limit_bytes":     strconv.FormatInt(maxBytes, 10),
		},
	)
	return newSafeActionFailureError(reason, failure)
}

func actionResultOriginalBytes(result tools.ToolResult) int64 {
	if raw, ok := result.Metadata[actionResultOriginalBytesKey]; ok {
		var bytes int64
		if err := json.Unmarshal(raw, &bytes); err == nil && bytes > 0 {
			return bytes
		}
	}
	var artifactBytes int64
	for _, artifact := range result.Artifacts {
		artifactBytes = max(artifactBytes, artifact.Bytes)
	}
	if artifactBytes > 0 {
		return artifactBytes
	}
	return max(0, result.Bytes)
}
