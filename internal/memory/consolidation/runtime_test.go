package consolidation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestRuntimeTriggerReturnsAlreadyRunningWhenLockUnavailable(t *testing.T) {
	t.Parallel()
	t.Run("Should report an already-running lock conflict as a clean skip", func(t *testing.T) {
		t.Parallel()

		service := &fakeDreamService{
			shouldRun: true,
			runErr:    memory.ErrLockUnavailable,
		}
		runtime := newTestRuntime(true, service, time.Minute)

		triggered, reason, err := runtime.Trigger(context.Background(), "ws-1")
		if err != nil {
			t.Fatalf("Trigger() error = %v", err)
		}
		if triggered {
			t.Fatal("Trigger() triggered = true, want false")
		}
		if reason != "dream consolidation is already running" {
			t.Fatalf("Trigger() reason = %q, want already-running message", reason)
		}
	})
}

func TestRuntimeTriggerReturnsGateMissWhenRunSignalGateMisses(t *testing.T) {
	t.Parallel()
	t.Run("Should report a signal-gate miss as a clean skip", func(t *testing.T) {
		t.Parallel()

		service := &fakeDreamService{
			shouldRun: true,
			runErr:    memory.ErrDreamGateNotSatisfied,
		}
		runtime := newTestRuntime(true, service, time.Minute)

		triggered, reason, err := runtime.Trigger(context.Background(), "ws-1")
		if err != nil {
			t.Fatalf("Trigger() error = %v", err)
		}
		if triggered {
			t.Fatal("Trigger() triggered = true, want false")
		}
		if reason != "dream consolidation gates are not satisfied" {
			t.Fatalf("Trigger() reason = %q, want gates-not-satisfied message", reason)
		}
	})
}

