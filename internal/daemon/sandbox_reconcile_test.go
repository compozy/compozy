package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestReconcileDaemonSandboxesWithNoRemoteSessionsIsNoop(t *testing.T) {
	daemon, state, provider, _ := newSandboxReconcileHarness(t)
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-local",
		state:  session.StateActive,
		env:    remoteMeta("env-local", sandbox.BackendLocal, "local", ""),
		agent:  "coder",
		worker: "ws-local",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)

	if got := len(provider.prepareRequests); got != 0 {
		t.Fatalf("Prepare calls = %d, want 0", got)
	}
	if got := len(provider.findRequests); got != 0 {
		t.Fatalf("FindSandbox calls = %d, want 0", got)
	}
	if got := len(provider.destroyStates); got != 0 {
		t.Fatalf("Destroy calls = %d, want 0", got)
	}
}

func TestReconcileDaemonSandboxesHandlesBootEdgeCases(t *testing.T) {
	daemon, state, provider, logs := newSandboxReconcileHarness(t)

	var nilBootContext context.Context
	daemon.reconcileDaemonSandboxes(nilBootContext, nil)

	logs.Reset()
	state.sandboxRegistry = nil
	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)
	if !strings.Contains(logs.String(), "sandbox registry is required") {
		t.Fatalf("logs missing nil registry warning: %s", logs.String())
	}

	logs.Reset()
	registry, err := sandbox.NewRegistry(provider)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	state.sandboxRegistry = registry
	daemon.homePaths.SessionsDir = filepath.Join(t.TempDir(), "missing-sessions")
	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)
	if got := len(provider.prepareRequests); got != 0 {
		t.Fatalf("Prepare calls = %d, want 0 for missing sessions dir", got)
	}

	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-canceled",
		state:  session.StateActive,
		env:    remoteMeta("env-canceled", sandbox.BackendDaytona, "daytona", "sandbox-canceled"),
		agent:  "coder",
		worker: "ws-canceled",
	})
	ctx, cancel := context.WithCancel(testutil.Context(t))
	cancel()
	daemon.reconcileDaemonSandboxes(ctx, state)
	if got := len(provider.prepareRequests); got != 0 {
		t.Fatalf("Prepare calls = %d, want 0 after context cancellation", got)
	}
	if !strings.Contains(logs.String(), "daemon: sandbox reconciliation canceled") {
		t.Fatalf("logs missing cancellation warning: %s", logs.String())
	}
}

func TestLoadSandboxReconcileSessionsSkipsUnreadableMetadata(t *testing.T) {
	daemon, state, _, logs := newSandboxReconcileHarness(t)
	sessionsDir := daemon.homePaths.SessionsDir
	if err := os.MkdirAll(filepath.Join(sessionsDir, "sess-bad"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(
		store.SessionMetaFile(filepath.Join(sessionsDir, "sess-bad")),
		[]byte("{not-json"),
		0o644,
	); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionsDir, "not-a-session-dir"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("WriteFile(non-dir) error = %v", err)
	}
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-good",
		state:  session.StateActive,
		env:    remoteMeta("env-good", sandbox.BackendDaytona, "daytona", "sandbox-good"),
		agent:  "coder",
		worker: "ws-good",
	})

	sessions, err := daemon.loadSandboxReconcileSessions(state)
	if err != nil {
		t.Fatalf("loadSandboxReconcileSessions() error = %v", err)
	}
	if got := len(sessions); got != 1 {
		t.Fatalf("remote sessions = %d, want 1", got)
	}
	if got := sessions[0].meta.ID; got != "sess-good" {
		t.Fatalf("remote session ID = %q, want sess-good", got)
	}
	if !strings.Contains(logs.String(), "skipped unreadable session metadata") {
		t.Fatalf("logs missing unreadable metadata warning: %s", logs.String())
	}
}

