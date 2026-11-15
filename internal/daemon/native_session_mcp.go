package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) sessionMCPList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct {
		SessionID string `json:"session_id"`
	}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	id := strings.TrimSpace(input.SessionID)
	if id == "" {
		id = firstNonEmpty(scope.SessionID, req.SessionID)
	}
	if id == "" {
		return toolspkg.ToolResult{}, errors.New("session_id is required outside a session")
	}
	ctx = toolspkg.WithCachedMCPProjection(ctx)
	info, err := n.deps.Sessions.Status(ctx, id)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	profileID := strings.TrimSpace(scope.ProfileID)
	if profileID == "" {
		profileID = store.DefaultProfileID
		if callerID := firstNonEmpty(scope.SessionID, req.SessionID); callerID != "" {
			caller, err := n.deps.Sessions.Status(ctx, callerID)
			if err != nil {
				return toolspkg.ToolResult{}, err
			}
			profileID = caller.ProfileID
		}
	}
	if info.WorkspaceID != scope.WorkspaceID || info.ProfileID != profileID {
		return toolspkg.ToolResult{}, session.ErrSessionNotFound
	}
	reader, ok := n.deps.Sessions.(session.MCPReader)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("session MCP read model unavailable")
	}
	result, err := reader.ListSessionMCP(ctx, id)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(result, "Session MCP servers")
}