func TestRuntimeTriggerStates(t *testing.T) {
	t.Parallel()

	t.Run("Should disabled returns disabled message", func(t *testing.T) {
		runtime := newTestRuntime(false, &fakeDreamService{shouldRun: true}, time.Minute)

		triggered, reason, err := runtime.Trigger(context.Background(), "ws-1")
		if err != nil {
			t.Fatalf("Trigger() error = %v", err)
		}
		if triggered {
			t.Fatal("Trigger() triggered = true, want false")
		}
		if reason != "dream consolidation is disabled" {
			t.Fatalf("Trigger() reason = %q, want disabled message", reason)
		}
	})

	t.Run("Should gate miss returns not satisfied message", func(t *testing.T) {
		runtime := newTestRuntime(true, &fakeDreamService{shouldRun: false}, time.Minute)

		triggered, reason, err := runtime.Trigger(context.Background(), "ws-1")
		if err != nil {
			t.Fatalf("Trigger() error = %v", err)
		}
		if triggered {
			t.Fatal("Trigger() triggered = true, want false")
		}
		if reason != "dream consolidation gates are not satisfied" {
			t.Fatalf("Trigger() reason = %q, want gates-not-satisfied message", reason)
		}
	})

	t.Run("Should service error is returned", func(t *testing.T) {
		expectedErr := errors.New("gate failed")
		runtime := newTestRuntime(true, &fakeDreamService{shouldRunErr: expectedErr}, time.Minute)

		_, _, err := runtime.Trigger(context.Background(), "ws-1")
		if !errors.Is(err, expectedErr) {
			t.Fatalf("Trigger() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("Should successful run trims workspace", func(t *testing.T) {
		service := &fakeDreamService{shouldRun: true}
		runtime := newTestRuntime(true, service, time.Minute)

		triggered, reason, err := runtime.Trigger(context.Background(), "  ws-1  ")
		if err != nil {
			t.Fatalf("Trigger() error = %v", err)
		}
		if !triggered {
			t.Fatal("Trigger() triggered = false, want true")
		}
		if reason != "" {
			t.Fatalf("Trigger() reason = %q, want empty", reason)
		}
		if got := service.lastWorkspace(); got != "ws-1" {
			t.Fatalf("service workspace = %q, want ws-1", got)
		}
	})
}

func TestRuntimeLastConsolidatedAt(t *testing.T) {
	t.Parallel()

	t.Run("Should nil callback returns zero time", func(t *testing.T) {
		runtime := NewRuntime(staticEnabled(true), nil, nil, time.Minute, discardLogger(), nil)
		got, err := runtime.LastConsolidatedAt()
		if err != nil {
			t.Fatalf("LastConsolidatedAt() error = %v", err)
		}
		if !got.IsZero() {
			t.Fatalf("LastConsolidatedAt() = %v, want zero time", got)
		}
	})

	t.Run("Should callback result is returned", func(t *testing.T) {
		expected := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
		runtime := NewRuntime(staticEnabled(true), nil, nil, time.Minute, discardLogger(), func() (time.Time, error) {
			return expected, nil
		})

		got, err := runtime.LastConsolidatedAt()
		if err != nil {
			t.Fatalf("LastConsolidatedAt() error = %v", err)
		}
		if !got.Equal(expected) {
			t.Fatalf("LastConsolidatedAt() = %v, want %v", got, expected)
		}
	})
}

func TestRuntimeTickerRunsAndStopsOnCancellation(t *testing.T) {
	t.Parallel()
	t.Run("Should run scheduled checks and stop after cancellation", func(t *testing.T) {
		t.Parallel()

		service := &fakeDreamService{shouldRun: true}
		runtime := newTestRuntime(true, service, 10*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		runtime.Start(ctx)
		t.Cleanup(runtime.Shutdown)

		waitForCondition(t, "dream ticker run", func() bool {
			return service.runCount() > 0
		})

		cancel()
		runtime.Shutdown()

		runCount := service.runCount()
		time.Sleep(30 * time.Millisecond)
		if got := service.runCount(); got != runCount {
			t.Fatalf("run count after shutdown = %d, want %d", got, runCount)
		}
	})
}

func TestRuntimeEnqueueCheckRunsQueuedRequest(t *testing.T) {
	t.Parallel()
	t.Run("Should execute a queued workspace check", func(t *testing.T) {
		t.Parallel()

		service := &fakeDreamService{shouldRun: true}
		runtime := newTestRuntime(true, service, time.Hour)

		ctx := t.Context()
		runtime.Start(ctx)
		t.Cleanup(runtime.Shutdown)

		runtime.EnqueueCheck("session_stop", "  ws-queued  ")
		waitForCondition(t, "queued dream check", func() bool {
			return service.runCount() == 1
		})

		if got := service.lastWorkspace(); got != "ws-queued" {
			t.Fatalf("queued workspace = %q, want trimmed queued workspace", got)
		}
	})
}

func TestRuntimeSkipsChecksWhileDisabled(t *testing.T) {
	t.Parallel()
	t.Run("Should keep the scheduler idle while disabled", func(t *testing.T) {
		t.Parallel()

		var observed atomic.Int64
		service := &fakeDreamService{shouldRun: true}
		runtime := NewRuntime(
			func() bool {
				observed.Add(1)
				return false
			},
			service,
			func(context.Context, string, string, string, time.Time) error { return nil },
			time.Hour,
			discardLogger(),
			nil,
		)

		runtime.Start(t.Context())
		t.Cleanup(runtime.Shutdown)
		runtime.EnqueueCheck("manual", "ws-disabled")
		waitForCondition(t, "disabled queued check", func() bool {
			return observed.Load() > 0
		})

		if got := service.runCount(); got != 0 {
			t.Fatalf("run count = %d, want 0", got)
		}
	})
}

func TestRuntimeReadsLiveEnabledState(t *testing.T) {
	t.Parallel()
	t.Run("Should read enabled state for each trigger", func(t *testing.T) {
		t.Parallel()

		enabled := false
		service := &fakeDreamService{shouldRun: true}
		runtime := NewRuntime(func() bool { return enabled }, service, func(
			context.Context,
			string,
			string,
			string,
			time.Time,
		) error {
			return nil
		}, time.Minute, discardLogger(), nil)

		triggered, _, err := runtime.Trigger(context.Background(), "ws-live")
		if err != nil {
			t.Fatalf("Trigger(disabled) error = %v", err)
		}
		if triggered || service.runCount() != 0 {
			t.Fatalf("Trigger(disabled) = %t with %d runs, want skipped", triggered, service.runCount())
		}

		enabled = true
		triggered, _, err = runtime.Trigger(context.Background(), "ws-live")
		if err != nil {
			t.Fatalf("Trigger(enabled) error = %v", err)
		}
		if !triggered || service.runCount() != 1 {
			t.Fatalf("Trigger(enabled) = %t with %d runs, want one run", triggered, service.runCount())
		}
	})

	t.Run("Should run a queued check after live enablement", func(t *testing.T) {
		t.Parallel()

		var enabled atomic.Bool
		service := &fakeDreamService{shouldRun: true}
		runtime := NewRuntime(
			enabled.Load,
			service,
			func(context.Context, string, string, string, time.Time) error { return nil },
			time.Hour,
			discardLogger(),
			nil,
		)
		runtime.Start(t.Context())
		t.Cleanup(runtime.Shutdown)

		enabled.Store(true)
		runtime.EnqueueCheck("live-enable", "ws-live")
		waitForCondition(t, "live-enabled queued dream check", func() bool {
			return service.runCount() == 1
		})
		if got := service.lastWorkspace(); got != "ws-live" {
			t.Fatalf("queued workspace = %q, want ws-live", got)
		}
	})
}

func TestRuntimeRunCheckStopsOnErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should lock unavailable is swallowed", func(t *testing.T) {
		service := &fakeDreamService{shouldRun: true, runErr: memory.ErrLockUnavailable}
		runtime := NewRuntime(
			staticEnabled(true),
			service,
			func(context.Context, string, string, string, time.Time) error { return nil },
			time.Minute,
			discardLogger(),
			nil,
		)

		runtime.runCheck(
			context.Background(),
			discardLogger(),
			service,
			func(context.Context, string, string, string, time.Time) error {
				return nil
			},
			"manual",
			"ws-1",
		)
		if got := service.runCount(); got != 1 {
			t.Fatalf("run count = %d, want 1", got)
		}
	})

	t.Run("Should signal gate miss is swallowed", func(t *testing.T) {
		service := &fakeDreamService{shouldRun: true, runErr: memory.ErrDreamGateNotSatisfied}
		runtime := NewRuntime(
			staticEnabled(true),
			service,
			func(context.Context, string, string, string, time.Time) error { return nil },
			time.Minute,
			discardLogger(),
			nil,
		)

		runtime.runCheck(
			context.Background(),
			discardLogger(),
			service,
			func(context.Context, string, string, string, time.Time) error {
				return nil
			},
			"manual",
			"ws-1",
		)
		if got := service.runCount(); got != 1 {
			t.Fatalf("run count = %d, want 1", got)
		}
	})

	t.Run("Should skip spawn on should-run error", func(t *testing.T) {
		service := &fakeDreamService{shouldRunErr: errors.New("gate failed")}
		spawnCalls := 0
		runtime := NewRuntime(
			staticEnabled(true),
			service,
			func(context.Context, string, string, string, time.Time) error {
				spawnCalls++
				return nil
			},
			time.Minute,
			discardLogger(),
			nil,
		)

		runtime.runCheck(
			context.Background(),
			discardLogger(),
			service,
			func(context.Context, string, string, string, time.Time) error {
				spawnCalls++
				return nil
			},
			"manual",
			"ws-1",
		)
		if spawnCalls != 0 {
			t.Fatalf("spawn calls = %d, want 0", spawnCalls)
		}
	})
}

