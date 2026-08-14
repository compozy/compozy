package observe

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
)

// seedObserveRunSnapshot builds read-model fixtures through the same public,
// fenced lifecycle and audit boundaries as production.
func seedObserveRunSnapshot(t *testing.T, registry *globaldb.GlobalDB, target taskpkg.Run) {
	t.Helper()

	ctx := testutil.Context(t)
	actor, err := taskpkg.DeriveDaemonActorContext("observe-fixture", "daemon.observe-fixture")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	timeline := normalizeObserveRunTimeline(target)
	var current taskpkg.Run
	err = registry.WithTaskExecutionTransaction(ctx, func(store taskpkg.ExecutionMutationStore) error {
		_, reserved, existing, reserveErr := store.ReserveQueuedRun(ctx, taskpkg.QueueRunReservation{
			TaskID:             target.TaskID,
			RunID:              target.ID,
			RunKind:            target.RunKind,
			LoopRunID:          target.LoopRunID,
			IdempotencyKey:     target.IdempotencyKey,
			Origin:             target.Origin,
			NetworkSpec:        target.NetworkSpecSnapshot(),
			DesignationGroupID: target.DesignationGroupID,
			Metadata:           target.Metadata,
			QueuedAt:           timeline.queuedAt,
		})
		if reserveErr != nil {
			return reserveErr
		}
		if existing {
			return fmt.Errorf("%w: observe fixture run %q already exists", taskpkg.ErrValidation, target.ID)
		}
		current = reserved
		return nil
	})
	if err != nil {
		t.Fatalf("ReserveQueuedRun(%q) error = %v", target.ID, err)
	}

	targetStatus := target.Status.Normalize()
	if targetStatus == taskpkg.TaskRunStatusQueued {
		return
	}
	admissionManager, err := taskpkg.NewManager(
		taskpkg.WithStore(registry),
		taskpkg.WithManagerNow(func() time.Time { return timeline.claimedAt }),
	)
	if err != nil {
		t.Fatalf("NewManager(admission) error = %v", err)
	}
	if _, startErr := admissionManager.StartRun(ctx, current.ID, taskpkg.StartRun{
		IdempotencyKey: "observe-fixture-admit:" + current.ID,
	}, actor); startErr == nil ||
		!errors.Is(startErr, taskpkg.ErrValidation) {
		t.Fatalf("StartRun(admit only) error = %v, want missing session executor validation", startErr)
	}
	current, err = registry.GetTaskRun(ctx, current.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(admitted) error = %v", err)
	}
	if targetStatus == taskpkg.TaskRunStatusClaimed {
		return
	}

	sessionID := target.SessionID
	if sessionID == "" {
		sessionID = "observe-fixture-" + target.ID
	}
	fixtureNow := timeline.startedAt
	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(registry),
		taskpkg.WithSessionExecutor(observeFixtureSessionExecutor{
			sessionID: sessionID, workspaceID: target.WorkspaceID, worktreeID: target.WorktreeIDValue(),
		}),
		taskpkg.WithManagerNow(func() time.Time { return fixtureNow }),
	)
	if err != nil {
		t.Fatalf("NewManager(lifecycle) error = %v", err)
	}
	if targetStatus == taskpkg.TaskRunStatusStarting {
		if _, err := manager.AttachRunSession(ctx, current.ID, sessionID, actor); err != nil {
			t.Fatalf("AttachRunSession(%q) error = %v", target.ID, err)
		}
		return
	}
	currentRef, err := manager.StartRun(ctx, current.ID, taskpkg.StartRun{
		IdempotencyKey: "observe-fixture-start:" + current.ID,
	}, actor)
	if err != nil {
		t.Fatalf("StartRun(%q) error = %v", target.ID, err)
	}
	current = *currentRef
	if targetStatus == taskpkg.TaskRunStatusRunning {
		return
	}
	fixtureNow = timeline.endedAt
	switch targetStatus {
	case taskpkg.TaskRunStatusCompleted:
		completed, completeErr := manager.CompleteRun(ctx, current.ID, taskpkg.RunResult{
			Value:      target.Result,
			TokensUsed: target.TokensUsed,
		}, actor)
		if completeErr != nil {
			t.Fatalf("CompleteRun(%q) error = %v", target.ID, completeErr)
		}
		current = *completed
	case taskpkg.TaskRunStatusFailed:
		failure := target.Error
		if failure == "" {
			failure = "observe fixture failure"
		}
		failed, failErr := manager.FailRun(ctx, current.ID, taskpkg.RunFailure{Error: failure}, actor)
		if failErr != nil {
			t.Fatalf("FailRun(%q) error = %v", target.ID, failErr)
		}
		current = *failed
	default:
		t.Fatalf("unsupported observe fixture run status %q", target.Status)
	}
}

type observeFixtureSessionExecutor struct {
	sessionID   string
	workspaceID string
	worktreeID  string
}

func (e observeFixtureSessionExecutor) StartTaskSession(
	context.Context,
	*taskpkg.StartTaskSession,
) (*taskpkg.SessionRef, error) {
	return &taskpkg.SessionRef{
		SessionID: e.sessionID, WorkspaceID: e.workspaceID, WorktreeID: e.worktreeID,
	}, nil
}

func (e observeFixtureSessionExecutor) AttachTaskSession(
	context.Context,
	string,
	string,
) (*taskpkg.SessionRef, error) {
	return &taskpkg.SessionRef{
		SessionID: e.sessionID, WorkspaceID: e.workspaceID, WorktreeID: e.worktreeID,
	}, nil
}

func (observeFixtureSessionExecutor) RequestTaskStop(context.Context, string, taskpkg.StopReason) error {
	return nil
}

func (observeFixtureSessionExecutor) ForceTaskStop(context.Context, string, taskpkg.StopReason) error {
	return nil
}

type observeRunTimeline struct {
	queuedAt  time.Time
	claimedAt time.Time
	startedAt time.Time
	endedAt   time.Time
}

func normalizeObserveRunTimeline(run taskpkg.Run) observeRunTimeline {
	queuedAt := run.QueuedAt.UTC()
	claimedAt := run.ClaimedAt.UTC()
	startedAt := run.StartedAt.UTC()
	endedAt := run.EndedAt.UTC()
	terminal := run.Status.Normalize() == taskpkg.TaskRunStatusCompleted ||
		run.Status.Normalize() == taskpkg.TaskRunStatusFailed
	if terminal && !endedAt.IsZero() && (queuedAt.IsZero() || !queuedAt.Before(endedAt)) {
		queuedAt = endedAt.Add(-3 * time.Minute)
	}
	if queuedAt.IsZero() {
		queuedAt = time.Now().UTC().Add(-3 * time.Minute)
	}
	if claimedAt.IsZero() || claimedAt.Before(queuedAt) {
		claimedAt = queuedAt.Add(time.Minute)
	}
	if startedAt.IsZero() || startedAt.Before(claimedAt) {
		startedAt = claimedAt.Add(time.Minute)
	}
	if terminal && (endedAt.IsZero() || endedAt.Before(startedAt)) {
		endedAt = startedAt.Add(time.Minute)
	}
	return observeRunTimeline{
		queuedAt: queuedAt, claimedAt: claimedAt, startedAt: startedAt, endedAt: endedAt,
	}
}
