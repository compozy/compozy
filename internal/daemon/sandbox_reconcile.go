package daemon

import (
	"context"

	"errors"

	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/sandbox"

	"github.com/compozy/agh/internal/store"
)

const (
	sandboxReconcileFoundKey       = "found"
	sandboxReconcileWorkspaceIDKey = "workspace_id"
)

const (
	sandboxReconcileStatePrepared  = "prepared"
	sandboxReconcileStateDestroyed = "destroyed"
)

type sandboxReconcileSession struct {
	metaPath string
	meta     store.SessionMeta
}

func (d *Daemon) reconcileDaemonSandboxes(ctx context.Context, state *bootState) {
	logger := sandboxReconcileLogger(state)
	if ctx == nil {
		ctx = context.Background()
	}
	if state == nil {
		logger.Warn("daemon: sandbox reconciliation skipped", "error", "boot state is required")
		return
	}
	if state.sandboxRegistry == nil {
		logger.Warn("daemon: sandbox reconciliation skipped", "error", "sandbox registry is required")
		return
	}

	sessions, err := d.loadSandboxReconcileSessions(state)
	if err != nil {
		logger.Warn("daemon: sandbox reconciliation failed to load sessions", "error", err)
		return
	}

	for _, candidate := range sessions {
		if err := ctx.Err(); err != nil {
			logger.Warn("daemon: sandbox reconciliation canceled", "error", err)
			return
		}
		d.reconcileDaemonSandboxSession(ctx, state, candidate)
	}

	logger.Info("daemon: sandbox reconciliation complete", "sessions", len(sessions))
}

