//go:build integration

package task_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/store/sessiondb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

type integrationStopCall struct {
	SessionID string
	Reason    taskpkg.StopReason
}

type exactRunClaimStore interface {
	GetTaskRun(context.Context, string) (taskpkg.Run, error)
	GetTask(context.Context, string) (taskpkg.Task, error)
	UpdateTaskRun(context.Context, taskpkg.Run) error
}

func claimExactRunIntegration(
	ctx context.Context,
	manager *taskpkg.Service,
	claimStore exactRunClaimStore,
	runID string,
	actor taskpkg.ActorContext,
) (*taskpkg.ClaimResult, error) {
	run, err := claimStore.GetTaskRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	taskRecord, err := claimStore.GetTask(ctx, run.TaskID)
	if err != nil {
		return nil, err
	}
	claimerSessionID := "integration-claim:" + run.ID
	claimerWorkspaceID := strings.TrimSpace(taskRecord.WorkspaceID)
	if claimerWorkspaceID == "" {
		claimerWorkspaceID = strings.TrimSpace(actor.Scope.WorkspaceID)
	}
	if claimerWorkspaceID == "" {
		claimerWorkspaceID = "workspace-integration-claim"
	}
	claimActor, err := taskpkg.DeriveAgentSessionActorContext(claimerSessionID, claimerWorkspaceID)
	if err != nil {
		return nil, err
	}
	result, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID:                run.ID,
		WorkspaceID:          taskRecord.WorkspaceID,
		ClaimerSessionID:     claimerSessionID,
		RequiredCapabilities: append([]string(nil), run.RequiredCapabilities...),
		LeaseDuration:        time.Hour,
	}, claimActor)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func seedNonLeasedClaimedRunIntegration(
	ctx context.Context,
	_ *taskpkg.Service,
	claimStore exactRunClaimStore,
	runID string,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	run, err := claimStore.GetTaskRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	claimedBy := actor.Actor
	run.Status = taskpkg.TaskRunStatusClaimed
	run.ClaimedBy = &claimedBy
	run.ClaimedAt = time.Now().UTC()
	run.SessionID = ""
	run.ClaimTokenHash = ""
	run.LeaseUntil = time.Time{}
	run.HeartbeatAt = time.Time{}
	if err := claimStore.UpdateTaskRun(ctx, run); err != nil {
		return nil, err
	}
	return &run, nil
}

type integrationSessionExecutor struct {
	startCalls       []taskpkg.StartTaskSession
	requestStopCalls []integrationStopCall
	forceStopCalls   []integrationStopCall
}

type integrationRuntimeViewReader struct {
	registry     *globaldb.GlobalDB
	sessionStore map[string]*sessiondb.SessionDB
}

type rollupPublicationFailureStore struct {
	taskpkg.Store
	cancel         context.CancelFunc
	reconcileErr   error
	failDependents atomic.Bool
}

func (s *rollupPublicationFailureStore) CompleteRunLeaseSettlement(
	ctx context.Context,
	completion taskpkg.LeaseCompletion,
) (taskpkg.CompletedRunSettlement, error) {
	settlement, err := s.Store.CompleteRunLeaseSettlement(ctx, completion)
	if err == nil && len(settlement.RolledUpRuns) > 0 {
		s.failDependents.Store(true)
		if s.cancel != nil {
			s.cancel()
		}
	}
	return settlement, err
}

func (s *rollupPublicationFailureStore) ListDependents(
	ctx context.Context,
	taskID string,
) ([]taskpkg.Dependency, error) {
	if s.failDependents.Load() {
		return nil, s.reconcileErr
	}
	return s.Store.ListDependents(ctx, taskID)
}

type countingParticipationResolver struct {
	inner        participation.Resolver
	mu           sync.Mutex
	calls        int
	observations []participation.ResolvedObservation
}

func (r *countingParticipationResolver) ObserveParticipationResolved(
	_ context.Context,
	observation participation.ResolvedObservation,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observations = append(r.observations, observation)
	return nil
}

func (r *countingParticipationResolver) Resolve(
	ctx context.Context,
	in participation.ResolveInput,
) (participation.Spec, error) {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	return r.inner.Resolve(ctx, in)
}

func (r *countingParticipationResolver) CallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func (r *countingParticipationResolver) ObservationCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.observations)
}

func (r *countingParticipationResolver) LastObservation() participation.ResolvedObservation {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.observations) == 0 {
		return participation.ResolvedObservation{}
	}
	return r.observations[len(r.observations)-1]
}

func (e *integrationSessionExecutor) StartTaskSession(
	_ context.Context,
	spec *taskpkg.StartTaskSession,
) (*taskpkg.SessionRef, error) {
	if spec == nil {
		return nil, errors.New("task integration session executor requires start spec")
	}
	e.startCalls = append(e.startCalls, *spec)
	return &taskpkg.SessionRef{SessionID: "sess-int-" + strconv.Itoa(len(e.startCalls))}, nil
}

func (e *integrationSessionExecutor) AttachTaskSession(
	_ context.Context,
	runID string,
	sessionID string,
) (*taskpkg.SessionRef, error) {
	return &taskpkg.SessionRef{SessionID: sessionID}, nil
}

func (e *integrationSessionExecutor) RequestTaskStop(
	_ context.Context,
	sessionID string,
	reason taskpkg.StopReason,
) error {
	e.requestStopCalls = append(e.requestStopCalls, integrationStopCall{SessionID: sessionID, Reason: reason})
	return nil
}

func (e *integrationSessionExecutor) ForceTaskStop(
	_ context.Context,
	sessionID string,
	reason taskpkg.StopReason,
) error {
	e.forceStopCalls = append(e.forceStopCalls, integrationStopCall{SessionID: sessionID, Reason: reason})
	return nil
}

func (r *integrationRuntimeViewReader) GetSession(
	ctx context.Context,
	sessionID string,
) (*taskpkg.RunSessionRef, error) {
	if r == nil || r.registry == nil {
		return nil, taskpkg.ErrTaskRunNotFound
	}

	trimmedSessionID := strings.TrimSpace(sessionID)
	sessions, err := r.registry.ListSessions(ctx, store.SessionListQuery{})
	if err != nil {
		return nil, err
	}
	for _, session := range sessions {
		if session.ID != trimmedSessionID {
			continue
		}
		return &taskpkg.RunSessionRef{
			SessionID:   session.ID,
			WorkspaceID: session.WorkspaceID,
			AgentName:   session.AgentName,
			Name:        session.Name,
			State:       session.State,
			CreatedAt:   session.CreatedAt,
			UpdatedAt:   session.UpdatedAt,
		}, nil
	}
	return nil, taskpkg.ErrTaskRunNotFound
}

func (r *integrationRuntimeViewReader) ListSessionEvents(
	ctx context.Context,
	sessionID string,
	query store.EventQuery,
) ([]store.SessionEvent, error) {
	if r == nil {
		return nil, taskpkg.ErrTaskRunNotFound
	}
	sessionDB := r.sessionStore[strings.TrimSpace(sessionID)]
	if sessionDB == nil {
		return nil, taskpkg.ErrTaskRunNotFound
	}
	return sessionDB.Query(ctx, query)
}

func (r *integrationRuntimeViewReader) ListSessionTokenStats(
	ctx context.Context,
	sessionID string,
) ([]store.TokenStats, error) {
	if r == nil || r.registry == nil {
		return nil, taskpkg.ErrTaskRunNotFound
	}
	return r.registry.ListTokenStats(ctx, store.TokenStatsQuery{SessionID: strings.TrimSpace(sessionID)})
}

func TestTaskManagerCreateTaskPersistsAgentSessionIdentity(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	manager := newTaskManagerIntegration(t, db)

	actor, err := taskpkg.DeriveAgentSessionActorContext("sess-agent-1", "ws-test")
	if err != nil {
		t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
	}

	created, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Investigate task manager",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	stored, err := db.GetTask(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got, want := stored.CreatedBy.Kind, taskpkg.ActorKindAgentSession; got != want {
		t.Fatalf("stored.CreatedBy.Kind = %q, want %q", got, want)
	}
	if got, want := stored.CreatedBy.Ref, "sess-agent-1"; got != want {
		t.Fatalf("stored.CreatedBy.Ref = %q, want %q", got, want)
	}
	if got, want := stored.Origin.Kind, taskpkg.OriginKindAgentSession; got != want {
		t.Fatalf("stored.Origin.Kind = %q, want %q", got, want)
	}
	if got, want := stored.Origin.Ref, "sess-agent-1"; got != want {
		t.Fatalf("stored.Origin.Ref = %q, want %q", got, want)
	}
	if stored.Owner != nil {
		t.Fatalf("stored.Owner = %#v, want nil", stored.Owner)
	}
	if got, want := stored.Status, taskpkg.TaskStatusReady; got != want {
		t.Fatalf("stored.Status = %q, want %q", got, want)
	}

	events, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: stored.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if got, want := events[0].EventType, "task.created"; got != want {
		t.Fatalf("events[0].EventType = %q, want %q", got, want)
	}
	if got, want := events[0].Actor.Kind, taskpkg.ActorKindAgentSession; got != want {
		t.Fatalf("events[0].Actor.Kind = %q, want %q", got, want)
	}
}

func TestTaskManagerRejectsInvalidTaskSemanticsBeforePersistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec taskpkg.CreateTask
	}{
		{
			name: "invalid priority",
			spec: taskpkg.CreateTask{
				Scope:    taskpkg.ScopeGlobal,
				Title:    "Bad priority",
				Priority: taskpkg.Priority("rush"),
			},
		},
		{
			name: "invalid max attempts",
			spec: taskpkg.CreateTask{Scope: taskpkg.ScopeGlobal, Title: "Bad attempts", MaxAttempts: intPtr(0)},
		},
		{
			name: "invalid approval policy",
			spec: taskpkg.CreateTask{
				Scope:          taskpkg.ScopeGlobal,
				Title:          "Bad approval",
				ApprovalPolicy: taskpkg.ApprovalPolicy("auto"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			db := openTaskManagerGlobalDB(t)
			manager := newTaskManagerIntegration(t, db)

			actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task create")
			if err != nil {
				t.Fatalf("DeriveHumanActorContext() error = %v", err)
			}

			_, err = manager.CreateTask(ctx, tt.spec, actor)
			if err == nil {
				t.Fatal("CreateTask() error = nil, want non-nil")
			}
			if !errors.Is(err, taskpkg.ErrValidation) {
				t.Fatalf("CreateTask() error = %v, want %v", err, taskpkg.ErrValidation)
			}

			tasks, err := db.ListTasks(ctx, taskpkg.Query{})
			if err != nil {
				t.Fatalf("ListTasks() error = %v", err)
			}
			if got := len(tasks); got != 0 {
				t.Fatalf("len(tasks) = %d, want 0", got)
			}
		})
	}
}

func TestTaskManagerCreateTaskPersistsAutomationLinkedAgentOrigin(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	manager := newTaskManagerIntegration(t, db)

	actor, err := taskpkg.DeriveAutomationLinkedAgentSessionActorContext(
		"sess-agent-2",
		"ws-test",
		"run:run-2",
	)
	if err != nil {
		t.Fatalf("DeriveAutomationLinkedAgentSessionActorContext() error = %v", err)
	}

	created, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Investigate automation-linked task creation",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	stored, err := db.GetTask(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got, want := stored.CreatedBy.Kind, taskpkg.ActorKindAgentSession; got != want {
		t.Fatalf("stored.CreatedBy.Kind = %q, want %q", got, want)
	}
	if got, want := stored.CreatedBy.Ref, "sess-agent-2"; got != want {
		t.Fatalf("stored.CreatedBy.Ref = %q, want %q", got, want)
	}
	if got, want := stored.Origin.Kind, taskpkg.OriginKindAutomation; got != want {
		t.Fatalf("stored.Origin.Kind = %q, want %q", got, want)
	}
	if got, want := stored.Origin.Ref, "run:run-2"; got != want {
		t.Fatalf("stored.Origin.Ref = %q, want %q", got, want)
	}
}

func TestTaskManagerPublishTaskReconcilesDraftLifecycleIntegration(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	executor := &integrationSessionExecutor{}
	manager := newTaskManagerIntegration(t, db, taskpkg.WithSessionExecutor(executor))

	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task publish")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	blocker, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Blocker",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(blocker) error = %v", err)
	}
	target, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Draft target",
		Draft: true,
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(target) error = %v", err)
	}
	if err := manager.AddDependency(ctx, taskpkg.AddDependency{
		TaskID:          target.ID,
		DependsOnTaskID: blocker.ID,
		Kind:            taskpkg.DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	if _, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: target.ID}, actor); !errors.Is(
		err,
		taskpkg.ErrInvalidStatusTransition,
	) {
		t.Fatalf("EnqueueRun(draft) error = %v, want %v", err, taskpkg.ErrInvalidStatusTransition)
	}

	if _, err := manager.PublishTask(ctx, target.ID, taskpkg.ExecutionRequest{}, actor); !errors.Is(
		err,
		taskpkg.ErrInvalidStatusTransition,
	) {
		t.Fatalf("PublishTask(blocked) error = %v, want %v", err, taskpkg.ErrInvalidStatusTransition)
	}

	blockerRun, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: blocker.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun(blocker) error = %v", err)
	}
	blockerRun, err = seedNonLeasedClaimedRunIntegration(ctx, manager, db, blockerRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(blocker) error = %v", err)
	}
	blockerRun, err = manager.StartRun(ctx, blockerRun.ID, taskpkg.StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(blocker) error = %v", err)
	}
	if _, err := manager.CompleteRun(ctx, blockerRun.ID, taskpkg.RunResult{
		Value: json.RawMessage(`{"ok":true}`),
	}, actor); err != nil {
		t.Fatalf("CompleteRun(blocker) error = %v", err)
	}

	reloadedTarget, err := db.GetTask(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetTask(target) error = %v", err)
	}
	if got, want := reloadedTarget.Status, taskpkg.TaskStatusDraft; got != want {
		t.Fatalf("reloadedTarget.Status = %q, want %q", got, want)
	}

	published, err := manager.PublishTask(ctx, target.ID, taskpkg.ExecutionRequest{}, actor)
	if err != nil {
		t.Fatalf("PublishTask(unblocked) error = %v", err)
	}
	if got, want := published.Task.Status, taskpkg.TaskStatusReady; got != want {
		t.Fatalf("published.Task.Status = %q, want %q", got, want)
	}
	if got, want := published.Run.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("published.Run.Status = %q, want %q", got, want)
	}

	events, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: target.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents(target) error = %v", err)
	}
	if !containsEventType(events, "task.published") {
		t.Fatalf("event types = %#v, want task.published", sortedEventTypes(events))
	}
}

func TestTaskManagerPublishTaskReadModelsStayConsistentAfterReload(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), "agh.db")

	first, err := globaldb.OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(first) error = %v", err)
	}

	executor := &integrationSessionExecutor{}
	firstManager := newTaskManagerIntegration(t, first, taskpkg.WithSessionExecutor(executor))
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task publish")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	blocker, err := firstManager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:      taskpkg.ScopeGlobal,
		Title:      "Release blocker",
		Identifier: "OPS-100",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(blocker) error = %v", err)
	}
	target, err := firstManager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:      taskpkg.ScopeGlobal,
		Title:      "Draft target",
		Identifier: "OPS-300",
		Draft:      true,
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(target) error = %v", err)
	}
	if err := firstManager.AddDependency(ctx, taskpkg.AddDependency{
		TaskID:          target.ID,
		DependsOnTaskID: blocker.ID,
		Kind:            taskpkg.DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}
	if _, err := firstManager.PublishTask(ctx, target.ID, taskpkg.ExecutionRequest{}, actor); !errors.Is(
		err,
		taskpkg.ErrInvalidStatusTransition,
	) {
		t.Fatalf("PublishTask(blocked) error = %v, want %v", err, taskpkg.ErrInvalidStatusTransition)
	}

	blockerRun, err := firstManager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: blocker.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun(blocker) error = %v", err)
	}
	blockerRun, err = seedNonLeasedClaimedRunIntegration(ctx, firstManager, first, blockerRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(blocker) error = %v", err)
	}
	blockerRun, err = firstManager.StartRun(ctx, blockerRun.ID, taskpkg.StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(blocker) error = %v", err)
	}
	if _, err := firstManager.CompleteRun(ctx, blockerRun.ID, taskpkg.RunResult{
		Value: json.RawMessage(`{"ok":true}`),
	}, actor); err != nil {
		t.Fatalf("CompleteRun(blocker) error = %v", err)
	}

	published, err := firstManager.PublishTask(ctx, target.ID, taskpkg.ExecutionRequest{}, actor)
	if err != nil {
		t.Fatalf("PublishTask(unblocked) error = %v", err)
	}
	if got, want := published.Task.Status, taskpkg.TaskStatusReady; got != want {
		t.Fatalf("published.Task.Status = %q, want %q", got, want)
	}
	if got, want := published.Run.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("published.Run.Status = %q, want %q", got, want)
	}

	if err := first.Close(ctx); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	second, err := globaldb.OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(second) error = %v", err)
	}
	t.Cleanup(func() {
		if err := second.Close(ctx); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
	})

	secondManager := newTaskManagerIntegration(t, second)

	view, err := secondManager.GetTask(ctx, target.ID, actor)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got, want := view.Task.Status, taskpkg.TaskStatusReady; got != want {
		t.Fatalf("view.Task.Status = %q, want %q", got, want)
	}
	if got, want := view.Summary.Status, taskpkg.TaskStatusReady; got != want {
		t.Fatalf("view.Summary.Status = %q, want %q", got, want)
	}
	if got, want := view.Summary.DependencyCount, int32(1); got != want {
		t.Fatalf("view.Summary.DependencyCount = %d, want %d", got, want)
	}
	if len(view.DependencyReferences) != 1 {
		t.Fatalf("len(view.DependencyReferences) = %d, want 1", len(view.DependencyReferences))
	}
	if got, want := view.DependencyReferences[0].DependsOn.Identifier, blocker.Identifier; got != want {
		t.Fatalf("view.DependencyReferences[0].DependsOn.Identifier = %q, want %q", got, want)
	}

	summaries, err := secondManager.ListTasks(ctx, taskpkg.Query{
		Status: taskpkg.TaskStatusReady,
		Search: "ops-300",
	}, actor)
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(summaries) != 1 || summaries[0].ID != target.ID {
		t.Fatalf("ListTasks() = %#v, want only %q", summaries, target.ID)
	}
	if got, want := summaries[0].Status, taskpkg.TaskStatusReady; got != want {
		t.Fatalf("summaries[0].Status = %q, want %q", got, want)
	}
	if got, want := summaries[0].Dependencies[0].DependsOn.Title, blocker.Title; got != want {
		t.Fatalf("summaries[0].Dependencies[0].DependsOn.Title = %q, want %q", got, want)
	}
}

func TestTaskManagerTriageMutationsRemainActorScopedAfterReload(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), "agh.db")

	first, err := globaldb.OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(first) error = %v", err)
	}

	firstManager := newTaskManagerIntegration(t, first)
	alice, err := taskpkg.DeriveHumanActorContext("alice", taskpkg.OriginKindCLI, "agh task inbox")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext(alice) error = %v", err)
	}
	bob, err := taskpkg.DeriveHumanActorContext("bob", taskpkg.OriginKindCLI, "agh task inbox")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext(bob) error = %v", err)
	}

	taskRecord, err := firstManager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Persist triage state",
	}, alice)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	if _, err := firstManager.MarkTaskRead(ctx, taskRecord.ID, alice); err != nil {
		t.Fatalf("MarkTaskRead(alice) error = %v", err)
	}
	archivedState, err := firstManager.ArchiveTask(ctx, taskRecord.ID, alice)
	if err != nil {
		t.Fatalf("ArchiveTask(alice) error = %v", err)
	}
	dismissedState, err := firstManager.DismissTask(ctx, taskRecord.ID, bob)
	if err != nil {
		t.Fatalf("DismissTask(bob) error = %v", err)
	}
	if !archivedState.Archived || !dismissedState.Dismissed {
		t.Fatalf("triage states = %#v / %#v, want archived and dismissed states", archivedState, dismissedState)
	}

	if err := first.Close(ctx); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	second, err := globaldb.OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(second) error = %v", err)
	}
	t.Cleanup(func() {
		if err := second.Close(ctx); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
	})

	storedAlice, err := second.GetTaskTriageState(ctx, taskRecord.ID, alice.Actor)
	if err != nil {
		t.Fatalf("GetTaskTriageState(alice) error = %v", err)
	}
	if storedAlice != archivedState {
		t.Fatalf("storedAlice = %#v, want %#v", storedAlice, archivedState)
	}
	storedBob, err := second.GetTaskTriageState(ctx, taskRecord.ID, bob.Actor)
	if err != nil {
		t.Fatalf("GetTaskTriageState(bob) error = %v", err)
	}
	if storedBob != dismissedState {
		t.Fatalf("storedBob = %#v, want %#v", storedBob, dismissedState)
	}
}

