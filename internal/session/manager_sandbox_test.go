package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestSessionSandboxStartPrepareSyncAndLaunchSequence(t *testing.T) {
	t.Parallel()

	runtimeRoot := filepath.Join(t.TempDir(), "runtime-root")
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(runtime root) error = %v", err)
	}
	canonicalRuntimeRoot, err := filepath.EvalSymlinks(runtimeRoot)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(runtime root) error = %v", err)
	}
	runtimeCWD := filepath.Join(canonicalRuntimeRoot, "nested-session-cwd")
	runtimeAdditional := []string{filepath.Join(t.TempDir(), "runtime-extra")}
	providerState := json.RawMessage(`{"prepared":true}`)
	var (
		h     *harness
		mu    sync.Mutex
		order []string
	)
	appendOrder := func(entry string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, entry)
	}

	provider := &recordingSandboxProvider{
		runtimeRoot:       runtimeRoot,
		runtimeAdditional: runtimeAdditional,
		instanceID:        "instance-start",
		providerState:     providerState,
		prepareHook: func(req sandbox.PrepareRequest) error {
			appendOrder("prepare")
			meta := readMeta(t, store.SessionMetaFile(filepath.Join(h.homePaths.SessionsDir, req.SessionID)))
			if meta.Sandbox == nil {
				t.Fatal("persisted sandbox before Prepare = nil, want creating metadata")
			}
			if got, want := meta.Sandbox.SandboxID, "env-1"; got != want {
				t.Fatalf("creating sandbox id = %q, want %q", got, want)
			}
			if got, want := meta.Sandbox.State, sandboxStateCreating; got != want {
				t.Fatalf("creating sandbox state = %q, want %q", got, want)
			}
			return nil
		},
		syncToHook: func(state sandbox.SessionState, opts sandbox.SyncOptions) (sandbox.SyncResult, error) {
			appendOrder("sync_to")
			if got, want := opts.Reason, sandbox.SyncReasonStart; got != want {
				t.Fatalf("SyncToRuntime reason = %q, want %q", got, want)
			}
			if got, want := state.SandboxID, "env-1"; got != want {
				t.Fatalf("SyncToRuntime sandbox id = %q, want %q", got, want)
			}
			if got, want := state.RuntimeRootDir, runtimeRoot; got != want {
				t.Fatalf("SyncToRuntime runtime root = %q, want %q", got, want)
			}
			if err := os.MkdirAll(runtimeCWD, 0o755); err != nil {
				return sandbox.SyncResult{}, fmt.Errorf("create fake runtime cwd: %w", err)
			}
			return sandbox.SyncResult{}, nil
		},
	}
	registry := newRegistryForProvider(t, provider)
	h = newHarness(t, WithSandboxRegistry(registry), WithSandboxIDGenerator(sequentialIDGenerator("env")))
	sessionCWD := filepath.Join(h.workspace, "nested-session-cwd")
	if err := os.MkdirAll(sessionCWD, 0o755); err != nil {
		t.Fatalf("MkdirAll(session CWD) error = %v", err)
	}
	h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
		appendOrder("launch")
		if got, want := opts.Cwd, runtimeCWD; got != want {
			t.Fatalf("StartOpts.Cwd = %q, want mapped runtime cwd %q", got, want)
		}
		if !reflect.DeepEqual(opts.AdditionalDirs, runtimeAdditional) {
			t.Fatalf("StartOpts.AdditionalDirs = %#v, want %#v", opts.AdditionalDirs, runtimeAdditional)
		}
		if opts.Launcher != nil {
			t.Fatal("fake provider launcher = non-nil, want fake-driver fallback")
		}
		meta := readMeta(t, store.SessionMetaFile(filepath.Join(h.homePaths.SessionsDir, "sess-1")))
		if got, want := meta.Sandbox.InstanceID, "instance-start"; got != want {
			t.Fatalf("persisted instance id before launch = %q, want %q", got, want)
		}
		assertJSONEqual(t, meta.Sandbox.ProviderState, providerState, "persisted provider state before launch")
		if meta.Sandbox.LastSyncAt == nil {
			t.Fatal("persisted LastSyncAt before launch = nil, want sync timestamp")
		}
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-start"), nil
	}

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
		CWD:       sessionCWD,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if got, want := provider.prepareRequests[0].SandboxID, "env-1"; got != want {
		t.Fatalf("PrepareRequest.SandboxID = %q, want %q", got, want)
	}
	if got, want := provider.prepareRequests[0].SessionID, session.ID; got != want {
		t.Fatalf("PrepareRequest.SessionID = %q, want %q", got, want)
	}
	if got, want := provider.prepareRequests[0].WorkspaceID, h.workspaceID; got != want {
		t.Fatalf("PrepareRequest.WorkspaceID = %q, want %q", got, want)
	}
	if got, want := orderSnapshot(&mu, order), []string{"prepare", "sync_to", "launch"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sandbox order = %#v, want %#v", got, want)
	}
	if info := session.Info(); info.Sandbox == nil || info.Sandbox.SandboxID != "env-1" {
		t.Fatalf("session.Info().Sandbox = %#v, want env-1", info.Sandbox)
	}
}

