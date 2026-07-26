package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
	loopdsl "github.com/compozy/agh/internal/loop/dsl"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/procutil"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestLoopActionRuntimeRetriesWorkspaceCapacityDeferral(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 18, 22, 0, 0, 0, time.UTC)
	taskRecord := taskpkg.Task{
		ID:          "task-loop-capacity",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: "workspace-loop-capacity",
	}
	run := taskpkg.Run{
		ID:          "run-loop-capacity",
		TaskID:      taskRecord.ID,
		WorkspaceID: taskRecord.WorkspaceID,
		LoopRunID:   "loop-run-capacity",
		RunKind:     taskpkg.RunKindWorker,
		Status:      taskpkg.TaskRunStatusQueued,
		QueuedAt:    now,
	}
	manager := &loopActionCapacityTestManager{completed: make(chan taskpkg.LeaseCompletion, 1), run: run}
	runner := &loopActionCapacityTestRunner{}
	runtime, err := newLoopActionRuntime(
		manager,
		&loopActionCapacityTestStore{taskRecord: taskRecord, run: run},
		runner,
		nil,
		discardLogger(),
		func() time.Time { return now },
		aghconfig.DefaultTaskActionRunTimeout,
	)
	if err != nil {
		t.Fatalf("newLoopActionRuntime() error = %v", err)
	}
	runtime.claimRetryInterval = time.Millisecond

	runtime.OnTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
		TaskRunContext: hookspkg.TaskRunContext{TaskID: taskRecord.ID, RunID: run.ID},
	})

	select {
	case completion := <-manager.completed:
		if completion.RunID != run.ID {
			t.Fatalf("CompleteRunLease().RunID = %q, want %q", completion.RunID, run.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for deferred loop action to drain")
	}
	if got := manager.claimCalls.Load(); got != 2 {
		t.Fatalf("ClaimNextRun() calls = %d, want 2", got)
	}
	if got := runner.executeCalls.Load(); got != 1 {
		t.Fatalf("ExecuteActionRun() calls = %d, want 1", got)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runtime.shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}

func TestLoopActionRuntimeEnforcesActionLiveness(t *testing.T) {
	t.Parallel()

	t.Run("Should inherit the configured timeout only when the node omits one", func(t *testing.T) {
		t.Parallel()

		runner := &loopActionLivenessTestRunner{}
		runtime := &loopActionRuntime{
			runner:           runner,
			actionRunTimeout: 30 * time.Minute,
		}
		got, err := runtime.actionTimeoutForRun(context.Background(), taskpkg.Run{})
		if err != nil {
			t.Fatalf("actionTimeoutForRun(unset) error = %v", err)
		}
		if got != 30*time.Minute {
			t.Fatalf("actionTimeoutForRun(unset) = %s, want 30m", got)
		}

		runner.timeout = 45 * time.Second
		runner.hasTimeout = true
		got, err = runtime.actionTimeoutForRun(context.Background(), taskpkg.Run{})
		if err != nil {
			t.Fatalf("actionTimeoutForRun(explicit) error = %v", err)
		}
		if got != runner.timeout {
			t.Fatalf("actionTimeoutForRun(explicit) = %s, want %s", got, runner.timeout)
		}

		runner.timeoutErr = fmt.Errorf("invalid node timeout: %w", looppkg.ErrValidation)
		if _, err := runtime.actionTimeoutForRun(context.Background(), taskpkg.Run{}); !errors.Is(
			err,
			looppkg.ErrValidation,
		) {
			t.Fatalf("actionTimeoutForRun(invalid explicit) error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should fail at the absolute default deadline with cumulative usage", func(t *testing.T) {
		t.Parallel()

		manager, taskRecord, run := newLoopActionLivenessTestFixture()
		runner := &loopActionLivenessTestRunner{tokensUsed: 17}
		runtime, err := newLoopActionRuntime(
			manager,
			&loopActionCapacityTestStore{taskRecord: taskRecord, run: run},
			runner,
			nil,
			discardLogger(),
			nil,
			40*time.Millisecond,
		)
		if err != nil {
			t.Fatalf("newLoopActionRuntime() error = %v", err)
		}
		runtime.livenessPollInterval = func(time.Duration) time.Duration { return time.Second }

		err = runtime.executeQueuedRun(context.Background(), taskRecord, run, loopActionRuntimeReasonEnqueued)
		assertLoopActionLivenessFailure(t, manager, err, loopActionReasonNodeTimeout, 17)
	})

	t.Run("Should fail on no progress even while lease heartbeats succeed", func(t *testing.T) {
		t.Parallel()

		manager, taskRecord, run := newLoopActionLivenessTestFixture()
		runner := &loopActionLivenessTestRunner{}
		runtime, err := newLoopActionRuntime(
			manager,
			&loopActionCapacityTestStore{taskRecord: taskRecord, run: run},
			runner,
			nil,
			discardLogger(),
			nil,
			200*time.Millisecond,
		)
		if err != nil {
			t.Fatalf("newLoopActionRuntime() error = %v", err)
		}
		runtime.heartbeatInterval = func(time.Duration) time.Duration { return 5 * time.Millisecond }
		runtime.livenessPollInterval = func(time.Duration) time.Duration { return 5 * time.Millisecond }

		err = runtime.executeQueuedRun(context.Background(), taskRecord, run, loopActionRuntimeReasonEnqueued)
		assertLoopActionLivenessFailure(t, manager, err, loopActionReasonNoProgress, 0)
		if manager.heartbeatCalls.Load() == 0 {
			t.Fatal("HeartbeatRunLease() calls = 0, want successful heartbeats before no-progress failure")
		}
	})

	t.Run("Should treat session activity and an active tool as progress truth", func(t *testing.T) {
		t.Parallel()

		base := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
		current := base
		usage := newLoopActionUsageState(func() time.Time { return current })
		usage.ReportActionSessionBound("sess-active-tool")
		activityAt := base.Add(time.Minute)
		runtime := &loopActionRuntime{sessions: loopActionSessionStatusStub{info: &session.Info{
			Liveness: &store.SessionLivenessMeta{Activity: &store.SessionActivityMeta{
				LastActivityAt: &activityAt,
				CurrentTool:    "agh__task_read",
			}},
		}}}

		activeTool, err := runtime.refreshActionProgress(context.Background(), usage)
		if err != nil {
			t.Fatalf("refreshActionProgress() error = %v", err)
		}
		if !activeTool {
			t.Fatal("refreshActionProgress() activeTool = false, want true")
		}
		if got := usage.snapshot().progressAt; !got.Equal(activityAt) {
			t.Fatalf("progressAt = %s, want %s", got, activityAt)
		}

		current = base.Add(2 * time.Minute)
		usage.ReportActionTokensUsed(9)
		snapshot := usage.snapshot()
		if snapshot.tokensUsed != 9 || !snapshot.progressAt.Equal(current) {
			t.Fatalf("usage snapshot = %#v, want cumulative tokens and advanced progress", snapshot)
		}
	})
}

func newLoopActionLivenessTestFixture() (
	*loopActionLivenessTestManager,
	taskpkg.Task,
	taskpkg.Run,
) {
	now := time.Now().UTC()
	taskRecord := taskpkg.Task{
		ID:          "task-loop-liveness",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: "workspace-loop-liveness",
	}
	run := taskpkg.Run{
		ID:          "run-loop-liveness",
		TaskID:      taskRecord.ID,
		WorkspaceID: taskRecord.WorkspaceID,
		LoopRunID:   "loop-run-liveness",
		RunKind:     taskpkg.RunKindWorker,
		Status:      taskpkg.TaskRunStatusQueued,
		QueuedAt:    now,
	}
	return &loopActionLivenessTestManager{run: run}, taskRecord, run
}

func assertLoopActionLivenessFailure(
	t *testing.T,
	manager *loopActionLivenessTestManager,
	err error,
	wantReason string,
	wantTokens int64,
) {
	t.Helper()
	var reason loopActionReasonCodeProvider
	if !errors.As(err, &reason) || reason.loopActionReasonCode() != wantReason {
		t.Fatalf("executeQueuedRun() error = %v, want reason %q", err, wantReason)
	}
	var metadata loopActionFailureMetadata
	if unmarshalErr := json.Unmarshal(manager.failure.Failure.Metadata, &metadata); unmarshalErr != nil {
		t.Fatalf("unmarshal failure metadata error = %v", unmarshalErr)
	}
	if metadata.ReasonCode != wantReason {
		t.Fatalf("failure reason code = %q, want %q", metadata.ReasonCode, wantReason)
	}
	if manager.failure.TokensUsed != wantTokens {
		t.Fatalf("failure tokens used = %d, want %d", manager.failure.TokensUsed, wantTokens)
	}
}

type loopActionCapacityTestStore struct {
	taskpkg.Store
	taskRecord taskpkg.Task
	run        taskpkg.Run
}

func (s *loopActionCapacityTestStore) GetTask(context.Context, string) (taskpkg.Task, error) {
	return s.taskRecord, nil
}

func (s *loopActionCapacityTestStore) GetTaskRun(context.Context, string) (taskpkg.Run, error) {
	return s.run, nil
}

type loopActionCapacityTestManager struct {
	claimCalls atomic.Int32
	completed  chan taskpkg.LeaseCompletion
	run        taskpkg.Run
}

func (m *loopActionCapacityTestManager) ClaimNextRun(
	context.Context,
	taskpkg.ClaimCriteria,
	taskpkg.ActorContext,
) (*taskpkg.ClaimResult, error) {
	if m.claimCalls.Add(1) == 1 {
		return nil, taskpkg.ErrWorkspaceActiveRunCapReached
	}
	claimed := m.run
	claimed.Status = taskpkg.TaskRunStatusClaimed
	return &taskpkg.ClaimResult{Run: claimed, ClaimToken: "claim-token"}, nil
}

func (m *loopActionCapacityTestManager) HeartbeatRunLease(
	context.Context,
	taskpkg.LeaseHeartbeat,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return &m.run, nil
}

func (m *loopActionCapacityTestManager) CompleteRunLease(
	_ context.Context,
	completion taskpkg.LeaseCompletion,
	_ taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	m.completed <- completion
	return &m.run, nil
}

func (m *loopActionCapacityTestManager) FailRunLease(
	context.Context,
	taskpkg.LeaseFailure,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return &m.run, nil
}

type loopActionCapacityTestRunner struct {
	executeCalls atomic.Int32
}

func (r *loopActionCapacityTestRunner) ExecuteActionRun(
	context.Context,
	taskpkg.Run,
	taskpkg.ActorContext,
) (taskpkg.RunResult, error) {
	r.executeCalls.Add(1)
	return taskpkg.RunResult{Value: json.RawMessage(`{"status":"completed"}`)}, nil
}

func (*loopActionCapacityTestRunner) ActionRunTimeout(
	context.Context,
	taskpkg.Run,
) (time.Duration, bool, error) {
	return 0, false, nil
}

type loopActionLivenessTestManager struct {
	heartbeatCalls atomic.Int32
	failure        taskpkg.LeaseFailure
	run            taskpkg.Run
}

func (m *loopActionLivenessTestManager) ClaimNextRun(
	_ context.Context,
	criteria taskpkg.ClaimCriteria,
	_ taskpkg.ActorContext,
) (*taskpkg.ClaimResult, error) {
	claimed := m.run
	claimed.Status = taskpkg.TaskRunStatusClaimed
	claimed.SessionID = criteria.ClaimerSessionID
	claimed.LeaseUntil = criteria.Now.Add(criteria.LeaseDuration)
	m.run = claimed
	return &taskpkg.ClaimResult{Run: claimed, ClaimToken: "claim-token"}, nil
}

func (m *loopActionLivenessTestManager) HeartbeatRunLease(
	context.Context,
	taskpkg.LeaseHeartbeat,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	m.heartbeatCalls.Add(1)
	return &m.run, nil
}

func (m *loopActionLivenessTestManager) CompleteRunLease(
	context.Context,
	taskpkg.LeaseCompletion,
	taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	return &m.run, nil
}

func (m *loopActionLivenessTestManager) FailRunLease(
	_ context.Context,
	failure taskpkg.LeaseFailure,
	_ taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	m.failure = failure
	return &m.run, nil
}

type loopActionLivenessTestRunner struct {
	timeout    time.Duration
	hasTimeout bool
	timeoutErr error
	tokensUsed int64
}

func (r *loopActionLivenessTestRunner) ExecuteActionRun(
	ctx context.Context,
	_ taskpkg.Run,
	_ taskpkg.ActorContext,
) (taskpkg.RunResult, error) {
	<-ctx.Done()
	return taskpkg.RunResult{TokensUsed: r.tokensUsed}, ctx.Err()
}

func (r *loopActionLivenessTestRunner) ActionRunTimeout(
	context.Context,
	taskpkg.Run,
) (time.Duration, bool, error) {
	return r.timeout, r.hasTimeout, r.timeoutErr
}

type loopActionSessionStatusStub struct {
	info *session.Info
}

func (s loopActionSessionStatusStub) Status(context.Context, string) (*session.Info, error) {
	return s.info, nil
}

func seedNonLeasedClaimedRunForDaemonTest(
	t *testing.T,
	claimStore taskpkg.Store,
	runID string,
	actor taskpkg.ActorContext,
) *taskpkg.Run {
	t.Helper()

	ctx := testutil.Context(t)
	run, err := claimStore.GetTaskRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetTaskRun(%q) error = %v", runID, err)
	}
	run.Status = taskpkg.TaskRunStatusClaimed
	run.ClaimedBy = &taskpkg.ActorIdentity{Kind: actor.Actor.Kind, Ref: actor.Actor.Ref}
	run.ClaimedAt = time.Now().UTC()
	run.SessionID = ""
	run.ClaimTokenHash = ""
	run.LeaseUntil = time.Time{}
	if err := claimStore.UpdateTaskRun(ctx, run); err != nil {
		t.Fatalf("UpdateTaskRun(%q) error = %v", runID, err)
	}
	return &run
}

func networkWakeIntegrationAcceptance(
	t *testing.T,
	now time.Time,
) store.AcceptNetworkMessageRequest {
	t.Helper()

	directID, _, _, err := store.NetworkDirectRoomIdentity(
		"workspace-network",
		"builders",
		"session-sender",
		"session-target",
	)
	if err != nil {
		t.Fatalf("NetworkDirectRoomIdentity() error = %v", err)
	}
	spec := participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     "workspace-network",
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       "builders",
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         1,
			MaxWakeWallTime:  "1s",
			MaxTotalWallTime: "1s",
			MaxInputTokens:   100,
			MaxOutputTokens:  100,
			MaxWakeDepth:     1,
			CoalesceWindow:   "100ms",
		},
	}
	return store.AcceptNetworkMessageRequest{
		Message: store.NetworkConversationMessage{
			MessageID:   "message-network-runner",
			SessionID:   "session-sender",
			WorkspaceID: "workspace-network",
			Channel:     "builders",
			Surface:     store.NetworkSurfaceDirect,
			DirectID:    directID,
			Direction:   "sent",
			PeerFrom:    "session-sender",
			PeerTo:      "session-target",
			Kind:        store.NetworkKindSay,
			Text:        "Review the durable wake",
			PreviewText: "Review the durable wake",
			Body:        []byte(`{"text":"Review the durable wake"}`),
			Timestamp:   now,
		},
		Dispositions: []store.NetworkMessageDisposition{{
			RecipientSessionID: "session-target",
			Decision:           store.NetworkDispositionDeliver,
		}},
		Admissions: []store.NetworkWakeAdmissionInput{{
			WorkspaceID:        "workspace-network",
			RecipientSessionID: "session-target",
			OwnerKey:           "session:session-target",
			Spec:               spec,
			Trigger:            store.NetworkWakeTriggerDirect,
			Eligible:           true,
			Addressed:          true,
			RootID:             "message-network-runner",
			Depth:              0,
			WakeID:             "wake-network-runner",
			TaskRunID:          "run-network-runner",
		}},
	}
}

func TestTaskSessionBridgeStartTaskSessionUsesDedicatedSystemSessions(t *testing.T) {
	t.Parallel()

	globalPath := t.TempDir()
	testCases := []struct {
		name          string
		taskRecord    taskpkg.Task
		run           taskpkg.Run
		wantWorkspace string
		wantPath      string
		wantChannel   string
		wantAgentName string
		wantOwnerKey  string
	}{
		{
			name: "Should use the workspace identifier for workspace-scoped tasks",
			taskRecord: taskpkg.Task{
				ID:          "task-workspace",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-123",
				Title:       "Workspace Task",
				Owner:       &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "frontend-engineer-agent"},
			},
			run: taskpkg.Run{
				ID:       "run-1",
				TaskID:   "task-workspace",
				Status:   taskpkg.TaskRunStatusStarting,
				Attempt:  2,
				Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt: time.Date(2026, 4, 14, 18, 0, 0, 0, time.UTC),
			},
			wantWorkspace: "ws-123",
			wantChannel:   "coord-builders",
			wantAgentName: "frontend-engineer-agent",
			wantOwnerKey:  "task_run:run-1",
		},
		{
			name: "Should use the global workspace path for global tasks",
			taskRecord: taskpkg.Task{
				ID:    "task-global",
				Scope: taskpkg.ScopeGlobal,
				Title: "Global Task",
			},
			run: taskpkg.Run{
				ID:       "run-1",
				TaskID:   "task-global",
				Status:   taskpkg.TaskRunStatusStarting,
				Attempt:  2,
				Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt: time.Date(2026, 4, 14, 18, 0, 0, 0, time.UTC),
			},
			wantPath:     globalPath,
			wantChannel:  "builders",
			wantOwnerKey: "task_run:run-1",
		},
		{
			name: "Should bind a loop-correlated task session to the loop owner",
			taskRecord: taskpkg.Task{
				ID:          "task-loop-worker",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-loop-owner",
				Title:       "Loop Worker Task",
			},
			run: taskpkg.Run{
				ID:        "run-loop-worker",
				TaskID:    "task-loop-worker",
				LoopRunID: "loop-run-owner",
				Status:    taskpkg.TaskRunStatusStarting,
				Attempt:   1,
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "loop-run-owner"},
				QueuedAt:  time.Date(2026, 4, 14, 18, 0, 0, 0, time.UTC),
			},
			wantWorkspace: "ws-loop-owner",
			wantChannel:   "loop-builders",
			wantOwnerKey:  "loop_run:loop-run-owner",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sessions := &fakeSessionManager{}
			bridge, err := newTaskSessionBridge(sessions, globalPath, discardLogger())
			if err != nil {
				t.Fatalf("newTaskSessionBridge() error = %v", err)
			}

			run := tc.run
			run.SetNetworkState(
				daemonTestLiveParticipation(tc.taskRecord.WorkspaceID, tc.wantChannel),
				"",
				"",
				"",
			)
			ref, err := bridge.StartTaskSession(context.Background(), &taskpkg.StartTaskSession{
				Task: tc.taskRecord,
				Run:  run,
			})
			if err != nil {
				t.Fatalf("StartTaskSession() error = %v", err)
			}

			if ref == nil || strings.TrimSpace(ref.SessionID) == "" {
				t.Fatalf("StartTaskSession() ref = %#v, want non-empty session id", ref)
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
			if got, want := participationSnapshotValue(
				createCall.ResolvedNetworkParticipation,
			).ChannelID, tc.wantChannel; got != want {
				t.Fatalf("createCall resolved participation channel = %q, want %q", got, want)
			}
			if got, want := createCall.AgentName, tc.wantAgentName; got != want {
				t.Fatalf("createCall.AgentName = %q, want %q", got, want)
			}
			if got := createCall.NetworkOwnerKey; got != tc.wantOwnerKey {
				t.Fatalf("createCall.NetworkOwnerKey = %q, want %q", got, tc.wantOwnerKey)
			}
			if got, want := createCall.Workspace, tc.wantWorkspace; got != want {
				t.Fatalf("createCall.Workspace = %q, want %q", got, want)
			}
			if got, want := createCall.WorkspacePath, tc.wantPath; got != want {
				t.Fatalf("createCall.WorkspacePath = %q, want %q", got, want)
			}
			if !strings.Contains(createCall.Name, tc.taskRecord.Title) {
				t.Fatalf("createCall.Name = %q, want task title %q", createCall.Name, tc.taskRecord.Title)
			}
		})
	}
}