func TestTaskManagerApprovalGateAndAttemptExhaustionIntegration(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	executor := &integrationSessionExecutor{}
	manager := newTaskManagerIntegration(t, db, taskpkg.WithSessionExecutor(executor))

	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task run")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:          taskpkg.ScopeGlobal,
		Title:          "Approval-gated task",
		ApprovalPolicy: taskpkg.ApprovalPolicyManual,
		MaxAttempts:    intPtr(1),
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if got, want := taskRecord.Status, taskpkg.TaskStatusBlocked; got != want {
		t.Fatalf("taskRecord.Status = %q, want %q", got, want)
	}

	runsBeforeApproval, err := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskRuns(before approval) error = %v", err)
	}
	if len(runsBeforeApproval) != 0 {
		t.Fatalf("runs before approval = %d, want 0", len(runsBeforeApproval))
	}

	approved, err := manager.ApproveTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{}, actor)
	if err != nil {
		t.Fatalf("ApproveTask() error = %v", err)
	}
	if got, want := approved.Task.ApprovalState, taskpkg.ApprovalStateApproved; got != want {
		t.Fatalf("approved.Task.ApprovalState = %q, want %q", got, want)
	}
	if got, want := approved.Run.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("approved.Run.Status = %q, want %q", got, want)
	}

	run := approved.Run
	claimedRun, err := seedNonLeasedClaimedRunIntegration(ctx, manager, db, run.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(approved) error = %v", err)
	}
	startedRun, err := manager.StartRun(ctx, claimedRun.ID, taskpkg.StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	if _, err := manager.FailRun(ctx, startedRun.ID, taskpkg.RunFailure{
		Error: "approval path failed",
	}, actor); err != nil {
		t.Fatalf("FailRun() error = %v", err)
	}

	reloaded, err := db.GetTask(ctx, taskRecord.ID)
	if err != nil {
		t.Fatalf("GetTask(reloaded) error = %v", err)
	}
	if got, want := reloaded.Status, taskpkg.TaskStatusFailed; got != want {
		t.Fatalf("reloaded.Status = %q, want %q", got, want)
	}
	if got, want := reloaded.ApprovalState, taskpkg.ApprovalStateApproved; got != want {
		t.Fatalf("reloaded.ApprovalState = %q, want %q", got, want)
	}

	if _, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, actor); !errors.Is(
		err,
		taskpkg.ErrInvalidStatusTransition,
	) {
		t.Fatalf("EnqueueRun(exhausted) error = %v, want %v", err, taskpkg.ErrInvalidStatusTransition)
	}

	events, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents() error = %v", err)
	}
	if !containsEventType(events, "task.approved") {
		t.Fatalf("event types = %#v, want task.approved", sortedEventTypes(events))
	}

	t.Run("Should approve and claim the existing gated run without duplication", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		manager := newTaskManagerIntegration(t, db)
		operator, err := taskpkg.DeriveHumanActorContext(
			"approval-operator",
			taskpkg.OriginKindCLI,
			"agh task approve",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		gatedTask, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:          taskpkg.ScopeGlobal,
			Title:          "Pre-enqueued approval integration",
			ApprovalPolicy: taskpkg.ApprovalPolicyManual,
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		pendingRun, err := manager.EnqueueRun(
			ctx,
			taskpkg.EnqueueRun{TaskID: gatedTask.ID},
			operator,
		)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}

		approved, err := manager.ApproveTask(
			ctx,
			gatedTask.ID,
			taskpkg.ExecutionRequest{},
			operator,
		)
		if err != nil {
			t.Fatalf("ApproveTask() error = %v", err)
		}
		if got, want := approved.Run.ID, pendingRun.ID; got != want {
			t.Fatalf("ApproveTask().Run.ID = %q, want existing %q", got, want)
		}
		if !approved.ExistingRun {
			t.Fatal("ApproveTask().ExistingRun = false, want true")
		}
		runs, err := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: gatedTask.ID})
		if err != nil {
			t.Fatalf("ListTaskRuns() error = %v", err)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("runs after approval = %d, want %d", got, want)
		}

		worker, err := taskpkg.DeriveAgentSessionActorContext("sess-approval-worker", "ws-test")
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
		}
		claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-approval-worker",
			LeaseDuration:    time.Minute,
		}, worker)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		if got, want := claim.Run.ID, pendingRun.ID; got != want {
			t.Fatalf("ClaimNextRun().Run.ID = %q, want %q", got, want)
		}
	})

	t.Run("Should stop a repeated expired-lease recovery loop at max attempts", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		manager := newTaskManagerIntegration(t, db)
		operator, err := taskpkg.DeriveHumanActorContext(
			"lease-recovery-operator",
			taskpkg.OriginKindCLI,
			"agh task recover",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		worker, err := taskpkg.DeriveAgentSessionActorContext("sess-crash-loop", "ws-test")
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
		}
		maxAttempts := 2
		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeGlobal,
			Title:       "Bound expired lease recovery",
			MaxAttempts: &maxAttempts,
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, operator)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		now := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
		firstClaim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID:            run.ID,
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-crash-loop",
			LeaseDuration:    time.Minute,
			Now:              now,
		}, worker)
		if err != nil {
			t.Fatalf("ClaimNextRun(first) error = %v", err)
		}
		firstRecoveryAt := firstClaim.LeaseUntil.Add(time.Second)
		firstRecovery, err := manager.RecoverExpiredRunLeases(ctx, taskpkg.ExpiredLeaseRecovery{
			Now:    firstRecoveryAt,
			Reason: "worker_crashed",
		}, operator)
		if err != nil {
			t.Fatalf("RecoverExpiredRunLeases(first) error = %v", err)
		}
		if len(firstRecovery) != 1 || firstRecovery[0].Exhausted ||
			firstRecovery[0].Run.RecoveryCount != 1 {
			t.Fatalf("first recovery = %#v, want one requeued recovery", firstRecovery)
		}

		secondClaim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			RunID:            run.ID,
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-crash-loop",
			LeaseDuration:    time.Minute,
			Now:              firstRecoveryAt.Add(time.Second),
		}, worker)
		if err != nil {
			t.Fatalf("ClaimNextRun(second) error = %v", err)
		}
		secondRecovery, err := manager.RecoverExpiredRunLeases(ctx, taskpkg.ExpiredLeaseRecovery{
			Now:    secondClaim.LeaseUntil.Add(time.Second),
			Reason: "worker_crashed",
		}, operator)
		if err != nil {
			t.Fatalf("RecoverExpiredRunLeases(second) error = %v", err)
		}
		if len(secondRecovery) != 1 || !secondRecovery[0].Exhausted ||
			secondRecovery[0].Run.Status != taskpkg.TaskRunStatusNeedsAttention {
			t.Fatalf("second recovery = %#v, want one exhausted run", secondRecovery)
		}
		if secondRecovery[0].Run.Error != taskpkg.LeaseRecoveryExhaustedReason {
			t.Fatalf("exhausted reason = %q, want %q", secondRecovery[0].Run.Error, taskpkg.LeaseRecoveryExhaustedReason)
		}

		escalated, err := db.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if escalated.NeedsAttention == nil ||
			escalated.NeedsAttention.Reason != taskpkg.LeaseRecoveryExhaustedReason {
			t.Fatalf("NeedsAttention = %#v, want lease recovery exhaustion", escalated.NeedsAttention)
		}
	})
}

func TestTaskManagerPlainWorkspaceStartPersistsLocalParticipationIntegration(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	workspaceID := registerTaskManagerWorkspace(t, db, "start-boundary", filepath.Join(t.TempDir(), "workspace"))
	manager := newTaskManagerIntegration(
		t,
		db,
		taskpkg.WithParticipationResolver(newTaskParticipationResolver(t, db, nil)),
	)

	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task start")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: workspaceID,
		Title:       "Manual start boundary",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	runsBefore, err := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskRuns(before start) error = %v", err)
	}
	if len(runsBefore) != 0 {
		t.Fatalf("runs before start = %d, want 0", len(runsBefore))
	}
	channelsBefore, err := db.ListNetworkChannels(ctx, store.NetworkChannelQuery{WorkspaceID: workspaceID})
	if err != nil {
		t.Fatalf("ListNetworkChannels(before start) error = %v", err)
	}
	if len(channelsBefore) != 0 {
		t.Fatalf("channels before start = %d, want 0", len(channelsBefore))
	}

	execution, err := manager.StartTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{}, actor)
	if err != nil {
		t.Fatalf("StartTask() error = %v", err)
	}
	if got, want := execution.Run.NetworkSpecSnapshot().Mode, participation.ModeLocal; got != want {
		t.Fatalf("StartTask().Run.NetworkSpecSnapshot().Mode = %q, want %q", got, want)
	}
	if got, want := execution.Run.NetworkSpecSnapshot().Source, participation.SourceBuiltInLocal; got != want {
		t.Fatalf("StartTask().Run.NetworkSpecSnapshot().Source = %q, want %q", got, want)
	}
	channelsAfter, err := db.ListNetworkChannels(ctx, store.NetworkChannelQuery{WorkspaceID: workspaceID})
	if err != nil {
		t.Fatalf("ListNetworkChannels(after start) error = %v", err)
	}
	if len(channelsAfter) != 0 {
		t.Fatalf("channels after start = %#v, want none", channelsAfter)
	}
	var conversationRows int
	if err := db.DB().QueryRowContext(
		ctx,
		"SELECT (SELECT COUNT(*) FROM network_timeline_log) + (SELECT COUNT(*) FROM network_threads) + (SELECT COUNT(*) FROM network_direct_rooms)",
	).
		Scan(&conversationRows); err != nil {
		t.Fatalf("count network conversation rows: %v", err)
	}
	if conversationRows != 0 {
		t.Fatalf("network conversation rows = %d, want 0", conversationRows)
	}

	claimActor, err := taskpkg.DeriveAgentSessionActorContext("sess-worker", workspaceID)
	if err != nil {
		t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
	}
	claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      workspaceID,
		ClaimerSessionID: "sess-worker",
		LeaseDuration:    time.Minute,
	}, claimActor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	if got, want := claim.Run.ID, execution.Run.ID; got != want {
		t.Fatalf("ClaimNextRun().Run.ID = %q, want %q", got, want)
	}
	if claim.CoordinationChannel != nil {
		t.Fatalf("ClaimNextRun().CoordinationChannel = %#v, want nil", claim.CoordinationChannel)
	}
}

func TestTaskManagerGlobalLocalParticipationIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should approve a global task without workspace participation resolution", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		resolver := &countingParticipationResolver{
			inner: newTaskParticipationResolver(t, db, nil),
		}
		manager := newTaskManagerIntegration(t, db, taskpkg.WithParticipationResolver(resolver))
		actor, err := taskpkg.DeriveHumanActorContext(
			"global-operator",
			taskpkg.OriginKindCLI,
			"agh task approve",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:          taskpkg.ScopeGlobal,
			Title:          "Global approval boundary",
			ApprovalPolicy: taskpkg.ApprovalPolicyManual,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		execution, err := manager.ApproveTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{}, actor)
		if err != nil {
			t.Fatalf("ApproveTask() error = %v", err)
		}
		if got, want := execution.Run.NetworkSpecSnapshot(), participation.LocalSpec(); got != want {
			t.Fatalf("ApproveTask().Run.NetworkSpecSnapshot() = %#v, want %#v", got, want)
		}
		if got := resolver.CallCount(); got != 0 {
			t.Fatalf("participation resolver calls = %d, want 0", got)
		}
		if got := resolver.ObservationCount(); got != 0 {
			t.Fatalf("participation observations = %d, want 0", got)
		}
	})

	t.Run("Should preserve an explicit local source for a global task", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		resolver := &countingParticipationResolver{
			inner: newTaskParticipationResolver(t, db, nil),
		}
		manager := newTaskManagerIntegration(t, db, taskpkg.WithParticipationResolver(resolver))
		actor, err := taskpkg.DeriveHumanActorContext(
			"global-operator",
			taskpkg.OriginKindCLI,
			"agh task start",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Global explicit local boundary",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		local := participation.ModeLocal

		execution, err := manager.StartTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{
			NetworkParticipation: &participation.Request{Mode: &local},
		}, actor)
		if err != nil {
			t.Fatalf("StartTask() error = %v", err)
		}
		if got, want := execution.Run.NetworkSpecSnapshot().Source, participation.SourceExplicitRequest; got != want {
			t.Fatalf("StartTask().Run.NetworkSpecSnapshot().Source = %q, want %q", got, want)
		}
		if got := resolver.CallCount(); got != 0 {
			t.Fatalf("participation resolver calls = %d, want 0", got)
		}
	})

	t.Run("Should delegate global live participation before reserving a run", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		resolver := &countingParticipationResolver{
			inner: newTaskParticipationResolver(t, db, nil),
		}
		manager := newTaskManagerIntegration(t, db, taskpkg.WithParticipationResolver(resolver))
		actor, err := taskpkg.DeriveHumanActorContext(
			"global-operator",
			taskpkg.OriginKindCLI,
			"agh task start",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Global live boundary",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		live := participation.ModeLive
		strategy := participation.StrategyRun

		_, err = manager.StartTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{
			NetworkParticipation: &participation.Request{
				Mode:            &live,
				ChannelStrategy: &strategy,
			},
		}, actor)
		if err == nil || !strings.Contains(err.Error(), "workspace_id is required") {
			t.Fatalf("StartTask() error = %v, want workspace participation resolution failure", err)
		}
		runs, err := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
		if err != nil {
			t.Fatalf("ListTaskRuns() error = %v", err)
		}
		if len(runs) != 0 {
			t.Fatalf("runs after rejected live participation = %#v, want none", runs)
		}
		if got := resolver.CallCount(); got != 1 {
			t.Fatalf("participation resolver calls = %d, want 1", got)
		}
	})
}

func TestTaskManagerParticipationPrecedenceAndWorkspaceToggleIntegration(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	workspaceID := registerTaskManagerWorkspace(
		t,
		db,
		"participation-precedence",
		filepath.Join(t.TempDir(), "workspace"),
	)
	baseResolver := newTaskParticipationResolver(t, db, nil)
	resolver := &countingParticipationResolver{inner: baseResolver}
	manager := newTaskManagerIntegration(t, db, taskpkg.WithParticipationResolver(resolver))
	actor, err := taskpkg.DeriveHumanActorContext(
		"operator",
		taskpkg.OriginKindCLI,
		"agh task participation integration",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	live := participation.ModeLive
	runStrategy := participation.StrategyRun
	local := participation.ModeLocal
	liveRequest := &participation.Request{Mode: &live, ChannelStrategy: &runStrategy}
	localRequest := &participation.Request{Mode: &local}

	t.Run("Should resolve an explicit live override once over a local task profile", func(t *testing.T) {
		taskRecord, createErr := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Title:       "Explicit override",
		}, actor)
		if createErr != nil {
			t.Fatalf("CreateTask() error = %v", createErr)
		}
		profile, profileErr := manager.GetExecutionProfile(ctx, taskRecord.ID, actor)
		if profileErr != nil {
			t.Fatalf("GetExecutionProfile() error = %v", profileErr)
		}
		profile.NetworkParticipation = localRequest
		if _, profileErr = manager.SetExecutionProfile(ctx, taskRecord.ID, &profile, actor); profileErr != nil {
			t.Fatalf("SetExecutionProfile() error = %v", profileErr)
		}

		callsBefore := resolver.CallCount()
		observationsBefore := resolver.ObservationCount()
		first, startErr := manager.StartTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{
			IdempotencyKey:       "explicit-live-override",
			NetworkParticipation: liveRequest,
		}, actor)
		if startErr != nil {
			t.Fatalf("StartTask(first) error = %v", startErr)
		}
		second, startErr := manager.StartTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{
			IdempotencyKey:       "explicit-live-override",
			NetworkParticipation: liveRequest,
		}, actor)
		if startErr != nil {
			t.Fatalf("StartTask(duplicate) error = %v", startErr)
		}
		if got, want := second.Run.ID, first.Run.ID; got != want {
			t.Fatalf("duplicate run id = %q, want %q", got, want)
		}
		if got, want := resolver.CallCount()-callsBefore, 1; got != want {
			t.Fatalf("resolver calls = %d, want %d", got, want)
		}
		if got, want := resolver.ObservationCount()-observationsBefore, 1; got != want {
			t.Fatalf("committed participation observations = %d, want %d", got, want)
		}
		observation := resolver.LastObservation()
		if observation.Owner.ID != first.Run.ID || observation.Owner.WorkspaceID != workspaceID ||
			observation.Spec != first.Run.NetworkSpecSnapshot() {
			t.Fatalf("committed participation observation = %#v, want first persisted run", observation)
		}
		spec := first.Run.NetworkSpecSnapshot()
		if got, want := spec.Mode, participation.ModeLive; got != want {
			t.Fatalf("snapshot mode = %q, want %q", got, want)
		}
		if got, want := spec.Source, participation.SourceExplicitRequest; got != want {
			t.Fatalf("snapshot source = %q, want %q", got, want)
		}
		if spec.ChannelID == "" {
			t.Fatal("snapshot channel id = empty, want run-derived channel")
		}
		runs, listErr := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
		if listErr != nil {
			t.Fatalf("ListTaskRuns() error = %v", listErr)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("len(runs) = %d, want %d", got, want)
		}
	})

	t.Run("Should let an explicit local request override a live task profile", func(t *testing.T) {
		taskRecord, createErr := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Title:       "Explicit local override",
		}, actor)
		if createErr != nil {
			t.Fatalf("CreateTask() error = %v", createErr)
		}
		profile, profileErr := manager.GetExecutionProfile(ctx, taskRecord.ID, actor)
		if profileErr != nil {
			t.Fatalf("GetExecutionProfile() error = %v", profileErr)
		}
		profile.NetworkParticipation = liveRequest
		if _, profileErr = manager.SetExecutionProfile(ctx, taskRecord.ID, &profile, actor); profileErr != nil {
			t.Fatalf("SetExecutionProfile() error = %v", profileErr)
		}
		execution, startErr := manager.StartTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{
			IdempotencyKey:       "explicit-local-override",
			NetworkParticipation: localRequest,
		}, actor)
		if startErr != nil {
			t.Fatalf("StartTask() error = %v", startErr)
		}
		spec := execution.Run.NetworkSpecSnapshot()
		if got, want := spec.Mode, participation.ModeLocal; got != want {
			t.Fatalf("snapshot mode = %q, want %q", got, want)
		}
		if got, want := spec.Source, participation.SourceExplicitRequest; got != want {
			t.Fatalf("snapshot source = %q, want %q", got, want)
		}
	})

	t.Run("Should reject an unauthorized override before reserving a run", func(t *testing.T) {
		denyResolver := newTaskParticipationResolver(
			t,
			db,
			func(context.Context, participation.ResolveInput, participation.Spec) (bool, error) {
				return false, nil
			},
		)
		deniedManager := newTaskManagerIntegration(
			t,
			db,
			taskpkg.WithParticipationResolver(denyResolver),
		)
		taskRecord, createErr := deniedManager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Title:       "Denied override",
		}, actor)
		if createErr != nil {
			t.Fatalf("CreateTask() error = %v", createErr)
		}
		_, startErr := deniedManager.StartTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{
			IdempotencyKey:       "denied-live-override",
			NetworkParticipation: liveRequest,
		}, actor)
		if !errors.Is(startErr, participation.ErrAuthorityDenied) {
			t.Fatalf("StartTask() error = %v, want %v", startErr, participation.ErrAuthorityDenied)
		}
		runs, listErr := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
		if listErr != nil {
			t.Fatalf("ListTaskRuns() error = %v", listErr)
		}
		if len(runs) != 0 {
			t.Fatalf("runs after authority denial = %#v, want none", runs)
		}
	})

	t.Run("Should apply workspace coordination only to future runs", func(t *testing.T) {
		if setErr := setWorkspaceCoordinationIntegration(ctx, db, workspaceID, true, actor); setErr != nil {
			t.Fatalf("CoordinationCommands.Set(true) error = %v", setErr)
		}
		firstTask, createErr := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Title:       "Coordination enabled",
		}, actor)
		if createErr != nil {
			t.Fatalf("CreateTask(first) error = %v", createErr)
		}
		first, startErr := manager.StartTask(ctx, firstTask.ID, taskpkg.ExecutionRequest{
			IdempotencyKey: "workspace-coordination-enabled",
		}, actor)
		if startErr != nil {
			t.Fatalf("StartTask(first) error = %v", startErr)
		}
		firstSpec := first.Run.NetworkSpecSnapshot()
		if got, want := firstSpec.Source, participation.SourceWorkspaceCoordination; got != want {
			t.Fatalf("first source = %q, want %q", got, want)
		}

		profileTask, createErr := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Title:       "Profile overrides workspace",
		}, actor)
		if createErr != nil {
			t.Fatalf("CreateTask(profile override) error = %v", createErr)
		}
		profile, profileErr := manager.GetExecutionProfile(ctx, profileTask.ID, actor)
		if profileErr != nil {
			t.Fatalf("GetExecutionProfile() error = %v", profileErr)
		}
		profile.NetworkParticipation = localRequest
		if _, profileErr = manager.SetExecutionProfile(ctx, profileTask.ID, &profile, actor); profileErr != nil {
			t.Fatalf("SetExecutionProfile() error = %v", profileErr)
		}
		profileExecution, startErr := manager.StartTask(ctx, profileTask.ID, taskpkg.ExecutionRequest{
			IdempotencyKey: "profile-overrides-workspace",
		}, actor)
		if startErr != nil {
			t.Fatalf("StartTask(profile override) error = %v", startErr)
		}
		if got, want := profileExecution.Run.NetworkSpecSnapshot().Source, participation.SourceTaskProfile; got != want {
			t.Fatalf("profile source = %q, want %q", got, want)
		}

		if setErr := setWorkspaceCoordinationIntegration(ctx, db, workspaceID, false, actor); setErr != nil {
			t.Fatalf("CoordinationCommands.Set(false) error = %v", setErr)
		}
		secondTask, createErr := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Title:       "Coordination disabled",
		}, actor)
		if createErr != nil {
			t.Fatalf("CreateTask(second) error = %v", createErr)
		}
		second, startErr := manager.StartTask(ctx, secondTask.ID, taskpkg.ExecutionRequest{
			IdempotencyKey: "workspace-coordination-disabled",
		}, actor)
		if startErr != nil {
			t.Fatalf("StartTask(second) error = %v", startErr)
		}
		if got, want := second.Run.NetworkSpecSnapshot().Source, participation.SourceBuiltInLocal; got != want {
			t.Fatalf("second source = %q, want %q", got, want)
		}
		persistedFirst, readErr := db.GetTaskRun(ctx, first.Run.ID)
		if readErr != nil {
			t.Fatalf("GetTaskRun(first) error = %v", readErr)
		}
		if got, want := persistedFirst.NetworkSpecSnapshot(), firstSpec; got != want {
			t.Fatalf("first snapshot after toggle = %#v, want %#v", got, want)
		}
	})

	t.Run("Should share one derived conversation across a designated fan-out group", func(t *testing.T) {
		if setErr := setWorkspaceCoordinationIntegration(ctx, db, workspaceID, true, actor); setErr != nil {
			t.Fatalf("CoordinationCommands.Set(true) error = %v", setErr)
		}
		fanOutTask, createErr := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Title:       "Shared coordinated fan-out",
		}, actor)
		if createErr != nil {
			t.Fatalf("CreateTask(fan-out) error = %v", createErr)
		}
		const groupID = "tdg-shared-conversation"
		runs := make([]taskpkg.Run, 0, 3)
		for index := range 3 {
			run, enqueueErr := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{
				TaskID:             fanOutTask.ID,
				IdempotencyKey:     fmt.Sprintf("shared-fanout-%d", index),
				DesignationGroupID: groupID,
			}, actor)
			if enqueueErr != nil {
				t.Fatalf("EnqueueRun(fan-out %d) error = %v", index, enqueueErr)
			}
			runs = append(runs, *run)
		}
		sharedChannel := runs[0].NetworkSpecSnapshot().ChannelID
		if sharedChannel == "" {
			t.Fatal("fan-out channel = empty, want group-derived conversation")
		}
		for index, run := range runs {
			spec := run.NetworkSpecSnapshot()
			if spec.Source != participation.SourceWorkspaceCoordination || spec.ChannelID != sharedChannel {
				t.Fatalf("fan-out run %d spec = %#v, want one workspace-coordination channel %q", index, spec, sharedChannel)
			}
		}

		independentTask, createErr := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Title:       "Independent coordinated fan-out",
		}, actor)
		if createErr != nil {
			t.Fatalf("CreateTask(independent fan-out) error = %v", createErr)
		}
		independent, enqueueErr := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{
			TaskID:             independentTask.ID,
			IdempotencyKey:     "independent-fanout-0",
			DesignationGroupID: "tdg-independent-conversation",
		}, actor)
		if enqueueErr != nil {
			t.Fatalf("EnqueueRun(independent fan-out) error = %v", enqueueErr)
		}
		if got := independent.NetworkSpecSnapshot().ChannelID; got == "" || got == sharedChannel {
			t.Fatalf("independent fan-out channel = %q, want non-empty channel distinct from %q", got, sharedChannel)
		}
	})

	channels, err := db.ListNetworkChannels(ctx, store.NetworkChannelQuery{WorkspaceID: workspaceID})
	if err != nil {
		t.Fatalf("ListNetworkChannels() error = %v", err)
	}
	if len(channels) != 0 {
		t.Fatalf("network channels = %#v, want none from owner resolution", channels)
	}
}