func TestReconcileDaemonSandboxesReattachesRecoverableSession(t *testing.T) {
	daemon, state, provider, _ := newSandboxReconcileHarness(t)
	provider.prepareState = sandbox.SessionState{
		SandboxID:     "env-remote",
		Backend:       sandbox.BackendDaytona,
		Profile:       "daytona",
		State:         "ready",
		InstanceID:    "sandbox-reattached",
		ProviderState: json.RawMessage(`{"reattached":true}`),
	}
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-active",
		state:  session.StateActive,
		env:    remoteMeta("env-remote", sandbox.BackendDaytona, "daytona", "sandbox-remote"),
		agent:  "coder",
		worker: "ws-active",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)

	if got := len(provider.prepareRequests); got != 1 {
		t.Fatalf("Prepare calls = %d, want 1", got)
	}
	req := provider.prepareRequests[0]
	if req.SandboxID != "env-remote" {
		t.Fatalf("PrepareRequest.SandboxID = %q, want env-remote", req.SandboxID)
	}
	if req.InstanceID != "sandbox-remote" {
		t.Fatalf("PrepareRequest.InstanceID = %q, want sandbox-remote", req.InstanceID)
	}
	assertSandboxReconcileJSON(
		t,
		req.ProviderState,
		json.RawMessage(`{"seed":true}`),
		"PrepareRequest.ProviderState",
	)

	meta := readSandboxReconcileMeta(t, daemon, "sess-active")
	if meta.Sandbox.InstanceID != "sandbox-reattached" {
		t.Fatalf("persisted InstanceID = %q, want sandbox-reattached", meta.Sandbox.InstanceID)
	}
	assertSandboxReconcileJSON(
		t,
		meta.Sandbox.ProviderState,
		json.RawMessage(`{"reattached":true}`),
		"persisted ProviderState",
	)
	if meta.Sandbox.State != sandboxReconcileStatePrepared {
		t.Fatalf("persisted sandbox state = %q, want %q", meta.Sandbox.State, sandboxReconcileStatePrepared)
	}
}

func TestReconcileDaemonSandboxesFindsAndAttachesPartialCreate(t *testing.T) {
	daemon, state, provider, _ := newSandboxReconcileHarness(t)
	provider.findState = sandbox.SessionState{
		SandboxID:     "env-partial",
		Backend:       sandbox.BackendDaytona,
		Profile:       "daytona",
		State:         "found",
		InstanceID:    "sandbox-found",
		ProviderState: json.RawMessage(`{"found":true}`),
	}
	provider.prepareState = sandbox.SessionState{
		SandboxID:     "env-partial",
		Backend:       sandbox.BackendDaytona,
		Profile:       "daytona",
		State:         "ready",
		InstanceID:    "sandbox-found",
		ProviderState: json.RawMessage(`{"attached":true}`),
	}
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-partial",
		state:  session.StateActive,
		env:    remoteMeta("env-partial", sandbox.BackendDaytona, "daytona", ""),
		agent:  "coder",
		worker: "ws-partial",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)

	if got := len(provider.findRequests); got != 1 {
		t.Fatalf("FindSandbox calls = %d, want 1", got)
	}
	if got := provider.findRequests[0].Labels["agh_sandbox_id"]; got != "env-partial" {
		t.Fatalf("FindSandbox label agh_sandbox_id = %q, want env-partial", got)
	}
	if got := len(provider.prepareRequests); got != 1 {
		t.Fatalf("Prepare calls = %d, want 1", got)
	}
	if got := provider.prepareRequests[0].InstanceID; got != "sandbox-found" {
		t.Fatalf("PrepareRequest.InstanceID = %q, want sandbox-found", got)
	}
	assertSandboxReconcileJSON(
		t,
		provider.prepareRequests[0].ProviderState,
		json.RawMessage(`{"found":true}`),
		"PrepareRequest.ProviderState",
	)

	meta := readSandboxReconcileMeta(t, daemon, "sess-partial")
	if meta.Sandbox.InstanceID != "sandbox-found" {
		t.Fatalf("persisted InstanceID = %q, want sandbox-found", meta.Sandbox.InstanceID)
	}
	assertSandboxReconcileJSON(
		t,
		meta.Sandbox.ProviderState,
		json.RawMessage(`{"attached":true}`),
		"persisted ProviderState",
	)
}

