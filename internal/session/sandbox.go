package session

import (
	"context"

	"errors"
	"fmt"

	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/diagnostics"
	hookspkg "github.com/compozy/agh/internal/hooks"
	envpkg "github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/vault"
)

const (
	sandboxSandboxPreparePath = "sandbox.prepare"
)

const (
	sandboxStateCreating  = "creating"
	sandboxStatePrepared  = "prepared"
	sandboxStateStopped   = "stopped"
	sandboxStateDestroyed = "destroyed"

	sandboxEventPrepareStart        = "sandbox.prepare.start"
	sandboxEventPrepareComplete     = "sandbox.prepare.complete"
	sandboxEventPrepareError        = "sandbox.prepare.error"
	sandboxEventSyncStart           = "sandbox.sync.start"
	sandboxEventSyncComplete        = "sandbox.sync.complete"
	sandboxEventSyncError           = "sandbox.sync.error"
	sandboxEventTransportConnect    = "sandbox.transport.connect"
	sandboxEventTransportDisconnect = "sandbox.transport.disconnect"
	sandboxEventTransportError      = "sandbox.transport.error"
	sandboxEventDestroyStart        = "sandbox.destroy.start"
	sandboxEventDestroyComplete     = "sandbox.destroy.complete"
	sandboxEventDestroyError        = "sandbox.destroy.error"
)

// SandboxLifecycleEvent reports provider lifecycle timing to optional observers.
type SandboxLifecycleEvent struct {
	Name        string
	Span        string
	SessionID   string
	WorkspaceID string
	SandboxID   string
	Backend     string
	Profile     string
	InstanceID  string
	Reason      string
	Duration    time.Duration
	ErrorKind   string
	Error       string
	Timestamp   time.Time
}

// SandboxLifecycleNotifier is an optional notifier extension for sandbox lifecycle spans.
type SandboxLifecycleNotifier interface {
	OnSandboxLifecycleEvent(context.Context, SandboxLifecycleEvent)
}

func (m *Manager) prepareSandboxForStart(
	ctx context.Context,
	spec *sessionStartSpec,
	session *Session,
	opts acp.StartOpts,
) (acp.StartOpts, error) {
	if spec == nil {
		return acp.StartOpts{}, errors.New("session: start spec is required")
	}
	if session == nil {
		return acp.StartOpts{}, errors.New("session: session is required")
	}
	if spec.sandboxDisabled {
		return opts, nil
	}
	if m.sandbox == nil {
		return acp.StartOpts{}, errors.New("session: sandbox registry is required")
	}

	resolvedEnv := normalizeResolvedSandbox(spec.workspace.Sandbox)
	provider, err := m.sandbox.Provider(resolvedEnv.Backend)
	if err != nil {
		return acp.StartOpts{}, fmt.Errorf("session: resolve sandbox provider %q: %w", resolvedEnv.Backend, err)
	}

	sandboxID, meta, err := m.initializeSandboxMetaForStart(spec, session, resolvedEnv)
	if err != nil {
		return acp.StartOpts{}, err
	}

	req := envpkg.PrepareRequest{
		SessionID:           session.ID,
		WorkspaceID:         session.WorkspaceID,
		SandboxID:           sandboxID,
		InstanceID:          meta.InstanceID,
		LocalRootDir:        spec.workspace.RootDir,
		LocalAdditionalDirs: append([]string(nil), spec.workspace.AdditionalDirs...),
		Sandbox:             resolvedEnv,
		AgentCommand:        opts.Command,
		AgentEnv:            sandboxAgentEnv(opts.Env, resolvedEnv),
		Permissions:         string(opts.Permissions),
		ResumeACPState:      opts.ResumeSessionID,
		ProviderState:       cloneRawMessage(meta.ProviderState),
	}
	req, err = m.dispatchSandboxPrepare(ctx, session, req)
	if err != nil {
		return acp.StartOpts{}, err
	}
	req, err = m.resolveSandboxSecretEnv(ctx, session, req)
	if err != nil {
		return acp.StartOpts{}, err
	}

	prepared, prepareErr := m.callSandboxPrepare(ctx, provider, req, meta)
	if prepareErr != nil {
		return acp.StartOpts{}, prepareErr
	}

	state, err := normalizePreparedSandboxState(prepared, meta, resolvedEnv)
	if err != nil {
		return acp.StartOpts{}, err
	}
	meta = sessionSandboxMetaFromState(state, sandboxStatePrepared)
	session.setSandbox(meta, m.now())
	if err := m.persistSessionMetadataOnly(session); err != nil {
		return acp.StartOpts{}, err
	}

	if err := m.syncSandboxToRuntime(ctx, provider, session, state, meta); err != nil {
		return acp.StartOpts{}, err
	}
	meta = cloneSessionSandboxMeta(session.Info().Sandbox)
	if err := m.dispatchSandboxReady(ctx, session, state, meta); err != nil {
		return acp.StartOpts{}, err
	}

	return sandboxStartOpts(opts, prepared, state, spec.workspace.RootDir)
}