func TestNewSessionSpawnerCreatesDreamSession(t *testing.T) {
	t.Parallel()
	t.Run("Should create and stop a dream session through the resolved route", testNewSessionSpawnerCreatesDreamSession)
}

func testNewSessionSpawnerCreatesDreamSession(t *testing.T) {
	t.Helper()
	t.Parallel()

	cfg := dreamConfig()
	cfg.Roles.Dream.Provider = "pi"
	sessions := &fakeSessionManager{}
	workspace := filepath.Join(t.TempDir(), "workspace")
	resolver := &fakeWorkspaceResolver{
		resolveOrRegisterResolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{ID: "ws-created", RootDir: workspace},
		},
	}

	spawner := newTestSessionSpawner(sessions, resolver, &cfg)
	if spawner == nil {
		t.Fatal("NewSessionSpawner() = nil, want non-nil")
	}

	if err := spawner(
		context.Background(),
		"memory-consolidation",
		"summarize recent sessions",
		workspace,
		time.Time{},
	); err != nil {
		t.Fatalf("spawner() error = %v", err)
	}

	if got := sessions.createCount(); got != 1 {
		t.Fatalf("Create() calls = %d, want 1", got)
	}
	if got := sessions.createCall(0).Type; got != session.SessionTypeDream {
		t.Fatalf("Create() type = %q, want %q", got, session.SessionTypeDream)
	}
	if got := sessions.createCall(0).Provider; got != "pi" {
		t.Fatalf("Create() provider = %q, want pi from dream route", got)
	}
	if got := sessions.createCall(0).AgentName; got != "memory-agent" {
		t.Fatalf("Create() agent = %q, want explicit configured memory-agent", got)
	}
	if got := sessions.createCall(0).Workspace; got != "ws-created" {
		t.Fatalf("Create() workspace = %q, want ws-created", got)
	}
	if got := sessions.createCall(0).WorkspacePath; got != "" {
		t.Fatalf("Create() workspace_path = %q, want empty", got)
	}
	if got := sessions.promptCount(); got != 1 || sessions.promptCall(0).msg != "summarize recent sessions" {
		t.Fatalf("Prompt() calls = %d, want one prompt payload", got)
	}
	if got := sessions.stopCount(); got != 1 || sessions.stopCall(0) != "dream-1" {
		t.Fatalf("Stop() calls = %d, want stop for created dream session", got)
	}
	if got := resolver.resolveOrRegisterCalls; got != 1 {
		t.Fatalf("ResolveOrRegister() calls = %d, want 1", got)
	}
}

