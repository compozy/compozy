package daemon

import (
	"context"

	"strings"

	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (d *Daemon) persistSandboxReconcileMeta(
	ctx context.Context,
	state *bootState,
	candidate sandboxReconcileSession,
	envMeta *store.SessionSandboxMeta,
) {
	logger := sandboxReconcileLogger(state)
	next := candidate.meta
	next.Sandbox = cloneDaemonSessionSandboxMeta(envMeta)
	next.UpdatedAt = d.now().UTC()
	if next.CreatedAt.IsZero() {
		next.CreatedAt = next.UpdatedAt
	}
	if err := store.WriteSessionMeta(candidate.metaPath, next); err != nil {
		logger.Warn(
			"daemon: sandbox reconciliation metadata write failed",
			sandboxReconcileLogAttrs(candidate.meta, envMeta, 0, err)...,
		)
		return
	}
	if state == nil || state.registry == nil {
		return
	}
	if err := state.registry.RegisterSession(ctx, sessionInfoFromSandboxReconcileMeta(next)); err != nil {
		logger.Warn(
			"daemon: sandbox reconciliation session index update failed",
			sandboxReconcileLogAttrs(candidate.meta, envMeta, 0, err)...,
		)
	}
}

func (d *Daemon) resolveSandboxReconcileWorkspace(
	ctx context.Context,
	state *bootState,
	meta store.SessionMeta,
	envMeta *store.SessionSandboxMeta,
) (*workspacepkg.ResolvedWorkspace, bool) {
	if state == nil || state.workspaceResolver == nil {
		return nil, false
	}
	resolved, err := state.workspaceResolver.Resolve(ctx, strings.TrimSpace(meta.WorkspaceID))
	if err != nil {
		sandboxReconcileLogger(state).Warn(
			"daemon: sandbox reconciliation workspace resolve failed",
			sandboxReconcileLogAttrs(meta, envMeta, 0, err)...,
		)
		return nil, false
	}
	return &resolved, true
}

func sessionHasRemoteSandbox(meta store.SessionMeta) bool {
	if meta.Sandbox == nil {
		return false
	}
	backend := strings.TrimSpace(meta.Sandbox.Backend)
	if backend == "" {
		backend = string(sandbox.BackendLocal)
	}
	return sandbox.Backend(backend) != sandbox.BackendLocal
}

func sessionStateRecoverable(state string) bool {
	switch session.State(strings.TrimSpace(state)) {
	case session.StateStarting, session.StateActive, session.StateStopping:
		return true
	default:
		return false
	}
}

func sandboxReconcileResolvedSandbox(
	envMeta *store.SessionSandboxMeta,
	resolvedWorkspace *workspacepkg.ResolvedWorkspace,
	workspaceResolved bool,
) sandbox.Resolved {
	resolved := sandbox.Resolved{}
	if workspaceResolved && resolvedWorkspace != nil {
		resolved = resolvedWorkspace.Sandbox
	}
	backend := sandbox.Backend(strings.TrimSpace(envMeta.Backend))
	if backend.Valid() {
		resolved.Backend = backend
	}
	if !resolved.Backend.Valid() {
		resolved.Backend = sandbox.BackendLocal
	}
	if profile := strings.TrimSpace(envMeta.Profile); profile != "" {
		resolved.Profile = profile
	}
	if strings.TrimSpace(resolved.Profile) == "" {
		resolved.Profile = string(resolved.Backend)
	}
	return resolved
}

func sandboxReconcileLocalRoots(
	resolvedWorkspace *workspacepkg.ResolvedWorkspace,
	workspaceResolved bool,
) (string, []string) {
	if !workspaceResolved || resolvedWorkspace == nil {
		return "", nil
	}
	return strings.TrimSpace(resolvedWorkspace.RootDir), append([]string(nil), resolvedWorkspace.AdditionalDirs...)
}

func sandboxSessionStateFromMeta(meta *store.SessionSandboxMeta) sandbox.SessionState {
	if meta == nil {
		return sandbox.SessionState{}
	}
	return sandbox.SessionState{
		SandboxID:             strings.TrimSpace(meta.SandboxID),
		Backend:               sandbox.Backend(strings.TrimSpace(meta.Backend)),
		Profile:               strings.TrimSpace(meta.Profile),
		State:                 strings.TrimSpace(meta.State),
		InstanceID:            strings.TrimSpace(meta.InstanceID),
		RuntimeRootDir:        strings.TrimSpace(meta.RuntimeRootDir),
		RuntimeAdditionalDirs: append([]string(nil), meta.RuntimeAdditionalDirs...),
		ProviderState:         cloneDaemonRawMessage(meta.ProviderState),
		SSHAccessExpiresAt:    cloneDaemonTimePointer(meta.SSHAccessExpiresAt),
	}
}

func sandboxMetaFromSessionState(
	state sandbox.SessionState,
	fallbackState string,
) *store.SessionSandboxMeta {
	envState := strings.TrimSpace(state.State)
	if envState == "" || envState == sandboxReconcileFoundKey || envState == string(RestartStatusReady) {
		envState = fallbackState
	}
	return &store.SessionSandboxMeta{
		SandboxID:             strings.TrimSpace(state.SandboxID),
		Backend:               string(state.Backend),
		Profile:               strings.TrimSpace(state.Profile),
		State:                 envState,
		InstanceID:            strings.TrimSpace(state.InstanceID),
		RuntimeRootDir:        strings.TrimSpace(state.RuntimeRootDir),
		RuntimeAdditionalDirs: append([]string(nil), state.RuntimeAdditionalDirs...),
		ProviderState:         cloneDaemonRawMessage(state.ProviderState),
		SSHAccessExpiresAt:    cloneDaemonTimePointer(state.SSHAccessExpiresAt),
	}
}