func (m *Manager) initializeSandboxMetaForStart(
	spec *sessionStartSpec,
	session *Session,
	resolvedEnv envpkg.Resolved,
) (string, *store.SessionSandboxMeta, error) {
	sandboxID := strings.TrimSpace(spec.sandboxID)
	if sandboxID == "" {
		sandboxID = sessionSandboxID(spec.sandbox)
	}
	if sandboxID == "" {
		sandboxID = strings.TrimSpace(m.newSandboxID())
	}
	if sandboxID == "" {
		return "", nil, errors.New("session: sandbox id generator returned empty id")
	}
	spec.sandboxID = sandboxID

	meta := initialSessionSandboxMeta(sandboxID, resolvedEnv, spec.sandbox)
	session.setSandbox(meta, m.now())
	if err := m.persistSessionMetadataOnly(session); err != nil {
		return "", nil, err
	}
	return sandboxID, meta, nil
}

func (m *Manager) callSandboxPrepare(
	ctx context.Context,
	provider envpkg.Provider,
	req envpkg.PrepareRequest,
	meta *store.SessionSandboxMeta,
) (envpkg.Prepared, error) {
	started := time.Now()
	event := sandboxEventFromMeta(meta, req.SessionID, req.WorkspaceID, sandboxEventPrepareStart, "")
	m.logSandboxLifecycle(event)

	prepared, err := provider.Prepare(ctx, req)
	duration := time.Since(started)
	if err != nil {
		errorEvent := sandboxEventFromMeta(meta, req.SessionID, req.WorkspaceID, sandboxEventPrepareError, "")
		errorEvent.Duration = duration
		attachSandboxError(&errorEvent, err)
		m.logSandboxLifecycle(errorEvent)
		return envpkg.Prepared{}, fmt.Errorf(
			"session: prepare sandbox %q for %q: %w",
			req.SandboxID,
			req.SessionID,
			err,
		)
	}

	completeMeta := sessionSandboxMetaFromState(prepared.State, sandboxStatePrepared)
	if completeMeta.SandboxID == "" {
		completeMeta.SandboxID = req.SandboxID
	}
	if completeMeta.Backend == "" {
		completeMeta.Backend = string(provider.Backend())
	}
	if completeMeta.Profile == "" {
		completeMeta.Profile = req.Sandbox.Profile
	}
	completeEvent := sandboxEventFromMeta(
		completeMeta,
		req.SessionID,
		req.WorkspaceID,
		sandboxEventPrepareComplete,
		"",
	)
	completeEvent.Duration = duration
	m.logSandboxLifecycle(completeEvent)
	return prepared, nil
}