func TestTaskSessionBridgeStartTaskSessionAppliesExecutionProfileWorkerRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should pass worker agent provider and model to session creation", func(t *testing.T) {
		t.Parallel()

		sessions := &fakeSessionManager{}
		bridge, err := newTaskSessionBridge(sessions, t.TempDir(), discardLogger())
		if err != nil {
			t.Fatalf("newTaskSessionBridge() error = %v", err)
		}

		_, err = bridge.StartTaskSession(context.Background(), &taskpkg.StartTaskSession{
			Task: taskpkg.Task{
				ID:          "task-profile",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-profile",
				Title:       "Profiled Task",
			},
			Run: taskpkg.Run{
				ID:       "run-profile",
				TaskID:   "task-profile",
				Status:   taskpkg.TaskRunStatusStarting,
				Attempt:  1,
				Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt: time.Date(2026, 5, 5, 11, 30, 0, 0, time.UTC),
			},
			ExecutionProfile: &taskpkg.ExecutionProfile{
				TaskID: "task-profile",
				Worker: taskpkg.WorkerProfile{
					Mode:      taskpkg.WorkerModeSelect,
					AgentName: "builder",
					Provider:  "codex",
					Model:     "gpt-5.4",
				},
			},
		})
		if err != nil {
			t.Fatalf("StartTaskSession() error = %v", err)
		}
		createCall := sessions.createCall(0)
		if got, want := createCall.AgentName, "builder"; got != want {
			t.Fatalf("createCall.AgentName = %q, want %q", got, want)
		}
		if got, want := createCall.Provider, "codex"; got != want {
			t.Fatalf("createCall.Provider = %q, want %q", got, want)
		}
		if got, want := createCall.Model, "gpt-5.4"; got != want {
			t.Fatalf("createCall.Model = %q, want %q", got, want)
		}
	})

	t.Run("Should pass sandbox ref selection to session creation", func(t *testing.T) {
		t.Parallel()

		sessions := &fakeSessionManager{}
		bridge, err := newTaskSessionBridge(sessions, t.TempDir(), discardLogger())
		if err != nil {
			t.Fatalf("newTaskSessionBridge() error = %v", err)
		}

		_, err = bridge.StartTaskSession(context.Background(), &taskpkg.StartTaskSession{
			Task: taskpkg.Task{
				ID:          "task-sandbox-ref",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-profile",
				Title:       "Sandbox Ref Task",
			},
			Run: taskpkg.Run{
				ID:       "run-sandbox-ref",
				TaskID:   "task-sandbox-ref",
				Status:   taskpkg.TaskRunStatusStarting,
				Attempt:  1,
				Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt: time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
			},
			ExecutionProfile: &taskpkg.ExecutionProfile{
				TaskID: "task-sandbox-ref",
				Sandbox: taskpkg.SandboxPolicy{
					Mode:       taskpkg.SandboxModeRef,
					SandboxRef: "task-runtime",
				},
			},
		})
		if err != nil {
			t.Fatalf("StartTaskSession() error = %v", err)
		}
		createCall := sessions.createCall(0)
		if got, want := createCall.SandboxRef, "task-runtime"; got != want {
			t.Fatalf("createCall.SandboxRef = %q, want %q", got, want)
		}
		if createCall.DisableSandbox {
			t.Fatal("createCall.DisableSandbox = true, want false")
		}
	})

	t.Run("Should pass no sandbox selection to session creation", func(t *testing.T) {
		t.Parallel()

		sessions := &fakeSessionManager{}
		bridge, err := newTaskSessionBridge(sessions, t.TempDir(), discardLogger())
		if err != nil {
			t.Fatalf("newTaskSessionBridge() error = %v", err)
		}

		_, err = bridge.StartTaskSession(context.Background(), &taskpkg.StartTaskSession{
			Task: taskpkg.Task{
				ID:          "task-sandbox-none",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-profile",
				Title:       "Sandbox None Task",
			},
			Run: taskpkg.Run{
				ID:       "run-sandbox-none",
				TaskID:   "task-sandbox-none",
				Status:   taskpkg.TaskRunStatusStarting,
				Attempt:  1,
				Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt: time.Date(2026, 5, 5, 12, 5, 0, 0, time.UTC),
			},
			ExecutionProfile: &taskpkg.ExecutionProfile{
				TaskID: "task-sandbox-none",
				Sandbox: taskpkg.SandboxPolicy{
					Mode: taskpkg.SandboxModeNone,
				},
			},
		})
		if err != nil {
			t.Fatalf("StartTaskSession() error = %v", err)
		}
		createCall := sessions.createCall(0)
		if !createCall.DisableSandbox {
			t.Fatal("createCall.DisableSandbox = false, want true")
		}
		if got := createCall.SandboxRef; got != "" {
			t.Fatalf("createCall.SandboxRef = %q, want empty", got)
		}
	})

	t.Run("Should grant evidence permissions only with an explicit sandbox ref", func(t *testing.T) {
		t.Parallel()

		sessions := &fakeSessionManager{}
		bridge, err := newTaskSessionBridge(sessions, t.TempDir(), discardLogger())
		if err != nil {
			t.Fatalf("newTaskSessionBridge() error = %v", err)
		}

		_, err = bridge.StartTaskSession(context.Background(), &taskpkg.StartTaskSession{
			Task: taskpkg.Task{
				ID:          "task-evidence-sandbox",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-profile",
				Title:       "Evidence Sandbox Task",
			},
			Run: taskpkg.Run{
				ID:       "run-evidence-sandbox",
				TaskID:   "task-evidence-sandbox",
				Status:   taskpkg.TaskRunStatusStarting,
				Attempt:  1,
				Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt: time.Date(2026, 5, 5, 12, 20, 0, 0, time.UTC),
			},
			ExecutionProfile: &taskpkg.ExecutionProfile{
				TaskID: "task-evidence-sandbox",
				Sandbox: taskpkg.SandboxPolicy{
					Mode:       taskpkg.SandboxModeRef,
					SandboxRef: "evidence-lab",
				},
				Runtime: taskpkg.RuntimePolicy{Mode: taskpkg.RuntimeModeEvidence},
			},
		})
		if err != nil {
			t.Fatalf("StartTaskSession() error = %v", err)
		}
		createCall := sessions.createCall(0)
		if got, want := createCall.Permissions, aghconfig.PermissionModeApproveAll; got != want {
			t.Fatalf("createCall.Permissions = %q, want %q", got, want)
		}
		if got, want := createCall.SandboxRef, "evidence-lab"; got != want {
			t.Fatalf("createCall.SandboxRef = %q, want %q", got, want)
		}
		if !strings.Contains(createCall.PromptOverlay, "Runtime evidence mode is enabled") {
			t.Fatalf("PromptOverlay missing runtime evidence guidance:\n%s", createCall.PromptOverlay)
		}
	})

	t.Run("Should keep configured permissions when evidence runtime does not select a sandbox", func(t *testing.T) {
		t.Parallel()

		sessions := &fakeSessionManager{}
		bridge, err := newTaskSessionBridge(sessions, t.TempDir(), discardLogger())
		if err != nil {
			t.Fatalf("newTaskSessionBridge() error = %v", err)
		}

		_, err = bridge.StartTaskSession(context.Background(), &taskpkg.StartTaskSession{
			Task: taskpkg.Task{
				ID:          "task-evidence-inherit",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-profile",
				Title:       "Evidence Inherit Task",
			},
			Run: taskpkg.Run{
				ID:       "run-evidence-inherit",
				TaskID:   "task-evidence-inherit",
				Status:   taskpkg.TaskRunStatusStarting,
				Attempt:  1,
				Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt: time.Date(2026, 5, 5, 12, 25, 0, 0, time.UTC),
			},
			ExecutionProfile: &taskpkg.ExecutionProfile{
				TaskID:  "task-evidence-inherit",
				Runtime: taskpkg.RuntimePolicy{Mode: taskpkg.RuntimeModeEvidence},
			},
		})
		if err != nil {
			t.Fatalf("StartTaskSession() error = %v", err)
		}
		createCall := sessions.createCall(0)
		if got := createCall.Permissions; got == aghconfig.PermissionModeApproveAll {
			t.Fatalf("createCall.Permissions = %q, want configured permission fallback", got)
		}
		if !strings.Contains(createCall.PromptOverlay, "AGH keeps the configured permission mode") {
			t.Fatalf("PromptOverlay missing permission boundary guidance:\n%s", createCall.PromptOverlay)
		}
	})
}

func TestTaskSessionBridgeStartTaskSessionInjectsTaskContextOverlay(t *testing.T) {
	t.Parallel()

	t.Run("Should include rendered task context in the session prompt overlay", func(t *testing.T) {
		t.Parallel()

		sessions := &fakeSessionManager{}
		overlay := &taskContextOverlayStub{overlay: "task context bundle"}
		bridge, err := newTaskSessionBridge(
			sessions,
			t.TempDir(),
			discardLogger(),
			withTaskSessionContextOverlay(overlay),
		)
		if err != nil {
			t.Fatalf("newTaskSessionBridge() error = %v", err)
		}

		_, err = bridge.StartTaskSession(context.Background(), &taskpkg.StartTaskSession{
			Task: taskpkg.Task{
				ID:          "task-context",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-context",
				Title:       "Context Task",
			},
			Run: taskpkg.Run{
				ID:       "run-context",
				TaskID:   "task-context",
				Status:   taskpkg.TaskRunStatusStarting,
				Attempt:  1,
				Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt: time.Date(2026, 5, 5, 12, 10, 0, 0, time.UTC),
			},
		})
		if err != nil {
			t.Fatalf("StartTaskSession() error = %v", err)
		}
		if got := sessions.createCall(0).PromptOverlay; got != "task context bundle" {
			t.Fatalf("PromptOverlay = %q, want task context bundle", got)
		}
		if len(overlay.calls) != 1 ||
			overlay.calls[0].taskID != "task-context" ||
			overlay.calls[0].runID != "run-context" {
			t.Fatalf("overlay calls = %#v, want task/run context", overlay.calls)
		}
	})

	t.Run("Should fail session start when task context rendering fails", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("render failed")
		bridge, err := newTaskSessionBridge(
			&fakeSessionManager{},
			t.TempDir(),
			discardLogger(),
			withTaskSessionContextOverlay(&taskContextOverlayStub{err: wantErr}),
		)
		if err != nil {
			t.Fatalf("newTaskSessionBridge() error = %v", err)
		}

		_, err = bridge.StartTaskSession(context.Background(), &taskpkg.StartTaskSession{
			Task: taskpkg.Task{
				ID:          "task-context-error",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-context",
				Title:       "Context Task",
			},
			Run: taskpkg.Run{
				ID:       "run-context-error",
				TaskID:   "task-context-error",
				Status:   taskpkg.TaskRunStatusStarting,
				Attempt:  1,
				Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt: time.Date(2026, 5, 5, 12, 15, 0, 0, time.UTC),
			},
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("StartTaskSession() error = %v, want %v", err, wantErr)
		}
	})
}

func TestTaskSessionBridgeAttachTaskSessionRejectsStoppedSessions(t *testing.T) {
	t.Parallel()

	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{
				ID:          "sess-active",
				State:       session.StateActive,
				WorkspaceID: "ws-active",
				CreatedAt:   time.Date(2026, 4, 14, 18, 0, 0, 0, time.UTC),
			},
			{
				ID:          "sess-stopped",
				State:       session.StateStopped,
				WorkspaceID: "ws-stopped",
				CreatedAt:   time.Date(2026, 4, 14, 17, 0, 0, 0, time.UTC),
			},
		},
	}
	bridge, err := newTaskSessionBridge(sessions, t.TempDir(), discardLogger())
	if err != nil {
		t.Fatalf("newTaskSessionBridge() error = %v", err)
	}

	ref, err := bridge.AttachTaskSession(context.Background(), "run-1", "sess-active")
	if err != nil {
		t.Fatalf("AttachTaskSession(active) error = %v", err)
	}
	if got, want := ref.SessionID, "sess-active"; got != want {
		t.Fatalf("AttachTaskSession(active).SessionID = %q, want %q", got, want)
	}

	if _, err := bridge.AttachTaskSession(
		context.Background(),
		"run-1",
		"sess-stopped",
	); !errors.Is(
		err,
		taskpkg.ErrSessionAttachNotAllowed,
	) {
		t.Fatalf("AttachTaskSession(stopped) error = %v, want %v", err, taskpkg.ErrSessionAttachNotAllowed)
	}
}

func TestTaskSessionBridgeStopPathsUseCooperativeThenForcedCalls(t *testing.T) {
	t.Parallel()

	sessions := &fakeSessionManager{}
	bridge, err := newTaskSessionBridge(sessions, t.TempDir(), discardLogger())
	if err != nil {
		t.Fatalf("newTaskSessionBridge() error = %v", err)
	}

	if err := bridge.RequestTaskStop(context.Background(), "sess-1", taskpkg.StopReasonCancellation); err != nil {
		t.Fatalf("RequestTaskStop() error = %v", err)
	}
	if err := bridge.ForceTaskStop(context.Background(), "sess-1", taskpkg.StopReasonCancellation); err != nil {
		t.Fatalf("ForceTaskStop() error = %v", err)
	}

	if got, want := len(sessions.requestStopCalls), 1; got != want {
		t.Fatalf("len(requestStopCalls) = %d, want %d", got, want)
	}
	if got, want := sessions.requestStopCalls[0].cause, session.CauseUserRequested; got != want {
		t.Fatalf("requestStopCalls[0].cause = %v, want %v", got, want)
	}
	if got, want := sessions.requestStopCalls[0].detail, "task cancellation"; got != want {
		t.Fatalf("requestStopCalls[0].detail = %q, want %q", got, want)
	}
	if got, want := len(sessions.stopWithCauseCalls), 1; got != want {
		t.Fatalf("len(stopWithCauseCalls) = %d, want %d", got, want)
	}
}

