//go:build integration

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/admission"
	automationpkg "github.com/compozy/compozy/internal/automation"
	shellquote "github.com/kballard/go-shellquote"

	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/compozy/compozy/internal/gateway"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/memory/consolidation"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/session"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"

	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const daemonSessionStopHelperEnvKey = "COMPOZY_TEST_DAEMON_SESSION_STOP_HELPER"

func daemonMigrationStreams() []store.MigrationStream {
	return []store.MigrationStream{
		globaldb.MigrationStream(),
		memory.MigrationStream(),
	}
}

func installExtensionForDaemonIntegration(
	t *testing.T,
	databasePath string,
	name string,
	opts daemonTestExtensionOptions,
	enabled bool,
) string {
	t.Helper()

	db, err := openDaemonTestGlobalDBAtPath(testutil.Context(t), databasePath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(%q) error = %v", databasePath, err)
	}
	defer func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("GlobalDB.Close() error = %v", err)
		}
	}()

	return installDaemonTestExtension(t, db, name, opts, enabled)
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

func (f *fakeSessionManager) promptCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.promptCalls)
}

func TestBootSequenceReady(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	if d.sessions == nil || d.observer == nil || d.registry == nil {
		t.Fatalf(
			"boot() did not wire runtime dependencies: sessions=%v observer=%v registry=%v",
			d.sessions,
			d.observer,
			d.registry,
		)
	}
	if d.workspaceResolver == nil {
		t.Fatal("boot() did not wire the workspace resolver")
	}
	if _, err := os.Stat(homePaths.DatabaseFile); err != nil {
		t.Fatalf("stat global database error = %v", err)
	}
	if _, err := os.Stat(homePaths.DaemonInfo); err != nil {
		t.Fatalf("stat daemon.json error = %v", err)
	}
	if _, err := AcquireLock(homePaths.DaemonLock, os.Getpid()); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("AcquireLock(second instance) error = %v, want ErrAlreadyRunning", err)
	}
}

func TestBootGatewayRefusalContinuesLocalOnly(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Gateway.Enabled = true
	registry := &recordingRegistry{
		path: homePaths.DatabaseFile,
		gatewaySnapshot: gateway.Snapshot{
			Providers: []gateway.ProviderActivation{{
				ProviderName: "connectivity-test", Tier: gateway.TierPrivate,
				Desired: gateway.DesiredEnabled, Observed: gateway.ProviderEstablishing, Generation: 1,
			}},
			Surfaces: []gateway.SurfaceExposure{{
				Surface: gateway.SurfaceOperatorUI, Tier: gateway.TierPrivate,
				Desired: gateway.DesiredEnabled, Observed: gateway.SurfaceOff, Generation: 1,
			}},
		},
	}
	d := newTestDaemon(t, homePaths, &cfg)
	d.openRegistry = func(context.Context, string) (Registry, error) { return registry, nil }
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) { return &fakeObserver{}, nil }
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}
	tierFactoryCalls := 0
	d.gatewayTierFactory = func(
		context.Context,
		*RuntimeDeps,
		gateway.Tier,
		[]gateway.Surface,
	) (Server, error) {
		tierFactoryCalls++
		return &fakeServer{name: "gateway-tier"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v, want local-only continuation [IT-002]", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
	status, err := d.gateway.Status(testutil.Context(t))
	if err != nil {
		t.Fatalf("gateway.Status() error = %v", err)
	}
	if status.Refusal == nil || !strings.Contains(status.Refusal.Cause, "authentication model is inactive") ||
		!strings.Contains(status.Refusal.Fix, "pairing and device authentication") {
		t.Fatalf("gateway refusal = %#v, want authentication cause and fix", status.Refusal)
	}
	if len(status.Addresses) != 0 || status.Tiers[0].Advertised ||
		status.Providers[0].Observed != gateway.ProviderDown {
		t.Fatalf("gateway status = %#v, want local-only and provider down", status)
	}
	if d.httpServer == nil || d.udsServer == nil {
		t.Fatal("local daemon servers did not start after gateway refusal")
	}
	if tierFactoryCalls != 0 {
		t.Fatalf("gateway tier factory calls = %d, want 0 while authentication is inactive [IT-001]", tierFactoryCalls)
	}
}

func TestBootGatewayReconcilesDisabledSurfaceBeforeServers(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Gateway.Enabled = true
	registry := &recordingRegistry{
		path: homePaths.DatabaseFile,
		gatewaySnapshot: gateway.Snapshot{
			Providers: []gateway.ProviderActivation{{
				ProviderName: "connectivity-test", Tier: gateway.TierPrivate,
				Desired: gateway.DesiredEnabled, Observed: gateway.ProviderEstablishing, Generation: 7,
			}},
			Surfaces: []gateway.SurfaceExposure{{
				Surface: gateway.SurfaceOperatorUI, Tier: gateway.TierPrivate,
				Desired: gateway.DesiredDisabled, Observed: gateway.SurfaceOff, Generation: 8,
			}},
		},
	}
	d := newTestDaemon(t, homePaths, &cfg)
	d.openRegistry = func(context.Context, string) (Registry, error) { return registry, nil }
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) { return &fakeObserver{}, nil }
	serverObservedReconciled := false
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		snapshot, err := registry.Snapshot(testutil.Context(t))
		if err != nil {
			t.Fatalf("Snapshot(before HTTP server) error = %v", err)
		}
		serverObservedReconciled = len(snapshot.Providers) == 1 &&
			snapshot.Providers[0].Observed == gateway.ProviderDown &&
			len(snapshot.Surfaces) == 1 && snapshot.Surfaces[0].Desired == gateway.DesiredDisabled &&
			snapshot.Surfaces[0].Observed == gateway.SurfaceOff
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v [IT-005]", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
	if !serverObservedReconciled {
		t.Fatal("HTTP server construction preceded gateway reconciliation")
	}
	if _, err := d.gateway.Reconcile(testutil.Context(t)); err != nil {
		t.Fatalf("gateway.Reconcile(restart replay) error = %v", err)
	}
	status, err := d.gateway.Status(testutil.Context(t))
	if err != nil {
		t.Fatalf("gateway.Status() error = %v", err)
	}
	if status.Surfaces[0].Desired != gateway.DesiredDisabled ||
		status.Surfaces[0].Observed != gateway.SurfaceOff || status.Tiers[0].Advertised || len(status.Addresses) != 0 {
		t.Fatalf("gateway status after replay = %#v, want disabled surface to remain unreachable", status)
	}
}

func TestBootWiresTaskRuntimeWithDedicatedSessionBridge(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	sessions := &fakeSessionManager{}

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return sessions, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	if d.tasks == nil || d.tasks.manager == nil {
		t.Fatal("boot() did not publish the task runtime")
	}

	workspaceRoot := filepath.Join(t.TempDir(), "task-runtime-workspace")
	resolved := resolveDaemonWorkspace(t, d.workspaceResolver, workspaceRoot)
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "compozy task run")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	taskRecord, err := d.tasks.manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
		ProfileID:   store.DefaultProfileID,
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: resolved.ID,
		Title:       "Bridge task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	run, err := d.tasks.manager.EnqueueRun(testutil.Context(t), taskpkg.EnqueueRun{TaskID: taskRecord.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	run, err = d.tasks.manager.StartRun(testutil.Context(t), run.ID, taskpkg.StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}

	if got, want := sessions.createCount(), 1; got != want {
		t.Fatalf("createCount() = %d, want %d", got, want)
	}
	createCall := sessions.createCall(0)
	if got, want := createCall.Type, session.SessionTypeSystem; got != want {
		t.Fatalf("createCall.Type = %q, want %q", got, want)
	}
	if got := createCall.Provider; got != "" {
		t.Fatalf("createCall.Provider = %q, want explicit empty provider", got)
	}
	if got, want := createCall.Workspace, resolved.ID; got != want {
		t.Fatalf("createCall.Workspace = %q, want %q", got, want)
	}

	storedRun, err := d.tasks.store.GetTaskRun(testutil.Context(t), run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun() error = %v", err)
	}
	if got, want := storedRun.Status, taskpkg.TaskRunStatusRunning; got != want {
		t.Fatalf("storedRun.Status = %q, want %q", got, want)
	}
	if strings.TrimSpace(storedRun.SessionID) == "" {
		t.Fatal("storedRun.SessionID = empty, want dedicated session id")
	}
}

func TestDetachedHarnessIntegration(t *testing.T) {
	testCases := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "ShouldWireDetachedHarnessTaskRuntimeAcrossScopes",
			run:  testBootWiresDetachedHarnessTaskRuntimeAcrossScopes,
		},
		{
			name: "ShouldEmitSyntheticReentryAfterDetachedHarnessCompletionEndToEnd",
			run:  testDetachedHarnessCompletionWakeEmitsSyntheticReentryEndToEnd,
		},
		{
			name: "ShouldRecordSilentDropWhenPolicySuppressesDetachedHarnessWakeEndToEnd",
			run:  testDetachedHarnessCompletionSilentPolicyRecordsDropEndToEnd,
		},
		{
			name: "ShouldPreserveDetachedHarnessWakeFIFOAcrossRuns",
			run:  testDetachedHarnessCompletionWakePreservesFIFOAcrossRuns,
		},
		{
			name: "ShouldReusePersistedSyntheticEventDuringDetachedHarnessRecoveryDedupe",
			run:  testBootRecoveryDetachedHarnessWakeUsesPersistedSyntheticEventForDedupe,
		},
		{
			name: "ShouldRecoverDetachedHarnessRunThroughTaskRuntimeRulesOnBoot",
			run:  testBootRecoversDetachedHarnessRunThroughTaskRuntimeRules,
		},
	}

	for _, tt := range testCases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func testBootWiresDetachedHarnessTaskRuntimeAcrossScopes(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	sessions := &fakeSessionManager{}
	daemonInstance := bootDetachedHarnessIntegrationDaemon(t, homePaths, &cfg, sessions)
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	if daemonInstance.tasks == nil || daemonInstance.tasks.detached == nil {
		t.Fatal("boot() did not wire the detached harness bridge")
	}

	workspace := resolveDaemonWorkspace(t, daemonInstance.workspaceResolver, filepath.Join(t.TempDir(), "workspace"))
	sessions.infos = []*session.Info{
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-owner-workspace",
			Type:        session.SessionTypeSystem,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-wake-workspace",
			Type:        session.SessionTypeSystem,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
		{
			ProfileID: store.DefaultProfileID,
			ID:        "sess-owner-global",
			Type:      session.SessionTypeSystem,
			State:     session.StateActive,
		},
		{
			ProfileID: store.DefaultProfileID,
			ID:        "sess-wake-global",
			Type:      session.SessionTypeSystem,
			State:     session.StateActive,
		},
	}

	workspaceSubmission, err := daemonInstance.tasks.submitDetachedHarnessWork(
		testutil.Context(t),
		detachedHarnessSubmitRequest{
			SubmissionKey:  "detached-integration-workspace",
			OwnerSessionID: "sess-owner-workspace",
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    workspace.ID,
			Summary:        "Workspace detached work",
			TurnSource:     session.TurnSourceUser,
			WakeTarget: detachedHarnessWakeTargetInput{
				SessionID: "sess-wake-workspace",
			},
		},
	)
	if err != nil {
		t.Fatalf("submitDetachedHarnessWork(workspace) error = %v", err)
	}
	globalSubmission, err := daemonInstance.tasks.submitDetachedHarnessWork(
		testutil.Context(t),
		detachedHarnessSubmitRequest{
			SubmissionKey:  "detached-integration-global",
			OwnerSessionID: "sess-owner-global",
			Scope:          taskpkg.ScopeGlobal,
			Summary:        "Global detached work",
			TurnSource:     session.TurnSourceSynthetic,
			WakeTarget: detachedHarnessWakeTargetInput{
				SessionID: "sess-wake-global",
			},
		},
	)
	if err != nil {
		t.Fatalf("submitDetachedHarnessWork(global) error = %v", err)
	}
	duplicateWorkspace, err := daemonInstance.tasks.submitDetachedHarnessWork(
		testutil.Context(t),
		detachedHarnessSubmitRequest{
			SubmissionKey:  "detached-integration-workspace",
			OwnerSessionID: "sess-owner-workspace",
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    workspace.ID,
			Summary:        "Workspace detached work",
			TurnSource:     session.TurnSourceUser,
			WakeTarget: detachedHarnessWakeTargetInput{
				SessionID: "sess-wake-workspace",
			},
		},
	)
	if err != nil {
		t.Fatalf("submitDetachedHarnessWork(workspace duplicate) error = %v", err)
	}
	if !duplicateWorkspace.ExistingTask || !duplicateWorkspace.ExistingRun {
		t.Fatalf(
			"duplicate detached submission flags = task:%v run:%v, want both true",
			duplicateWorkspace.ExistingTask,
			duplicateWorkspace.ExistingRun,
		)
	}
	if got, want := duplicateWorkspace.Run.ID, workspaceSubmission.Run.ID; got != want {
		t.Fatalf("duplicate workspace run id = %q, want %q", got, want)
	}

	readActor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "compozy task inspect")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	workspaceView, err := daemonInstance.tasks.manager.GetTask(
		testutil.Context(t),
		workspaceSubmission.Task.ID,
		readActor,
	)
	if err != nil {
		t.Fatalf("manager.GetTask(workspace) error = %v", err)
	}
	if got, want := workspaceView.Task.Scope, taskpkg.ScopeWorkspace; got != want {
		t.Fatalf("workspaceView.Task.Scope = %q, want %q", got, want)
	}
	if got, want := workspaceView.Task.WorkspaceID, workspace.ID; got != want {
		t.Fatalf("workspaceView.Task.WorkspaceID = %q, want %q", got, want)
	}
	workspaceRuns, err := daemonInstance.tasks.manager.ListTaskRuns(
		testutil.Context(t),
		workspaceSubmission.Task.ID,
		taskpkg.RunQuery{},
		readActor,
	)
	if err != nil {
		t.Fatalf("manager.ListTaskRuns(workspace) error = %v", err)
	}
	if got, want := len(workspaceRuns), 1; got != want {
		t.Fatalf("len(workspaceRuns) = %d, want %d", got, want)
	}

	globalView, err := daemonInstance.tasks.manager.GetTask(testutil.Context(t), globalSubmission.Task.ID, readActor)
	if err != nil {
		t.Fatalf("manager.GetTask(global) error = %v", err)
	}
	if got, want := globalView.Task.Scope, taskpkg.ScopeGlobal; got != want {
		t.Fatalf("globalView.Task.Scope = %q, want %q", got, want)
	}
	if got := globalView.Task.WorkspaceID; got != "" {
		t.Fatalf("globalView.Task.WorkspaceID = %q, want empty", got)
	}
	globalRuns, err := daemonInstance.tasks.manager.ListTaskRuns(
		testutil.Context(t),
		globalSubmission.Task.ID,
		taskpkg.RunQuery{},
		readActor,
	)
	if err != nil {
		t.Fatalf("manager.ListTaskRuns(global) error = %v", err)
	}
	if got, want := len(globalRuns), 1; got != want {
		t.Fatalf("len(globalRuns) = %d, want %d", got, want)
	}

	workspaceRunMetadata, err := decodeDetachedHarnessRunMetadata(workspaceRuns[0].Metadata)
	if err != nil {
		t.Fatalf("decodeDetachedHarnessRunMetadata(workspace) error = %v", err)
	}
	if got, want := workspaceRunMetadata.OwnerSessionID, "sess-owner-workspace"; got != want {
		t.Fatalf("workspace run metadata owner session id = %q, want %q", got, want)
	}
	if got, want := workspaceRunMetadata.WakeTarget.SessionID, "sess-wake-workspace"; got != want {
		t.Fatalf("workspace run metadata wake target = %q, want %q", got, want)
	}

	globalRunMetadata, err := decodeDetachedHarnessRunMetadata(globalRuns[0].Metadata)
	if err != nil {
		t.Fatalf("decodeDetachedHarnessRunMetadata(global) error = %v", err)
	}
	if got, want := globalRunMetadata.OwnerSessionID, "sess-owner-global"; got != want {
		t.Fatalf("global run metadata owner session id = %q, want %q", got, want)
	}
	if got, want := globalRunMetadata.WakeTarget.SessionID, "sess-wake-global"; got != want {
		t.Fatalf("global run metadata wake target = %q, want %q", got, want)
	}
}