func TestReconcileDaemonSandboxesDestroysUnrecoverablePartialCreate(t *testing.T) {
	daemon, state, provider, logs := newSandboxReconcileHarness(t)
	provider.findState = sandbox.SessionState{
		SandboxID:     "env-stopped-partial",
		Backend:       sandbox.BackendDaytona,
		Profile:       "daytona",
		State:         "found",
		InstanceID:    "sandbox-stopped-partial",
		ProviderState: json.RawMessage(`{"found":true}`),
	}
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-stopped-partial",
		state:  session.StateStopped,
		env:    remoteMeta("env-stopped-partial", sandbox.BackendDaytona, "daytona", ""),
		agent:  "coder",
		worker: "ws-stopped-partial",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)

	if got := len(provider.prepareRequests); got != 0 {
		t.Fatalf("Prepare calls = %d, want 0", got)
	}
	if got := len(provider.destroyStates); got != 1 {
		t.Fatalf("Destroy calls = %d, want 1", got)
	}
	if got := provider.destroyStates[0].InstanceID; got != "sandbox-stopped-partial" {
		t.Fatalf("Destroy state InstanceID = %q, want sandbox-stopped-partial", got)
	}
	meta := readSandboxReconcileMeta(t, daemon, "sess-stopped-partial")
	if meta.Sandbox.State != sandboxReconcileStateDestroyed {
		t.Fatalf("persisted sandbox state = %q, want destroyed", meta.Sandbox.State)
	}
	if !strings.Contains(logs.String(), "daemon: sandbox destroy complete") {
		t.Fatalf("logs missing destroy completion: %s", logs.String())
	}
}

func TestReconcileDaemonSandboxesDestroysUnrecoverableSession(t *testing.T) {
	daemon, state, provider, _ := newSandboxReconcileHarness(t)
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-stopped",
		state:  session.StateStopped,
		env:    remoteMeta("env-stopped", sandbox.BackendDaytona, "daytona", "sandbox-stopped"),
		agent:  "coder",
		worker: "ws-stopped",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)

	if got := len(provider.destroyStates); got != 1 {
		t.Fatalf("Destroy calls = %d, want 1", got)
	}
	if got := provider.destroyStates[0].InstanceID; got != "sandbox-stopped" {
		t.Fatalf("Destroy state InstanceID = %q, want sandbox-stopped", got)
	}
}

func TestReconcileDaemonSandboxesUsesResolvedWorkspaceInputs(t *testing.T) {
	daemon, state, provider, _ := newSandboxReconcileHarness(t)
	rootDir := t.TempDir()
	additionalDir := t.TempDir()
	expectedRootDir, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(root) error = %v", err)
	}
	expectedRootDir, err = filepath.Abs(expectedRootDir)
	if err != nil {
		t.Fatalf("Abs(root) error = %v", err)
	}
	resolver, err := workspacepkg.NewResolver(
		&sandboxReconcileWorkspaceStore{
			workspaces: map[string]workspacepkg.Workspace{
				"ws-resolved": {
					ID:             "ws-resolved",
					RootDir:        rootDir,
					AdditionalDirs: []string{additionalDir},
					Name:           "resolved",
					SandboxRef:     "daytona-dev",
					CreatedAt:      time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC),
					UpdatedAt:      time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC),
				},
			},
		},
		workspacepkg.WithConfigLoader(func(string) (aghconfig.Config, error) {
			return aghconfig.Config{
				Sandboxes: map[string]aghconfig.SandboxProfile{
					"daytona-dev": {
						Backend:  string(sandbox.BackendDaytona),
						SyncMode: string(sandbox.SyncModeNone),
						Daytona: aghconfig.DaytonaProfile{
							Snapshot: "snap-reconcile",
						},
					},
				},
			}, nil
		}),
		workspacepkg.WithLogger(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))),
	)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	state.workspaceResolver = resolver
	provider.prepareState = sandbox.SessionState{
		SandboxID:  "env-resolved",
		Backend:    sandbox.BackendDaytona,
		Profile:    "daytona-dev",
		State:      "ready",
		InstanceID: "sandbox-resolved",
	}
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-resolved",
		state:  session.StateActive,
		env:    remoteMeta("env-resolved", sandbox.BackendDaytona, "daytona-dev", "sandbox-resolved"),
		agent:  "coder",
		worker: "ws-resolved",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)

	if got := len(provider.prepareRequests); got != 1 {
		t.Fatalf("Prepare calls = %d, want 1", got)
	}
	req := provider.prepareRequests[0]
	if req.LocalRootDir != expectedRootDir {
		t.Fatalf("PrepareRequest.LocalRootDir = %q, want %q", req.LocalRootDir, expectedRootDir)
	}
	if len(req.LocalAdditionalDirs) != 1 || req.LocalAdditionalDirs[0] != additionalDir {
		t.Fatalf("PrepareRequest.LocalAdditionalDirs = %#v, want [%q]", req.LocalAdditionalDirs, additionalDir)
	}
	if req.Sandbox.Profile != "daytona-dev" ||
		req.Sandbox.Backend != sandbox.BackendDaytona ||
		req.Sandbox.SyncMode != sandbox.SyncModeNone {
		t.Fatalf("PrepareRequest.Sandbox = %#v, want resolved Daytona profile", req.Sandbox)
	}
}