func TestSessionSandboxCreateAppliesRuntimeSandboxOverride(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve explicit sandbox ref through workspace config", func(t *testing.T) {
		t.Parallel()

		provider := &recordingSandboxProvider{}
		h := newHarness(t, WithSandboxRegistry(newRegistryForProvider(t, provider)))
		resolved, err := h.resolver.Resolve(context.Background(), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
		}
		resolved.Config.Sandboxes["task-ref"] = aghconfig.SandboxProfile{
			Backend:     string(sandbox.BackendLocal),
			SyncMode:    string(sandbox.SyncModeNone),
			Persistence: string(sandbox.PersistenceReuse),
			Env:         map[string]string{"TASK_PROFILE": "enabled"},
		}
		h.resolver.upsert(&resolved)

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:  "coder",
			Workspace:  h.workspaceID,
			SandboxRef: "task-ref",
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("Stop() error = %v", err)
			}
		})

		if got, want := provider.prepareRequests[0].Sandbox.Profile, "task-ref"; got != want {
			t.Fatalf("PrepareRequest.Sandbox.Profile = %q, want %q", got, want)
		}
		if got, want := provider.prepareRequests[0].Sandbox.Env["TASK_PROFILE"], "enabled"; got != want {
			t.Fatalf("PrepareRequest.Sandbox.Env[TASK_PROFILE] = %q, want %q", got, want)
		}
		if info := session.Info(); info.Sandbox == nil || info.Sandbox.Profile != "task-ref" {
			t.Fatalf("session.Info().Sandbox = %#v, want task-ref profile", info.Sandbox)
		}
	})

	t.Run("Should disable sandbox startup when explicitly requested", func(t *testing.T) {
		t.Parallel()

		provider := &recordingSandboxProvider{
			prepareHook: func(sandbox.PrepareRequest) error {
				t.Fatal("Prepare() called for disabled sandbox")
				return nil
			},
		}
		h := newHarness(t, WithSandboxRegistry(newRegistryForProvider(t, provider)))

		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:      "coder",
			Workspace:      h.workspaceID,
			DisableSandbox: true,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("Stop() error = %v", err)
			}
		})

		if got := len(provider.prepareRequests); got != 0 {
			t.Fatalf("Prepare() calls = %d, want 0", got)
		}
		if info := session.Info(); info.Sandbox != nil {
			t.Fatalf("session.Info().Sandbox = %#v, want nil", info.Sandbox)
		}
		if meta := readMeta(t, session.MetaPath()); meta.Sandbox != nil {
			t.Fatalf("meta.Sandbox = %#v, want nil", meta.Sandbox)
		}
		canonicalWorkspace, err := canonicalDirectory(h.workspace)
		if err != nil {
			t.Fatalf("canonicalDirectory(workspace) error = %v", err)
		}
		if got, want := h.driver.startCalls[0].Cwd, canonicalWorkspace; got != want {
			t.Fatalf("StartOpts.Cwd = %q, want workspace root %q", got, want)
		}
	})

	t.Run("Should reject conflicting sandbox ref and disabled sandbox", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:      "coder",
			Workspace:      h.workspaceID,
			SandboxRef:     "task-ref",
			DisableSandbox: true,
		})
		if err == nil {
			t.Fatal("Create() error = nil, want sandbox conflict")
		}
		if !strings.Contains(err.Error(), "mutually exclusive") {
			t.Fatalf("Create() error = %v, want mutually exclusive", err)
		}
	})
}

func TestSessionSandboxRejectsSymlinkCWDOutsideWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve a valid child below a symlinked runtime root", func(t *testing.T) {
		t.Parallel()

		realRoot := t.TempDir()
		realChild := filepath.Join(realRoot, "project")
		if err := os.Mkdir(realChild, 0o755); err != nil {
			t.Fatalf("Mkdir(project) error = %v", err)
		}
		linkedRoot := filepath.Join(t.TempDir(), "runtime-root")
		if err := os.Symlink(realRoot, linkedRoot); err != nil {
			t.Skipf("os.Symlink() unavailable: %v", err)
		}

		got, err := sandboxRuntimeCWD(
			realRoot,
			realChild,
			sandbox.Prepared{RuntimeRootDir: linkedRoot},
			sandbox.SessionState{Backend: sandbox.BackendLocal},
		)
		if err != nil {
			t.Fatalf("sandboxRuntimeCWD() error = %v", err)
		}
		want, err := canonicalDirectory(realChild)
		if err != nil {
			t.Fatalf("canonicalDirectory(project) error = %v", err)
		}
		if got != want {
			t.Fatalf("sandboxRuntimeCWD() = %q, want %q", got, want)
		}
	})

	t.Run("Should reject a workspace child symlink that resolves outside the workspace", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		outside := t.TempDir()
		linkedProject := filepath.Join(h.workspace, "linked-project")
		if err := os.Symlink(outside, linkedProject); err != nil {
			t.Skipf("os.Symlink() unavailable: %v", err)
		}

		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			CWD:       linkedProject,
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Create(symlink escape) error = %v, want ErrValidation", err)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver Start() calls = %d, want 0", got)
		}
	})

	t.Run("Should reject a persisted child replaced by an escaping symlink before resume", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		project := filepath.Join(h.workspace, "project")
		if err := os.Mkdir(project, 0o755); err != nil {
			t.Fatalf("Mkdir(project) error = %v", err)
		}
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			CWD:       project,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if err := os.Remove(project); err != nil {
			t.Fatalf("Remove(project) error = %v", err)
		}
		if err := os.Symlink(t.TempDir(), project); err != nil {
			t.Skipf("os.Symlink() unavailable: %v", err)
		}

		_, err = h.manager.Resume(testutil.Context(t), session.ID)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Resume(symlink escape) error = %v, want ErrValidation", err)
		}
		if got, want := len(h.driver.startCalls), 1; got != want {
			t.Fatalf("driver Start() calls = %d, want %d", got, want)
		}
	})
}

func TestSessionSandboxStopSyncsBeforeRecorderCloseAndDestroyPolicy(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name          string
		destroyOnStop bool
		wantState     string
		wantOrder     []string
	}{
		{
			name:          "keeps sandbox when destroy on stop is false",
			destroyOnStop: false,
			wantState:     sandboxStateStopped,
			wantOrder:     []string{"sync_from", "close"},
		},
		{
			name:          "destroys sandbox when destroy on stop is true",
			destroyOnStop: true,
			wantState:     sandboxStateDestroyed,
			wantOrder:     []string{"sync_from", "destroy", "close"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				mu    sync.Mutex
				order []string
			)
			appendOrder := func(entry string) {
				mu.Lock()
				defer mu.Unlock()
				order = append(order, entry)
			}
			provider := &recordingSandboxProvider{
				syncFromHook: func(_ sandbox.SessionState, opts sandbox.SyncOptions) (sandbox.SyncResult, error) {
					appendOrder("sync_from")
					if got, want := opts.Reason, sandbox.SyncReasonStop; got != want {
						t.Fatalf("SyncFromRuntime reason = %q, want %q", got, want)
					}
					return sandbox.SyncResult{}, nil
				},
				destroyHook: func(sandbox.SessionState) error {
					appendOrder("destroy")
					return nil
				},
			}
			h := newHarness(
				t,
				WithSandboxRegistry(newRegistryForProvider(t, provider)),
				WithStore(func(context.Context, string, string) (EventRecorder, error) {
					return &orderingRecorder{onClose: func() { appendOrder("close") }}, nil
				}),
			)
			setHarnessSandbox(t, h, tt.destroyOnStop)
			session := createSession(t, h)

			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("Stop() error = %v", err)
			}

			if got := orderSnapshot(&mu, order); !reflect.DeepEqual(got, tt.wantOrder) {
				t.Fatalf("stop sandbox order = %#v, want %#v", got, tt.wantOrder)
			}
			meta := readMeta(t, session.MetaPath())
			if got := meta.Sandbox.State; got != tt.wantState {
				t.Fatalf("sandbox state after stop = %q, want %q", got, tt.wantState)
			}
		})
	}
}