func (m *Manager) dispatchSandboxPrepare(
	ctx context.Context,
	session *Session,
	req envpkg.PrepareRequest,
) (envpkg.PrepareRequest, error) {
	payload := hookspkg.SandboxPreparePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSandboxPrepare,
			Timestamp: m.now(),
		},
		SessionContext:      hookSessionContext(session),
		SandboxID:           strings.TrimSpace(req.SandboxID),
		Backend:             string(req.Sandbox.Backend),
		Profile:             sandboxProfilePayload(req.Sandbox),
		LocalRootDir:        strings.TrimSpace(req.LocalRootDir),
		LocalAdditionalDirs: append([]string(nil), req.LocalAdditionalDirs...),
		AgentCommand:        strings.TrimSpace(req.AgentCommand),
		AgentEnv:            append([]string(nil), req.AgentEnv...),
		Permissions:         strings.TrimSpace(req.Permissions),
		ResumeACPState:      strings.TrimSpace(req.ResumeACPState),
	}
	patched, err := m.hooks.sandbox().DispatchSandboxPrepare(ctx, payload)
	if err != nil {
		return req, err
	}
	if patched.Denied {
		if reason := strings.TrimSpace(patched.DenyReason); reason != "" {
			return req, fmt.Errorf("session: sandbox prepare denied: %s", reason)
		}
		return req, errors.New("session: sandbox prepare denied")
	}
	if len(patched.EnvOverrides) == 0 {
		return req, nil
	}

	req.Sandbox.Env = mergeSandboxEnv(req.Sandbox.Env, patched.EnvOverrides)
	req.AgentEnv = applySandboxEnvOverrides(req.AgentEnv, patched.EnvOverrides)
	return req, nil
}

func (m *Manager) resolveSandboxSecretEnv(
	ctx context.Context,
	session *Session,
	req envpkg.PrepareRequest,
) (envpkg.PrepareRequest, error) {
	if len(req.Sandbox.SecretEnv) == 0 {
		return req, nil
	}
	if ctx == nil {
		return req, errors.New("session: sandbox secret env context is required")
	}
	if m.providerSecrets == nil {
		return req, errors.New("session: sandbox secret resolver is not configured")
	}
	values := make(map[string]string, len(req.Sandbox.SecretEnv))
	cleanups := []func(){}
	keys := make([]string, 0, len(req.Sandbox.SecretEnv))
	for key := range req.Sandbox.SecretEnv {
		keys = append(keys, strings.TrimSpace(key))
	}
	sort.Strings(keys)
	for _, key := range keys {
		ref := vault.NormalizeRef(req.Sandbox.SecretEnv[key])
		value, err := m.providerSecrets.ResolveRef(ctx, ref)
		if err != nil {
			runProviderSecretRedactions(cleanups)
			return req, fmt.Errorf("session: resolve sandbox secret_env.%s: %w", key, err)
		}
		values[key] = value
		cleanups = append(cleanups, diagnostics.RegisterDynamicSecret(value))
	}
	req.Sandbox.Env = mergeSandboxEnv(req.Sandbox.Env, values)
	req.AgentEnv = applySandboxEnvOverrides(req.AgentEnv, values)
	if session != nil {
		session.addProviderSecretRedactions(cleanups)
	}
	return req, nil
}

func (m *Manager) dispatchSandboxReady(
	ctx context.Context,
	session *Session,
	state envpkg.SessionState,
	meta *store.SessionSandboxMeta,
) error {
	payload := hookspkg.SandboxReadyPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSandboxReady,
			Timestamp: m.now(),
		},
		SessionContext:        hookSessionContext(session),
		SandboxID:             strings.TrimSpace(state.SandboxID),
		Backend:               string(state.Backend),
		Profile:               strings.TrimSpace(state.Profile),
		InstanceID:            strings.TrimSpace(state.InstanceID),
		RuntimeRootDir:        strings.TrimSpace(state.RuntimeRootDir),
		RuntimeAdditionalDirs: append([]string(nil), state.RuntimeAdditionalDirs...),
	}
	if meta != nil {
		if payload.SandboxID == "" {
			payload.SandboxID = strings.TrimSpace(meta.SandboxID)
		}
		if payload.Backend == "" {
			payload.Backend = strings.TrimSpace(meta.Backend)
		}
		if payload.Profile == "" {
			payload.Profile = strings.TrimSpace(meta.Profile)
		}
		if payload.InstanceID == "" {
			payload.InstanceID = strings.TrimSpace(meta.InstanceID)
		}
		if payload.RuntimeRootDir == "" {
			payload.RuntimeRootDir = strings.TrimSpace(meta.RuntimeRootDir)
		}
	}
	_, err := m.hooks.sandbox().DispatchSandboxReady(ctx, payload)
	return err
}

