package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionSideInput struct {
	Workspace       string `json:"workspace,omitempty"`
	SessionID       string `json:"session_id"`
	Message         string `json:"message,omitempty"`
	Name            string `json:"name,omitempty"`
	IdempotencyKey  string `json:"idempotency_key,omitempty"`
	IncludeArchived bool   `json:"include_archived,omitempty"`
}

func (n *daemonNativeTools) sessionSideOpen(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionSideInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	_, workspaceID, err := n.nativeDeriveTarget(ctx, scope, req.ToolID, input.Workspace)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, ok := n.deps.Sessions.(core.SessionSideManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: side chats are unavailable")
	}
	reader, ok := n.deps.Sessions.(interface {
		SessionOwner(context.Context, string) (store.SessionOwner, error)
	})
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: side source owner reader is unavailable")
	}
	owner, err := reader.SessionOwner(ctx, strings.TrimSpace(input.SessionID))
	if err != nil {
		return toolspkg.ToolResult{}, nativeDeriveToolError(req.ToolID, err)
	}
	result, err := manager.OpenSide(
		ctx,
		session.OpenSideOpts{
			SourceSessionID: strings.TrimSpace(input.SessionID),
			WorkspaceID:     workspaceID,
			ProfileID:       owner.ProfileID,
			Message:         input.Message,
			Name:            input.Name,
			IdempotencyKey:  input.IdempotencyKey,
		},
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeDeriveToolError(req.ToolID, core.DeriveFailure(err, result))
	}
	return structuredResult(struct {
		SideSessionID        string `json:"side_session_id"`
		SourceSessionID      string `json:"source_session_id"`
		Seed                 string `json:"seed"`
		SourceTurnInProgress bool   `json:"source_turn_in_progress"`
		Replayed             bool   `json:"replayed"`
	}{
		SideSessionID: result.ChildSessionID, SourceSessionID: result.SourceSessionID, Seed: string(result.Seed),
		SourceTurnInProgress: result.SourceTurnInProgress, Replayed: result.Replayed,
	}, "Opened side chat "+result.ChildSessionID)
}

func (n *daemonNativeTools) sessionSideList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionSideInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	_, workspaceID, err := n.nativeDeriveTarget(ctx, scope, req.ToolID, input.Workspace)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, ok := n.deps.Sessions.(core.SessionSideManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: side chats are unavailable")
	}
	sides, err := manager.ListSides(ctx, workspaceID, strings.TrimSpace(input.SessionID), input.IncludeArchived)
	if err != nil {
		return toolspkg.ToolResult{}, nativeDeriveToolError(req.ToolID, err)
	}
	return structuredResult(core.SessionSidesPayload(sides), "Side chats")
}