func TestSessionSandboxCrashSyncIsBestEffort(t *testing.T) {
	t.Parallel()

	syncErr := errors.New("runtime sync failed")
	provider := &recordingSandboxProvider{
		syncFromHook: func(_ sandbox.SessionState, opts sandbox.SyncOptions) (sandbox.SyncResult, error) {
			if got, want := opts.Reason, sandbox.SyncReasonCrash; got != want {
				t.Fatalf("SyncFromRuntime reason = %q, want %q", got, want)
			}
			return sandbox.SyncResult{}, syncErr
		},
	}
	h := newHarness(t, WithSandboxRegistry(newRegistryForProvider(t, provider)))
	session := createSession(t, h)

	h.driver.lastProcess().crash(errors.New("boom"), "stderr trace")
	waitForCondition(t, "session stopped after crash sync failure", func() bool {
		return h.notifier.stoppedCount() == 1
	})

	meta := readMeta(t, session.MetaPath())
	if got, want := meta.State, string(StateStopped); got != want {
		t.Fatalf("session state after crash = %q, want %q", got, want)
	}
	if meta.StopReason == nil || *meta.StopReason != store.StopAgentCrashed {
		t.Fatalf("StopReason after crash = %#v, want agent_crashed", meta.StopReason)
	}
	if got := meta.Sandbox.LastSyncError; got != syncErr.Error() {
		t.Fatalf("sandbox LastSyncError = %q, want %q", got, syncErr.Error())
	}
}

func TestSessionSandboxResumeRestoresProviderState(t *testing.T) {
	t.Parallel()

	provider := &recordingSandboxProvider{}
	h := newHarness(t, WithSandboxRegistry(newRegistryForProvider(t, provider)))
	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta := readMeta(t, session.MetaPath())
	meta.Sandbox.SandboxID = "env-resume"
	meta.Sandbox.InstanceID = "instance-resume"
	meta.Sandbox.ProviderState = json.RawMessage(`{"resume":true}`)
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	if got, want := len(provider.prepareRequests), 2; got != want {
		t.Fatalf("Prepare calls = %d, want %d", got, want)
	}
	req := provider.prepareRequests[1]
	if got, want := req.SandboxID, "env-resume"; got != want {
		t.Fatalf("resume PrepareRequest.SandboxID = %q, want %q", got, want)
	}
	if got, want := req.InstanceID, "instance-resume"; got != want {
		t.Fatalf("resume PrepareRequest.InstanceID = %q, want %q", got, want)
	}
	assertJSONEqual(t, req.ProviderState, json.RawMessage(`{"resume":true}`), "resume PrepareRequest.ProviderState")
}

func TestSessionSandboxLifecycleObserverReceivesRequiredFields(t *testing.T) {
	t.Parallel()

	observer := &recordingSandboxNotifier{}
	h := newHarness(t, WithNotifier(observer))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	events := observer.eventsSnapshot()
	if len(events) == 0 {
		t.Fatal("sandbox lifecycle events = empty, want events")
	}
	for _, event := range events {
		if event.SessionID != session.ID {
			t.Fatalf("event.SessionID = %q, want %q in %#v", event.SessionID, session.ID, event)
		}
		if event.WorkspaceID != h.workspaceID {
			t.Fatalf("event.WorkspaceID = %q, want %q in %#v", event.WorkspaceID, h.workspaceID, event)
		}
		if event.SandboxID == "" || event.Backend == "" || event.Profile == "" {
			t.Fatalf("event missing sandbox fields: %#v", event)
		}
		if event.Name == "" || event.Span == "" {
			t.Fatalf("event missing name/span: %#v", event)
		}
	}
}

func TestSessionSandboxHooksDispatchPayloadsAcrossLifecycle(t *testing.T) {
	t.Parallel()

	if err := os.WriteFile(filepath.Join(t.TempDir(), "unrelated.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("WriteFile(unrelated) error = %v", err)
	}
	provider := &recordingSandboxProvider{
		instanceID: "instance-hooked",
		syncToResult: sandbox.SyncResult{
			FilesSynced:      2,
			BytesTransferred: 17,
			Errors:           []string{"provider warning"},
		},
		syncFromResult: sandbox.SyncResult{
			FilesSynced:      1,
			BytesTransferred: 9,
		},
	}
	hooks := &recordingSandboxHooks{}
	h := newHarness(
		t,
		WithSandboxRegistry(newRegistryForProvider(t, provider)),
		WithSandboxIDGenerator(sequentialIDGenerator("env")),
		WithHookSet(HookSet{Sandbox: hooks}),
	)
	if err := os.WriteFile(filepath.Join(h.workspace, "tracked.txt"), []byte("workspace"), 0o644); err != nil {
		t.Fatalf("WriteFile(tracked) error = %v", err)
	}

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	events := hooks.eventsSnapshot()
	wantStart := []string{
		"sandbox.prepare",
		"sandbox.sync.before:to_runtime",
		"sandbox.sync.after:to_runtime",
		"sandbox.ready",
	}
	if !reflect.DeepEqual(events, wantStart) {
		t.Fatalf("start sandbox hook events = %#v, want %#v", events, wantStart)
	}

	prepare := hooks.prepareSnapshot()[0]
	if prepare.SandboxID != "env-1" || prepare.WorkspaceID != h.workspaceID || prepare.AgentName != "coder" {
		t.Fatalf("sandbox.prepare payload = %#v, want env/session context", prepare)
	}
	if prepare.Profile.Profile == "" || prepare.Backend != string(sandbox.BackendLocal) {
		t.Fatalf("sandbox.prepare profile/backend = %#v/%q, want local profile", prepare.Profile, prepare.Backend)
	}

	syncBefore := hooks.syncBeforeSnapshot()[0]
	if syncBefore.Direction != string(sandbox.SyncDirectionToRuntime) ||
		syncBefore.Reason != string(sandbox.SyncReasonStart) {
		t.Fatalf("sync.before start direction/reason = %q/%q", syncBefore.Direction, syncBefore.Reason)
	}
	syncAfter := hooks.syncAfterSnapshot()[0]
	if syncAfter.FilesSynced != 2 || syncAfter.BytesTransferred != 17 {
		t.Fatalf("sync.after stats = files %d bytes %d, want 2/17", syncAfter.FilesSynced, syncAfter.BytesTransferred)
	}
	if !reflect.DeepEqual(syncAfter.Errors, []string{"provider warning"}) {
		t.Fatalf("sync.after errors = %#v, want provider warning", syncAfter.Errors)
	}
	if syncAfter.DurationMS < 0 {
		t.Fatalf("sync.after duration_ms = %d, want non-negative", syncAfter.DurationMS)
	}
	ready := hooks.readySnapshot()[0]
	if ready.SandboxID != "env-1" || ready.InstanceID != "instance-hooked" {
		t.Fatalf("sandbox.ready payload = %#v, want env-1/instance-hooked", ready)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	events = hooks.eventsSnapshot()
	wantAll := []string{
		"sandbox.prepare",
		"sandbox.sync.before:to_runtime",
		"sandbox.sync.after:to_runtime",
		"sandbox.ready",
		"sandbox.sync.before:from_runtime",
		"sandbox.sync.after:from_runtime",
		"sandbox.stop",
	}
	if !reflect.DeepEqual(events, wantAll) {
		t.Fatalf("sandbox hook events = %#v, want %#v", events, wantAll)
	}
	stop := hooks.stopSnapshot()[0]
	if stop.SandboxID != "env-1" || stop.InstanceID != "instance-hooked" {
		t.Fatalf("sandbox.stop payload = %#v, want env-1/instance-hooked", stop)
	}
}

func TestSessionSandboxPrepareHookDenyAbortsSessionCreation(t *testing.T) {
	t.Parallel()

	provider := &recordingSandboxProvider{
		prepareHook: func(sandbox.PrepareRequest) error {
			t.Fatal("Prepare() called after sandbox.prepare denial")
			return nil
		},
	}
	hooks := &recordingSandboxHooks{
		prepareFn: func(
			_ context.Context,
			payload hookspkg.SandboxPreparePayload,
		) (hookspkg.SandboxPreparePayload, error) {
			payload.Denied = true
			payload.DenyReason = "policy"
			return payload, nil
		},
	}
	h := newHarness(
		t,
		WithSandboxRegistry(newRegistryForProvider(t, provider)),
		WithHookSet(HookSet{Sandbox: hooks}),
	)

	_, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
	})
	if err == nil {
		t.Fatal("Create() error = nil, want prepare denied error")
	}
	if !strings.Contains(err.Error(), "sandbox prepare denied") {
		t.Fatalf("Create() error = %v, want sandbox prepare denied", err)
	}
	if got := len(provider.prepareRequests); got != 0 {
		t.Fatalf("provider Prepare calls = %d, want 0", got)
	}
}

