package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) sessionCompact(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSessionTargetInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, _, err := n.nativeOrchestrationTarget(ctx, scope, req.ToolID, input.SessionID, false)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, ok := n.deps.Sessions.(core.SessionCompactionManager)
	if !ok {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "session compaction unavailable")
	}
	result, _, err := manager.RequestCompaction(session.WithCompactionRequestedBy(ctx, "tool"), target)
	if err != nil {
		return toolspkg.ToolResult{}, nativeSessionOrchestrationError(req.ToolID, err)
	}
	return structuredResult(core.SessionCompactPayload(result), "Compaction accepted")
}
