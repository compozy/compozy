package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	profilepkg "github.com/compozy/compozy/internal/profile"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) nativeTerminalContext(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (terminalpkg.Manager, terminalpkg.Actor, string, error) {
	if n == nil || n.deps == nil || n.deps.Terminals == nil {
		return nil, terminalpkg.Actor{}, "", fmt.Errorf(
			"terminal service is unavailable: %w", terminalpkg.ErrServiceUnavailable,
		)
	}
	manager := n.deps.Terminals()
	if manager == nil {
		return nil, terminalpkg.Actor{}, "", fmt.Errorf(
			"terminal service is unavailable: %w", terminalpkg.ErrServiceUnavailable,
		)
	}
	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	if workspaceID == "" {
		return nil, terminalpkg.Actor{}, "", &terminalpkg.Error{
			Code:    terminalpkg.ErrorCodeRequiresWorkspace,
			Message: "terminal actions require a workspace",
			Err:     terminalpkg.ErrRequiresWorkspace,
		}
	}
	profileID := strings.TrimSpace(scope.ProfileID)
	if profileID == "" {
		return nil, terminalpkg.Actor{}, "", toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			req.ToolID,
			"terminal profile scope is required",
			toolspkg.ErrToolInvalidInput,
			toolspkg.ReasonSchemaInvalid,
		)
	}
	actor, err := n.nativeTerminalActor(ctx, workspaceID, profileID, req)
	if err != nil {
		return nil, terminalpkg.Actor{}, "", err
	}
	return manager, actor, workspaceID, nil
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
	if toolErr, ok := errors.AsType[*toolspkg.ToolError](err); ok {
		return toolErr
	}
	if profileErr, ok := errors.AsType[*profilepkg.Error](err); ok {
		var code terminalpkg.ErrorCode
		switch {
		case errors.Is(profileErr, profilepkg.ErrArchived):
			code = terminalpkg.ErrorCodeProfileArchived
		case errors.Is(profileErr, profilepkg.ErrUnavailable):
			code = terminalpkg.ErrorCodeProfileUnavailable
		case errors.Is(profileErr, profilepkg.ErrSessionConflict):
			code = terminalpkg.ErrorCodeProfileSessionConflict
		}
		if code != "" {
			return terminalCodeToolError(id, code, profileErr.Message, &terminalpkg.Error{
				Code: code, Message: profileErr.Message, Action: profileErr.Action, Err: profileErr,
			})
		}
	}
	terminalErr, ok := errors.AsType[*terminalpkg.Error](err)
	if !ok {
		return nonContractTerminalToolError(id, err)
	}
	if strings.TrimSpace(string(terminalErr.Code)) == "" {
		return nonContractTerminalToolError(id, terminalErr)
	}
	code := contract.TerminalErrorCode(strings.TrimSpace(string(terminalErr.Code)))
	if !contract.IsTerminalErrorCode(code) {
		return nonContractTerminalToolError(id, terminalErr)
	}
	return terminalCodeToolError(id, terminalErr.Code, terminalErr.Error(), err)
}

func terminalSessionDeniedError(id toolspkg.ToolID) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		"terminal agent session identity is unavailable",
		terminalpkg.ErrRunIdentityIncomplete,
		toolspkg.ReasonSessionDenied,
	)
}

func terminalCodeToolError(
	id toolspkg.ToolID,
	code terminalpkg.ErrorCode,
	message string,
	err error,
) error {
	if !terminalpkg.IsErrorCode(code) {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed, id, strings.TrimSpace(message), err,
			toolspkg.ReasonBackendUnhealthy,
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

func nonContractTerminalToolError(id toolspkg.ToolID, err error) error {
	switch {
	case errors.Is(err, terminalpkg.ErrApprovalRequired):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeApprovalRequired, id, err.Error(), err, toolspkg.ReasonApprovalRequired,
		)
	case errors.Is(err, terminalpkg.ErrPolicyDenied):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied, id, err.Error(), err, toolspkg.ReasonPolicyDenied,
		)
	case errors.Is(err, terminalpkg.ErrRunIdentityIncomplete):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied, id, err.Error(), err, toolspkg.ReasonSessionDenied,
		)
	case errors.Is(err, terminalpkg.ErrUnsupported):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput, id, err.Error(), err, toolspkg.ReasonSchemaInvalid,
		)
	case errors.Is(err, terminalpkg.ErrShuttingDown):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable, id, err.Error(), err, toolspkg.ReasonBackendUnhealthy,
		)
	case errors.Is(err, terminalpkg.ErrServiceUnavailable):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable, id, err.Error(), err, toolspkg.ReasonBackendUnhealthy,
		)
	case errors.Is(err, terminalpkg.ErrInputPending),
		errors.Is(err, terminalpkg.ErrInputResolved),
		errors.Is(err, terminalpkg.ErrInputResolving),
		errors.Is(err, terminalpkg.ErrWriteLeaseRequired):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict, id, err.Error(), err,
		)
	default:
		return toolspkg.NewToolError(toolspkg.ErrorCodeBackendFailed, id, err.Error(), err)
	}
}