func TestSessionSandboxPrepareHookEnvOverridesMergeIntoSandboxConfig(t *testing.T) {
	t.Parallel()

	provider := &recordingSandboxProvider{}
	hooks := &recordingSandboxHooks{
		prepareFn: func(
			_ context.Context,
			payload hookspkg.SandboxPreparePayload,
		) (hookspkg.SandboxPreparePayload, error) {
			payload.EnvOverrides = map[string]string{
				"BASE":   "patched",
				"SECRET": "token",
			}
			return payload, nil
		},
	}
	h := newHarness(
		t,
		WithSandboxRegistry(newRegistryForProvider(t, provider)),
		WithHookSet(HookSet{Sandbox: hooks}),
	)
	resolved, err := h.resolver.Resolve(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
	}
	resolved.Sandbox.Env = map[string]string{"BASE": "original"}
	h.resolver.upsert(&resolved)

	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	req := provider.prepareRequests[0]
	if got := req.Sandbox.Env["BASE"]; got != "patched" {
		t.Fatalf("PrepareRequest.Sandbox.Env[BASE] = %q, want patched", got)
	}
	if got := req.Sandbox.Env["SECRET"]; got != "token" {
		t.Fatalf("PrepareRequest.Sandbox.Env[SECRET] = %q, want token", got)
	}
	if !stringSliceContains(req.AgentEnv, "BASE=patched") || !stringSliceContains(req.AgentEnv, "SECRET=token") {
		t.Fatalf("PrepareRequest.AgentEnv = %#v, want patched env overrides", req.AgentEnv)
	}
}

func TestSessionSandboxSyncBeforeDenySkipsSyncOperation(t *testing.T) {
	t.Parallel()

	provider := &recordingSandboxProvider{
		syncToHook: func(sandbox.SessionState, sandbox.SyncOptions) (sandbox.SyncResult, error) {
			t.Fatal("SyncToRuntime() called after sandbox.sync.before denial")
			return sandbox.SyncResult{}, nil
		},
	}
	hooks := &recordingSandboxHooks{
		syncBeforeFn: func(
			_ context.Context,
			payload hookspkg.SandboxSyncBeforePayload,
		) (hookspkg.SandboxSyncBeforePayload, error) {
			if payload.Direction == string(sandbox.SyncDirectionToRuntime) {
				payload.Denied = true
				payload.DenyReason = "skip initial sync"
			}
			return payload, nil
		},
	}
	h := newHarness(
		t,
		WithSandboxRegistry(newRegistryForProvider(t, provider)),
		WithHookSet(HookSet{Sandbox: hooks}),
	)

	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})
	if got := len(provider.syncToReasons); got != 0 {
		t.Fatalf("SyncToRuntime calls = %d, want 0", got)
	}
	if before := hooks.syncBeforeSnapshot()[0]; !before.Denied || before.DenyReason != "skip initial sync" {
		t.Fatalf("sync.before denied payload = %#v, want skip denial", before)
	}
}

func TestSessionSandboxSyncBeforeExcludePatternsPassToProvider(t *testing.T) {
	t.Parallel()

	wantPatterns := []string{"node_modules/**", "*.log"}
	provider := &recordingSandboxProvider{
		syncToHook: func(_ sandbox.SessionState, opts sandbox.SyncOptions) (sandbox.SyncResult, error) {
			if !reflect.DeepEqual(opts.ExcludePatterns, wantPatterns) {
				t.Fatalf("SyncToRuntime ExcludePatterns = %#v, want %#v", opts.ExcludePatterns, wantPatterns)
			}
			return sandbox.SyncResult{}, nil
		},
	}
	hooks := &recordingSandboxHooks{
		syncBeforeFn: func(
			_ context.Context,
			payload hookspkg.SandboxSyncBeforePayload,
		) (hookspkg.SandboxSyncBeforePayload, error) {
			if payload.Direction == string(sandbox.SyncDirectionToRuntime) {
				payload.ExcludePatterns = append([]string(nil), wantPatterns...)
			}
			return payload, nil
		},
	}
	h := newHarness(
		t,
		WithSandboxRegistry(newRegistryForProvider(t, provider)),
		WithHookSet(HookSet{Sandbox: hooks}),
	)

	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})
	if got := len(provider.syncToOptions); got != 1 {
		t.Fatalf("SyncToRuntime calls = %d, want 1", got)
	}
	if !reflect.DeepEqual(provider.syncToOptions[0].ExcludePatterns, wantPatterns) {
		t.Fatalf("recorded ExcludePatterns = %#v, want %#v", provider.syncToOptions[0].ExcludePatterns, wantPatterns)
	}
}