func setWorkspaceCoordinationIntegration(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	enabled bool,
	actor taskpkg.ActorContext,
) error {
	commands := aghworkspace.NewCoordinationService(db)
	ref := aghworkspace.CoordinationRef{
		WorkspaceID: workspaceID,
		ScopeKind:   aghworkspace.InvitationScopeWorkspace,
	}
	current, err := commands.Get(ctx, ref, actor)
	if err != nil {
		return err
	}
	_, err = commands.Set(ctx, aghworkspace.SetCoordination{
		Ref:              ref,
		Enabled:          enabled,
		ExpectedRevision: current.Setting.Revision,
	}, actor)
	return err
}

func TestTaskManagerAutoEnqueueOnReadyEnqueuesDependentOnCompletionIntegration(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should auto-enqueue an opted-in dependent and skip an opted-out one on blocker completion",
		func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			db := openTaskManagerGlobalDB(t)
			manager := newTaskManagerIntegration(t, db)

			actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task create")
			if err != nil {
				t.Fatalf("DeriveHumanActorContext() error = %v", err)
			}

			blocker, err := manager.CreateTask(
				ctx,
				taskpkg.CreateTask{Scope: taskpkg.ScopeGlobal, Title: "Blocker"},
				actor,
			)
			if err != nil {
				t.Fatalf("CreateTask(blocker) error = %v", err)
			}
			optedIn, err := manager.CreateTask(ctx, taskpkg.CreateTask{
				Scope:              taskpkg.ScopeGlobal,
				Title:              "Opted-in dependent",
				AutoEnqueueOnReady: true,
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask(opted-in) error = %v", err)
			}
			optedOut, err := manager.CreateTask(
				ctx,
				taskpkg.CreateTask{Scope: taskpkg.ScopeGlobal, Title: "Opted-out dependent"},
				actor,
			)
			if err != nil {
				t.Fatalf("CreateTask(opted-out) error = %v", err)
			}
			for _, dependentID := range []string{optedIn.ID, optedOut.ID} {
				if err := manager.AddDependency(ctx, taskpkg.AddDependency{
					TaskID:          dependentID,
					DependsOnTaskID: blocker.ID,
					Kind:            taskpkg.DependencyKindBlocks,
				}, actor); err != nil {
					t.Fatalf("AddDependency(%s) error = %v", dependentID, err)
				}
			}

			// Complete the blocker through the lease path (the autonomy worker flow that
			// pool-owned phase workers use: claim_next -> ... -> task_run_complete).
			blockerRun, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: blocker.ID}, actor)
			if err != nil {
				t.Fatalf("EnqueueRun(blocker) error = %v", err)
			}
			worker, err := taskpkg.DeriveAgentSessionActorContext("sess-worker", "ws-test")
			if err != nil {
				t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
			}
			claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				Scope:            taskpkg.ScopeGlobal,
				ClaimerSessionID: "sess-worker",
				LeaseDuration:    time.Minute,
			}, worker)
			if err != nil {
				t.Fatalf("ClaimNextRun() error = %v", err)
			}
			if got, want := claim.Run.ID, blockerRun.ID; got != want {
				t.Fatalf("claimed run = %q, want blocker run %q", got, want)
			}
			if _, err := manager.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
				RunID:      claim.Run.ID,
				ClaimToken: claim.ClaimToken,
				Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
			}, worker); err != nil {
				t.Fatalf("CompleteRunLease() error = %v", err)
			}

			optedInRuns, err := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: optedIn.ID})
			if err != nil {
				t.Fatalf("ListTaskRuns(opted-in) error = %v", err)
			}
			if len(optedInRuns) != 1 {
				t.Fatalf("opted-in dependent runs = %d, want 1 auto-enqueued run", len(optedInRuns))
			}
			if got, want := optedInRuns[0].Status, taskpkg.TaskRunStatusQueued; got != want {
				t.Fatalf("auto-enqueued run status = %q, want %q", got, want)
			}

			optedOutRuns, err := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: optedOut.ID})
			if err != nil {
				t.Fatalf("ListTaskRuns(opted-out) error = %v", err)
			}
			if len(optedOutRuns) != 0 {
				t.Fatalf("opted-out dependent runs = %d, want 0", len(optedOutRuns))
			}
		},
	)

	t.Run(
		"Should enqueue exactly one run when two distinct blockers of one dependent complete concurrently",
		func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			db := openTaskManagerGlobalDB(t)
			manager := newTaskManagerIntegration(t, db)

			actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task create")
			if err != nil {
				t.Fatalf("DeriveHumanActorContext() error = %v", err)
			}

			blockerA, err := manager.CreateTask(
				ctx,
				taskpkg.CreateTask{Scope: taskpkg.ScopeGlobal, Title: "Blocker A"},
				actor,
			)
			if err != nil {
				t.Fatalf("CreateTask(blocker A) error = %v", err)
			}
			blockerB, err := manager.CreateTask(
				ctx,
				taskpkg.CreateTask{Scope: taskpkg.ScopeGlobal, Title: "Blocker B"},
				actor,
			)
			if err != nil {
				t.Fatalf("CreateTask(blocker B) error = %v", err)
			}
			dependent, err := manager.CreateTask(ctx, taskpkg.CreateTask{
				Scope:              taskpkg.ScopeGlobal,
				Title:              "Opted-in dependent",
				AutoEnqueueOnReady: true,
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask(dependent) error = %v", err)
			}
			for _, blockerID := range []string{blockerA.ID, blockerB.ID} {
				if err := manager.AddDependency(ctx, taskpkg.AddDependency{
					TaskID:          dependent.ID,
					DependsOnTaskID: blockerID,
					Kind:            taskpkg.DependencyKindBlocks,
				}, actor); err != nil {
					t.Fatalf("AddDependency(%s) error = %v", blockerID, err)
				}
			}

			// Claim both blocker runs up front so the two completions can race on the same
			// ready dependent. Each blocker yields a distinct trigger run id (and idempotency
			// key), so duplicate-enqueue prevention rests solely on the open-run reservation.
			type leaseClaim struct {
				runID string
				token string
				actor taskpkg.ActorContext
			}
			claims := make([]leaseClaim, 0, 2)
			for i, blockerID := range []string{blockerA.ID, blockerB.ID} {
				if _, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: blockerID}, actor); err != nil {
					t.Fatalf("EnqueueRun(%s) error = %v", blockerID, err)
				}
				session := "sess-worker-" + strconv.Itoa(i)
				worker, err := taskpkg.DeriveAgentSessionActorContext(session, "ws-test")
				if err != nil {
					t.Fatalf("DeriveAgentSessionActorContext(%s) error = %v", session, err)
				}
				claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
					Scope:            taskpkg.ScopeGlobal,
					ClaimerSessionID: session,
					LeaseDuration:    time.Minute,
				}, worker)
				if err != nil {
					t.Fatalf("ClaimNextRun(%s) error = %v", session, err)
				}
				claims = append(claims, leaseClaim{runID: claim.Run.ID, token: claim.ClaimToken, actor: worker})
			}

			var wg sync.WaitGroup
			errCh := make(chan error, len(claims))
			for _, c := range claims {
				wg.Add(1)
				go func(c leaseClaim) {
					defer wg.Done()
					if _, err := manager.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
						RunID:      c.runID,
						ClaimToken: c.token,
						Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
					}, c.actor); err != nil {
						errCh <- err
					}
				}(c)
			}
			wg.Wait()
			close(errCh)
			for err := range errCh {
				t.Fatalf("concurrent CompleteRunLease() error = %v", err)
			}

			dependentRuns, err := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: dependent.ID})
			if err != nil {
				t.Fatalf("ListTaskRuns(dependent) error = %v", err)
			}
			if len(dependentRuns) != 1 {
				t.Fatalf("dependent runs = %d, want exactly 1 auto-enqueued run", len(dependentRuns))
			}
			if got, want := dependentRuns[0].Status, taskpkg.TaskRunStatusQueued; got != want {
				t.Fatalf("auto-enqueued run status = %q, want %q", got, want)
			}
		},
	)
}

func TestTaskManagerCompletedChildrenRollUpParentIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should complete parent exactly once after final child settles", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		manager := newTaskManagerIntegration(t, db)
		actor, err := taskpkg.DeriveHumanActorContext(
			"user-parent-rollup",
			taskpkg.OriginKindCLI,
			"agh task create",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}

		parent, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Parent rollup",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(parent) error = %v", err)
		}
		parentRun, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: parent.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun(parent) error = %v", err)
		}
		if _, err := manager.MarkRunNeedsAttention(
			ctx,
			parentRun.ID,
			"no eligible worker claimed the run",
			actor,
		); err != nil {
			t.Fatalf("MarkRunNeedsAttention(parent) error = %v", err)
		}

		children := make([]*taskpkg.Task, 0, 2)
		for _, title := range []string{"Child A", "Child B"} {
			child, err := manager.CreateChildTask(ctx, parent.ID, taskpkg.CreateTask{
				Scope: taskpkg.ScopeGlobal,
				Title: title,
			}, actor)
			if err != nil {
				t.Fatalf("CreateChildTask(%s) error = %v", title, err)
			}
			children = append(children, child)
		}

		completeChild := func(child *taskpkg.Task, sessionID string) taskpkg.LeaseCompletion {
			t.Helper()
			run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: child.ID}, actor)
			if err != nil {
				t.Fatalf("EnqueueRun(%s) error = %v", child.ID, err)
			}
			worker, err := taskpkg.DeriveAgentSessionActorContext(sessionID, "ws-test")
			if err != nil {
				t.Fatalf("DeriveAgentSessionActorContext(%s) error = %v", sessionID, err)
			}
			claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				RunID:            run.ID,
				Scope:            taskpkg.ScopeGlobal,
				ClaimerSessionID: sessionID,
				LeaseDuration:    time.Minute,
			}, worker)
			if err != nil {
				t.Fatalf("ClaimNextRun(%s) error = %v", child.ID, err)
			}
			completion := taskpkg.LeaseCompletion{
				RunID:      claim.Run.ID,
				ClaimToken: claim.ClaimToken,
				Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
			}
			if _, err := manager.CompleteRunLease(ctx, completion, worker); err != nil {
				t.Fatalf("CompleteRunLease(%s) error = %v", child.ID, err)
			}
			return completion
		}

		completeChild(children[0], "sess-parent-rollup-a")
		storedParent, err := db.GetTask(ctx, parent.ID)
		if err != nil {
			t.Fatalf("GetTask(parent after first child) error = %v", err)
		}
		if got := storedParent.Status.Normalize(); got == taskpkg.TaskStatusCompleted {
			t.Fatalf("parent status after first child = %q, want nonterminal", got)
		}

		finalCompletion := completeChild(children[1], "sess-parent-rollup-b")
		storedParent, err = db.GetTask(ctx, parent.ID)
		if err != nil {
			t.Fatalf("GetTask(parent after final child) error = %v", err)
		}
		if got, want := storedParent.Status.Normalize(), taskpkg.TaskStatusCompleted; got != want {
			t.Fatalf("parent status after final child = %q, want %q", got, want)
		}

		storedParentRuns, err := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: parent.ID})
		if err != nil {
			t.Fatalf("ListTaskRuns(parent) error = %v", err)
		}
		if len(storedParentRuns) != 1 {
			t.Fatalf("parent runs = %d, want exactly 1 settled historical run", len(storedParentRuns))
		}
		if got, want := storedParentRuns[0].Status.Normalize(), taskpkg.TaskRunStatusCompleted; got != want {
			t.Fatalf("parent run status after rollup = %q, want %q", got, want)
		}

		statusChangedCount := 0
		records, err := db.ListTaskEventRecords(ctx, taskpkg.EventRecordQuery{TaskID: parent.ID})
		if err != nil {
			t.Fatalf("ListTaskEventRecords(parent) error = %v", err)
		}
		for _, record := range records {
			if record.Event.EventType == "task.status_changed" {
				statusChangedCount++
			}
		}
		if statusChangedCount != 1 {
			t.Fatalf("parent task.status_changed event count = %d, want 1", statusChangedCount)
		}

		worker, err := taskpkg.DeriveAgentSessionActorContext("sess-parent-rollup-b", "ws-test")
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext(replay) error = %v", err)
		}
		if _, err := manager.CompleteRunLease(ctx, finalCompletion, worker); !errors.Is(
			err,
			taskpkg.ErrInvalidStatusTransition,
		) {
			t.Fatalf("CompleteRunLease(replay) error = %v, want %v", err, taskpkg.ErrInvalidStatusTransition)
		}
		replayedRecords, err := db.ListTaskEventRecords(ctx, taskpkg.EventRecordQuery{TaskID: parent.ID})
		if err != nil {
			t.Fatalf("ListTaskEventRecords(parent after replay) error = %v", err)
		}
		if got, want := len(replayedRecords), len(records); got != want {
			t.Fatalf("parent event count after replay = %d, want unchanged %d", got, want)
		}
	})

	t.Run("Should publish the committed parent completion after request cancellation and reconcile failure", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		requestCtx, cancelRequest := context.WithCancel(ctx)
		reconcileErr := errors.New("forced dependent reconciliation failure")
		store := &rollupPublicationFailureStore{
			Store: db, cancel: cancelRequest, reconcileErr: reconcileErr,
		}
		var hookMu sync.Mutex
		completedByTask := make(map[string]int)
		hooks := integrationTaskRunHooks{
			completed: func(
				hookCtx context.Context,
				payload hookspkg.TaskRunCompletedPayload,
			) (hookspkg.TaskRunCompletedPayload, error) {
				if hookCtx.Err() != nil {
					t.Errorf("TaskRunCompleted hook context error = %v", hookCtx.Err())
				}
				if _, ok := hookCtx.Deadline(); !ok {
					t.Error("TaskRunCompleted hook context has no publication deadline")
				}
				hookMu.Lock()
				completedByTask[payload.TaskID]++
				hookMu.Unlock()
				return payload, nil
			},
		}
		manager := newTaskManagerIntegration(t, store, taskpkg.WithTaskRunHooks(hooks))
		actor, err := taskpkg.DeriveHumanActorContext(
			"user-parent-publication",
			taskpkg.OriginKindCLI,
			"agh task create",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		parent, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal, Title: "Parent publication",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(parent) error = %v", err)
		}
		parentRun, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: parent.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun(parent) error = %v", err)
		}
		if _, err := manager.MarkRunNeedsAttention(ctx, parentRun.ID, "await child", actor); err != nil {
			t.Fatalf("MarkRunNeedsAttention(parent) error = %v", err)
		}
		child, err := manager.CreateChildTask(ctx, parent.ID, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal, Title: "Only child",
		}, actor)
		if err != nil {
			t.Fatalf("CreateChildTask() error = %v", err)
		}
		completion, worker := claimTaskRunForRollupIntegration(
			t,
			ctx,
			manager,
			child.ID,
			actor,
			"sess-parent-publication",
		)

		completedRun, err := manager.CompleteRunLease(requestCtx, completion, worker)
		if completedRun == nil {
			t.Fatal("CompleteRunLease() run = nil after committed settlement")
		}
		if !errors.Is(err, reconcileErr) {
			t.Fatalf("CompleteRunLease() error = %v, want %v", err, reconcileErr)
		}
		if requestCtx.Err() != context.Canceled {
			t.Fatalf("request context error = %v, want %v", requestCtx.Err(), context.Canceled)
		}

		storedParent, err := db.GetTask(ctx, parent.ID)
		if err != nil {
			t.Fatalf("GetTask(parent) error = %v", err)
		}
		if got, want := storedParent.Status.Normalize(), taskpkg.TaskStatusCompleted; got != want {
			t.Fatalf("parent status = %q, want %q", got, want)
		}
		parentRuns, err := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: parent.ID})
		if err != nil {
			t.Fatalf("ListTaskRuns(parent) error = %v", err)
		}
		if len(parentRuns) != 1 || parentRuns[0].Status.Normalize() != taskpkg.TaskRunStatusCompleted {
			t.Fatalf("parent runs = %#v, want one completed run", parentRuns)
		}
		parentEvents, err := db.ListTaskEventRecords(ctx, taskpkg.EventRecordQuery{TaskID: parent.ID})
		if err != nil {
			t.Fatalf("ListTaskEventRecords(parent) error = %v", err)
		}
		completedEventCount := 0
		for _, record := range parentEvents {
			if record.Event.EventType == "task.run.completed" {
				completedEventCount++
			}
		}
		if completedEventCount != 1 {
			t.Fatalf("parent task.run.completed event count = %d, want 1", completedEventCount)
		}
		hookMu.Lock()
		parentHookCount := completedByTask[parent.ID]
		hookMu.Unlock()
		if parentHookCount != 1 {
			t.Fatalf("parent TaskRunCompleted hook count = %d, want 1", parentHookCount)
		}

		if _, replayErr := manager.CompleteRunLease(ctx, completion, worker); !errors.Is(
			replayErr,
			taskpkg.ErrInvalidStatusTransition,
		) {
			t.Fatalf("CompleteRunLease(replay) error = %v, want %v", replayErr, taskpkg.ErrInvalidStatusTransition)
		}
		hookMu.Lock()
		parentHookCount = completedByTask[parent.ID]
		hookMu.Unlock()
		if parentHookCount != 1 {
			t.Fatalf("parent TaskRunCompleted hook count after replay = %d, want 1", parentHookCount)
		}
	})

	t.Run("Should roll up once when final children complete concurrently", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		executor := &integrationSessionExecutor{}
		manager := newTaskManagerIntegration(t, db, taskpkg.WithSessionExecutor(executor))
		actor, err := taskpkg.DeriveHumanActorContext(
			"user-parent-concurrent",
			taskpkg.OriginKindCLI,
			"agh task create",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		parent, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Concurrent parent rollup",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(parent) error = %v", err)
		}
		parentRun, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: parent.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun(parent) error = %v", err)
		}
		if _, err := manager.MarkRunNeedsAttention(ctx, parentRun.ID, "starved", actor); err != nil {
			t.Fatalf("MarkRunNeedsAttention(parent) error = %v", err)
		}

		type claimedCompletion struct {
			completion taskpkg.LeaseCompletion
			worker     taskpkg.ActorContext
		}
		claims := make([]claimedCompletion, 0, 2)
		for idx, title := range []string{"Concurrent child A", "Concurrent child B"} {
			child, err := manager.CreateChildTask(ctx, parent.ID, taskpkg.CreateTask{
				Scope: taskpkg.ScopeGlobal,
				Title: title,
			}, actor)
			if err != nil {
				t.Fatalf("CreateChildTask(%s) error = %v", title, err)
			}
			completion, worker := claimTaskRunForRollupIntegration(
				t,
				ctx,
				manager,
				child.ID,
				actor,
				"sess-parent-concurrent-"+strconv.Itoa(idx),
			)
			claims = append(claims, claimedCompletion{completion: completion, worker: worker})
		}

		var wg sync.WaitGroup
		errCh := make(chan error, len(claims))
		for _, claim := range claims {
			wg.Add(1)
			go func(claim claimedCompletion) {
				defer wg.Done()
				if _, err := manager.CompleteRunLease(ctx, claim.completion, claim.worker); err != nil {
					errCh <- err
				}
			}(claim)
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			t.Fatalf("concurrent CompleteRunLease() error = %v", err)
		}

		storedParent, err := db.GetTask(ctx, parent.ID)
		if err != nil {
			t.Fatalf("GetTask(parent) error = %v", err)
		}
		if got, want := storedParent.Status.Normalize(), taskpkg.TaskStatusCompleted; got != want {
			t.Fatalf("parent status = %q, want %q", got, want)
		}
		records, err := db.ListTaskEventRecords(ctx, taskpkg.EventRecordQuery{TaskID: parent.ID})
		if err != nil {
			t.Fatalf("ListTaskEventRecords(parent) error = %v", err)
		}
		statusChangedCount := 0
		for _, record := range records {
			if record.Event.EventType == "task.status_changed" {
				statusChangedCount++
			}
		}
		if statusChangedCount != 1 {
			t.Fatalf("parent task.status_changed event count = %d, want 1", statusChangedCount)
		}
		if len(executor.startCalls) != 0 {
			t.Fatalf("parent rollup started %d sessions, want 0", len(executor.startCalls))
		}
	})

	t.Run("Should apply the same rollup through unfenced completion", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		manager := newTaskManagerIntegration(t, db, taskpkg.WithSessionExecutor(&integrationSessionExecutor{}))
		actor, err := taskpkg.DeriveHumanActorContext(
			"user-parent-unfenced",
			taskpkg.OriginKindCLI,
			"agh task complete",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		parent, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Unfenced parent rollup",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(parent) error = %v", err)
		}

		for idx, title := range []string{"Unfenced child A", "Unfenced child B"} {
			child, err := manager.CreateChildTask(ctx, parent.ID, taskpkg.CreateTask{
				Scope: taskpkg.ScopeGlobal,
				Title: title,
			}, actor)
			if err != nil {
				t.Fatalf("CreateChildTask(%s) error = %v", title, err)
			}
			run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: child.ID}, actor)
			if err != nil {
				t.Fatalf("EnqueueRun(%s) error = %v", title, err)
			}
			run, err = seedNonLeasedClaimedRunIntegration(ctx, manager, db, run.ID, actor)
			if err != nil {
				t.Fatalf("seedNonLeasedClaimedRunIntegration(%s) error = %v", title, err)
			}
			run, err = manager.StartRun(ctx, run.ID, taskpkg.StartRun{}, actor)
			if err != nil {
				t.Fatalf("StartRun(%s) error = %v", title, err)
			}
			if _, err := manager.CompleteRun(ctx, run.ID, taskpkg.RunResult{
				Value: json.RawMessage(`{"child_index":` + strconv.Itoa(idx) + `}`),
			}, actor); err != nil {
				t.Fatalf("CompleteRun(%s) error = %v", title, err)
			}
		}

		storedParent, err := db.GetTask(ctx, parent.ID)
		if err != nil {
			t.Fatalf("GetTask(parent) error = %v", err)
		}
		if got, want := storedParent.Status.Normalize(), taskpkg.TaskStatusCompleted; got != want {
			t.Fatalf("parent status = %q, want %q", got, want)
		}
	})

	for _, terminalStatus := range []taskpkg.RunStatus{
		taskpkg.TaskRunStatusFailed,
		taskpkg.TaskRunStatusCanceled,
	} {
		terminalStatus := terminalStatus
		t.Run("Should keep parent nonterminal when child is "+terminalStatus.String(), func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			db := openTaskManagerGlobalDB(t)
			manager := newTaskManagerIntegration(t, db)
			actor, err := taskpkg.DeriveHumanActorContext(
				"user-parent-negative-"+terminalStatus.String(),
				taskpkg.OriginKindCLI,
				"agh task create",
			)
			if err != nil {
				t.Fatalf("DeriveHumanActorContext() error = %v", err)
			}
			parent, err := manager.CreateTask(ctx, taskpkg.CreateTask{
				Scope: taskpkg.ScopeGlobal,
				Title: "Negative parent rollup",
			}, actor)
			if err != nil {
				t.Fatalf("CreateTask(parent) error = %v", err)
			}
			completedChild, err := manager.CreateChildTask(ctx, parent.ID, taskpkg.CreateTask{
				Scope: taskpkg.ScopeGlobal,
				Title: "Completed child",
			}, actor)
			if err != nil {
				t.Fatalf("CreateChildTask(completed) error = %v", err)
			}
			terminalChild, err := manager.CreateChildTask(ctx, parent.ID, taskpkg.CreateTask{
				Scope:       taskpkg.ScopeGlobal,
				Title:       "Non-success child",
				MaxAttempts: new(1),
			}, actor)
			if err != nil {
				t.Fatalf("CreateChildTask(non-success) error = %v", err)
			}

			completion, worker := claimTaskRunForRollupIntegration(
				t,
				ctx,
				manager,
				completedChild.ID,
				actor,
				"sess-parent-negative-complete-"+terminalStatus.String(),
			)
			if _, err := manager.CompleteRunLease(ctx, completion, worker); err != nil {
				t.Fatalf("CompleteRunLease(completed child) error = %v", err)
			}
			terminalCompletion, terminalWorker := claimTaskRunForRollupIntegration(
				t,
				ctx,
				manager,
				terminalChild.ID,
				actor,
				"sess-parent-negative-terminal-"+terminalStatus.String(),
			)
			switch terminalStatus {
			case taskpkg.TaskRunStatusFailed:
				if _, err := manager.FailRunLease(ctx, taskpkg.LeaseFailure{
					RunID:      terminalCompletion.RunID,
					ClaimToken: terminalCompletion.ClaimToken,
					Failure:    taskpkg.RunFailure{Error: "worker failed"},
				}, terminalWorker); err != nil {
					t.Fatalf("FailRunLease() error = %v", err)
				}
			case taskpkg.TaskRunStatusCanceled:
				if _, err := manager.CancelRun(ctx, terminalCompletion.RunID, taskpkg.CancelRun{
					Reason: "operator canceled child",
				}, actor); err != nil {
					t.Fatalf("CancelRun() error = %v", err)
				}
			default:
				t.Fatalf("unexpected terminal status %q", terminalStatus)
			}

			storedParent, err := db.GetTask(ctx, parent.ID)
			if err != nil {
				t.Fatalf("GetTask(parent) error = %v", err)
			}
			if got := storedParent.Status.Normalize(); got == taskpkg.TaskStatusCompleted {
				t.Fatalf("parent status = %q, want nonterminal", got)
			}
			records, err := db.ListTaskEventRecords(ctx, taskpkg.EventRecordQuery{TaskID: parent.ID})
			if err != nil {
				t.Fatalf("ListTaskEventRecords(parent) error = %v", err)
			}
			for _, record := range records {
				if record.Event.EventType == "task.status_changed" {
					t.Fatalf("parent emitted unexpected task.status_changed event: %#v", record.Event)
				}
			}
		})
	}
}