func TestNewSessionSpawnerUsesResolvedBuiltInDreamingCurator(t *testing.T) {
	t.Parallel()
	t.Run("Should use the resolved built-in dreaming curator", testNewSessionSpawnerUsesResolvedBuiltInDreamingCurator)
}

func testNewSessionSpawnerUsesResolvedBuiltInDreamingCurator(t *testing.T) {
	t.Helper()
	t.Parallel()

	cfg := dreamConfig()
	cfg.Roles.Dream.Agent = aghconfig.BuiltinDreamingCuratorAgentName
	sessions := &fakeSessionManager{}
	resolver := &fakeWorkspaceResolver{
		resolveResolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{ID: "ws-default", RootDir: filepath.Join(t.TempDir(), "workspace")},
		},
	}

	spawner := newTestSessionSpawner(sessions, resolver, &cfg)
	if err := spawner(context.Background(), "memory-consolidation", "prompt", "ws-default", time.Time{}); err != nil {
		t.Fatalf("spawner() error = %v", err)
	}

	if got := sessions.createCall(0).AgentName; got != aghconfig.BuiltinDreamingCuratorAgentName {
		t.Fatalf("Create() agent = %q, want %q", got, aghconfig.BuiltinDreamingCuratorAgentName)
	}
}

func TestNewSessionSpawnerResolvesExplicitAliasWorkspace(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should resolve an explicit workspace alias before spawning",
		testNewSessionSpawnerResolvesExplicitAliasWorkspace,
	)
}

func testNewSessionSpawnerResolvesExplicitAliasWorkspace(t *testing.T) {
	t.Helper()
	t.Parallel()

	cfg := dreamConfig()
	sessions := &fakeSessionManager{}
	resolver := &fakeWorkspaceResolver{
		resolveResolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{ID: "ws-alias", RootDir: filepath.Join(t.TempDir(), "workspace")},
		},
	}

	spawner := newTestSessionSpawner(sessions, resolver, &cfg)
	if err := spawner(
		context.Background(),
		"memory-consolidation",
		"prompt",
		"workspace-alias",
		time.Time{},
	); err != nil {
		t.Fatalf("spawner() error = %v", err)
	}

	if got := resolver.resolveCalls; got != 1 {
		t.Fatalf("Resolve() calls = %d, want 1", got)
	}
	if got := resolver.lastResolveArg; got != "workspace-alias" {
		t.Fatalf("Resolve() arg = %q, want workspace-alias", got)
	}
	if got := sessions.createCall(0).Workspace; got != "ws-alias" {
		t.Fatalf("Create() workspace = %q, want ws-alias", got)
	}
	if got := sessions.createCall(0).Provider; got != "" {
		t.Fatalf("Create() provider = %q, want explicit empty provider", got)
	}
}

func TestNewSessionSpawnerPropagatesWorkspaceResolveErrors(t *testing.T) {
	t.Parallel()
	t.Run("Should propagate workspace resolution errors", testNewSessionSpawnerPropagatesWorkspaceResolveErrors)
}

func testNewSessionSpawnerPropagatesWorkspaceResolveErrors(t *testing.T) {
	t.Helper()
	t.Parallel()

	cfg := dreamConfig()
	expectedErr := errors.New("lookup failed")
	spawner := NewSessionSpawner(
		&fakeSessionManager{},
		&fakeWorkspaceResolver{resolveErr: expectedErr},
		cfg.Memory.Enabled,
		testDreamRouteResolver(&cfg),
	)

	err := spawner(context.Background(), "memory-consolidation", "prompt", "workspace-alias", time.Time{})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("spawner() error = %v, want %v", err, expectedErr)
	}
}

func TestNewSessionSpawnerRejectsAWorkspaceDisabledDreamRole(t *testing.T) {
	t.Parallel()
	t.Run("Should return ErrDreamRoleDisabled when the workspace dream role is off", func(t *testing.T) {
		t.Parallel()

		cfg := dreamConfig()
		cfg.Roles.Dream.Enabled = false
		resolver := &fakeWorkspaceResolver{
			resolveResolved: workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{ID: "ws-disabled", RootDir: t.TempDir()},
			},
		}
		spawner := newTestSessionSpawner(&fakeSessionManager{}, resolver, &cfg)
		err := spawner(context.Background(), "memory-consolidation", "prompt", "ws-disabled", time.Time{})
		if !errors.Is(err, memory.ErrDreamRoleDisabled) {
			t.Fatalf("spawner() error = %v, want ErrDreamRoleDisabled", err)
		}
	})
}

