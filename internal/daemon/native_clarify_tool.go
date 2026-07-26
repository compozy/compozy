package daemon

import (
	"context"
	"errors"
	"fmt"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) clarifyToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDClarify: {
			call:         n.clarify,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) clarify(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	broker := n.clarifyBroker()
	if broker == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "clarification broker is unavailable")
	}
	var input toolspkg.ClarifyQuestion
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	normalized, err := input.Normalize()
	if err != nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			req.ToolID,
			"clarification input is invalid",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	answer, err := broker.Ask(ctx, scope, normalized)
	if err != nil {
		switch {
		case errors.Is(err, toolspkg.ErrClarifyPending):
			return toolspkg.ToolResult{}, toolspkg.NewToolError(
				toolspkg.ErrorCodeConflict,
				req.ToolID,
				"a clarification is already pending for this session",
				err,
			)
		case errors.Is(err, toolspkg.ErrClarifyCanceled), errors.Is(err, context.Canceled):
			return toolspkg.ToolResult{}, fmt.Errorf("daemon: clarification canceled: %w", err)
		default:
			return toolspkg.ToolResult{}, fmt.Errorf("daemon: ask clarification: %w", err)
		}
	}
	return structuredResult(answer, clarifyPreview(answer))
}

func (n *daemonNativeTools) clarifyBroker() toolspkg.ClarifyBroker {
	if n == nil || n.deps == nil || n.deps.Clarify == nil {
		return nil
	}
	return n.deps.Clarify()
}

func clarifyPreview(answer toolspkg.ClarifyAnswer) string {
	switch {
	case answer.Fallback:
		return "No clarification answer received before timeout."
	case answer.Choice != nil:
		return fmt.Sprintf("Clarification answered with choice %d.", *answer.Choice)
	default:
		return "Clarification answered."
	}
}