func testDetachedHarnessCompletionWakeEmitsSyntheticReentryEndToEnd(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	sessions := &fakeSessionManager{}
	daemonInstance := bootDetachedHarnessIntegrationDaemon(t, homePaths, &cfg, sessions)
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	workspace := resolveDaemonWorkspace(t, daemonInstance.workspaceResolver, filepath.Join(t.TempDir(), "workspace"))
	sessions.infos = []*session.Info{
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-owner",
			AgentName:   "coder",
			Type:        session.SessionTypeSystem,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-wake",
			AgentName:   "coder",
			Type:        session.SessionTypeSystem,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
	}
	seedDetachedHarnessSessionIndex(t, homePaths, sessions.infos)

	submission := submitDetachedHarnessWorkForTest(t, daemonInstance.tasks, detachedHarnessSubmitRequest{
		SubmissionKey:  "integration-reentry-live",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "Live detached completion",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})

	completeDetachedHarnessRunForTest(t, daemonInstance.tasks, submission.Run.ID, "sess-owner")
	metadata := waitForDetachedHarnessReentryState(
		t,
		daemonInstance.tasks,
		submission.Run.ID,
		harnessReentryOutcomeEmitted,
	)
	if got, want := metadata.Reentry.Reason, harnessReentryReasonCompleted; got != want {
		t.Fatalf("metadata.Reentry.Reason = %q, want %q", got, want)
	}

	waitForTaskRuntimeCondition(t, 2*time.Second, func() bool {
		return sessions.syntheticPromptCount() == 1
	})
	types := waitForEventSummaryTypes(
		t,
		daemonInstance.tasks,
		"sess-wake",
		harnessSummaryDetachedCompleted,
		harnessSummaryContextResolved,
		harnessSummarySyntheticReentryEmitted,
	)
	wantTypes := []string{
		harnessSummaryContextResolved,
		harnessSummaryDetachedCompleted,
		harnessSummarySyntheticReentryEmitted,
	}
	if !slices.Equal(types, wantTypes) {
		t.Fatalf("event summary types = %#v, want %#v", types, wantTypes)
	}

	sessions.mu.Lock()
	if got, want := len(sessions.syntheticPromptCalls), 1; got != want {
		sessions.mu.Unlock()
		t.Fatalf("len(syntheticPromptCalls) = %d, want %d", got, want)
	}
	call := sessions.syntheticPromptCalls[0]
	events := append([]store.SessionEvent(nil), sessions.sessionEvents["sess-wake"]...)
	sessions.mu.Unlock()

	if got, want := call.id, "sess-wake"; got != want {
		t.Fatalf("synthetic prompt target = %q, want %q", got, want)
	}
	if got, want := call.opts.Metadata.TaskID, submission.Task.ID; got != want {
		t.Fatalf("synthetic prompt task id = %q, want %q", got, want)
	}
	if got, want := call.opts.Metadata.TaskRunID, submission.Run.ID; got != want {
		t.Fatalf("synthetic prompt run id = %q, want %q", got, want)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("len(synthetic session events) = %d, want %d", got, want)
	}
	if got, want := events[0].Type, acp.EventTypeSyntheticReentry; got != want {
		t.Fatalf("session event type = %q, want %q", got, want)
	}

	var payload struct {
		Synthetic *acp.PromptSyntheticMeta `json:"synthetic,omitempty"`
	}
	if err := json.Unmarshal([]byte(events[0].Content), &payload); err != nil {
		t.Fatalf("json.Unmarshal(session event) error = %v", err)
	}
	if payload.Synthetic == nil {
		t.Fatal("session event synthetic payload = nil, want metadata")
	}
	if got, want := payload.Synthetic.TaskRunID, submission.Run.ID; got != want {
		t.Fatalf("session event synthetic run id = %q, want %q", got, want)
	}
}

func testDetachedHarnessCompletionSilentPolicyRecordsDropEndToEnd(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	sessions := &fakeSessionManager{}
	daemonInstance := bootDetachedHarnessIntegrationDaemon(t, homePaths, &cfg, sessions)
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	workspace := resolveDaemonWorkspace(t, daemonInstance.workspaceResolver, filepath.Join(t.TempDir(), "workspace"))
	sessions.infos = []*session.Info{
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-owner",
			AgentName:   "coder",
			Type:        session.SessionTypeSystem,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-wake",
			AgentName:   "coder",
			Type:        session.SessionTypeUser,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
	}
	seedDetachedHarnessSessionIndex(t, homePaths, sessions.infos)

	submission := submitDetachedHarnessWorkForTest(t, daemonInstance.tasks, detachedHarnessSubmitRequest{
		SubmissionKey:  "integration-reentry-silent",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "Silent detached completion",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})

	completeDetachedHarnessRunForTest(t, daemonInstance.tasks, submission.Run.ID, "sess-owner")
	metadata := waitForDetachedHarnessReentryState(
		t,
		daemonInstance.tasks,
		submission.Run.ID,
		harnessReentryOutcomeSilent,
	)
	if got, want := metadata.Reentry.Reason, harnessReentryReasonPolicySilent; got != want {
		t.Fatalf("metadata.Reentry.Reason = %q, want %q", got, want)
	}
	if got := sessions.syntheticPromptCount(); got != 0 {
		t.Fatalf("synthetic prompt count = %d, want 0 for silent completion", got)
	}

	types := waitForEventSummaryTypes(
		t,
		daemonInstance.tasks,
		"sess-wake",
		harnessSummaryDetachedCompleted,
		harnessSummarySyntheticReentryDropped,
	)
	wantTypes := []string{
		harnessSummaryDetachedCompleted,
		harnessSummarySyntheticReentryDropped,
	}
	if !slices.Equal(types, wantTypes) {
		t.Fatalf("event summary types = %#v, want %#v", types, wantTypes)
	}

	sessions.mu.Lock()
	events := append([]store.SessionEvent(nil), sessions.sessionEvents["sess-wake"]...)
	sessions.mu.Unlock()

	if got := len(events); got != 0 {
		t.Fatalf("len(synthetic session events) = %d, want 0 for silent completion", got)
	}
}