func TestPlanTaskRunRecoveryClassifiesClaimedStartingRunning(t *testing.T) {
	t.Parallel()

	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-active", State: session.StateActive},
			{ID: "sess-stopping", State: session.StateStopping},
			{ID: "sess-stopped", State: session.StateStopped},
		},
	}

	testCases := []struct {
		name       string
		run        taskpkg.Run
		wantAction taskpkg.RunBootRecoveryAction
		wantState  string
		wantNil    bool
	}{
		{
			name: "Should requeue claimed runs without a bound session",
			run: taskpkg.Run{
				ID:       "run-claimed",
				TaskID:   "task-1",
				Status:   taskpkg.TaskRunStatusClaimed,
				Attempt:  1,
				Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt: time.Now().UTC(),
			},
			wantAction: taskpkg.RunBootRecoveryRequeue,
			wantState:  taskRecoverySessionMissing,
		},
		{
			name: "Should resume starting runs when the bound session is active",
			run: taskpkg.Run{
				ID:        "run-starting",
				TaskID:    "task-2",
				Status:    taskpkg.TaskRunStatusStarting,
				Attempt:   1,
				SessionID: "sess-active",
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt:  time.Now().UTC(),
			},
			wantAction: taskpkg.RunBootRecoveryMarkRunning,
			wantState:  string(session.StateActive),
		},
		{
			name: "Should keep running runs live while the bound session is stopping",
			run: taskpkg.Run{
				ID:        "run-running",
				TaskID:    "task-3",
				Status:    taskpkg.TaskRunStatusRunning,
				Attempt:   1,
				SessionID: "sess-stopping",
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt:  time.Now().UTC(),
			},
			wantNil: true,
		},
		{
			name: "Should fail starting runs when the bound session is stopped",
			run: taskpkg.Run{
				ID:        "run-orphaned-starting",
				TaskID:    "task-4",
				Status:    taskpkg.TaskRunStatusStarting,
				Attempt:   1,
				SessionID: "sess-stopped",
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt:  time.Now().UTC(),
			},
			wantAction: taskpkg.RunBootRecoveryFail,
			wantState:  string(session.StateStopped),
		},
		{
			name: "Should fail running runs when the bound session is missing",
			run: taskpkg.Run{
				ID:        "run-orphaned-running",
				TaskID:    "task-5",
				Status:    taskpkg.TaskRunStatusRunning,
				Attempt:   1,
				SessionID: "sess-missing",
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt:  time.Now().UTC(),
			},
			wantAction: taskpkg.RunBootRecoveryFail,
			wantState:  taskRecoverySessionMissing,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			recovery, err := planTaskRunRecovery(context.Background(), sessions, tc.run)
			if err != nil {
				t.Fatalf("planTaskRunRecovery() error = %v", err)
			}
			if tc.wantNil {
				if recovery != nil {
					t.Fatalf("planTaskRunRecovery() = %#v, want nil", recovery)
				}
				return
			}
			if recovery == nil {
				t.Fatal("planTaskRunRecovery() = nil, want recovery action")
				return
			}
			if got, want := recovery.Action, tc.wantAction; got != want {
				t.Fatalf("recovery.Action = %q, want %q", got, want)
			}
			if got, want := recovery.SessionState, tc.wantState; got != want {
				t.Fatalf("recovery.SessionState = %q, want %q", got, want)
			}
		})
	}
}

func TestPlanTaskRunRecoveryClassifiesCrashedOrphanedAndStalledSessions(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	lastUpdate := now.Add(-session.DefaultLivenessStallAfter - time.Minute)
	startedAt, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		t.Fatalf("procutil.StartedAt(self) error = %v", err)
	}
	mismatchedStartedAt := startedAt.Add(-time.Hour)
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{
				ID:         "sess-crashed",
				State:      session.StateStopped,
				StopReason: store.StopAgentCrashed,
				StopDetail: "daemon crashed while session active",
			},
			{
				ID:    "sess-orphaned",
				State: session.StateStopped,
				Liveness: &store.SessionLivenessMeta{
					SubprocessPID:       os.Getpid(),
					SubprocessStartedAt: &startedAt,
				},
				StopDetail: "daemon exited while session subprocess remained alive",
			},
			{
				ID:    "sess-stalled",
				State: session.StateStopped,
				Liveness: &store.SessionLivenessMeta{
					SubprocessPID:       os.Getpid(),
					SubprocessStartedAt: &startedAt,
					LastUpdateAt:        &lastUpdate,
					StallState:          store.SessionStallStateDetected,
					StallReason:         store.SessionStallReasonActivityTimeout,
				},
				StopDetail: "daemon exited while stalled session subprocess remained alive",
			},
			{
				ID:    "sess-reused-pid",
				State: session.StateStopped,
				Liveness: &store.SessionLivenessMeta{
					SubprocessPID:       os.Getpid(),
					SubprocessStartedAt: &mismatchedStartedAt,
					LastUpdateAt:        &lastUpdate,
				},
				StopDetail: "daemon exited after pid reuse",
			},
		},
	}

	testCases := []struct {
		name               string
		sessionID          string
		wantClassification string
		wantDetail         string
	}{
		{
			name:               "Should classify stopped session without live subprocess as crashed",
			sessionID:          "sess-crashed",
			wantClassification: taskRecoveryClassificationCrashed,
			wantDetail:         "daemon crashed while session active",
		},
		{
			name:               "Should classify stopped session with live subprocess as orphaned",
			sessionID:          "sess-orphaned",
			wantClassification: taskRecoveryClassificationOrphaned,
			wantDetail:         "subprocess pid",
		},
		{
			name:               "Should classify stale stopped session with live subprocess as stalled",
			sessionID:          "sess-stalled",
			wantClassification: taskRecoveryClassificationStalled,
			wantDetail:         store.SessionStallReasonActivityTimeout,
		},
		{
			name:               "Should treat pid reuse with mismatched start time as crashed",
			sessionID:          "sess-reused-pid",
			wantClassification: taskRecoveryClassificationCrashed,
			wantDetail:         "daemon exited after pid reuse",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			recovery, err := planTaskRunRecovery(context.Background(), sessions, taskpkg.Run{
				ID:        "run-" + tc.sessionID,
				TaskID:    "task-" + tc.sessionID,
				Status:    taskpkg.TaskRunStatusRunning,
				Attempt:   1,
				SessionID: tc.sessionID,
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"},
				QueuedAt:  now,
			})
			if err != nil {
				t.Fatalf("planTaskRunRecovery() error = %v", err)
			}
			if recovery == nil {
				t.Fatal("planTaskRunRecovery() = nil, want recovery action")
			}
			if got, want := recovery.Action, taskpkg.RunBootRecoveryFail; got != want {
				t.Fatalf("recovery.Action = %q, want %q", got, want)
			}
			if got, want := recovery.Classification, tc.wantClassification; got != want {
				t.Fatalf("recovery.Classification = %q, want %q", got, want)
			}
			if got := recovery.Detail; !strings.Contains(got, tc.wantDetail) {
				t.Fatalf("recovery.Detail = %q, want substring %q", got, tc.wantDetail)
			}
		})
	}
}

func TestTaskSessionBridgeGuardsAndFallbackStopPaths(t *testing.T) {
	t.Parallel()

	if _, err := newTaskSessionBridge(nil, t.TempDir(), discardLogger()); err == nil {
		t.Fatal("newTaskSessionBridge(nil) error = nil, want validation error")
	}

	bridge, err := newTaskSessionBridge(&fakeSessionManager{}, "", discardLogger())
	if err != nil {
		t.Fatalf("newTaskSessionBridge() error = %v", err)
	}

	if _, err := bridge.StartTaskSession(nilTaskRuntimeContext(), &taskpkg.StartTaskSession{}); err == nil {
		t.Fatal("StartTaskSession(nil ctx) error = nil, want validation error")
	}
	if _, err := bridge.StartTaskSession(context.Background(), &taskpkg.StartTaskSession{
		Task: taskpkg.Task{
			ID:    "task-global",
			Scope: taskpkg.ScopeGlobal,
		},
		Run: taskpkg.Run{
			ID:      "run-global",
			Attempt: 1,
		},
	}); err == nil {
		t.Fatal("StartTaskSession(global without workspace path) error = nil, want validation error")
	}
	if _, err := bridge.StartTaskSession(context.Background(), &taskpkg.StartTaskSession{
		Task: taskpkg.Task{
			ID:    "task-invalid",
			Scope: taskpkg.Scope("invalid"),
		},
		Run: taskpkg.Run{
			ID:      "run-invalid",
			Attempt: 1,
		},
	}); !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf("StartTaskSession(invalid scope) error = %v, want %v", err, taskpkg.ErrValidation)
	}
	if _, err := bridge.AttachTaskSession(nilTaskRuntimeContext(), "run-1", "sess-1"); err == nil {
		t.Fatal("AttachTaskSession(nil ctx) error = nil, want validation error")
	}
	if err := bridge.RequestTaskStop(nilTaskRuntimeContext(), "sess-1", taskpkg.StopReasonCancellation); err == nil {
		t.Fatal("RequestTaskStop(nil ctx) error = nil, want validation error")
	}
	if err := bridge.ForceTaskStop(
		context.Background(),
		"   ",
		taskpkg.StopReasonCancellation,
	); !errors.Is(
		err,
		taskpkg.ErrValidation,
	) {
		t.Fatalf("ForceTaskStop(blank id) error = %v, want %v", err, taskpkg.ErrValidation)
	}

	sessions := &fakeSessionManager{
		requestStopErr: func(string, session.StopCause, string) error {
			return session.ErrSessionNotFound
		},
		stopWithCauseErr: func(string, session.StopCause, string) error {
			return session.ErrSessionNotFound
		},
	}
	bridge, err = newTaskSessionBridge(sessions, t.TempDir(), discardLogger())
	if err != nil {
		t.Fatalf("newTaskSessionBridge() error = %v", err)
	}
	if err := bridge.RequestTaskStop(context.Background(), "sess-missing", taskpkg.StopReasonShutdown); err != nil {
		t.Fatalf("RequestTaskStop(missing) error = %v, want nil", err)
	}
	if err := bridge.ForceTaskStop(context.Background(), "sess-missing", taskpkg.StopReasonOrphanedRun); err != nil {
		t.Fatalf("ForceTaskStop(missing) error = %v, want nil", err)
	}

	stopOnlyBridge, err := newTaskSessionBridge(&taskBridgeStopOnlySessionManager{}, t.TempDir(), discardLogger())
	if err != nil {
		t.Fatalf("newTaskSessionBridge(stop-only) error = %v", err)
	}
	if err := stopOnlyBridge.RequestTaskStop(
		context.Background(),
		"sess-fallback",
		taskpkg.StopReasonShutdown,
	); err != nil {
		t.Fatalf("RequestTaskStop(fallback) error = %v", err)
	}

	stopOnly := stopOnlyBridge.sessions.(*taskBridgeStopOnlySessionManager)
	if got, want := len(stopOnly.stopCalls), 1; got != want {
		t.Fatalf("len(stopCalls) = %d, want %d", got, want)
	}
	if got, want := stopOnly.stopCalls[0].cause, session.CauseShutdown; got != want {
		t.Fatalf("stopCalls[0].cause = %v, want %v", got, want)
	}
	if got, want := stopOnly.stopCalls[0].detail, "task shutdown"; got != want {
		t.Fatalf("stopCalls[0].detail = %q, want %q", got, want)
	}
}

func TestTaskSessionBridgeErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a nil start spec", func(t *testing.T) {
		t.Parallel()

		bridge, err := newTaskSessionBridge(&fakeSessionManager{}, t.TempDir(), discardLogger())
		if err != nil {
			t.Fatalf("newTaskSessionBridge() error = %v", err)
		}

		if _, err := bridge.StartTaskSession(context.Background(), nil); err == nil {
			t.Fatal("StartTaskSession(nil spec) error = nil, want validation error")
		}
	})

	t.Run("Should propagate session creation failures", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("create failed")
		bridge, err := newTaskSessionBridge(
			&taskBridgeCreateErrorSessionManager{err: wantErr},
			t.TempDir(),
			discardLogger(),
		)
		if err != nil {
			t.Fatalf("newTaskSessionBridge() error = %v", err)
		}

		_, err = bridge.StartTaskSession(context.Background(), &taskpkg.StartTaskSession{
			Task: taskpkg.Task{
				ID:          "task-workspace",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-123",
			},
			Run: taskpkg.Run{
				ID:      "run-1",
				Attempt: 1,
			},
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("StartTaskSession(create error) error = %v, want %v", err, wantErr)
		}
	})

	t.Run("Should reject create calls that return a nil session", func(t *testing.T) {
		t.Parallel()

		bridge, err := newTaskSessionBridge(&taskBridgeStopOnlySessionManager{}, t.TempDir(), discardLogger())
		if err != nil {
			t.Fatalf("newTaskSessionBridge() error = %v", err)
		}

		_, err = bridge.StartTaskSession(context.Background(), &taskpkg.StartTaskSession{
			Task: taskpkg.Task{
				ID:          "task-workspace",
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-123",
			},
			Run: taskpkg.Run{
				ID:      "run-1",
				Attempt: 1,
			},
		})
		if !errors.Is(err, taskpkg.ErrValidation) {
			t.Fatalf("StartTaskSession(nil session) error = %v, want %v", err, taskpkg.ErrValidation)
		}
	})

	t.Run("Should reject attach calls when the session metadata is unavailable", func(t *testing.T) {
		t.Parallel()

		bridge, err := newTaskSessionBridge(&taskBridgeNilStatusSessionManager{}, t.TempDir(), discardLogger())
		if err != nil {
			t.Fatalf("newTaskSessionBridge() error = %v", err)
		}

		_, err = bridge.AttachTaskSession(context.Background(), "run-1", "sess-missing")
		if !errors.Is(err, taskpkg.ErrSessionAttachNotAllowed) {
			t.Fatalf("AttachTaskSession(nil status) error = %v, want %v", err, taskpkg.ErrSessionAttachNotAllowed)
		}
	})

	t.Run("Should validate stop requests and propagate non-notfound failures", func(t *testing.T) {
		t.Parallel()

		wantRequestErr := errors.New("request stop failed")
		wantForceErr := errors.New("force stop failed")
		sessions := &fakeSessionManager{
			requestStopErr: func(string, session.StopCause, string) error {
				return wantRequestErr
			},
			stopWithCauseErr: func(string, session.StopCause, string) error {
				return wantForceErr
			},
		}
		bridge, err := newTaskSessionBridge(sessions, t.TempDir(), discardLogger())
		if err != nil {
			t.Fatalf("newTaskSessionBridge() error = %v", err)
		}

		if err := bridge.RequestTaskStop(context.Background(), "   ", taskpkg.StopReasonCancellation); !errors.Is(
			err,
			taskpkg.ErrValidation,
		) {
			t.Fatalf("RequestTaskStop(blank id) error = %v, want %v", err, taskpkg.ErrValidation)
		}
		if err := bridge.RequestTaskStop(
			context.Background(),
			"sess-request",
			taskpkg.StopReasonCancellation,
		); !errors.Is(err, wantRequestErr) {
			t.Fatalf("RequestTaskStop(request failure) error = %v, want %v", err, wantRequestErr)
		}
		if err := bridge.ForceTaskStop(
			context.Background(),
			"sess-force",
			taskpkg.StopReasonCancellation,
		); !errors.Is(err, wantForceErr) {
			t.Fatalf("ForceTaskStop(force failure) error = %v, want %v", err, wantForceErr)
		}
		if err := bridge.ForceTaskStop(
			nilTaskRuntimeContext(),
			"sess-force",
			taskpkg.StopReasonCancellation,
		); err == nil {
			t.Fatal("ForceTaskStop(nil ctx) error = nil, want validation error")
		}
	})
}

func TestBootTasksSkipsMissingPrerequisites(t *testing.T) {
	t.Parallel()

	daemon := &Daemon{
		homePaths: aghconfig.HomePaths{HomeDir: t.TempDir()},
	}

	testCases := []struct {
		name  string
		state *bootState
	}{
		{
			name:  "Should skip when the boot state is nil",
			state: nil,
		},
		{
			name: "Should skip when the registry is missing",
			state: &bootState{
				logger:   discardLogger(),
				sessions: &fakeSessionManager{},
			},
		},
		{
			name: "Should skip when the session manager is missing",
			state: &bootState{
				logger:   discardLogger(),
				registry: openDaemonTestGlobalDB(t),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := daemon.bootTasks(context.Background(), tc.state); err != nil {
				t.Fatalf("bootTasks() error = %v, want nil", err)
			}
		})
	}
}

func TestBootTasksBuildsRuntimeWhenDependenciesAreAvailable(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	homePaths := testHomePaths(t)
	resolver, err := workspacepkg.NewResolver(
		db,
		workspacepkg.WithHomePaths(homePaths),
		workspacepkg.WithLogger(discardLogger()),
		workspacepkg.WithConfigLoader(func(rootDir string) (aghconfig.Config, error) {
			return aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(rootDir))
		}),
	)
	if err != nil {
		t.Fatalf("workspace.NewResolver() error = %v", err)
	}

	daemon := &Daemon{
		homePaths: homePaths,
	}
	state := &bootState{
		cfg: aghconfig.Config{
			Network: aghconfig.DefaultNetworkConfig(),
			Task:    aghconfig.DefaultTaskConfig(),
		},
		logger:   discardLogger(),
		registry: db,
		sessions: &fakeSessionManager{},
		harnessResolver: NewHarnessContextResolver(HarnessRuntimeSignals{
			MemoryPromptSectionEnabled: true,
			SkillsPromptSectionEnabled: true,
			SyntheticTurnsEnabled:      true,
			DetachedTaskRuntimeEnabled: true,
		}),
		workspaceResolver: resolver,
	}

	if err := daemon.bootTasks(testutil.Context(t), state); err != nil {
		t.Fatalf("bootTasks() error = %v", err)
	}
	if state.tasks == nil {
		t.Fatal("bootTasks() did not install a task runtime")
	}
	t.Cleanup(func() {
		if err := state.tasks.shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("task runtime shutdown error = %v", err)
		}
	})
	if state.tasks.manager == nil {
		t.Fatal("bootTasks() task manager = nil, want initialized manager")
	}
	if state.tasks.store == nil {
		t.Fatal("bootTasks() task store = nil, want initialized store")
	}
	if state.tasks.detached == nil {
		t.Fatal("bootTasks() detached harness bridge = nil, want initialized bridge")
	}
	if state.tasks.reentry == nil {
		t.Fatal("bootTasks() harness reentry bridge = nil, want initialized bridge")
	}
	if state.deps.Tasks == nil {
		t.Fatal("bootTasks() runtime deps tasks = nil, want published manager")
	}
}