func TestReconcileDaemonSandboxesUnavailableProviderLogsAndContinues(t *testing.T) {
	daemon, state, _, logs := newSandboxReconcileHarness(t)
	emptyRegistry, err := sandbox.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	state.sandboxRegistry = emptyRegistry
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-provider-missing",
		state:  session.StateActive,
		env:    remoteMeta("env-provider-missing", sandbox.BackendDaytona, "daytona", "sandbox-missing"),
		agent:  "coder",
		worker: "ws-provider-missing",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)
	if !strings.Contains(logs.String(), "daemon: sandbox reconciliation provider unavailable") {
		t.Fatalf("logs missing provider unavailable warning: %s", logs.String())
	}
}

func TestReconcileDaemonSandboxesFailureDoesNotBlockBoot(t *testing.T) {
	daemon, state, provider, logs := newSandboxReconcileHarness(t)
	provider.prepareErr = errors.New("provider unavailable")
	provider.destroyErr = errors.New("destroy unavailable")
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-failure",
		state:  session.StateActive,
		env:    remoteMeta("env-failure", sandbox.BackendDaytona, "daytona", "sandbox-failure"),
		agent:  "coder",
		worker: "ws-failure",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)
	if got := len(provider.prepareRequests); got != 1 {
		t.Fatalf("Prepare calls = %d, want 1", got)
	}
	if got := len(provider.destroyStates); got != 1 {
		t.Fatalf("Destroy calls = %d, want 1 cleanup attempt", got)
	}
	if !strings.Contains(logs.String(), "daemon: sandbox reattach failed") ||
		!strings.Contains(logs.String(), "daemon: sandbox destroy failed") {
		t.Fatalf("logs missing non-blocking failure evidence: %s", logs.String())
	}
}

func TestReconcileDaemonSandboxesPersistsSessionIndexBestEffort(t *testing.T) {
	daemon, state, provider, _ := newSandboxReconcileHarness(t)
	wantParticipation := daemonTestLiveParticipation("ws-indexed", "sandbox-indexed")
	registry := &sandboxReconcileRegistry{}
	state.registry = registry
	provider.prepareState = sandbox.SessionState{
		SandboxID:     "env-indexed",
		Backend:       sandbox.BackendDaytona,
		Profile:       "daytona",
		State:         "ready",
		InstanceID:    "sandbox-indexed",
		ProviderState: json.RawMessage(`{"indexed":true}`),
	}
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:      "sess-indexed",
		state:   session.StateActive,
		env:     remoteMeta("env-indexed", sandbox.BackendDaytona, "daytona", "sandbox-seed"),
		agent:   "coder",
		worker:  "ws-indexed",
		network: wantParticipation,
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)

	if got := len(registry.sessions); got != 1 {
		t.Fatalf("RegisterSession calls = %d, want 1", got)
	}
	indexed := registry.sessions[0]
	if indexed.ID != "sess-indexed" || indexed.Sandbox == nil {
		t.Fatalf("indexed session = %#v, want session with sandbox", indexed)
	}
	if indexed.Sandbox.InstanceID != "sandbox-indexed" {
		t.Fatalf("indexed sandbox instance = %q, want sandbox-indexed", indexed.Sandbox.InstanceID)
	}
	if got := indexed.NetworkSpecSnapshot(); got != wantParticipation {
		t.Fatalf("indexed network participation = %#v, want %#v", got, wantParticipation)
	}
}