func testDetachedHarnessCompletionWakePreservesFIFOAcrossRuns(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	sessions := &fakeSessionManager{}
	daemonInstance := bootDetachedHarnessIntegrationDaemon(t, homePaths, &cfg, sessions)
	t.Cleanup(func() {
		if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	workspace := resolveDaemonWorkspace(t, daemonInstance.workspaceResolver, filepath.Join(t.TempDir(), "workspace"))
	sessions.infos = []*session.Info{
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-owner",
			AgentName:   "coder",
			Type:        session.SessionTypeSystem,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-wake",
			AgentName:   "coder",
			Type:        session.SessionTypeSystem,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
	}
	seedDetachedHarnessSessionIndex(t, homePaths, sessions.infos)

	first := submitDetachedHarnessWorkForTest(t, daemonInstance.tasks, detachedHarnessSubmitRequest{
		SubmissionKey:  "integration-reentry-fifo-1",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "First FIFO completion",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})
	second := submitDetachedHarnessWorkForTest(t, daemonInstance.tasks, detachedHarnessSubmitRequest{
		SubmissionKey:  "integration-reentry-fifo-2",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "Second FIFO completion",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})

	completeDetachedHarnessRunForTest(t, daemonInstance.tasks, first.Run.ID, "sess-owner")
	waitForTaskRuntimeCondition(t, 2*time.Second, func() bool {
		return sessions.syntheticPromptCount() == 1
	})
	completeDetachedHarnessRunForTest(t, daemonInstance.tasks, second.Run.ID, "sess-owner")

	waitForDetachedHarnessReentryState(t, daemonInstance.tasks, first.Run.ID, harnessReentryOutcomeEmitted)
	waitForDetachedHarnessReentryState(t, daemonInstance.tasks, second.Run.ID, harnessReentryOutcomeEmitted)
	waitForTaskRuntimeCondition(t, 2*time.Second, func() bool {
		return sessions.syntheticPromptCount() == 2
	})

	sessions.mu.Lock()
	calls := append([]fakeSyntheticPromptCall(nil), sessions.syntheticPromptCalls...)
	sessions.mu.Unlock()

	if got, want := len(calls), 2; got != want {
		t.Fatalf("len(syntheticPromptCalls) = %d, want %d", got, want)
	}
	if got, want := calls[0].opts.Metadata.TaskRunID, first.Run.ID; got != want {
		t.Fatalf("first synthetic wake run id = %q, want %q", got, want)
	}
	if got, want := calls[1].opts.Metadata.TaskRunID, second.Run.ID; got != want {
		t.Fatalf("second synthetic wake run id = %q, want %q", got, want)
	}
}

func testBootRecoveryDetachedHarnessWakeUsesPersistedSyntheticEventForDedupe(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	sessionsOne := &fakeSessionManager{}
	firstDaemon := bootDetachedHarnessIntegrationDaemon(t, homePaths, &cfg, sessionsOne)
	workspace := resolveDaemonWorkspace(t, firstDaemon.workspaceResolver, filepath.Join(t.TempDir(), "workspace"))
	sessionsOne.infos = []*session.Info{
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-owner",
			AgentName:   "coder",
			Type:        session.SessionTypeSystem,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-wake",
			AgentName:   "coder",
			Type:        session.SessionTypeSystem,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
	}
	seedDetachedHarnessSessionIndex(t, homePaths, sessionsOne.infos)

	submission := submitDetachedHarnessWorkForTest(t, firstDaemon.tasks, detachedHarnessSubmitRequest{
		SubmissionKey:  "integration-reentry-recovery",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "Recovery dedupe completion",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})

	completeDetachedHarnessRunForTest(t, firstDaemon.tasks, submission.Run.ID, "sess-owner")
	waitForDetachedHarnessReentryState(t, firstDaemon.tasks, submission.Run.ID, harnessReentryOutcomeEmitted)
	waitForTaskRuntimeCondition(t, 2*time.Second, func() bool {
		return sessionsOne.syntheticPromptCount() == 1
	})

	run, err := firstDaemon.tasks.store.GetTaskRun(testutil.Context(t), submission.Run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun() error = %v", err)
	}
	runMetadata, ok, err := maybeDecodeDetachedHarnessRunMetadata(run.Metadata)
	if err != nil {
		t.Fatalf("maybeDecodeDetachedHarnessRunMetadata() error = %v", err)
	}
	if !ok {
		t.Fatal("task run metadata = non-detached, want detached harness metadata")
	}
	previousMetadata := append(json.RawMessage(nil), run.Metadata...)
	runMetadata.Reentry = nil
	run.Metadata, err = marshalDetachedHarnessMetadata(runMetadata)
	if err != nil {
		t.Fatalf("marshalDetachedHarnessMetadata() error = %v", err)
	}
	if _, err := firstDaemon.tasks.store.UpdateTaskRunMetadata(
		testutil.Context(t),
		taskpkg.RunMetadataMutation{
			RunID: run.ID, ExpectedMetadata: previousMetadata, Metadata: run.Metadata,
		},
	); err != nil {
		t.Fatalf("UpdateTaskRunMetadata() error = %v", err)
	}

	sessionsOne.mu.Lock()
	recoveredEvents := cloneFakeSessionEvents(sessionsOne.sessionEvents)
	nextSequence := sessionsOne.nextEventSequence
	sessionsOne.mu.Unlock()

	if err := firstDaemon.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown(first daemon) error = %v", err)
	}

	sessionsTwo := &fakeSessionManager{
		infos: []*session.Info{
			{
				ProfileID:   store.DefaultProfileID,
				ID:          "sess-owner",
				AgentName:   "coder",
				Type:        session.SessionTypeSystem,
				State:       session.StateActive,
				WorkspaceID: workspace.ID,
				Workspace:   workspace.RootDir,
			},
			{
				ProfileID:   store.DefaultProfileID,
				ID:          "sess-wake",
				AgentName:   "coder",
				Type:        session.SessionTypeSystem,
				State:       session.StateActive,
				WorkspaceID: workspace.ID,
				Workspace:   workspace.RootDir,
			},
		},
		sessionEvents:     recoveredEvents,
		nextEventSequence: nextSequence,
	}
	secondDaemon := bootDetachedHarnessIntegrationDaemon(t, homePaths, &cfg, sessionsTwo)
	t.Cleanup(func() {
		if err := secondDaemon.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown(second daemon) error = %v", err)
		}
	})

	metadata := waitForDetachedHarnessReentryState(
		t,
		secondDaemon.tasks,
		submission.Run.ID,
		harnessReentryOutcomeEmitted,
	)
	if got, want := metadata.Reentry.Reason, harnessReentryReasonAlreadyRecorded; got != want {
		t.Fatalf("metadata.Reentry.Reason = %q, want %q", got, want)
	}
	if got := sessionsTwo.syntheticPromptCount(); got != 0 {
		t.Fatalf("synthetic prompt count after recovery = %d, want 0", got)
	}

	sessionsTwo.mu.Lock()
	events := append([]store.SessionEvent(nil), sessionsTwo.sessionEvents["sess-wake"]...)
	sessionsTwo.mu.Unlock()
	if got, want := len(events), 1; got != want {
		t.Fatalf("len(recovered synthetic session events) = %d, want %d", got, want)
	}
}

func testBootRecoversDetachedHarnessRunThroughTaskRuntimeRules(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	sessionsOne := &fakeSessionManager{}
	firstDaemon := bootDetachedHarnessIntegrationDaemon(t, homePaths, &cfg, sessionsOne)
	workspace := resolveDaemonWorkspace(t, firstDaemon.workspaceResolver, filepath.Join(t.TempDir(), "workspace"))
	sessionsOne.infos = []*session.Info{
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-owner",
			Type:        session.SessionTypeSystem,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-wake",
			Type:        session.SessionTypeSystem,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
		{
			ProfileID:   store.DefaultProfileID,
			ID:          "sess-runtime",
			Type:        session.SessionTypeSystem,
			State:       session.StateActive,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.RootDir,
		},
	}

	submission, err := firstDaemon.tasks.submitDetachedHarnessWork(testutil.Context(t), detachedHarnessSubmitRequest{
		SubmissionKey:  "detached-boot-recovery",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "Recover detached work on next boot",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})
	if err != nil {
		t.Fatalf("submitDetachedHarnessWork() error = %v", err)
	}

	detachedActor, err := detachedHarnessActorContext("sess-owner")
	if err != nil {
		t.Fatalf("detachedHarnessActorContext() error = %v", err)
	}
	starting, err := firstDaemon.tasks.manager.AttachRunSession(
		testutil.Context(t),
		submission.Run.ID,
		"sess-runtime",
		detachedActor,
	)
	if err != nil {
		t.Fatalf("AttachRunSession() error = %v", err)
	}
	if got, want := starting.Status, taskpkg.TaskRunStatusStarting; got != want {
		t.Fatalf("starting.Status = %q, want %q", got, want)
	}

	if err := firstDaemon.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown(first daemon) error = %v", err)
	}

	sessionsTwo := &fakeSessionManager{
		infos: []*session.Info{
			{
				ProfileID:   store.DefaultProfileID,
				ID:          "sess-runtime",
				Type:        session.SessionTypeSystem,
				State:       session.StateActive,
				WorkspaceID: workspace.ID,
				Workspace:   workspace.RootDir,
			},
		},
	}
	secondDaemon := bootDetachedHarnessIntegrationDaemon(t, homePaths, &cfg, sessionsTwo)
	t.Cleanup(func() {
		if err := secondDaemon.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown(second daemon) error = %v", err)
		}
	})

	recoveredRun, err := secondDaemon.tasks.store.GetTaskRun(testutil.Context(t), submission.Run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(recovered) error = %v", err)
	}
	if got, want := recoveredRun.Status, taskpkg.TaskRunStatusRunning; got != want {
		t.Fatalf("recoveredRun.Status = %q, want %q", got, want)
	}
	recoveredMetadata, err := decodeDetachedHarnessRunMetadata(recoveredRun.Metadata)
	if err != nil {
		t.Fatalf("decodeDetachedHarnessRunMetadata(recovered) error = %v", err)
	}
	if got, want := recoveredMetadata.SubmissionKey, "detached-boot-recovery"; got != want {
		t.Fatalf("recovered metadata submission key = %q, want %q", got, want)
	}
	if got, want := recoveredMetadata.WakeTarget.SessionID, "sess-wake"; got != want {
		t.Fatalf("recovered metadata wake target = %q, want %q", got, want)
	}
}

func TestBootRecoversOrphanedTaskRunsAndRecordsAudit(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	seedDB, err := openDaemonTestGlobalDBAtPath(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB(seed) error = %v", err)
	}

	seedManager, err := taskpkg.NewManager(taskpkg.WithStore(seedDB))
	if err != nil {
		t.Fatalf("task.NewManager(seed) error = %v", err)
	}
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "compozy task seed")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	createTask := func(title string) taskpkg.Task {
		taskRecord, err := seedManager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			ProfileID: store.DefaultProfileID,
			Scope:     taskpkg.ScopeGlobal,
			Title:     title,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(%q) error = %v", title, err)
		}
		return *taskRecord
	}

	claimedTask := createTask("Claimed run")
	startingTask := createTask("Starting run")
	runningTask := createTask("Running run")

	now := time.Date(2026, 4, 14, 19, 0, 0, 0, time.UTC)
	for _, run := range []taskpkg.Run{
		{
			ID:      "run-claimed",
			TaskID:  claimedTask.ID,
			Status:  taskpkg.TaskRunStatusClaimed,
			Attempt: 1,
			Origin:  taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "compozy task seed"},

			QueuedAt:  now,
			ClaimedAt: now.Add(30 * time.Second),
		},
		{
			ID:        "run-starting",
			TaskID:    startingTask.ID,
			Status:    taskpkg.TaskRunStatusStarting,
			Attempt:   1,
			SessionID: "sess-stopped",
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "compozy task seed"},

			QueuedAt:  now,
			StartedAt: now.Add(time.Minute),
		},
		{
			ID:        "run-running",
			TaskID:    runningTask.ID,
			Status:    taskpkg.TaskRunStatusRunning,
			Attempt:   1,
			SessionID: "sess-missing",
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "compozy task seed"},

			QueuedAt:  now,
			StartedAt: now.Add(2 * time.Minute),
		},
	} {
		seedDaemonTaskRunLifecycle(t, seedDB, run)
	}

	if err := seedDB.Close(testutil.Context(t)); err != nil {
		t.Fatalf("seedDB.Close() error = %v", err)
	}

	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{
				ProfileID: store.DefaultProfileID, ID: "sess-stopped", State: session.StateStopped},
		},
	}

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return sessions, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	claimedRun, err := d.tasks.store.GetTaskRun(testutil.Context(t), "run-claimed")
	if err != nil {
		t.Fatalf("GetTaskRun(run-claimed) error = %v", err)
	}
	if got, want := claimedRun.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("claimedRun.Status = %q, want %q", got, want)
	}

	startingRun, err := d.tasks.store.GetTaskRun(testutil.Context(t), "run-starting")
	if err != nil {
		t.Fatalf("GetTaskRun(run-starting) error = %v", err)
	}
	if got, want := startingRun.Status, taskpkg.TaskRunStatusFailed; got != want {
		t.Fatalf("startingRun.Status = %q, want %q", got, want)
	}

	runningRun, err := d.tasks.store.GetTaskRun(testutil.Context(t), "run-running")
	if err != nil {
		t.Fatalf("GetTaskRun(run-running) error = %v", err)
	}
	if got, want := runningRun.Status, taskpkg.TaskRunStatusFailed; got != want {
		t.Fatalf("runningRun.Status = %q, want %q", got, want)
	}

	claimedEvents, err := d.tasks.store.ListTaskEvents(testutil.Context(t), taskpkg.EventQuery{TaskID: claimedTask.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents(claimed) error = %v", err)
	}
	if !containsTaskEventType(claimedEvents, "task.run_recovered") {
		t.Fatalf("claimed task events = %#v, want task.run_recovered", taskEventTypes(claimedEvents))
	}

	startingEvents, err := d.tasks.store.ListTaskEvents(
		testutil.Context(t),
		taskpkg.EventQuery{TaskID: startingTask.ID},
	)
	if err != nil {
		t.Fatalf("ListTaskEvents(starting) error = %v", err)
	}
	if !containsTaskEventType(startingEvents, "task.run_failed") ||
		!containsTaskEventType(startingEvents, "task.run_recovered") {
		t.Fatalf(
			"starting task events = %#v, want task.run_failed + task.run_recovered",
			taskEventTypes(startingEvents),
		)
	}
}