func claimTaskRunForRollupIntegration(
	t *testing.T,
	ctx context.Context,
	manager *taskpkg.Service,
	taskID string,
	actor taskpkg.ActorContext,
	sessionID string,
) (taskpkg.LeaseCompletion, taskpkg.ActorContext) {
	t.Helper()
	run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun(%s) error = %v", taskID, err)
	}
	worker, err := taskpkg.DeriveAgentSessionActorContext(sessionID, "ws-test")
	if err != nil {
		t.Fatalf("DeriveAgentSessionActorContext(%s) error = %v", sessionID, err)
	}
	claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		RunID:            run.ID,
		Scope:            taskpkg.ScopeGlobal,
		ClaimerSessionID: sessionID,
		LeaseDuration:    time.Minute,
	}, worker)
	if err != nil {
		t.Fatalf("ClaimNextRun(%s) error = %v", taskID, err)
	}
	return taskpkg.LeaseCompletion{
		RunID:      claim.Run.ID,
		ClaimToken: claim.ClaimToken,
		Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
	}, worker
}

func TestTaskManagerAgentCreatedTaskApprovesThenClaimsIntegration(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	workspaceID := registerTaskManagerWorkspace(t, db, "approval-boundary", filepath.Join(t.TempDir(), "workspace"))
	manager := newTaskManagerIntegration(t, db)

	agentActor, err := taskpkg.DeriveAgentSessionActorContext("sess-author", workspaceID)
	if err != nil {
		t.Fatalf("DeriveAgentSessionActorContext(author) error = %v", err)
	}
	taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspaceID,
		Title:          "Agent approval boundary",
		ApprovalPolicy: taskpkg.ApprovalPolicyManual,
	}, agentActor)
	if err != nil {
		t.Fatalf("CreateTask(agent) error = %v", err)
	}
	if got, want := taskRecord.CreatedBy.Kind, taskpkg.ActorKindAgentSession; got != want {
		t.Fatalf("task.CreatedBy.Kind = %q, want %q", got, want)
	}
	runsBefore, err := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskRuns(before approval) error = %v", err)
	}
	if len(runsBefore) != 0 {
		t.Fatalf("runs before approval = %d, want 0", len(runsBefore))
	}

	operator, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "agh task approve")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	execution, err := manager.ApproveTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{}, operator)
	if err != nil {
		t.Fatalf("ApproveTask() error = %v", err)
	}
	if got, want := execution.Task.ApprovalState, taskpkg.ApprovalStateApproved; got != want {
		t.Fatalf("execution.Task.ApprovalState = %q, want %q", got, want)
	}
	if got, want := execution.Run.NetworkSpecSnapshot().Source, participation.SourceBuiltInLocal; got != want {
		t.Fatalf("ApproveTask().Run.NetworkSpecSnapshot().Source = %q, want %q", got, want)
	}

	worker, err := taskpkg.DeriveAgentSessionActorContext("sess-worker", workspaceID)
	if err != nil {
		t.Fatalf("DeriveAgentSessionActorContext(worker) error = %v", err)
	}
	claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      workspaceID,
		ClaimerSessionID: "sess-worker",
		LeaseDuration:    time.Minute,
	}, worker)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	if got, want := claim.Run.ID, execution.Run.ID; got != want {
		t.Fatalf("ClaimNextRun().Run.ID = %q, want %q", got, want)
	}
}

func TestTaskManagerChildAndDependencyFlowsPersistAudit(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	manager := newTaskManagerIntegration(t, db)
	workspaceID := registerTaskManagerWorkspace(
		t,
		db,
		"task-manager-integration",
		filepath.Join(t.TempDir(), "workspace"),
	)

	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task create")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	parent, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Coordinator",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(parent) error = %v", err)
	}
	child, err := manager.CreateChildTask(ctx, parent.ID, taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: workspaceID,
		Title:       "Workspace child",
		Owner:       &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "triage"},
	}, actor)
	if err != nil {
		t.Fatalf("CreateChildTask() error = %v", err)
	}
	blocker, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: workspaceID,
		Title:       "Blocking task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(blocker) error = %v", err)
	}

	if err := manager.AddDependency(ctx, taskpkg.AddDependency{
		TaskID:          child.ID,
		DependsOnTaskID: blocker.ID,
		Kind:            taskpkg.DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	storedChild, err := db.GetTask(ctx, child.ID)
	if err != nil {
		t.Fatalf("GetTask(child) error = %v", err)
	}
	if got, want := storedChild.ParentTaskID, parent.ID; got != want {
		t.Fatalf("storedChild.ParentTaskID = %q, want %q", got, want)
	}
	if got, want := storedChild.Status, taskpkg.TaskStatusBlocked; got != want {
		t.Fatalf("storedChild.Status = %q, want %q", got, want)
	}

	dependencies, err := db.ListDependencies(ctx, child.ID)
	if err != nil {
		t.Fatalf("ListDependencies(child) error = %v", err)
	}
	if len(dependencies) != 1 {
		t.Fatalf("len(dependencies) = %d, want 1", len(dependencies))
	}
	if got, want := dependencies[0].DependsOnTaskID, blocker.ID; got != want {
		t.Fatalf("dependencies[0].DependsOnTaskID = %q, want %q", got, want)
	}

	childEvents, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: child.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents(child) error = %v", err)
	}
	if !testutil.EqualStringSlices(
		sortedEventTypes(childEvents),
		[]string{"task.created", "task.dependency_added", "task.status_changed"},
	) {
		t.Fatalf(
			"child event types = %#v, want task.created + task.dependency_added + task.status_changed",
			sortedEventTypes(childEvents),
		)
	}

	parentEvents, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: parent.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents(parent) error = %v", err)
	}
	if !containsEventType(parentEvents, "task.child_created") {
		t.Fatalf("parent events = %#v, want task.child_created", sortedEventTypes(parentEvents))
	}

	view, err := manager.GetTask(ctx, child.ID, actor)
	if err != nil {
		t.Fatalf("GetTask(view) error = %v", err)
	}
	if got, want := len(view.Dependencies), 1; got != want {
		t.Fatalf("len(view.Dependencies) = %d, want %d", got, want)
	}
	if got, want := view.Task.Status, taskpkg.TaskStatusBlocked; got != want {
		t.Fatalf("view.Task.Status = %q, want %q", got, want)
	}
	if got, want := view.Summary.DependencyCount, int32(1); got != want {
		t.Fatalf("view.Summary.DependencyCount = %d, want %d", got, want)
	}
	if got, want := view.Summary.ChildCount, int32(0); got != want {
		t.Fatalf("view.Summary.ChildCount = %d, want %d", got, want)
	}
	if len(view.DependencyReferences) != 1 {
		t.Fatalf("len(view.DependencyReferences) = %d, want 1", len(view.DependencyReferences))
	}
	if got, want := view.DependencyReferences[0].DependsOn.Title, blocker.Title; got != want {
		t.Fatalf("view.DependencyReferences[0].DependsOn.Title = %q, want %q", got, want)
	}
	if view.Summary.LastActivityAt.IsZero() {
		t.Fatal("view.Summary.LastActivityAt is zero, want latest activity timestamp")
	}
}

func TestTaskManagerListTasksReturnsEnrichedSummariesIntegration(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	manager := newTaskManagerIntegration(t, db)
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task list")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	first, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:      taskpkg.ScopeGlobal,
		Title:      "Alpha planning",
		Identifier: "OPS-100",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(first) error = %v", err)
	}
	second, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:      taskpkg.ScopeGlobal,
		Title:      "Beta rollout",
		Identifier: "OPS-200",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(second) error = %v", err)
	}
	if err := manager.AddDependency(ctx, taskpkg.AddDependency{
		TaskID:          second.ID,
		DependsOnTaskID: first.ID,
		Kind:            taskpkg.DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}
	if err := db.CreateTaskRun(ctx, taskpkg.Run{
		ID:        "run-beta",
		TaskID:    second.ID,
		Status:    taskpkg.TaskRunStatusRunning,
		Attempt:   1,
		Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler"},
		QueuedAt:  time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		StartedAt: time.Date(2026, 4, 17, 12, 5, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}

	byTitle, err := manager.ListTasks(ctx, taskpkg.Query{Search: "alpha"}, actor)
	if err != nil {
		t.Fatalf("ListTasks(search title) error = %v", err)
	}
	if len(byTitle) != 1 || byTitle[0].ID != first.ID {
		t.Fatalf("ListTasks(search title) = %#v, want only %q", byTitle, first.ID)
	}

	byIdentifier, err := manager.ListTasks(ctx, taskpkg.Query{Search: "ops-200"}, actor)
	if err != nil {
		t.Fatalf("ListTasks(search identifier) error = %v", err)
	}
	if len(byIdentifier) != 1 || byIdentifier[0].ID != second.ID {
		t.Fatalf("ListTasks(search identifier) = %#v, want only %q", byIdentifier, second.ID)
	}
	if got, want := byIdentifier[0].DependencyCount, int32(1); got != want {
		t.Fatalf("byIdentifier[0].DependencyCount = %d, want %d", got, want)
	}
	if byIdentifier[0].ActiveRun == nil || byIdentifier[0].ActiveRun.ID != "run-beta" {
		t.Fatalf("byIdentifier[0].ActiveRun = %#v, want run-beta", byIdentifier[0].ActiveRun)
	}
	if len(byIdentifier[0].Dependencies) != 1 {
		t.Fatalf("len(byIdentifier[0].Dependencies) = %d, want 1", len(byIdentifier[0].Dependencies))
	}
	if got, want := byIdentifier[0].Dependencies[0].DependsOn.Identifier, first.Identifier; got != want {
		t.Fatalf("byIdentifier[0].Dependencies[0].DependsOn.Identifier = %q, want %q", got, want)
	}

	all, err := manager.ListTasks(ctx, taskpkg.Query{}, actor)
	if err != nil {
		t.Fatalf("ListTasks(all) error = %v", err)
	}
	if got, want := []string{
		all[0].ID,
		all[1].ID,
	}, []string{
		second.ID,
		first.ID,
	}; !testutil.EqualStringSlices(
		got,
		want,
	) {
		t.Fatalf("ListTasks(all) order = %#v, want %#v", got, want)
	}
}

func TestTaskManagerCatalogMatchesCanonicalDependencyStatusIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should release a dependent when its stale dependency has a completed latest run", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		manager := newTaskManagerIntegration(t, db)
		actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task list")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}

		dependency, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Canonical dependency",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(dependency) error = %v", err)
		}
		dependent, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Canonical dependent parity target",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(dependent) error = %v", err)
		}
		if err := manager.AddDependency(ctx, taskpkg.AddDependency{
			TaskID:          dependent.ID,
			DependsOnTaskID: dependency.ID,
			Kind:            taskpkg.DependencyKindBlocks,
		}, actor); err != nil {
			t.Fatalf("AddDependency() error = %v", err)
		}
		now := time.Date(2026, 7, 10, 18, 0, 0, 0, time.UTC)
		if err := db.CreateTaskRun(ctx, taskpkg.Run{
			ID:        "run-canonical-dependency-completed",
			TaskID:    dependency.ID,
			Status:    taskpkg.TaskRunStatusCompleted,
			Attempt:   1,
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler"},
			QueuedAt:  now,
			StartedAt: now.Add(time.Minute),
			EndedAt:   now.Add(2 * time.Minute),
		}); err != nil {
			t.Fatalf("CreateTaskRun(completed) error = %v", err)
		}

		enriched, err := manager.ListTasks(ctx, taskpkg.Query{Search: dependent.Title}, actor)
		if err != nil {
			t.Fatalf("ListTasks() error = %v", err)
		}
		catalog, err := manager.ListTaskCatalog(ctx, taskpkg.CatalogQuery{
			Scope:         taskpkg.CatalogScopeGlobal,
			Search:        dependent.Title,
			IncludeDrafts: true,
		}, actor)
		if err != nil {
			t.Fatalf("ListTaskCatalog() error = %v", err)
		}
		if len(enriched) != 1 || len(catalog.Tasks) != 1 {
			t.Fatalf("summary counts enriched=%d catalog=%d, want 1/1", len(enriched), len(catalog.Tasks))
		}
		if got, want := catalog.Tasks[0].Status, enriched[0].Status; got != want {
			t.Fatalf("catalog status = %q, enriched service status = %q", got, want)
		}
		if got, want := catalog.Tasks[0].Status, taskpkg.TaskStatusReady; got != want {
			t.Fatalf("catalog status = %q, want %q", got, want)
		}
	})
}

