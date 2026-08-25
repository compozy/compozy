package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type terminalExecApprover interface {
	ApproveTerminalExec(context.Context, toolspkg.Scope, toolspkg.CallRequest) (string, error)
}

type terminalPermissionBridge struct {
	mu       sync.RWMutex
	approval toolspkg.ApprovalBridge
}

var _ terminalpkg.TypingGrantAuthorizer = (*terminalPermissionBridge)(nil)
var _ terminalpkg.ExecAuthorizer = (*terminalPermissionBridge)(nil)
var _ terminalExecApprover = (*terminalPermissionBridge)(nil)

func newTerminalPermissionBridge() *terminalPermissionBridge {
	return &terminalPermissionBridge{}
}

func (b *terminalPermissionBridge) bind(approval toolspkg.ApprovalBridge) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.approval = approval
	b.mu.Unlock()
}

func (b *terminalPermissionBridge) ApproveTerminalExec(
	ctx context.Context,
	scope toolspkg.Scope,
	call toolspkg.CallRequest,
) (string, error) {
	approval := b.current()
	if approval == nil {
		return "", toolspkg.NewToolError(
			toolspkg.ErrorCodeApprovalRequired,
			call.ToolID,
			"approval_required — terminal approval is unavailable",
			toolspkg.ErrToolApprovalRequired,
			toolspkg.ReasonApprovalRequired,
			toolspkg.ReasonApprovalUnreachable,
		)
	}
	view := &toolspkg.ToolView{Descriptor: toolspkg.Descriptor{ID: call.ToolID}}
	if err := approval.RequestToolApproval(ctx, scope, &call, view); err != nil {
		return "", err
	}
	label := strings.TrimSpace(call.ApprovalLabel)
	if label == "" {
		label = "approved_once"
	}
	return label, nil
}

func (b *terminalPermissionBridge) AuthorizeTerminalExec(
	ctx context.Context,
	request terminalpkg.ExecRequest,
	_ terminalpkg.CommandClassification,
) (string, error) {
	input, err := json.Marshal(map[string]any{
		"command": request.Command,
		"args":    request.Args,
		"cwd":     request.Cwd,
		"visible": request.Visible,
	})
	if err != nil {
		return "", err
	}
	scope := toolspkg.Scope{
		ProfileID: request.Actor.ProfileID, WorkspaceID: request.WS, SessionID: request.Actor.SessionID,
		AgentName: request.Actor.ID, ActorKind: string(request.Actor.Kind),
	}
	call := toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDTerminalExec, ProfileID: request.Actor.ProfileID,
		WorkspaceID: request.WS, SessionID: request.Actor.SessionID,
		AgentName: request.Actor.ID, ActorKind: string(request.Actor.Kind), Input: input,
	}
	return b.ApproveTerminalExec(ctx, scope, call)
}

func (b *terminalPermissionBridge) AuthorizeTerminalInput(
	ctx context.Context,
	actor terminalpkg.Actor,
	info terminalpkg.Info,
) error {
	input, err := json.Marshal(map[string]any{
		"terminal_id":      info.ID,
		"grant_generation": info.TypingGeneration,
	})
	if err != nil {
		return &terminalpkg.Error{
			Code: "typing_grant_rejected", Message: "terminal typing grant could not be prepared",
			Err: errors.Join(terminalpkg.ErrTypingGrant, err),
		}
	}
	scope := toolspkg.Scope{
		ProfileID: info.ProfileID, WorkspaceID: info.WS, SessionID: actor.SessionID,
		AgentName: actor.ID, ActorKind: string(actor.Kind),
	}
	call := toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDTerminalWrite, ProfileID: info.ProfileID, WorkspaceID: info.WS,
		SessionID: actor.SessionID, AgentName: actor.ID, ActorKind: string(actor.Kind), Input: input,
	}
	if _, err := b.ApproveTerminalExec(ctx, scope, call); err != nil {
		return &terminalpkg.Error{
			Code: "typing_grant_rejected", Message: "agent typing requires a terminal grant",
			Err: errors.Join(terminalpkg.ErrTypingGrant, err),
		}
	}
	return nil
}

func (b *terminalPermissionBridge) current() toolspkg.ApprovalBridge {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.approval
}
