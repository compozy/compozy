package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/session"
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
	exec sandboxExecSessionManager
}

type hostAPINetworkSessionManagerAdapter struct {
	hostAPISessionManagerAdapter
	bridgePrompts hostAPIBridgePromptSessionManager
}

var (
	_ hostAPIPromptOptsSessionManager = (*hostAPISessionManagerAdapter)(nil)
	_ hostAPIPromptOptsSessionManager = (*hostAPINetworkSessionManagerAdapter)(nil)
)

func newHostAPISessionManagerAdapter(sessions SessionManager) hostAPIExtensionSessionManager {
	adapter := hostAPISessionManagerAdapter{SessionManager: sessions}
	if exec, ok := sessions.(sandboxExecSessionManager); ok {
		adapter.exec = exec
	}
	if bridgePrompts, ok := sessions.(hostAPIBridgePromptSessionManager); ok {
		return hostAPINetworkSessionManagerAdapter{
			hostAPISessionManagerAdapter: adapter,
			bridgePrompts:                bridgePrompts,
		}
	}
	return adapter
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