func (d *Daemon) loadSandboxReconcileSessions(
	state *bootState,
) ([]sandboxReconcileSession, error) {
	entries, err := os.ReadDir(d.homePaths.SessionsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	logger := sandboxReconcileLogger(state)
	sessions := make([]sandboxReconcileSession, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		metaPath := store.SessionMetaFile(filepath.Join(d.homePaths.SessionsDir, entry.Name()))
		meta, err := store.ReadSessionMeta(metaPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			logger.Warn(
				"daemon: sandbox reconciliation skipped unreadable session metadata",
				daemonPayloadSessionIDKey, strings.TrimSpace(entry.Name()),
				"path", metaPath,
				"error", err,
			)
			continue
		}
		if !sessionHasRemoteSandbox(meta) {
			continue
		}
		sessions = append(sessions, sandboxReconcileSession{metaPath: metaPath, meta: meta})
	}
	return sessions, nil
}

func (d *Daemon) reconcileDaemonSandboxSession(
	ctx context.Context,
	state *bootState,
	candidate sandboxReconcileSession,
) {
	meta := candidate.meta
	envMeta := cloneDaemonSessionSandboxMeta(meta.Sandbox)
	logger := sandboxReconcileLogger(state)
	if envMeta == nil {
		return
	}

	backend := sandbox.Backend(strings.TrimSpace(envMeta.Backend))
	provider, err := state.sandboxRegistry.Provider(backend)
	if err != nil {
		logger.Warn(
			"daemon: sandbox reconciliation provider unavailable",
			sandboxReconcileLogAttrs(meta, envMeta, 0, err)...,
		)
		return
	}

	resolvedWorkspace, workspaceResolved := d.resolveSandboxReconcileWorkspace(ctx, state, meta, envMeta)
	resolvedEnv := sandboxReconcileResolvedSandbox(envMeta, resolvedWorkspace, workspaceResolved)
	localRoot, localAdditional := sandboxReconcileLocalRoots(resolvedWorkspace, workspaceResolved)

	stateForProvider := sandboxSessionStateFromMeta(envMeta)
	if strings.TrimSpace(stateForProvider.InstanceID) == "" && strings.TrimSpace(envMeta.SandboxID) != "" {
		if found, ok := d.findDaemonSandbox(
			ctx,
			state,
			provider,
			meta,
			envMeta,
			resolvedEnv,
			localRoot,
			localAdditional,
		); ok {
			stateForProvider = mergeSandboxSessionState(stateForProvider, found)
			envMeta = sandboxMetaFromSessionState(stateForProvider, envMeta.State)
		}
	}
	if strings.TrimSpace(stateForProvider.InstanceID) == "" {
		logger.Warn(
			"daemon: sandbox reconciliation skipped missing instance",
			sandboxReconcileLogAttrs(
				meta,
				envMeta,
				0,
				errors.New("sandbox instance id is required"),
			)...,
		)
		return
	}

	if !sessionStateRecoverable(meta.State) {
		d.destroyDaemonSandbox(ctx, state, provider, candidate, envMeta, stateForProvider)
		return
	}

	d.reattachDaemonSandbox(
		ctx,
		state,
		provider,
		candidate,
		envMeta,
		stateForProvider,
		resolvedEnv,
		localRoot,
		localAdditional,
	)
}

func (d *Daemon) findDaemonSandbox(
	ctx context.Context,
	state *bootState,
	provider sandbox.Provider,
	meta store.SessionMeta,
	envMeta *store.SessionSandboxMeta,
	resolvedEnv sandbox.Resolved,
	localRoot string,
	localAdditional []string,
) (sandbox.SessionState, bool) {
	finder, ok := provider.(sandbox.Finder)
	if !ok {
		return sandbox.SessionState{}, false
	}

	labels := map[string]string{
		"agh_sandbox_id": strings.TrimSpace(envMeta.SandboxID),
	}
	found, err := finder.FindSandbox(ctx, sandbox.FindSandboxRequest{
		SessionID:           strings.TrimSpace(meta.ID),
		WorkspaceID:         strings.TrimSpace(meta.WorkspaceID),
		SandboxID:           strings.TrimSpace(envMeta.SandboxID),
		LocalRootDir:        localRoot,
		LocalAdditionalDirs: append([]string(nil), localAdditional...),
		Sandbox:             resolvedEnv,
		ProviderState:       cloneDaemonRawMessage(envMeta.ProviderState),
		Labels:              labels,
	})
	if err != nil {
		attrs := sandboxReconcileLogAttrs(meta, envMeta, 0, err)
		if errors.Is(err, sandbox.ErrSandboxNotFound) {
			sandboxReconcileLogger(state).Info("daemon: sandbox reconciliation remote not found", attrs...)
		} else {
			sandboxReconcileLogger(state).Warn("daemon: sandbox reconciliation remote lookup failed", attrs...)
		}
		return sandbox.SessionState{}, false
	}
	return found, true
}

func (d *Daemon) reattachDaemonSandbox(
	ctx context.Context,
	state *bootState,
	provider sandbox.Provider,
	candidate sandboxReconcileSession,
	envMeta *store.SessionSandboxMeta,
	providerState sandbox.SessionState,
	resolvedEnv sandbox.Resolved,
	localRoot string,
	localAdditional []string,
) {
	meta := candidate.meta
	started := time.Now()
	prepared, err := provider.Prepare(ctx, sandbox.PrepareRequest{
		SessionID:           strings.TrimSpace(meta.ID),
		WorkspaceID:         strings.TrimSpace(meta.WorkspaceID),
		SandboxID:           strings.TrimSpace(envMeta.SandboxID),
		InstanceID:          strings.TrimSpace(providerState.InstanceID),
		LocalRootDir:        localRoot,
		LocalAdditionalDirs: append([]string(nil), localAdditional...),
		Sandbox:             resolvedEnv,
		ProviderState:       cloneDaemonRawMessage(providerState.ProviderState),
	})
	duration := time.Since(started)
	if err != nil {
		sandboxReconcileLogger(state).Warn(
			"daemon: sandbox reattach failed",
			sandboxReconcileLogAttrs(meta, envMeta, duration, err)...,
		)
		if strings.TrimSpace(providerState.InstanceID) != "" {
			d.destroyDaemonSandbox(ctx, state, provider, candidate, envMeta, providerState)
		}
		return
	}

	nextState := mergeSandboxSessionState(providerState, prepared.State)
	nextMeta := sandboxMetaFromSessionState(nextState, sandboxReconcileStatePrepared)
	if nextMeta.Backend == "" {
		nextMeta.Backend = envMeta.Backend
	}
	if nextMeta.Profile == "" {
		nextMeta.Profile = envMeta.Profile
	}
	d.persistSandboxReconcileMeta(ctx, state, candidate, nextMeta)
	sandboxReconcileLogger(state).Info(
		"daemon: sandbox reattach complete",
		sandboxReconcileLogAttrs(meta, nextMeta, duration, nil)...,
	)
}

func (d *Daemon) destroyDaemonSandbox(
	ctx context.Context,
	state *bootState,
	provider sandbox.Provider,
	candidate sandboxReconcileSession,
	envMeta *store.SessionSandboxMeta,
	providerState sandbox.SessionState,
) {
	meta := candidate.meta
	logger := sandboxReconcileLogger(state)
	if strings.TrimSpace(providerState.InstanceID) == "" {
		logger.Warn(
			"daemon: sandbox destroy skipped",
			sandboxReconcileLogAttrs(meta, envMeta, 0, errors.New("sandbox instance id is required"))...,
		)
		return
	}

	started := time.Now()
	err := provider.Destroy(ctx, providerState)
	duration := time.Since(started)
	if err != nil {
		logger.Warn(
			"daemon: sandbox destroy failed",
			sandboxReconcileLogAttrs(meta, envMeta, duration, err)...,
		)
		return
	}

	nextMeta := sandboxMetaFromSessionState(providerState, sandboxReconcileStateDestroyed)
	if nextMeta.Backend == "" {
		nextMeta.Backend = envMeta.Backend
	}
	if nextMeta.Profile == "" {
		nextMeta.Profile = envMeta.Profile
	}
	nextMeta.State = sandboxReconcileStateDestroyed
	d.persistSandboxReconcileMeta(ctx, state, candidate, nextMeta)
	logger.Info(
		"daemon: sandbox destroy complete",
		sandboxReconcileLogAttrs(meta, nextMeta, duration, nil)...,
	)
}