func TestLoopCoordinatorRunnerShouldPollThroughExtensionRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should wire daemon extension watch poller into the coordinator runner", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		now := time.Now().UTC()
		if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
			ID:        "ws-1",
			Name:      "ws-1",
			RootDir:   t.TempDir(),
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace(ws-1) error = %v", err)
		}
		catalog := newResourceCatalog(looppkg.CloneResourceSpec)
		loopName := "watch-source-daemon"
		watchSpec := testWatchLoopSpec(t, loopName)
		catalog.Replace(1, []resources.Record[looppkg.ResourceSpec]{{
			ID:      loopName,
			Version: 1,
			Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			Spec:    watchSpec,
		}})
		definition, err := daemonLoopDefinitionFromSpec(watchSpec)
		if err != nil {
			t.Fatalf("daemonLoopDefinitionFromSpec() error = %v", err)
		}
		resolved, err := newLoopCompilerFactory(nil)(ctx).Compile(definition)
		if err != nil {
			t.Fatalf("Compile(watch Loop pin) error = %v", err)
		}
		seedRun := looppkg.Run{
			ID:                "looprun-watch-daemon",
			WorkspaceID:       "ws-1",
			LoopName:          loopName,
			Status:            looppkg.StatusRunning,
			ReattemptStrategy: looppkg.ReattemptFailedOnly,
			CreatedAt:         now,
			LastProgressAt:    now.Add(-time.Minute),
			IterationCap:      3,
			BudgetOnExceeded:  loopdsl.BudgetExceededHalt,
			Inputs:            map[string]any{},
		}
		applyResolvedLoopRunPinningForTest(t, &seedRun, now, resolved)
		if _, err := db.CreateLoopRunForStart(ctx, seedRun, loopdsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		claim, err := db.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      "ws-1",
			RunKind:          taskpkg.RunKindCoordinator,
			ClaimerSessionID: "daemon-loop-watch-test",
			ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop"},
			LeaseDuration:    time.Minute,
			Now:              now,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		var polled bool
		extensions := &watchPollerExtensionRuntime{
			poll: func(_ context.Context, req watchpkg.PollRequest) (watchpkg.PollResponse, error) {
				polled = true
				if string(req.Spec) != `{"kind":"reviews","query":"open"}` {
					t.Fatalf("PollRequest.Spec = %s, want watch spec", string(req.Spec))
				}
				return watchpkg.PollResponse{Ready: false, StateDigest: "sha256:daemon"}, nil
			},
		}
		state := &bootState{
			logger:      discardLogger(),
			loopCatalog: catalog,
		}
		runner, err := newBootLoopCoordinatorRunner(db, state, testHomePaths(t))
		if err != nil {
			t.Fatalf("newBootLoopCoordinatorRunner() error = %v", err)
		}
		state.setExtensionRuntime(extensions)
		plan, err := runner.Run(ctx, taskpkg.RunID(claim.Run.ID))
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !polled {
			t.Fatal("extension watch poller was not called")
		}
		if plan.Terminal == nil {
			t.Fatal("Terminal = nil, want watching terminal")
		}
		if got, want := plan.Terminal.Status, string(looppkg.StatusWatching); got != want {
			t.Fatalf("Terminal.Status = %q, want %q", got, want)
		}
	})
}

func TestBootTasksSchedulerStatusUsesDurableStarvationEpisodes(t *testing.T) {
	t.Run("Should count active convergence episodes and honor the configured resume grace", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		homePaths := testHomePaths(t)
		resolver, err := workspacepkg.NewResolver(
			db,
			workspacepkg.WithHomePaths(homePaths),
			workspacepkg.WithLogger(discardLogger()),
			workspacepkg.WithConfigLoader(func(rootDir string) (aghconfig.Config, error) {
				return aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(rootDir))
			}),
		)
		if err != nil {
			t.Fatalf("workspace.NewResolver() error = %v", err)
		}
		workspaceRecord, err := resolver.Register(ctx, workspacepkg.RegisterOptions{
			RootDir: homePaths.HomeDir,
			Name:    "scheduler-status-workspace",
		})
		if err != nil {
			t.Fatalf("workspace.Register() error = %v", err)
		}

		cfg := aghconfig.DefaultWithHome(homePaths)
		cfg.Autonomy.Scheduler.MinQueuedAge = time.Hour
		daemon := &Daemon{homePaths: homePaths}
		state := &bootState{
			cfg:      cfg,
			logger:   discardLogger(),
			registry: db,
			sessions: &fakeSessionManager{},
			harnessResolver: NewHarnessContextResolver(HarnessRuntimeSignals{
				MemoryPromptSectionEnabled: true,
				SkillsPromptSectionEnabled: true,
				SyntheticTurnsEnabled:      true,
				DetachedTaskRuntimeEnabled: true,
			}),
			workspaceResolver: resolver,
		}

		if err := daemon.bootTasks(ctx, state); err != nil {
			t.Fatalf("bootTasks() error = %v", err)
		}
		if state.tasks == nil || state.tasks.manager == nil {
			t.Fatal("bootTasks() did not install a task manager")
		}
		t.Cleanup(func() {
			if err := state.tasks.shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("task runtime shutdown error = %v", err)
			}
		})

		actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
			"scheduler-status-test",
			workspaceRecord.ID,
			taskpkg.OriginKindHTTP,
			"daemon.test",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
		}
		taskRecord, err := state.tasks.manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: workspaceRecord.ID,
			Title:       "Verify scheduler status threshold wiring",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		runRecord, err := state.tasks.manager.EnqueueRun(ctx, taskpkg.EnqueueRun{
			TaskID:         taskRecord.ID,
			IdempotencyKey: "scheduler-status-threshold",
		}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}

		runRecord.QueuedAt = time.Now().Add(-30 * time.Minute).UTC()
		if err := state.tasks.store.UpdateTaskRun(ctx, *runRecord); err != nil {
			t.Fatalf("UpdateTaskRun(30m old) error = %v", err)
		}
		status, err := state.tasks.manager.SchedulerStatus(ctx, actor)
		if err != nil {
			t.Fatalf("SchedulerStatus(30m old) error = %v", err)
		}
		if status.StarvedRunCount != 0 {
			t.Fatalf("StarvedRunCount for 30m old run = %d, want 0 with 1h min_queued_age", status.StarvedRunCount)
		}

		runRecord.QueuedAt = time.Now().Add(-2 * time.Hour).UTC()
		if err := state.tasks.store.UpdateTaskRun(ctx, *runRecord); err != nil {
			t.Fatalf("UpdateTaskRun(2h old) error = %v", err)
		}
		status, err = state.tasks.manager.SchedulerStatus(ctx, actor)
		if err != nil {
			t.Fatalf("SchedulerStatus(2h old) error = %v", err)
		}
		if status.StarvedRunCount != 0 {
			t.Fatalf("StarvedRunCount for age-only 2h run = %d, want 0", status.StarvedRunCount)
		}

		starvationStore, ok := state.tasks.store.(interface {
			UpsertRunStarvation(
				context.Context,
				taskpkg.RunStarvationMutation,
			) (taskpkg.RunStarvation, error)
		})
		if !ok {
			t.Fatal("task store does not expose durable starvation episodes")
		}
		now := time.Now().UTC()
		if _, err := starvationStore.UpsertRunStarvation(ctx, taskpkg.RunStarvationMutation{
			RunID:          runRecord.ID,
			WakeCount:      1,
			FirstStarvedAt: now,
			LastWakeAt:     now,
			UpdatedAt:      now,
		}); err != nil {
			t.Fatalf("UpsertRunStarvation() error = %v", err)
		}
		status, err = state.tasks.manager.SchedulerStatus(ctx, actor)
		if err != nil {
			t.Fatalf("SchedulerStatus(active episode) error = %v", err)
		}
		if status.StarvedRunCount != 1 {
			t.Fatalf("StarvedRunCount for active episode = %d, want 1", status.StarvedRunCount)
		}

		pauseStore, ok := state.tasks.store.(interface {
			SetSchedulerResumed(context.Context) (taskpkg.SchedulerPauseState, error)
		})
		if !ok {
			t.Fatal("task store does not expose scheduler resume state")
		}
		if _, err := pauseStore.SetSchedulerResumed(ctx); err != nil {
			t.Fatalf("SetSchedulerResumed() error = %v", err)
		}
		status, err = state.tasks.manager.SchedulerStatus(ctx, actor)
		if err != nil {
			t.Fatalf("SchedulerStatus(resume grace) error = %v", err)
		}
		if status.StarvedRunCount != 0 {
			t.Fatalf("StarvedRunCount during 1h resume grace = %d, want 0", status.StarvedRunCount)
		}
	})
}

func TestBootHarnessReentryBridgeSkipsUnsupportedRegistryWithoutFailing(t *testing.T) {
	t.Parallel()

	state := &bootState{
		logger:          discardLogger(),
		registry:        nil,
		sessions:        &fakeSessionManager{},
		harnessResolver: NewHarnessContextResolver(HarnessRuntimeSignals{SyntheticTurnsEnabled: true}),
	}

	reentry, err := bootHarnessReentryBridge(testutil.Context(t), state)
	if err != nil {
		t.Fatalf("bootHarnessReentryBridge() error = %v, want nil when reentry support is unavailable", err)
	}
	if reentry != nil {
		t.Fatal("bootHarnessReentryBridge() != nil, want feature downgrade")
	}
}

func TestBootTasksRecoversPendingRunsOnStartup(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	homePaths := testHomePaths(t)
	now := time.Now().UTC()
	if err := db.InsertWorkspace(testutil.Context(t), workspacepkg.Workspace{
		ID: "global", RootDir: homePaths.HomeDir, Name: "global",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace(global) error = %v", err)
	}
	for _, sessionID := range []string{"sess-wake-live", "sess-wake-sender"} {
		if err := db.RegisterSession(testutil.Context(t), store.SessionInfo{
			ID: sessionID, AgentName: "coder", Provider: "test",
			WorkspaceID: "global", State: string(session.StateActive), CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("RegisterSession(%q) error = %v", sessionID, err)
		}
	}
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{
				ID:                   "sess-live",
				Type:                 session.SessionTypeSystem,
				State:                session.StateActive,
				WorkspaceID:          "global",
				Workspace:            homePaths.HomeDir,
				NetworkParticipation: daemonTestLiveParticipation("global", "builders"),
			},
		},
	}
	sessionBridge, err := newTaskSessionBridge(sessions, homePaths.HomeDir, discardLogger())
	if err != nil {
		t.Fatalf("newTaskSessionBridge() error = %v", err)
	}
	seedManager, err := taskpkg.NewManager(
		taskpkg.WithStore(db),
		taskpkg.WithSessionExecutor(sessionBridge),
		taskpkg.WithCancelGracePeriod(defaultTaskCancelGrace),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	seedActor, err := taskpkg.DeriveDaemonActorContext("boot-seed", "daemon.boot.seed")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext(seed) error = %v", err)
	}
	taskRecord, err := seedManager.CreateTask(context.Background(), taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Recover boot task",
	}, seedActor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	runRecord, err := seedManager.EnqueueRun(context.Background(), taskpkg.EnqueueRun{
		TaskID:         taskRecord.ID,
		IdempotencyKey: "enqueue-boot-recovery",
	}, seedActor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	claimedRun := seedNonLeasedClaimedRunForDaemonTest(t, db, runRecord.ID, seedActor)
	if _, err := seedManager.AttachRunSession(context.Background(), claimedRun.ID, "sess-live", seedActor); err != nil {
		t.Fatalf("AttachRunSession() error = %v", err)
	}
	const wakeRunID = "run-wake-boot-recovery"
	acceptance := networkWakeIntegrationAcceptance(t, now)
	acceptance.Message.MessageID = "message-wake-boot-recovery"
	acceptance.Message.SessionID = "sess-wake-sender"
	acceptance.Message.WorkspaceID = "global"
	acceptance.Message.PeerFrom = "sess-wake-sender"
	acceptance.Message.PeerTo = "sess-wake-live"
	directID, _, _, err := store.NetworkDirectRoomIdentity(
		"global",
		acceptance.Message.Channel,
		"sess-wake-sender",
		"sess-wake-live",
	)
	if err != nil {
		t.Fatalf("NetworkDirectRoomIdentity() error = %v", err)
	}
	acceptance.Message.DirectID = directID
	acceptance.Dispositions[0].RecipientSessionID = "sess-wake-live"
	acceptance.Admissions[0].WorkspaceID = "global"
	acceptance.Admissions[0].RecipientSessionID = "sess-wake-live"
	acceptance.Admissions[0].OwnerKey = "session:sess-wake-live"
	acceptance.Admissions[0].Spec.WorkspaceID = "global"
	acceptance.Admissions[0].Spec.Bounds.MaxWakes = 2
	acceptance.Admissions[0].Spec.Bounds.MaxTotalWallTime = "2s"
	acceptance.Admissions[0].WakeID = "wake-boot-recovery"
	acceptance.Admissions[0].TaskRunID = wakeRunID
	wantWakeParticipation := acceptance.Admissions[0].Spec
	acceptedWake, err := db.AcceptNetworkMessage(context.Background(), acceptance)
	if err != nil {
		t.Fatalf("AcceptNetworkMessage(network wake) error = %v", err)
	}
	if got, want := len(acceptedWake.Notify), 1; got != want {
		t.Fatalf("len(AcceptNetworkMessage(network wake).Notify) = %d, want %d", got, want)
	}
	wakeActor, err := taskpkg.DeriveAgentSessionActorContext("sess-wake-live", "global")
	if err != nil {
		t.Fatalf("DeriveAgentSessionActorContext(network wake) error = %v", err)
	}
	wakeClaim, err := seedManager.ClaimNextRun(context.Background(), taskpkg.ClaimCriteria{
		RunID: wakeRunID, RunKind: taskpkg.RunKindNetworkWake,
		Scope: taskpkg.ScopeWorkspace, WorkspaceID: "global",
		TargetSessionID: "sess-wake-live", ClaimerSessionID: "sess-wake-live",
	}, wakeActor)
	if err != nil {
		t.Fatalf("ClaimNextRun(network wake) error = %v", err)
	}
	runningWake := wakeClaim.Run
	runningWake.Status = taskpkg.TaskRunStatusRunning
	runningWake.SessionID = "sess-wake-live"
	runningWake.StartedAt = now.Add(time.Millisecond)
	if err := db.UpdateTaskRun(context.Background(), runningWake); err != nil {
		t.Fatalf("UpdateTaskRun(running network wake) error = %v", err)
	}

	daemon := &Daemon{
		homePaths: homePaths,
	}
	state := &bootState{
		cfg: aghconfig.Config{
			Network: aghconfig.DefaultNetworkConfig(),
			Task:    aghconfig.DefaultTaskConfig(),
		},
		logger:   discardLogger(),
		registry: db,
		sessions: sessions,
		harnessResolver: NewHarnessContextResolver(HarnessRuntimeSignals{
			MemoryPromptSectionEnabled: true,
			SkillsPromptSectionEnabled: true,
			SyntheticTurnsEnabled:      true,
			DetachedTaskRuntimeEnabled: true,
		}),
	}

	if err := daemon.bootTasks(testutil.Context(t), state); err != nil {
		t.Fatalf("bootTasks() error = %v", err)
	}
	if state.tasks == nil {
		t.Fatal("bootTasks() did not install a task runtime")
	}
	t.Cleanup(func() {
		if err := state.tasks.shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("task runtime shutdown error = %v", err)
		}
	})

	recoveredRun, err := db.GetTaskRun(context.Background(), runRecord.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(recovered) error = %v", err)
	}
	if got, want := recoveredRun.Status, taskpkg.TaskRunStatusRunning; got != want {
		t.Fatalf("recovered run status = %q, want %q", got, want)
	}
	recoveredWake, err := db.GetTaskRun(context.Background(), wakeRunID)
	if err != nil {
		t.Fatalf("GetTaskRun(recovered network wake) error = %v", err)
	}
	if recoveredWake.Status != taskpkg.TaskRunStatusQueued || recoveredWake.TaskID != "" ||
		recoveredWake.SessionID != "" || recoveredWake.ClaimTokenHash != "" {
		t.Fatalf("recovered network wake = %#v, want taskless queued run", recoveredWake)
	}
	if got := recoveredWake.NetworkSpecSnapshot(); got != wantWakeParticipation {
		t.Fatalf("recovered network wake participation = %#v, want %#v", got, wantWakeParticipation)
	}
	wakeID, targetSessionID, ownerKey := recoveredWake.NetworkWakeCorrelation()
	if wakeID != "wake-boot-recovery" || targetSessionID != "sess-wake-live" || ownerKey != "session:sess-wake-live" {
		t.Fatalf(
			"recovered network wake correlation = (%q, %q, %q), want exact admission tuple",
			wakeID,
			targetSessionID,
			ownerKey,
		)
	}

	missingRecipientPrompter := &networkWakePrompterStub{
		resume: func(context.Context, string, int) (*session.Session, error) {
			return nil, session.ErrSessionNotFound
		},
	}
	wakeRunner, err := newNetworkWakeRunner(
		state.tasks.manager,
		missingRecipientPrompter,
		db,
		nil,
		1,
	)
	if err != nil {
		t.Fatalf("newNetworkWakeRunner() error = %v", err)
	}
	if _, err := wakeRunner.processNotification(
		context.Background(),
		wakeActor,
		acceptedWake.Notify[0],
	); !errors.Is(
		err,
		session.ErrSessionNotFound,
	) {
		t.Fatalf("processNotification(missing recipient) error = %v, want %v", err, session.ErrSessionNotFound)
	}
	settledWake, err := db.GetTaskRun(context.Background(), wakeRunID)
	if err != nil {
		t.Fatalf("GetTaskRun(settled network wake) error = %v", err)
	}
	if settledWake.Status != taskpkg.TaskRunStatusFailed || !settledWake.LeaseUntil.IsZero() {
		t.Fatalf("settled network wake = %#v, want failed terminal run with no active lease", settledWake)
	}

	followUp := acceptance
	followUp.Message.MessageID = "message-wake-after-boot-recovery"
	followUp.Message.Text = "Verify the recovered wake released its reservation"
	followUp.Message.PreviewText = followUp.Message.Text
	followUp.Message.Body = []byte(`{"text":"Verify the recovered wake released its reservation"}`)
	followUp.Message.Timestamp = now.Add(time.Second)
	followUp.Admissions[0].RootID = followUp.Message.MessageID
	followUp.Admissions[0].WakeID = "wake-after-boot-recovery"
	followUp.Admissions[0].TaskRunID = "run-wake-after-boot-recovery"
	followUpResult, err := db.AcceptNetworkMessage(context.Background(), followUp)
	if err != nil {
		t.Fatalf("AcceptNetworkMessage(after boot recovery) error = %v", err)
	}
	if got := len(followUpResult.Admitted); got != 0 {
		t.Fatalf("len(AcceptNetworkMessage(after boot recovery).Admitted) = %d, want 0", got)
	}
	if got := followUpResult.Skipped; len(got) != 1 || got[0].Reason != store.NetworkWakeSkipBudgetExhausted {
		t.Fatalf("AcceptNetworkMessage(after boot recovery).Skipped = %#v, want conservative budget exhaustion", got)
	}
}