func TestBootPublishesRunningAutomationBeforeServersStart(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Automation.Enabled = true

	var httpSawRunning bool
	var udsSawRunning bool

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(ctx context.Context, deps RuntimeDeps) (Server, error) {
		if deps.Automation == nil {
			t.Fatal("http factory received nil automation manager")
		}
		status, err := deps.Automation.Status(ctx)
		if err != nil {
			t.Fatalf("deps.Automation.Status(http) error = %v", err)
		}
		if !status.Running || !status.SchedulerRunning {
			t.Fatalf("http factory automation status = %#v, want running scheduler", status)
		}
		httpSawRunning = true
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(ctx context.Context, deps RuntimeDeps) (Server, error) {
		if deps.Automation == nil {
			t.Fatal("uds factory received nil automation manager")
		}
		status, err := deps.Automation.Status(ctx)
		if err != nil {
			t.Fatalf("deps.Automation.Status(uds) error = %v", err)
		}
		if !status.Running || !status.SchedulerRunning {
			t.Fatalf("uds factory automation status = %#v, want running scheduler", status)
		}
		udsSawRunning = true
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	if d.automation == nil {
		t.Fatal("boot() did not publish the automation manager")
	}
	if !httpSawRunning || !udsSawRunning {
		t.Fatalf(
			"server factories observed automation running: http=%v uds=%v, want both true",
			httpSawRunning,
			udsSawRunning,
		)
	}
}

func TestBootPreservesAutomationEnabledOverlaysAcrossRestart(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Automation.Enabled = true
	cfg.Automation.Jobs = []compozyconfig.AutomationJob{
		{
			Scope:     automationpkg.AutomationScopeGlobal,
			Name:      "restart-job",
			AgentName: "researcher",
			Prompt:    "Summarize the latest state.",
			Schedule: automationpkg.ScheduleSpec{
				Mode:     automationpkg.ScheduleModeEvery,
				Interval: "1h",
			},
			Enabled:   true,
			Retry:     automationpkg.DefaultRetryConfig(),
			FireLimit: automationpkg.DefaultFireLimitConfig(),
			Source:    automationpkg.JobSourceConfig,
		},
	}
	cfg.Automation.Triggers = []compozyconfig.AutomationTrigger{
		{
			Scope:     automationpkg.AutomationScopeGlobal,
			Name:      "restart-trigger",
			AgentName: "reviewer",
			Prompt:    `Review session {{ index .Data "session_id" }}`,
			Event:     "session.stopped",
			Filter:    map[string]string{"data.agent_name": "reviewer"},
			Enabled:   true,
			Retry:     automationpkg.DefaultRetryConfig(),
			FireLimit: automationpkg.DefaultFireLimitConfig(),
			Source:    automationpkg.JobSourceConfig,
		},
	}

	newDaemon := func() *Daemon {
		d, err := New(
			WithHomePaths(homePaths),
			WithConfig(&cfg),
			WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
			return &fakeSessionManager{}, nil
		}
		d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
			return &fakeObserver{}, nil
		}
		d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "http"}, nil
		}
		d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "uds"}, nil
		}
		return d
	}

	first := newDaemon()
	if err := first.boot(testutil.Context(t)); err != nil {
		t.Fatalf("first boot() error = %v", err)
	}

	jobs, err := first.automation.Jobs(testutil.Context(t))
	if err != nil {
		t.Fatalf("first automation.Jobs() error = %v", err)
	}
	job := findAutomationJobByName(jobs, "restart-job")
	if job == nil {
		t.Fatal("first boot missing restart-job")
	}
	triggers, err := first.automation.Triggers(testutil.Context(t))
	if err != nil {
		t.Fatalf("first automation.Triggers() error = %v", err)
	}
	trigger := findAutomationTriggerByName(triggers, "restart-trigger")
	if trigger == nil {
		t.Fatal("first boot missing restart-trigger")
	}

	if _, err := first.automation.SetJobEnabled(testutil.Context(t), job.ID, false); err != nil {
		t.Fatalf("SetJobEnabled() error = %v", err)
	}
	if _, err := first.automation.SetTriggerEnabled(testutil.Context(t), trigger.ID, false); err != nil {
		t.Fatalf("SetTriggerEnabled() error = %v", err)
	}
	if err := first.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("first Shutdown() error = %v", err)
	}

	second := newDaemon()
	if err := second.boot(testutil.Context(t)); err != nil {
		t.Fatalf("second boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := second.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("second Shutdown() error = %v", err)
		}
	})

	jobs, err = second.automation.Jobs(testutil.Context(t))
	if err != nil {
		t.Fatalf("second automation.Jobs() error = %v", err)
	}
	job = findAutomationJobByName(jobs, "restart-job")
	if job == nil || job.Enabled {
		t.Fatalf("restarted job = %#v, want disabled overlay", job)
	}

	triggers, err = second.automation.Triggers(testutil.Context(t))
	if err != nil {
		t.Fatalf("second automation.Triggers() error = %v", err)
	}
	trigger = findAutomationTriggerByName(triggers, "restart-trigger")
	if trigger == nil || trigger.Enabled {
		t.Fatalf("restarted trigger = %#v, want disabled overlay", trigger)
	}

	db, err := openDaemonTestGlobalDBAtPath(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	defer func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("GlobalDB.Close() error = %v", err)
		}
	}()

	kernel, err := resources.NewKernel(db.DB())
	if err != nil {
		t.Fatalf("NewKernel() error = %v", err)
	}
	jobCodec, err := automationpkg.NewJobResourceCodec()
	if err != nil {
		t.Fatalf("NewJobResourceCodec() error = %v", err)
	}
	jobStore, err := resources.NewStore(kernel, jobCodec)
	if err != nil {
		t.Fatalf("NewStore(job) error = %v", err)
	}
	triggerCodec, err := automationpkg.NewTriggerResourceCodec()
	if err != nil {
		t.Fatalf("NewTriggerResourceCodec() error = %v", err)
	}
	triggerStore, err := resources.NewStore(kernel, triggerCodec)
	if err != nil {
		t.Fatalf("NewStore(trigger) error = %v", err)
	}

	storedJob, err := jobStore.Get(testutil.Context(t), resourceReconcileActor(), job.ID)
	if err != nil {
		t.Fatalf("jobStore.Get() error = %v", err)
	}
	if !storedJob.Spec.Enabled {
		t.Fatal("stored resource job enabled default = false, want true")
	}
	jobOverlay, err := db.GetJobEnabledOverlay(testutil.Context(t), job.ID)
	if err != nil {
		t.Fatalf("GetJobEnabledOverlay() error = %v", err)
	}
	if jobOverlay.EnabledOverride {
		t.Fatal("job overlay enabled_override = true, want false")
	}

	storedTrigger, err := triggerStore.Get(testutil.Context(t), resourceReconcileActor(), trigger.ID)
	if err != nil {
		t.Fatalf("triggerStore.Get() error = %v", err)
	}
	if !storedTrigger.Spec.Enabled {
		t.Fatal("stored resource trigger enabled default = false, want true")
	}
	triggerOverlay, err := db.GetTriggerEnabledOverlay(testutil.Context(t), trigger.ID)
	if err != nil {
		t.Fatalf("GetTriggerEnabledOverlay() error = %v", err)
	}
	if triggerOverlay.EnabledOverride {
		t.Fatal("trigger overlay enabled_override = true, want false")
	}
}

func TestShutdownCancelsActiveAutomationPrompt(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Automation.Enabled = true
	cfg.Automation.MaxConcurrentJobs = 1
	cfg.Automation.Jobs = []compozyconfig.AutomationJob{
		{
			Scope:     automationpkg.AutomationScopeGlobal,
			Name:      "shutdown-job",
			AgentName: "researcher",
			Prompt:    "Summarize the latest state.",
			Schedule: automationpkg.ScheduleSpec{
				Mode:     automationpkg.ScheduleModeEvery,
				Interval: "10ms",
			},
			Enabled:   true,
			Retry:     automationpkg.DefaultRetryConfig(),
			FireLimit: automationpkg.DefaultFireLimitConfig(),
			Source:    automationpkg.JobSourceConfig,
		},
	}

	promptStarted := make(chan struct{}, 1)
	promptCancelled := make(chan struct{}, 1)
	sessions := &fakeSessionManager{
		promptStarted:      promptStarted,
		promptCtxCancelled: promptCancelled,
	}

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return sessions, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}

	select {
	case <-promptStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("automation scheduler did not reach Prompt() in time")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Shutdown(testutil.Context(t))
	}()

	select {
	case <-promptCancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("automation prompt context was not cancelled during shutdown")
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown() did not finish after automation prompt cancellation")
	}
}

func TestDrainAllowsActiveAutomationPromptToFinishBeforeJoinedShutdown(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Automation.Enabled = true
	cfg.Automation.MaxConcurrentJobs = 1
	cfg.Automation.Jobs = []compozyconfig.AutomationJob{
		{
			Scope:     automationpkg.AutomationScopeGlobal,
			Name:      "drain-job",
			AgentName: "researcher",
			Prompt:    "Finish admitted work.",
			Schedule: automationpkg.ScheduleSpec{
				Mode:     automationpkg.ScheduleModeEvery,
				Interval: "10ms",
			},
			Enabled:   true,
			Retry:     automationpkg.DefaultRetryConfig(),
			FireLimit: automationpkg.FireLimitConfig{Max: 1, Window: "1h"},
			Source:    automationpkg.JobSourceConfig,
		},
	}

	promptStarted := make(chan struct{})
	releasePrompt := make(chan struct{})
	promptFinished := make(chan struct{})
	var d *Daemon
	sessions := &fakeSessionManager{}
	sessions.promptHook = func(context.Context, string, string) (<-chan acp.AgentEvent, error) {
		if d.IsDraining() {
			return nil, admission.ErrDraining
		}
		close(promptStarted)
		events := make(chan acp.AgentEvent, 1)
		go func() {
			defer close(promptFinished)
			defer close(events)
			<-releasePrompt
			events <- acp.AgentEvent{
				Type:             acp.EventTypeDone,
				Timestamp:        time.Now().UTC(),
				StopReason:       string(acp.PromptStopReasonEndTurn),
				PromptStopReason: acp.PromptStopReasonEndTurn,
			}
		}()
		return events, nil
	}

	var err error
	d, err = New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return sessions, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	select {
	case <-promptStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("automation scheduler did not admit prompt in time")
	}
	if err := d.Drain(testutil.Context(t)); err != nil {
		t.Fatalf("Drain() error = %v", err)
	}
	if !d.IsDraining() {
		t.Fatal("IsDraining() = false after Drain")
	}
	select {
	case <-promptFinished:
		t.Fatal("admitted prompt finished before release")
	default:
	}

	close(releasePrompt)
	select {
	case <-promptFinished:
	case <-time.After(2 * time.Second):
		t.Fatal("admitted prompt did not finish after release")
	}
	if err := d.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if got := sessions.shutdownCalls; got != 1 {
		t.Fatalf("session manager Shutdown() calls = %d, want 1", got)
	}
	if !d.IsDraining() {
		t.Fatal("IsDraining() = false after joined shutdown")
	}
}

func TestBootLoadsExtensionsRebuildsHooksAndStopsOnShutdown(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	hookMarker := filepath.Join(t.TempDir(), "hook.json")
	shutdownMarker := filepath.Join(t.TempDir(), "shutdown.txt")
	installExtensionForDaemonIntegration(t, homePaths.DatabaseFile, "ext-daemon", daemonTestExtensionOptions{
		runtimeCommand: daemonExtensionHelperCommand(t),
		runtimeArgs:    daemonExtensionHelperArgs(),
		runtimeEnv:     daemonExtensionHelperEnv(shutdownMarker),
		hookCommand:    "/bin/sh",
		hookArgs: []string{
			"-c",
			`cat > "$1"; printf '{}'`,
			"compozy-extension-hook",
			hookMarker,
		},
		hookEvent: hookspkg.HookSessionPostCreate,
	}, true)

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	if d.extensions == nil {
		t.Fatal("boot() did not publish the extension runtime")
	}
	projector, ok := d.extensions.(profiledExtensionHookRuntime)
	if !ok {
		t.Fatal("boot() extension runtime does not support profile hook projection")
	}
	profiles, err := d.profiles.List(testutil.Context(t))
	if err != nil {
		t.Fatalf("profiles.List() error = %v", err)
	}
	activeProfiles := activeExtensionProfileLenses(profiles)
	extensionHooks, err := projector.HookDeclarationsForProfiles(testutil.Context(t), activeProfiles)
	if err != nil {
		t.Fatalf("HookDeclarationsForProfiles() error = %v", err)
	}
	var daemonHooks []hookspkg.HookDecl
	for _, hook := range extensionHooks {
		if hook.Name == "ext-daemon-hook" {
			daemonHooks = append(daemonHooks, hook)
		}
	}
	if len(daemonHooks) != len(activeProfiles) {
		t.Fatalf("ext-daemon hook count = %d, want %d (one per active profile)", len(daemonHooks), len(activeProfiles))
	}
	for _, profile := range activeProfiles {
		count := 0
		for _, hook := range daemonHooks {
			if hook.PlacementProfileID() == profile.ID {
				count++
				if hook.Source != hookspkg.HookSourceExtension || hook.Priority != 300 {
					t.Fatalf("extension hook = %#v, want source extension with priority 300", hook)
				}
			}
		}
		if count != 1 {
			t.Fatalf("profile %q has %d ext-daemon hooks, want 1", profile.Name, count)
		}
	}

	payload := hookspkg.SessionPostCreatePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSessionPostCreate,
			Timestamp: time.Now().UTC(),
		},
		SessionContext: hookspkg.SessionContext{
			ProfileID: store.DefaultProfileID,
			SessionID: "sess-ext",
			AgentName: "coder",
			State:     string(session.StateActive),
		},
	}
	if _, err := d.hooks.DispatchSessionPostCreate(testutil.Context(t), payload); err != nil {
		t.Fatalf("DispatchSessionPostCreate() error = %v", err)
	}

	waitForCondition(t, "extension hook marker", func() bool {
		_, err := os.Stat(hookMarker)
		return err == nil
	})
	hookPayload, err := os.ReadFile(hookMarker)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", hookMarker, err)
	}
	if !strings.Contains(string(hookPayload), "sess-ext") {
		t.Fatalf("hook payload = %q, want session id", string(hookPayload))
	}

	if err := d.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if payload, err := os.ReadFile(shutdownMarker); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", shutdownMarker, err)
	} else if strings.TrimSpace(string(payload)) != "shutdown" {
		t.Fatalf("shutdown marker = %q, want shutdown", string(payload))
	}
}

