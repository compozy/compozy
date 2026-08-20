package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"time"

	"github.com/compozy/compozy/internal/cmdpalette"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
)

const cmdPaletteActionKey = "action"

type cmdPaletteClientDirectory struct {
	windowManager windowmanager.Service
}

var _ cmdpalette.ClientDirectory = (*cmdPaletteClientDirectory)(nil)
var _ cmdpalette.GlobalShortcutStatusDirectory = (*cmdPaletteClientDirectory)(nil)

func (d *cmdPaletteClientDirectory) Clients(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
) ([]cmdpalette.Client, error) {
	if d == nil || d.windowManager == nil {
		return []cmdpalette.Client{}, nil
	}
	views, err := d.windowManager.Clients(ctx, windowmanager.WorkspaceID(workspaceID))
	if err != nil {
		return nil, fmt.Errorf("cmd palette: list window-manager clients: %w", err)
	}
	clients := make([]cmdpalette.Client, 0, len(views))
	for _, view := range views {
		clients = append(clients, cmdpalette.Client{
			ID: cmdpalette.ClientID(view.ClientID), Kind: string(view.Kind), WorkspaceID: workspaceID,
			AttachedAt: view.ConnectedAt, ContextRevision: strconv.FormatUint(view.ContextRevision, 10),
		})
	}
	return clients, nil
}

func (d *cmdPaletteClientDirectory) Context(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
	clientID cmdpalette.ClientID,
) (cmdpalette.ContextSnapshot, error) {
	clients, err := d.windowManager.Clients(ctx, windowmanager.WorkspaceID(workspaceID))
	if err != nil {
		return cmdpalette.ContextSnapshot{}, fmt.Errorf("cmd palette: resolve client context: %w", err)
	}
	for _, client := range clients {
		if client.ClientID != windowmanager.ClientID(clientID) {
			continue
		}
		return cmdPaletteContextSnapshot(client), nil
	}
	return cmdpalette.ContextSnapshot{}, cmdpalette.ErrNoAttachedShell
}

func (d *cmdPaletteClientDirectory) Authorize(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
	clientID cmdpalette.ClientID,
	token string,
) error {
	if d == nil || d.windowManager == nil {
		return cmdpalette.ErrClientUnauthorized
	}
	return d.windowManager.AuthorizeClient(
		ctx, windowmanager.WorkspaceID(workspaceID), windowmanager.ClientID(clientID), token,
	)
}

func (d *cmdPaletteClientDirectory) GlobalShortcutStatuses(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
	clientID cmdpalette.ClientID,
) (map[cmdpalette.CommandID]cmdpalette.GlobalShortcut, error) {
	if d == nil || d.windowManager == nil {
		return map[cmdpalette.CommandID]cmdpalette.GlobalShortcut{}, nil
	}
	clients, err := d.windowManager.Clients(ctx, windowmanager.WorkspaceID(workspaceID))
	if err != nil {
		return nil, fmt.Errorf("cmd palette: resolve global shortcut status: %w", err)
	}
	for _, client := range clients {
		if client.ClientID != windowmanager.ClientID(clientID) {
			continue
		}
		result := make(map[cmdpalette.CommandID]cmdpalette.GlobalShortcut, len(client.GlobalShortcuts))
		for _, status := range client.GlobalShortcuts {
			result[cmdpalette.CommandID(status.CommandID)] = cmdpalette.GlobalShortcut{
				IntendedChord: status.IntendedChord,
				ActiveChord:   status.ActiveChord,
				Status:        string(status.Status),
				Reason:        status.Reason,
				SettingsURL:   status.SettingsURL,
			}
		}
		return result, nil
	}
	return nil, cmdpalette.ErrNoAttachedShell
}

func cmdPaletteContextSnapshot(client windowmanager.ClientView) cmdpalette.ContextSnapshot {
	context := client.PaletteContext
	return cmdpalette.ContextSnapshot{
		Revision: strconv.FormatUint(client.ContextRevision, 10),
		Values: map[cmdpalette.ContextKey]any{
			cmdpalette.ContextWindowFocused:      context.WindowFocused,
			cmdpalette.ContextWindowFloating:     context.WindowFloating,
			cmdpalette.ContextWindowStacked:      context.WindowStacked,
			cmdpalette.ContextDesktopWindowCount: context.DesktopWindowCount,
			cmdpalette.ContextScopeGlobal:        context.ScopeGlobal,
			cmdpalette.ContextShellDesktop:       context.ShellDesktop,
			cmdpalette.ContextSessionState:       context.FocusedSessionState,
			cmdpalette.ContextWorkspaceTrusted:   context.WorkspaceTrusted,
		},
	}
}

type cmdPaletteActionExecutor struct {
	tools          toolspkg.Registry
	approvalTokens toolspkg.ApprovalTokenIssuer
	approvals      toolspkg.ApprovalCoordinator
	windowManager  windowmanager.Service
	approvalTTL    time.Duration
	now            func() time.Time
}

