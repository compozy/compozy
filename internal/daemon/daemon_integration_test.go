//go:build integration

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/admission"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/memory/consolidation"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	"github.com/compozy/agh/internal/vault"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/kballard/go-shellquote"
)

const daemonSessionStopHelperEnvKey = "AGH_TEST_DAEMON_SESSION_STOP_HELPER"

type daemonMigrationExpectation struct {
	stream  store.MigrationStream
	version int64
}

func daemonMigrationExpectations() []daemonMigrationExpectation {
	return []daemonMigrationExpectation{
		{stream: globaldb.MigrationStream(), version: 25},
		{stream: memory.MigrationStream(), version: 1},
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

	db, err := globaldb.OpenGlobalDB(testutil.Context(t), databasePath)
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

func (f *fakeNetworkBindableSessionManager) setPrompting(sessionID string, prompting bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if prompting {
		f.prompting[sessionID] = true
		return
	}
	delete(f.prompting, sessionID)
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
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task run")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	taskRecord, err := d.tasks.manager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
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
	run = seedNonLeasedClaimedRunForDaemonTest(t, d.tasks.store, run.ID, actor)
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
	if got, want := daemonTestParticipationFromCreateOpts(createCall), participation.LocalSpec(); got != want {
		t.Fatalf("createCall participation = %#v, want %#v", got, want)
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
			ID:                   "sess-owner-workspace",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
		},
		{
			ID:                   "sess-wake-workspace",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
		},
		{
			ID:                   "sess-owner-global",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			NetworkParticipation: daemonTestLiveParticipation("global", "ops"),
		},
		{
			ID:                   "sess-wake-global",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			NetworkParticipation: daemonTestLiveParticipation("global", "ops"),
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
			TurnSource:     session.TurnSourceNetwork,
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
			TurnSource:     session.TurnSourceNetwork,
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

	readActor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task inspect")
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
			ID:                   "sess-owner",
			AgentName:            "coder",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
		},
		{
			ID:                   "sess-wake",
			AgentName:            "coder",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
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
			ID:                   "sess-owner",
			AgentName:            "coder",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
		},
		{
			ID:                   "sess-wake",
			AgentName:            "coder",
			Type:                 session.SessionTypeUser,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
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
			ID:                   "sess-owner",
			AgentName:            "coder",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
		},
		{
			ID:                   "sess-wake",
			AgentName:            "coder",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
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
			ID:                   "sess-owner",
			AgentName:            "coder",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
		},
		{
			ID:                   "sess-wake",
			AgentName:            "coder",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
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
	runMetadata.Reentry = nil
	run.Metadata, err = marshalDetachedHarnessMetadata(runMetadata)
	if err != nil {
		t.Fatalf("marshalDetachedHarnessMetadata() error = %v", err)
	}
	if err := firstDaemon.tasks.store.UpdateTaskRun(testutil.Context(t), run); err != nil {
		t.Fatalf("UpdateTaskRun() error = %v", err)
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
				ID:                   "sess-owner",
				AgentName:            "coder",
				Type:                 session.SessionTypeSystem,
				State:                session.StateActive,
				WorkspaceID:          workspace.ID,
				Workspace:            workspace.RootDir,
				NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
			},
			{
				ID:                   "sess-wake",
				AgentName:            "coder",
				Type:                 session.SessionTypeSystem,
				State:                session.StateActive,
				WorkspaceID:          workspace.ID,
				Workspace:            workspace.RootDir,
				NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
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
			ID:                   "sess-owner",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
		},
		{
			ID:                   "sess-wake",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
		},
		{
			ID:                   "sess-runtime",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
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
	claimed := seedNonLeasedClaimedRunForDaemonTest(
		t,
		firstDaemon.tasks.store,
		submission.Run.ID,
		detachedActor,
	)
	starting, err := firstDaemon.tasks.manager.AttachRunSession(
		testutil.Context(t),
		claimed.ID,
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
				ID:                   "sess-runtime",
				Type:                 session.SessionTypeSystem,
				State:                session.StateActive,
				WorkspaceID:          workspace.ID,
				Workspace:            workspace.RootDir,
				NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
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

	seedDB, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB(seed) error = %v", err)
	}

	seedManager, err := taskpkg.NewManager(taskpkg.WithStore(seedDB))
	if err != nil {
		t.Fatalf("task.NewManager(seed) error = %v", err)
	}
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task seed")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	createTask := func(title string) taskpkg.Task {
		taskRecord, err := seedManager.CreateTask(testutil.Context(t), taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: title,
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
			ID:              "run-claimed",
			TaskID:          claimedTask.ID,
			Status:          taskpkg.TaskRunStatusClaimed,
			Attempt:         1,
			Origin:          taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task seed"},
			RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: participation.LocalSpec()},
			QueuedAt:        now,
			ClaimedAt:       now.Add(30 * time.Second),
		},
		{
			ID:              "run-starting",
			TaskID:          startingTask.ID,
			Status:          taskpkg.TaskRunStatusStarting,
			Attempt:         1,
			SessionID:       "sess-stopped",
			Origin:          taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task seed"},
			RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: participation.LocalSpec()},
			QueuedAt:        now,
			StartedAt:       now.Add(time.Minute),
		},
		{
			ID:              "run-running",
			TaskID:          runningTask.ID,
			Status:          taskpkg.TaskRunStatusRunning,
			Attempt:         1,
			SessionID:       "sess-missing",
			Origin:          taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task seed"},
			RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: participation.LocalSpec()},
			QueuedAt:        now,
			StartedAt:       now.Add(2 * time.Minute),
		},
	} {
		if err := seedDB.CreateTaskRun(testutil.Context(t), run); err != nil {
			t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
		}
	}

	if err := seedDB.Close(testutil.Context(t)); err != nil {
		t.Fatalf("seedDB.Close() error = %v", err)
	}

	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-stopped", State: session.StateStopped},
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
	if got, want := claimedRun.NetworkSpecSnapshot(), participation.LocalSpec(); got != want {
		t.Fatalf("claimedRun.NetworkSpecSnapshot() = %#v, want %#v", got, want)
	}

	startingRun, err := d.tasks.store.GetTaskRun(testutil.Context(t), "run-starting")
	if err != nil {
		t.Fatalf("GetTaskRun(run-starting) error = %v", err)
	}
	if got, want := startingRun.Status, taskpkg.TaskRunStatusFailed; got != want {
		t.Fatalf("startingRun.Status = %q, want %q", got, want)
	}
	if got, want := startingRun.NetworkSpecSnapshot(), participation.LocalSpec(); got != want {
		t.Fatalf("startingRun.NetworkSpecSnapshot() = %#v, want %#v", got, want)
	}

	runningRun, err := d.tasks.store.GetTaskRun(testutil.Context(t), "run-running")
	if err != nil {
		t.Fatalf("GetTaskRun(run-running) error = %v", err)
	}
	if got, want := runningRun.Status, taskpkg.TaskRunStatusFailed; got != want {
		t.Fatalf("runningRun.Status = %q, want %q", got, want)
	}
	if got, want := runningRun.NetworkSpecSnapshot(), participation.LocalSpec(); got != want {
		t.Fatalf("runningRun.NetworkSpecSnapshot() = %#v, want %#v", got, want)
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
	cfg.Automation.Jobs = []aghconfig.AutomationJob{
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
	cfg.Automation.Triggers = []aghconfig.AutomationTrigger{
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

	db, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
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

func TestBridgeResourceProjectionReconcilesWritesAndBootRebuild(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Automation.Enabled = false
	installExtensionForDaemonIntegration(t, homePaths.DatabaseFile, "ext-bridge", daemonTestExtensionOptions{
		capabilities:      []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		bridgePlatform:    "telegram",
		bridgeDisplayName: "Telegram",
	}, true)

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
		d.newExtensionManager = func(extensionManagerDeps) extensionRuntime {
			return nil
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
	dbSource, ok := first.registry.(extensionDBSource)
	if !ok || dbSource.DB() == nil {
		t.Fatal("first registry does not expose database handle")
	}
	kernel, err := resources.NewKernel(dbSource.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}
	bridgeCodec, err := bridgepkg.NewBridgeInstanceResourceCodec(bridgeProviderLookup(first.bridges))
	if err != nil {
		t.Fatalf("NewBridgeInstanceResourceCodec() error = %v", err)
	}
	bridgeStore, err := resources.NewStore(kernel, bridgeCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(bridge.instance) error = %v", err)
	}

	operator := resourceReconcileActor()
	spec := bridgeResourceIntegrationSpec("Projected Bridge", true)
	record, err := bridgeStore.Put(testutil.Context(t), operator, resources.Draft[bridgepkg.BridgeInstanceSpec]{
		ID:              "brg-resource",
		Scope:           resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		ExpectedVersion: 0,
		Spec:            spec,
	})
	if err != nil {
		t.Fatalf("bridgeStore.Put(create) error = %v", err)
	}
	if err := first.resourceReconcile.Trigger(
		testutil.Context(t),
		bridgepkg.BridgeInstanceResourceKind,
		resources.ReconcileReasonWrite,
	); err != nil {
		t.Fatalf("resourceReconcile.Trigger(create) error = %v", err)
	}
	waitForDaemonBridgeInstance(t, first.bridges, "brg-resource", "Projected Bridge")

	spec.DisplayName = "Updated Bridge"
	record, err = bridgeStore.Put(testutil.Context(t), operator, resources.Draft[bridgepkg.BridgeInstanceSpec]{
		ID:              record.ID,
		Scope:           record.Scope,
		ExpectedVersion: record.Version,
		Spec:            spec,
	})
	if err != nil {
		t.Fatalf("bridgeStore.Put(update) error = %v", err)
	}
	if err := first.resourceReconcile.Trigger(
		testutil.Context(t),
		bridgepkg.BridgeInstanceResourceKind,
		resources.ReconcileReasonWrite,
	); err != nil {
		t.Fatalf("resourceReconcile.Trigger(update) error = %v", err)
	}
	waitForDaemonBridgeInstance(t, first.bridges, "brg-resource", "Updated Bridge")

	if err := bridgeStore.Delete(testutil.Context(t), operator, record.ID, record.Version); err != nil {
		t.Fatalf("bridgeStore.Delete() error = %v", err)
	}
	if err := first.resourceReconcile.Trigger(
		testutil.Context(t),
		bridgepkg.BridgeInstanceResourceKind,
		resources.ReconcileReasonWrite,
	); err != nil {
		t.Fatalf("resourceReconcile.Trigger(delete) error = %v", err)
	}
	waitForDaemonBridgeMissing(t, first.bridges, "brg-resource")

	bootRecord, err := bridgeStore.Put(testutil.Context(t), operator, resources.Draft[bridgepkg.BridgeInstanceSpec]{
		ID:              "brg-boot",
		Scope:           resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		ExpectedVersion: 0,
		Spec:            bridgeResourceIntegrationSpec("Boot Bridge", true),
	})
	if err != nil {
		t.Fatalf("bridgeStore.Put(boot) error = %v", err)
	}
	if err := first.resourceReconcile.Trigger(
		testutil.Context(t),
		bridgepkg.BridgeInstanceResourceKind,
		resources.ReconcileReasonWrite,
	); err != nil {
		t.Fatalf("resourceReconcile.Trigger(boot) error = %v", err)
	}
	waitForDaemonBridgeInstance(t, first.bridges, bootRecord.ID, "Boot Bridge")
	if err := first.registry.(interface {
		DeleteBridgeInstance(context.Context, string) error
	}).DeleteBridgeInstance(testutil.Context(t), bootRecord.ID); err != nil {
		t.Fatalf("DeleteBridgeInstance(cache) error = %v", err)
	}
	waitForDaemonBridgeMissing(t, first.bridges, bootRecord.ID)
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
	waitForDaemonBridgeInstance(t, second.bridges, bootRecord.ID, "Boot Bridge")
}

func TestShutdownCancelsActiveAutomationPrompt(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Automation.Enabled = true
	cfg.Automation.MaxConcurrentJobs = 1
	cfg.Automation.Jobs = []aghconfig.AutomationJob{
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
	cfg.Automation.Jobs = []aghconfig.AutomationJob{
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
func TestBootNetworkEnabledDeliversInboundAndShutsDownCleanly(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Network.Enabled = true

	bindableSessions := newFakeNetworkBindableSessionManager()
	seedNetworkDeliveryIntegrationSessions(t, homePaths, bindableSessions)
	promptStarted := make(chan string, 1)
	bindableSessions.promptNetworkFn = func(ctx context.Context, sessionID string, message string) (<-chan acp.AgentEvent, error) {
		bindableSessions.setPrompting(sessionID, true)
		select {
		case promptStarted <- message:
		default:
		}

		events := make(chan acp.AgentEvent)
		go func() {
			<-ctx.Done()
			bindableSessions.setPrompting(sessionID, false)
			close(events)
		}()
		return events, nil
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
		return bindableSessions, nil
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

	lifecycle := bindableSessions.currentNetworkPeerLifecycle()
	if lifecycle == nil {
		t.Fatal("network lifecycle binding = nil, want boot-time late binding")
	}
	if err := lifecycle.JoinChannel(testutil.Context(t), session.NetworkPeerJoin{
		SessionID:            "sess-net",
		WorkspaceID:          "ws-integration",
		PeerID:               "coder.sess-net",
		Channel:              "builders",
		OwnerKey:             "session:sess-net",
		NetworkParticipation: daemonTestLiveParticipation("ws-integration", "builders"),
	}); err != nil {
		t.Fatalf("JoinChannel() error = %v", err)
	}
	if err := lifecycle.JoinChannel(testutil.Context(t), session.NetworkPeerJoin{
		SessionID:            "sess-sender",
		WorkspaceID:          "ws-integration",
		PeerID:               "coder.sess-sender",
		Channel:              "builders",
		OwnerKey:             "session:sess-sender",
		NetworkParticipation: daemonTestLiveParticipation("ws-integration", "builders"),
	}); err != nil {
		t.Fatalf("JoinChannel(sender) error = %v", err)
	}

	body, err := json.Marshal(map[string]any{"text": "hello from network"})
	if err != nil {
		t.Fatalf("json.Marshal(body) error = %v", err)
	}
	surface := network.SurfaceThread
	threadID := "thread_builders"
	if _, err := d.network.Send(testutil.Context(t), network.SendRequest{
		SessionID: "sess-sender",
		Channel:   "builders",
		Surface:   &surface,
		ThreadID:  &threadID,
		Kind:      network.KindSay,
		Body:      body,
		// Only direct or mentioned say messages are wake-eligible. Mention the
		// recipient so this delivery test exercises the inbound-prompt path.
		Mentions: []string{"coder.sess-net"},
	}); err != nil {
		t.Fatalf("network.Send() error = %v", err)
	}

	select {
	case message := <-promptStarted:
		if !strings.Contains(message, "hello from network") {
			t.Fatalf("prompt message = %q, want network payload preview", message)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for inbound network delivery")
	}

	status, err := d.network.Status(testutil.Context(t))
	if err != nil {
		t.Fatalf("network.Status() error = %v", err)
	}
	if status.LocalPeers != 2 || status.Channels != 1 {
		t.Fatalf("network.Status() = %#v, want 2 local peers and 1 channel", status)
	}

	if err := d.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if _, err := os.Stat(homePaths.DaemonInfo); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("daemon info exists after shutdown: stat error = %v, want os.ErrNotExist", err)
	}
}

func TestBootNetworkShutdownPreservesCommittedDelivery(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Network.Enabled = true

	bindableSessions := newFakeNetworkBindableSessionManager()
	seedNetworkDeliveryIntegrationSessions(t, homePaths, bindableSessions)
	promptStarted := make(chan string, 1)
	bindableSessions.promptNetworkFn = func(ctx context.Context, sessionID string, message string) (<-chan acp.AgentEvent, error) {
		bindableSessions.setPrompting(sessionID, true)
		select {
		case promptStarted <- message:
		default:
		}

		events := make(chan acp.AgentEvent)
		go func() {
			<-ctx.Done()
			bindableSessions.setPrompting(sessionID, false)
			close(events)
		}()
		return events, nil
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
		return bindableSessions, nil
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

	lifecycle := bindableSessions.currentNetworkPeerLifecycle()
	if lifecycle == nil {
		t.Fatal("network lifecycle binding = nil, want boot-time late binding")
	}
	if err := lifecycle.JoinChannel(testutil.Context(t), session.NetworkPeerJoin{
		SessionID:            "sess-net",
		WorkspaceID:          "ws-integration",
		PeerID:               "coder.sess-net",
		Channel:              "builders",
		OwnerKey:             "session:sess-net",
		NetworkParticipation: daemonTestLiveParticipation("ws-integration", "builders"),
	}); err != nil {
		t.Fatalf("JoinChannel() error = %v", err)
	}
	if err := lifecycle.JoinChannel(testutil.Context(t), session.NetworkPeerJoin{
		SessionID:            "sess-sender",
		WorkspaceID:          "ws-integration",
		PeerID:               "coder.sess-sender",
		Channel:              "builders",
		OwnerKey:             "session:sess-sender",
		NetworkParticipation: daemonTestLiveParticipation("ws-integration", "builders"),
	}); err != nil {
		t.Fatalf("JoinChannel(sender) error = %v", err)
	}

	body, err := json.Marshal(map[string]any{"text": "shutdown during delivery"})
	if err != nil {
		t.Fatalf("json.Marshal(body) error = %v", err)
	}
	surface := network.SurfaceThread
	threadID := "thread_builders"
	if _, err := d.network.Send(testutil.Context(t), network.SendRequest{
		SessionID: "sess-sender",
		Channel:   "builders",
		Surface:   &surface,
		ThreadID:  &threadID,
		Kind:      network.KindSay,
		Body:      body,
		// Only direct or mentioned say messages are wake-eligible. Mention the
		// recipient so this delivery test exercises the inbound-prompt path.
		Mentions: []string{"coder.sess-net"},
	}); err != nil {
		t.Fatalf("network.Send() error = %v", err)
	}

	select {
	case message := <-promptStarted:
		if !strings.Contains(message, "shutdown during delivery") {
			t.Fatalf("prompt message = %q, want network payload preview", message)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for inbound network delivery")
	}

	status, err := d.network.Status(testutil.Context(t))
	if err != nil {
		t.Fatalf("network.Status() error = %v", err)
	}
	if status.Status != network.StatusActive || status.LocalPeers != 2 || status.MessagesDelivered != 1 {
		t.Fatalf("network.Status() before shutdown = %#v, want active with 2 Live participants and one committed delivery", status)
	}

	if err := d.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if bindableSessions.IsPrompting("sess-net") {
		t.Fatal("sess-net remains prompting after daemon shutdown")
	}

	db, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB(after shutdown) error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("GlobalDB.Close() error = %v", err)
		}
	})
	messages, err := db.ListConversationMessages(
		testutil.Context(t),
		store.NetworkConversationRef{
			WorkspaceID: "ws-integration",
			Channel:     "builders",
			Surface:     store.NetworkSurfaceThread,
			ThreadID:    "thread_builders",
		},
		store.NetworkConversationMessageQuery{Limit: 10},
	)
	if err != nil {
		t.Fatalf("ListConversationMessages(after shutdown) error = %v", err)
	}
	if len(messages) != 1 || !strings.Contains(string(messages[0].Body), "shutdown during delivery") {
		t.Fatalf("messages after shutdown = %#v, want one committed delivery", messages)
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
			"agh-extension-hook",
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

	payload := hookspkg.SessionPostCreatePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSessionPostCreate,
			Timestamp: time.Now().UTC(),
		},
		SessionContext: hookspkg.SessionContext{
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
			"agh-extension-hook",
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

	if !strings.Contains(logBuffer.String(), "extension manager start failed") {
		t.Fatalf("log output = %q, want extension start failure entry", logBuffer.String())
	}

	payload := hookspkg.SessionPostCreatePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSessionPostCreate,
			Timestamp: time.Now().UTC(),
		},
		SessionContext: hookspkg.SessionContext{
			SessionID: "sess-good",
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
	for _, expectation := range daemonMigrationExpectations() {
		status, err := store.Status(testutil.Context(t), registry.DB(), expectation.stream)
		if err != nil {
			t.Fatalf("Status(%s after boot) error = %v", expectation.stream.Name, err)
		}
		if status.Version != expectation.version || status.AppliedCount != int(expectation.version) {
			t.Fatalf(
				"Status(%s after boot) = %#v, want version %d with %d applied migrations",
				expectation.stream.Name,
				status,
				expectation.version,
				expectation.version,
			)
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
	if _, ok := d.skillsRegistry.Get("agh"); !ok {
		t.Fatal("skills registry does not contain bundled skill agh")
	}

	workspace := workspacepkg.ResolvedWorkspace{
		Agents: []aghconfig.AgentDef{testPromptAgent("Base prompt.")},
	}
	prompt, err := capturedDeps.PromptAssembler.Assemble(
		context.Background(),
		testPromptAgent("Base prompt."),
		&workspace,
	)
	if err != nil {
		t.Fatalf("PromptAssembler.Assemble() error = %v", err)
	}

	assertPromptContainsInOrder(t, prompt, "Base prompt.", "<available-skills>", "agh")
	assertPromptExcludes(t, prompt, "# Persistent Memory")

	t.Run("Should migrate every shared database stream while memory is disabled", func(t *testing.T) {
		if d.memoryStore != nil {
			t.Fatal("boot() initialized the memory runtime while memory is disabled")
		}
		registry, ok := d.registry.(*globaldb.GlobalDB)
		if !ok {
			t.Fatalf("registry type = %T, want *globaldb.GlobalDB", d.registry)
		}
		for _, expectation := range daemonMigrationExpectations() {
			status, statusErr := store.Status(testutil.Context(t), registry.DB(), expectation.stream)
			if statusErr != nil {
				t.Fatalf(
					"Status(%s after memory-disabled boot) error = %v",
					expectation.stream.Name,
					statusErr,
				)
			}
			if status.Version != expectation.version || status.AppliedCount != int(expectation.version) {
				t.Fatalf(
					"Status(%s after memory-disabled boot) = %#v, want version %d with %d applied migrations",
					expectation.stream.Name,
					status,
					expectation.version,
					expectation.version,
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

func TestBootBuildsHooksFromWorkspaceConfigAgentAndSkills(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Memory.Enabled = false
	cfg.Skills.Enabled = true

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, aghconfig.DirName), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Join(workspaceRoot, aghconfig.DirName), err)
	}

	scriptPath := writeDaemonHookScript(t, t.TempDir(), "capture.sh", "#!/bin/sh\ncat > \"$1\"\n")
	configOutput := filepath.Join(t.TempDir(), "config-create.json")
	agentOutput := filepath.Join(t.TempDir(), "agent-stop.json")
	skillOutput := filepath.Join(t.TempDir(), "skill-create.json")

	writeDaemonFile(t, filepath.Join(workspaceRoot, aghconfig.DirName, "config.toml"), `
[[hooks.declarations]]
name = "config-create"
event = "session.post_create"
mode = "sync"
command = "`+scriptPath+`"
args = ["`+configOutput+`"]
`)
	writeDaemonFile(t, filepath.Join(workspaceRoot, aghconfig.DirName, "agents", "coder", "AGENT.md"), `---
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
	writeDaemonFile(t, filepath.Join(workspaceRoot, aghconfig.DirName, "skills", "local-hook", "SKILL.md"), `---
name: local-hook
description: workspace lifecycle hook
metadata:
  agh:
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
		WorkspaceID: resolvedWorkspace.WorkspaceID,
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
}

func TestBootRunsWorkspaceTaskRunHookWithRelativeScriptPath(t *testing.T) {
	t.Run("Should run workspace task-run hook with relative script path", func(t *testing.T) {
		homePaths := integrationHomePaths(t)
		cfg := testConfig(t, homePaths)
		cfg.Memory.Enabled = false
		cfg.Skills.Enabled = false

		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		if err := os.MkdirAll(filepath.Join(workspaceRoot, aghconfig.DirName, "hooks"), 0o755); err != nil {
			t.Fatalf(
				"os.MkdirAll(%q) error = %v",
				filepath.Join(workspaceRoot, aghconfig.DirName, "hooks"),
				err,
			)
		}
		writeDaemonFile(
			t,
			filepath.Join(workspaceRoot, aghconfig.DirName, "hooks", "capture-task-run.sh"),
			"#!/bin/sh\ncat > \"$1\"\n",
		)
		if err := os.Chmod(
			filepath.Join(workspaceRoot, aghconfig.DirName, "hooks", "capture-task-run.sh"),
			0o755,
		); err != nil {
			t.Fatalf("os.Chmod(capture-task-run.sh) error = %v", err)
		}
		writeDaemonFile(t, filepath.Join(workspaceRoot, aghconfig.DirName, "config.toml"), `
[[hooks.declarations]]
name = "workspace-task-run"
event = "task.run.enqueued"
mode = "sync"
command = "/bin/sh"
args = [".agh/hooks/capture-task-run.sh", ".agh/task-run-enqueued.json"]
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
				WorkspaceID: resolvedWorkspace.WorkspaceID,
				ResolvedNetworkParticipation: daemonTestLiveParticipationPtr(
					resolvedWorkspace.WorkspaceID,
					"operations",
				),
				AgentName:  "qa",
				TaskStatus: "ready",
				RunStatus:  "queued",
			},
			IdempotencyKey: "task.start.task-1",
		}

		if _, err := d.hooks.DispatchTaskRunEnqueued(testutil.Context(t), payload); err != nil {
			t.Fatalf("DispatchTaskRunEnqueued() error = %v", err)
		}

		outputPath := filepath.Join(workspaceRoot, aghconfig.DirName, "task-run-enqueued.json")
		body, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", outputPath, err)
		}

		var captured hookspkg.TaskRunEnqueuedPayload
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("json.Unmarshal(task run hook payload) error = %v; body=%s", err, string(body))
		}
		if captured.Event != hookspkg.HookTaskRunEnqueued ||
			captured.WorkspaceID != resolvedWorkspace.WorkspaceID ||
			captured.RunID != "run-1" {
			t.Fatalf("captured payload = %#v, want enqueued payload for the seeded workspace run", captured)
		}
	})
}

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
  agh:
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
		WorkspaceID: resolvedWorkspace.WorkspaceID,
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

		skillRoot := filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.SkillsDirName)
		writeDaemonSkill(t, skillRoot, "watched-workspace-skill", "Workspace watched skill")

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
	})
}

func TestRunDreamTickerAndSpawnerIntegration(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Memory.Dream.CheckInterval = 10 * time.Millisecond

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

func TestBootStartsBridgeExtensionWithBoundRuntime(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	markerPath := filepath.Join(t.TempDir(), "bridge-init.jsonl")
	extensionName := "ext-bridge-daemon"
	instanceID := "brg-daemon-init"
	installExtensionForDaemonIntegration(t, homePaths.DatabaseFile, extensionName, daemonTestExtensionOptions{
		runtimeCommand:    daemonExtensionHelperCommand(t),
		runtimeArgs:       daemonExtensionHelperArgs(),
		runtimeEnv:        daemonExtensionHelperScenarioEnv("record_initialize", markerPath),
		capabilities:      []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		bridgePlatform:    "slack",
		bridgeDisplayName: "Slack",
		actions: []string{
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
			string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		},
		security: []string{"bridge.read", "bridge.write"},
	}, true)

	registry := openDaemonIntegrationGlobalDB(t, homePaths.DatabaseFile)
	instance := seedDaemonBridgeInstanceFixture(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            instanceID,
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "slack",
		ExtensionName: extensionName,
		DisplayName:   "Daemon Bridge",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	if err := registry.PutBridgeSecretBinding(testutil.Context(t), bridgepkg.BridgeSecretBinding{
		BridgeInstanceID: instance.ID,
		BindingName:      "bot_token",
		SecretRef:        "vault:bridges/ext-bridge-daemon/bot-token",
		Kind:             "bot_token",
		CreatedAt:        time.Date(2026, 4, 11, 13, 30, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 4, 11, 13, 30, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("PutBridgeSecretBinding() error = %v", err)
	}

	resolver := &recordingBridgeSecretResolver{
		values: map[string]string{
			"bot_token": "token-daemon",
		},
	}

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
		WithBridgeSecretResolver(resolver),
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

	if d.bridges == nil {
		t.Fatal("boot() did not publish the bridge runtime")
	}

	waitForCondition(t, "bridge initialize marker", func() bool {
		return markerLineCount(markerPath) >= 1
	})

	markers := readDaemonInitializeMarkers(t, markerPath)
	if len(markers) == 0 {
		t.Fatal("initialize markers = empty, want bridge launch handshake")
	}
	request := markers[0].Request
	if !slices.Contains(request.Methods.ExtensionServices, "bridges/deliver") ||
		!slices.Contains(request.Methods.ExtensionServices, "bridges/targets/snapshot") {
		t.Fatalf(
			"initialize extension services = %#v, want bridges/deliver and bridges/targets/snapshot",
			request.Methods.ExtensionServices,
		)
	}
	if request.Runtime.Bridge == nil {
		t.Fatal("initialize runtime bridge = nil, want bound launch payload")
	}
	managed, err := request.Runtime.Bridge.SingleManagedInstance()
	if err != nil {
		t.Fatalf("request.Runtime.Bridge.SingleManagedInstance() error = %v", err)
	}
	if got, want := managed.Instance.ID, instanceID; got != want {
		t.Fatalf("initialize runtime bridge instance id = %q, want %q", got, want)
	}
	if got := managed.BoundSecrets; len(got) != 1 || got[0].BindingName != "bot_token" ||
		got[0].Value != "token-daemon" {
		t.Fatalf("initialize runtime bridge bound secrets = %#v, want resolved bot_token binding", got)
	}
	if len(resolver.calls) != 1 || resolver.calls[0].BridgeInstanceID != instanceID {
		t.Fatalf("ResolveBridgeSecret() calls = %#v, want one call for %q", resolver.calls, instanceID)
	}
}

func TestBootStartsBridgeExtensionWithDefaultVaultSecretResolver(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	markerPath := filepath.Join(t.TempDir(), "bridge-init-default-vault.jsonl")
	extensionName := "ext-bridge-daemon-default-vault"
	instanceID := "brg-daemon-default-vault"
	installExtensionForDaemonIntegration(t, homePaths.DatabaseFile, extensionName, daemonTestExtensionOptions{
		runtimeCommand:    daemonExtensionHelperCommand(t),
		runtimeArgs:       daemonExtensionHelperArgs(),
		runtimeEnv:        daemonExtensionHelperScenarioEnv("record_initialize", markerPath),
		capabilities:      []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		bridgePlatform:    "slack",
		bridgeDisplayName: "Slack",
		actions: []string{
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
			string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		},
		security: []string{"bridge.read", "bridge.write"},
	}, true)

	registry := openDaemonIntegrationGlobalDB(t, homePaths.DatabaseFile)
	instance := seedDaemonBridgeInstanceFixture(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            instanceID,
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "slack",
		ExtensionName: extensionName,
		DisplayName:   "Daemon Bridge Default Vault",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	secretRef := "vault:bridges/" + instance.ID + "/bot_token"
	secretStore, err := vault.NewService(
		registry,
		vault.NewFileKeyProvider(homePaths.HomeDir, nil),
	)
	if err != nil {
		t.Fatalf("vault.NewService() error = %v", err)
	}
	if _, err := secretStore.PutSecret(testutil.Context(t), secretRef, "bot_token", "token-from-vault"); err != nil {
		t.Fatalf("PutSecret(%q) error = %v", secretRef, err)
	}
	if err := registry.PutBridgeSecretBinding(testutil.Context(t), bridgepkg.BridgeSecretBinding{
		BridgeInstanceID: instance.ID,
		BindingName:      "bot_token",
		SecretRef:        secretRef,
		Kind:             "bot_token",
		CreatedAt:        time.Date(2026, 4, 11, 13, 32, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 4, 11, 13, 32, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("PutBridgeSecretBinding() error = %v", err)
	}

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

	waitForCondition(t, "bridge initialize marker", func() bool {
		return markerLineCount(markerPath) >= 1
	})

	markers := readDaemonInitializeMarkers(t, markerPath)
	if len(markers) == 0 {
		t.Fatal("initialize markers = empty, want bridge launch handshake")
	}

	request := markers[0].Request
	if request.Runtime.Bridge == nil {
		t.Fatal("initialize runtime bridge = nil, want bound launch payload")
	}
	managed, err := request.Runtime.Bridge.SingleManagedInstance()
	if err != nil {
		t.Fatalf("request.Runtime.Bridge.SingleManagedInstance() error = %v", err)
	}
	if got, want := managed.BoundSecrets[0].Value, "token-from-vault"; got != want {
		t.Fatalf(
			"initialize runtime bridge bound secrets = %#v, want vault-resolved bot_token binding",
			managed.BoundSecrets,
		)
	}
}

func TestBootFailsWhenDefaultBridgeSecretVaultValueIsMissing(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	markerPath := filepath.Join(t.TempDir(), "bridge-init-missing-vault.jsonl")
	extensionName := "ext-bridge-daemon-missing-vault"
	instanceID := "brg-daemon-missing-vault"
	installExtensionForDaemonIntegration(t, homePaths.DatabaseFile, extensionName, daemonTestExtensionOptions{
		runtimeCommand:    daemonExtensionHelperCommand(t),
		runtimeArgs:       daemonExtensionHelperArgs(),
		runtimeEnv:        daemonExtensionHelperScenarioEnv("record_initialize", markerPath),
		capabilities:      []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		bridgePlatform:    "slack",
		bridgeDisplayName: "Slack",
		actions: []string{
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
			string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		},
		security: []string{"bridge.read", "bridge.write"},
	}, true)

	registry := openDaemonIntegrationGlobalDB(t, homePaths.DatabaseFile)
	instance := seedDaemonBridgeInstanceFixture(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            instanceID,
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "slack",
		ExtensionName: extensionName,
		DisplayName:   "Daemon Bridge Missing Vault",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	if err := registry.PutBridgeSecretBinding(testutil.Context(t), bridgepkg.BridgeSecretBinding{
		BridgeInstanceID: instance.ID,
		BindingName:      "bot_token",
		SecretRef:        "vault:bridges/" + instance.ID + "/bot_token",
		Kind:             "bot_token",
		CreatedAt:        time.Date(2026, 4, 11, 13, 33, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 4, 11, 13, 33, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("PutBridgeSecretBinding() error = %v", err)
	}

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v, want daemon to stay up with extension failure recorded", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	ext, err := d.extensions.Get(extensionName)
	if err != nil {
		t.Fatalf("extensions.Get(%q) error = %v", extensionName, err)
	}
	if ext == nil {
		t.Fatalf("extensions.Get(%q) = nil, want extension snapshot", extensionName)
	}
	if !strings.Contains(ext.Status.LastError, `vault: secret not found`) {
		t.Fatalf("extension last error = %q, want missing vault secret message", ext.Status.LastError)
	}
	if strings.Contains(ext.Status.LastError, errBridgeSecretResolverRequired.Error()) {
		t.Fatalf(
			"extension last error = %q, want missing vault failure instead of missing resolver",
			ext.Status.LastError,
		)
	}
	if ext.Status.Active {
		t.Fatalf("extension active = %v, want false after missing vault secret", ext.Status.Active)
	}
}

func TestBootStartsBridgeExtensionWithMultipleOwnedInstances(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	markerPath := filepath.Join(t.TempDir(), "bridge-init-multi.jsonl")
	extensionName := "ext-bridge-daemon-multi"
	firstID := "brg-daemon-init-a"
	secondID := "brg-daemon-init-b"
	installExtensionForDaemonIntegration(t, homePaths.DatabaseFile, extensionName, daemonTestExtensionOptions{
		runtimeCommand:    daemonExtensionHelperCommand(t),
		runtimeArgs:       daemonExtensionHelperArgs(),
		runtimeEnv:        daemonExtensionHelperScenarioEnv("record_initialize", markerPath),
		capabilities:      []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		bridgePlatform:    "slack",
		bridgeDisplayName: "Slack",
		actions: []string{
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
			string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		},
		security: []string{"bridge.read", "bridge.write"},
	}, true)

	registry := openDaemonIntegrationGlobalDB(t, homePaths.DatabaseFile)
	for _, req := range []bridgepkg.CreateInstanceRequest{
		{
			ID:            firstID,
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: extensionName,
			DisplayName:   "Daemon Bridge A",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusReady,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		},
		{
			ID:            secondID,
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "slack",
			ExtensionName: extensionName,
			DisplayName:   "Daemon Bridge B",
			Enabled:       true,
			Status:        bridgepkg.BridgeStatusDegraded,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		},
	} {
		seedDaemonBridgeInstanceFixture(t, registry, req)
	}
	for _, binding := range []bridgepkg.BridgeSecretBinding{
		{
			BridgeInstanceID: firstID,
			BindingName:      "bot_token",
			SecretRef:        "vault:bridges/ext-bridge-daemon-multi/bot-token",
			Kind:             "bot_token",
			CreatedAt:        time.Date(2026, 4, 11, 13, 35, 0, 0, time.UTC),
			UpdatedAt:        time.Date(2026, 4, 11, 13, 35, 0, 0, time.UTC),
		},
		{
			BridgeInstanceID: secondID,
			BindingName:      "webhook_secret",
			SecretRef:        "vault:bridges/ext-bridge-daemon-multi/webhook-secret",
			Kind:             "webhook_secret",
			CreatedAt:        time.Date(2026, 4, 11, 13, 35, 0, 0, time.UTC),
			UpdatedAt:        time.Date(2026, 4, 11, 13, 35, 0, 0, time.UTC),
		},
	} {
		if err := registry.PutBridgeSecretBinding(testutil.Context(t), binding); err != nil {
			t.Fatalf("PutBridgeSecretBinding(%q) error = %v", binding.BridgeInstanceID, err)
		}
	}

	resolver := &recordingBridgeSecretResolver{
		values: map[string]string{
			"bot_token":      "token-daemon",
			"webhook_secret": "webhook-daemon",
		},
	}

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
		WithBridgeSecretResolver(resolver),
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

	waitForCondition(t, "bridge initialize marker", func() bool {
		return markerLineCount(markerPath) >= 1
	})

	markers := readDaemonInitializeMarkers(t, markerPath)
	if got, want := len(markers), 1; got != want {
		t.Fatalf("len(initialize markers) = %d, want %d", got, want)
	}
	request := markers[0].Request
	if request.Runtime.Bridge == nil {
		t.Fatal("initialize runtime bridge = nil, want bound launch payload")
	}
	if got, want := request.Runtime.Bridge.ManagedBridgeInstanceIDs(), []string{
		firstID,
		secondID,
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("initialize runtime managed ids = %#v, want %#v", got, want)
	}
	firstManaged, ok := request.Runtime.Bridge.ManagedInstance(firstID)
	if !ok {
		t.Fatalf("initialize runtime missing managed instance %q", firstID)
	}
	secondManaged, ok := request.Runtime.Bridge.ManagedInstance(secondID)
	if !ok {
		t.Fatalf("initialize runtime missing managed instance %q", secondID)
	}
	if got, want := firstManaged.BoundSecrets[0].Value, "token-daemon"; got != want {
		t.Fatalf("first managed bound secret value = %q, want %q", got, want)
	}
	if got, want := secondManaged.BoundSecrets[0].Value, "webhook-daemon"; got != want {
		t.Fatalf("second managed bound secret value = %q, want %q", got, want)
	}
	if got, want := len(resolver.calls), 2; got != want {
		t.Fatalf("ResolveBridgeSecret() calls = %#v, want %d calls", resolver.calls, want)
	}
	for _, instanceID := range []string{firstID, secondID} {
		instance, err := d.bridges.GetInstance(testutil.Context(t), instanceID)
		if err != nil {
			t.Fatalf("GetInstance(%q) error = %v", instanceID, err)
		}
		if got, want := instance.Status.Normalize(), bridgepkg.BridgeStatusStarting; got != want {
			t.Fatalf("GetInstance(%q).Status = %q, want %q", instanceID, got, want)
		}
	}
}

func TestCreateEnabledBridgeAfterBootReloadsErroredExtension(t *testing.T) {
	aghExecutable := e2etest.BuildAGHBinary(t)
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	markerPath := filepath.Join(t.TempDir(), "bridge-create.jsonl")
	extensionName := "ext-bridge-create"
	instanceID := "brg-daemon-create"
	installExtensionForDaemonIntegration(t, homePaths.DatabaseFile, extensionName, daemonTestExtensionOptions{
		runtimeCommand:    daemonExtensionHelperCommand(t),
		runtimeArgs:       daemonExtensionHelperArgs(),
		runtimeEnv:        daemonExtensionHelperScenarioEnv("record_initialize", markerPath),
		capabilities:      []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		bridgePlatform:    "slack",
		bridgeDisplayName: "Slack",
		actions: []string{
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
			string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		},
		security: []string{"bridge.read", "bridge.write"},
	}, true)

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.executable = func() (string, error) {
		return aghExecutable, nil
	}
	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	if d.bridges == nil {
		t.Fatal("boot() did not publish the bridge runtime")
	}

	waitForCondition(t, "bridge extension stays registered until an instance exists", func() bool {
		ext, err := d.extensions.Get(extensionName)
		return err == nil && ext != nil && ext.Status.Registered && !ext.Status.Active && ext.Status.LastError == ""
	})
	if got := markerLineCount(markerPath); got != 0 {
		t.Fatalf("initialize marker count before create = %d, want 0", got)
	}

	created, err := d.bridges.CreateInstance(testutil.Context(t), bridgepkg.CreateInstanceRequest{
		ID:            instanceID,
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "slack",
		ExtensionName: extensionName,
		DisplayName:   "Create Bridge",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusStarting,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}
	if created == nil {
		t.Fatal("CreateInstance() = nil, want non-nil")
	}

	waitForCondition(t, "bridge initialize marker after create", func() bool {
		return markerLineCount(markerPath) >= 1
	})
	markers := readDaemonInitializeMarkers(t, markerPath)
	if len(markers) == 0 {
		t.Fatal("initialize markers after create = empty, want launch handshake")
	}
	managed, err := markers[len(markers)-1].Request.Runtime.Bridge.SingleManagedInstance()
	if err != nil {
		t.Fatalf("markers[len(markers)-1].Request.Runtime.Bridge.SingleManagedInstance() error = %v", err)
	}
	if got, want := managed.Instance.ID, instanceID; got != want {
		t.Fatalf("initialize runtime bridge instance id after create = %q, want %q", got, want)
	}

	waitForCondition(t, "bridge extension recovers after create", func() bool {
		ext, err := d.extensions.Get(extensionName)
		return err == nil && ext != nil && ext.Status.Active
	})
}

func TestBridgeRuntimeRestartPreservesRouteContinuity(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	markerPath := filepath.Join(t.TempDir(), "bridge-restart.jsonl")
	extensionName := "ext-bridge-restart"
	instanceID := "brg-daemon-restart"
	installExtensionForDaemonIntegration(t, homePaths.DatabaseFile, extensionName, daemonTestExtensionOptions{
		runtimeCommand:    daemonExtensionHelperCommand(t),
		runtimeArgs:       daemonExtensionHelperArgs(),
		runtimeEnv:        daemonExtensionHelperScenarioEnv("exit_once_record_deliveries", markerPath),
		capabilities:      []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		bridgePlatform:    "slack",
		bridgeDisplayName: "Slack",
		actions: []string{
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
			string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		},
		security: []string{"bridge.read", "bridge.write"},
	}, true)

	registry := openDaemonIntegrationGlobalDB(t, homePaths.DatabaseFile)
	seedDaemonBridgeInstanceFixture(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            instanceID,
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "slack",
		ExtensionName: extensionName,
		DisplayName:   "Restart Bridge",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})
	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	if d.bridges == nil {
		t.Fatal("boot() did not publish the bridge runtime")
	}

	route, err := d.bridges.UpsertRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		Scope:            bridgepkg.ScopeGlobal,
		BridgeInstanceID: instanceID,
		PeerID:           "peer-restart",
		SessionID:        "sess-restart",
		AgentName:        "coder",
		LastActivityAt:   time.Date(2026, 4, 11, 13, 45, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("UpsertRoute() error = %v", err)
	}

	target := bridgepkg.DeliveryTarget{
		BridgeInstanceID: instanceID,
		PeerID:           "peer-restart",
		Mode:             bridgepkg.DeliveryModeDirectSend,
	}
	if _, err := d.bridges.Broker().RegisterPromptDelivery(testutil.Context(t), bridgepkg.PromptDeliveryRegistration{
		SessionID:      "sess-restart",
		TurnID:         "turn-restart",
		ExtensionName:  extensionName,
		DeliveryID:     "del-restart",
		RoutingKey:     route.RoutingKey(),
		DeliveryTarget: target,
	}); err != nil {
		t.Fatalf("RegisterPromptDelivery() error = %v", err)
	}
	if err := d.bridges.Broker().Deliver(testutil.Context(t), bridgepkg.DeliveryEvent{
		DeliveryID:       "del-restart",
		BridgeInstanceID: instanceID,
		RoutingKey:       route.RoutingKey(),
		DeliveryTarget:   target,
		Seq:              1,
		EventType:        bridgepkg.DeliveryEventTypeStart,
		Content:          bridgepkg.MessageContent{Text: "hello"},
	}); err != nil {
		t.Fatalf("Deliver(start) error = %v", err)
	}
	if err := d.bridges.Broker().Deliver(testutil.Context(t), bridgepkg.DeliveryEvent{
		DeliveryID:       "del-restart",
		BridgeInstanceID: instanceID,
		RoutingKey:       route.RoutingKey(),
		DeliveryTarget:   target,
		Seq:              2,
		EventType:        bridgepkg.DeliveryEventTypeFinal,
		Content:          bridgepkg.MessageContent{Text: "hello"},
		Final:            true,
	}); err != nil {
		t.Fatalf("Deliver(final) error = %v", err)
	}

	waitForCondition(t, "bridge delivery resume marker", func() bool {
		payload, err := os.ReadFile(markerPath)
		return err == nil && strings.Contains(string(payload), `"event_type":"resume"`)
	})

	markers := readDaemonDeliveryMarkers(t, markerPath)
	if len(markers) < 2 {
		t.Fatalf("delivery markers = %d, want at least start + resume", len(markers))
	}
	if got := markers[0].Request.Event.EventType; got != bridgepkg.DeliveryEventTypeStart {
		t.Fatalf("first delivery event = %q, want start", got)
	}

	resumeIndex := -1
	for idx, marker := range markers {
		if marker.Request.Event.EventType == bridgepkg.DeliveryEventTypeResume {
			resumeIndex = idx
			break
		}
	}
	if resumeIndex < 0 {
		t.Fatalf("delivery markers = %#v, want resume event", markers)
	}
	if markers[resumeIndex].PID == markers[0].PID {
		t.Fatalf(
			"resume marker pid = %d, want restart to use a different process than %d",
			markers[resumeIndex].PID,
			markers[0].PID,
		)
	}
	if markers[resumeIndex].Request.Snapshot == nil {
		t.Fatal("resume marker snapshot = nil, want resumable state")
	}
	if got, want := markers[resumeIndex].Request.Snapshot.DeliveryID, "del-restart"; got != want {
		t.Fatalf("resume snapshot delivery id = %q, want %q", got, want)
	}

	resolved, err := d.bridges.ResolveRoute(testutil.Context(t), route.RoutingKey())
	if err != nil {
		t.Fatalf("ResolveRoute(after restart) error = %v", err)
	}
	if got, want := resolved.RoutingKeyHash, route.RoutingKeyHash; got != want {
		t.Fatalf("ResolveRoute(after restart).RoutingKeyHash = %q, want %q", got, want)
	}
}

func TestDaemonShutdownClosesBridgeRuntimeCleanly(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	markerPath := filepath.Join(t.TempDir(), "bridge-shutdown.txt")
	extensionName := "ext-bridge-shutdown"
	instanceID := "brg-daemon-shutdown"
	installExtensionForDaemonIntegration(t, homePaths.DatabaseFile, extensionName, daemonTestExtensionOptions{
		runtimeCommand:    daemonExtensionHelperCommand(t),
		runtimeArgs:       daemonExtensionHelperArgs(),
		runtimeEnv:        daemonExtensionHelperScenarioEnv("slow_record_deliveries", markerPath),
		capabilities:      []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		bridgePlatform:    "slack",
		bridgeDisplayName: "Slack",
		actions: []string{
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
			string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		},
		security: []string{"bridge.read", "bridge.write"},
	}, true)

	registry := openDaemonIntegrationGlobalDB(t, homePaths.DatabaseFile)
	seedDaemonBridgeInstanceFixture(t, registry, bridgepkg.CreateInstanceRequest{
		ID:            instanceID,
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "slack",
		ExtensionName: extensionName,
		DisplayName:   "Shutdown Bridge",
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})

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
	if d.bridges == nil {
		t.Fatal("boot() did not publish the bridge runtime")
	}

	route, err := d.bridges.UpsertRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		Scope:            bridgepkg.ScopeGlobal,
		BridgeInstanceID: instanceID,
		PeerID:           "peer-shutdown",
		SessionID:        "sess-shutdown",
		AgentName:        "coder",
		LastActivityAt:   time.Date(2026, 4, 11, 14, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("UpsertRoute() error = %v", err)
	}

	target := bridgepkg.DeliveryTarget{
		BridgeInstanceID: instanceID,
		PeerID:           "peer-shutdown",
		Mode:             bridgepkg.DeliveryModeDirectSend,
	}
	if _, err := d.bridges.Broker().RegisterPromptDelivery(testutil.Context(t), bridgepkg.PromptDeliveryRegistration{
		SessionID:      "sess-shutdown",
		TurnID:         "turn-shutdown",
		ExtensionName:  extensionName,
		DeliveryID:     "del-shutdown",
		RoutingKey:     route.RoutingKey(),
		DeliveryTarget: target,
	}); err != nil {
		t.Fatalf("RegisterPromptDelivery() error = %v", err)
	}
	if err := d.bridges.Broker().Deliver(testutil.Context(t), bridgepkg.DeliveryEvent{
		DeliveryID:       "del-shutdown",
		BridgeInstanceID: instanceID,
		RoutingKey:       route.RoutingKey(),
		DeliveryTarget:   target,
		Seq:              1,
		EventType:        bridgepkg.DeliveryEventTypeStart,
		Content:          bridgepkg.MessageContent{Text: "hello"},
	}); err != nil {
		t.Fatalf("Deliver(start) error = %v", err)
	}

	waitForCondition(t, "bridge delivery started before shutdown", func() bool {
		return markerLineCount(markerPath) >= 1
	})

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := d.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	payload, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", markerPath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(payload)), "\n")
	if got, want := lines[len(lines)-1], "shutdown"; got != want {
		t.Fatalf("shutdown marker final line = %q, want %q", got, want)
	}
}

func integrationHomePaths(t *testing.T) aghconfig.HomePaths {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("AGH_HOME", homeDir)
	t.Setenv("HOME", homeDir)

	homePaths, err := aghconfig.ResolveHomePathsFrom(homeDir)
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	homePaths.DaemonSocket = shortSocketPath(t)
	return homePaths
}

func bootDetachedHarnessIntegrationDaemon(
	t *testing.T,
	homePaths aghconfig.HomePaths,
	cfg *aghconfig.Config,
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
	homePaths aghconfig.HomePaths,
	infos []*session.Info,
) {
	t.Helper()

	db, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
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
			ID:          info.ID,
			Name:        info.Name,
			AgentName:   agentName,
			WorkspaceID: workspaceID,
			SessionNetworkState: &store.SessionNetworkState{
				NetworkSpec: info.NetworkParticipation,
			},
			SessionType: string(info.Type),
			State:       string(info.State),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}); err != nil {
			t.Fatalf("RegisterSession(%q) error = %v", info.ID, err)
		}
	}
}

func seedNetworkDeliveryIntegrationSessions(
	t *testing.T,
	homePaths aghconfig.HomePaths,
	manager *fakeNetworkBindableSessionManager,
) {
	t.Helper()

	workspaceID := "ws-integration"
	workspaceRoot := filepath.Join(homePaths.HomeDir, workspaceID)
	manager.infos = []*session.Info{
		{
			ID:                   "sess-net",
			AgentName:            "coder",
			Type:                 session.SessionTypeUser,
			State:                session.StateActive,
			WorkspaceID:          workspaceID,
			Workspace:            workspaceRoot,
			NetworkParticipation: daemonTestLiveParticipation(workspaceID, "builders"),
		},
		{
			ID:                   "sess-sender",
			AgentName:            "coder",
			Type:                 session.SessionTypeUser,
			State:                session.StateActive,
			WorkspaceID:          workspaceID,
			Workspace:            workspaceRoot,
			NetworkParticipation: daemonTestLiveParticipation(workspaceID, "builders"),
		},
	}
	seedDetachedHarnessSessionIndex(t, homePaths, manager.infos)
}

func ensureDetachedHarnessWorkspaceIndex(
	t *testing.T,
	db *globaldb.GlobalDB,
	homePaths aghconfig.HomePaths,
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

func seedDaemonWorkspace(t *testing.T, homePaths aghconfig.HomePaths, root string) workspacepkg.ResolvedWorkspace {
	t.Helper()

	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", root, err)
	}

	registry, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
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
		workspacepkg.WithConfigLoader(func(rootDir string) (aghconfig.Config, error) {
			return aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(rootDir))
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

func bridgeResourceIntegrationSpec(displayName string, enabled bool) bridgepkg.BridgeInstanceSpec {
	return bridgepkg.BridgeInstanceSpec{
		Scope:            bridgepkg.ScopeGlobal,
		Platform:         "telegram",
		ExtensionName:    "ext-bridge",
		DisplayName:      displayName,
		Source:           bridgepkg.BridgeInstanceSourceDynamic,
		Enabled:          enabled,
		DMPolicy:         bridgepkg.BridgeDMPolicyPairing,
		RoutingPolicy:    bridgepkg.RoutingPolicy{IncludePeer: true},
		ProviderConfig:   []byte(`{"tenant":"acme"}`),
		DeliveryDefaults: []byte(`{"peer_id":"peer-default","mode":"reply"}`),
	}
}

func waitForDaemonBridgeInstance(t *testing.T, runtime *bridgeRuntime, id string, displayName string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		instance, err := runtime.GetInstance(testutil.Context(t), id)
		if err == nil && instance.DisplayName == displayName {
			return
		}
		if time.Now().After(deadline) {
			if err != nil {
				t.Fatalf("GetInstance(%q) did not become available: %v", id, err)
			}
			t.Fatalf("GetInstance(%q).DisplayName did not become %q", id, displayName)
		}
		timer := time.NewTimer(10 * time.Millisecond)
		<-timer.C
	}
}

func waitForDaemonBridgeMissing(t *testing.T, runtime *bridgeRuntime, id string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		_, err := runtime.GetInstance(testutil.Context(t), id)
		if errors.Is(err, bridgepkg.ErrBridgeInstanceNotFound) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("GetInstance(%q) still exists or failed unexpectedly: %v", id, err)
		}
		timer := time.NewTimer(10 * time.Millisecond)
		<-timer.C
	}
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

func writeDaemonIntegrationAgentDef(t *testing.T, homePaths aghconfig.HomePaths, name string, command string) {
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
	homePaths aghconfig.HomePaths,
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

	db, err := globaldb.OpenGlobalDB(testutil.Context(t), databasePath)
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

func seedDaemonBridgeInstanceFixture(
	t *testing.T,
	registry *globaldb.GlobalDB,
	req bridgepkg.CreateInstanceRequest,
) *bridgepkg.BridgeInstance {
	t.Helper()

	if registry == nil {
		t.Fatal("seedDaemonBridgeInstanceFixture() registry = nil")
	}

	instance, err := bridgepkg.NewRegistry(registry).CreateInstance(testutil.Context(t), req)
	if err != nil {
		t.Fatalf("CreateInstance(%q) error = %v", strings.TrimSpace(req.ID), err)
	}

	kernel, err := resources.NewKernel(registry.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}
	codec, err := bridgepkg.NewBridgeInstanceResourceCodec(
		bridgeProviderLookup(newBridgeRuntime(registry, discardLogger(), nil, nil)),
	)
	if err != nil {
		t.Fatalf("NewBridgeInstanceResourceCodec() error = %v", err)
	}
	resourceStore, err := resources.NewStore(kernel, codec)
	if err != nil {
		t.Fatalf("resources.NewStore(bridge.instance) error = %v", err)
	}

	if _, err := resourceStore.Put(
		testutil.Context(t),
		resourceReconcileActor(),
		resources.Draft[bridgepkg.BridgeInstanceSpec]{
			ID:              instance.ID,
			Scope:           bridgepkg.ResourceScopeForBridge(instance.Scope, instance.WorkspaceID),
			ExpectedVersion: 0,
			Spec:            bridgepkg.BridgeInstanceSpecFromInstance(*instance),
		},
	); err != nil {
		t.Fatalf("bridge resource put(%q) error = %v", instance.ID, err)
	}

	return instance
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

func readDaemonDeliveryMarkers(t *testing.T, path string) []daemonDeliveryMarker {
	t.Helper()

	lines, err := readDaemonMarkerLines(path)
	if err != nil {
		t.Fatalf("readDaemonMarkerLines(%q) error = %v", path, err)
	}

	markers := make([]daemonDeliveryMarker, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "shutdown" {
			continue
		}
		var marker daemonDeliveryMarker
		if err := json.Unmarshal([]byte(line), &marker); err != nil {
			t.Fatalf("json.Unmarshal(delivery marker) error = %v; line=%q", err, line)
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
		if payload.WorkspaceID != wantWorkspace.WorkspaceID {
			t.Fatalf("payload.WorkspaceID = %q, want %q", payload.WorkspaceID, wantWorkspace.WorkspaceID)
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