func TestReconcileDaemonSandboxesLogsSessionIndexFailure(t *testing.T) {
	daemon, state, provider, logs := newSandboxReconcileHarness(t)
	state.registry = &failingSandboxReconcileRegistry{}
	provider.prepareState = sandbox.SessionState{
		SandboxID:  "env-index-fails",
		Backend:    sandbox.BackendDaytona,
		Profile:    "daytona",
		State:      "ready",
		InstanceID: "sandbox-index-fails",
	}
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-index-fails",
		state:  session.StateActive,
		env:    remoteMeta("env-index-fails", sandbox.BackendDaytona, "daytona", "sandbox-seed"),
		agent:  "coder",
		worker: "ws-index-fails",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)

	if !strings.Contains(logs.String(), "daemon: sandbox reconciliation session index update failed") {
		t.Fatalf("logs missing session index failure: %s", logs.String())
	}
}

func TestReconcileDaemonSandboxesPartialCreateNotFoundDoesNotBlock(t *testing.T) {
	daemon, state, provider, logs := newSandboxReconcileHarness(t)
	provider.findErr = sandbox.ErrSandboxNotFound
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-not-found",
		state:  session.StateActive,
		env:    remoteMeta("env-not-found", sandbox.BackendDaytona, "daytona", ""),
		agent:  "coder",
		worker: "ws-not-found",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)
	if got := len(provider.prepareRequests); got != 0 {
		t.Fatalf("Prepare calls = %d, want 0 without a discovered instance", got)
	}
	if !strings.Contains(logs.String(), "daemon: sandbox reconciliation remote not found") {
		t.Fatalf("logs missing remote not found info: %s", logs.String())
	}
}

func TestReconcileDaemonSandboxesDestroySkippedWithoutInstance(t *testing.T) {
	daemon, state, provider, logs := newSandboxReconcileHarness(t)
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-no-instance",
		state:  session.StateStopped,
		env:    remoteMeta("env-no-instance", sandbox.BackendDaytona, "daytona", ""),
		agent:  "coder",
		worker: "ws-no-instance",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)
	if got := len(provider.destroyStates); got != 0 {
		t.Fatalf("Destroy calls = %d, want 0", got)
	}
	if !strings.Contains(logs.String(), "daemon: sandbox reconciliation skipped missing instance") {
		t.Fatalf("logs missing missing instance warning: %s", logs.String())
	}
}

type sandboxReconcileMeta struct {
	id      string
	state   session.State
	env     *store.SessionSandboxMeta
	agent   string
	worker  string
	network participation.Spec
}

func newSandboxReconcileHarness(
	t *testing.T,
) (*Daemon, *bootState, *recordingSandboxReconcileProvider, *bytes.Buffer) {
	t.Helper()

	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	provider := &recordingSandboxReconcileProvider{backend: sandbox.BackendDaytona}
	registry, err := sandbox.NewRegistry(provider)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	daemon := &Daemon{
		homePaths: aghconfig.HomePaths{SessionsDir: filepath.Join(t.TempDir(), "sessions")},
		now: func() time.Time {
			return time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
		},
	}
	state := &bootState{
		logger:          logger,
		sandboxRegistry: registry,
	}
	return daemon, state, provider, logs
}