func TestBootTasksRequiresHarnessResolver(t *testing.T) {
	t.Parallel()

	daemon := &Daemon{
		homePaths: aghconfig.HomePaths{HomeDir: t.TempDir()},
	}
	state := &bootState{
		logger:   discardLogger(),
		registry: openDaemonTestGlobalDB(t),
		sessions: &fakeSessionManager{},
	}

	err := daemon.bootTasks(testutil.Context(t), state)
	if err == nil {
		t.Fatal("bootTasks() error = nil, want harness resolver validation error")
	}
	if !strings.Contains(err.Error(), "harness resolver") {
		t.Fatalf("bootTasks() error = %v, want harness resolver detail", err)
	}
}

func TestTaskRuntimeHelpers(t *testing.T) {
	t.Parallel()

	if got, want := taskSessionName(&taskpkg.StartTaskSession{
		Task: taskpkg.Task{
			Identifier: "build-index",
		},
		Run: taskpkg.Run{
			ID:      "run-identifier",
			Attempt: 3,
		},
	}), "task:build-index#3"; got != want {
		t.Fatalf("taskSessionName(identifier) = %q, want %q", got, want)
	}
	if got, want := taskSessionName(&taskpkg.StartTaskSession{
		Run: taskpkg.Run{
			ID:      "run-fallback",
			Attempt: 4,
		},
	}), "task:run-fallback#4"; got != want {
		t.Fatalf("taskSessionName(run fallback) = %q, want %q", got, want)
	}

	if got, want := taskStopCause(taskpkg.StopReasonShutdown), session.CauseShutdown; got != want {
		t.Fatalf("taskStopCause(shutdown) = %v, want %v", got, want)
	}
	if got, want := taskStopCause(taskpkg.StopReasonOrphanedRun), session.CauseFailed; got != want {
		t.Fatalf("taskStopCause(orphaned) = %v, want %v", got, want)
	}
	if got, want := taskStopCause(taskpkg.StopReasonCancellation), session.CauseUserRequested; got != want {
		t.Fatalf("taskStopCause(cancellation) = %v, want %v", got, want)
	}
	if got, want := taskStopCause(taskpkg.StopReasonCompleted), session.CauseCompleted; got != want {
		t.Fatalf("taskStopCause(completed) = %v, want %v", got, want)
	}
	if got, want := taskStopCause(taskpkg.StopReasonFailed), session.CauseFailed; got != want {
		t.Fatalf("taskStopCause(failed) = %v, want %v", got, want)
	}
	if got, want := taskStopDetail(taskpkg.StopReasonShutdown), "task shutdown"; got != want {
		t.Fatalf("taskStopDetail(shutdown) = %q, want %q", got, want)
	}
	if got, want := taskStopDetail(taskpkg.StopReasonOrphanedRun), "task run orphaned"; got != want {
		t.Fatalf("taskStopDetail(orphaned) = %q, want %q", got, want)
	}
	if got, want := taskStopDetail(taskpkg.StopReasonCancellation), "task cancellation"; got != want {
		t.Fatalf("taskStopDetail(cancellation) = %q, want %q", got, want)
	}
	if got, want := taskStopDetail(taskpkg.StopReasonCompleted), "task completed"; got != want {
		t.Fatalf("taskStopDetail(completed) = %q, want %q", got, want)
	}
	if got, want := taskStopDetail(taskpkg.StopReasonFailed), "task failed"; got != want {
		t.Fatalf("taskStopDetail(failed) = %q, want %q", got, want)
	}

	live, state, err := taskSessionRuntimeState(context.Background(), &taskBridgeStopOnlySessionManager{}, "")
	if err != nil {
		t.Fatalf("taskSessionRuntimeState(blank id) error = %v", err)
	}
	if live {
		t.Fatal("taskSessionRuntimeState(blank id) live = true, want false")
	}
	if got, want := state, taskRecoverySessionMissing; got != want {
		t.Fatalf("taskSessionRuntimeState(blank id) state = %q, want %q", got, want)
	}

	if _, err := planTaskRunRecovery(context.Background(), nil, taskpkg.Run{
		ID:     "run-1",
		Status: taskpkg.TaskRunStatusClaimed,
	}); err == nil {
		t.Fatal("planTaskRunRecovery(nil sessions) error = nil, want validation error")
	}
}

func TestTaskRecoveryLivenessHelpers(t *testing.T) {
	t.Parallel()

	live, state, err := taskSessionRuntimeState(context.Background(), &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-live", State: session.StateActive},
		},
	}, "sess-live")
	if err != nil {
		t.Fatalf("taskSessionRuntimeState(live) error = %v", err)
	}
	if !live {
		t.Fatal("taskSessionRuntimeState(live) = false, want true")
	}
	if got, want := state, string(session.StateActive); got != want {
		t.Fatalf("taskSessionRuntimeState(live) state = %q, want %q", got, want)
	}

	if got := taskSessionMatchesRecordedSubprocess(nil); got {
		t.Fatal("taskSessionMatchesRecordedSubprocess(nil) = true, want false")
	}
	if got := taskSessionMatchesRecordedSubprocess(&store.SessionLivenessMeta{}); got {
		t.Fatal("taskSessionMatchesRecordedSubprocess(blank) = true, want false")
	}
	startedAt, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		t.Fatalf("procutil.StartedAt(self) error = %v", err)
	}
	if got := taskSessionMatchesRecordedSubprocess(&store.SessionLivenessMeta{
		SubprocessPID: os.Getpid(),
	}); got {
		t.Fatal("taskSessionMatchesRecordedSubprocess(missing start time) = true, want false")
	}
	if got := taskSessionMatchesRecordedSubprocess(&store.SessionLivenessMeta{
		SubprocessPID:       os.Getpid(),
		SubprocessStartedAt: &startedAt,
	}); !got {
		t.Fatal("taskSessionMatchesRecordedSubprocess(self) = false, want true")
	}
	if got, want := firstTaskRecoveryDetail("", " detail ", "fallback"), "detail"; got != want {
		t.Fatalf("firstTaskRecoveryDetail() = %q, want %q", got, want)
	}
}

type taskContextOverlayCall struct {
	taskID string
	runID  string
}

type taskContextOverlayStub struct {
	overlay string
	err     error
	calls   []taskContextOverlayCall
}

func (s *taskContextOverlayStub) TaskRunPromptOverlay(
	_ context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	_ *taskpkg.ExecutionProfile,
) (string, error) {
	s.calls = append(s.calls, taskContextOverlayCall{
		taskID: strings.TrimSpace(taskRecord.ID),
		runID:  strings.TrimSpace(run.ID),
	})
	if s.err != nil {
		return "", s.err
	}
	return s.overlay, nil
}

func TestTaskRuntimeDetachedHarnessSubmissionPersistsMetadataAndReusesIdempotency(t *testing.T) {
	t.Parallel()

	sessions := &fakeSessionManager{}
	runtime, resolver, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)
	workspace := resolveDaemonWorkspace(t, resolver, filepath.Join(t.TempDir(), "workspace"))
	sessions.infos = []*session.Info{
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
	}

	req := detachedHarnessSubmitRequest{
		SubmissionKey:  "detached-work-1",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "Workspace detached audit",
		Description:    "Review the queued harness work.",
		TurnSource:     session.TurnSourceNetwork,
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	}

	first, err := runtime.submitDetachedHarnessWork(context.Background(), req)
	if err != nil {
		t.Fatalf("submitDetachedHarnessWork(first) error = %v", err)
	}
	if first == nil {
		t.Fatal("submitDetachedHarnessWork(first) = nil, want submission")
		return
	}
	if first.ExistingTask {
		t.Fatal("submitDetachedHarnessWork(first).ExistingTask = true, want false")
	}
	if first.ExistingRun {
		t.Fatal("submitDetachedHarnessWork(first).ExistingRun = true, want false")
	}

	second, err := runtime.submitDetachedHarnessWork(context.Background(), req)
	if err != nil {
		t.Fatalf("submitDetachedHarnessWork(duplicate) error = %v", err)
	}
	if second == nil {
		t.Fatal("submitDetachedHarnessWork(duplicate) = nil, want submission")
		return
	}
	if !second.ExistingTask || !second.ExistingRun {
		t.Fatalf("duplicate submission flags = task:%v run:%v, want both true", second.ExistingTask, second.ExistingRun)
	}
	if got, want := second.Task.ID, first.Task.ID; got != want {
		t.Fatalf("duplicate task id = %q, want %q", got, want)
	}
	if got, want := second.Run.ID, first.Run.ID; got != want {
		t.Fatalf("duplicate run id = %q, want %q", got, want)
	}

	actor, err := detachedHarnessActorContext("sess-owner")
	if err != nil {
		t.Fatalf("detachedHarnessActorContext() error = %v", err)
	}
	storedTask, err := runtime.store.GetTask(context.Background(), first.Task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got, want := storedTask.Scope, taskpkg.ScopeWorkspace; got != want {
		t.Fatalf("storedTask.Scope = %q, want %q", got, want)
	}
	if got, want := storedTask.WorkspaceID, workspace.ID; got != want {
		t.Fatalf("storedTask.WorkspaceID = %q, want %q", got, want)
	}
	if storedTask.Owner == nil {
		t.Fatal("storedTask.Owner = nil, want owner session")
	}
	if got, want := storedTask.Owner.Kind, taskpkg.OwnerKindAgentSession; got != want {
		t.Fatalf("storedTask.Owner.Kind = %q, want %q", got, want)
	}
	if got, want := storedTask.Owner.Ref, "sess-owner"; got != want {
		t.Fatalf("storedTask.Owner.Ref = %q, want %q", got, want)
	}
	if got, want := storedTask.CreatedBy, actor.Actor; got != want {
		t.Fatalf("storedTask.CreatedBy = %#v, want %#v", got, want)
	}
	if got, want := storedTask.Origin, actor.Origin; got != want {
		t.Fatalf("storedTask.Origin = %#v, want %#v", got, want)
	}

	taskMetadata, err := decodeDetachedHarnessTaskMetadata(storedTask.Metadata)
	if err != nil {
		t.Fatalf("decodeDetachedHarnessTaskMetadata() error = %v", err)
	}
	if got, want := taskMetadata, (detachedHarnessTaskMetadata{
		Schema:               harnessDetachedMetadataSchema,
		Kind:                 harnessDetachedTaskMetadataKey,
		SubmissionKey:        "detached-work-1",
		Summary:              "Workspace detached audit",
		SubmissionTurnSource: string(session.TurnSourceNetwork),
		OwnerSessionID:       "sess-owner",
		OwnerSessionType:     string(session.SessionTypeSystem),
		OwnerWorkspaceID:     workspace.ID,
		WakeTarget: detachedHarnessWakeTarget{
			SessionID:   "sess-wake",
			SessionType: string(session.SessionTypeSystem),
			WorkspaceID: workspace.ID,
		},
	}); got != want {
		t.Fatalf("task metadata = %#v, want %#v", got, want)
	}

	storedRun, err := runtime.store.GetTaskRun(context.Background(), first.Run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun() error = %v", err)
	}
	if got, want := storedRun.TaskID, storedTask.ID; got != want {
		t.Fatalf("storedRun.TaskID = %q, want %q", got, want)
	}
	if got, want := storedRun.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("storedRun.Status = %q, want %q", got, want)
	}
	if got, want := storedRun.Origin, actor.Origin; got != want {
		t.Fatalf("storedRun.Origin = %#v, want %#v", got, want)
	}
	if got, want := storedRun.IdempotencyKey, "detached-work-1"; got != want {
		t.Fatalf("storedRun.IdempotencyKey = %q, want %q", got, want)
	}

	runMetadata, err := decodeDetachedHarnessRunMetadata(storedRun.Metadata)
	if err != nil {
		t.Fatalf("decodeDetachedHarnessRunMetadata() error = %v", err)
	}
	if got, want := runMetadata, (detachedHarnessRunMetadata{
		Schema:               harnessDetachedMetadataSchema,
		Kind:                 harnessDetachedRunMetadataKey,
		SubmissionKey:        "detached-work-1",
		Summary:              "Workspace detached audit",
		SubmissionTurnSource: string(session.TurnSourceNetwork),
		OwnerSessionID:       "sess-owner",
		OwnerSessionType:     string(session.SessionTypeSystem),
		OwnerWorkspaceID:     workspace.ID,
		WakeTarget: detachedHarnessWakeTarget{
			SessionID:   "sess-wake",
			SessionType: string(session.SessionTypeSystem),
			WorkspaceID: workspace.ID,
		},
	}); got != want {
		t.Fatalf("run metadata = %#v, want %#v", got, want)
	}

	readActor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task inspect")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	view, err := runtime.manager.GetTask(context.Background(), storedTask.ID, readActor)
	if err != nil {
		t.Fatalf("manager.GetTask() error = %v", err)
	}
	if got, want := len(view.Runs), 1; got != want {
		t.Fatalf("len(view.Runs) = %d, want %d", got, want)
	}
	runs, err := runtime.manager.ListTaskRuns(context.Background(), storedTask.ID, taskpkg.RunQuery{}, readActor)
	if err != nil {
		t.Fatalf("manager.ListTaskRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
}

func TestTaskRuntimeDetachedHarnessSubmissionValidationErrors(t *testing.T) {
	t.Parallel()

	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{
				ID:                   "sess-owner",
				Type:                 session.SessionTypeSystem,
				State:                session.StateActive,
				WorkspaceID:          "ws-owner",
				Workspace:            "/tmp/ws-owner",
				NetworkParticipation: daemonTestLiveParticipation("ws-owner", "builders"),
			},
			{
				ID:                   "sess-other-workspace",
				Type:                 session.SessionTypeSystem,
				State:                session.StateActive,
				WorkspaceID:          "ws-other",
				Workspace:            "/tmp/ws-other",
				NetworkParticipation: daemonTestLiveParticipation("ws-other", "builders"),
			},
		},
	}
	runtime, _, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)

	testCases := []struct {
		name string
		req  detachedHarnessSubmitRequest
	}{
		{
			name: "Should reject blank wake target session id",
			req: detachedHarnessSubmitRequest{
				SubmissionKey:  "detached-invalid-blank-wake",
				OwnerSessionID: "sess-owner",
				Scope:          taskpkg.ScopeGlobal,
				WakeTarget:     detachedHarnessWakeTargetInput{},
			},
		},
		{
			name: "Should reject unsupported scope",
			req: detachedHarnessSubmitRequest{
				SubmissionKey:  "detached-invalid-scope",
				OwnerSessionID: "sess-owner",
				Scope:          taskpkg.Scope("invalid"),
				WakeTarget: detachedHarnessWakeTargetInput{
					SessionID: "sess-owner",
				},
			},
		},
		{
			name: "Should reject workspace mismatch between owner and wake target",
			req: detachedHarnessSubmitRequest{
				SubmissionKey:  "detached-invalid-workspace",
				OwnerSessionID: "sess-owner",
				Scope:          taskpkg.ScopeWorkspace,
				WakeTarget: detachedHarnessWakeTargetInput{
					SessionID: "sess-other-workspace",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := runtime.submitDetachedHarnessWork(
				context.Background(),
				tc.req,
			); !errors.Is(
				err,
				taskpkg.ErrValidation,
			) {
				t.Fatalf("submitDetachedHarnessWork() error = %v, want %v", err, taskpkg.ErrValidation)
			}
		})
	}
}

