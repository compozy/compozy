package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) sessionInputsClear(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionInputListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sessionID, err := n.nativeSessionInputScope(ctx, scope, req.ToolID, input.Workspace, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	actorKind, actorID := "agent", firstNonEmpty(scope.AgentName, scope.SessionID)
	if scope.Operator {
		actorKind, actorID = "human", "operator"
	}
	result, err := n.deps.Sessions.ClearPendingInputs(ctx, sessionID, session.PromptCaller{
		Kind: actorKind, ID: actorID, Source: req.ToolID.String(),
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(
		core.SessionInputClearPayload(result),
		fmt.Sprintf("Cleared %d queued entries", result.ClearedCount),
	)
}

func (n *daemonNativeTools) sessionInputToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDSessionInputsList: {
			call:         n.sessionInputsList,
			availability: availability,
		},
		toolspkg.ToolIDSessionInputsClear: {
			call:         n.sessionInputsClear,
			availability: availability,
		},
		toolspkg.ToolIDSessionInputReplace: {
			call:         n.sessionInputReplace,
			availability: availability,
		},
		toolspkg.ToolIDSessionInputCancel: {
			call:         n.sessionInputCancel,
			availability: availability,
		},
		toolspkg.ToolIDSessionInputPromote: {
			call:         n.sessionInputPromote,
			availability: availability,
		},
	}
}