var (
	_ cmdpalette.ActionExecutor           = (*cmdPaletteActionExecutor)(nil)
	_ cmdpalette.ApprovalPreflight        = (*cmdPaletteActionExecutor)(nil)
	_ cmdpalette.ApprovalCompletionReader = (*cmdPaletteActionExecutor)(nil)
	_ toolspkg.ApprovalDispatcher         = (*cmdPaletteActionExecutor)(nil)
)

func (e *cmdPaletteActionExecutor) ApprovalCompletionStatus(
	ctx context.Context,
	approvalID string,
) (string, error) {
	status, err := e.approvals.Status(ctx, approvalID)
	if err != nil {
		return "", fmt.Errorf("cmd palette: read approval completion: %w", err)
	}
	switch status.ExecutionStatus {
	case toolspkg.ApprovalCompleted:
		return "ok", nil
	case toolspkg.ApprovalFailed:
		return hookAgentEventsFailedKey, nil
	case toolspkg.ApprovalUncertain:
		return "uncertain", nil
	}
	switch status.ApprovalStatus {
	case toolspkg.ApprovalDenied:
		return "denied", nil
	case toolspkg.ApprovalTimedOut:
		return "timeout", nil
	case toolspkg.ApprovalCanceled:
		return nativeMemoryAdminToolsCanceledKey, nil
	default:
		return "", nil
	}
}

func (e *cmdPaletteActionExecutor) ApprovalRequired(
	ctx context.Context,
	request cmdpalette.ExecutionRequest,
) (bool, error) {
	if request.Descriptor.Action.Kind != cmdpalette.ActionKindTool {
		return request.Descriptor.Destructive, nil
	}
	view, err := e.tools.Get(ctx, cmdPaletteToolScope(request), toolspkg.ToolID(request.Descriptor.Action.Tool))
	if err != nil {
		return false, fmt.Errorf("cmd palette: inspect tool policy: %w", err)
	}
	return view.Decision.ApprovalRequired, nil
}

func (e *cmdPaletteActionExecutor) ExecuteAction(
	ctx context.Context,
	request cmdpalette.ExecutionRequest,
) (cmdpalette.ExecutionResult, error) {
	requiresApproval, err := e.ApprovalRequired(ctx, request)
	if err != nil {
		return cmdpalette.ExecutionResult{}, err
	}
	if requiresApproval {
		return e.beginApproval(ctx, request)
	}
	result, err := e.dispatch(ctx, request, "")
	if err != nil {
		return cmdpalette.ExecutionResult{}, err
	}
	return cmdpalette.ExecutionResult{Result: result}, nil
}

func (e *cmdPaletteActionExecutor) beginApproval(
	ctx context.Context,
	request cmdpalette.ExecutionRequest,
) (cmdpalette.ExecutionResult, error) {
	if e.approvals == nil {
		return cmdpalette.ExecutionResult{}, errors.New("cmd palette: approval coordinator is unavailable")
	}
	args, err := json.Marshal(request.Args)
	if err != nil {
		return cmdpalette.ExecutionResult{}, fmt.Errorf("cmd palette: encode approval arguments: %w", err)
	}
	targetPayload, err := json.Marshal(cmdPaletteDeferredTarget{
		ClientID: request.ClientID, Action: request.Descriptor.Action,
	})
	if err != nil {
		return cmdpalette.ExecutionResult{}, fmt.Errorf("cmd palette: encode approval target: %w", err)
	}
	ticket, err := e.approvals.Begin(ctx, toolspkg.ApprovalRequest{
		WorkspaceID: string(request.WorkspaceID), InvocationID: request.InvocationID,
		CommandID: string(request.Descriptor.ID),
		Target: toolspkg.ApprovalTarget{
			Kind:   cmdPaletteApprovalTargetKind(request.Descriptor.Action.Kind),
			ToolID: toolspkg.ToolID(request.Descriptor.Action.Tool), Payload: targetPayload,
		},
		Args: args, ExpiresAt: e.now().UTC().Add(e.approvalTTL),
	})
	if err != nil {
		return cmdpalette.ExecutionResult{}, fmt.Errorf("cmd palette: begin approval: %w", err)
	}
	return cmdpalette.ExecutionResult{ApprovalID: ticket.ApprovalID, Completion: ticket.Completion}, nil
}