func TestRecoverTaskRunsOnBootPreservesDetachedHarnessMetadata(t *testing.T) {
	t.Parallel()

	sessions := &fakeSessionManager{}
	runtime, resolver, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)
	workspace := resolveDaemonWorkspace(t, resolver, filepath.Join(t.TempDir(), "workspace"))
	sessions.infos = []*session.Info{
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

	submission, err := runtime.submitDetachedHarnessWork(context.Background(), detachedHarnessSubmitRequest{
		SubmissionKey:  "detached-recovery-1",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "Recover detached harness run",
		TurnSource:     session.TurnSourceSynthetic,
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})
	if err != nil {
		t.Fatalf("submitDetachedHarnessWork() error = %v", err)
	}

	actor, err := detachedHarnessActorContext("sess-owner")
	if err != nil {
		t.Fatalf("detachedHarnessActorContext() error = %v", err)
	}
	claimed := seedNonLeasedClaimedRunForDaemonTest(t, runtime.store, submission.Run.ID, actor)
	starting, err := runtime.manager.AttachRunSession(context.Background(), claimed.ID, "sess-runtime", actor)
	if err != nil {
		t.Fatalf("AttachRunSession() error = %v", err)
	}
	if got, want := starting.Status, taskpkg.TaskRunStatusStarting; got != want {
		t.Fatalf("starting.Status = %q, want %q", got, want)
	}

	bootActor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "daemon.boot")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	stats, err := recoverTaskRunsOnBoot(context.Background(), runtime.manager, runtime.store, sessions, bootActor)
	if err != nil {
		t.Fatalf("recoverTaskRunsOnBoot() error = %v", err)
	}
	if got, want := stats.markedRunning, 1; got != want {
		t.Fatalf("stats.markedRunning = %d, want %d", got, want)
	}

	recovered, err := runtime.store.GetTaskRun(context.Background(), submission.Run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(recovered) error = %v", err)
	}
	if got, want := recovered.Status, taskpkg.TaskRunStatusRunning; got != want {
		t.Fatalf("recovered.Status = %q, want %q", got, want)
	}
	metadata, err := decodeDetachedHarnessRunMetadata(recovered.Metadata)
	if err != nil {
		t.Fatalf("decodeDetachedHarnessRunMetadata(recovered) error = %v", err)
	}
	if got, want := metadata.SubmissionKey, "detached-recovery-1"; got != want {
		t.Fatalf("recovered metadata submission key = %q, want %q", got, want)
	}
	if got, want := metadata.OwnerSessionID, "sess-owner"; got != want {
		t.Fatalf("recovered metadata owner session id = %q, want %q", got, want)
	}
	if got, want := metadata.WakeTarget.SessionID, "sess-wake"; got != want {
		t.Fatalf("recovered metadata wake target session id = %q, want %q", got, want)
	}
}

func TestRecoverTaskRunsOnBootTracksAllRecoveryOutcomes(t *testing.T) {
	t.Parallel()

	sessions := &fakeSessionManager{}
	runtime, resolver, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)
	workspace := resolveDaemonWorkspace(t, resolver, filepath.Join(t.TempDir(), "workspace"))

	ownerInfo := &session.Info{
		ID:                   "sess-owner",
		Type:                 session.SessionTypeSystem,
		State:                session.StateActive,
		WorkspaceID:          workspace.ID,
		Workspace:            workspace.RootDir,
		NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
	}
	wakeInfo := &session.Info{
		ID:                   "sess-wake",
		Type:                 session.SessionTypeSystem,
		State:                session.StateActive,
		WorkspaceID:          workspace.ID,
		Workspace:            workspace.RootDir,
		NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
	}
	liveInfo := &session.Info{
		ID:                   "sess-live",
		Type:                 session.SessionTypeSystem,
		State:                session.StateActive,
		WorkspaceID:          workspace.ID,
		Workspace:            workspace.RootDir,
		NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
	}
	failedInfo := &session.Info{
		ID:                   "sess-fail",
		Type:                 session.SessionTypeSystem,
		State:                session.StateActive,
		WorkspaceID:          workspace.ID,
		Workspace:            workspace.RootDir,
		NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
	}
	sessions.infos = []*session.Info{ownerInfo, wakeInfo, liveInfo, failedInfo}

	makeSubmission := func(key string) *detachedHarnessSubmission {
		t.Helper()
		return submitDetachedHarnessWorkForTest(t, runtime, detachedHarnessSubmitRequest{
			SubmissionKey:  key,
			OwnerSessionID: "sess-owner",
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    workspace.ID,
			Summary:        "Recover " + key,
			TurnSource:     session.TurnSourceSynthetic,
			WakeTarget: detachedHarnessWakeTargetInput{
				SessionID: "sess-wake",
			},
		})
	}

	requeueSubmission := makeSubmission("detached-requeue")
	markSubmission := makeSubmission("detached-mark")
	failSubmission := makeSubmission("detached-fail")

	actor, err := detachedHarnessActorContext("sess-owner")
	if err != nil {
		t.Fatalf("detachedHarnessActorContext() error = %v", err)
	}

	seedNonLeasedClaimedRunForDaemonTest(t, runtime.store, requeueSubmission.Run.ID, actor)
	claimed := seedNonLeasedClaimedRunForDaemonTest(t, runtime.store, markSubmission.Run.ID, actor)
	if _, err := runtime.manager.AttachRunSession(context.Background(), claimed.ID, "sess-live", actor); err != nil {
		t.Fatalf("AttachRunSession(mark) error = %v", err)
	}
	claimed = seedNonLeasedClaimedRunForDaemonTest(t, runtime.store, failSubmission.Run.ID, actor)
	if _, err := runtime.manager.AttachRunSession(context.Background(), claimed.ID, "sess-fail", actor); err != nil {
		t.Fatalf("AttachRunSession(fail) error = %v", err)
	}
	failedInfo.State = session.StateStopped
	failedInfo.StopDetail = "daemon lost the task session"

	bootActor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "daemon.boot")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	stats, err := recoverTaskRunsOnBoot(context.Background(), runtime.manager, runtime.store, sessions, bootActor)
	if err != nil {
		t.Fatalf("recoverTaskRunsOnBoot() error = %v", err)
	}
	if got, want := stats.requeued, 1; got != want {
		t.Fatalf("stats.requeued = %d, want %d", got, want)
	}
	if got, want := stats.markedRunning, 1; got != want {
		t.Fatalf("stats.markedRunning = %d, want %d", got, want)
	}
	if got, want := stats.failed, 1; got != want {
		t.Fatalf("stats.failed = %d, want %d", got, want)
	}

	requeuedRun, err := runtime.store.GetTaskRun(context.Background(), requeueSubmission.Run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(requeue) error = %v", err)
	}
	if got, want := requeuedRun.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("requeued run status = %q, want %q", got, want)
	}
	if got, want := requeuedRun.NetworkSpec, participation.LocalSpec(); got != want {
		t.Fatalf("requeued run NetworkSpec = %#v, want %#v", got, want)
	}

	markedRun, err := runtime.store.GetTaskRun(context.Background(), markSubmission.Run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(mark) error = %v", err)
	}
	if got, want := markedRun.Status, taskpkg.TaskRunStatusRunning; got != want {
		t.Fatalf("marked run status = %q, want %q", got, want)
	}
	if got, want := markedRun.NetworkSpec, participation.LocalSpec(); got != want {
		t.Fatalf("marked run NetworkSpec = %#v, want %#v", got, want)
	}

	failedRun, err := runtime.store.GetTaskRun(context.Background(), failSubmission.Run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(fail) error = %v", err)
	}
	if got, want := failedRun.Status, taskpkg.TaskRunStatusFailed; got != want {
		t.Fatalf("failed run status = %q, want %q", got, want)
	}
	if got, want := failedRun.NetworkSpec, participation.LocalSpec(); got != want {
		t.Fatalf("failed run NetworkSpec = %#v, want %#v", got, want)
	}
}

func TestDetachedHarnessWorkBridgeHelperValidation(t *testing.T) {
	t.Parallel()

	if _, err := newHarnessDetachedWorkBridge(nil, openDaemonTestGlobalDB(t), &fakeSessionManager{}); err == nil {
		t.Fatal("newHarnessDetachedWorkBridge(nil tasks) error = nil, want validation error")
	}
	if _, err := newHarnessDetachedWorkBridge(&taskpkg.Service{}, nil, &fakeSessionManager{}); err == nil {
		t.Fatal("newHarnessDetachedWorkBridge(nil store) error = nil, want validation error")
	}
	if _, err := newHarnessDetachedWorkBridge(&taskpkg.Service{}, openDaemonTestGlobalDB(t), nil); err == nil {
		t.Fatal("newHarnessDetachedWorkBridge(nil sessions) error = nil, want validation error")
	}

	if _, err := decodeDetachedHarnessTaskMetadata(
		json.RawMessage(`{"schema":"bad","kind":"other"}`),
	); !errors.Is(
		err,
		taskpkg.ErrValidation,
	) {
		t.Fatalf("decodeDetachedHarnessTaskMetadata(wrong schema) error = %v, want %v", err, taskpkg.ErrValidation)
	}
	if _, err := decodeDetachedHarnessTaskMetadata(json.RawMessage(`{`)); !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf("decodeDetachedHarnessTaskMetadata(invalid json) error = %v, want %v", err, taskpkg.ErrValidation)
	}
	if _, err := decodeDetachedHarnessTaskMetadata(json.RawMessage(
		`{"schema":"agh.harness.detached.v1","kind":"harness_detached_task","owner_channel":"legacy"}`,
	)); !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf(
			"decodeDetachedHarnessTaskMetadata(removed owner_channel) error = %v, want %v",
			err,
			taskpkg.ErrValidation,
		)
	}
	if _, err := decodeDetachedHarnessRunMetadata(
		json.RawMessage(`{"schema":"bad","kind":"other"}`),
	); !errors.Is(
		err,
		taskpkg.ErrValidation,
	) {
		t.Fatalf("decodeDetachedHarnessRunMetadata(wrong schema) error = %v, want %v", err, taskpkg.ErrValidation)
	}
	if _, err := decodeDetachedHarnessRunMetadata(json.RawMessage(`{`)); !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf("decodeDetachedHarnessRunMetadata(invalid json) error = %v, want %v", err, taskpkg.ErrValidation)
	}
	if _, err := decodeDetachedHarnessRunMetadata(json.RawMessage(
		`{"schema":"agh.harness.detached.v1","kind":"harness_detached_run","wake_target":{"channel":"legacy"}}`,
	)); !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf(
			"decodeDetachedHarnessRunMetadata(removed wake_target.channel) error = %v, want %v",
			err,
			taskpkg.ErrValidation,
		)
	}

	if got, want := detachedHarnessSummary("   "), defaultDetachedHarnessSummary; got != want {
		t.Fatalf("detachedHarnessSummary(blank) = %q, want %q", got, want)
	}
	if got, want := normalizeDetachedHarnessTurnSource(
		session.TurnSource("unexpected"),
	), session.TurnSourceUser; got != want {
		t.Fatalf("normalizeDetachedHarnessTurnSource(unexpected) = %q, want %q", got, want)
	}
}

func TestTaskRuntimeDetachedHarnessSubmissionRejectsExistingMismatches(t *testing.T) {
	t.Parallel()

	sessions := &fakeSessionManager{}
	runtime, resolver, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)
	workspace := resolveDaemonWorkspace(t, resolver, filepath.Join(t.TempDir(), "workspace"))
	sessions.infos = []*session.Info{
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
	}

	baseReq := detachedHarnessSubmitRequest{
		SubmissionKey:  "detached-mismatch",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "Original detached work",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	}
	if _, err := runtime.submitDetachedHarnessWork(context.Background(), baseReq); err != nil {
		t.Fatalf("submitDetachedHarnessWork(base) error = %v", err)
	}

	if _, err := runtime.submitDetachedHarnessWork(context.Background(), detachedHarnessSubmitRequest{
		SubmissionKey:  "detached-mismatch",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "Changed detached work",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	}); !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf("submitDetachedHarnessWork(run mismatch) error = %v, want %v", err, taskpkg.ErrValidation)
	}

	actor, err := detachedHarnessActorContext("sess-owner")
	if err != nil {
		t.Fatalf("detachedHarnessActorContext() error = %v", err)
	}
	conflictMetadata, err := marshalDetachedHarnessMetadata(detachedHarnessTaskMetadata{
		Schema:               harnessDetachedMetadataSchema,
		Kind:                 harnessDetachedTaskMetadataKey,
		SubmissionKey:        "detached-conflict",
		Summary:              "Conflicting stored task",
		SubmissionTurnSource: string(session.TurnSourceUser),
		OwnerSessionID:       "sess-owner",
		OwnerSessionType:     string(session.SessionTypeSystem),
		OwnerWorkspaceID:     workspace.ID,
		WakeTarget: detachedHarnessWakeTarget{
			SessionID:   "sess-wake",
			SessionType: string(session.SessionTypeSystem),
			WorkspaceID: workspace.ID,
		},
	})
	if err != nil {
		t.Fatalf("marshalDetachedHarnessMetadata() error = %v", err)
	}
	conflictingTask := taskpkg.Task{
		ID:             detachedHarnessTaskID("sess-owner", "detached-conflict"),
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Title:          "Conflicting stored task",
		Status:         taskpkg.TaskStatusPending,
		MaxAttempts:    taskpkg.DefaultTaskMaxAttempts,
		ApprovalPolicy: taskpkg.ApprovalPolicyNone,
		ApprovalState:  taskpkg.ApprovalStateNotRequired,
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindAgentSession,
			Ref:  "sess-owner",
		},
		CreatedBy: actor.Actor,
		Origin:    actor.Origin,
		CreatedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
		Metadata:  conflictMetadata,
	}
	if err := runtime.store.CreateTask(context.Background(), conflictingTask); err != nil {
		t.Fatalf("CreateTask(conflictingTask) error = %v", err)
	}

	if _, err := runtime.submitDetachedHarnessWork(context.Background(), detachedHarnessSubmitRequest{
		SubmissionKey:  "detached-conflict",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "Expected detached work",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	}); !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf("submitDetachedHarnessWork(task mismatch) error = %v, want %v", err, taskpkg.ErrValidation)
	}

	if _, err := runtime.submitDetachedHarnessWork(context.Background(), detachedHarnessSubmitRequest{
		SubmissionKey:  "missing-session",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "Missing wake target",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-missing",
		},
	}); !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf("submitDetachedHarnessWork(missing session) error = %v, want %v", err, taskpkg.ErrValidation)
	}
}

func TestTaskRuntimeSubmitDetachedHarnessWorkGuards(t *testing.T) {
	t.Parallel()

	var nilRuntime *taskRuntime
	if _, err := nilRuntime.submitDetachedHarnessWork(
		context.Background(),
		detachedHarnessSubmitRequest{},
	); err == nil {
		t.Fatal("nil runtime submit error = nil, want validation error")
	}

	runtime := &taskRuntime{}
	if _, err := runtime.submitDetachedHarnessWork(context.Background(), detachedHarnessSubmitRequest{}); err == nil {
		t.Fatal("runtime without detached bridge error = nil, want validation error")
	}

	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-owner", Type: session.SessionTypeSystem, State: session.StateActive},
			{ID: "sess-wake", Type: session.SessionTypeSystem, State: session.StateActive},
		},
	}
	readyRuntime, _, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)
	if _, err := readyRuntime.submitDetachedHarnessWork(nilTaskRuntimeContext(), detachedHarnessSubmitRequest{
		SubmissionKey:  "detached-guard",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeGlobal,
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	}); err == nil {
		t.Fatal("submitDetachedHarnessWork(nil ctx) error = nil, want validation error")
	}
}

func TestDetachedHarnessMatchValidatorsRejectConflicts(t *testing.T) {
	t.Parallel()

	req := normalizedDetachedHarnessSubmitRequest{
		TaskID:           detachedHarnessTaskID("sess-owner", "validator"),
		SubmissionKey:    "validator",
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      "ws-1",
		Summary:          "Validator task",
		Description:      "Ensure helper coverage",
		TurnSource:       session.TurnSourceSynthetic,
		OwnerSessionID:   "sess-owner",
		OwnerSessionType: string(session.SessionTypeSystem),
		OwnerWorkspaceID: "ws-1",
		WakeTarget: detachedHarnessWakeTarget{
			SessionID:   "sess-wake",
			SessionType: string(session.SessionTypeSystem),
			WorkspaceID: "ws-1",
		},
	}
	actor, err := detachedHarnessActorContext(req.OwnerSessionID)
	if err != nil {
		t.Fatalf("detachedHarnessActorContext() error = %v", err)
	}
	taskMetadata := buildDetachedHarnessTaskMetadata(req)
	taskMetadataJSON, err := marshalDetachedHarnessMetadata(taskMetadata)
	if err != nil {
		t.Fatalf("marshalDetachedHarnessMetadata(task) error = %v", err)
	}
	runMetadata := buildDetachedHarnessRunMetadata(req)
	runMetadataJSON, err := marshalDetachedHarnessMetadata(runMetadata)
	if err != nil {
		t.Fatalf("marshalDetachedHarnessMetadata(run) error = %v", err)
	}

	matchingTask := taskpkg.Task{
		ID:             req.TaskID,
		Scope:          req.Scope,
		WorkspaceID:    req.WorkspaceID,
		Title:          req.Summary,
		Description:    req.Description,
		Status:         taskpkg.TaskStatusPending,
		MaxAttempts:    taskpkg.DefaultTaskMaxAttempts,
		ApprovalPolicy: taskpkg.ApprovalPolicyNone,
		ApprovalState:  taskpkg.ApprovalStateNotRequired,
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindAgentSession,
			Ref:  req.OwnerSessionID,
		},
		CreatedBy: actor.Actor,
		Origin:    actor.Origin,
		CreatedAt: time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC),
		Metadata:  taskMetadataJSON,
	}
	if err := validateDetachedHarnessTaskMatch(matchingTask, req, actor, taskMetadata); err != nil {
		t.Fatalf("validateDetachedHarnessTaskMatch(match) error = %v", err)
	}

	missingOwnerTask := matchingTask
	missingOwnerTask.Owner = nil
	if err := validateDetachedHarnessTaskMatch(
		missingOwnerTask,
		req,
		actor,
		taskMetadata,
	); !errors.Is(
		err,
		taskpkg.ErrValidation,
	) {
		t.Fatalf("validateDetachedHarnessTaskMatch(missing owner) error = %v, want %v", err, taskpkg.ErrValidation)
	}

	matchingRun := taskpkg.Run{
		ID:             "run-validator",
		TaskID:         req.TaskID,
		Status:         taskpkg.TaskRunStatusQueued,
		Attempt:        1,
		Origin:         actor.Origin,
		IdempotencyKey: req.SubmissionKey,
		Metadata:       runMetadataJSON,
		QueuedAt:       time.Date(2026, 4, 18, 11, 5, 0, 0, time.UTC),
	}
	if err := validateDetachedHarnessRunMatch(matchingRun, req, actor.Origin, runMetadata); err != nil {
		t.Fatalf("validateDetachedHarnessRunMatch(match) error = %v", err)
	}

	wrongOriginRun := matchingRun
	wrongOriginRun.Origin = taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "agh task run"}
	if err := validateDetachedHarnessRunMatch(
		wrongOriginRun,
		req,
		actor.Origin,
		runMetadata,
	); !errors.Is(
		err,
		taskpkg.ErrValidation,
	) {
		t.Fatalf("validateDetachedHarnessRunMatch(wrong origin) error = %v, want %v", err, taskpkg.ErrValidation)
	}
}