func TestSessionSandboxSyncBeforeWithoutHooksSkipsFileCount(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "workspace.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile(workspace) error = %v", err)
	}

	manager := &Manager{
		now: func() time.Time {
			return time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
		},
	}
	session := &Session{
		ID:          "sess-no-hooks",
		AgentName:   "coder",
		WorkspaceID: "ws-alpha",
		Workspace:   root,
		Type:        SessionTypeUser,
		State:       StateActive,
	}
	state := sandbox.SessionState{
		SandboxID:      "env-alpha",
		Backend:        sandbox.BackendLocal,
		Profile:        "local",
		RuntimeRootDir: root,
	}
	meta := &store.SessionSandboxMeta{
		SandboxID:      "env-alpha",
		Backend:        string(sandbox.BackendLocal),
		Profile:        "local",
		RuntimeRootDir: root,
	}

	_, err := manager.dispatchSandboxSyncBefore(
		context.Background(),
		session,
		state,
		meta,
		sandbox.SyncDirectionToRuntime,
		sandbox.SyncReasonStart,
	)
	if err != nil {
		t.Fatalf("dispatchSandboxSyncBefore() error = %v", err)
	}
}

func TestSessionSandboxStopDenyPreventsDestroyButStopsSession(t *testing.T) {
	t.Parallel()

	provider := &recordingSandboxProvider{
		destroyHook: func(sandbox.SessionState) error {
			t.Fatal("Destroy() called after sandbox.stop denial")
			return nil
		},
	}
	hooks := &recordingSandboxHooks{
		stopFn: func(
			_ context.Context,
			payload hookspkg.SandboxStopPayload,
		) (hookspkg.SandboxStopPayload, error) {
			payload.Denied = true
			payload.DenyReason = "retain sandbox"
			return payload, nil
		},
	}
	h := newHarness(
		t,
		WithSandboxRegistry(newRegistryForProvider(t, provider)),
		WithHookSet(HookSet{Sandbox: hooks}),
	)
	setHarnessSandbox(t, h, true)
	session := createSession(t, h)

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	info, err := h.manager.Status(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Status(%q) error = %v", session.ID, err)
	}
	if info.State != StateStopped {
		t.Fatalf("session state = %q, want stopped", info.State)
	}
	meta := readMeta(t, session.MetaPath())
	if got := meta.Sandbox.State; got != sandboxStateStopped {
		t.Fatalf("sandbox state = %q, want %q", got, sandboxStateStopped)
	}
	if got := len(provider.destroyStates); got != 0 {
		t.Fatalf("Destroy calls = %d, want 0", got)
	}
	stop := hooks.stopSnapshot()[0]
	if !stop.Denied || stop.WillDestroy != true {
		t.Fatalf("sandbox.stop payload = %#v, want denied with initial will_destroy", stop)
	}
}

func TestManagerExecSandboxUsesPreparedToolHost(t *testing.T) {
	t.Parallel()

	runtimeRoot := filepath.Join(t.TempDir(), "runtime")
	toolHost := &recordingSandboxToolHost{exitCode: 7, output: "terminal output"}
	provider := &recordingSandboxProvider{
		runtimeRoot: runtimeRoot,
		toolHost:    toolHost,
	}
	h := newHarness(t, WithSandboxRegistry(newRegistryForProvider(t, provider)))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	result, err := h.manager.ExecSandbox(testutil.Context(t), SandboxExecRequest{
		SessionID: session.ID,
		Command:   " echo ready ",
		Timeout:   time.Second,
	})
	if err != nil {
		t.Fatalf("ExecSandbox() error = %v", err)
	}
	if result.ExitCode != 7 || result.Stdout != "terminal output" || result.Stderr != "" {
		t.Fatalf("ExecSandbox() result = %#v, want exit/output without stderr", result)
	}

	toolHost.mu.Lock()
	defer toolHost.mu.Unlock()
	if len(toolHost.createRequests) != 1 {
		t.Fatalf("CreateTerminal calls = %d, want 1", len(toolHost.createRequests))
	}
	req := toolHost.createRequests[0]
	if req.Command != "echo ready" {
		t.Fatalf("CreateTerminal command = %q, want trimmed command", req.Command)
	}
	if req.Cwd == nil || *req.Cwd != runtimeRoot {
		t.Fatalf("CreateTerminal cwd = %v, want %q", req.Cwd, runtimeRoot)
	}
	if got, want := toolHost.waitIDs, []string{"term-1"}; !slices.Equal(got, want) {
		t.Fatalf("WaitForTerminalExit ids = %v, want %v", got, want)
	}
	if got, want := toolHost.outputIDs, []string{"term-1"}; !slices.Equal(got, want) {
		t.Fatalf("TerminalOutput ids = %v, want %v", got, want)
	}
	if got, want := toolHost.releaseIDs, []string{"term-1"}; !slices.Equal(got, want) {
		t.Fatalf("ReleaseTerminal ids = %v, want %v", got, want)
	}
}

