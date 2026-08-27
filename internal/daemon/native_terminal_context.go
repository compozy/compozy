package daemon

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) nativeTerminalContext(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (terminalpkg.Manager, terminalpkg.Actor, string, error) {
	if n == nil || n.deps == nil || n.deps.Terminals == nil {
		return nil, terminalpkg.Actor{}, "", errors.New("terminal service is unavailable")
	}
	manager := n.deps.Terminals()
	if manager == nil {
		return nil, terminalpkg.Actor{}, "", errors.New("terminal service is unavailable")
	}
	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	if workspaceID == "" {
		return nil, terminalpkg.Actor{}, "", &terminalpkg.Error{
			Code:    "terminal_requires_workspace",
			Message: "terminal actions require a workspace",
			Err:     terminalpkg.ErrRequiresWorkspace,
		}
	}
	profileID := strings.TrimSpace(scope.ProfileID)
	if profileID == "" {
		return nil, terminalpkg.Actor{}, "", errors.New("terminal actor profile is unavailable")
	}
	sessionID := firstNonEmpty(scope.SessionID, req.SessionID)
	agentName := firstNonEmpty(scope.AgentName, req.AgentName)
	generation := int64(0)
	if sessionID != "" {
		if n.deps.Sessions == nil {
			return nil, terminalpkg.Actor{}, "", errors.New("session service is unavailable")
		}
		info, err := n.deps.Sessions.Status(ctx, sessionID)
		if err != nil {
			return nil, terminalpkg.Actor{}, "", err
		}
		if info == nil {
			return nil, terminalpkg.Actor{}, "", errors.New("terminal agent session is stale")
		}
		generation = info.RuntimeGeneration
		if agentName == "" {
			agentName = info.AgentName
		}
	}
	if agentName == "" {
		return nil, terminalpkg.Actor{}, "", errors.New("terminal agent identity is unavailable")
	}
	actor := terminalpkg.Actor{
		Kind: terminalpkg.ActorKindAgent, ID: agentName, ProfileID: profileID,
		SessionID: sessionID, RunID: strings.TrimSpace(req.TurnID), Generation: generation,
	}
	return manager, actor, workspaceID, nil
}

func (n *daemonNativeTools) nativeTerminalCapabilities(
	ctx context.Context,
	workspaceID string,
) (terminalpkg.Capabilities, error) {
	if n == nil || n.deps == nil || n.deps.Workspaces == nil {
		return terminalpkg.Capabilities{}, errors.New("workspace service is unavailable")
	}
	workspace, err := n.deps.Workspaces.Get(ctx, workspaceID)
	if err != nil {
		return terminalpkg.Capabilities{}, fmt.Errorf("terminal: resolve workspace capabilities: %w", err)
	}
	workspaceKind := terminalpkg.WorkspaceKindLocal
	if strings.TrimSpace(workspace.SandboxRef) != "" {
		workspaceKind = "sandbox"
	}
	return terminalpkg.ResolveCapabilities(runtime.GOOS, workspaceKind), nil
}

func (n *daemonNativeTools) terminalToolInfos(
	ctx context.Context,
	items []terminalpkg.Info,
) ([]terminalToolInfo, error) {
	projected := make([]terminalToolInfo, 0, len(items))
	profileNames := make(map[string]string)
	for _, item := range items {
		profileName, resolved := profileNames[item.ProfileID]
		if !resolved && n.deps.Profiles != nil {
			name, err := n.deps.Profiles.ProfileName(ctx, item.ProfileID)
			if err != nil {
				return nil, err
			}
			profileName = name
			profileNames[item.ProfileID] = name
		}
		projected = append(projected, contract.TerminalInfoPayloadFromDomain(item, profileName))
	}
	return projected, nil
}

func terminalToolError(id toolspkg.ToolID, err error) error {
	terminalErr, ok := errors.AsType[*terminalpkg.Error](err)
	if !ok || strings.TrimSpace(terminalErr.Code) == "" {
		return err
	}
	code := contract.TerminalErrorCode(strings.TrimSpace(terminalErr.Code))
	if !contract.IsTerminalErrorCode(code) {
		return nonContractTerminalToolError(id, terminalErr)
	}
	return terminalCodeToolError(id, terminalErr.Code, terminalErr.Error(), err)
}

func terminalCodeToolError(id toolspkg.ToolID, code, message string, err error) error {
	code = strings.TrimSpace(code)
	if code == string(toolspkg.ReasonApprovalRequired) {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeApprovalRequired,
			id,
			fmt.Sprintf("%s — %s", code, strings.TrimSpace(message)),
			err,
			toolspkg.ReasonApprovalRequired,
		)
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCode(code),
		id,
		fmt.Sprintf("%s — %s", code, strings.TrimSpace(message)),
		err,
		toolspkg.ReasonCode(code),
	)
}

func nonContractTerminalToolError(id toolspkg.ToolID, err *terminalpkg.Error) error {
	switch {
	case errors.Is(err, terminalpkg.ErrApprovalRequired):
		return terminalCodeToolError(id, string(toolspkg.ReasonApprovalRequired), err.Error(), err)
	case errors.Is(err, terminalpkg.ErrUnsupported):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			err.Error(),
			err,
			toolspkg.ReasonSchemaInvalid,
		)
	case errors.Is(err, terminalpkg.ErrShuttingDown):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			err.Error(),
			err,
			toolspkg.ReasonBackendUnhealthy,
		)
	default:
		return toolspkg.NewToolError(toolspkg.ErrorCodeBackendFailed, id, err.Error(), err)
	}
}