func TestHarnessReentryBridgeScenarios(t *testing.T) {
	testCases := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "ShouldEmitSyntheticWakeAndObservabilityForDetachedCompletion",
			run:  testHarnessReentryBridgeEmitsSyntheticWakeAndObservability,
		},
		{
			name: "ShouldRecordDropSummaryWhenPolicyIsSilent",
			run:  testHarnessReentryBridgeSilentPolicyRecordsDropSummary,
		},
		{
			name: "ShouldDropMissingOrStoppedTargetsWithoutDispatchingWake",
			run:  testHarnessReentryBridgeMissingAndStoppedTargetsDropWithoutWake,
		},
		{
			name: "ShouldStayIdempotentAcrossDuplicateTerminalNotifications",
			run:  testHarnessReentryBridgeDuplicateTerminalNotificationsStayIdempotent,
		},
		{
			name: "ShouldPreserveSyntheticWakeFIFOOrdering",
			run:  testHarnessReentryBridgePreservesSyntheticWakeFIFO,
		},
		{
			name: "ShouldCoverHarnessReentryBridgeHelperBehaviors",
			run:  testHarnessReentryBridgeHelperCoverage,
		},
		{
			name: "ShouldDropWhenSyntheticDispatchFails",
			run:  testHarnessReentryBridgeDropsWhenSyntheticDispatchFails,
		},
		{
			name: "ShouldDropWhenSyntheticPromptChannelHasNoEvent",
			run:  testHarnessReentryBridgeDropsWhenSyntheticPromptChannelHasNoEvent,
		},
		{
			name: "ShouldDropWhenSyntheticPromptReturnsAnErrorEvent",
			run:  testHarnessReentryBridgeDropsWhenSyntheticPromptReturnsErrorEvent,
		},
		{
			name: "ShouldUseRecordedSyntheticEventForDispatchDedupe",
			run:  testHarnessReentryBridgeDispatchWakeUsesRecordedSyntheticEvent,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func testHarnessReentryBridgeEmitsSyntheticWakeAndObservability(t *testing.T) {
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
			{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
		},
	}
	runtime, _, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)

	submission := submitDetachedHarnessWorkForTest(t, runtime, detachedHarnessSubmitRequest{
		SubmissionKey:  "reentry-emitted",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeGlobal,
		Summary:        "Bridge emitted wake",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})
	completeDetachedHarnessRunForTest(t, runtime, submission.Run.ID, "sess-owner")

	metadata := waitForDetachedHarnessReentryState(t, runtime, submission.Run.ID, harnessReentryOutcomeEmitted)
	if got, want := metadata.Reentry.Reason, harnessReentryReasonCompleted; got != want {
		t.Fatalf("reentry reason = %q, want %q", got, want)
	}
	if got, want := sessions.syntheticPromptCount(), 1; got != want {
		t.Fatalf("synthetic prompt count = %d, want %d", got, want)
	}

	events, err := sessions.Events(
		testutil.Context(t),
		"sess-wake",
		store.EventQuery{Type: acp.EventTypeSyntheticReentry},
	)
	if err != nil {
		t.Fatalf("Events(synthetic) error = %v", err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf("len(synthetic events) = %d, want %d", got, want)
	}

	types := waitForEventSummaryTypes(
		t,
		runtime,
		"sess-wake",
		harnessSummaryDetachedCompleted,
		harnessSummarySyntheticReentryEmitted,
	)
	if !slices.Contains(types, harnessSummaryDetachedCompleted) {
		t.Fatalf("event summary types = %#v, want %q", types, harnessSummaryDetachedCompleted)
	}
	if !slices.Contains(types, harnessSummarySyntheticReentryEmitted) {
		t.Fatalf("event summary types = %#v, want %q", types, harnessSummarySyntheticReentryEmitted)
	}
}

func testHarnessReentryBridgeSilentPolicyRecordsDropSummary(t *testing.T) {
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
			{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeUser, State: session.StateActive},
		},
	}
	runtime, _, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)

	submission := submitDetachedHarnessWorkForTest(t, runtime, detachedHarnessSubmitRequest{
		SubmissionKey:  "reentry-silent",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeGlobal,
		Summary:        "Bridge silent completion",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})
	completeDetachedHarnessRunForTest(t, runtime, submission.Run.ID, "sess-owner")

	metadata := waitForDetachedHarnessReentryState(t, runtime, submission.Run.ID, harnessReentryOutcomeSilent)
	if got, want := metadata.Reentry.Reason, harnessReentryReasonPolicySilent; got != want {
		t.Fatalf("reentry reason = %q, want %q", got, want)
	}
	if got := sessions.syntheticPromptCount(); got != 0 {
		t.Fatalf("synthetic prompt count = %d, want 0 for silent completion", got)
	}

	types := waitForEventSummaryTypes(
		t,
		runtime,
		"sess-wake",
		harnessSummaryDetachedCompleted,
		harnessSummarySyntheticReentryDropped,
	)
	if !slices.Contains(types, harnessSummaryDetachedCompleted) {
		t.Fatalf("event summary types = %#v, want %q", types, harnessSummaryDetachedCompleted)
	}
	if !slices.Contains(types, harnessSummarySyntheticReentryDropped) {
		t.Fatalf("event summary types = %#v, want %q", types, harnessSummarySyntheticReentryDropped)
	}
}

func testHarnessReentryBridgeMissingAndStoppedTargetsDropWithoutWake(t *testing.T) {
	testCases := []struct {
		name       string
		mutate     func(*fakeSessionManager)
		wantReason string
	}{
		{
			name: "Should drop when the target session disappears before completion",
			mutate: func(sessions *fakeSessionManager) {
				sessions.mu.Lock()
				defer sessions.mu.Unlock()
				sessions.infos = []*session.Info{
					{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
				}
			},
			wantReason: harnessReentryReasonTargetMissing,
		},
		{
			name: "Should drop when the target session is stopped before completion",
			mutate: func(sessions *fakeSessionManager) {
				sessions.mu.Lock()
				defer sessions.mu.Unlock()
				sessions.infos = []*session.Info{
					{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
					{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateStopped},
				}
			},
			wantReason: inactiveTargetReason(session.StateStopped),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sessions := &fakeSessionManager{
				infos: []*session.Info{
					{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
					{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
				},
			}
			runtime, _, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)
			submission := submitDetachedHarnessWorkForTest(t, runtime, detachedHarnessSubmitRequest{
				SubmissionKey:  strings.ReplaceAll(strings.ToLower(tc.name), " ", "-"),
				OwnerSessionID: "sess-owner",
				Scope:          taskpkg.ScopeGlobal,
				Summary:        "Bridge unavailable target",
				WakeTarget: detachedHarnessWakeTargetInput{
					SessionID: "sess-wake",
				},
			})

			tc.mutate(sessions)
			completeDetachedHarnessRunForTest(t, runtime, submission.Run.ID, "sess-owner")

			metadata := waitForDetachedHarnessReentryState(t, runtime, submission.Run.ID, harnessReentryOutcomeDropped)
			if got, want := metadata.Reentry.Reason, tc.wantReason; got != want {
				t.Fatalf("reentry reason = %q, want %q", got, want)
			}
			if got := sessions.syntheticPromptCount(); got != 0 {
				t.Fatalf("synthetic prompt count = %d, want 0 when the target is unavailable", got)
			}
		})
	}
}

func testHarnessReentryBridgeDuplicateTerminalNotificationsStayIdempotent(t *testing.T) {
	releaseFirst := make(chan struct{})
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
			{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
		},
	}
	runtime, _, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)

	var firstRunID string
	sessions.syntheticPromptHook = func(ctx context.Context, id string, opts session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error) {
		info, err := sessions.Status(ctx, id)
		if err != nil {
			return nil, err
		}
		sessions.recordSyntheticEvent(id, info, opts)
		ch := make(chan acp.AgentEvent)
		if opts.Metadata.TaskRunID == firstRunID {
			go func() {
				<-releaseFirst
				close(ch)
			}()
			return ch, nil
		}
		close(ch)
		return ch, nil
	}

	submission := submitDetachedHarnessWorkForTest(t, runtime, detachedHarnessSubmitRequest{
		SubmissionKey:  "reentry-duplicate",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeGlobal,
		Summary:        "Bridge duplicate terminal event",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})
	firstRunID = submission.Run.ID
	completion := completeDetachedHarnessRunForTest(t, runtime, submission.Run.ID, "sess-owner")
	waitForTaskRuntimeCondition(t, 2*time.Second, func() bool {
		return sessions.syntheticPromptCount() == 1
	})

	if err := runtime.reentry.processTerminalRun(submission.Task.ID, completion.ID, 999, time.Now().UTC()); err != nil {
		t.Fatalf("processTerminalRun(duplicate) error = %v", err)
	}
	if got, want := sessions.syntheticPromptCount(), 1; got != want {
		t.Fatalf("synthetic prompt count after duplicate = %d, want %d", got, want)
	}

	close(releaseFirst)
	waitForDetachedHarnessReentryState(t, runtime, submission.Run.ID, harnessReentryOutcomeEmitted)
}

func testHarnessReentryBridgePreservesSyntheticWakeFIFO(t *testing.T) {
	releaseFirst := make(chan struct{})
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
			{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
		},
	}
	runtime, _, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)

	var firstRunID string
	sessions.syntheticPromptHook = func(ctx context.Context, id string, opts session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error) {
		info, err := sessions.Status(ctx, id)
		if err != nil {
			return nil, err
		}
		sessions.recordSyntheticEvent(id, info, opts)
		ch := make(chan acp.AgentEvent)
		if opts.Metadata.TaskRunID == firstRunID {
			go func() {
				<-releaseFirst
				close(ch)
			}()
			return ch, nil
		}
		close(ch)
		return ch, nil
	}

	first := submitDetachedHarnessWorkForTest(t, runtime, detachedHarnessSubmitRequest{
		SubmissionKey:  "reentry-fifo-first",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeGlobal,
		Summary:        "First detached wake",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})
	second := submitDetachedHarnessWorkForTest(t, runtime, detachedHarnessSubmitRequest{
		SubmissionKey:  "reentry-fifo-second",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeGlobal,
		Summary:        "Second detached wake",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})
	firstRunID = first.Run.ID

	completeDetachedHarnessRunForTest(t, runtime, first.Run.ID, "sess-owner")
	waitForTaskRuntimeCondition(t, 2*time.Second, func() bool {
		return sessions.syntheticPromptCount() == 1
	})
	completeDetachedHarnessRunForTest(t, runtime, second.Run.ID, "sess-owner")
	waitForTaskRuntimeCondition(t, 2*time.Second, func() bool {
		return sessions.syntheticPromptCount() == 2
	})

	sessions.mu.Lock()
	if got, want := sessions.syntheticPromptCalls[0].opts.Metadata.TaskRunID, first.Run.ID; got != want {
		sessions.mu.Unlock()
		t.Fatalf("first synthetic wake run id = %q, want %q", got, want)
	}
	if got, want := sessions.syntheticPromptCalls[1].opts.Metadata.TaskRunID, second.Run.ID; got != want {
		sessions.mu.Unlock()
		t.Fatalf("second synthetic wake run id = %q, want %q", got, want)
	}
	sessions.mu.Unlock()

	close(releaseFirst)
	waitForDetachedHarnessReentryState(t, runtime, first.Run.ID, harnessReentryOutcomeEmitted)
	waitForDetachedHarnessReentryState(t, runtime, second.Run.ID, harnessReentryOutcomeEmitted)
}

func testHarnessReentryBridgeHelperCoverage(t *testing.T) {
	resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		MemoryPromptSectionEnabled: true,
		SkillsPromptSectionEnabled: true,
		SyntheticTurnsEnabled:      true,
		DetachedTaskRuntimeEnabled: true,
	})
	db := openDaemonTestGlobalDB(t)
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
		},
	}

	if _, err := newHarnessReentryBridge(
		nilTaskRuntimeContext(),
		resolver,
		nil,
		db,
		sessions,
		discardLogger(),
	); err == nil {
		t.Fatal("newHarnessReentryBridge(nil ctx) error = nil, want validation error")
	}
	if _, err := newHarnessReentryBridge(context.Background(), nil, nil, db, sessions, discardLogger()); err == nil {
		t.Fatal("newHarnessReentryBridge(nil resolver) error = nil, want validation error")
	}
	if _, err := newHarnessReentryBridge(
		context.Background(),
		resolver,
		nil,
		nil,
		sessions,
		discardLogger(),
	); err == nil {
		t.Fatal("newHarnessReentryBridge(nil store) error = nil, want validation error")
	}
	if _, err := newHarnessReentryBridge(context.Background(), resolver, nil, db, nil, discardLogger()); err == nil {
		t.Fatal("newHarnessReentryBridge(nil sessions) error = nil, want validation error")
	}

	bridge, err := newHarnessReentryBridge(context.Background(), resolver, nil, db, sessions, discardLogger())
	if err != nil {
		t.Fatalf("newHarnessReentryBridge() error = %v", err)
	}
	bridge.OnTaskEvent(context.Background(), taskpkg.EventRecord{})
	bridge.shutdown()
	bridge.shutdown()

	var nilBridge *harnessReentryBridge
	nilBridge.shutdown()
	if err := nilBridge.recover(context.Background()); err == nil {
		t.Fatal("nil bridge recover error = nil, want validation error")
	}
	if err := bridge.recover(nilTaskRuntimeContext()); err == nil {
		t.Fatal("recover(nil ctx) error = nil, want validation error")
	}

	left := harnessSyntheticWake{
		runID:         "run-a",
		completedAt:   time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
		completionSeq: 1,
	}
	right := harnessSyntheticWake{
		runID:         "run-b",
		completedAt:   time.Date(2026, 4, 18, 12, 1, 0, 0, time.UTC),
		completionSeq: 2,
	}
	if got := compareSyntheticWake(left, right); got >= 0 {
		t.Fatalf("compareSyntheticWake(time) = %d, want negative", got)
	}
	right.completedAt = left.completedAt
	if got := compareSyntheticWake(left, right); got >= 0 {
		t.Fatalf("compareSyntheticWake(sequence) = %d, want negative", got)
	}
	right.completionSeq = left.completionSeq
	if got := compareSyntheticWake(left, right); got >= 0 {
		t.Fatalf("compareSyntheticWake(run id) = %d, want negative", got)
	}

	failedReason, failedTrigger := syntheticReasonForTerminalRun(taskpkg.TaskRunStatusFailed)
	if failedReason != harnessReentryReasonFailed || failedTrigger != "task.run_failed" {
		t.Fatalf("syntheticReasonForTerminalRun(failed) = %q/%q", failedReason, failedTrigger)
	}
	canceledReason, canceledTrigger := syntheticReasonForTerminalRun(taskpkg.TaskRunStatusCanceled)
	if canceledReason != harnessReentryReasonCanceled || canceledTrigger != "task.run_canceled" {
		t.Fatalf("syntheticReasonForTerminalRun(canceled) = %q/%q", canceledReason, canceledTrigger)
	}
	completedReason, completedTrigger := syntheticReasonForTerminalRun(taskpkg.TaskRunStatusCompleted)
	if completedReason != harnessReentryReasonCompleted || completedTrigger != "task.run_completed" {
		t.Fatalf("syntheticReasonForTerminalRun(completed) = %q/%q", completedReason, completedTrigger)
	}

	taskRecord := taskpkg.Task{ID: "task-1"}
	if got := buildDetachedHarnessSyntheticMessage(taskRecord, taskpkg.Run{
		ID:     "run-complete",
		Status: taskpkg.TaskRunStatusCompleted,
	}, "summary"); !strings.Contains(got, "completed") {
		t.Fatalf("completed synthetic message = %q, want completion text", got)
	}
	if got := buildDetachedHarnessSyntheticMessage(taskRecord, taskpkg.Run{
		ID:     "run-failed",
		Status: taskpkg.TaskRunStatusFailed,
		Error:  "boom",
	}, "summary"); !strings.Contains(got, "failed") || !strings.Contains(got, "boom") {
		t.Fatalf("failed synthetic message = %q, want failure text", got)
	}
	if got := buildDetachedHarnessSyntheticMessage(taskRecord, taskpkg.Run{
		ID:     "run-canceled",
		Status: taskpkg.TaskRunStatusCanceled,
	}, "summary"); !strings.Contains(got, "canceled") {
		t.Fatalf("canceled synthetic message = %q, want canceled text", got)
	}

	if isDetachedHarnessTerminalRun(taskpkg.TaskRunStatusRunning) {
		t.Fatal("isDetachedHarnessTerminalRun(running) = true, want false")
	}
	if got, want := inactiveTargetReason(""), harnessReentryReasonTargetInactivePrefix; got != want {
		t.Fatalf("inactiveTargetReason(blank) = %q, want %q", got, want)
	}
	if got, want := inactiveTargetReason(session.StateStopped), "target_inactive:stopped"; got != want {
		t.Fatalf("inactiveTargetReason(stopped) = %q, want %q", got, want)
	}
	if got, want := classifySyntheticPromptError(
		session.ErrSessionNotFound,
	), harnessReentryReasonTargetMissing; got != want {
		t.Fatalf("classifySyntheticPromptError(not found) = %q, want %q", got, want)
	}
	if got, want := classifySyntheticPromptError(
		session.ErrSessionNotActive,
	), harnessReentryReasonTargetInactivePrefix; got != want {
		t.Fatalf("classifySyntheticPromptError(not active) = %q, want %q", got, want)
	}
	if got, want := classifySyntheticPromptError(errors.New("boom")), harnessReentryReasonDispatchFailed; got != want {
		t.Fatalf("classifySyntheticPromptError(generic) = %q, want %q", got, want)
	}

	if found, err := bridge.syntheticEventExists("", ""); err != nil || found {
		t.Fatalf("syntheticEventExists(blank) = %v, %v, want false, nil", found, err)
	}
}