func TestManagerExecSandboxReturnsWaitFailures(t *testing.T) {
	t.Parallel()

	runtimeRoot := filepath.Join(t.TempDir(), "runtime")
	toolHost := &recordingSandboxToolHost{
		exitCode: 1,
		output:   "partial output",
		waitErr:  errors.New("terminal exited due to signal"),
	}
	provider := &recordingSandboxProvider{
		runtimeRoot: runtimeRoot,
		toolHost:    toolHost,
	}
	h := newHarness(t, WithSandboxRegistry(newRegistryForProvider(t, provider)))
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	result, err := h.manager.ExecSandbox(testutil.Context(t), SandboxExecRequest{
		SessionID: session.ID,
		Command:   "echo ready",
		Timeout:   time.Second,
	})
	if err == nil {
		t.Fatal("ExecSandbox() error = nil, want wait failure")
	}
	if !strings.Contains(err.Error(), "sandbox exec wait") {
		t.Fatalf("ExecSandbox() error = %v, want sandbox exec wait context", err)
	}
	if !errors.Is(err, toolHost.waitErr) {
		t.Fatalf("ExecSandbox() error = %v, want wrapped wait error", err)
	}
	if result.ExitCode != toolHost.exitCode || result.Stdout != toolHost.output ||
		result.Stderr != toolHost.waitErr.Error() {
		t.Fatalf("ExecSandbox() result = %#v, want preserved wait failure result", result)
	}
	if got, want := len(toolHost.createRequests), 1; got != want {
		t.Fatalf("CreateTerminal calls = %d, want %d", got, want)
	}
	if got, want := toolHost.createRequests[0].Command, "echo ready"; got != want {
		t.Fatalf("CreateTerminal command = %q, want %q", got, want)
	}
	if toolHost.createRequests[0].Cwd == nil || *toolHost.createRequests[0].Cwd != runtimeRoot {
		t.Fatalf("CreateTerminal cwd = %v, want %q", toolHost.createRequests[0].Cwd, runtimeRoot)
	}
	if got, want := toolHost.waitIDs, []string{"term-1"}; !slices.Equal(got, want) {
		t.Fatalf("WaitForTerminalExit ids = %v, want %v", got, want)
	}
	if got, want := toolHost.outputIDs, []string{"term-1"}; !slices.Equal(got, want) {
		t.Fatalf("TerminalOutput ids = %v, want %v", got, want)
	}
	if got, want := toolHost.releaseIDs, []string{"term-1"}; !slices.Equal(got, want) {
		t.Fatalf("ReleaseTerminal ids = %v, want %v", got, want)
	}
}

func TestManagerExecSandboxValidationErrors(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	tests := []struct {
		name string
		ctx  context.Context
		req  SandboxExecRequest
	}{
		{
			name: "nil context",
			req:  SandboxExecRequest{SessionID: session.ID, Command: "pwd"},
		},
		{
			name: "blank session id",
			ctx:  testutil.Context(t),
			req:  SandboxExecRequest{Command: "pwd"},
		},
		{
			name: "blank command",
			ctx:  testutil.Context(t),
			req:  SandboxExecRequest{SessionID: session.ID},
		},
		{
			name: "missing session",
			ctx:  testutil.Context(t),
			req:  SandboxExecRequest{SessionID: "missing", Command: "pwd"},
		},
		{
			name: "missing tool host",
			ctx:  testutil.Context(t),
			req:  SandboxExecRequest{SessionID: session.ID, Command: "pwd"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := h.manager.ExecSandbox(tc.ctx, tc.req); err == nil {
				t.Fatal("ExecSandbox() error = nil, want error")
			}
		})
	}
}

type recordingSandboxProvider struct {
	mu                sync.Mutex
	prepareRequests   []sandbox.PrepareRequest
	syncToReasons     []sandbox.SyncReason
	syncFromReasons   []sandbox.SyncReason
	syncToOptions     []sandbox.SyncOptions
	syncFromOptions   []sandbox.SyncOptions
	destroyStates     []sandbox.SessionState
	runtimeRoot       string
	runtimeAdditional []string
	instanceID        string
	providerState     json.RawMessage
	syncToResult      sandbox.SyncResult
	syncFromResult    sandbox.SyncResult
	toolHost          sandbox.ToolHost
	prepareHook       func(sandbox.PrepareRequest) error
	syncToHook        func(sandbox.SessionState, sandbox.SyncOptions) (sandbox.SyncResult, error)
	syncFromHook      func(sandbox.SessionState, sandbox.SyncOptions) (sandbox.SyncResult, error)
	destroyHook       func(sandbox.SessionState) error
}

func (p *recordingSandboxProvider) Backend() sandbox.Backend {
	return sandbox.BackendLocal
}

func (p *recordingSandboxProvider) Prepare(
	_ context.Context,
	req sandbox.PrepareRequest,
) (sandbox.Prepared, error) {
	p.mu.Lock()
	p.prepareRequests = append(p.prepareRequests, clonePrepareRequest(req))
	p.mu.Unlock()

	if p.prepareHook != nil {
		if err := p.prepareHook(req); err != nil {
			return sandbox.Prepared{}, err
		}
	}

	runtimeRoot := p.runtimeRoot
	if runtimeRoot == "" {
		runtimeRoot = req.LocalRootDir
	}
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		return sandbox.Prepared{}, fmt.Errorf("create fake runtime root: %w", err)
	}
	runtimeAdditional := append([]string(nil), p.runtimeAdditional...)
	if runtimeAdditional == nil {
		runtimeAdditional = append([]string(nil), req.LocalAdditionalDirs...)
	}
	instanceID := p.instanceID
	if instanceID == "" {
		instanceID = req.InstanceID
	}
	providerState := append(json.RawMessage(nil), p.providerState...)
	if providerState == nil {
		providerState = append(json.RawMessage(nil), req.ProviderState...)
	}
	state := sandbox.SessionState{
		SandboxID:             req.SandboxID,
		Backend:               sandbox.BackendLocal,
		Profile:               req.Sandbox.Profile,
		State:                 sandboxStatePrepared,
		InstanceID:            instanceID,
		RuntimeRootDir:        runtimeRoot,
		RuntimeAdditionalDirs: append([]string(nil), runtimeAdditional...),
		ProviderState:         providerState,
		PreparedAt:            time.Now().UTC(),
	}
	return sandbox.Prepared{
		State:                 state,
		RuntimeRootDir:        runtimeRoot,
		RuntimeAdditionalDirs: append([]string(nil), runtimeAdditional...),
		ToolHost:              p.toolHost,
		Launch: sandbox.LaunchSpec{
			Command:        req.AgentCommand,
			Cwd:            runtimeRoot,
			AdditionalDirs: append([]string(nil), runtimeAdditional...),
			Env:            append([]string(nil), req.AgentEnv...),
		},
	}, nil
}

func (p *recordingSandboxProvider) SyncToRuntime(
	_ context.Context,
	state sandbox.SessionState,
	opts sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	p.mu.Lock()
	p.syncToReasons = append(p.syncToReasons, opts.Reason)
	p.syncToOptions = append(p.syncToOptions, cloneSyncOptions(opts))
	p.mu.Unlock()
	if p.syncToHook != nil {
		return p.syncToHook(state, opts)
	}
	return cloneSyncResult(p.syncToResult), nil
}