func TestIsPathLikeWorkspaceRefRecognizesSlashSeparatedRefs(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should recognize slash-separated workspace references",
		testIsPathLikeWorkspaceRefRecognizesSlashSeparatedRefs,
	)
}

func testIsPathLikeWorkspaceRefRecognizesSlashSeparatedRefs(t *testing.T) {
	t.Helper()
	t.Parallel()

	if !isPathLikeWorkspaceRef("subdir/workspace") {
		t.Fatal(`isPathLikeWorkspaceRef("subdir/workspace") = false, want true`)
	}
	if !isPathLikeWorkspaceRef(`subdir\workspace`) {
		t.Fatal(`isPathLikeWorkspaceRef("subdir\\workspace") = false, want true`)
	}
}

func TestNewSessionSpawnerDerivesRecentWorkspacesFromSessions(t *testing.T) {
	t.Parallel()
	t.Run("Should derive recent workspaces in recency order", testNewSessionSpawnerDerivesRecentWorkspacesFromSessions)
}

func testNewSessionSpawnerDerivesRecentWorkspacesFromSessions(t *testing.T) {
	t.Helper()
	t.Parallel()

	cfg := dreamConfig()
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{
				ID:          "dream-old",
				WorkspaceID: "ws-a",
				Type:        session.SessionTypeDream,
				UpdatedAt:   time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC),
			},
			{
				ID:          "user-old",
				WorkspaceID: "ws-a",
				Type:        session.SessionTypeUser,
				UpdatedAt:   time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
			},
			{
				ID:          "user-new",
				WorkspaceID: "ws-b",
				Type:        session.SessionTypeUser,
				UpdatedAt:   time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC),
			},
			{
				ID:          "user-dup",
				WorkspaceID: "ws-a",
				Type:        session.SessionTypeUser,
				UpdatedAt:   time.Date(2026, 4, 4, 9, 0, 0, 0, time.UTC),
			},
		},
	}
	prior := time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC)

	spawner := newTestSessionSpawner(sessions, &fakeWorkspaceResolver{}, &cfg)
	if err := spawner(context.Background(), "memory-consolidation", "prompt", "", prior); err != nil {
		t.Fatalf("spawner() error = %v", err)
	}

	if got := sessions.createCount(); got != 2 {
		t.Fatalf("Create() calls = %d, want 2", got)
	}
	if got := sessions.createCall(0).Workspace; got != "ws-b" {
		t.Fatalf("Create() workspace[0] = %q, want ws-b", got)
	}
	if got := sessions.createCall(0).Provider; got != "" {
		t.Fatalf("Create() provider[0] = %q, want explicit empty provider", got)
	}
	if got := sessions.createCall(1).Workspace; got != "ws-a" {
		t.Fatalf("Create() workspace[1] = %q, want ws-a", got)
	}
	if got := sessions.createCall(1).Provider; got != "" {
		t.Fatalf("Create() provider[1] = %q, want explicit empty provider", got)
	}
}