func writeSandboxReconcileMeta(t *testing.T, daemon *Daemon, spec sandboxReconcileMeta) {
	t.Helper()
	networkSpec := spec.network
	if networkSpec == (participation.Spec{}) {
		networkSpec = participation.LocalSpec()
	}
	meta := store.SessionMeta{
		ID:                   spec.id,
		AgentName:            spec.agent,
		WorkspaceID:          spec.worker,
		NetworkParticipation: participation.CloneSpec(networkSpec),
		SessionType:          string(session.SessionTypeUser),
		State:                string(spec.state),
		Sandbox:              spec.env,
		CreatedAt:            time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC),
	}
	path := store.SessionMetaFile(filepath.Join(daemon.homePaths.SessionsDir, spec.id))
	if err := store.WriteSessionMeta(path, meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}
}

func readSandboxReconcileMeta(t *testing.T, daemon *Daemon, sessionID string) store.SessionMeta {
	t.Helper()
	meta, err := store.ReadSessionMeta(store.SessionMetaFile(filepath.Join(daemon.homePaths.SessionsDir, sessionID)))
	if err != nil {
		t.Fatalf("ReadSessionMeta() error = %v", err)
	}
	return meta
}

func remoteMeta(
	sandboxID string,
	backend sandbox.Backend,
	profile string,
	instanceID string,
) *store.SessionSandboxMeta {
	return &store.SessionSandboxMeta{
		SandboxID:     sandboxID,
		Backend:       string(backend),
		Profile:       profile,
		State:         "creating",
		InstanceID:    instanceID,
		ProviderState: json.RawMessage(`{"seed":true}`),
	}
}

func assertSandboxReconcileJSON(t *testing.T, got json.RawMessage, want json.RawMessage, label string) {
	t.Helper()
	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("%s got invalid JSON %s: %v", label, got, err)
	}
	var wantValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("%s want invalid JSON %s: %v", label, want, err)
	}
	if !jsonValuesEqual(gotValue, wantValue) {
		t.Fatalf("%s = %s, want %s", label, got, want)
	}
}

func jsonValuesEqual(left any, right any) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftRaw, rightRaw)
}

type recordingSandboxReconcileProvider struct {
	backend         sandbox.Backend
	prepareRequests []sandbox.PrepareRequest
	findRequests    []sandbox.FindSandboxRequest
	destroyStates   []sandbox.SessionState
	prepareState    sandbox.SessionState
	findState       sandbox.SessionState
	prepareErr      error
	findErr         error
	destroyErr      error
}

type sandboxReconcileRegistry struct {
	recordingRegistry
	sessions []store.SessionInfo
}

func (r *sandboxReconcileRegistry) RegisterSession(_ context.Context, session store.SessionInfo) error {
	r.sessions = append(r.sessions, session)
	return nil
}

type failingSandboxReconcileRegistry struct {
	recordingRegistry
}

func (r *failingSandboxReconcileRegistry) RegisterSession(context.Context, store.SessionInfo) error {
	return errors.New("index unavailable")
}

type sandboxReconcileWorkspaceStore struct {
	workspaces map[string]workspacepkg.Workspace
}

func (s *sandboxReconcileWorkspaceStore) InsertWorkspace(context.Context, workspacepkg.Workspace) error {
	return nil
}

func (s *sandboxReconcileWorkspaceStore) UpdateWorkspace(_ context.Context, ws workspacepkg.Workspace) error {
	s.workspaces[ws.ID] = ws
	return nil
}

func (s *sandboxReconcileWorkspaceStore) DeleteWorkspace(context.Context, string) error {
	return nil
}

