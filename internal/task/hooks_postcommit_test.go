package task

import (
	"context"
	"errors"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func TestTaskRunPostCommitHookFailuresDoNotFailCommittedMutations(t *testing.T) {
	t.Parallel()

	t.Run("Should keep a claimed run successful when post-claim hook fails", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			postClaim: func(
				_ context.Context,
				payload hookspkg.TaskRunPostClaimPayload,
			) (hookspkg.TaskRunPostClaimPayload, error) {
				return payload, errors.New("post-claim observer failed")
			},
		}))
		actor := validActorContext()
		taskRecord, run := enqueueRunForPostCommitHookTest(t, manager, actor)

		claimed, err := claimExactRunForTest(context.Background(), manager, run.ID, actor)
		if err != nil {
			t.Fatalf("claimExactRunForTest() error = %v", err)
		}
		if got, want := claimed.Status, TaskRunStatusClaimed; got != want {
			t.Fatalf("claimed.Status = %q, want %q", got, want)
		}
		assertTaskRunEventExists(t, store, taskRecord.ID, run.ID, taskEventRunClaimed)
	})

	t.Run("Should keep a heartbeat successful when lease-extended hook fails", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			leaseExtended: func(
				_ context.Context,
				payload hookspkg.TaskRunLeaseExtendedPayload,
			) (hookspkg.TaskRunLeaseExtendedPayload, error) {
				return payload, errors.New("lease-extended observer failed")
			},
		}))
		actor := validActorContext()
		agent := validAgentActorContextForPostCommitHookTest()
		taskRecord, _ := enqueueRunForPostCommitHookTest(t, manager, actor)
		claimed := claimNextRunForPostCommitHookTest(t, manager, agent, time.Date(2026, 5, 16, 16, 0, 0, 0, time.UTC))

		heartbeat, err := manager.HeartbeatRunLease(context.Background(), LeaseHeartbeat{
			RunID:         claimed.Run.ID,
			ClaimToken:    claimed.ClaimToken,
			LeaseDuration: 2 * time.Minute,
			Now:           time.Date(2026, 5, 16, 16, 1, 0, 0, time.UTC),
		}, agent)
		if err != nil {
			t.Fatalf("HeartbeatRunLease() error = %v", err)
		}
		if got, want := heartbeat.Status, TaskRunStatusClaimed; got != want {
			t.Fatalf("heartbeat.Status = %q, want %q", got, want)
		}
		assertTaskRunEventExists(t, store, taskRecord.ID, claimed.Run.ID, taskEventRunLeaseExtended)
	})

	t.Run("Should keep completion successful when completed hook fails", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			completed: func(
				_ context.Context,
				payload hookspkg.TaskRunCompletedPayload,
			) (hookspkg.TaskRunCompletedPayload, error) {
				return payload, errors.New("completed observer failed")
			},
		}))
		actor := validActorContext()
		agent := validAgentActorContextForPostCommitHookTest()
		taskRecord, _ := enqueueRunForPostCommitHookTest(t, manager, actor)
		claimed := claimNextRunForPostCommitHookTest(t, manager, agent, time.Date(2026, 5, 16, 17, 0, 0, 0, time.UTC))

		completed, err := manager.CompleteRunLease(context.Background(), LeaseCompletion{
			RunID:      claimed.Run.ID,
			ClaimToken: claimed.ClaimToken,
			Result:     RunResult{},
			Now:        time.Date(2026, 5, 16, 17, 1, 0, 0, time.UTC),
		}, agent)
		if err != nil {
			t.Fatalf("CompleteRunLease() error = %v", err)
		}
		if got, want := completed.Status, TaskRunStatusCompleted; got != want {
			t.Fatalf("completed.Status = %q, want %q", got, want)
		}
		assertTaskRunEventExists(t, store, taskRecord.ID, claimed.Run.ID, taskWatchEventRunCompleted)
	})
}

func enqueueRunForPostCommitHookTest(
	t *testing.T,
	manager *Service,
	actor ActorContext,
) (*Task, *Run) {
	t.Helper()

	taskRecord, err := manager.CreateTask(context.Background(), CreateTask{
		Scope: ScopeGlobal,
		Title: "Post-commit hook task",
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	run, err := manager.EnqueueRun(context.Background(), EnqueueRun{TaskID: taskRecord.ID}, actor)
	if err != nil {
		t.Fatalf("EnqueueRun() error = %v", err)
	}
	return taskRecord, run
}

func claimNextRunForPostCommitHookTest(
	t *testing.T,
	manager *Service,
	agent ActorContext,
	now time.Time,
) *ClaimResult {
	t.Helper()

	claim, err := manager.ClaimNextRun(context.Background(), ClaimCriteria{
		Scope:            ScopeGlobal,
		ClaimerSessionID: "sess-post-commit-hooks",
		LeaseDuration:    2 * time.Minute,
		Now:              now,
	}, agent)
	if err != nil {
		t.Fatalf("ClaimNextRun() error = %v", err)
	}
	return claim
}

func validAgentActorContextForPostCommitHookTest() ActorContext {
	agent := validActorContext()
	agent.Actor = ActorIdentity{Kind: ActorKindAgentSession, Ref: "sess-post-commit-hooks"}
	agent.Origin = Origin{Kind: OriginKindAgentSession, Ref: "codex"}
	agent.Scope = CallerScope{SessionID: "sess-post-commit-hooks", WorkspaceID: "ws-post-commit-hooks"}
	return agent
}

func assertTaskRunEventExists(
	t *testing.T,
	store *inMemoryManagerStore,
	taskID string,
	runID string,
	eventType string,
) {
	t.Helper()

	events, err := store.ListTaskEvents(context.Background(), EventQuery{TaskID: taskID, RunID: runID})
	if err != nil {
		t.Fatalf("ListTaskEvents() error = %v", err)
	}
	if !containsEventType(events, eventType) {
		t.Fatalf("event types = %#v, want %q", sortedEventTypes(events), eventType)
	}
}
