//go:build integration

package daemon

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
)

// seedDaemonTaskRunLifecycle creates an active integration fixture through the
// canonical reservation and fully fenced lifecycle boundaries. Loop-bound
// workers use a real lease claim; direct daemon runs use tokenless admission.
func seedDaemonTaskRunLifecycle(t *testing.T, store taskpkg.Store, target taskpkg.Run) taskpkg.Run {
	t.Helper()

	ctx := testutil.Context(t)
	taskRecord, err := store.GetTask(ctx, target.TaskID)
	if err != nil {
		t.Fatalf("GetTask(%q) error = %v", target.TaskID, err)
	}
	actor, err := taskpkg.DeriveDaemonActorContext("daemon-fixture", "daemon.integration-fixture")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	timeline := daemonRunFixtureTimeline(target)
	var current taskpkg.Run
	err = store.WithTaskExecutionTransaction(ctx, func(tx taskpkg.ExecutionMutationStore) error {
		_, reserved, existing, reserveErr := tx.ReserveQueuedRun(ctx, taskpkg.QueueRunReservation{
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
			return fmt.Errorf("%w: daemon fixture run %q already exists", taskpkg.ErrValidation, target.ID)
		}
		current = reserved
		return nil
	})
	if err != nil {
		t.Fatalf("ReserveQueuedRun(%q) error = %v", target.ID, err)
	}
	status := target.Status.Normalize()
	if status == taskpkg.TaskRunStatusQueued {
		return current
	}

	fixtureSessionID := target.SessionID
	if fixtureSessionID == "" {
		fixtureSessionID = "daemon-fixture-" + target.ID
	}
	var claimToken string
	if target.LoopRunID != "" {
		claimedBy := actor.Actor
		if target.ClaimedBy != nil {
			claimedBy = *target.ClaimedBy
		}
		err = store.WithLeaseSettlementTransaction(
			ctx,
			"seed daemon leased run",
			func(tx taskpkg.LeaseSettlementMutationStore) error {
				claim, claimErr := tx.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
					RunID:            current.ID,
					Scope:            taskRecord.Scope,
					WorkspaceID:      taskRecord.WorkspaceID,
					ClaimerSessionID: fixtureSessionID,
					ClaimedBy:        &claimedBy,
					LeaseDuration:    time.Hour,
					Now:              timeline.claimedAt,
				})
				current = claim.Run
				claimToken = claim.ClaimToken
				return claimErr
			},
		)
	} else {
		admissionManager, managerErr := taskpkg.NewManager(
			taskpkg.WithStore(store),
			taskpkg.WithManagerNow(func() time.Time { return timeline.claimedAt }),
		)
		if managerErr != nil {
			t.Fatalf("NewManager(admission) error = %v", managerErr)
		}
		_, startErr := admissionManager.StartRun(ctx, current.ID, taskpkg.StartRun{
			IdempotencyKey: "daemon-fixture-admit:" + current.ID,
		}, actor)
		if startErr == nil || !errors.Is(startErr, taskpkg.ErrValidation) {
			t.Fatalf(
				"StartRun(admit only %q) error = %v, want missing session executor validation",
				target.ID,
				startErr,
			)
		}
		current, err = store.GetTaskRun(ctx, current.ID)
	}
	if err != nil {
		t.Fatalf("seed claimed task run %q error = %v", target.ID, err)
	}
	if status == taskpkg.TaskRunStatusClaimed {
		return current
	}

	manager, err := taskpkg.NewManager(
		taskpkg.WithStore(store),
		taskpkg.WithSessionExecutor(daemonFixtureSessionExecutor{sessionID: fixtureSessionID}),
		taskpkg.WithManagerNow(func() time.Time { return timeline.startedAt }),
	)
	if err != nil {
		t.Fatalf("NewManager(lifecycle) error = %v", err)
	}
	if status == taskpkg.TaskRunStatusStarting {
		starting, attachErr := manager.AttachRunSession(ctx, current.ID, fixtureSessionID, actor)
		if attachErr != nil {
			t.Fatalf("AttachRunSession(%q) error = %v", target.ID, attachErr)
		}
		return *starting
	}
	if status != taskpkg.TaskRunStatusRunning {
		t.Fatalf("unsupported daemon fixture status %q", target.Status)
	}
	running, err := manager.StartRun(ctx, current.ID, taskpkg.StartRun{
		IdempotencyKey: "daemon-fixture-start:" + current.ID,
		ClaimToken:     claimToken,
	}, actor)
	if err != nil {
		t.Fatalf("StartRun(%q) error = %v", target.ID, err)
	}
	return *running
}

type daemonFixtureSessionExecutor struct {
	sessionID string
}

func (e daemonFixtureSessionExecutor) StartTaskSession(
	context.Context,
	*taskpkg.StartTaskSession,
) (*taskpkg.SessionRef, error) {
	return &taskpkg.SessionRef{SessionID: e.sessionID}, nil
}

func (e daemonFixtureSessionExecutor) AttachTaskSession(
	context.Context,
	string,
	string,
) (*taskpkg.SessionRef, error) {
	return &taskpkg.SessionRef{SessionID: e.sessionID}, nil
}

func (daemonFixtureSessionExecutor) RequestTaskStop(context.Context, string, taskpkg.StopReason) error {
	return nil
}

func (daemonFixtureSessionExecutor) ForceTaskStop(context.Context, string, taskpkg.StopReason) error {
	return nil
}

type daemonRunTimeline struct {
	queuedAt  time.Time
	claimedAt time.Time
	startedAt time.Time
}

func daemonRunFixtureTimeline(run taskpkg.Run) daemonRunTimeline {
	queuedAt := run.QueuedAt.UTC()
	if queuedAt.IsZero() {
		queuedAt = time.Now().UTC().Add(-2 * time.Minute)
	}
	claimedAt := run.ClaimedAt.UTC()
	if claimedAt.IsZero() || claimedAt.Before(queuedAt) {
		claimedAt = queuedAt.Add(time.Minute)
	}
	startedAt := run.StartedAt.UTC()
	if startedAt.IsZero() || startedAt.Before(claimedAt) {
		startedAt = claimedAt.Add(time.Minute)
	}
	return daemonRunTimeline{queuedAt: queuedAt, claimedAt: claimedAt, startedAt: startedAt}
}