func TestBootContinuesAfterCorruptExtensionAndKeepsHealthyExtensions(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	hookMarker := filepath.Join(t.TempDir(), "hook.json")
	shutdownMarker := filepath.Join(t.TempDir(), "shutdown.txt")
	installExtensionForDaemonIntegration(t, homePaths.DatabaseFile, "ext-good", daemonTestExtensionOptions{
		runtimeCommand: daemonExtensionHelperCommand(t),
		runtimeArgs:    daemonExtensionHelperArgs(),
		runtimeEnv:     daemonExtensionHelperEnv(shutdownMarker),
		hookCommand:    "/bin/sh",
		hookArgs: []string{
			"-c",
			`cat > "$1"; printf '{}'`,
			"compozy-extension-hook",
			hookMarker,
		},
		hookEvent: hookspkg.HookSessionPostCreate,
	}, true)
	badDir := installExtensionForDaemonIntegration(t, homePaths.DatabaseFile, "ext-bad", daemonTestExtensionOptions{
		runtimeCommand: daemonExtensionHelperCommand(t),
		runtimeArgs:    daemonExtensionHelperArgs(),
		runtimeEnv:     daemonExtensionHelperEnv(""),
	}, true)
	writeDaemonFile(t, filepath.Join(badDir, "extension.toml"), "not = [valid")

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, nil))

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v, want boot to continue after corrupt extension", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	payload := hookspkg.SessionPostCreatePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSessionPostCreate,
			Timestamp: time.Now().UTC(),
		},
		SessionContext: hookspkg.SessionContext{
			SessionID: "sess-good",
			ProfileID: store.DefaultProfileID,
			AgentName: "coder",
			State:     string(session.StateActive),
		},
	}
	if _, err := d.hooks.DispatchSessionPostCreate(testutil.Context(t), payload); err != nil {
		t.Fatalf("DispatchSessionPostCreate() error = %v", err)
	}

	waitForCondition(t, "healthy extension hook marker", func() bool {
		_, err := os.Stat(hookMarker)
		return err == nil
	})
	hookPayload, err := os.ReadFile(hookMarker)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", hookMarker, err)
	}
	if !strings.Contains(string(hookPayload), "sess-good") {
		t.Fatalf("hook payload = %q, want healthy extension session id", string(hookPayload))
	}
	if err := d.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() before reading logs error = %v", err)
	}
	if !strings.Contains(logBuffer.String(), "extension manager start failed") {
		t.Fatalf("log output = %q, want extension start failure entry", logBuffer.String())
	}

}

func TestRunGracefulShutdownViaContextCancellation(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(runCtx)
	}()

	<-d.readyCh
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if _, err := os.Stat(homePaths.DaemonInfo); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("daemon.json after shutdown: stat error = %v, want os.ErrNotExist", err)
	}

	lock, err := AcquireLock(homePaths.DaemonLock, os.Getpid())
	if err != nil {
		t.Fatalf("AcquireLock(after shutdown) error = %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("lock.Release() error = %v", err)
	}
}

func TestRunGracefulShutdownViaSignal(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	signalCh := make(chan os.Signal, 1)

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
		WithSignalBridge(signalCh),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(context.Background())
	}()

	<-d.readyCh
	signalCh <- syscall.SIGINT

	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if _, err := os.Stat(homePaths.DaemonInfo); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("daemon.json after signal shutdown: stat error = %v, want os.ErrNotExist", err)
	}
}

func TestShutdownPersistsShutdownStopReason(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	command := daemonSessionStopHelperCommand(t)
	cfg.Defaults.Provider = acpmock.ProviderName
	cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(command)
	writeDaemonIntegrationProviderConfig(t, homePaths, acpmock.ProviderName, command)
	writeDaemonIntegrationAgentDef(t, homePaths, "coder", command)

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", workspaceRoot, err)
	}

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	shutdown := false
	t.Cleanup(func() {
		if shutdown {
			return
		}
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("cleanup Shutdown() error = %v", err)
		}
	})

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}

	sess, err := d.sessions.Create(testutil.Context(t), session.CreateOpts{
		AgentName:     "coder",
		WorkspacePath: workspaceRoot,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := d.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	shutdown = true

	meta, err := store.ReadSessionMeta(sess.MetaPath())
	if err != nil {
		t.Fatalf("ReadSessionMeta(%q) error = %v", sess.MetaPath(), err)
	}
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopShutdown {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopShutdown)
	}
}