func TestTaskManagerRunLifecyclePersistsAndReconcilesAgainstStorage(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	executor := &integrationSessionExecutor{}
	manager := newTaskManagerIntegration(t, db, taskpkg.WithSessionExecutor(executor))
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task run")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Lifecycle integration",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	storedRun, err := db.GetTaskRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(queued) error = %v", err)
	}
	if got, want := storedRun.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("queued run status = %q, want %q", got, want)
	}

	claim, err := claimExactRunIntegration(ctx, manager, db, run.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	run = &claim.Run
	if got, want := run.Status, taskpkg.TaskRunStatusClaimed; got != want {
		t.Fatalf("claimed run status = %q, want %q", got, want)
	}
	storedTask, err := db.GetTask(ctx, taskRecord.ID)
	if err != nil {
		t.Fatalf("GetTask(claimed) error = %v", err)
	}
	if got, want := storedTask.Status, taskpkg.TaskStatusReady; got != want {
		t.Fatalf("task status after claim = %q, want %q", got, want)
	}

	run, err = manager.StartRun(ctx, run.ID, taskpkg.StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	if got, want := run.Status, taskpkg.TaskRunStatusRunning; got != want {
		t.Fatalf("running run status = %q, want %q", got, want)
	}
	if got := run.SessionID; got == "" {
		t.Fatal("run.SessionID = empty, want dedicated session id")
	}
	storedTask, err = db.GetTask(ctx, taskRecord.ID)
	if err != nil {
		t.Fatalf("GetTask(running) error = %v", err)
	}
	if got, want := storedTask.Status, taskpkg.TaskStatusInProgress; got != want {
		t.Fatalf("task status after start = %q, want %q", got, want)
	}

	run, err = manager.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
		RunID:      run.ID,
		ClaimToken: claim.ClaimToken,
		Result: taskpkg.RunResult{
			Value: json.RawMessage(`{"result":"ok"}`),
		},
		Now: claim.LeaseUntil.Add(-time.Second),
	}, actor)
	if err != nil {
		t.Fatalf("CompleteRunLease() error = %v", err)
	}
	if got, want := run.Status, taskpkg.TaskRunStatusCompleted; got != want {
		t.Fatalf("completed run status = %q, want %q", got, want)
	}
	storedTask, err = db.GetTask(ctx, taskRecord.ID)
	if err != nil {
		t.Fatalf("GetTask(completed) error = %v", err)
	}
	if got, want := storedTask.Status, taskpkg.TaskStatusCompleted; got != want {
		t.Fatalf("task status after complete = %q, want %q", got, want)
	}

	events, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents() error = %v", err)
	}
	wantTypes := []string{
		"task.created",
		"task.run.completed",
		"task.run_claimed",
		"task.run_enqueued",
		"task.run_session_bound",
		"task.run_started",
		"task.run_starting",
		"task.status_changed",
		"task.status_changed",
	}
	if !testutil.EqualStringSlices(sortedEventTypes(events), wantTypes) {
		t.Fatalf("event types = %#v, want %#v", sortedEventTypes(events), wantTypes)
	}
}

func TestTaskManagerRecoverRunOnBootRequeuesBoundRunWithGlobalDB(t *testing.T) {
	t.Parallel()

	t.Run("Should requeue bound run on boot and release session binding", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		manager := newTaskManagerIntegration(t, db)
		operator, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "agh task run")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		agent, err := taskpkg.DeriveAgentSessionActorContext("sess-stale-boot", "ws-test")
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
		}
		daemon, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "daemon.boot")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}

		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeGlobal,
			Title:       "Boot recovery integration",
			MaxAttempts: intPtr(2),
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, operator)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-stale-boot",
			LeaseDuration:    time.Hour,
			Now:              time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC),
		}, agent)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		if claim.Run.ID != run.ID || claim.Run.SessionID != "sess-stale-boot" {
			t.Fatalf("claim.Run = %#v, want run %q bound to sess-stale-boot", claim.Run, run.ID)
		}
		originalSpec := run.NetworkSpecSnapshot()
		if got, want := originalSpec.Source, participation.SourceBuiltInLocal; got != want {
			t.Fatalf("original source = %q, want %q", got, want)
		}

		recovered, err := manager.RecoverRunOnBoot(ctx, run.ID, taskpkg.RunBootRecovery{
			Action:       taskpkg.RunBootRecoveryRequeue,
			Reason:       "orphaned_on_boot",
			SessionState: "stopped",
		}, daemon)
		if err != nil {
			t.Fatalf("RecoverRunOnBoot(requeue) error = %v", err)
		}
		if recovered.Status != taskpkg.TaskRunStatusQueued || recovered.SessionID != "" || recovered.ClaimedBy != nil {
			t.Fatalf("recovered = %#v, want queued run with released session binding", recovered)
		}
		if got, want := recovered.NetworkSpecSnapshot(), originalSpec; got != want {
			t.Fatalf("recovered snapshot = %#v, want %#v", got, want)
		}

		stored, err := db.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(recovered) error = %v", err)
		}
		if stored.Status != taskpkg.TaskRunStatusQueued || stored.SessionID != "" || stored.ClaimedBy != nil {
			t.Fatalf("stored = %#v, want queued run with released session binding", stored)
		}
		if got, want := stored.NetworkSpecSnapshot(), originalSpec; got != want {
			t.Fatalf("stored snapshot = %#v, want %#v", got, want)
		}

		reclaimed, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-stale-boot",
			LeaseDuration:    time.Hour,
			Now:              time.Now().UTC(),
		}, agent)
		if err != nil {
			t.Fatalf("ClaimNextRun(recovered) error = %v", err)
		}
		failed, err := manager.FailRunLease(ctx, taskpkg.LeaseFailure{
			RunID:      reclaimed.Run.ID,
			ClaimToken: reclaimed.ClaimToken,
			Failure:    taskpkg.RunFailure{Error: "recovery probe failed"},
		}, agent)
		if err != nil {
			t.Fatalf("FailRunLease() error = %v", err)
		}
		if got, want := failed.NetworkSpecSnapshot(), originalSpec; got != want {
			t.Fatalf("failed snapshot = %#v, want %#v", got, want)
		}
		retry, err := manager.RetryRun(ctx, failed.ID, taskpkg.RetryRunRequest{}, operator)
		if err != nil {
			t.Fatalf("RetryRun() error = %v", err)
		}
		if got, want := retry.Run.NetworkSpecSnapshot(), originalSpec; got != want {
			t.Fatalf("retry snapshot = %#v, want %#v", got, want)
		}
	})
}

func TestTaskManagerCancelTaskTreePersistsCancellationAudit(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	executor := &integrationSessionExecutor{}
	manager := newTaskManagerIntegration(
		t,
		db,
		taskpkg.WithSessionExecutor(executor),
		taskpkg.WithCancelGracePeriod(0),
	)
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task cancel")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	parent, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Cancellation parent",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(parent) error = %v", err)
	}
	queuedChild, err := manager.CreateChildTask(ctx, parent.ID, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Queued child",
	}, actor)
	if err != nil {
		t.Fatalf("CreateChildTask(queued child) error = %v", err)
	}
	activeChild, err := manager.CreateChildTask(ctx, parent.ID, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Active child",
	}, actor)
	if err != nil {
		t.Fatalf("CreateChildTask(active child) error = %v", err)
	}

	queuedRun, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: queuedChild.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun(queued child) error = %v", err)
	}
	activeRun, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: activeChild.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun(active child) error = %v", err)
	}
	activeRun, err = seedNonLeasedClaimedRunIntegration(ctx, manager, db, activeRun.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun(active child) error = %v", err)
	}
	activeRun, err = manager.StartRun(ctx, activeRun.ID, taskpkg.StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun(active child) error = %v", err)
	}

	cancelledParent, err := manager.CancelTask(ctx, parent.ID, taskpkg.CancelTask{
		Reason: "stop tree",
	}, actor)
	if err != nil {
		t.Fatalf("CancelTask() error = %v", err)
	}
	if got, want := cancelledParent.Status, taskpkg.TaskStatusCanceled; got != want {
		t.Fatalf("cancelled parent status = %q, want %q", got, want)
	}

	for _, taskID := range []string{parent.ID, queuedChild.ID, activeChild.ID} {
		record, err := db.GetTask(ctx, taskID)
		if err != nil {
			t.Fatalf("GetTask(%q) error = %v", taskID, err)
		}
		if got, want := record.Status, taskpkg.TaskStatusCanceled; got != want {
			t.Fatalf("task %q status = %q, want %q", taskID, got, want)
		}
	}

	storedQueuedRun, err := db.GetTaskRun(ctx, queuedRun.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(queued) error = %v", err)
	}
	if got, want := storedQueuedRun.Status, taskpkg.TaskRunStatusCanceled; got != want {
		t.Fatalf("queued child run status = %q, want %q", got, want)
	}
	storedActiveRun, err := db.GetTaskRun(ctx, activeRun.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(active) error = %v", err)
	}
	if got, want := storedActiveRun.Status, taskpkg.TaskRunStatusCanceled; got != want {
		t.Fatalf("active child run status = %q, want %q", got, want)
	}

	if len(executor.requestStopCalls) != 1 {
		t.Fatalf("len(requestStopCalls) = %d, want 1", len(executor.requestStopCalls))
	}
	if got, want := executor.requestStopCalls[0].SessionID, activeRun.SessionID; got != want {
		t.Fatalf("requestStopCalls[0].SessionID = %q, want %q", got, want)
	}
	if len(executor.forceStopCalls) != 1 {
		t.Fatalf("len(forceStopCalls) = %d, want 1", len(executor.forceStopCalls))
	}

	parentEvents, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: parent.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents(parent) error = %v", err)
	}
	if !containsEventType(parentEvents, "task.canceled") {
		t.Fatalf("parent event types = %#v, want task.canceled", sortedEventTypes(parentEvents))
	}

	activeChildEvents, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: activeChild.ID})
	if err != nil {
		t.Fatalf("ListTaskEvents(active child) error = %v", err)
	}
	if !containsEventType(activeChildEvents, "task.run_canceled") {
		t.Fatalf("active child event types = %#v, want task.run_canceled", sortedEventTypes(activeChildEvents))
	}
	if !containsEventType(activeChildEvents, "task.run_force_stopped") {
		t.Fatalf("active child event types = %#v, want task.run_force_stopped", sortedEventTypes(activeChildEvents))
	}
}

func TestTaskManagerTimelineLiveReadsIntegration(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	executor := &integrationSessionExecutor{}
	fixedNow := time.Date(2026, 4, 17, 14, 0, 0, 0, time.UTC)
	counter := 0
	manager := newTaskManagerIntegration(
		t,
		db,
		taskpkg.WithSessionExecutor(executor),
		taskpkg.WithManagerNow(func() time.Time { return fixedNow }),
		taskpkg.WithIDGenerator(func(prefix string) string {
			counter++
			return prefix + "-timeline-" + strconv.Itoa(counter)
		}),
	)
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task timeline")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Timeline detail task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	claim, err := claimExactRunIntegration(ctx, manager, db, run.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	run = &claim.Run
	run, err = manager.StartRun(ctx, run.ID, taskpkg.StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	if _, err := manager.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
		RunID:      run.ID,
		ClaimToken: claim.ClaimToken,
		Result: taskpkg.RunResult{
			Value: json.RawMessage(`{"ok":true}`),
		},
		Now: claim.LeaseUntil.Add(-time.Second),
	}, actor); err != nil {
		t.Fatalf("CompleteRunLease() error = %v", err)
	}

	pageOne, err := manager.Timeline(ctx, taskRecord.ID, taskpkg.TimelineQuery{Limit: 3}, actor)
	if err != nil {
		t.Fatalf("Timeline(page one) error = %v", err)
	}
	if got, want := len(pageOne), 3; got != want {
		t.Fatalf("len(pageOne) = %d, want %d", got, want)
	}
	if got, want := []string{
		pageOne[0].EventType,
		pageOne[1].EventType,
		pageOne[2].EventType,
	}, []string{"task.created", "task.run_enqueued", "task.run_claimed"}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("pageOne event types = %#v, want %#v", got, want)
	}
	for idx, item := range pageOne {
		if got, want := item.Sequence, int64(idx+1); got != want {
			t.Fatalf("pageOne[%d].Sequence = %d, want %d", idx, got, want)
		}
		if idx == 0 {
			if item.Run != nil {
				t.Fatalf("pageOne[0].Run = %#v, want nil", item.Run)
			}
			continue
		}
		if item.Run == nil || item.Run.ID != run.ID {
			t.Fatalf("pageOne[%d].Run = %#v, want run %q", idx, item.Run, run.ID)
		}
	}

	pageTwo, err := manager.Timeline(ctx, taskRecord.ID, taskpkg.TimelineQuery{
		AfterSequence: pageOne[len(pageOne)-1].Sequence,
		Limit:         6,
	}, actor)
	if err != nil {
		t.Fatalf("Timeline(page two) error = %v", err)
	}
	if got, want := len(pageTwo), 6; got != want {
		t.Fatalf("len(pageTwo) = %d, want %d", got, want)
	}
	if got, want := []string{
		pageTwo[0].EventType,
		pageTwo[1].EventType,
		pageTwo[2].EventType,
		pageTwo[3].EventType,
		pageTwo[4].EventType,
		pageTwo[5].EventType,
	}, []string{
		"task.status_changed",
		"task.run_starting",
		"task.run_session_bound",
		"task.run_started",
		"task.run.completed",
		"task.status_changed",
	}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("pageTwo event types = %#v, want %#v", got, want)
	}
	for idx, item := range pageTwo {
		if got, want := item.Sequence, int64(idx+4); got != want {
			t.Fatalf("pageTwo[%d].Sequence = %d, want %d", idx, got, want)
		}
		if idx == 0 || idx == 5 {
			if item.Run != nil {
				t.Fatalf("pageTwo[%d].Run = %#v, want nil for task status event", idx, item.Run)
			}
			continue
		}
		if item.Run == nil || item.Run.ID != run.ID {
			t.Fatalf("pageTwo[%d].Run = %#v, want run %q", idx, item.Run, run.ID)
		}
	}
}

func TestTaskManagerRunDetailUsesPersistedRuntimeDataIntegration(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	workspaceID := registerTaskManagerWorkspace(t, db, "runtime-detail", filepath.Join(t.TempDir(), "workspace"))
	executor := &integrationSessionExecutor{}
	runtimeReader := &integrationRuntimeViewReader{registry: db, sessionStore: make(map[string]*sessiondb.SessionDB)}
	fixedNow := time.Date(2026, 4, 17, 15, 0, 0, 0, time.UTC)
	counter := 0
	manager := newTaskManagerIntegration(
		t,
		db,
		taskpkg.WithSessionExecutor(executor),
		taskpkg.WithRuntimeViewReader(runtimeReader),
		taskpkg.WithManagerNow(func() time.Time { return fixedNow }),
		taskpkg.WithIDGenerator(func(prefix string) string {
			counter++
			return prefix + "-detail-" + strconv.Itoa(counter)
		}),
	)
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task run-detail")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: workspaceID,
		Title:       "Run detail task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	run, err = seedNonLeasedClaimedRunIntegration(ctx, manager, db, run.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	run, err = manager.StartRun(ctx, run.ID, taskpkg.StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}

	sessionInfo := store.SessionInfo{
		ID:          run.SessionID,
		Name:        "Task detail session",
		AgentName:   "codex",
		WorkspaceID: workspaceID,
		SessionType: "task",
		State:       "running",
		CreatedAt:   fixedNow,
		UpdatedAt:   fixedNow.Add(5 * time.Minute),
	}
	sessionInfo.SetNetworkSpec(run.NetworkSpecSnapshot())
	if err := db.RegisterSession(ctx, sessionInfo); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}

	sessionDir := filepath.Join(t.TempDir(), "sessions", run.SessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", sessionDir, err)
	}
	sessionDB, err := sessiondb.OpenSessionDB(ctx, run.SessionID, filepath.Join(sessionDir, store.SessionDatabaseName))
	if err != nil {
		t.Fatalf("OpenSessionDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := sessionDB.Close(ctx); err != nil {
			t.Fatalf("SessionDB.Close() error = %v", err)
		}
	})
	runtimeReader.sessionStore[run.SessionID] = sessionDB

	for _, event := range []store.SessionEvent{
		{
			ID:        "event-1",
			TurnID:    "turn-1",
			Type:      "agent_message",
			AgentName: "codex",
			Content:   `{"text":"planning"}`,
			Timestamp: fixedNow.Add(time.Minute),
		},
		{
			ID:        "event-2",
			TurnID:    "turn-1",
			Type:      "tool_call",
			AgentName: "codex",
			Content:   `{"tool_call_id":"call-1"}`,
			Timestamp: fixedNow.Add(2 * time.Minute),
		},
		{
			ID:        "event-3",
			TurnID:    "turn-1",
			Type:      "tool_result",
			AgentName: "codex",
			Content:   `{"tool_call_id":"call-1"}`,
			Timestamp: fixedNow.Add(3 * time.Minute),
		},
		{
			ID:        "event-4",
			TurnID:    "turn-2",
			Type:      "tool_call",
			AgentName: "codex",
			Content:   `{"toolCallId":"call-2"}`,
			Timestamp: fixedNow.Add(4 * time.Minute),
		},
	} {
		if err := sessionDB.Record(ctx, event); err != nil {
			t.Fatalf("SessionDB.Record(%q) error = %v", event.ID, err)
		}
	}

	for _, update := range []store.TokenStatsUpdate{
		{
			SessionID:    run.SessionID,
			AgentName:    "codex",
			InputTokens:  int64Ptr(10),
			OutputTokens: int64Ptr(6),
			TotalTokens:  int64Ptr(16),
			CostAmount:   float64Ptr(0.2),
			CostCurrency: stringPtr("USD"),
			CostStatus:   "actual",
			CostSource:   "agent_reported",
			Turns:        1,
			UpdatedAt:    fixedNow.Add(5 * time.Minute),
		},
		{
			SessionID:    run.SessionID,
			AgentName:    "reviewer",
			InputTokens:  int64Ptr(4),
			TotalTokens:  int64Ptr(4),
			CostAmount:   float64Ptr(0.1),
			CostCurrency: stringPtr("USD"),
			CostStatus:   "actual",
			CostSource:   "agent_reported",
			Turns:        2,
			UpdatedAt:    fixedNow.Add(6 * time.Minute),
		},
	} {
		if err := db.UpdateTokenStats(ctx, update); err != nil {
			t.Fatalf("UpdateTokenStats(%q) error = %v", update.AgentName, err)
		}
	}

	detail, err := manager.RunDetail(ctx, run.ID, actor)
	if err != nil {
		t.Fatalf("RunDetail() error = %v", err)
	}
	if got, want := detail.Task.ID, taskRecord.ID; got != want {
		t.Fatalf("detail.Task.ID = %q, want %q", got, want)
	}
	if got, want := detail.Task.Status, taskpkg.TaskStatusInProgress; got != want {
		t.Fatalf("detail.Task.Status = %q, want %q", got, want)
	}
	if detail.Session == nil {
		t.Fatal("detail.Session = nil, want session reference")
	}
	if got, want := detail.Session.AgentName, "codex"; got != want {
		t.Fatalf("detail.Session.AgentName = %q, want %q", got, want)
	}
	if detail.Summary.ToolCallCount == nil || *detail.Summary.ToolCallCount != 2 {
		t.Fatalf("detail.Summary.ToolCallCount = %#v, want 2", detail.Summary.ToolCallCount)
	}
	if detail.Summary.InputTokens == nil || *detail.Summary.InputTokens != 14 {
		t.Fatalf("detail.Summary.InputTokens = %#v, want 14", detail.Summary.InputTokens)
	}
	if detail.Summary.OutputTokens == nil || *detail.Summary.OutputTokens != 6 {
		t.Fatalf("detail.Summary.OutputTokens = %#v, want 6", detail.Summary.OutputTokens)
	}
	if detail.Summary.TotalTokens == nil || *detail.Summary.TotalTokens != 20 {
		t.Fatalf("detail.Summary.TotalTokens = %#v, want 20", detail.Summary.TotalTokens)
	}
	if detail.Summary.TurnCount == nil || *detail.Summary.TurnCount != 3 {
		t.Fatalf("detail.Summary.TurnCount = %#v, want 3", detail.Summary.TurnCount)
	}
	if detail.Summary.TotalCost == nil || math.Abs(*detail.Summary.TotalCost-0.3) > 1e-9 {
		t.Fatalf("detail.Summary.TotalCost = %#v, want 0.3", detail.Summary.TotalCost)
	}
	if detail.Summary.CostCurrency == nil || *detail.Summary.CostCurrency != "USD" {
		t.Fatalf("detail.Summary.CostCurrency = %#v, want USD", detail.Summary.CostCurrency)
	}
	if detail.Summary.CostStatus != "actual" || detail.Summary.CostSource != "agent_reported" {
		t.Fatalf("detail.Summary cost provenance = %q/%q, want actual/agent_reported", detail.Summary.CostStatus, detail.Summary.CostSource)
	}
	if got, want := detail.Summary.LastEventType, "tool_call"; got != want {
		t.Fatalf("detail.Summary.LastEventType = %q, want %q", got, want)
	}
	if got, want := detail.Summary.LastActivityAt, fixedNow.Add(6*time.Minute); !got.Equal(want) {
		t.Fatalf("detail.Summary.LastActivityAt = %s, want %s", got, want)
	}
}