func (p *recordingSandboxProvider) SyncFromRuntime(
	_ context.Context,
	state sandbox.SessionState,
	opts sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	p.mu.Lock()
	p.syncFromReasons = append(p.syncFromReasons, opts.Reason)
	p.syncFromOptions = append(p.syncFromOptions, cloneSyncOptions(opts))
	p.mu.Unlock()
	if p.syncFromHook != nil {
		return p.syncFromHook(state, opts)
	}
	return cloneSyncResult(p.syncFromResult), nil
}

func (p *recordingSandboxProvider) Destroy(
	_ context.Context,
	state sandbox.SessionState,
) error {
	p.mu.Lock()
	p.destroyStates = append(p.destroyStates, state)
	p.mu.Unlock()
	if p.destroyHook != nil {
		return p.destroyHook(state)
	}
	return nil
}

type recordingSandboxToolHost struct {
	mu             sync.Mutex
	createRequests []acpsdk.CreateTerminalRequest
	waitIDs        []string
	outputIDs      []string
	releaseIDs     []string
	exitCode       int
	output         string
	waitErr        error
}

func (h *recordingSandboxToolHost) ReadTextFile(context.Context, string) (string, error) {
	return "", errors.New("test: ReadTextFile not implemented")
}

func (h *recordingSandboxToolHost) WriteTextFile(context.Context, string, string) error {
	return errors.New("test: WriteTextFile not implemented")
}

func (h *recordingSandboxToolHost) ResolvePath(path string) (string, error) {
	return path, nil
}

func (h *recordingSandboxToolHost) Authorize(sandbox.PermissionOperation) error {
	return nil
}

func (h *recordingSandboxToolHost) PermissionDecision(
	acpsdk.RequestPermissionRequest,
) (sandbox.PermissionDecision, bool) {
	return "", false
}

func (h *recordingSandboxToolHost) CreateTerminal(
	_ context.Context,
	req acpsdk.CreateTerminalRequest,
) (acpsdk.CreateTerminalResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.createRequests = append(h.createRequests, req)
	return acpsdk.CreateTerminalResponse{TerminalId: "term-1"}, nil
}

func (h *recordingSandboxToolHost) KillTerminal(string) error {
	return nil
}

func (h *recordingSandboxToolHost) TerminalOutput(id string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.outputIDs = append(h.outputIDs, id)
	return h.output, nil
}

func (h *recordingSandboxToolHost) WaitForTerminalExit(_ context.Context, id string) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.waitIDs = append(h.waitIDs, id)
	return h.exitCode, h.waitErr
}

func (h *recordingSandboxToolHost) ReleaseTerminal(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.releaseIDs = append(h.releaseIDs, id)
	return nil
}

type recordingSandboxHooks struct {
	mu           sync.Mutex
	events       []string
	prepare      []hookspkg.SandboxPreparePayload
	ready        []hookspkg.SandboxReadyPayload
	syncBefore   []hookspkg.SandboxSyncBeforePayload
	syncAfter    []hookspkg.SandboxSyncAfterPayload
	stop         []hookspkg.SandboxStopPayload
	prepareFn    func(context.Context, hookspkg.SandboxPreparePayload) (hookspkg.SandboxPreparePayload, error)
	readyFn      func(context.Context, hookspkg.SandboxReadyPayload) (hookspkg.SandboxReadyPayload, error)
	syncBeforeFn func(context.Context, hookspkg.SandboxSyncBeforePayload) (hookspkg.SandboxSyncBeforePayload, error)
	syncAfterFn  func(context.Context, hookspkg.SandboxSyncAfterPayload) (hookspkg.SandboxSyncAfterPayload, error)
	stopFn       func(context.Context, hookspkg.SandboxStopPayload) (hookspkg.SandboxStopPayload, error)
}

func (h *recordingSandboxHooks) DispatchSandboxPrepare(
	ctx context.Context,
	payload hookspkg.SandboxPreparePayload,
) (hookspkg.SandboxPreparePayload, error) {
	result := payload
	var err error
	if h.prepareFn != nil {
		result, err = h.prepareFn(ctx, payload)
	}
	h.mu.Lock()
	h.events = append(h.events, string(hookspkg.HookSandboxPrepare))
	h.prepare = append(h.prepare, cloneSandboxPreparePayload(result))
	h.mu.Unlock()
	return result, err
}

func (h *recordingSandboxHooks) DispatchSandboxReady(
	ctx context.Context,
	payload hookspkg.SandboxReadyPayload,
) (hookspkg.SandboxReadyPayload, error) {
	result := payload
	var err error
	if h.readyFn != nil {
		result, err = h.readyFn(ctx, payload)
	}
	h.mu.Lock()
	h.events = append(h.events, string(hookspkg.HookSandboxReady))
	h.ready = append(h.ready, cloneSandboxReadyPayload(result))
	h.mu.Unlock()
	return result, err
}

func (h *recordingSandboxHooks) DispatchSandboxSyncBefore(
	ctx context.Context,
	payload hookspkg.SandboxSyncBeforePayload,
) (hookspkg.SandboxSyncBeforePayload, error) {
	result := payload
	var err error
	if h.syncBeforeFn != nil {
		result, err = h.syncBeforeFn(ctx, payload)
	}
	h.mu.Lock()
	h.events = append(h.events, string(hookspkg.HookSandboxSyncBefore)+":"+result.Direction)
	h.syncBefore = append(h.syncBefore, cloneSandboxSyncBeforePayload(result))
	h.mu.Unlock()
	return result, err
}

func (h *recordingSandboxHooks) DispatchSandboxSyncAfter(
	ctx context.Context,
	payload hookspkg.SandboxSyncAfterPayload,
) (hookspkg.SandboxSyncAfterPayload, error) {
	result := payload
	var err error
	if h.syncAfterFn != nil {
		result, err = h.syncAfterFn(ctx, payload)
	}
	h.mu.Lock()
	h.events = append(h.events, string(hookspkg.HookSandboxSyncAfter)+":"+result.Direction)
	h.syncAfter = append(h.syncAfter, cloneSandboxSyncAfterPayload(result))
	h.mu.Unlock()
	return result, err
}

func (h *recordingSandboxHooks) DispatchSandboxStop(
	ctx context.Context,
	payload hookspkg.SandboxStopPayload,
) (hookspkg.SandboxStopPayload, error) {
	result := payload
	var err error
	if h.stopFn != nil {
		result, err = h.stopFn(ctx, payload)
	}
	h.mu.Lock()
	h.events = append(h.events, string(hookspkg.HookSandboxStop))
	h.stop = append(h.stop, result)
	h.mu.Unlock()
	return result, err
}