func (m *Manager) dispatchSandboxSyncBefore(
	ctx context.Context,
	session *Session,
	state envpkg.SessionState,
	meta *store.SessionSandboxMeta,
	direction envpkg.SyncDirection,
	reason envpkg.SyncReason,
) (hookspkg.SandboxSyncBeforePayload, error) {
	payload := hookspkg.SandboxSyncBeforePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSandboxSyncBefore,
			Timestamp: m.now(),
		},
		SessionContext: hookSessionContext(session),
		SandboxID:      strings.TrimSpace(state.SandboxID),
		Backend:        string(state.Backend),
		Profile:        strings.TrimSpace(state.Profile),
		InstanceID:     strings.TrimSpace(state.InstanceID),
		RuntimeRootDir: strings.TrimSpace(state.RuntimeRootDir),
		Direction:      string(direction),
		Reason:         string(reason),
	}
	applySandboxMetaFallbacks(&payload.SandboxID, &payload.Backend, &payload.Profile, &payload.InstanceID, meta)
	return m.hooks.sandbox().DispatchSandboxSyncBefore(ctx, payload)
}

func (m *Manager) dispatchSandboxSyncAfter(
	ctx context.Context,
	session *Session,
	state envpkg.SessionState,
	meta *store.SessionSandboxMeta,
	direction envpkg.SyncDirection,
	reason envpkg.SyncReason,
	duration time.Duration,
	result envpkg.SyncResult,
	errorsList []string,
) error {
	payload := hookspkg.SandboxSyncAfterPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSandboxSyncAfter,
			Timestamp: m.now(),
		},
		SessionContext:   hookSessionContext(session),
		SandboxID:        strings.TrimSpace(state.SandboxID),
		Backend:          string(state.Backend),
		Profile:          strings.TrimSpace(state.Profile),
		InstanceID:       strings.TrimSpace(state.InstanceID),
		RuntimeRootDir:   strings.TrimSpace(state.RuntimeRootDir),
		Direction:        string(direction),
		Reason:           string(reason),
		FilesSynced:      result.FilesSynced,
		BytesTransferred: result.BytesTransferred,
		DurationMS:       duration.Milliseconds(),
		Errors:           append([]string(nil), errorsList...),
	}
	applySandboxMetaFallbacks(&payload.SandboxID, &payload.Backend, &payload.Profile, &payload.InstanceID, meta)
	_, err := m.hooks.sandbox().DispatchSandboxSyncAfter(ctx, payload)
	return err
}

func (m *Manager) dispatchSandboxStop(
	ctx context.Context,
	session *Session,
	state envpkg.SessionState,
	meta *store.SessionSandboxMeta,
	reason envpkg.SyncReason,
	willDestroy bool,
) (hookspkg.SandboxStopPayload, error) {
	stopReason := string(reason)
	if info := session.Info(); info != nil && strings.TrimSpace(string(info.StopReason)) != "" {
		stopReason = string(info.StopReason)
	}
	payload := hookspkg.SandboxStopPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSandboxStop,
			Timestamp: m.now(),
		},
		SessionContext: hookSessionContext(session),
		SandboxID:      strings.TrimSpace(state.SandboxID),
		Backend:        string(state.Backend),
		Profile:        strings.TrimSpace(state.Profile),
		InstanceID:     strings.TrimSpace(state.InstanceID),
		RuntimeRootDir: strings.TrimSpace(state.RuntimeRootDir),
		StopReason:     strings.TrimSpace(stopReason),
		WillDestroy:    willDestroy,
	}
	applySandboxMetaFallbacks(&payload.SandboxID, &payload.Backend, &payload.Profile, &payload.InstanceID, meta)
	return m.hooks.sandbox().DispatchSandboxStop(ctx, payload)
}
