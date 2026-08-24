package task

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	storepkg "github.com/compozy/compozy/internal/store"
)

func TestTaskRunPostCommitHookFailuresDoNotFailCommittedMutations(t *testing.T) {
	t.Parallel()

	t.Run("Should dispatch a tokenless direct claim only after its audit commits", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		executor := &recordingSessionExecutor{}
		postClaimCalls := 0
		manager := newTaskManagerForTestWithOptions(
			t,
			store,
			WithSessionExecutor(executor),
			WithTaskRunHooks(recordingTaskRunHooks{
				postClaim: func(
					_ context.Context,
					payload hookspkg.TaskRunPostClaimPayload,
				) (hookspkg.TaskRunPostClaimPayload, error) {
					postClaimCalls++
					if got, want := payload.RunStatus, TaskRunStatusClaimed.String(); got != want {
						t.Fatalf("post-claim RunStatus = %q, want %q", got, want)
					}
					if !payload.LeaseUntil.IsZero() {
						t.Fatalf("post-claim LeaseUntil = %s, want zero for direct execution", payload.LeaseUntil)
					}
					encoded, err := json.Marshal(payload)
					if err != nil {
						t.Fatalf("json.Marshal(post-claim payload) error = %v", err)
					}
					if strings.Contains(string(encoded), "claim_token") {
						t.Fatalf("direct post-claim hook exposes claim-token material: %s", encoded)
					}
					assertTaskRunEventExists(t, store, payload.TaskID, payload.RunID, taskEventRunClaimed)
					return payload, nil
				},
			}),
		)
		actor := validActorContext()
		_, run := enqueueRunForPostCommitHookTest(t, manager, actor)

		started, err := manager.StartRun(context.Background(), run.ID, StartRun{}, actor)
		if err != nil {
			t.Fatalf("StartRun(direct) error = %v", err)
		}
		if got, want := started.Status.Normalize(), TaskRunStatusRunning; got != want {
			t.Fatalf("started.Status = %q, want %q", got, want)
		}
		if got, want := postClaimCalls, 1; got != want {
			t.Fatalf("post-claim hook calls = %d, want %d", got, want)
		}
		stored := store.runs[run.ID]
		if stored.ClaimTokenHash != "" || !stored.LeaseUntil.IsZero() || !stored.HeartbeatAt.IsZero() {
			t.Fatalf("stored direct execution run = %#v, want tokenless claim fence", stored)
		}
	})

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

	t.Run("Should keep a release successful when released hook fails", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			released: func(
				_ context.Context,
				payload hookspkg.TaskRunReleasedPayload,
			) (hookspkg.TaskRunReleasedPayload, error) {
				return payload, errors.New("released observer failed")
			},
		}))
		actor := validActorContext()
		agent := validAgentActorContextForPostCommitHookTest()
		taskRecord, _ := enqueueRunForPostCommitHookTest(t, manager, actor)
		claimed := claimNextRunForPostCommitHookTest(
			t,
			manager,
			agent,
			time.Date(2026, 5, 16, 16, 30, 0, 0, time.UTC),
		)

		released, err := manager.ReleaseRunLease(context.Background(), LeaseRelease{
			RunID:      claimed.Run.ID,
			ClaimToken: claimed.ClaimToken,
			Reason:     "handoff",
			Now:        time.Date(2026, 5, 16, 16, 31, 0, 0, time.UTC),
		}, agent)
		if err != nil {
			t.Fatalf("ReleaseRunLease() error = %v", err)
		}
		if got, want := released.Status, TaskRunStatusQueued; got != want {
			t.Fatalf("released.Status = %q, want %q", got, want)
		}
		assertTaskRunEventExists(t, store, taskRecord.ID, claimed.Run.ID, taskEventRunReleased)
	})

	t.Run("Should keep a failure successful when failed hook fails", func(t *testing.T) {
		t.Parallel()

		store := newInMemoryManagerStore()
		manager := newTaskManagerForTestWithOptions(t, store, WithTaskRunHooks(recordingTaskRunHooks{
			failed: func(
				_ context.Context,
				payload hookspkg.TaskRunFailedPayload,
			) (hookspkg.TaskRunFailedPayload, error) {
				return payload, errors.New("failed observer failed")
			},
		}))
		actor := validActorContext()
		agent := validAgentActorContextForPostCommitHookTest()
		taskRecord, _ := enqueueRunForPostCommitHookTest(t, manager, actor)
		claimed := claimNextRunForPostCommitHookTest(
			t,
			manager,
			agent,
			time.Date(2026, 5, 16, 16, 45, 0, 0, time.UTC),
		)

		failed, err := manager.FailRunLease(context.Background(), LeaseFailure{
			RunID:      claimed.Run.ID,
			ClaimToken: claimed.ClaimToken,
			Failure:    RunFailure{Error: "worker failed"},
			Now:        time.Date(2026, 5, 16, 16, 46, 0, 0, time.UTC),
		}, agent)
		if err != nil {
			t.Fatalf("FailRunLease() error = %v", err)
		}
		if got, want := failed.Status, TaskRunStatusFailed; got != want {
			t.Fatalf("failed.Status = %q, want %q", got, want)
		}
		assertTaskRunEventExists(t, store, taskRecord.ID, claimed.Run.ID, taskWatchEventRunFailed)
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
		ProfileID: storepkg.DefaultProfileID,
		Scope:     ScopeGlobal,
		Title:     "Post-commit hook task",
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