func (e *cmdPaletteActionExecutor) DispatchApproval(
	ctx context.Context,
	status toolspkg.ApprovalStatus,
) (json.RawMessage, error) {
	var target cmdPaletteDeferredTarget
	if err := json.Unmarshal(status.Target.Payload, &target); err != nil {
		return nil, fmt.Errorf("cmd palette: decode deferred approval target: %w", err)
	}
	var args map[string]any
	if err := json.Unmarshal(status.Args, &args); err != nil {
		return nil, fmt.Errorf("cmd palette: decode deferred approval arguments: %w", err)
	}
	request := cmdpalette.ExecutionRequest{
		WorkspaceID: cmdpalette.WorkspaceID(status.WorkspaceID), InvocationID: status.InvocationID,
		ClientID: target.ClientID, Descriptor: cmdpalette.Descriptor{
			ID: cmdpalette.CommandID(status.CommandID), Action: target.Action,
		},
		Args: args,
	}
	approvalToken := ""
	if target.Action.Kind == cmdpalette.ActionKindTool {
		if e.approvalTokens == nil {
			return nil, errors.New("cmd palette: approval token issuer is unavailable")
		}
		approvalInput, err := json.Marshal(mergedCmdPaletteArgs(target.Action.Args, args))
		if err != nil {
			return nil, fmt.Errorf("cmd palette: encode resumed approval arguments: %w", err)
		}
		grant, err := e.approvalTokens.CreateToolApproval(
			ctx,
			cmdPaletteToolScope(request),
			toolspkg.ApprovalTokenRequest{
				ToolID: toolspkg.ToolID(target.Action.Tool), SessionID: approvalSessionID(status.InvocationID),
				WorkspaceID: status.WorkspaceID, Input: approvalInput,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("cmd palette: mint resumed approval token: %w", err)
		}
		approvalToken = grant.ApprovalToken
	}
	return e.dispatch(ctx, request, approvalToken)
}

type cmdPaletteDeferredTarget struct {
	ClientID cmdpalette.ClientID `json:"client_id,omitempty"`
	Action   cmdpalette.Action   `json:"action"`
}

func (e *cmdPaletteActionExecutor) dispatch(
	ctx context.Context,
	request cmdpalette.ExecutionRequest,
	approvalToken string,
) (json.RawMessage, error) {
	if request.Descriptor.Action.Kind == cmdpalette.ActionKindTool {
		input, err := json.Marshal(mergedCmdPaletteArgs(request.Descriptor.Action.Args, request.Args))
		if err != nil {
			return nil, fmt.Errorf("cmd palette: encode tool arguments: %w", err)
		}
		result, err := e.tools.Call(ctx, cmdPaletteToolScope(request), toolspkg.CallRequest{
			ToolID: toolspkg.ToolID(request.Descriptor.Action.Tool), ToolCallID: request.InvocationID,
			SessionID: approvalSessionID(request.InvocationID), WorkspaceID: string(request.WorkspaceID),
			CorrelationID: request.InvocationID, Input: input,
			ApprovalToken: approvalToken,
		})
		if err != nil {
			return nil, fmt.Errorf("cmd palette: invoke tool: %w", err)
		}
		if len(result.Structured) > 0 {
			return append(json.RawMessage(nil), result.Structured...), nil
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("cmd palette: encode tool result: %w", err)
		}
		return encoded, nil
	}
	if e.windowManager == nil || request.ClientID == "" {
		return nil, cmdpalette.ErrNoAttachedShell
	}
	payload, err := json.Marshal(map[string]any{
		cmdPaletteActionKey: request.Descriptor.Action,
		"args":              mergedCmdPaletteArgs(request.Descriptor.Action.Args, request.Args),
	})
	if err != nil {
		return nil, fmt.Errorf("cmd palette: encode client command: %w", err)
	}
	response, err := e.windowManager.DispatchClientCommand(
		ctx, windowmanager.WorkspaceID(request.WorkspaceID), windowmanager.ClientID(request.ClientID),
		windowmanager.ClientCommand{
			CommandID: request.InvocationID, Op: cmdPaletteClientOp(request.Descriptor.Action), Payload: payload,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("cmd palette: dispatch client command: %w", err)
	}
	return append(json.RawMessage(nil), response.Result...), nil
}

func cmdPaletteToolScope(request cmdpalette.ExecutionRequest) toolspkg.Scope {
	return toolspkg.Scope{
		WorkspaceID: string(request.WorkspaceID), SessionID: approvalSessionID(request.InvocationID),
		ActorKind: "cmd_palette", Operator: true,
	}
}

func approvalSessionID(invocationID string) string { return "cmd-palette:" + invocationID }

func mergedCmdPaletteArgs(bound map[string]any, supplied map[string]any) map[string]any {
	merged := make(map[string]any, len(bound)+len(supplied))
	maps.Copy(merged, bound)
	maps.Copy(merged, supplied)
	return merged
}

func cmdPaletteApprovalTargetKind(kind cmdpalette.ActionKind) toolspkg.ApprovalTargetKind {
	switch kind {
	case cmdpalette.ActionKindTool:
		return toolspkg.ApprovalTargetTool
	case cmdpalette.ActionKindView:
		return toolspkg.ApprovalTargetView
	case cmdpalette.ActionKindNavigate, cmdpalette.ActionKindURL:
		return toolspkg.ApprovalTargetNavigate
	default:
		return toolspkg.ApprovalTargetClientOp
	}
}

func cmdPaletteClientOp(action cmdpalette.Action) string {
	switch action.Kind {
	case cmdpalette.ActionKindClientOp:
		return action.Op
	case cmdpalette.ActionKindView:
		return "view.open"
	case cmdpalette.ActionKindNavigate:
		return "navigate"
	case cmdpalette.ActionKindURL:
		return "url.open"
	default:
		return string(action.Kind)
	}
}
