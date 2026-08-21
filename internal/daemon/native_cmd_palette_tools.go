package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeCmdPaletteListInput struct {
	Workspace string `json:"workspace,omitempty"`
	Source    string `json:"source,omitempty"`
	Client    string `json:"client,omitempty"`
}

type nativeCmdPaletteInvokeInput struct {
	ID        string         `json:"id"`
	Workspace string         `json:"workspace,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
	Client    string         `json:"client,omitempty"`
}

func (n *daemonNativeTools) cmdPaletteRegistry() cmdpalette.Registry {
	if n == nil || n.deps == nil || n.deps.CmdPalette == nil {
		return nil
	}
	return n.deps.CmdPalette()
}

func (n *daemonNativeTools) cmdPaletteList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeCmdPaletteListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeCmdPaletteWorkspaceID(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	profileLens, err := n.nativeCmdPaletteProfileLens(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeCmdPaletteError(req.ToolID, err)
	}
	registry := n.cmdPaletteRegistry()
	if registry == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "command palette is unavailable")
	}
	catalog, err := registry.Catalog(ctx, cmdpalette.CatalogRequest{
		ProfileLens: profileLens,
		WorkspaceID: cmdpalette.WorkspaceID(workspaceID),
		ClientID:    cmdpalette.ClientID(strings.TrimSpace(input.Client)),
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeCmdPaletteError(req.ToolID, err)
	}
	commands := contract.CmdPaletteCommandsFromDomain(catalog).Commands
	if source := strings.TrimSpace(input.Source); source != "" {
		filtered := make([]contract.CmdPaletteCommand, 0, len(commands))
		for _, command := range commands {
			if command.Source == source {
				filtered = append(filtered, command)
			}
		}
		commands = filtered
	}
	return structuredResult(map[string]any{"commands": commands}, fmt.Sprintf("%d commands", len(commands)))
}

func (n *daemonNativeTools) cmdPaletteInvoke(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeCmdPaletteInvokeInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	commandID, err := requiredNativeString(req.ToolID, "id", input.ID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeCmdPaletteWorkspaceID(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	profileLens, err := n.nativeCmdPaletteProfileLens(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeCmdPaletteError(req.ToolID, err)
	}
	registry := n.cmdPaletteRegistry()
	if registry == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "command palette is unavailable")
	}
	result, err := registry.Invoke(ctx, cmdpalette.InvokeRequest{
		ProfileLens: profileLens,
		WorkspaceID: cmdpalette.WorkspaceID(workspaceID),
		CommandID:   cmdpalette.CommandID(commandID),
		Args:        input.Args,
		ClientID:    cmdpalette.ClientID(strings.TrimSpace(input.Client)),
		Caller:      cmdpalette.CallerControlPlane,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeCmdPaletteError(req.ToolID, err)
	}
	payload := contract.CmdPaletteInvokeResult{
		Status: result.Status, Result: result.Result, ApprovalID: result.ApprovalID,
	}
	return structuredResult(payload, string(result.Status))
}

func (n *daemonNativeTools) nativeCmdPaletteProfileLens(
	ctx context.Context,
	scope toolspkg.Scope,
) (cmdpalette.ProfileLens, error) {
	profileID, _, _, err := n.nativeCurrentProfileIdentity(ctx, scope)
	if err != nil {
		return cmdpalette.ProfileLens{}, err
	}
	profileName := ""
	if n != nil && n.deps != nil && n.deps.Profiles != nil {
		profileName, err = n.deps.Profiles.ProfileName(ctx, profileID)
		if err != nil {
			return cmdpalette.ProfileLens{}, err
		}
	} else if profileID == store.DefaultProfileID {
		profileName = daemonDefaultProfileName
	}
	if strings.TrimSpace(profileName) == "" {
		return cmdpalette.ProfileLens{}, errors.New("daemon: command palette profile name is unavailable")
	}
	return cmdpalette.ScopedProfileLens(cmdpalette.ProfileLensID(profileID), profileName), nil
}

func (n *daemonNativeTools) nativeCmdPaletteWorkspaceID(
	ctx context.Context,
	id toolspkg.ToolID,
	workspaceRef string,
	scope toolspkg.Scope,
) (string, error) {
	bound, err := n.nativeBoundSession(ctx, scope)
	if err != nil {
		return "", nativeCmdPaletteError(id, err)
	}
	ref := strings.TrimSpace(workspaceRef)
	if ref == "" && bound != nil {
		ref = bound.workspaceID
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, id, ref, scope)
	if err != nil {
		return "", err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return "", nativeNetworkInputError(id, err)
	}
	if bound != nil && workspaceID != bound.workspaceID {
		return "", nativeScopeMismatchError(id, nativeWorkspaceInputKey)
	}
	return workspaceID, nil
}

func nativeCmdPaletteError(id toolspkg.ToolID, err error) error {
	var invalidArguments *cmdpalette.InvalidArgumentsError
	var unavailable *cmdpalette.UnavailableError
	switch {
	case errors.Is(err, cmdpalette.ErrCommandNotFound):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound, id, err.Error(), fmt.Errorf("%w: %w", toolspkg.ErrToolNotFound, err),
		)
	case errors.As(err, &invalidArguments), errors.Is(err, cmdpalette.ErrCannotDeferSecrets):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput, id, err.Error(), fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	case errors.Is(err, cmdpalette.ErrAlreadyRunning), errors.Is(err, cmdpalette.ErrMultipleClients):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict, id, err.Error(), fmt.Errorf("%w: %w", toolspkg.ErrToolConflict, err),
		)
	case errors.Is(err, cmdpalette.ErrClientUnauthorized):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied, id, err.Error(), fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
			toolspkg.ReasonScopeMismatch,
		)
	case errors.Is(err, cmdpalette.ErrNoAttachedShell), errors.As(err, &unavailable):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable, id, err.Error(), fmt.Errorf("%w: %w", toolspkg.ErrToolUnavailable, err),
			toolspkg.ReasonDependencyMissing,
		)
	default:
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed, id, err.Error(), fmt.Errorf("%w: %w", toolspkg.ErrToolBackendFailed, err),
			toolspkg.ReasonBackendUnhealthy,
		)
	}
}
