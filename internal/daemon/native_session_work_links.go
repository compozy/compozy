package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) sessionLink(ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest) (toolspkg.ToolResult, error) {
	return n.sessionWorkLink(ctx, scope, req, false)
}
func (n *daemonNativeTools) sessionUnlink(ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest) (toolspkg.ToolResult, error) {
	return n.sessionWorkLink(ctx, scope, req, true)
}
func (n *daemonNativeTools) sessionWorkLink(ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest, unlink bool) (toolspkg.ToolResult, error) {
	var input contract.SessionWorkLinkRequest
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if scope.SessionID == "" || scope.WorkspaceID == "" {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, errors.New("calling session is required"))
	}
	if _, err := n.nativeSessionInWorkspace(ctx, scope, req.ToolID, scope.WorkspaceID, scope.SessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, ok := n.deps.Sessions.(core.SessionLinkedWorkManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("session linked work unavailable")
	}
	var err error
	if unlink {
		err = manager.UnlinkSessionWork(ctx, scope.SessionID, input.Repository, input.Branch)
	} else {
		err = manager.LinkSessionWork(ctx, scope.SessionID, store.SessionWorkLink{Repository: input.Repository, Branch: input.Branch, PRNumber: input.PRNumber, Provenance: "agent"}, false)
	}
	if err != nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(toolspkg.ErrorCode(core.LinkedWorkErrorCode(err)), req.ToolID, err.Error(), err)
	}
	return structuredResult(map[string]any{"session_id": scope.SessionID, "branch": input.Branch, "unlinked": unlink}, scope.SessionID)
}