func TestResolveWorkspaceRefValidatesInputs(t *testing.T) {
	t.Parallel()

	t.Run("Should blank ref is rejected", func(t *testing.T) {
		_, err := resolveWorkspaceRef(context.Background(), &fakeWorkspaceResolver{}, "   ")
		if err == nil {
			t.Fatal("resolveWorkspaceRef() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "dream workspace is required") {
			t.Fatalf("resolveWorkspaceRef() error = %v, want blank workspace error", err)
		}
	})

	t.Run("Should empty resolved id is rejected", func(t *testing.T) {
		_, err := resolveWorkspaceRef(context.Background(), &fakeWorkspaceResolver{
			resolveResolved: workspacepkg.ResolvedWorkspace{},
		}, "workspace-alias")
		if err == nil {
			t.Fatal("resolveWorkspaceRef() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "dream workspace id is required") {
			t.Fatalf("resolveWorkspaceRef() error = %v, want empty id error", err)
		}
	})
}

func TestNewSessionSpawnerReturnsNoRecentWorkspacesWhenSessionsAreOld(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should reject a run with no recent workspaces",
		testNewSessionSpawnerReturnsNoRecentWorkspacesWhenSessionsAreOld,
	)
}

func testNewSessionSpawnerReturnsNoRecentWorkspacesWhenSessionsAreOld(t *testing.T) {
	t.Helper()
	t.Parallel()

	cfg := dreamConfig()
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{
				ID:          "user-old",
				WorkspaceID: "ws-a",
				Type:        session.SessionTypeUser,
				UpdatedAt:   time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	globalMemoryDir := filepath.Join(t.TempDir(), "memory")
	lockPath := memory.ConsolidationLockPath(globalMemoryDir)
	prior := time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(lock dir) error = %v", err)
	}
	if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
		t.Fatalf("os.WriteFile(lock) error = %v", err)
	}
	if err := os.Chtimes(lockPath, prior, prior); err != nil {
		t.Fatalf("os.Chtimes(lock) error = %v", err)
	}

	spawner := newTestSessionSpawner(sessions, &fakeWorkspaceResolver{}, &cfg)
	err := spawner(context.Background(), "memory-consolidation", "prompt", "", prior)
	if err == nil {
		t.Fatal("spawner() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "no recent workspaces available") {
		t.Fatalf("spawner() error = %v, want no recent workspaces error", err)
	}
}

func TestSpawnSessionWrapsPromptAndStopErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should prompt error is wrapped", func(t *testing.T) {
		sessions := &fakeSessionManager{promptErr: errors.New("prompt failed")}
		err := spawnSession(
			context.Background(),
			sessions,
			SessionRoute{AgentName: "memory-agent"},
			"goal",
			"prompt",
			"ws-1",
			0,
		)
		if err == nil {
			t.Fatal("spawnSession() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "prompt dream session") {
			t.Fatalf("spawnSession() error = %v, want prompt context", err)
		}
	})

	t.Run("Should stop error is joined", func(t *testing.T) {
		stopErr := errors.New("stop failed")
		sessions := &fakeSessionManager{stopErr: stopErr}
		err := spawnSession(
			context.Background(),
			sessions,
			SessionRoute{AgentName: "memory-agent"},
			"goal",
			"prompt",
			"ws-1",
			0,
		)
		if !errors.Is(err, stopErr) {
			t.Fatalf("spawnSession() error = %v, want stop failure", err)
		}
	})

	t.Run("Should prompt event errors are surfaced", func(t *testing.T) {
		sessions := &fakeSessionManager{
			promptEvents: []acp.AgentEvent{{Type: acp.EventTypeError, Error: "tool failed"}},
		}
		err := spawnSession(
			context.Background(),
			sessions,
			SessionRoute{AgentName: "memory-agent"},
			"goal",
			"prompt",
			"ws-1",
			0,
		)
		if err == nil || !strings.Contains(err.Error(), "tool failed") {
			t.Fatalf("spawnSession() error = %v, want prompt event failure", err)
		}
	})

	t.Run("Should stop uses fresh context after caller cancellation", func(t *testing.T) {
		sessions := &fakeSessionManager{}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := spawnSession(
			ctx,
			sessions,
			SessionRoute{AgentName: "memory-agent"},
			"goal",
			"prompt",
			"ws-1",
			0,
		); err != nil {
			t.Fatalf("spawnSession() error = %v", err)
		}
		if got, want := sessions.lastStopContextErr(), error(nil); got != want {
			t.Fatalf("Stop() context err = %v, want nil", got)
		}
	})

	t.Run("Should use the first accepted fallback route", func(t *testing.T) {
		t.Parallel()

		sessions := &fakeSessionManager{createResults: []fakeCreateResult{
			{err: errors.New("primary rejected")},
			{accepted: true},
		}}
		events := 0
		err := spawnSession(
			t.Context(),
			sessions,
			SessionRoute{
				AgentName:       "memory-agent",
				Provider:        "primary",
				Model:           "m1",
				ReasoningEffort: "low",
				Fallbacks: []aghconfig.RoleFallback{
					{Provider: "secondary", Model: "m2", ReasoningEffort: "high"},
					{Provider: "tertiary", Model: "m3"},
				},
				BeforeFallback: func(_ context.Context, attempt int, fallback aghconfig.RoleFallback) error {
					events++
					if attempt != 1 || fallback.Provider != "secondary" {
						t.Fatalf("fallback event = %d/%#v, want first secondary route", attempt, fallback)
					}
					return nil
				},
			},
			"goal",
			"prompt",
			"ws-1",
			0,
		)
		if err != nil {
			t.Fatalf("spawnSession() error = %v", err)
		}
		calls := sessions.createdOptions()
		if len(calls) != 2 || calls[0].Provider != "primary" || calls[0].Model != "m1" ||
			calls[0].ReasoningEffort != "low" || calls[1].Provider != "secondary" ||
			calls[1].Model != "m2" || calls[1].ReasoningEffort != "high" {
			t.Fatalf("Create() calls = %#v", calls)
		}
		if events != 1 || sessions.stopCount() != 1 {
			t.Fatalf("fallback events/stops = %d/%d, want 1/1", events, sessions.stopCount())
		}
	})

	t.Run("Should not use fallback after session acceptance", func(t *testing.T) {
		t.Parallel()

		acceptedErr := errors.New("accepted startup failed")
		sessions := &fakeSessionManager{createResults: []fakeCreateResult{{accepted: true, err: acceptedErr}}}
		events := 0
		err := spawnSession(
			t.Context(),
			sessions,
			SessionRoute{
				AgentName: "memory-agent",
				Provider:  "primary",
				Fallbacks: []aghconfig.RoleFallback{{Provider: "secondary", Model: "m2"}},
				BeforeFallback: func(context.Context, int, aghconfig.RoleFallback) error {
					events++
					return nil
				},
			},
			"goal",
			"prompt",
			"ws-1",
			0,
		)
		if !errors.Is(err, acceptedErr) {
			t.Fatalf("spawnSession() error = %v, want accepted startup failure", err)
		}
		if sessions.createCount() != 1 || events != 0 || sessions.stopCount() != 1 {
			t.Fatalf(
				"creates/events/stops = %d/%d/%d, want 1/0/1",
				sessions.createCount(),
				events,
				sessions.stopCount(),
			)
		}
	})
}