func TestTaskManagerTreeLiveViewIntegration(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	executor := &integrationSessionExecutor{}
	clock := incrementingClock(time.Date(2026, 4, 17, 16, 0, 0, 0, time.UTC), time.Minute)
	counter := 0
	manager := newTaskManagerIntegration(
		t,
		db,
		taskpkg.WithSessionExecutor(executor),
		taskpkg.WithManagerNow(clock),
		taskpkg.WithIDGenerator(func(prefix string) string {
			counter++
			return prefix + "-tree-" + strconv.Itoa(counter)
		}),
	)
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task live-tree")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	root, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Root live task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(root) error = %v", err)
	}
	childActive, err := manager.CreateChildTask(ctx, root.ID, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Active child",
	}, actor)
	if err != nil {
		t.Fatalf("CreateChildTask(active) error = %v", err)
	}
	childIdle, err := manager.CreateChildTask(ctx, root.ID, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Idle child",
	}, actor)
	if err != nil {
		t.Fatalf("CreateChildTask(idle) error = %v", err)
	}
	grandchild, err := manager.CreateChildTask(ctx, childActive.ID, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Grandchild",
	}, actor)
	if err != nil {
		t.Fatalf("CreateChildTask(grandchild) error = %v", err)
	}

	run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: childActive.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	run, err = seedNonLeasedClaimedRunIntegration(ctx, manager, db, run.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	run, err = manager.StartRun(ctx, run.ID, taskpkg.StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}

	tree, err := manager.Tree(ctx, root.ID, actor)
	if err != nil {
		t.Fatalf("Tree() error = %v", err)
	}
	if got, want := tree.Root.Task.ID, root.ID; got != want {
		t.Fatalf("tree.Root.Task.ID = %q, want %q", got, want)
	}
	if got, want := len(tree.Descendants), 3; got != want {
		t.Fatalf("len(tree.Descendants) = %d, want %d", got, want)
	}
	if got, want := []string{
		tree.Descendants[0].Task.ID,
		tree.Descendants[1].Task.ID,
		tree.Descendants[2].Task.ID,
	}, []string{childActive.ID, childIdle.ID, grandchild.ID}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("tree.Descendants order = %#v, want %#v", got, want)
	}
	if tree.Descendants[0].ActiveRun == nil || tree.Descendants[0].ActiveRun.ID != run.ID {
		t.Fatalf("tree.Descendants[0].ActiveRun = %#v, want run %q", tree.Descendants[0].ActiveRun, run.ID)
	}
	if got, want := tree.Descendants[0].Depth, 1; got != want {
		t.Fatalf("tree.Descendants[0].Depth = %d, want %d", got, want)
	}
	if got, want := tree.Descendants[0].ChildCount, 1; got != want {
		t.Fatalf("tree.Descendants[0].ChildCount = %d, want %d", got, want)
	}
	if got, want := tree.Descendants[2].Task.ID, grandchild.ID; got != want {
		t.Fatalf("tree.Descendants[2].Task.ID = %q, want %q", got, want)
	}
	if got, want := tree.Descendants[2].ParentTaskID, childActive.ID; got != want {
		t.Fatalf("tree.Descendants[2].ParentTaskID = %q, want %q", got, want)
	}
	if got, want := tree.Descendants[2].Depth, 2; got != want {
		t.Fatalf("tree.Descendants[2].Depth = %d, want %d", got, want)
	}
}

func TestTaskManagerStreamSupportsReplayAndReconnectIntegration(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	executor := &integrationSessionExecutor{}
	clock := incrementingClock(time.Date(2026, 4, 17, 17, 0, 0, 0, time.UTC), time.Minute)
	counter := 0
	manager := newTaskManagerIntegration(
		t,
		db,
		taskpkg.WithSessionExecutor(executor),
		taskpkg.WithManagerNow(clock),
		taskpkg.WithIDGenerator(func(prefix string) string {
			counter++
			return prefix + "-stream-" + strconv.Itoa(counter)
		}),
	)
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task live-stream")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	root, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Stream root",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask(root) error = %v", err)
	}
	child, err := manager.CreateChildTask(ctx, root.ID, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Stream child",
	}, actor)
	if err != nil {
		t.Fatalf("CreateChildTask(child) error = %v", err)
	}

	streamCtx, cancel := context.WithCancel(ctx)
	stream, err := manager.Stream(streamCtx, root.ID, taskpkg.StreamQuery{AfterSequence: 1}, actor)
	if err != nil {
		t.Fatalf("Stream(first) error = %v", err)
	}

	backlogChildCreated := awaitIntegrationTaskStreamEvent(t, stream)
	backlogParentJoin := awaitIntegrationTaskStreamEvent(t, stream)
	if got, want := []int64{
		backlogChildCreated.Sequence,
		backlogParentJoin.Sequence,
	}, []int64{
		2,
		3,
	}; !equalInt64s(
		got,
		want,
	) {
		t.Fatalf("backlog sequences = %#v, want [2 3]", got)
	}
	if got, want := backlogChildCreated.Timeline.Task.ID, child.ID; got != want {
		t.Fatalf("backlogChildCreated.Timeline.Task.ID = %q, want %q", got, want)
	}
	if got, want := backlogChildCreated.Type, "task.created"; got != want {
		t.Fatalf("backlogChildCreated.Type = %q, want %q", got, want)
	}
	if got, want := backlogParentJoin.Timeline.Task.ID, root.ID; got != want {
		t.Fatalf("backlogParentJoin.Timeline.Task.ID = %q, want %q", got, want)
	}
	if got, want := backlogParentJoin.Type, "task.child_created"; got != want {
		t.Fatalf("backlogParentJoin.Type = %q, want %q", got, want)
	}

	run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: child.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	liveEnqueued := awaitIntegrationTaskStreamEvent(t, stream)
	if got, want := liveEnqueued.Sequence, int64(4); got != want {
		t.Fatalf("liveEnqueued.Sequence = %d, want %d", got, want)
	}
	if got, want := liveEnqueued.Timeline.Task.ID, child.ID; got != want {
		t.Fatalf("liveEnqueued.Timeline.Task.ID = %q, want %q", got, want)
	}
	if got, want := liveEnqueued.Type, "task.run_enqueued"; got != want {
		t.Fatalf("liveEnqueued.Type = %q, want %q", got, want)
	}

	claim, err := claimExactRunIntegration(ctx, manager, db, run.ID, actor)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	run = &claim.Run
	liveClaimed := awaitIntegrationTaskStreamEvent(t, stream)
	if got, want := liveClaimed.Type, "task.run_claimed"; got != want {
		t.Fatalf("liveClaimed.Type = %q, want %q", got, want)
	}

	run, err = manager.StartRun(ctx, run.ID, taskpkg.StartRun{}, actor)
	if err != nil {
		t.Fatalf("StartRun() error = %v", err)
	}
	liveStatusChanged := awaitIntegrationTaskStreamEvent(t, stream)
	liveStarting := awaitIntegrationTaskStreamEvent(t, stream)
	liveBound := awaitIntegrationTaskStreamEvent(t, stream)
	liveStarted := awaitIntegrationTaskStreamEvent(t, stream)
	if got, want := []string{
		liveStatusChanged.Type,
		liveStarting.Type,
		liveBound.Type,
		liveStarted.Type,
	}, []string{
		"task.status_changed",
		"task.run_starting",
		"task.run_session_bound",
		"task.run_started",
	}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("live start event types = %#v, want %#v", got, want)
	}
	lastSequence := liveStarted.Sequence
	cancel()

	reconnectCtx, reconnectCancel := context.WithCancel(ctx)
	defer reconnectCancel()
	reconnected, err := manager.Stream(reconnectCtx, root.ID, taskpkg.StreamQuery{AfterSequence: lastSequence}, actor)
	if err != nil {
		t.Fatalf("Stream(reconnected) error = %v", err)
	}
	assertNoIntegrationTaskStreamEvent(t, reconnected, 150*time.Millisecond)

	if _, err := manager.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
		RunID:      run.ID,
		ClaimToken: claim.ClaimToken,
		Result: taskpkg.RunResult{
			Value: json.RawMessage(`{"ok":true}`),
		},
		Now: claim.LeaseUntil.Add(-time.Second),
	}, actor); err != nil {
		t.Fatalf("CompleteRunLease() error = %v", err)
	}
	liveCompleted := awaitIntegrationTaskStreamEvent(t, reconnected)
	liveStatusCompleted := awaitIntegrationTaskStreamEvent(t, reconnected)
	liveParentStatusCompleted := awaitIntegrationTaskStreamEvent(t, reconnected)
	if got, want := liveStatusCompleted.Type, "task.status_changed"; got != want {
		t.Fatalf("liveStatusCompleted.Type = %q, want %q", got, want)
	}
	if got, want := liveStatusCompleted.Timeline.Task.ID, child.ID; got != want {
		t.Fatalf("liveStatusCompleted.Timeline.Task.ID = %q, want %q", got, want)
	}
	if got, want := liveParentStatusCompleted.Type, "task.status_changed"; got != want {
		t.Fatalf("liveParentStatusCompleted.Type = %q, want %q", got, want)
	}
	if got, want := liveParentStatusCompleted.Timeline.Task.ID, root.ID; got != want {
		t.Fatalf("liveParentStatusCompleted.Timeline.Task.ID = %q, want %q", got, want)
	}
	if liveCompleted.Sequence <= lastSequence || liveStatusCompleted.Sequence <= liveCompleted.Sequence {
		t.Fatalf(
			"completion sequences = completed:%d status:%d, want both after %d in order",
			liveCompleted.Sequence,
			liveStatusCompleted.Sequence,
			lastSequence,
		)
	}
	if got, want := liveCompleted.Timeline.Task.ID, child.ID; got != want {
		t.Fatalf("liveCompleted.Timeline.Task.ID = %q, want %q", got, want)
	}
	if got, want := liveCompleted.Type, "task.run.completed"; got != want {
		t.Fatalf("liveCompleted.Type = %q, want %q", got, want)
	}

	t.Run("Should publish committed recovery lifecycle events exactly once", func(t *testing.T) {
		attentionTask, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Stream recovered task",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask(attention) error = %v", err)
		}

		var latestBlock taskpkg.TaskBlock
		for idx := range 3 {
			latestBlock, err = manager.BlockTask(ctx, taskpkg.BlockRequest{
				TaskID: attentionTask.ID,
				Kind:   taskpkg.BlockKindNeedsInput,
				Reason: fmt.Sprintf("stream escalation %d", idx),
			}, actor)
			if err != nil {
				t.Fatalf("BlockTask(%d) error = %v", idx, err)
			}
			if idx < 2 {
				if _, err := manager.ClearTaskBlock(
					ctx,
					attentionTask.ID,
					latestBlock.ID,
					"resolved",
					actor,
				); err != nil {
					t.Fatalf("ClearTaskBlock(%d) error = %v", idx, err)
				}
			}
		}
		if _, err := manager.ClearTaskBlock(
			ctx,
			attentionTask.ID,
			latestBlock.ID,
			"resolved final",
			actor,
		); err != nil {
			t.Fatalf("ClearTaskBlock(final) error = %v", err)
		}

		view, err := manager.GetTask(ctx, attentionTask.ID, actor)
		if err != nil {
			t.Fatalf("GetTask(attention) error = %v", err)
		}
		if view.Task.NeedsAttention == nil {
			t.Fatal("GetTask(attention).NeedsAttention = nil, want escalation")
		}
		recoveryCtx, recoveryCancel := context.WithCancel(ctx)
		defer recoveryCancel()
		recoveryStream, err := manager.Stream(
			recoveryCtx,
			attentionTask.ID,
			taskpkg.StreamQuery{AfterSequence: view.Task.LatestEventSeq},
			actor,
		)
		if err != nil {
			t.Fatalf("Stream(recovery) error = %v", err)
		}

		if _, err := manager.RecoverTask(ctx, attentionTask.ID, "operator recovered", actor); err != nil {
			t.Fatalf("RecoverTask() error = %v", err)
		}
		recoveredEvent := awaitIntegrationTaskStreamEvent(t, recoveryStream)
		if got, want := recoveredEvent.Type, eventspkg.TaskRecovered; got != want {
			t.Fatalf("recoveredEvent.Type = %q, want %q", got, want)
		}
		statusEvent := awaitIntegrationTaskStreamEvent(t, recoveryStream)
		if got, want := statusEvent.Type, "task.status_changed"; got != want {
			t.Fatalf("statusEvent.Type = %q, want %q", got, want)
		}
		if statusEvent.Sequence <= recoveredEvent.Sequence {
			t.Fatalf(
				"statusEvent.Sequence = %d, want > recovered sequence %d",
				statusEvent.Sequence,
				recoveredEvent.Sequence,
			)
		}
		assertNoIntegrationTaskStreamEvent(t, recoveryStream, 150*time.Millisecond)
	})
}

func TestTaskManagerBlockReleaseUnblockClaimableCycleIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should park release and reclaim workspace task runs after unblock", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		claimNow := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		manager := newTaskManagerIntegration(t, db, taskpkg.WithManagerNow(func() time.Time {
			return claimNow
		}))
		workspaceID := registerTaskManagerWorkspace(
			t,
			db,
			"block-release-cycle",
			filepath.Join(t.TempDir(), "workspace"),
		)
		operator, err := taskpkg.DeriveHumanActorContext("operator-1", taskpkg.OriginKindCLI, "agh task block")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		agent, err := taskpkg.DeriveAgentSessionActorContext("sess-block-cycle", workspaceID)
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
		}

		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Title:       "Block release cycle",
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, operator)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      workspaceID,
			ClaimerSessionID: "sess-block-cycle",
			LeaseDuration:    2 * time.Minute,
			Now:              claimNow,
		}, agent)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		if got, want := claim.Run.ID, run.ID; got != want {
			t.Fatalf("ClaimNextRun().Run.ID = %q, want %q", got, want)
		}

		block, err := manager.BlockTask(ctx, taskpkg.BlockRequest{
			TaskID:     taskRecord.ID,
			Kind:       taskpkg.BlockKindNeedsInput,
			Reason:     "creator clarification required",
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
		}, agent)
		if err != nil {
			t.Fatalf("BlockTask(active run) error = %v", err)
		}
		if got, want := block.WorkspaceID, workspaceID; got != want {
			t.Fatalf("block.WorkspaceID = %q, want %q", got, want)
		}
		parked, err := db.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(parked) error = %v", err)
		}
		if parked.Status != taskpkg.TaskRunStatusQueued ||
			parked.Attempt != run.Attempt ||
			parked.SessionID != "" ||
			parked.ClaimTokenHash != "" ||
			!parked.LeaseUntil.IsZero() {
			t.Fatalf("parked run = %#v, want queued unleased run with unchanged attempt %d", parked, run.Attempt)
		}
		blockedTask, err := db.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(blocked) error = %v", err)
		}
		if got, want := blockedTask.Status, taskpkg.TaskStatusBlocked; got != want {
			t.Fatalf("blockedTask.Status = %q, want %q", got, want)
		}

		cleared, err := manager.ClearTaskBlock(ctx, taskRecord.ID, block.ID, "creator answered", operator)
		if err != nil {
			t.Fatalf("ClearTaskBlock() error = %v", err)
		}
		if cleared.ClearedAt.IsZero() {
			t.Fatal("ClearTaskBlock().ClearedAt is zero, want clear stamp")
		}
		readyTask, err := db.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(ready) error = %v", err)
		}
		if got, want := readyTask.Status, taskpkg.TaskStatusReady; got != want {
			t.Fatalf("readyTask.Status = %q, want %q", got, want)
		}

		reclaimed, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      workspaceID,
			ClaimerSessionID: "sess-block-cycle",
			LeaseDuration:    2 * time.Minute,
			Now:              claimNow.Add(time.Minute),
		}, agent)
		if err != nil {
			t.Fatalf("ClaimNextRun(after unblock) error = %v", err)
		}
		if got, want := reclaimed.Run.ID, run.ID; got != want {
			t.Fatalf("reclaimed.Run.ID = %q, want same parked run %q", got, want)
		}
		if got, want := reclaimed.Run.Attempt, run.Attempt; got != want {
			t.Fatalf("reclaimed.Run.Attempt = %d, want unchanged %d", got, want)
		}
		if reclaimed.ClaimToken == "" ||
			reclaimed.ClaimToken == claim.ClaimToken ||
			!taskpkg.VerifyClaimToken(reclaimed.ClaimToken, reclaimed.Run.ClaimTokenHash) {
			t.Fatalf(
				"reclaimed claim token/hash invalid: token=%q hash=%q",
				reclaimed.ClaimToken,
				reclaimed.Run.ClaimTokenHash,
			)
		}
	})
}

func TestTaskManagerGlobalBlockReleaseUnblockClaimableCycleIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should park release and reclaim global task runs after unblock", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		claimNow := time.Date(2026, 4, 26, 13, 0, 0, 0, time.UTC)
		manager := newTaskManagerIntegration(t, db, taskpkg.WithManagerNow(func() time.Time {
			return claimNow
		}))
		operator, err := taskpkg.DeriveHumanActorContext("operator-global", taskpkg.OriginKindCLI, "agh task block")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		agent, err := taskpkg.DeriveAgentSessionActorContext("sess-global-block-cycle", "ws-test")
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
		}

		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Global block release cycle",
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask(global) error = %v", err)
		}
		run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, operator)
		if err != nil {
			t.Fatalf("EnqueueRun(global) error = %v", err)
		}
		claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-global-block-cycle",
			LeaseDuration:    2 * time.Minute,
			Now:              claimNow,
		}, agent)
		if err != nil {
			t.Fatalf("ClaimNextRun(global) error = %v", err)
		}
		if got, want := claim.Run.ID, run.ID; got != want {
			t.Fatalf("ClaimNextRun(global).Run.ID = %q, want %q", got, want)
		}

		block, err := manager.BlockTask(ctx, taskpkg.BlockRequest{
			TaskID:     taskRecord.ID,
			Kind:       taskpkg.BlockKindNeedsInput,
			Reason:     "global creator clarification required",
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
		}, agent)
		if err != nil {
			t.Fatalf("BlockTask(global active run) error = %v", err)
		}
		if got, want := block.WorkspaceID, ""; got != want {
			t.Fatalf("block.WorkspaceID = %q, want global empty workspace", got)
		}
		parked, err := db.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(global parked) error = %v", err)
		}
		if parked.Status != taskpkg.TaskRunStatusQueued ||
			parked.Attempt != run.Attempt ||
			parked.SessionID != "" ||
			parked.ClaimTokenHash != "" ||
			!parked.LeaseUntil.IsZero() {
			t.Fatalf("global parked run = %#v, want queued unleased run with unchanged attempt %d", parked, run.Attempt)
		}
		blockedTask, err := db.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(global blocked) error = %v", err)
		}
		if got, want := blockedTask.Status, taskpkg.TaskStatusBlocked; got != want {
			t.Fatalf("global blockedTask.Status = %q, want %q", got, want)
		}

		cleared, err := manager.ClearTaskBlock(ctx, taskRecord.ID, block.ID, "global creator answered", operator)
		if err != nil {
			t.Fatalf("ClearTaskBlock(global) error = %v", err)
		}
		if cleared.WorkspaceID != "" || cleared.ClearedAt.IsZero() {
			t.Fatalf("cleared global block = %#v, want empty workspace and clear stamp", cleared)
		}
		readyTask, err := db.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(global ready) error = %v", err)
		}
		if got, want := readyTask.Status, taskpkg.TaskStatusReady; got != want {
			t.Fatalf("global readyTask.Status = %q, want %q", got, want)
		}

		reclaimed, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-global-block-cycle",
			LeaseDuration:    2 * time.Minute,
			Now:              claimNow.Add(time.Minute),
		}, agent)
		if err != nil {
			t.Fatalf("ClaimNextRun(global after unblock) error = %v", err)
		}
		if got, want := reclaimed.Run.ID, run.ID; got != want {
			t.Fatalf("reclaimed global Run.ID = %q, want same parked run %q", got, want)
		}
		if got, want := reclaimed.Run.Attempt, run.Attempt; got != want {
			t.Fatalf("reclaimed global Run.Attempt = %d, want unchanged %d", got, want)
		}
		if reclaimed.ClaimToken == "" ||
			reclaimed.ClaimToken == claim.ClaimToken ||
			!taskpkg.VerifyClaimToken(reclaimed.ClaimToken, reclaimed.Run.ClaimTokenHash) {
			t.Fatalf(
				"reclaimed global claim token/hash invalid: token=%q hash=%q",
				reclaimed.ClaimToken,
				reclaimed.Run.ClaimTokenHash,
			)
		}
	})
}

func openTaskManagerGlobalDB(t *testing.T) *globaldb.GlobalDB {
	t.Helper()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), "agh.db")
	db, err := globaldb.OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Fatalf("GlobalDB.Close() error = %v", err)
		}
	})
	return db
}

func newTaskManagerIntegration(t *testing.T, store taskpkg.Store, extraOpts ...taskpkg.Option) *taskpkg.Service {
	t.Helper()

	options := []taskpkg.Option{taskpkg.WithStore(store)}
	options = append(options, extraOpts...)
	manager, err := taskpkg.NewManager(options...)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}

func newTaskParticipationResolver(
	t *testing.T,
	db *globaldb.GlobalDB,
	authority participation.AuthorityFunc,
) participation.Resolver {
	t.Helper()

	defaults := participation.Bounds{
		MaxWakes:         4,
		MaxWakeWallTime:  "30s",
		MaxTotalWallTime: "2m",
		MaxInputTokens:   4096,
		MaxOutputTokens:  4096,
		MaxWakeDepth:     4,
		CoalesceWindow:   "250ms",
	}
	resolver, err := participation.NewResolver(participation.ResolverOptions{
		Defaults: defaults,
		Limits: participation.Limits{
			MaxWakes:          16,
			MaxWakeWallTime:   "2m",
			MaxTotalWallTime:  "10m",
			MaxInputTokens:    65536,
			MaxOutputTokens:   65536,
			MaxWakeDepth:      16,
			MinCoalesceWindow: "100ms",
			MaxCoalesceWindow: "5s",
		},
		Availability: func(ctx context.Context) (bool, error) {
			state, readErr := db.GetNetworkAvailability(ctx)
			if readErr != nil {
				return false, readErr
			}
			return state.Enabled, nil
		},
		ChannelExists: func(ctx context.Context, workspaceID, channelID string) (bool, error) {
			_, readErr := db.GetNetworkChannel(ctx, store.NetworkChannelRef{
				WorkspaceID: workspaceID,
				Channel:     channelID,
			})
			switch {
			case errors.Is(readErr, sql.ErrNoRows):
				return false, nil
			case readErr != nil:
				return false, readErr
			default:
				return true, nil
			}
		},
		WorkspaceCoordination: func(ctx context.Context, workspaceID string) (bool, error) {
			setting, readErr := db.Get(ctx, workspaceID)
			if readErr != nil {
				return false, readErr
			}
			return setting.Enabled, nil
		},
		LiveSupport: func(context.Context, participation.ResolveInput) (bool, error) {
			return true, nil
		},
		Authority: authority,
	})
	if err != nil {
		t.Fatalf("participation.NewResolver() error = %v", err)
	}
	return resolver
}

