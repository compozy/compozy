package session

import (
	"context"
	"errors"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
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
// launch restarts on the carried context. Other errors surface.
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
	if native == nil {
		return candidate, err
	}
	if err == nil {
		session.settleNativeBootstrap(store.SessionNativeStateLoaded, "", m.now())
		return candidate, nil
	}
	if !nativeCloneLoadFailed(ctx, err) {
		return nil, err
	}
	session.settleNativeBootstrap(store.SessionNativeStateFailed, nativeLoadErrorText(err), m.now())
	m.sessionLogger(session).Info("session.fork.native_load_failed", "error", err)
	plan.spec.acpSessionID = ""
	plan.spec.resumeReplay = true
	plan.spec.resumeReplayReason = nativeBootstrapReplayFallback
	startOpts, prepareErr := m.prepareSessionLaunch(ctx, &plan.spec, session, runtime, nil)
	if prepareErr != nil {
		return nil, errors.Join(err, prepareErr)
	}
	candidate, fallbackErr := m.startAgentProcess(ctx, &plan.spec, startOpts)
	if fallbackErr != nil {
		return nil, errors.Join(err, fallbackErr)
	}
	return candidate, nil
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