func dreamConfig() aghconfig.Config {
	cfg := aghconfig.DefaultWithHome(aghconfig.HomePaths{})
	cfg.Memory.Enabled = true
	cfg.Roles.Dream.Enabled = true
	cfg.Roles.Dream.Agent = "memory-agent"
	cfg.Memory.Dream.CheckInterval = time.Minute
	return cfg
}

func newTestSessionSpawner(
	sessions SessionManager,
	resolver workspacepkg.RuntimeResolver,
	cfg *aghconfig.Config,
) memory.SessionSpawner {
	return NewSessionSpawner(sessions, resolver, cfg.Memory.Enabled, testDreamRouteResolver(cfg))
}

func testDreamRouteResolver(cfg *aghconfig.Config) SessionRouteResolver {
	return func(context.Context, string) (SessionRoute, error) {
		role := cfg.Roles.Dream
		return SessionRoute{
			Enabled:         role.Enabled,
			AgentName:       role.Agent,
			Provider:        role.Provider,
			Model:           role.Model,
			ReasoningEffort: role.ReasoningEffort,
			Fallbacks:       append([]aghconfig.RoleFallback(nil), role.FallbackChain...),
		}, nil
	}
}

func staticEnabled(enabled bool) func() bool {
	return func() bool { return enabled }
}

func newTestRuntime(enabled bool, service Service, interval time.Duration) *Runtime {
	return NewRuntime(staticEnabled(enabled), service, noopSpawner, interval, discardLogger(), nil)
}

func noopSpawner(context.Context, string, string, string, time.Time) error {
	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitForCondition(t *testing.T, label string, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", label)
}

type fakeDreamService struct {
	mu             sync.Mutex
	shouldRun      bool
	shouldRunErr   error
	runErr         error
	shouldRunCalls int
	runCalls       int
	workspaceRefs  []string
}

func (f *fakeDreamService) ShouldRun() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shouldRunCalls++
	return f.shouldRun, f.shouldRunErr
}

func (f *fakeDreamService) Run(_ context.Context, _ memory.SessionSpawner, workspace string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runCalls++
	f.workspaceRefs = append(f.workspaceRefs, workspace)
	return f.runErr
}

func (f *fakeDreamService) runCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runCalls
}

func (f *fakeDreamService) lastWorkspace() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.workspaceRefs) == 0 {
		return ""
	}
	return f.workspaceRefs[len(f.workspaceRefs)-1]
}

type fakeSessionManager struct {
	mu           sync.Mutex
	infos        []*session.Info
	promptErr    error
	promptEvents []acp.AgentEvent
	stopErr      error
	createCalls  []session.CreateOpts
	promptCalls  []struct {
		id  string
		msg string
	}
	stopCalls     []string
	stopCtxErr    []error
	createResults []fakeCreateResult
}

