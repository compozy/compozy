package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
	builtintools "github.com/compozy/compozy/internal/tools/builtin"
)

type terminalExecApprover interface {
	ApproveTerminalExec(context.Context, toolspkg.Scope, toolspkg.CallRequest) (string, error)
}

type terminalPermissionBridge struct {
	mu       sync.RWMutex
	approval toolspkg.ApprovalBridge
}

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
	view := &toolspkg.ToolView{Descriptor: terminalToolDescriptor(call.ToolID)}
	if err := approval.RequestToolApproval(ctx, scope, &call, view); err != nil {
		return "", err
	}
	label := strings.TrimSpace(call.ApprovalLabel)
	if label == "" {
		label = toolApprovalApprovedOnceLabel
	}
	return label, nil
}

func terminalToolDescriptor(id toolspkg.ToolID) toolspkg.Descriptor {
	for _, descriptor := range builtintools.NativeDescriptors() {
		if descriptor.ID == id {
			return descriptor
		}
	}
	return toolspkg.Descriptor{ID: id}
}

func (b *terminalPermissionBridge) AuthorizeTerminalExec(
	ctx context.Context,
	request terminalpkg.ExecRequest,
	classification terminalpkg.CommandClassification,
) (string, error) {
	input, err := json.Marshal(map[string]any{
		"command":          request.Command,
		nativeToolsArgsKey: request.Args,
		"cwd":              request.Cwd,
		"env":              request.Env,
		"visible":          request.Visible,
		"risk":             terminalPermissionRisk(classification),
	})
	if err != nil {
		return "", fmt.Errorf("prepare terminal exec approval: %w", err)
	}
	scope := toolspkg.Scope{
		ProfileID: request.Actor.ProfileID, WorkspaceID: request.WS, SessionID: request.Actor.SessionID,
		RunID: request.Actor.RunID, Generation: request.Actor.Generation,
		AgentName: request.Actor.ID, ActorKind: string(request.Actor.Kind),
	}
	call := toolspkg.CallRequest{
		ToolID: toolspkg.ToolIDTerminalExec, ProfileID: request.Actor.ProfileID,
		WorkspaceID: request.WS, SessionID: request.Actor.SessionID,
		RunID: request.Actor.RunID, Generation: request.Actor.Generation,
		AgentName: request.Actor.ID, ActorKind: string(request.Actor.Kind), Input: input,
	}
	label, err := b.ApproveTerminalExec(ctx, scope, call)
	if errors.Is(err, errToolApprovalRejected) {
		return "", &terminalpkg.Error{
			Code: terminalpkg.ErrorCodeApprovalRejected, Message: "terminal command approval was rejected",
			Err: terminalpkg.ErrApprovalRejected,
		}
	}
	return label, err
}

func terminalPermissionRisk(classification terminalpkg.CommandClassification) string {
	if terminalApprovalMustBeForced(classification) {
		return classification.Reason
	}
	return "ordinary"
}

func terminalApprovalMustBeForced(classification terminalpkg.CommandClassification) bool {
	return classification.Reason == "irreversible" || classification.Reason == "unclassifiable"
}

func (b *terminalPermissionBridge) current() toolspkg.ApprovalBridge {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.approval
}