func registerTaskManagerWorkspace(t *testing.T, db *globaldb.GlobalDB, name string, rootDir string) string {
	t.Helper()

	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", rootDir, err)
	}

	workspace := aghworkspace.Workspace{
		ID:        "ws-" + strings.ReplaceAll(name, " ", "-"),
		RootDir:   rootDir,
		Name:      name,
		CreatedAt: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
	}
	if err := db.InsertWorkspace(testutil.Context(t), workspace); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	return workspace.ID
}

func intPtr(value int) *int {
	return &value
}

func sortedEventTypes(events []taskpkg.Event) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.EventType)
	}
	sort.Strings(types)
	return types
}

func containsEventType(events []taskpkg.Event, want string) bool {
	for _, event := range events {
		if event.EventType == want {
			return true
		}
	}
	return false
}

func createIntegrationClaimedRun(
	t *testing.T,
	ctx context.Context,
	db *globaldb.GlobalDB,
	manager *taskpkg.Service,
	creator taskpkg.ActorContext,
	claimer taskpkg.ActorContext,
	workspaceID string,
	title string,
	claimerSessionID string,
	now time.Time,
) (*taskpkg.Task, *taskpkg.Run, *taskpkg.ClaimResult) {
	t.Helper()

	taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: workspaceID,
		Title:       title,
	}, creator)
	if err != nil {
		t.Fatalf("CreateTask(%q) error = %v", title, err)
	}
	run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, creator)
	if err != nil {
		t.Fatalf("EnqueueRun(%q) error = %v", title, err)
	}
	claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      workspaceID,
		ClaimerSessionID: claimerSessionID,
		LeaseDuration:    10 * time.Minute,
		Now:              now,
	}, claimer)
	if err != nil {
		t.Fatalf("ClaimNextRun(%q) error = %v", title, err)
	}
	if got, want := claim.Run.ID, run.ID; got != want {
		t.Fatalf("ClaimNextRun(%q).Run.ID = %q, want %q", title, got, want)
	}
	return taskRecord, run, claim
}

func findIntegrationTaskEvent(
	t *testing.T,
	ctx context.Context,
	db *globaldb.GlobalDB,
	query taskpkg.EventQuery,
) taskpkg.Event {
	t.Helper()

	events, err := db.ListTaskEvents(ctx, query)
	if err != nil {
		t.Fatalf("ListTaskEvents(%#v) error = %v", query, err)
	}
	if got, want := len(events), 1; got != want {
		t.Fatalf(
			"ListTaskEvents(%#v) count = %d, want %d; event types = %#v",
			query,
			got,
			want,
			sortedEventTypes(events),
		)
	}
	return events[0]
}

func assertIntegrationEventCorrelation(
	t *testing.T,
	event taskpkg.Event,
	taskID string,
	runID string,
	eventType string,
	actorKind taskpkg.ActorKind,
	actorRef string,
	originKind taskpkg.OriginKind,
) {
	t.Helper()

	if strings.TrimSpace(event.ID) == "" {
		t.Fatal("event.ID is empty")
	}
	if event.Timestamp.IsZero() {
		t.Fatal("event.Timestamp is zero")
	}
	if got, want := event.TaskID, taskID; got != want {
		t.Fatalf("event.TaskID = %q, want %q", got, want)
	}
	if got, want := event.RunID, runID; got != want {
		t.Fatalf("event.RunID = %q, want %q", got, want)
	}
	if got, want := event.EventType, eventType; got != want {
		t.Fatalf("event.EventType = %q, want %q", got, want)
	}
	if got, want := event.Actor.Kind, actorKind; got != want {
		t.Fatalf("event.Actor.Kind = %q, want %q", got, want)
	}
	if got, want := event.Actor.Ref, actorRef; got != want {
		t.Fatalf("event.Actor.Ref = %q, want %q", got, want)
	}
	if got, want := event.Origin.Kind, originKind; got != want {
		t.Fatalf("event.Origin.Kind = %q, want %q", got, want)
	}
	if strings.TrimSpace(event.Origin.Ref) == "" {
		t.Fatal("event.Origin.Ref is empty")
	}
}

func decodeIntegrationTaskEventPayload(t *testing.T, event taskpkg.Event) map[string]any {
	t.Helper()

	if len(event.Payload) == 0 {
		t.Fatalf("event %s payload is empty", event.EventType)
	}
	payload := make(map[string]any)
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("decode event %s payload %s: %v", event.EventType, string(event.Payload), err)
	}
	return payload
}

func assertIntegrationPayloadString(t *testing.T, payload map[string]any, key string, want string) {
	t.Helper()

	got, ok := payload[key].(string)
	if !ok {
		t.Fatalf("payload[%q] = %#v, want string %q", key, payload[key], want)
	}
	if got != want {
		t.Fatalf("payload[%q] = %q, want %q", key, got, want)
	}
}

func assertIntegrationPayloadHasString(t *testing.T, payload map[string]any, key string) string {
	t.Helper()

	got, ok := payload[key].(string)
	if !ok {
		t.Fatalf("payload[%q] = %#v, want non-empty string", key, payload[key])
	}
	if strings.TrimSpace(got) == "" {
		t.Fatalf("payload[%q] is empty", key)
	}
	return got
}

func assertIntegrationPayloadNumberAtLeast(t *testing.T, payload map[string]any, key string, minimum float64) {
	t.Helper()

	got, ok := payload[key].(float64)
	if !ok {
		t.Fatalf("payload[%q] = %#v, want number >= %v", key, payload[key], minimum)
	}
	if got < minimum {
		t.Fatalf("payload[%q] = %v, want >= %v", key, got, minimum)
	}
}

func assertIntegrationPayloadStringSlice(t *testing.T, payload map[string]any, key string, want []string) {
	t.Helper()

	raw, ok := payload[key].([]any)
	if !ok {
		t.Fatalf("payload[%q] = %#v, want string slice %#v", key, payload[key], want)
	}
	got := make([]string, 0, len(raw))
	for idx, item := range raw {
		value, itemOK := item.(string)
		if !itemOK {
			t.Fatalf("payload[%q][%d] = %#v, want string", key, idx, item)
		}
		got = append(got, value)
	}
	if len(got) != len(want) {
		t.Fatalf("payload[%q] = %#v, want %#v", key, got, want)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("payload[%q] = %#v, want %#v", key, got, want)
		}
	}
}

func assertIntegrationPayloadOmitsRawValue(t *testing.T, event taskpkg.Event, raw string) {
	t.Helper()

	if strings.TrimSpace(raw) == "" {
		t.Fatal("raw value is empty; cannot assert redaction")
	}
	if strings.Contains(string(event.Payload), raw) {
		t.Fatalf("event %s payload leaked raw value %q: %s", event.EventType, raw, string(event.Payload))
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}

func float64Ptr(value float64) *float64 {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func incrementingClock(start time.Time, step time.Duration) func() time.Time {
	current := start.Add(-step)
	return func() time.Time {
		current = current.Add(step)
		return current
	}
}

func awaitIntegrationTaskStreamEvent(
	t *testing.T,
	stream <-chan taskpkg.StreamEvent,
) taskpkg.StreamEvent {
	t.Helper()

	select {
	case event, ok := <-stream:
		if !ok {
			t.Fatal("task stream closed before event was available")
		}
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for task stream event")
		return taskpkg.StreamEvent{}
	}
}

func assertNoIntegrationTaskStreamEvent(
	t *testing.T,
	stream <-chan taskpkg.StreamEvent,
	wait time.Duration,
) {
	t.Helper()

	select {
	case event, ok := <-stream:
		if !ok {
			return
		}
		t.Fatalf("unexpected task stream event = %#v", event)
	case <-time.After(wait):
	}
}

func equalInt64s(left []int64, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func TestTaskManagerGetTaskRequiresReadAuthorityIntegration(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openTaskManagerGlobalDB(t)
	manager := newTaskManagerIntegration(t, db)
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh task create")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope: taskpkg.ScopeGlobal,
		Title: "Read auth check",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	denied := actor
	denied.Authority.Read = false
	_, err = manager.GetTask(ctx, taskRecord.ID, denied)
	if !errors.Is(err, taskpkg.ErrPermissionDenied) {
		t.Fatalf("GetTask(no read) error = %v, want %v", err, taskpkg.ErrPermissionDenied)
	}
}

func TestTaskManagerBlockBreakerRecoverAndCompletionResetIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should recover block breaker state and reset recurrence after completion", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		manager := newTaskManagerIntegration(t, db)
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "agh task recover")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Breaker integration target",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		var latest taskpkg.TaskBlock
		for idx := range 3 {
			latest, err = manager.BlockTask(ctx, taskpkg.BlockRequest{
				TaskID: taskRecord.ID,
				Kind:   taskpkg.BlockKindNeedsInput,
				Reason: fmt.Sprintf("input loop %d", idx),
			}, actor)
			if err != nil {
				t.Fatalf("BlockTask(%d) error = %v", idx, err)
			}
			if idx < 2 {
				if _, err := manager.ClearTaskBlock(ctx, taskRecord.ID, latest.ID, "resolved", actor); err != nil {
					t.Fatalf("ClearTaskBlock(%d) error = %v", idx, err)
				}
			}
		}
		escalated, err := db.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(escalated) error = %v", err)
		}
		if got, want := escalated.Status, taskpkg.TaskStatusNeedsAttention; got != want {
			t.Fatalf("escalated Status = %q, want %q", got, want)
		}
		if escalated.NeedsAttention == nil {
			t.Fatal("NeedsAttention = nil, want durable escalation")
		}

		if _, err := manager.ClearTaskBlock(ctx, taskRecord.ID, latest.ID, "resolved final", actor); err != nil {
			t.Fatalf("ClearTaskBlock(final) error = %v", err)
		}
		recovered, err := manager.RecoverTask(ctx, taskRecord.ID, "operator reviewed", actor)
		if err != nil {
			t.Fatalf("RecoverTask() error = %v", err)
		}
		if got, want := recovered.Status, taskpkg.TaskStatusReady; got != want {
			t.Fatalf("RecoverTask().Status = %q, want %q", got, want)
		}
		reblock, err := manager.BlockTask(ctx, taskpkg.BlockRequest{
			TaskID: taskRecord.ID,
			Kind:   taskpkg.BlockKindNeedsInput,
			Reason: "input loop after recover",
		}, actor)
		if err != nil {
			t.Fatalf("BlockTask(after recover) error = %v", err)
		}
		escalatedAgain, err := db.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(escalated again) error = %v", err)
		}
		if got, want := escalatedAgain.Status, taskpkg.TaskStatusNeedsAttention; got != want {
			t.Fatalf("Status after recover re-block = %q, want %q", got, want)
		}
		recurrenceBeforeCompletion, err := db.GetTaskBlockRecurrence(ctx, taskRecord.ID, taskpkg.BlockKindNeedsInput)
		if err != nil {
			t.Fatalf("GetTaskBlockRecurrence(before completion) error = %v", err)
		}
		if got, want := recurrenceBeforeCompletion.Count, 3; got != want {
			t.Fatalf("recurrence.Count after recover re-block = %d, want %d", got, want)
		}
		if _, err := manager.ClearTaskBlock(
			ctx,
			taskRecord.ID,
			reblock.ID,
			"resolved after recover",
			actor,
		); err != nil {
			t.Fatalf("ClearTaskBlock(after recover) error = %v", err)
		}
		recoveredAgain, err := manager.RecoverTask(ctx, taskRecord.ID, "operator reviewed again", actor)
		if err != nil {
			t.Fatalf("RecoverTask(second) error = %v", err)
		}
		if got, want := recoveredAgain.Status, taskpkg.TaskStatusReady; got != want {
			t.Fatalf("RecoverTask(second).Status = %q, want %q", got, want)
		}

		run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, actor)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		agent, err := taskpkg.DeriveAgentSessionActorContext("sess-reset", "ws-test")
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
		}
		claimNow := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
		claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-reset",
			LeaseDuration:    time.Minute,
			Now:              claimNow,
		}, agent)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		if claim.Run.ID != run.ID {
			t.Fatalf("ClaimNextRun().Run.ID = %q, want %q", claim.Run.ID, run.ID)
		}
		if _, err := manager.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Result:     taskpkg.RunResult{Value: json.RawMessage(`{"ok":true}`)},
			Now:        claimNow.Add(30 * time.Second),
		}, agent); err != nil {
			t.Fatalf("CompleteRunLease() error = %v", err)
		}
		recurrence, err := db.GetTaskBlockRecurrence(ctx, taskRecord.ID, taskpkg.BlockKindNeedsInput)
		if err != nil {
			t.Fatalf("GetTaskBlockRecurrence() error = %v", err)
		}
		if got, want := recurrence.Count, 0; got != want {
			t.Fatalf("recurrence.Count after completion = %d, want %d", got, want)
		}
	})
}

func TestTaskManagerExpireTransientBlocksAutoEnqueuesIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should auto enqueue runs after transient block expiry", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 6, 3, 13, 0, 0, 0, time.UTC)
		db := openTaskManagerGlobalDB(t)
		manager := newTaskManagerIntegration(t, db, taskpkg.WithManagerNow(func() time.Time { return now }))
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "agh task block")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:              taskpkg.ScopeGlobal,
			Title:              "Transient expiry integration target",
			AutoEnqueueOnReady: true,
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		block, err := manager.BlockTask(ctx, taskpkg.BlockRequest{
			TaskID:    taskRecord.ID,
			Kind:      taskpkg.BlockKindTransient,
			Reason:    "temporary outage",
			ExpiresAt: now.Add(time.Minute),
		}, actor)
		if err != nil {
			t.Fatalf("BlockTask(transient) error = %v", err)
		}
		daemon, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		result, err := manager.ExpireTaskBlocks(ctx, now.Add(2*time.Minute), daemon)
		if err != nil {
			t.Fatalf("ExpireTaskBlocks() error = %v", err)
		}
		if got, want := len(result.Blocks), 1; got != want {
			t.Fatalf("expired blocks = %d, want %d", got, want)
		}
		if result.Blocks[0].ID != block.ID {
			t.Fatalf("expired block ID = %q, want %q", result.Blocks[0].ID, block.ID)
		}
		if got, want := result.Blocks[0].ClearedBy.Kind, taskpkg.ActorKindDaemon; got != want {
			t.Fatalf("expired block ClearedBy.Kind = %q, want %q", got, want)
		}
		runs, err := manager.ListTaskRuns(ctx, taskRecord.ID, taskpkg.RunQuery{}, actor)
		if err != nil {
			t.Fatalf("ListTaskRuns() error = %v", err)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("runs after expiry auto enqueue = %d, want %d", got, want)
		}
		if got, want := runs[0].Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("auto-enqueued run Status = %q, want %q", got, want)
		}
	})
}