func (s *sandboxReconcileWorkspaceStore) GetWorkspace(
	_ context.Context,
	id string,
) (workspacepkg.Workspace, error) {
	if ws, ok := s.workspaces[id]; ok {
		return ws, nil
	}
	return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (s *sandboxReconcileWorkspaceStore) GetWorkspaceByPath(
	_ context.Context,
	rootDir string,
) (workspacepkg.Workspace, error) {
	for _, ws := range s.workspaces {
		if ws.RootDir == rootDir {
			return ws, nil
		}
	}
	return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (s *sandboxReconcileWorkspaceStore) GetWorkspaceByName(
	_ context.Context,
	name string,
) (workspacepkg.Workspace, error) {
	for _, ws := range s.workspaces {
		if ws.Name == name {
			return ws, nil
		}
	}
	return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (s *sandboxReconcileWorkspaceStore) ListWorkspaces(context.Context) ([]workspacepkg.Workspace, error) {
	workspaces := make([]workspacepkg.Workspace, 0, len(s.workspaces))
	for _, ws := range s.workspaces {
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

func (p *recordingSandboxReconcileProvider) Backend() sandbox.Backend {
	return p.backend
}

func (p *recordingSandboxReconcileProvider) Prepare(
	_ context.Context,
	req sandbox.PrepareRequest,
) (sandbox.Prepared, error) {
	p.prepareRequests = append(p.prepareRequests, clonePrepareRequest(req))
	if p.prepareErr != nil {
		return sandbox.Prepared{}, p.prepareErr
	}
	state := p.prepareState
	if strings.TrimSpace(state.SandboxID) == "" {
		state.SandboxID = req.SandboxID
	}
	if !state.Backend.Valid() {
		state.Backend = p.backend
	}
	if strings.TrimSpace(state.Profile) == "" {
		state.Profile = req.Sandbox.Profile
	}
	if strings.TrimSpace(state.InstanceID) == "" {
		state.InstanceID = req.InstanceID
	}
	if len(state.ProviderState) == 0 {
		state.ProviderState = append(json.RawMessage(nil), req.ProviderState...)
	}
	return sandbox.Prepared{State: state}, nil
}

func (p *recordingSandboxReconcileProvider) SyncToRuntime(
	context.Context,
	sandbox.SessionState,
	sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	return sandbox.SyncResult{}, nil
}

func (p *recordingSandboxReconcileProvider) SyncFromRuntime(
	context.Context,
	sandbox.SessionState,
	sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	return sandbox.SyncResult{}, nil
}

func (p *recordingSandboxReconcileProvider) Destroy(
	_ context.Context,
	state sandbox.SessionState,
) error {
	p.destroyStates = append(p.destroyStates, cloneSessionState(state))
	return p.destroyErr
}

func (p *recordingSandboxReconcileProvider) FindSandbox(
	_ context.Context,
	req sandbox.FindSandboxRequest,
) (sandbox.SessionState, error) {
	p.findRequests = append(p.findRequests, cloneFindRequest(req))
	if p.findErr != nil {
		return sandbox.SessionState{}, p.findErr
	}
	return cloneSessionState(p.findState), nil
}

func clonePrepareRequest(req sandbox.PrepareRequest) sandbox.PrepareRequest {
	cloned := req
	cloned.LocalAdditionalDirs = append([]string(nil), req.LocalAdditionalDirs...)
	cloned.AgentEnv = append([]string(nil), req.AgentEnv...)
	cloned.ProviderState = append(json.RawMessage(nil), req.ProviderState...)
	return cloned
}

func cloneFindRequest(req sandbox.FindSandboxRequest) sandbox.FindSandboxRequest {
	cloned := req
	cloned.LocalAdditionalDirs = append([]string(nil), req.LocalAdditionalDirs...)
	cloned.ProviderState = append(json.RawMessage(nil), req.ProviderState...)
	if req.Labels != nil {
		cloned.Labels = make(map[string]string, len(req.Labels))
		maps.Copy(cloned.Labels, req.Labels)
	}
	return cloned
}

func cloneSessionState(state sandbox.SessionState) sandbox.SessionState {
	cloned := state
	cloned.RuntimeAdditionalDirs = append([]string(nil), state.RuntimeAdditionalDirs...)
	cloned.ProviderState = append(json.RawMessage(nil), state.ProviderState...)
	if state.SSHAccessExpiresAt != nil {
		expires := *state.SSHAccessExpiresAt
		cloned.SSHAccessExpiresAt = &expires
	}
	return cloned
}