func (f *fakeSessionManager) Create(_ context.Context, opts session.CreateOpts) (*session.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls = append(f.createCalls, opts)
	sessionID := fmt.Sprintf("dream-%d", len(f.createCalls))
	result := fakeCreateResult{accepted: true}
	if index := len(f.createCalls) - 1; index < len(f.createResults) {
		result = f.createResults[index]
	}
	if !result.accepted {
		return nil, result.err
	}
	return &session.Session{
		ID:          sessionID,
		AgentName:   opts.AgentName,
		WorkspaceID: strings.TrimSpace(opts.Workspace),
		Workspace:   strings.TrimSpace(opts.Workspace),
		Type:        opts.Type,
		State:       session.StateActive,
	}, result.err
}

type fakeCreateResult struct {
	accepted bool
	err      error
}

func (f *fakeSessionManager) ListAll(context.Context) ([]*session.Info, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*session.Info(nil), f.infos...), nil
}

func (f *fakeSessionManager) Prompt(_ context.Context, id string, msg string) (<-chan acp.AgentEvent, error) {
	f.mu.Lock()
	f.promptCalls = append(f.promptCalls, struct {
		id  string
		msg string
	}{id: id, msg: msg})
	promptErr := f.promptErr
	f.mu.Unlock()
	if promptErr != nil {
		return nil, promptErr
	}

	ch := make(chan acp.AgentEvent, len(f.promptEvents))
	for _, event := range f.promptEvents {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func (f *fakeSessionManager) Stop(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls = append(f.stopCalls, id)
	if ctx != nil {
		f.stopCtxErr = append(f.stopCtxErr, ctx.Err())
	} else {
		f.stopCtxErr = append(f.stopCtxErr, context.Canceled)
	}
	return f.stopErr
}

func (f *fakeSessionManager) createCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.createCalls)
}

func (f *fakeSessionManager) createdOptions() []session.CreateOpts {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]session.CreateOpts(nil), f.createCalls...)
}

func (f *fakeSessionManager) createCall(index int) session.CreateOpts {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createCalls[index]
}

func (f *fakeSessionManager) promptCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.promptCalls)
}

func (f *fakeSessionManager) promptCall(index int) struct {
	id  string
	msg string
} {
	f.mu.Lock()
	defer f.mu.Unlock()
	if index < 0 || index >= len(f.promptCalls) {
		return struct {
			id  string
			msg string
		}{}
	}
	return f.promptCalls[index]
}

func (f *fakeSessionManager) stopCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.stopCalls)
}

func (f *fakeSessionManager) lastStopContextErr() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.stopCtxErr) == 0 {
		return nil
	}
	return f.stopCtxErr[len(f.stopCtxErr)-1]
}

func (f *fakeSessionManager) stopCall(index int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopCalls[index]
}

type fakeWorkspaceResolver struct {
	resolveResolved           workspacepkg.ResolvedWorkspace
	resolveOrRegisterResolved workspacepkg.ResolvedWorkspace
	resolveErr                error
	resolveOrRegisterErr      error
	lastResolveArg            string
	lastResolveOrRegisterArg  string
	resolveCalls              int
	resolveOrRegisterCalls    int
}

func (f *fakeWorkspaceResolver) Register(
	context.Context,
	workspacepkg.RegisterOptions,
) (workspacepkg.Workspace, error) {
	return workspacepkg.Workspace{}, errors.New("unexpected Register call")
}

func (f *fakeWorkspaceResolver) Unregister(context.Context, string) error {
	return errors.New("unexpected Unregister call")
}

func (f *fakeWorkspaceResolver) Update(context.Context, string, workspacepkg.UpdateOptions) error {
	return errors.New("unexpected Update call")
}

func (f *fakeWorkspaceResolver) List(context.Context) ([]workspacepkg.Workspace, error) {
	return nil, errors.New("unexpected List call")
}

func (f *fakeWorkspaceResolver) Get(context.Context, string) (workspacepkg.Workspace, error) {
	return workspacepkg.Workspace{}, errors.New("unexpected Get call")
}

func (f *fakeWorkspaceResolver) Resolve(
	_ context.Context,
	idOrNameOrPath string,
) (workspacepkg.ResolvedWorkspace, error) {
	f.resolveCalls++
	f.lastResolveArg = idOrNameOrPath
	if f.resolveErr != nil {
		return workspacepkg.ResolvedWorkspace{}, f.resolveErr
	}
	return f.resolveResolved, nil
}

func (f *fakeWorkspaceResolver) ResolveOrRegister(
	_ context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	f.resolveOrRegisterCalls++
	f.lastResolveOrRegisterArg = path
	if f.resolveOrRegisterErr != nil {
		return workspacepkg.ResolvedWorkspace{}, f.resolveOrRegisterErr
	}
	return f.resolveOrRegisterResolved, nil
}
