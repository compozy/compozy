package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
)

type sandboxExecSessionManager interface {
	ExecSandbox(context.Context, session.SandboxExecRequest) (session.SandboxExecResult, error)
}

type hostAPIExtensionSessionManager interface {
	SessionManager
	ExecSandbox(context.Context, session.SandboxExecRequest) (session.SandboxExecResult, error)
}

type hostAPIBridgePromptSessionManager interface {
	PromptNetwork(
		ctx context.Context,
		sessionID string,
		message string,
		meta ...acp.PromptNetworkMeta,
	) (<-chan acp.AgentEvent, error)
	IsPrompting(sessionID string) bool
}

type hostAPIPromptOptsSessionManager interface {
	PromptWithOpts(
		ctx context.Context,
		id string,
		opts session.PromptOpts,
	) (<-chan acp.AgentEvent, error)
}

type hostAPISessionManagerAdapter struct {
	core.SessionManager
	exec             sandboxExecSessionManager
	archive          core.SessionArchiveManager
	runtimeSelection core.SessionRuntimeSelectionManager
}

type hostAPINetworkSessionManagerAdapter struct {
	hostAPISessionManagerAdapter
	bridgePrompts hostAPIBridgePromptSessionManager
}

type hostAPISessionAcceptance struct {
	acceptance core.SessionAcceptanceManager
}

type hostAPIAcceptanceSessionManagerAdapter struct {
	hostAPISessionManagerAdapter
	hostAPISessionAcceptance
}

type hostAPIAcceptanceNetworkSessionManagerAdapter struct {
	hostAPINetworkSessionManagerAdapter
	hostAPISessionAcceptance
}

var (
	_ hostAPIPromptOptsSessionManager           = (*hostAPISessionManagerAdapter)(nil)
	_ hostAPIPromptOptsSessionManager           = (*hostAPINetworkSessionManagerAdapter)(nil)
	_ core.SessionArchiveManager                = (*hostAPISessionManagerAdapter)(nil)
	_ core.SessionRuntimeSelectionManager       = (*hostAPISessionManagerAdapter)(nil)
	_ core.SessionAcceptanceManager             = (*hostAPIAcceptanceSessionManagerAdapter)(nil)
	_ core.SessionAcceptanceManager             = (*hostAPIAcceptanceNetworkSessionManagerAdapter)(nil)
	_ core.SessionWorktreeForkAcceptanceManager = (*hostAPIAcceptanceSessionManagerAdapter)(nil)
	_ core.SessionWorktreeForkAcceptanceManager = (*hostAPIAcceptanceNetworkSessionManagerAdapter)(nil)
)

func newHostAPISessionManagerAdapter(sessions SessionManager) hostAPIExtensionSessionManager {
	adapter := hostAPISessionManagerAdapter{SessionManager: sessions}
	if exec, ok := sessions.(sandboxExecSessionManager); ok {
		adapter.exec = exec
	}
	if archive, ok := sessions.(core.SessionArchiveManager); ok {
		adapter.archive = archive
	}
	if runtimeSelection, ok := sessions.(core.SessionRuntimeSelectionManager); ok {
		adapter.runtimeSelection = runtimeSelection
	}
	bridgePrompts, supportsBridgePrompts := sessions.(hostAPIBridgePromptSessionManager)
	acceptance, supportsAcceptance := sessions.(core.SessionAcceptanceManager)
	networkAdapter := hostAPINetworkSessionManagerAdapter{
		hostAPISessionManagerAdapter: adapter,
		bridgePrompts:                bridgePrompts,
	}
	acceptanceAdapter := hostAPISessionAcceptance{acceptance: acceptance}
	switch {
	case supportsBridgePrompts && supportsAcceptance:
		return hostAPIAcceptanceNetworkSessionManagerAdapter{
			hostAPINetworkSessionManagerAdapter: networkAdapter,
			hostAPISessionAcceptance:            acceptanceAdapter,
		}
	case supportsBridgePrompts:
		return networkAdapter
	case supportsAcceptance:
		return hostAPIAcceptanceSessionManagerAdapter{
			hostAPISessionManagerAdapter: adapter,
			hostAPISessionAcceptance:     acceptanceAdapter,
		}
	default:
		return adapter
	}
}