func testHarnessReentryBridgeDropsWhenSyntheticDispatchFails(t *testing.T) {
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
			{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
		},
	}
	runtime, _, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)
	sessions.syntheticPromptHook = func(context.Context, string, session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error) {
		return nil, errors.New("dispatch failed")
	}

	submission := submitDetachedHarnessWorkForTest(t, runtime, detachedHarnessSubmitRequest{
		SubmissionKey:  "reentry-dispatch-failed",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeGlobal,
		Summary:        "Bridge dispatch failure",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})

	completeDetachedHarnessRunForTest(t, runtime, submission.Run.ID, "sess-owner")
	metadata := waitForDetachedHarnessReentryState(t, runtime, submission.Run.ID, harnessReentryOutcomeDropped)
	if got, want := metadata.Reentry.Reason, harnessReentryReasonDispatchFailed; got != want {
		t.Fatalf("metadata.Reentry.Reason = %q, want %q", got, want)
	}
	waitForEventSummaryTypes(
		t,
		runtime,
		"sess-wake",
		harnessSummaryDetachedCompleted,
		harnessSummarySyntheticReentryDropped,
	)
}

func testHarnessReentryBridgeDropsWhenSyntheticPromptChannelHasNoEvent(t *testing.T) {
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
			{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
		},
	}
	runtime, _, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)
	sessions.syntheticPromptHook = func(context.Context, string, session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error) {
		ch := make(chan acp.AgentEvent)
		close(ch)
		return ch, nil
	}

	submission := submitDetachedHarnessWorkForTest(t, runtime, detachedHarnessSubmitRequest{
		SubmissionKey:  "reentry-event-missing",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeGlobal,
		Summary:        "Bridge missing synthetic event",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})

	completeDetachedHarnessRunForTest(t, runtime, submission.Run.ID, "sess-owner")
	metadata := waitForDetachedHarnessReentryState(t, runtime, submission.Run.ID, harnessReentryOutcomeDropped)
	if got, want := metadata.Reentry.Reason, harnessReentryReasonEventMissing; got != want {
		t.Fatalf("metadata.Reentry.Reason = %q, want %q", got, want)
	}
}

func testHarnessReentryBridgeDropsWhenSyntheticPromptReturnsErrorEvent(t *testing.T) {
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
			{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
		},
	}
	runtime, _, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)
	sessions.syntheticPromptHook = func(context.Context, string, session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error) {
		ch := make(chan acp.AgentEvent, 1)
		ch <- acp.AgentEvent{Type: acp.EventTypeError}
		close(ch)
		return ch, nil
	}

	submission := submitDetachedHarnessWorkForTest(t, runtime, detachedHarnessSubmitRequest{
		SubmissionKey:  "reentry-dispatch-error-event",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeGlobal,
		Summary:        "Bridge synthetic error event",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})

	completeDetachedHarnessRunForTest(t, runtime, submission.Run.ID, "sess-owner")
	metadata := waitForDetachedHarnessReentryState(t, runtime, submission.Run.ID, harnessReentryOutcomeDropped)
	if got, want := metadata.Reentry.Reason, harnessReentryReasonDispatchFailed; got != want {
		t.Fatalf("metadata.Reentry.Reason = %q, want %q", got, want)
	}
}

func testHarnessReentryBridgeDispatchWakeUsesRecordedSyntheticEvent(t *testing.T) {
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
			{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
		},
	}
	runtime, _, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)

	submission := submitDetachedHarnessWorkForTest(t, runtime, detachedHarnessSubmitRequest{
		SubmissionKey:  "reentry-existing-synthetic-event",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeGlobal,
		Summary:        "Bridge dispatch existing event",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})

	completeDetachedHarnessRunForTest(t, runtime, submission.Run.ID, "sess-owner")
	waitForDetachedHarnessReentryState(t, runtime, submission.Run.ID, harnessReentryOutcomeEmitted)

	run, err := runtime.store.GetTaskRun(testutil.Context(t), submission.Run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun() error = %v", err)
	}
	metadata, ok, err := maybeDecodeDetachedHarnessRunMetadata(run.Metadata)
	if err != nil {
		t.Fatalf("maybeDecodeDetachedHarnessRunMetadata() error = %v", err)
	}
	if !ok {
		t.Fatal("detached harness metadata = missing, want metadata")
	}
	metadata.Reentry = nil
	run.Metadata, err = marshalDetachedHarnessMetadata(metadata)
	if err != nil {
		t.Fatalf("marshalDetachedHarnessMetadata() error = %v", err)
	}
	if err := runtime.store.UpdateTaskRun(testutil.Context(t), run); err != nil {
		t.Fatalf("UpdateTaskRun() error = %v", err)
	}

	runtime.reentry.dispatchWake(harnessSyntheticWake{
		taskID:          submission.Task.ID,
		runID:           submission.Run.ID,
		targetSessionID: "sess-wake",
		targetAgentName: "coder",
		reason:          harnessReentryReasonCompleted,
	})

	metadata = waitForDetachedHarnessReentryState(t, runtime, submission.Run.ID, harnessReentryOutcomeEmitted)
	if got, want := metadata.Reentry.Reason, harnessReentryReasonAlreadyRecorded; got != want {
		t.Fatalf("metadata.Reentry.Reason = %q, want %q", got, want)
	}
	if got, want := sessions.syntheticPromptCount(), 1; got != want {
		t.Fatalf("synthetic prompt count = %d, want %d", got, want)
	}
}

type taskBridgeStopOnlySessionManager struct {
	stopCalls []fakeStopWithCauseCall
}

type taskBridgeCreateErrorSessionManager struct {
	err error
}

type taskBridgeNilStatusSessionManager struct{}

func (m *taskBridgeCreateErrorSessionManager) Create(context.Context, session.CreateOpts) (*session.Session, error) {
	return nil, m.err
}

func (m *taskBridgeCreateErrorSessionManager) Status(context.Context, string) (*session.Info, error) {
	return nil, session.ErrSessionNotFound
}

func (m *taskBridgeCreateErrorSessionManager) StopWithCause(
	context.Context,
	string,
	session.StopCause,
	string,
) error {
	return nil
}

func (m *taskBridgeNilStatusSessionManager) Create(context.Context, session.CreateOpts) (*session.Session, error) {
	return &session.Session{ID: "unused"}, nil
}

func (m *taskBridgeNilStatusSessionManager) Status(context.Context, string) (*session.Info, error) {
	return nil, nil
}

func (m *taskBridgeNilStatusSessionManager) StopWithCause(
	context.Context,
	string,
	session.StopCause,
	string,
) error {
	return nil
}

func (m *taskBridgeStopOnlySessionManager) Create(context.Context, session.CreateOpts) (*session.Session, error) {
	return nil, nil
}

func (m *taskBridgeStopOnlySessionManager) Status(context.Context, string) (*session.Info, error) {
	return nil, session.ErrSessionNotFound
}

func (m *taskBridgeStopOnlySessionManager) StopWithCause(
	_ context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	m.stopCalls = append(m.stopCalls, fakeStopWithCauseCall{id: id, cause: cause, detail: detail})
	return nil
}

func nilTaskRuntimeContext() context.Context {
	return nil
}

func submitDetachedHarnessWorkForTest(
	t *testing.T,
	runtime *taskRuntime,
	req detachedHarnessSubmitRequest,
) *detachedHarnessSubmission {
	t.Helper()

	submission, err := runtime.submitDetachedHarnessWork(testutil.Context(t), req)
	if err != nil {
		t.Fatalf("submitDetachedHarnessWork() error = %v", err)
	}
	return submission
}

func completeDetachedHarnessRunForTest(
	t *testing.T,
	runtime *taskRuntime,
	runID string,
	ownerSessionID string,
) taskpkg.Run {
	t.Helper()

	actor, err := detachedHarnessActorContext(ownerSessionID)
	if err != nil {
		t.Fatalf("detachedHarnessActorContext() error = %v", err)
	}
	claimed := seedNonLeasedClaimedRunForDaemonTest(t, runtime.store, runID, actor)
	started, err := runtime.manager.StartRun(testutil.Context(t), claimed.ID, taskpkg.StartRun{
		IdempotencyKey: "start-" + runID,
	}, actor)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	completed, err := runtime.manager.CompleteRun(testutil.Context(t), started.ID, taskpkg.RunResult{
		Value: json.RawMessage(`{"ok":true}`),
	}, actor)
	if err != nil {
		t.Fatalf("CompleteRun() error = %v", err)
	}
	return *completed
}

func waitForDetachedHarnessReentryState(
	t *testing.T,
	runtime *taskRuntime,
	runID string,
	wantOutcome string,
) detachedHarnessRunMetadata {
	t.Helper()

	var got detachedHarnessRunMetadata
	waitForTaskRuntimeCondition(t, 2*time.Second, func() bool {
		run, err := runtime.store.GetTaskRun(testutil.Context(t), runID)
		if err != nil {
			return false
		}
		metadata, ok, decodeErr := maybeDecodeDetachedHarnessRunMetadata(run.Metadata)
		if decodeErr != nil || !ok || metadata.Reentry == nil {
			return false
		}
		got = metadata
		return metadata.Reentry.Outcome == wantOutcome
	})
	return got
}

func eventSummaryTypesForRunSession(t *testing.T, runtime *taskRuntime, sessionID string) []string {
	t.Helper()

	summaryStore, ok := runtime.store.(interface {
		ListEventSummaries(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error)
	})
	if !ok {
		t.Fatal("runtime.store does not expose event summaries")
	}
	summaries, err := summaryStore.ListEventSummaries(
		testutil.Context(t),
		store.EventSummaryQuery{SessionID: sessionID},
	)
	if err != nil {
		t.Fatalf("ListEventSummaries() error = %v", err)
	}
	types := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		types = append(types, summary.Type)
	}
	return types
}

func waitForEventSummaryTypes(
	t *testing.T,
	runtime *taskRuntime,
	sessionID string,
	wantTypes ...string,
) []string {
	t.Helper()

	var got []string
	waitForTaskRuntimeCondition(t, 2*time.Second, func() bool {
		got = eventSummaryTypesForRunSession(t, runtime, sessionID)
		for _, want := range wantTypes {
			if !slices.Contains(got, want) {
				return false
			}
		}
		return true
	})
	return got
}

func waitForTaskRuntimeCondition(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()

	if check() {
		return
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if check() {
				return
			}
		case <-timer.C:
			t.Fatal("timed out waiting for task runtime condition")
		}
	}
}

func daemonTaskRecordForTest(id string, now time.Time) taskpkg.Task {
	return taskpkg.Task{
		ID:             id,
		Identifier:     "identifier-" + id,
		Scope:          taskpkg.ScopeGlobal,
		Title:          "Task " + id,
		Priority:       taskpkg.DefaultPriority,
		MaxAttempts:    taskpkg.DefaultTaskMaxAttempts,
		Status:         taskpkg.TaskStatusReady,
		ApprovalPolicy: taskpkg.ApprovalPolicyNone,
		ApprovalState:  taskpkg.ApprovalStateNotRequired,
		CreatedBy:      taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "test"},
		Origin:         taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "daemon.test"},
		CreatedAt:      now,
		UpdatedAt:      now,
		WakeCreator:    true,
	}
}

func testWatchLoopSpec(t *testing.T, name string) looppkg.ResourceSpec {
	t.Helper()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "loop.yaml")
	data := []byte(testWatchLoopYAML(name))
	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		t.Fatalf("WriteFile(loop.yaml) error = %v", err)
	}
	spec, _, err := looppkg.ParseResource(data, looppkg.ResourceParseOptions{
		Source:   looppkg.SourceUser,
		Dir:      dir,
		FilePath: filePath,
	})
	if err != nil {
		t.Fatalf("ParseResource(watch loop) error = %v", err)
	}
	return spec
}

func testWatchLoopYAML(name string) string {
	return `apiVersion: agh.loop/v1
kind: Loop
meta:
  name: ` + name + `
  version: 1
  description: Test daemon watch-source wiring
concurrency: queue
contract:
  goal: Test daemon watch-source wiring
  definition_of_done: Watch source yielded
  terminal_states: [done, blocked, stalled]
  iteration_cap: 3
  no_progress:
    window: 2
    hash_fields: []
  budget:
    tokens: 0
    wall_clock_sec: 0
    on_exceeded: halt
start:
  - kind: cli
graph:
  nodes:
    - id: watch_reviews
      class: source
      kind: watch-source
      watch:
        kind: reviews
        query: open
    - id: normalize_review
      class: action
      kind: transform
      params:
        map:
          review:
            value: ok
  edges:
    - from: watch_reviews
      to: normalize_review
`
}

type watchPollerExtensionRuntime struct {
	poll func(context.Context, watchpkg.PollRequest) (watchpkg.PollResponse, error)
}

func (*watchPollerExtensionRuntime) Start(context.Context) error {
	return nil
}

func (*watchPollerExtensionRuntime) Stop(context.Context) error {
	return nil
}

func (*watchPollerExtensionRuntime) Reload(context.Context) error {
	return nil
}

func (*watchPollerExtensionRuntime) Get(string) (*extensionpkg.Extension, error) {
	return nil, nil
}

func (*watchPollerExtensionRuntime) HookDeclarations(context.Context) ([]hookspkg.HookDecl, error) {
	return nil, nil
}

func (r *watchPollerExtensionRuntime) Poll(
	ctx context.Context,
	req watchpkg.PollRequest,
) (watchpkg.PollResponse, error) {
	if r == nil || r.poll == nil {
		return watchpkg.PollResponse{}, errors.New("watch poller test runtime is not configured")
	}
	return r.poll(ctx, req)
}

func newDetachedHarnessTaskRuntimeForTest(
	t *testing.T,
	sessions *fakeSessionManager,
) (*taskRuntime, workspacepkg.RuntimeResolver, aghconfig.HomePaths) {
	t.Helper()

	if sessions == nil {
		sessions = &fakeSessionManager{}
	}

	db := openDaemonTestGlobalDB(t)
	homePaths := testHomePaths(t)
	resolver, err := workspacepkg.NewResolver(
		db,
		workspacepkg.WithHomePaths(homePaths),
		workspacepkg.WithLogger(discardLogger()),
		workspacepkg.WithConfigLoader(func(rootDir string) (aghconfig.Config, error) {
			return aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(rootDir))
		}),
	)
	if err != nil {
		t.Fatalf("workspace.NewResolver() error = %v", err)
	}
	registeredWorkspaces := make(map[string]struct{})
	for _, info := range sessions.infos {
		if info == nil {
			continue
		}
		workspaceID := strings.TrimSpace(info.WorkspaceID)
		if workspaceID == "" {
			workspaceID = "global"
		}
		if _, ok := registeredWorkspaces[workspaceID]; !ok {
			if err := db.InsertWorkspace(testutil.Context(t), workspacepkg.Workspace{
				ID:        workspaceID,
				Name:      workspaceID,
				RootDir:   filepath.Join(t.TempDir(), workspaceID),
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}); err != nil {
				t.Fatalf("InsertWorkspace(%q) error = %v", workspaceID, err)
			}
			registeredWorkspaces[workspaceID] = struct{}{}
		}
		agentName := strings.TrimSpace(info.AgentName)
		if agentName == "" {
			agentName = "daemon-test-agent"
		}
		storedInfo := store.SessionInfo{
			ID:          info.ID,
			Name:        info.Name,
			AgentName:   agentName,
			WorkspaceID: workspaceID,
			SessionType: string(info.Type),
			State:       string(info.State),
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		storedInfo.SetNetworkSpec(info.NetworkParticipation)
		if err := db.RegisterSession(testutil.Context(t), storedInfo); err != nil {
			t.Fatalf("RegisterSession(%q) error = %v", info.ID, err)
		}
	}

	sessionBridge, err := newTaskSessionBridge(sessions, homePaths.HomeDir, discardLogger())
	if err != nil {
		t.Fatalf("newTaskSessionBridge() error = %v", err)
	}
	harnessResolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		MemoryPromptSectionEnabled: true,
		SkillsPromptSectionEnabled: true,
		SyntheticTurnsEnabled:      true,
		DetachedTaskRuntimeEnabled: true,
	})
	reentry, err := newHarnessReentryBridge(
		testutil.Context(t),
		harnessResolver,
		nil,
		db,
		sessions,
		discardLogger(),
	)
	if err != nil {
		t.Fatalf("newHarnessReentryBridge() error = %v", err)
	}
	t.Cleanup(reentry.shutdown)
	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(db),
		taskpkg.WithSessionExecutor(sessionBridge),
		taskpkg.WithEventObserver(reentry),
		taskpkg.WithCancelGracePeriod(defaultTaskCancelGrace),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	detached, err := newHarnessDetachedWorkBridge(manager, db, sessions)
	if err != nil {
		t.Fatalf("newHarnessDetachedWorkBridge() error = %v", err)
	}

	return &taskRuntime{
		manager:  manager,
		store:    db,
		detached: detached,
		reentry:  reentry,
	}, resolver, homePaths
}