func (h *recordingSandboxHooks) eventsSnapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.events...)
}

func (h *recordingSandboxHooks) prepareSnapshot() []hookspkg.SandboxPreparePayload {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]hookspkg.SandboxPreparePayload(nil), h.prepare...)
}

func (h *recordingSandboxHooks) readySnapshot() []hookspkg.SandboxReadyPayload {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]hookspkg.SandboxReadyPayload(nil), h.ready...)
}

func (h *recordingSandboxHooks) syncBeforeSnapshot() []hookspkg.SandboxSyncBeforePayload {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]hookspkg.SandboxSyncBeforePayload(nil), h.syncBefore...)
}

func (h *recordingSandboxHooks) syncAfterSnapshot() []hookspkg.SandboxSyncAfterPayload {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]hookspkg.SandboxSyncAfterPayload(nil), h.syncAfter...)
}

func (h *recordingSandboxHooks) stopSnapshot() []hookspkg.SandboxStopPayload {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]hookspkg.SandboxStopPayload(nil), h.stop...)
}

type orderingRecorder struct {
	onClose func()
}

func (r *orderingRecorder) Record(context.Context, store.SessionEvent) error {
	return nil
}

func (r *orderingRecorder) RecordTokenUsage(context.Context, store.TokenUsage) error {
	return nil
}

func (r *orderingRecorder) Query(context.Context, store.EventQuery) ([]store.SessionEvent, error) {
	return nil, nil
}

func (r *orderingRecorder) History(context.Context, store.EventQuery) ([]store.TurnHistory, error) {
	return nil, nil
}

func (r *orderingRecorder) Close(context.Context) error {
	if r.onClose != nil {
		r.onClose()
	}
	return nil
}

type recordingSandboxNotifier struct {
	mu     sync.Mutex
	events []SandboxLifecycleEvent
}

func (n *recordingSandboxNotifier) OnSessionCreated(context.Context, *Session) {}

func (n *recordingSandboxNotifier) OnSessionStopped(context.Context, *Session) {}

func (n *recordingSandboxNotifier) OnAgentEvent(context.Context, string, any) {}

func (n *recordingSandboxNotifier) OnSandboxLifecycleEvent(
	_ context.Context,
	event SandboxLifecycleEvent,
) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.events = append(n.events, event)
}

func (n *recordingSandboxNotifier) eventsSnapshot() []SandboxLifecycleEvent {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]SandboxLifecycleEvent(nil), n.events...)
}

func newRegistryForProvider(t *testing.T, provider sandbox.Provider) *sandbox.Registry {
	t.Helper()
	registry, err := sandbox.NewRegistry(provider)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}

func setHarnessSandbox(t *testing.T, h *harness, destroyOnStop bool) {
	t.Helper()
	resolved, err := h.resolver.Resolve(context.Background(), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
	}
	resolved.Sandbox.DestroyOnStop = destroyOnStop
	h.resolver.upsert(&resolved)
}

func clonePrepareRequest(req sandbox.PrepareRequest) sandbox.PrepareRequest {
	cloned := req
	cloned.LocalAdditionalDirs = append([]string(nil), req.LocalAdditionalDirs...)
	cloned.AgentEnv = append([]string(nil), req.AgentEnv...)
	cloned.ProviderState = append(json.RawMessage(nil), req.ProviderState...)
	cloned.Sandbox.Env = cloneStringMapForSandboxTests(req.Sandbox.Env)
	return cloned
}

func cloneSyncOptions(opts sandbox.SyncOptions) sandbox.SyncOptions {
	cloned := opts
	cloned.ExcludePatterns = append([]string(nil), opts.ExcludePatterns...)
	return cloned
}

func cloneSyncResult(result sandbox.SyncResult) sandbox.SyncResult {
	cloned := result
	cloned.Errors = append([]string(nil), result.Errors...)
	return cloned
}

func cloneSandboxPreparePayload(
	payload hookspkg.SandboxPreparePayload,
) hookspkg.SandboxPreparePayload {
	cloned := payload
	cloned.LocalAdditionalDirs = append([]string(nil), payload.LocalAdditionalDirs...)
	cloned.AgentEnv = append([]string(nil), payload.AgentEnv...)
	cloned.EnvOverrides = cloneStringMapForSandboxTests(payload.EnvOverrides)
	cloned.Profile.Env = cloneStringMapForSandboxTests(payload.Profile.Env)
	return cloned
}

func cloneSandboxReadyPayload(payload hookspkg.SandboxReadyPayload) hookspkg.SandboxReadyPayload {
	cloned := payload
	cloned.RuntimeAdditionalDirs = append([]string(nil), payload.RuntimeAdditionalDirs...)
	return cloned
}

func cloneSandboxSyncBeforePayload(
	payload hookspkg.SandboxSyncBeforePayload,
) hookspkg.SandboxSyncBeforePayload {
	cloned := payload
	cloned.ExcludePatterns = append([]string(nil), payload.ExcludePatterns...)
	return cloned
}

func cloneSandboxSyncAfterPayload(
	payload hookspkg.SandboxSyncAfterPayload,
) hookspkg.SandboxSyncAfterPayload {
	cloned := payload
	cloned.Errors = append([]string(nil), payload.Errors...)
	return cloned
}

func cloneStringMapForSandboxTests(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	maps.Copy(cloned, values)
	return cloned
}

func stringSliceContains(values []string, want string) bool {
	return slices.Contains(values, want)
}

func orderSnapshot(mu *sync.Mutex, order []string) []string {
	mu.Lock()
	defer mu.Unlock()
	return append([]string(nil), order...)
}

func assertJSONEqual(t *testing.T, got json.RawMessage, want json.RawMessage, label string) {
	t.Helper()
	var gotValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("%s invalid JSON %s: %v", label, string(got), err)
	}
	var wantValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("%s invalid expected JSON %s: %v", label, string(want), err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("%s = %#v, want %#v", label, gotValue, wantValue)
	}
}

var _ sandbox.Provider = (*recordingSandboxProvider)(nil)
var _ EventRecorder = (*orderingRecorder)(nil)
var _ SandboxLifecycleNotifier = (*recordingSandboxNotifier)(nil)
var _ SandboxHooks = (*recordingSandboxHooks)(nil)
var _ workspacepkg.RuntimeResolver = (*fakeWorkspaceResolver)(nil)
