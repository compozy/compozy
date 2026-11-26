package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/toolmeta"
)

const (
	nativeBootstrapRouteMismatch  = "route_mismatch: the resolved route differs from the route the clone was created under"
	nativeBootstrapReplayFallback = "native_fork_load_failed"
)

// prepareNativeBootstrapBind applies a pending native clone to the first bind of a fork
// child. The clone is loaded only under the provider identity it was pinned to; a route
// that does not match settles the bootstrap failed (route_mismatch) and the bind replays
// the carried context. It returns the bootstrap the launch must settle, or nil.
func (m *Manager) prepareNativeBootstrapBind(
	session *Session,
	snapshot *runtimeBindingSnapshot,
	plan *promptRuntimePlan,
) *store.SessionNativeBootstrap {
	if snapshot.process != nil {
		return nil
	}
	native := session.pendingNativeBootstrap()
	if native == nil {
		return nil
	}
	if !nativeBootstrapMatchesRoute(*native, plan.runtime.agent) {
		session.settleNativeBootstrap(store.SessionNativeStateFailed, nativeBootstrapRouteMismatch, m.now())
		m.sessionLogger(session).Info("session.fork.native_route_mismatch", "provider", plan.runtime.agent.Provider)
		return nil
	}
	plan.spec.acpSessionID = native.ACPSessionID
	plan.spec.resumeReplay = false
	return native
}

// launchPromptRuntimeCandidate starts the binding's process. For a pending native clone
// the launch runs session/load of the clone id and settles the bootstrap: loaded on
// success; failed when the clone cannot load (see nativeCloneLoadFailed), after which the
// launch restarts on the carried context. A released idle runtime reloads its own ACP
// session the same way and replays the transcript when the load fails. Other errors surface.
func (m *Manager) launchPromptRuntimeCandidate(
	ctx context.Context,
	session *Session,
	plan *promptRuntimePlan,
	runtime *sessionStartRuntime,
	native *store.SessionNativeBootstrap,
) (*AgentProcess, error) {
	startOpts, err := m.prepareSessionLaunch(ctx, &plan.spec, session, runtime, nil)
	if err != nil {
		return nil, err
	}
	candidate, err := m.startAgentProcess(ctx, &plan.spec, startOpts)
	reload := native == nil && plan.reloadACPSessionID != ""
	if native == nil && !reload {
		return candidate, err
	}
	if err == nil {
		if native != nil {
			session.settleNativeBootstrap(store.SessionNativeStateLoaded, "", m.now())
		}
		return candidate, nil
	}
	if !nativeCloneLoadFailed(ctx, err) {
		return nil, err
	}
	plan.spec.resumeReplayReason = resumeFallbackReason(err)
	if native != nil {
		session.settleNativeBootstrap(store.SessionNativeStateFailed, nativeLoadErrorText(err), m.now())
		m.sessionLogger(session).Info("session.fork.native_load_failed", "error", err)
		plan.spec.resumeReplayReason = nativeBootstrapReplayFallback
	} else {
		m.sessionLogger(session).Info("session.runtime.rebind_load_failed", "error", err)
	}
	plan.spec.acpSessionID = ""
	plan.spec.resumeReplay = true
	loadTimedOut := acp.IsLoadSessionTimeout(err)
	replayFailure := func(fallbackErr error) error {
		if loadTimedOut && ctx.Err() == nil {
			return &SessionRebindTimeoutError{LoadTimeout: err, Fallback: fallbackErr}
		}
		return errors.Join(err, fallbackErr)
	}
	if remintErr := m.remintHostedMCPForReplay(ctx, plan, runtime); remintErr != nil {
		return nil, replayFailure(remintErr)
	}
	startOpts, prepareErr := m.prepareSessionLaunch(ctx, &plan.spec, session, runtime, nil)
	if prepareErr != nil {
		return nil, replayFailure(prepareErr)
	}
	candidate, fallbackErr := m.startAgentProcess(ctx, &plan.spec, startOpts)
	if fallbackErr != nil {
		return nil, replayFailure(fallbackErr)
	}
	return candidate, nil
}

func (m *Manager) remintHostedMCPForReplay(
	ctx context.Context,
	plan *promptRuntimePlan,
	runtime *sessionStartRuntime,
) error {
	if m.hostedMCP == nil {
		return nil
	}
	for index, server := range runtime.mcpServers {
		if server.Name != toolmeta.HostedServerName ||
			server.Transport != compozyconfig.MCPServerTransportStdio {
			continue
		}
		// The failed load process released its proxy bind, so transcript replay needs a newly
		// minted hosted launch rather than reusing the launch entry that process consumed.
		hosted, err := m.hostedMCP.Launch(ctx, HostedMCPLaunchRequest{
			EagerTools:  true,
			SessionID:   plan.spec.sessionID,
			ProfileID:   plan.spec.profileID,
			WorkspaceID: plan.spec.workspace.ID,
			AgentName:   runtime.agent.Name,
		})
		if err != nil {
			return errors.Join(
				ErrHostedMCPUnavailable,
				fmt.Errorf("session: re-mint hosted MCP launch for transcript replay: %w", err),
			)
		}
		runtime.mcpServers = append([]compozyconfig.MCPServer(nil), runtime.mcpServers...)
		runtime.mcpServers[index] = hosted
		return nil
	}
	return nil
}

// nativeCloneLoadFailed reports a clone the child's own process could not load (ADR-003):
// the agent does not support session/load, or session/load failed for any reason — a
// missing clone, or a clone the forking process still holds open (codex-acp answers
// "thread … already has an active writer" until the source process exits). A canceled
// caller and failures before session/load (launch, initialize) are not clone failures.
func nativeCloneLoadFailed(ctx context.Context, err error) bool {
	if errors.Is(err, acp.ErrAgentDoesNotSupportSession) {
		return true
	}
	return errors.Is(err, acp.ErrLoadSessionFailed) && ctx.Err() == nil
}

// nativeLoadErrorText renders a failed clone load as `session/load: <agent message>`,
// redacted and bounded like a native fork error.
func nativeLoadErrorText(err error) string {
	message := "not supported by the agent"
	if reqErr, ok := errors.AsType[*acpsdk.RequestError](err); ok && reqErr != nil {
		message = strings.ToLower(strings.TrimSpace(reqErr.Message))
	}
	return nativeForkErrorText(errors.New("session/load: "+message), "")
}