func TestTaskManagerObservabilityCoverageMatrixIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should emit canonical task-block lifecycle events with correlation keys", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		now := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
		manager := newTaskManagerIntegration(t, db, taskpkg.WithManagerNow(incrementingClock(now, time.Second)))
		workspaceID := registerTaskManagerWorkspace(
			t,
			db,
			"observability-coverage",
			filepath.Join(t.TempDir(), "workspace"),
		)
		operator, err := taskpkg.DeriveHumanActorContext(
			"operator-observability",
			taskpkg.OriginKindCLI,
			"agh task qa",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		agent, err := taskpkg.DeriveAgentSessionActorContext("sess-observability-worker", workspaceID)
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
		}

		releasedTask, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: workspaceID,
			Title:       "Coverage matrix release",
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask(release) error = %v", err)
		}
		releasedRun, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: releasedTask.ID}, operator)
		if err != nil {
			t.Fatalf("EnqueueRun(release) error = %v", err)
		}
		releaseClaim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      workspaceID,
			ClaimerSessionID: "sess-observability-worker",
			LeaseDuration:    time.Minute,
			Now:              now.Add(time.Minute),
		}, agent)
		if err != nil {
			t.Fatalf("ClaimNextRun(release) error = %v", err)
		}
		if got, want := releaseClaim.Run.ID, releasedRun.ID; got != want {
			t.Fatalf("release claim Run.ID = %q, want %q", got, want)
		}
		releaseBlock, err := manager.BlockTask(ctx, taskpkg.BlockRequest{
			TaskID:     releasedTask.ID,
			Kind:       taskpkg.BlockKindNeedsInput,
			Reason:     "creator clarification required",
			RunID:      releaseClaim.Run.ID,
			ClaimToken: releaseClaim.ClaimToken,
		}, agent)
		if err != nil {
			t.Fatalf("BlockTask(release) error = %v", err)
		}
		releasedBlockEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    releasedTask.ID,
			RunID:     releasedRun.ID,
			EventType: eventspkg.TaskBlockCreated,
		})
		assertIntegrationEventCorrelation(
			t,
			releasedBlockEvent,
			releasedTask.ID,
			releasedRun.ID,
			eventspkg.TaskBlockCreated,
			taskpkg.ActorKindAgentSession,
			"sess-observability-worker",
			taskpkg.OriginKindAgentSession,
		)
		releasedBlockPayload := decodeIntegrationTaskEventPayload(t, releasedBlockEvent)
		assertIntegrationPayloadString(t, releasedBlockPayload, "status", string(taskpkg.TaskStatusBlocked))
		assertIntegrationPayloadString(t, releasedBlockPayload, "block_id", releaseBlock.ID)
		assertIntegrationPayloadString(t, releasedBlockPayload, "block_kind", string(taskpkg.BlockKindNeedsInput))
		assertIntegrationPayloadHasString(t, releasedBlockPayload, "claim_token_hash")
		assertIntegrationPayloadOmitsRawValue(t, releasedBlockEvent, releaseClaim.ClaimToken)

		releasedEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    releasedTask.ID,
			RunID:     releasedRun.ID,
			EventType: eventspkg.TaskRunReleased,
		})
		assertIntegrationEventCorrelation(
			t,
			releasedEvent,
			releasedTask.ID,
			releasedRun.ID,
			eventspkg.TaskRunReleased,
			taskpkg.ActorKindAgentSession,
			"sess-observability-worker",
			taskpkg.OriginKindAgentSession,
		)
		releasedPayload := decodeIntegrationTaskEventPayload(t, releasedEvent)
		assertIntegrationPayloadString(t, releasedPayload, "reason", "blocked")
		assertIntegrationPayloadString(t, releasedPayload, "status", taskpkg.TaskRunStatusQueued.String())
		assertIntegrationPayloadString(t, releasedPayload, "task_status", string(taskpkg.TaskStatusBlocked))
		assertIntegrationPayloadOmitsRawValue(t, releasedEvent, releaseClaim.ClaimToken)

		attentionTask, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Coverage matrix attention",
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask(attention) error = %v", err)
		}
		var latestBlock taskpkg.TaskBlock
		for idx := range 3 {
			latestBlock, err = manager.BlockTask(ctx, taskpkg.BlockRequest{
				TaskID: attentionTask.ID,
				Kind:   taskpkg.BlockKindNeedsInput,
				Reason: fmt.Sprintf("needs input loop %d", idx),
			}, operator)
			if err != nil {
				t.Fatalf("BlockTask(attention %d) error = %v", idx, err)
			}
			if idx < 2 {
				if _, err := manager.ClearTaskBlock(
					ctx,
					attentionTask.ID,
					latestBlock.ID,
					"resolved",
					operator,
				); err != nil {
					t.Fatalf("ClearTaskBlock(attention %d) error = %v", idx, err)
				}
			}
		}
		needsAttentionEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    attentionTask.ID,
			EventType: eventspkg.TaskNeedsAttention,
		})
		assertIntegrationEventCorrelation(
			t,
			needsAttentionEvent,
			attentionTask.ID,
			"",
			eventspkg.TaskNeedsAttention,
			taskpkg.ActorKindHuman,
			"operator-observability",
			taskpkg.OriginKindCLI,
		)
		needsAttentionPayload := decodeIntegrationTaskEventPayload(t, needsAttentionEvent)
		assertIntegrationPayloadString(t, needsAttentionPayload, "status", string(taskpkg.TaskStatusNeedsAttention))
		assertIntegrationPayloadString(t, needsAttentionPayload, "block_id", latestBlock.ID)
		assertIntegrationPayloadString(t, needsAttentionPayload, "block_kind", string(taskpkg.BlockKindNeedsInput))
		assertIntegrationPayloadNumberAtLeast(t, needsAttentionPayload, "recurrence_count", 2)

		if _, err := manager.ClearTaskBlock(
			ctx,
			attentionTask.ID,
			latestBlock.ID,
			"resolved final",
			operator,
		); err != nil {
			t.Fatalf("ClearTaskBlock(attention final) error = %v", err)
		}
		if _, err := manager.RecoverTask(ctx, attentionTask.ID, "operator reviewed escalation", operator); err != nil {
			t.Fatalf("RecoverTask(attention) error = %v", err)
		}
		recoveredEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    attentionTask.ID,
			EventType: eventspkg.TaskRecovered,
		})
		assertIntegrationEventCorrelation(
			t,
			recoveredEvent,
			attentionTask.ID,
			"",
			eventspkg.TaskRecovered,
			taskpkg.ActorKindHuman,
			"operator-observability",
			taskpkg.OriginKindCLI,
		)
		recoveredPayload := decodeIntegrationTaskEventPayload(t, recoveredEvent)
		assertIntegrationPayloadString(t, recoveredPayload, "status", string(taskpkg.TaskStatusReady))
		assertIntegrationPayloadString(t, recoveredPayload, "note", "operator reviewed escalation")

		clearAutoTask, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:              taskpkg.ScopeGlobal,
			Title:              "Coverage matrix block clear auto enqueue",
			AutoEnqueueOnReady: true,
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask(clear auto enqueue) error = %v", err)
		}
		clearAutoBlock, err := manager.BlockTask(ctx, taskpkg.BlockRequest{
			TaskID: clearAutoTask.ID,
			Kind:   taskpkg.BlockKindNeedsInput,
			Reason: "clear should requeue",
		}, operator)
		if err != nil {
			t.Fatalf("BlockTask(clear auto enqueue) error = %v", err)
		}
		clearAutoCreatedEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    clearAutoTask.ID,
			EventType: eventspkg.TaskBlockCreated,
		})
		assertIntegrationEventCorrelation(
			t,
			clearAutoCreatedEvent,
			clearAutoTask.ID,
			"",
			eventspkg.TaskBlockCreated,
			taskpkg.ActorKindHuman,
			"operator-observability",
			taskpkg.OriginKindCLI,
		)
		clearAutoCreatedPayload := decodeIntegrationTaskEventPayload(t, clearAutoCreatedEvent)
		assertIntegrationPayloadString(t, clearAutoCreatedPayload, "status", string(taskpkg.TaskStatusBlocked))
		assertIntegrationPayloadString(t, clearAutoCreatedPayload, "block_id", clearAutoBlock.ID)
		assertIntegrationPayloadString(t, clearAutoCreatedPayload, "block_kind", string(taskpkg.BlockKindNeedsInput))

		if _, err := manager.ClearTaskBlock(
			ctx,
			clearAutoTask.ID,
			clearAutoBlock.ID,
			"resolved clear auto",
			operator,
		); err != nil {
			t.Fatalf("ClearTaskBlock(clear auto enqueue) error = %v", err)
		}
		clearBlockEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    clearAutoTask.ID,
			EventType: eventspkg.TaskBlockCleared,
		})
		assertIntegrationEventCorrelation(
			t,
			clearBlockEvent,
			clearAutoTask.ID,
			"",
			eventspkg.TaskBlockCleared,
			taskpkg.ActorKindHuman,
			"operator-observability",
			taskpkg.OriginKindCLI,
		)
		clearBlockPayload := decodeIntegrationTaskEventPayload(t, clearBlockEvent)
		assertIntegrationPayloadString(t, clearBlockPayload, "status", string(taskpkg.TaskStatusReady))
		assertIntegrationPayloadString(t, clearBlockPayload, "block_id", clearAutoBlock.ID)
		assertIntegrationPayloadString(t, clearBlockPayload, "block_kind", string(taskpkg.BlockKindNeedsInput))
		assertIntegrationPayloadString(t, clearBlockPayload, "clear_note", "resolved clear auto")

		clearAutoEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    clearAutoTask.ID,
			EventType: eventspkg.TaskAutoEnqueueTriggered,
		})
		assertIntegrationEventCorrelation(
			t,
			clearAutoEvent,
			clearAutoTask.ID,
			clearAutoEvent.RunID,
			eventspkg.TaskAutoEnqueueTriggered,
			taskpkg.ActorKindHuman,
			"operator-observability",
			taskpkg.OriginKindCLI,
		)
		clearAutoPayload := decodeIntegrationTaskEventPayload(t, clearAutoEvent)
		assertIntegrationPayloadString(t, clearAutoPayload, "status", string(taskpkg.TaskStatusReady))
		assertIntegrationPayloadString(t, clearAutoPayload, "run_status", taskpkg.TaskRunStatusQueued.String())
		assertIntegrationPayloadString(t, clearAutoPayload, "trigger_kind", "block_clear")
		assertIntegrationPayloadString(t, clearAutoPayload, "trigger_ref", clearAutoBlock.ID)

		expiryTask, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:              taskpkg.ScopeGlobal,
			Title:              "Coverage matrix transient expiry",
			AutoEnqueueOnReady: true,
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask(expiry) error = %v", err)
		}
		expiringBlock, err := manager.BlockTask(ctx, taskpkg.BlockRequest{
			TaskID:    expiryTask.ID,
			Kind:      taskpkg.BlockKindTransient,
			Reason:    "temporary outage",
			ExpiresAt: now.Add(30 * time.Second),
		}, operator)
		if err != nil {
			t.Fatalf("BlockTask(expiry) error = %v", err)
		}
		daemon, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		if _, err := manager.ExpireTaskBlocks(ctx, now.Add(time.Minute), daemon); err != nil {
			t.Fatalf("ExpireTaskBlocks() error = %v", err)
		}
		expiredBlockEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    expiryTask.ID,
			EventType: eventspkg.TaskBlockExpired,
		})
		assertIntegrationEventCorrelation(
			t,
			expiredBlockEvent,
			expiryTask.ID,
			"",
			eventspkg.TaskBlockExpired,
			taskpkg.ActorKindDaemon,
			"scheduler",
			taskpkg.OriginKindDaemon,
		)
		expiredBlockPayload := decodeIntegrationTaskEventPayload(t, expiredBlockEvent)
		assertIntegrationPayloadString(t, expiredBlockPayload, "status", string(taskpkg.TaskStatusReady))
		assertIntegrationPayloadString(t, expiredBlockPayload, "block_id", expiringBlock.ID)
		assertIntegrationPayloadString(t, expiredBlockPayload, "block_kind", string(taskpkg.BlockKindTransient))

		expiryAutoEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    expiryTask.ID,
			EventType: eventspkg.TaskAutoEnqueueTriggered,
		})
		assertIntegrationEventCorrelation(
			t,
			expiryAutoEvent,
			expiryTask.ID,
			expiryAutoEvent.RunID,
			eventspkg.TaskAutoEnqueueTriggered,
			taskpkg.ActorKindDaemon,
			"scheduler",
			taskpkg.OriginKindDaemon,
		)
		expiryAutoPayload := decodeIntegrationTaskEventPayload(t, expiryAutoEvent)
		assertIntegrationPayloadString(t, expiryAutoPayload, "trigger_kind", "transient_expiry")
		assertIntegrationPayloadString(t, expiryAutoPayload, "trigger_ref", expiringBlock.ID)

		blockedTask, blockedRun, blockedClaim := createIntegrationClaimedRun(
			t,
			ctx,
			db,
			manager,
			operator,
			agent,
			workspaceID,
			"Coverage matrix blocked hallucination",
			"sess-observability-worker",
			now.Add(2*time.Minute),
		)
		_, err = manager.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			RunID:          blockedClaim.Run.ID,
			ClaimToken:     blockedClaim.ClaimToken,
			Result:         taskpkg.RunResult{Value: json.RawMessage(`{"summary":"claimed phantom child"}`)},
			CreatedTaskIDs: []string{"task-phantom-0001"},
			Now:            now.Add(3 * time.Minute),
		}, agent)
		if !errors.Is(err, taskpkg.ErrHallucinatedTaskRefs) {
			t.Fatalf(
				"CompleteRunLease(blocked hallucination) error = %v, want %v",
				err,
				taskpkg.ErrHallucinatedTaskRefs,
			)
		}
		blockedEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    blockedTask.ID,
			RunID:     blockedRun.ID,
			EventType: eventspkg.TaskCompletionHallucinationBlocked,
		})
		assertIntegrationEventCorrelation(
			t,
			blockedEvent,
			blockedTask.ID,
			blockedRun.ID,
			eventspkg.TaskCompletionHallucinationBlocked,
			taskpkg.ActorKindAgentSession,
			"sess-observability-worker",
			taskpkg.OriginKindAgentSession,
		)
		blockedPayload := decodeIntegrationTaskEventPayload(t, blockedEvent)
		assertIntegrationPayloadString(t, blockedPayload, "status", taskpkg.TaskRunStatusClaimed.String())
		assertIntegrationPayloadStringSlice(t, blockedPayload, "invalid_task_ids", []string{"task-phantom-0001"})
		assertIntegrationPayloadHasString(t, blockedPayload, "claim_token_hash")
		assertIntegrationPayloadOmitsRawValue(t, blockedEvent, blockedClaim.ClaimToken)

		advisoryAgent, err := taskpkg.DeriveAgentSessionActorContext(
			"sess-observability-advisory",
			workspaceID,
		)
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext(advisory) error = %v", err)
		}
		suspectedTask, suspectedRun, suspectedClaim := createIntegrationClaimedRun(
			t,
			ctx,
			db,
			manager,
			operator,
			advisoryAgent,
			workspaceID,
			"Coverage matrix suspected hallucination",
			"sess-observability-advisory",
			now.Add(4*time.Minute),
		)
		if _, err := manager.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			RunID:      suspectedClaim.Run.ID,
			ClaimToken: suspectedClaim.ClaimToken,
			Result: taskpkg.RunResult{
				Value: json.RawMessage(`{"summary":"I delegated follow-up to task-phantom-7777"}`),
			},
			Now: now.Add(5 * time.Minute),
		}, advisoryAgent); err != nil {
			t.Fatalf("CompleteRunLease(suspected hallucination) error = %v", err)
		}
		suspectedEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    suspectedTask.ID,
			RunID:     suspectedRun.ID,
			EventType: eventspkg.TaskCompletionHallucinationSuspected,
		})
		assertIntegrationEventCorrelation(
			t,
			suspectedEvent,
			suspectedTask.ID,
			suspectedRun.ID,
			eventspkg.TaskCompletionHallucinationSuspected,
			taskpkg.ActorKindAgentSession,
			"sess-observability-advisory",
			taskpkg.OriginKindAgentSession,
		)
		suspectedPayload := decodeIntegrationTaskEventPayload(t, suspectedEvent)
		assertIntegrationPayloadString(t, suspectedPayload, "status", taskpkg.TaskRunStatusCompleted.String())
		assertIntegrationPayloadStringSlice(t, suspectedPayload, "suspected_task_ids", []string{"task-phantom-7777"})
		assertIntegrationPayloadHasString(t, suspectedPayload, "claim_token_hash")
		assertIntegrationPayloadOmitsRawValue(t, suspectedEvent, suspectedClaim.ClaimToken)

		creator, err := taskpkg.DeriveAgentSessionActorContext("sess-task-creator", workspaceID)
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext(creator) error = %v", err)
		}
		wakeTask, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Coverage matrix wake delivered",
		}, creator)
		if err != nil {
			t.Fatalf("CreateTask(wake delivered) error = %v", err)
		}
		if _, err := manager.BlockTask(ctx, taskpkg.BlockRequest{
			TaskID: wakeTask.ID,
			Kind:   taskpkg.BlockKindNeedsInput,
			Reason: "creator clarification required",
		}, operator); err != nil {
			t.Fatalf("BlockTask(wake delivered) error = %v", err)
		}
		deliveredEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    wakeTask.ID,
			EventType: eventspkg.TaskWakeDelivered,
		})
		assertIntegrationEventCorrelation(
			t,
			deliveredEvent,
			wakeTask.ID,
			"",
			eventspkg.TaskWakeDelivered,
			taskpkg.ActorKindHuman,
			"operator-observability",
			taskpkg.OriginKindCLI,
		)
		deliveredPayload := decodeIntegrationTaskEventPayload(t, deliveredEvent)
		assertIntegrationPayloadString(t, deliveredPayload, "reason", string(taskpkg.WakeReasonBlocked))
		assertIntegrationPayloadString(t, deliveredPayload, "creator_session_id", "sess-task-creator")
		deliveredWakeEventID := assertIntegrationPayloadHasString(t, deliveredPayload, "wake_event_id")
		assertIntegrationPayloadHasString(t, deliveredPayload, "summary")
		deliveredWakeRecorded, err := db.TaskWakeEventExists(ctx, wakeTask.ID, deliveredWakeEventID)
		if err != nil {
			t.Fatalf("TaskWakeEventExists(delivered) error = %v", err)
		}
		if !deliveredWakeRecorded {
			t.Fatal("TaskWakeEventExists(delivered) = false, want true")
		}

		wakeDisabled := false
		suppressedTask, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope:       taskpkg.ScopeGlobal,
			Title:       "Coverage matrix wake suppressed",
			WakeCreator: &wakeDisabled,
		}, creator)
		if err != nil {
			t.Fatalf("CreateTask(wake suppressed) error = %v", err)
		}
		if _, err := manager.BlockTask(ctx, taskpkg.BlockRequest{
			TaskID: suppressedTask.ID,
			Kind:   taskpkg.BlockKindNeedsInput,
			Reason: "creator opted out",
		}, operator); err != nil {
			t.Fatalf("BlockTask(wake suppressed) error = %v", err)
		}
		suppressedEvent := findIntegrationTaskEvent(t, ctx, db, taskpkg.EventQuery{
			TaskID:    suppressedTask.ID,
			EventType: eventspkg.TaskWakeSuppressed,
		})
		assertIntegrationEventCorrelation(
			t,
			suppressedEvent,
			suppressedTask.ID,
			"",
			eventspkg.TaskWakeSuppressed,
			taskpkg.ActorKindHuman,
			"operator-observability",
			taskpkg.OriginKindCLI,
		)
		suppressedPayload := decodeIntegrationTaskEventPayload(t, suppressedEvent)
		assertIntegrationPayloadString(t, suppressedPayload, "reason", string(taskpkg.WakeReasonBlocked))
		assertIntegrationPayloadString(t, suppressedPayload, "creator_session_id", "sess-task-creator")
		assertIntegrationPayloadString(t, suppressedPayload, "suppression_reason", "wake_creator_disabled")
		suppressedWakeEventID := assertIntegrationPayloadHasString(t, suppressedPayload, "wake_event_id")
		suppressedWakeRecorded, err := db.TaskWakeEventExists(ctx, suppressedTask.ID, suppressedWakeEventID)
		if err != nil {
			t.Fatalf("TaskWakeEventExists(suppressed) error = %v", err)
		}
		if !suppressedWakeRecorded {
			t.Fatal("TaskWakeEventExists(suppressed) = false, want true")
		}
		crossTaskWakeRecorded, err := db.TaskWakeEventExists(ctx, suppressedTask.ID, deliveredWakeEventID)
		if err != nil {
			t.Fatalf("TaskWakeEventExists(cross-task) error = %v", err)
		}
		if crossTaskWakeRecorded {
			t.Fatal("TaskWakeEventExists(cross-task) = true, want false")
		}
	})
}

func TestTaskManagerNeedsAttentionDurableAcrossRestartIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should persist needs-attention state across restart", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		dbPath := filepath.Join(t.TempDir(), "agh.db")
		firstDB, err := globaldb.OpenGlobalDB(ctx, dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}
		firstManager := newTaskManagerIntegration(t, firstDB)
		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "agh task block")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		taskRecord, err := firstManager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Durable needs attention target",
		}, actor)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		if _, err := firstManager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, actor); err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		for idx := 0; idx < 3; idx++ {
			block, err := firstManager.BlockTask(ctx, taskpkg.BlockRequest{
				TaskID: taskRecord.ID,
				Kind:   taskpkg.BlockKindNeedsInput,
				Reason: fmt.Sprintf("restart loop %d", idx),
			}, actor)
			if err != nil {
				t.Fatalf("BlockTask(%d) error = %v", idx, err)
			}
			if idx < 2 {
				if _, err := firstManager.ClearTaskBlock(ctx, taskRecord.ID, block.ID, "resolved", actor); err != nil {
					t.Fatalf("ClearTaskBlock(%d) error = %v", idx, err)
				}
			}
		}
		if err := firstDB.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		secondDB, err := globaldb.OpenGlobalDB(ctx, dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(second) error = %v", err)
		}
		t.Cleanup(func() {
			if err := secondDB.Close(ctx); err != nil {
				t.Fatalf("Close(second) error = %v", err)
			}
		})
		secondManager := newTaskManagerIntegration(t, secondDB)
		reopened, err := secondDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(reopened) error = %v", err)
		}
		if got, want := reopened.Status, taskpkg.TaskStatusNeedsAttention; got != want {
			t.Fatalf("reopened Status = %q, want %q", got, want)
		}
		if reopened.NeedsAttention == nil {
			t.Fatal("reopened NeedsAttention = nil, want durable metadata")
		}
		agent, err := taskpkg.DeriveAgentSessionActorContext("sess-after-restart", "ws-test")
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
		}
		_, err = secondManager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "sess-after-restart",
			LeaseDuration:    time.Minute,
			Now:              time.Date(2026, 6, 3, 14, 0, 0, 0, time.UTC),
		}, agent)
		if !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(reopened needs_attention) error = %v, want ErrNoClaimableRun", err)
		}
	})
}

func TestTaskManagerSubprocessHealthEscalationIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should mark a running session-bound run once under concurrent escalation", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		manager := newTaskManagerIntegration(
			t,
			db,
			taskpkg.WithSessionExecutor(&integrationSessionExecutor{}),
		)
		operator, err := taskpkg.DeriveHumanActorContext(
			"operator-health",
			taskpkg.OriginKindCLI,
			"agh task run",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		escalator, err := taskpkg.DeriveDaemonActorContext(
			"subprocess_health",
			"daemon.subprocess_health",
		)
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}

		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Subprocess health escalation target",
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, operator)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		run, err = seedNonLeasedClaimedRunIntegration(ctx, manager, db, run.ID, operator)
		if err != nil {
			t.Fatalf("seedNonLeasedClaimedRunIntegration() error = %v", err)
		}
		run, err = manager.StartRun(ctx, run.ID, taskpkg.StartRun{}, operator)
		if err != nil {
			t.Fatalf("StartRun() error = %v", err)
		}
		if run.SessionID == "" {
			t.Fatal("StartRun().SessionID = empty, want subprocess correlation")
		}

		const callers = 2
		results := make(chan taskpkg.Run, callers)
		errs := make(chan error, callers)
		var wg sync.WaitGroup
		for range callers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				updated, markErr := manager.MarkRunNeedsAttention(
					ctx,
					run.ID,
					"subprocess health failed three consecutive checks",
					escalator,
				)
				results <- updated
				errs <- markErr
			}()
		}
		wg.Wait()
		close(results)
		close(errs)
		for markErr := range errs {
			if markErr != nil {
				t.Fatalf("MarkRunNeedsAttention() concurrent error = %v", markErr)
			}
		}
		for updated := range results {
			if updated.Status.Normalize() != taskpkg.TaskRunStatusNeedsAttention ||
				updated.SessionID != run.SessionID {
				t.Fatalf("MarkRunNeedsAttention() = %#v, want correlated needs_attention", updated)
			}
		}

		events, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{
			TaskID:    taskRecord.ID,
			RunID:     run.ID,
			EventType: eventspkg.TaskRunNeedsAttention,
		})
		if err != nil {
			t.Fatalf("ListTaskEvents(needs_attention) error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("needs_attention event count = %d, want %d", got, want)
		}
		event := events[0]
		assertIntegrationEventCorrelation(
			t,
			event,
			taskRecord.ID,
			run.ID,
			eventspkg.TaskRunNeedsAttention,
			taskpkg.ActorKindDaemon,
			"subprocess_health",
			taskpkg.OriginKindDaemon,
		)
		payload := decodeIntegrationTaskEventPayload(t, event)
		assertIntegrationPayloadString(t, payload, "session_id", run.SessionID)
		assertIntegrationPayloadString(t, payload, "previous_status", taskpkg.TaskRunStatusRunning.String())
		assertIntegrationPayloadString(t, payload, "status", taskpkg.TaskRunStatusNeedsAttention.String())
	})

	t.Run("Should preserve a terminal run when health escalation arrives late", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTaskManagerGlobalDB(t)
		manager := newTaskManagerIntegration(
			t,
			db,
			taskpkg.WithSessionExecutor(&integrationSessionExecutor{}),
		)
		operator, err := taskpkg.DeriveHumanActorContext(
			"operator-terminal",
			taskpkg.OriginKindCLI,
			"agh task run",
		)
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		escalator, err := taskpkg.DeriveDaemonActorContext(
			"subprocess_health",
			"daemon.subprocess_health",
		)
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}

		taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
			Scope: taskpkg.ScopeGlobal,
			Title: "Terminal subprocess health target",
		}, operator)
		if err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run, err := manager.EnqueueRun(ctx, taskpkg.EnqueueRun{TaskID: taskRecord.ID}, operator)
		if err != nil {
			t.Fatalf("EnqueueRun() error = %v", err)
		}
		run, err = seedNonLeasedClaimedRunIntegration(ctx, manager, db, run.ID, operator)
		if err != nil {
			t.Fatalf("seedNonLeasedClaimedRunIntegration() error = %v", err)
		}
		run, err = manager.StartRun(ctx, run.ID, taskpkg.StartRun{}, operator)
		if err != nil {
			t.Fatalf("StartRun() error = %v", err)
		}
		completed, err := manager.CompleteRun(ctx, run.ID, taskpkg.RunResult{
			Value: json.RawMessage(`{"ok":true}`),
		}, operator)
		if err != nil {
			t.Fatalf("CompleteRun() error = %v", err)
		}

		_, err = manager.MarkRunNeedsAttention(ctx, run.ID, "late subprocess health failure", escalator)
		if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			t.Fatalf("MarkRunNeedsAttention(terminal) error = %v, want ErrInvalidStatusTransition", err)
		}
		stored, err := db.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if stored.Status != completed.Status || stored.Status != taskpkg.TaskRunStatusCompleted {
			t.Fatalf("terminal status = %q, want completed", stored.Status)
		}
		events, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{
			TaskID:    taskRecord.ID,
			RunID:     run.ID,
			EventType: eventspkg.TaskRunNeedsAttention,
		})
		if err != nil {
			t.Fatalf("ListTaskEvents(needs_attention) error = %v", err)
		}
		if len(events) != 0 {
			t.Fatalf("terminal needs_attention event count = %d, want 0", len(events))
		}
	})
}