func TestBootInitializesMemoryStoreAndAssemblerIntegration(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Memory.GlobalDir = filepath.Join(homePaths.HomeDir, "external-memory")

	var capturedDeps SessionManagerDeps

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(_ context.Context, deps SessionManagerDeps) (SessionManager, error) {
		capturedDeps = deps
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	if d.memoryStore == nil {
		t.Fatal("boot() did not initialize the memory store")
	}
	registry, ok := d.registry.(*globaldb.GlobalDB)
	if !ok {
		t.Fatalf("registry type = %T, want *globaldb.GlobalDB", d.registry)
	}
	for _, stream := range daemonMigrationStreams() {
		if err := store.RequireCurrent(testutil.Context(t), registry.DB(), stream); err != nil {
			t.Fatalf("RequireCurrent(%s after boot) error = %v", stream.Name, err)
		}
	}
	if capturedDeps.PromptAssembler == nil {
		t.Fatal("boot() did not inject the prompt assembler")
	}
	if capturedDeps.SkillRegistry == nil {
		t.Fatal("boot() did not inject the skills registry")
	}
	if capturedDeps.MCPResolver == nil {
		t.Fatal("boot() did not inject the MCP resolver")
	}
	if capturedDeps.WorkspaceResolver == nil {
		t.Fatal("boot() did not inject the workspace resolver")
	}
	if _, err := os.Stat(cfg.Memory.GlobalDir); err != nil {
		t.Fatalf("stat external memory directory error = %v", err)
	}
}

func TestBootLoadsBundledSkillsIntoPromptAssemblerInSkillsOnlyMode(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Memory.Enabled = false
	cfg.Skills.Enabled = true

	var capturedDeps SessionManagerDeps

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(_ context.Context, deps SessionManagerDeps) (SessionManager, error) {
		capturedDeps = deps
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	if capturedDeps.PromptAssembler == nil {
		t.Fatal("boot() did not inject the prompt assembler")
	}
	if capturedDeps.WorkspaceResolver == nil {
		t.Fatal("boot() did not inject the workspace resolver")
	}
	if d.skillsRegistry == nil {
		t.Fatal("boot() did not initialize the skills registry")
	}
	if _, ok := d.skillsRegistry.Get("compozy"); !ok {
		t.Fatal("skills registry does not contain bundled skill compozy")
	}

	workspace := workspacepkg.ResolvedWorkspace{
		Agents: []compozyconfig.AgentDef{testPromptAgent("Base prompt.")},
	}
	prompt, err := capturedDeps.PromptAssembler.Assemble(
		context.Background(),
		testPromptAgent("Base prompt."),
		&workspace,
	)
	if err != nil {
		t.Fatalf("PromptAssembler.Assemble() error = %v", err)
	}

	assertPromptContainsInOrder(t, prompt, "Base prompt.", "<available-skills>", "compozy")
	assertPromptExcludes(t, prompt, "# Persistent Memory")

	t.Run("Should migrate every shared database stream while memory is disabled", func(t *testing.T) {
		if d.memoryStore != nil {
			t.Fatal("boot() initialized the memory runtime while memory is disabled")
		}
		registry, ok := d.registry.(*globaldb.GlobalDB)
		if !ok {
			t.Fatalf("registry type = %T, want *globaldb.GlobalDB", d.registry)
		}
		for _, stream := range daemonMigrationStreams() {
			if err := store.RequireCurrent(testutil.Context(t), registry.DB(), stream); err != nil {
				t.Fatalf(
					"RequireCurrent(%s after memory-disabled boot) error = %v",
					stream.Name,
					err,
				)
			}
		}

		var memoryTable string
		if queryErr := registry.DB().QueryRowContext(
			testutil.Context(t),
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'memory_catalog_entries'`,
		).Scan(&memoryTable); queryErr != nil {
			t.Fatalf("query memory domain table after memory-disabled boot: %v", queryErr)
		}
		if memoryTable != "memory_catalog_entries" {
			t.Fatalf("memory domain table = %q, want memory_catalog_entries", memoryTable)
		}
	})
}

func TestBootLeavesSkillDependenciesNilWhenSkillsDisabled(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Skills.Enabled = false

	var capturedDeps SessionManagerDeps

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(_ context.Context, deps SessionManagerDeps) (SessionManager, error) {
		capturedDeps = deps
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	if capturedDeps.SkillRegistry != nil {
		t.Fatalf("boot() SkillRegistry = %#v, want nil when skills are disabled", capturedDeps.SkillRegistry)
	}
	if capturedDeps.MCPResolver != nil {
		t.Fatalf("boot() MCPResolver = %#v, want nil when skills are disabled", capturedDeps.MCPResolver)
	}
}

// TestBootBuildsHooksFromWorkspaceConfigAgentAndSkills verifies boot registers workspace, agent, and skill hooks under the registered workspace identity.
func TestBootBuildsHooksFromWorkspaceConfigAgentAndSkills(t *testing.T) {
	t.Run("Should build hooks from workspace config agent and skills", func(t *testing.T) {
		homePaths := integrationHomePaths(t)
		cfg := testConfig(t, homePaths)
		cfg.Memory.Enabled = false
		cfg.Skills.Enabled = true

		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		if err := os.MkdirAll(filepath.Join(workspaceRoot, compozyconfig.DirName), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Join(workspaceRoot, compozyconfig.DirName), err)
		}

		scriptPath := writeDaemonHookScript(t, t.TempDir(), "capture.sh", "#!/bin/sh\ncat > \"$1\"\n")
		windowScriptPath := writeDaemonHookScript(
			t,
			t.TempDir(),
			"capture-window.sh",
			"#!/bin/sh\ncat > \"$2\"\nprintf 'x\\n' >> \"$1\"\n",
		)
		configOutput := filepath.Join(t.TempDir(), "config-create.json")
		agentOutput := filepath.Join(t.TempDir(), "agent-stop.json")
		skillOutput := filepath.Join(t.TempDir(), "skill-create.json")
		windowCountOutput := filepath.Join(t.TempDir(), "window-count.txt")
		windowOpenedOutput := filepath.Join(t.TempDir(), "window-opened.json")
		windowClosedOutput := filepath.Join(t.TempDir(), "window-closed.json")
		windowGroupedOutput := filepath.Join(t.TempDir(), "window-grouped.json")
		windowUngroupedOutput := filepath.Join(t.TempDir(), "window-ungrouped.json")
		windowActivatedOutput := filepath.Join(t.TempDir(), "window-activated.json")

		writeDaemonFile(t, filepath.Join(workspaceRoot, compozyconfig.DirName, "config.toml"), `
[[hooks.declarations]]
name = "config-create"
event = "session.post_create"
mode = "sync"
command = "`+scriptPath+`"
args = ["`+configOutput+`"]

[[hooks.declarations]]
name = "window-opened"
event = "window_manager.window.opened"
mode = "async"
command = "`+windowScriptPath+`"
args = ["`+windowCountOutput+`", "`+windowOpenedOutput+`"]

[[hooks.declarations]]
name = "window-closed"
event = "window_manager.window.closed"
mode = "async"
command = "`+windowScriptPath+`"
args = ["`+windowCountOutput+`", "`+windowClosedOutput+`"]

[[hooks.declarations]]
name = "window-grouped"
event = "window_manager.stack.grouped"
mode = "async"
command = "`+windowScriptPath+`"
args = ["`+windowCountOutput+`", "`+windowGroupedOutput+`"]

[[hooks.declarations]]
name = "window-ungrouped"
event = "window_manager.stack.ungrouped"
mode = "async"
command = "`+windowScriptPath+`"
args = ["`+windowCountOutput+`", "`+windowUngroupedOutput+`"]

[[hooks.declarations]]
name = "window-activated"
event = "window_manager.stack.activated"
mode = "async"
command = "`+windowScriptPath+`"
args = ["`+windowCountOutput+`", "`+windowActivatedOutput+`"]
`)
		writeDaemonFile(t, filepath.Join(workspaceRoot, compozyconfig.DirName, "agents", "coder", "AGENT.md"), `---
name: coder
provider: claude
hooks:
  - name: agent-stop
    event: session.post_stop
    mode: sync
    command: `+scriptPath+`
    args: ["`+agentOutput+`"]
---

Prompt.
`)
		writeDaemonFile(t, filepath.Join(workspaceRoot, compozyconfig.DirName, "skills", "local-hook", "SKILL.md"), `---
name: local-hook
description: workspace lifecycle hook
metadata:
  compozy:
    hooks:
      - event: session.post_create
        mode: sync
        command: `+scriptPath+`
        args:
          - `+skillOutput+`
---

body
`)

		resolvedWorkspace := seedDaemonWorkspace(t, homePaths, workspaceRoot)
		if resolvedWorkspace.ID == resolvedWorkspace.WorkspaceID {
			t.Fatal("fixture must distinguish registered and durable workspace identities")
		}

		var capturedDeps SessionManagerDeps
		d, err := New(
			WithHomePaths(homePaths),
			WithConfig(&cfg),
			WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		d.newSessionManager = func(_ context.Context, deps SessionManagerDeps) (SessionManager, error) {
			capturedDeps = deps
			return &fakeSessionManager{}, nil
		}
		d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
			return &fakeObserver{}, nil
		}
		d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "http"}, nil
		}
		d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "uds"}, nil
		}

		if err := d.boot(testutil.Context(t)); err != nil {
			t.Fatalf("boot() error = %v", err)
		}
		t.Cleanup(func() {
			if err := d.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
		})

		if d.hooks == nil {
			t.Fatal("boot() did not initialize hooks runtime")
		}
		if capturedDeps.Notifier == nil {
			t.Fatal("boot() did not inject the hooks notifier")
		}
		if capturedDeps.Hooks.Session == nil {
			t.Fatal("boot() did not inject the hooks dispatcher")
		}

		sess := &session.Session{
			ID:          "sess-1",
			Name:        "demo",
			AgentName:   "coder",
			WorkspaceID: resolvedWorkspace.ID,
			Workspace:   resolvedWorkspace.RootDir,
			Type:        session.SessionTypeUser,
			State:       session.StateStopped,
			CreatedAt:   time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 4, 9, 11, 0, 0, 0, time.UTC),
		}

		if _, err := capturedDeps.Hooks.Session.DispatchSessionPostCreate(
			testutil.Context(t),
			hookspkg.SessionPostCreatePayload(
				hookSessionLifecyclePayload(sess, hookspkg.HookSessionPostCreate, time.Now().UTC()),
			),
		); err != nil {
			t.Fatalf("DispatchSessionPostCreate() error = %v", err)
		}
		if _, err := capturedDeps.Hooks.Session.DispatchSessionPostStop(
			testutil.Context(t),
			hookspkg.SessionPostStopPayload(
				hookSessionLifecyclePayload(sess, hookspkg.HookSessionPostStop, time.Now().UTC()),
			),
		); err != nil {
			t.Fatalf("DispatchSessionPostStop() error = %v", err)
		}

		assertLifecycleHookPayload(t, configOutput, hookspkg.HookSessionPostCreate, resolvedWorkspace)
		assertLifecycleHookPayload(t, skillOutput, hookspkg.HookSessionPostCreate, resolvedWorkspace)
		assertLifecycleHookPayload(t, agentOutput, hookspkg.HookSessionPostStop, resolvedWorkspace)

		t.Run("Should dispatch configured window-manager hooks", func(t *testing.T) {
			windowHookDispatcher, ok := d.hooks.(*hookspkg.Hooks)
			if !ok {
				t.Fatalf("boot() hooks runtime = %T, want *hooks.Hooks", d.hooks)
			}
			windowObserver := newWindowManagerHookObserver(&bootState{
				logger:         discardLogger(),
				hookDispatcher: windowHookDispatcher,
			})
			windowEvents := []struct {
				command windowmanager.CommandID
				changes windowmanager.ChangeSet
				event   hookspkg.HookEvent
				output  string
			}{
				{
					command: windowmanager.CommandWindowOpen,
					event:   hookspkg.HookWindowManagerWindowOpened,
					output:  windowOpenedOutput,
				},
				{
					command: windowmanager.CommandWindowClose,
					event:   hookspkg.HookWindowManagerWindowClosed,
					output:  windowClosedOutput,
				},
				{
					command: windowmanager.CommandWindowStackGroup,
					changes: windowmanager.ChangeSet{StackGrouped: []windowmanager.NodeID{"stack-a"}},
					event:   hookspkg.HookWindowManagerStackGrouped,
					output:  windowGroupedOutput,
				},
				{
					command: windowmanager.CommandWindowMove,
					changes: windowmanager.ChangeSet{StackUngrouped: []windowmanager.NodeID{"stack-a"}},
					event:   hookspkg.HookWindowManagerStackUngrouped,
					output:  windowUngroupedOutput,
				},
				{
					command: windowmanager.CommandWindowStackSetActive,
					event:   hookspkg.HookWindowManagerStackActivated,
					output:  windowActivatedOutput,
				},
			}
			for index, item := range windowEvents {
				windowObserver(testutil.Context(t), windowmanager.Event{
					WorkspaceID: windowmanager.WorkspaceID(resolvedWorkspace.ID),
					Revision:    windowmanager.Revision(index + 1),
					CommandID:   item.command,
					Changes:     item.changes,
					OccurredAt:  time.Date(2026, 7, 30, 12, index, 0, 0, time.UTC),
				})
			}
			waitForCondition(t, "five window-manager script hook deliveries", func() bool {
				payload, err := os.ReadFile(windowCountOutput)
				return err == nil && len(strings.Fields(string(payload))) == len(windowEvents)
			})
			for _, item := range windowEvents {
				payload, err := os.ReadFile(item.output)
				if err != nil {
					t.Fatalf("os.ReadFile(%q) error = %v", item.output, err)
				}
				var captured hookspkg.WindowManagerPayload
				if err := json.Unmarshal(payload, &captured); err != nil {
					t.Fatalf("json.Unmarshal(%s hook payload) error = %v; body=%s", item.event, err, payload)
				}
				if captured.Event != item.event || captured.WorkspaceID != resolvedWorkspace.ID {
					t.Fatalf("captured %s hook payload = %#v", item.event, captured)
				}
			}
			windowObserver(testutil.Context(t), windowmanager.Event{
				WorkspaceID: windowmanager.WorkspaceID(resolvedWorkspace.ID),
				CommandID:   windowmanager.CommandWindowFocus,
			})
			countPayload, err := os.ReadFile(windowCountOutput)
			if err != nil {
				t.Fatalf("os.ReadFile(%q) error = %v", windowCountOutput, err)
			}
			if len(strings.Fields(string(countPayload))) != len(windowEvents) {
				t.Fatalf("window hook invocation count = %q, want exactly %d", countPayload, len(windowEvents))
			}
		})
	})
}

// TestBootRunsWorkspaceTaskRunHookWithRelativeScriptPath verifies task hooks resolve relative scripts within their registered workspace.
func TestBootRunsWorkspaceTaskRunHookWithRelativeScriptPath(t *testing.T) {
	t.Run("Should run workspace task-run hook with relative script path", func(t *testing.T) {
		homePaths := integrationHomePaths(t)
		cfg := testConfig(t, homePaths)
		cfg.Memory.Enabled = false
		cfg.Skills.Enabled = false

		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		if err := os.MkdirAll(filepath.Join(workspaceRoot, compozyconfig.DirName, "hooks"), 0o755); err != nil {
			t.Fatalf(
				"os.MkdirAll(%q) error = %v",
				filepath.Join(workspaceRoot, compozyconfig.DirName, "hooks"),
				err,
			)
		}
		writeDaemonFile(
			t,
			filepath.Join(workspaceRoot, compozyconfig.DirName, "hooks", "capture-task-run.sh"),
			"#!/bin/sh\ncat > \"$1\"\n",
		)
		if err := os.Chmod(
			filepath.Join(workspaceRoot, compozyconfig.DirName, "hooks", "capture-task-run.sh"),
			0o755,
		); err != nil {
			t.Fatalf("os.Chmod(capture-task-run.sh) error = %v", err)
		}
		writeDaemonFile(t, filepath.Join(workspaceRoot, compozyconfig.DirName, "config.toml"), `
[[hooks.declarations]]
name = "workspace-task-run"
event = "task.run.enqueued"
mode = "sync"
command = "/bin/sh"
args = [".compozy/hooks/capture-task-run.sh", ".compozy/task-run-enqueued.json"]
`)

		resolvedWorkspace := seedDaemonWorkspace(t, homePaths, workspaceRoot)

		d, err := New(
			WithHomePaths(homePaths),
			WithConfig(&cfg),
			WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		d.newSessionManager = func(_ context.Context, deps SessionManagerDeps) (SessionManager, error) {
			return &fakeSessionManager{}, nil
		}
		d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
			return &fakeObserver{}, nil
		}
		d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "http"}, nil
		}
		d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "uds"}, nil
		}

		if err := d.boot(testutil.Context(t)); err != nil {
			t.Fatalf("boot() error = %v", err)
		}
		t.Cleanup(func() {
			if err := d.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
		})
		if d.hooks == nil {
			t.Fatal("boot() did not initialize daemon hooks")
		}

		payload := hookspkg.TaskRunEnqueuedPayload{
			PayloadBase: hookspkg.PayloadBase{
				Event:     hookspkg.HookTaskRunEnqueued,
				Timestamp: time.Date(2026, 4, 26, 19, 30, 0, 0, time.UTC),
			},
			TaskRunContext: hookspkg.TaskRunContext{
				TaskID:      "task-1",
				RunID:       "run-1",
				WorkspaceID: resolvedWorkspace.ID,

				AgentName:  "qa",
				TaskStatus: "ready",
				RunStatus:  "queued",
			},
			IdempotencyKey: "task.start.task-1",
		}

		outputPath := filepath.Join(workspaceRoot, compozyconfig.DirName, "task-run-enqueued.json")
		outside := payload
		outside.WorkspaceID = "ws-outside"
		if _, err := d.hooks.DispatchTaskRunEnqueued(testutil.Context(t), outside); err != nil {
			t.Fatalf("DispatchTaskRunEnqueued(outside workspace) error = %v", err)
		}
		if _, err := os.Stat(outputPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("outside workspace dispatched the hook: stat error = %v", err)
		}

		if _, err := d.hooks.DispatchTaskRunEnqueued(testutil.Context(t), payload); err != nil {
			t.Fatalf("DispatchTaskRunEnqueued() error = %v", err)
		}

		body, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", outputPath, err)
		}

		var captured hookspkg.TaskRunEnqueuedPayload
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("json.Unmarshal(task run hook payload) error = %v; body=%s", err, string(body))
		}
		if captured.Event != hookspkg.HookTaskRunEnqueued ||
			captured.WorkspaceID != resolvedWorkspace.ID ||
			captured.RunID != "run-1" {
			t.Fatalf("captured payload = %#v, want enqueued payload for the seeded workspace run", captured)
		}
	})
}

// TestBootSkillsWatcherRebuildsHooksBeforeNextDispatch verifies changed skill hooks are rebuilt before the next workspace event dispatch.
func TestBootSkillsWatcherRebuildsHooksBeforeNextDispatch(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Memory.Enabled = false
	cfg.Skills.Enabled = true
	cfg.Skills.PollInterval = 10 * time.Millisecond

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	resolvedWorkspace := seedDaemonWorkspace(t, homePaths, workspaceRoot)
	outputPath := filepath.Join(t.TempDir(), "watched-create.json")
	scriptPath := writeDaemonHookScript(t, t.TempDir(), "capture.sh", "#!/bin/sh\ncat > \"$1\"\n")

	var capturedDeps SessionManagerDeps
	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(_ context.Context, deps SessionManagerDeps) (SessionManager, error) {
		capturedDeps = deps
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})
	if capturedDeps.Hooks.Session == nil {
		t.Fatal("boot() did not inject the hooks dispatcher")
	}

	writeDaemonFile(t, filepath.Join(homePaths.SkillsDir, "watched-hook", "SKILL.md"), `---
name: watched-hook
description: reloaded hook
metadata:
  compozy:
    hooks:
      - event: session.post_create
        mode: sync
        command: `+scriptPath+`
        args:
          - `+outputPath+`
---

body
`)

	sess := &session.Session{
		ID:          "sess-watch",
		AgentName:   "general",
		WorkspaceID: resolvedWorkspace.ID,
		Workspace:   resolvedWorkspace.RootDir,
		Type:        session.SessionTypeUser,
		State:       session.StateActive,
		CreatedAt:   time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC),
	}

	waitForConditionWithin(t, "hooks rebuild before next dispatch", 10*time.Second, func() bool {
		if _, ok := d.skillsRegistry.Get("watched-hook"); !ok {
			return false
		}
		if _, err := capturedDeps.Hooks.Session.DispatchSessionPostCreate(
			testutil.Context(t),
			hookspkg.SessionPostCreatePayload(
				hookSessionLifecyclePayload(sess, hookspkg.HookSessionPostCreate, time.Now().UTC()),
			),
		); err != nil {
			t.Fatalf("DispatchSessionPostCreate() error = %v", err)
		}
		_, err := os.Stat(outputPath)
		return err == nil
	})
	assertLifecycleHookPayload(t, outputPath, hookspkg.HookSessionPostCreate, resolvedWorkspace)
}

func TestBootSkillsWatcherRefreshesWorkspaceSkillsWithoutRestart(t *testing.T) {
	t.Run("Should publish workspace skills after a hot add", func(t *testing.T) {
		homePaths := integrationHomePaths(t)
		cfg := testConfig(t, homePaths)
		cfg.Memory.Enabled = false
		cfg.Skills.Enabled = true
		cfg.Skills.Sources = []string{compozyconfig.SkillSourceAgents}
		cfg.Skills.PollInterval = 10 * time.Millisecond

		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		resolvedWorkspace := seedDaemonWorkspace(t, homePaths, workspaceRoot)

		d, err := New(
			WithHomePaths(homePaths),
			WithConfig(&cfg),
			WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
			return &fakeSessionManager{}, nil
		}
		d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
			return &fakeObserver{}, nil
		}
		var runtimeDeps RuntimeDeps
		d.httpFactory = func(_ context.Context, deps RuntimeDeps) (Server, error) {
			runtimeDeps = deps
			return &fakeServer{name: "http"}, nil
		}
		d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "uds"}, nil
		}

		if err := d.boot(testutil.Context(t)); err != nil {
			t.Fatalf("boot() error = %v", err)
		}
		t.Cleanup(func() {
			if err := d.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
		})

		profileAgentFile := filepath.Join(
			workspaceRoot,
			compozyconfig.DirName,
			compozyconfig.ProfilesDirName,
			compozyconfig.DefaultProfileDirName,
			compozyconfig.AgentsDirName,
			"marketing",
			"AGENT.md",
		)
		writeDaemonFile(t, profileAgentFile, `---
name: marketing
provider: claude
---

You are a marketing assistant.
`)
		profileResolver, ok := d.workspaceResolver.(workspacepkg.ProfileRuntimeResolver)
		if !ok {
			t.Fatal("workspace resolver does not support profile layers")
		}
		waitForCondition(t, "default profile agent refresh after watcher sync", func() bool {
			resolved, err := profileResolver.ResolveForProfile(
				testutil.Context(t),
				resolvedWorkspace.ID,
				compozyconfig.DefaultProfileDirName,
			)
			if err != nil {
				return false
			}
			return slices.ContainsFunc(resolved.Agents, func(agent compozyconfig.AgentDef) bool {
				return agent.Name == "marketing"
			})
		})

		skillRoot := filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.SkillsDirName)
		skillFile := filepath.Join(skillRoot, "marketing", "watched-workspace-skill", "SKILL.md")
		writeDaemonFile(
			t,
			skillFile,
			`---
name: watched-workspace-skill
description: Workspace watched skill
---

# Watched Workspace Skill
`,
		)

		waitForCondition(t, "workspace skill refresh after watcher sync", func() bool {
			resolved, err := d.workspaceResolver.Resolve(testutil.Context(t), resolvedWorkspace.ID)
			if err != nil {
				return false
			}

			projectedSkills, err := d.skillsRegistry.ForWorkspace(testutil.Context(t), &resolved)
			if err != nil {
				return false
			}

			return findIntegrationSkill(projectedSkills, "watched-workspace-skill") != nil
		})

		writeDaemonFile(
			t,
			skillFile,
			`---
name: watched-workspace-skill
description: Updated workspace watched skill
---

# Updated Watched Workspace Skill
`,
		)
		waitForCondition(t, "workspace skill refresh after nested edit", func() bool {
			resolved, err := d.workspaceResolver.Resolve(testutil.Context(t), resolvedWorkspace.ID)
			if err != nil {
				return false
			}

			projectedSkills, err := d.skillsRegistry.ForWorkspace(testutil.Context(t), &resolved)
			if err != nil {
				return false
			}

			skill := findIntegrationSkill(projectedSkills, "watched-workspace-skill")
			return skill != nil && skill.Meta.Description == "Updated workspace watched skill"
		})

		if err := os.Remove(skillFile); err != nil {
			t.Fatalf("Remove(%q) error = %v", skillFile, err)
		}
		waitForCondition(t, "workspace skill refresh after nested removal", func() bool {
			resolved, err := d.workspaceResolver.Resolve(testutil.Context(t), resolvedWorkspace.ID)
			if err != nil {
				return false
			}

			projectedSkills, err := d.skillsRegistry.ForWorkspace(testutil.Context(t), &resolved)
			if err != nil {
				return false
			}

			return findIntegrationSkill(projectedSkills, "watched-workspace-skill") == nil
		})

		presetSkillFile := filepath.Join(
			workspaceRoot,
			".agents",
			"skills",
			"watched-preset-skill",
			"SKILL.md",
		)
		writeDaemonFile(t, presetSkillFile, `---
name: watched-preset-skill
description: Workspace preset skill
---

# Watched Preset Skill
`)
		waitForCondition(t, "enabled preset skill publication", func() bool {
			resolved, err := d.workspaceResolver.Resolve(testutil.Context(t), resolvedWorkspace.ID)
			if err != nil {
				return false
			}
			projectedSkills, err := d.skillsRegistry.ForWorkspace(testutil.Context(t), &resolved)
			return err == nil && findIntegrationSkill(projectedSkills, "watched-preset-skill") != nil
		})

		if runtimeDeps.Settings == nil {
			t.Fatal("boot runtime settings service = nil")
		}
		currentEnvelope, err := runtimeDeps.Settings.GetSection(testutil.Context(t), settingspkg.SectionRequest{
			Section: settingspkg.SectionSkills, Scope: settingspkg.ScopeUser,
		})
		if err != nil {
			t.Fatalf("GetSection(before source toggle) error = %v", err)
		}
		if currentEnvelope.Skills == nil {
			t.Fatal("GetSection(before source toggle).Skills = nil")
		}
		disabledSources := currentEnvelope.Skills.Config
		disabledSources.Sources = []string{}
		disableResult, err := runtimeDeps.Settings.ApplySection(
			settingspkg.WithMutationSource(testutil.Context(t), "http"),
			settingspkg.SectionUpdateRequest{
				SectionRequest: settingspkg.SectionRequest{
					Section: settingspkg.SectionSkills,
					Scope:   settingspkg.ScopeUser,
				},
				Skills: &disabledSources,
			},
		)
		if err != nil {
			t.Fatalf("ApplySection(disable agents) error = %v", err)
		}
		if !disableResult.Applied || disableResult.RestartRequired {
			t.Fatalf("ApplySection(disable agents) = %#v, want live apply", disableResult)
		}
		waitForCondition(t, "disabled preset leaves registry", func() bool {
			resolved, err := d.workspaceResolver.Resolve(testutil.Context(t), resolvedWorkspace.ID)
			if err != nil {
				return false
			}
			projectedSkills, err := d.skillsRegistry.ForWorkspace(testutil.Context(t), &resolved)
			return err == nil && findIntegrationSkill(projectedSkills, "watched-preset-skill") == nil
		})
		envelope, err := runtimeDeps.Settings.GetSection(testutil.Context(t), settingspkg.SectionRequest{
			Section: settingspkg.SectionSkills, Scope: settingspkg.ScopeUser,
		})
		if err != nil {
			t.Fatalf("GetSection(disabled sources) error = %v", err)
		}
		if envelope.Skills == nil {
			t.Fatal("GetSection(disabled sources).Skills = nil")
		}
		if source := findDaemonIntegrationSkillSource(
			envelope.Skills.Sources,
			compozyconfig.SkillSourceAgents,
		); source == nil ||
			source.Enabled {
			t.Fatalf("agents source after disable = %#v, want disabled", source)
		}

		enabledSources := disabledSources
		enabledSources.Sources = []string{compozyconfig.SkillSourceAgents}
		if _, err := runtimeDeps.Settings.ApplySection(
			settingspkg.WithMutationSource(testutil.Context(t), "http"),
			settingspkg.SectionUpdateRequest{
				SectionRequest: settingspkg.SectionRequest{
					Section: settingspkg.SectionSkills,
					Scope:   settingspkg.ScopeUser,
				},
				Skills: &enabledSources,
			},
		); err != nil {
			t.Fatalf("ApplySection(enable agents) error = %v", err)
		}
		waitForCondition(t, "reenabled preset returns to registry", func() bool {
			resolved, err := d.workspaceResolver.Resolve(testutil.Context(t), resolvedWorkspace.ID)
			if err != nil {
				return false
			}
			projectedSkills, err := d.skillsRegistry.ForWorkspace(testutil.Context(t), &resolved)
			return err == nil && findIntegrationSkill(projectedSkills, "watched-preset-skill") != nil
		})
	})
}

func findDaemonIntegrationSkillSource(
	sources []settingspkg.SkillSourceItem,
	slug string,
) *settingspkg.SkillSourceItem {
	for index := range sources {
		if sources[index].Slug == slug {
			return &sources[index]
		}
	}
	return nil
}

func TestRunDreamTickerAndSpawnerIntegration(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Memory.Dream.CheckInterval = 10 * time.Millisecond
	if err := os.WriteFile(
		homePaths.ConfigFile,
		[]byte("[memory]\nenabled = true\n[roles.dream]\nenabled = true\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	workspace := filepath.Join(t.TempDir(), "workspace")
	resolvedWorkspace := seedDaemonWorkspace(t, homePaths, workspace)
	dream := &fakeDreamService{
		shouldRun: true,
		runHook: func(ctx context.Context, spawn memory.SessionSpawner, workspace string) error {
			return spawn(ctx, "memory-consolidation", "integration prompt", workspace, time.Time{})
		},
	}
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{
				ProfileID:   store.DefaultProfileID,
				ID:          "sess-user",
				WorkspaceID: resolvedWorkspace.WorkspaceID,
				Type:        session.SessionTypeUser,
				UpdatedAt:   time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC),
			},
		},
	}

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return sessions, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.newDreamService = func(opts ...memory.Option) consolidation.Service {
		return dream
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	runCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(runCtx)
	}()

	<-d.readyCh
	waitForCondition(t, "integration dream run", func() bool {
		return sessions.createCount() > 0
	})

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := sessions.createCall(0).Type; got != session.SessionTypeDream {
		t.Fatalf("Create() session type = %q, want %q", got, session.SessionTypeDream)
	}
	if got := sessions.createCall(0).Provider; got != "" {
		t.Fatalf("Create() provider = %q, want explicit empty provider", got)
	}
	if got := sessions.createCall(0).Workspace; got != resolvedWorkspace.WorkspaceID {
		t.Fatalf("Create() workspace = %q, want %q", got, resolvedWorkspace.WorkspaceID)
	}
	if got := sessions.createCall(0).WorkspacePath; got != "" {
		t.Fatalf("Create() workspace_path = %q, want empty", got)
	}
	if got := sessions.promptCount(); got == 0 || sessions.promptCall(0).msg != "integration prompt" {
		t.Fatalf("Prompt() calls = %d, want integration prompt", got)
	}
}

func integrationHomePaths(t *testing.T) compozyconfig.HomePaths {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("COMPOZY_HOME", homeDir)
	t.Setenv("HOME", homeDir)

	homePaths, err := compozyconfig.ResolveHomePathsFrom(homeDir)
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	homePaths.DaemonSocket = shortSocketPath(t)
	return homePaths
}

func bootDetachedHarnessIntegrationDaemon(
	t *testing.T,
	homePaths compozyconfig.HomePaths,
	cfg *compozyconfig.Config,
	sessions *fakeSessionManager,
) *Daemon {
	t.Helper()

	if sessions == nil {
		sessions = &fakeSessionManager{}
	}

	daemonInstance, err := New(
		WithHomePaths(homePaths),
		WithConfig(cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	daemonInstance.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return sessions, nil
	}
	daemonInstance.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	daemonInstance.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	daemonInstance.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}
	if err := daemonInstance.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	return daemonInstance
}

func seedDetachedHarnessSessionIndex(
	t *testing.T,
	homePaths compozyconfig.HomePaths,
	infos []*session.Info,
) {
	t.Helper()

	db, err := openDaemonTestGlobalDBAtPath(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	defer func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("GlobalDB.Close() error = %v", err)
		}
	}()

	insertedWorkspaces := make(map[string]struct{})
	for _, info := range infos {
		if info == nil {
			continue
		}

		workspaceID := strings.TrimSpace(info.WorkspaceID)
		if workspaceID == "" {
			workspaceID = "global"
		}
		if _, ok := insertedWorkspaces[workspaceID]; !ok {
			if err := ensureDetachedHarnessWorkspaceIndex(
				t,
				db,
				homePaths,
				workspaceID,
				strings.TrimSpace(info.Workspace),
			); err != nil {
				t.Fatalf("ensureDetachedHarnessWorkspaceIndex(%q) error = %v", workspaceID, err)
			}
			insertedWorkspaces[workspaceID] = struct{}{}
		}

		agentName := strings.TrimSpace(info.AgentName)
		if agentName == "" {
			agentName = "daemon-test-agent"
		}
		if err := db.RegisterSession(testutil.Context(t), store.SessionInfo{
			ProfileID:   info.ProfileID,
			ID:          info.ID,
			Name:        info.Name,
			AgentName:   agentName,
			WorkspaceID: workspaceID,

			SessionType:   string(info.Type),
			State:         string(info.State),
			RuntimeStatus: store.SessionRuntimeUnbound,
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
		}); err != nil {
			t.Fatalf("RegisterSession(%q) error = %v", info.ID, err)
		}
	}
}

func ensureDetachedHarnessWorkspaceIndex(
	t *testing.T,
	db *globaldb.GlobalDB,
	homePaths compozyconfig.HomePaths,
	workspaceID string,
	workspaceRoot string,
) error {
	t.Helper()

	if _, err := db.GetWorkspace(testutil.Context(t), workspaceID); err == nil {
		return nil
	} else if !errors.Is(err, workspacepkg.ErrWorkspaceNotFound) {
		return fmt.Errorf("get workspace %q: %w", workspaceID, err)
	}

	rootDir := strings.TrimSpace(workspaceRoot)
	if rootDir == "" {
		rootDir = filepath.Join(homePaths.HomeDir, workspaceID)
	}
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return fmt.Errorf("mkdir workspace root %q: %w", rootDir, err)
	}
	if err := db.InsertWorkspace(testutil.Context(t), workspacepkg.Workspace{
		ID:        workspaceID,
		Name:      workspaceID,
		RootDir:   rootDir,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("insert workspace %q: %w", workspaceID, err)
	}
	return nil
}

func cloneFakeSessionEvents(source map[string][]store.SessionEvent) map[string][]store.SessionEvent {
	if len(source) == 0 {
		return nil
	}

	cloned := make(map[string][]store.SessionEvent, len(source))
	for sessionID, events := range source {
		cloned[sessionID] = append([]store.SessionEvent(nil), events...)
	}
	return cloned
}

func TestDaemonSessionStopACPHelperProcess(t *testing.T) {
	if os.Getenv(daemonSessionStopHelperEnvKey) != "1" {
		return
	}

	conn := acpsdk.NewAgentSideConnection(daemonSessionStopACPAgent{}, os.Stdout, os.Stdin)
	<-conn.Done()
	os.Exit(0)
}

func seedDaemonWorkspace(t *testing.T, homePaths compozyconfig.HomePaths, root string) workspacepkg.ResolvedWorkspace {
	t.Helper()

	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", root, err)
	}

	registry, err := openDaemonTestGlobalDBAtPath(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	defer func() {
		if err := registry.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	resolver, err := workspacepkg.NewResolver(
		registry,
		workspacepkg.WithHomePaths(homePaths),
		workspacepkg.WithLogger(discardLogger()),
		workspacepkg.WithConfigLoader(func(rootDir string) (compozyconfig.Config, error) {
			return compozyconfig.LoadForHome(homePaths, compozyconfig.WithWorkspaceRoot(rootDir))
		}),
	)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	resolved, err := resolver.ResolveOrRegister(testutil.Context(t), root)
	if err != nil {
		t.Fatalf("ResolveOrRegister(%q) error = %v", root, err)
	}
	return resolved
}

func findAutomationJobByName(jobs []automationpkg.Job, name string) *automationpkg.Job {
	for idx := range jobs {
		if jobs[idx].Name == name {
			return &jobs[idx]
		}
	}
	return nil
}

func findAutomationTriggerByName(triggers []automationpkg.Trigger, name string) *automationpkg.Trigger {
	for idx := range triggers {
		if triggers[idx].Name == name {
			return &triggers[idx]
		}
	}
	return nil
}

func writeDaemonHookScript(t *testing.T, dir string, name string, contents string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	return path
}

func daemonSessionStopHelperCommand(t *testing.T) string {
	t.Helper()

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	return shellquote.Join(
		"env",
		daemonSessionStopHelperEnvKey+"=1",
		bin,
		"-test.run=TestDaemonSessionStopACPHelperProcess",
	)
}

func writeDaemonIntegrationAgentDef(t *testing.T, homePaths compozyconfig.HomePaths, name string, command string) {
	t.Helper()

	path := filepath.Join(homePaths.AgentsDir, name, "AGENT.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}

	content := strings.Join([]string{
		"---",
		"name: " + name,
		"provider: " + acpmock.ProviderName,
		"command: " + command,
		"---",
		"You are a coding assistant.",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func writeDaemonIntegrationProviderConfig(
	t *testing.T,
	homePaths compozyconfig.HomePaths,
	providerName string,
	command string,
) {
	t.Helper()

	content := strings.Join([]string{
		"[defaults]",
		"agent = \"coder\"",
		"provider = " + fmt.Sprintf("%q", providerName),
		"",
		"[providers." + providerName + "]",
		"command = " + fmt.Sprintf("%q", command),
		"harness = \"acp\"",
		"auth_mode = \"none\"",
		"none_security = \"local_transport\"",
		"",
	}, "\n")
	if err := os.WriteFile(homePaths.ConfigFile, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", homePaths.ConfigFile, err)
	}
}

func openDaemonIntegrationGlobalDB(t *testing.T, databasePath string) *globaldb.GlobalDB {
	t.Helper()

	db, err := openDaemonTestGlobalDBAtPath(testutil.Context(t), databasePath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(%q) error = %v", databasePath, err)
	}
	t.Cleanup(func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("GlobalDB.Close() error = %v", err)
		}
	})
	return db
}

func readDaemonInitializeMarkers(t *testing.T, path string) []daemonInitializeMarker {
	t.Helper()

	lines, err := readDaemonMarkerLines(path)
	if err != nil {
		t.Fatalf("readDaemonMarkerLines(%q) error = %v", path, err)
	}

	markers := make([]daemonInitializeMarker, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "shutdown" {
			continue
		}
		var marker daemonInitializeMarker
		if err := json.Unmarshal([]byte(line), &marker); err != nil {
			t.Fatalf("json.Unmarshal(initialize marker) error = %v; line=%q", err, line)
		}
		markers = append(markers, marker)
	}
	return markers
}

func readDaemonMarkerLines(path string) ([]string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(payload)), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		filtered = append(filtered, line)
	}
	return filtered, nil
}

type daemonSessionStopACPAgent struct{}

func (daemonSessionStopACPAgent) Authenticate(
	context.Context,
	acpsdk.AuthenticateRequest,
) (acpsdk.AuthenticateResponse, error) {
	return acpsdk.AuthenticateResponse{}, nil
}

func (daemonSessionStopACPAgent) Logout(context.Context, acpsdk.LogoutRequest) (acpsdk.LogoutResponse, error) {
	return acpsdk.LogoutResponse{}, nil
}

func (daemonSessionStopACPAgent) Initialize(
	context.Context,
	acpsdk.InitializeRequest,
) (acpsdk.InitializeResponse, error) {
	return acpsdk.InitializeResponse{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		AgentCapabilities: acpsdk.AgentCapabilities{
			LoadSession: true,
		},
		AuthMethods: []acpsdk.AuthMethod{},
	}, nil
}

func (daemonSessionStopACPAgent) Cancel(context.Context, acpsdk.CancelNotification) error {
	return nil
}

func (daemonSessionStopACPAgent) CloseSession(
	context.Context,
	acpsdk.CloseSessionRequest,
) (acpsdk.CloseSessionResponse, error) {
	return acpsdk.CloseSessionResponse{}, nil
}

func (daemonSessionStopACPAgent) ListSessions(
	context.Context,
	acpsdk.ListSessionsRequest,
) (acpsdk.ListSessionsResponse, error) {
	return acpsdk.ListSessionsResponse{Sessions: []acpsdk.SessionInfo{}}, nil
}

func (daemonSessionStopACPAgent) NewSession(
	context.Context,
	acpsdk.NewSessionRequest,
) (acpsdk.NewSessionResponse, error) {
	return acpsdk.NewSessionResponse{SessionId: "daemon-stop-helper"}, nil
}

func (daemonSessionStopACPAgent) ResumeSession(
	context.Context,
	acpsdk.ResumeSessionRequest,
) (acpsdk.ResumeSessionResponse, error) {
	return acpsdk.ResumeSessionResponse{}, nil
}

func (daemonSessionStopACPAgent) SetSessionConfigOption(
	context.Context,
	acpsdk.SetSessionConfigOptionRequest,
) (acpsdk.SetSessionConfigOptionResponse, error) {
	return acpsdk.SetSessionConfigOptionResponse{ConfigOptions: []acpsdk.SessionConfigOption{}}, nil
}

func (daemonSessionStopACPAgent) LoadSession(
	context.Context,
	acpsdk.LoadSessionRequest,
) (acpsdk.LoadSessionResponse, error) {
	return acpsdk.LoadSessionResponse{}, nil
}

func (daemonSessionStopACPAgent) Prompt(context.Context, acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonEndTurn}, nil
}

func (daemonSessionStopACPAgent) SetSessionMode(
	context.Context,
	acpsdk.SetSessionModeRequest,
) (acpsdk.SetSessionModeResponse, error) {
	return acpsdk.SetSessionModeResponse{}, nil
}

// assertLifecycleHookPayload checks that dispatched hook payloads retain their workspace and lifecycle identities.
func assertLifecycleHookPayload(
	t *testing.T,
	path string,
	wantEvent hookspkg.HookEvent,
	wantWorkspace workspacepkg.ResolvedWorkspace,
) {
	t.Helper()

	var (
		payloadBytes []byte
		payload      hookspkg.SessionLifecyclePayload
		readOK       bool
		unmarshalOK  bool
	)

	t.Run("Should read file", func(t *testing.T) {
		var err error
		payloadBytes, err = os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", path, err)
		}
		readOK = true
	})

	t.Run("Should unmarshal", func(t *testing.T) {
		if !readOK {
			t.Skip("payload unavailable after read failure")
		}
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", path, err)
		}
		unmarshalOK = true
	})

	t.Run("Should event", func(t *testing.T) {
		if !unmarshalOK {
			t.Skip("payload unavailable after unmarshal failure")
		}
		if payload.Event != wantEvent {
			t.Fatalf("payload.Event = %q, want %q", payload.Event, wantEvent)
		}
	})

	t.Run("Should workspace id", func(t *testing.T) {
		if !unmarshalOK {
			t.Skip("payload unavailable after unmarshal failure")
		}
		if payload.WorkspaceID != wantWorkspace.ID {
			t.Fatalf("payload.WorkspaceID = %q, want %q", payload.WorkspaceID, wantWorkspace.ID)
		}
	})

	t.Run("Should workspace path", func(t *testing.T) {
		if !unmarshalOK {
			t.Skip("payload unavailable after unmarshal failure")
		}
		if payload.Workspace != wantWorkspace.RootDir {
			t.Fatalf("payload.Workspace = %q, want %q", payload.Workspace, wantWorkspace.RootDir)
		}
	})
}

func containsTaskEventType(events []taskpkg.Event, want string) bool {
	for _, event := range events {
		if event.EventType == want {
			return true
		}
	}
	return false
}

func taskEventTypes(events []taskpkg.Event) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.EventType)
	}
	return types
}

func waitForRuntimeCondition(
	t testing.TB,
	label string,
	timeout time.Duration,
	fn func() bool,
) {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		if fn() {
			return
		}
		select {
		case <-timer.C:
			t.Fatalf("timed out waiting for %s", label)
		case <-ticker.C:
		}
	}
}

func copyDirectory(sourceDir string, targetDir string) error {
	return filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("rel %q from %q: %w", path, sourceDir, err)
		}
		targetPath := filepath.Join(targetDir, relativePath)

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat %q: %w", path, err)
		}

		if entry.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}

		bytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %q: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("mkdir %q: %w", filepath.Dir(targetPath), err)
		}
		if err := os.WriteFile(targetPath, bytes, info.Mode().Perm()); err != nil {
			return fmt.Errorf("write %q: %w", targetPath, err)
		}
		return nil
	})
}