func (a hostAPISessionManagerAdapter) Archive(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.Info, error) {
	if a.archive == nil {
		return nil, errors.New("daemon: session manager does not support session archiving")
	}
	return a.archive.Archive(ctx, workspaceID, sessionID)
}

func (a hostAPISessionManagerAdapter) Unarchive(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.Info, error) {
	if a.archive == nil {
		return nil, errors.New("daemon: session manager does not support session archiving")
	}
	return a.archive.Unarchive(ctx, workspaceID, sessionID)
}

func (a hostAPISessionManagerAdapter) SetRuntimeSelection(
	ctx context.Context,
	id string,
	selection session.RuntimeSelection,
	expectedRevision int64,
) (*session.Info, error) {
	if a.runtimeSelection == nil {
		return nil, errors.New("daemon: session manager does not support runtime selection")
	}
	return a.runtimeSelection.SetRuntimeSelection(ctx, id, selection, expectedRevision)
}

func (a hostAPISessionManagerAdapter) ClearRuntimeSelection(
	ctx context.Context,
	id string,
	expectedRevision int64,
) (*session.Info, error) {
	if a.runtimeSelection == nil {
		return nil, errors.New("daemon: session manager does not support runtime selection")
	}
	return a.runtimeSelection.ClearRuntimeSelection(ctx, id, expectedRevision)
}

func (a hostAPISessionAcceptance) CreateAccepted(
	ctx context.Context,
	opts session.CreateAcceptedOpts,
) (*session.Info, error) {
	info, err := a.acceptance.CreateAccepted(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("daemon: host api CreateAccepted: %w", err)
	}
	return info, nil
}

func (a hostAPISessionAcceptance) CreateWorktreeForkAccepted(
	ctx context.Context,
	originSessionID string,
	opts session.CreateAcceptedOpts,
) (*session.Info, error) {
	acceptance, ok := a.acceptance.(core.SessionWorktreeForkAcceptanceManager)
	if !ok {
		return nil, errors.New("daemon: atomic worktree fork acceptance is unavailable")
	}
	info, err := acceptance.CreateWorktreeForkAccepted(ctx, originSessionID, opts)
	if err != nil {
		return nil, fmt.Errorf("daemon: host api CreateWorktreeForkAccepted: %w", err)
	}
	return info, nil
}

func (a hostAPISessionManagerAdapter) ExecSandbox(
	ctx context.Context,
	req session.SandboxExecRequest,
) (session.SandboxExecResult, error) {
	if a.exec == nil {
		return session.SandboxExecResult{}, session.ErrSessionNotActive
	}
	return a.exec.ExecSandbox(ctx, req)
}

func (a hostAPINetworkSessionManagerAdapter) PromptNetwork(
	ctx context.Context,
	sessionID string,
	message string,
	meta ...acp.PromptNetworkMeta,
) (<-chan acp.AgentEvent, error) {
	return a.bridgePrompts.PromptNetwork(ctx, sessionID, message, meta...)
}

func (a hostAPINetworkSessionManagerAdapter) IsPrompting(sessionID string) bool {
	return a.bridgePrompts.IsPrompting(sessionID)
}

// PromptWithOpts forwards provenance-aware prompts to the concrete session manager.
// Bridge ingress relies on this path so Local bridge sessions keep turn_source=network
// metadata without requiring Live NetworkParticipation.
func (a hostAPISessionManagerAdapter) PromptWithOpts(
	ctx context.Context,
	id string,
	opts session.PromptOpts,
) (<-chan acp.AgentEvent, error) {
	prompter, ok := a.SessionManager.(hostAPIPromptOptsSessionManager)
	if !ok {
		return nil, errors.New("daemon: session manager does not support PromptWithOpts")
	}
	events, err := prompter.PromptWithOpts(ctx, id, opts)
	if err != nil {
		return nil, fmt.Errorf("daemon: host api PromptWithOpts: %w", err)
	}
	return events, nil
}
