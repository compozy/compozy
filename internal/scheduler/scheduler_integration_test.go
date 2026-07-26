//go:build integration

package scheduler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
	"github.com/jonboulle/clockwork"
)

func TestSchedulerWakeLeavesClaimToTaskServiceIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should wake an eligible session without claiming the run for it", func(t *testing.T) {
		ctx := testutil.Context(t)
		base := time.Date(2026, 4, 26, 14, 0, 0, 0, time.UTC)
		db := openSchedulerGlobalDB(t, filepath.Join(t.TempDir(), "agh.db"))
		workspaceID := registerSchedulerWorkspace(t, db, "wake-claim", filepath.Join(t.TempDir(), "workspace"))
		manager := newSchedulerTaskManager(t, db)
		execution := createSchedulerTaskRun(t, ctx, manager, workspaceID, "Wake then claim")
		runChannel := execution.Run.NetworkSpecSnapshot().ChannelID
		if runChannel == "" {
			t.Fatal("execution.Run.NetworkSpecSnapshot().ChannelID = empty, want derived channel")
		}
		run := execution.Run
		run.RequiredCapabilities = []string{"go"}
		if err := db.UpdateTaskRun(ctx, run); err != nil {
			t.Fatalf("UpdateTaskRun(required capabilities) error = %v", err)
		}
		waker := &fakeWaker{}
		scheduler := newTestScheduler(
			t,
			integrationTaskSource{manager: manager, store: db},
			&fakeSessionSource{sessions: []SessionSnapshot{
				integrationSessionSnapshot(
					"sess-worker",
					workspaceID,
					runChannel,
					"active",
					false,
					[]string{"go", "sqlite"},
					base,
				),
			}},
			waker,
			WithClock(clockwork.NewFakeClockAt(base)),
		)

		before, err := db.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(before) error = %v", err)
		}
		if before.Status != taskpkg.TaskRunStatusQueued || before.SessionID != "" {
			t.Fatalf("before run = %#v, want queued and unowned", before)
		}

		result, err := scheduler.RunOnce(ctx)
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.WakeSucceeded != 1 {
			t.Fatalf("WakeSucceeded = %d, want 1 (result %#v)", result.WakeSucceeded, result)
		}
		targets := waker.targetsSnapshot()
		if got, want := len(targets), 1; got != want {
			t.Fatalf("wake targets = %d, want %d", got, want)
		}
		if got, want := targets[0].Work.Run.ID, run.ID; got != want {
			t.Fatalf("woken run = %q, want %q", got, want)
		}

		afterWake, err := db.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(after wake) error = %v", err)
		}
		if afterWake.Status != taskpkg.TaskRunStatusQueued || afterWake.SessionID != "" {
			t.Fatalf("after wake run = %#v, want scheduler to leave queued ownership untouched", afterWake)
		}

		claimActor, err := taskpkg.DeriveAgentSessionActorContext("sess-worker", workspaceID)
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
		}
		claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:                taskpkg.ScopeWorkspace,
			WorkspaceID:          workspaceID,
			ClaimerSessionID:     "sess-worker",
			ParticipationChannel: runChannel,
			RequiredCapabilities: []string{"go"},
			LeaseDuration:        time.Minute,
			Now:                  base.Add(time.Second),
		}, claimActor)
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		if got, want := claim.Run.ID, run.ID; got != want {
			t.Fatalf("ClaimNextRun().Run.ID = %q, want %q", got, want)
		}
	})
}

func TestSchedulerRecoversExpiredLeaseAfterDatabaseRestartIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should recover an expired lease after restart and make the run claimable again", func(t *testing.T) {
		ctx := testutil.Context(t)
		base := time.Date(2026, 4, 26, 15, 0, 0, 0, time.UTC)
		dbPath := filepath.Join(t.TempDir(), "agh.db")
		first := openSchedulerGlobalDB(t, dbPath)
		workspaceID := registerSchedulerWorkspace(t, first, "restart-recovery", filepath.Join(t.TempDir(), "workspace"))
		firstManager := newSchedulerTaskManager(t, first)
		execution := createSchedulerTaskRun(t, ctx, firstManager, workspaceID, "Restart recovery")
		runChannel := execution.Run.NetworkSpecSnapshot().ChannelID
		if runChannel == "" {
			t.Fatal("execution.Run.NetworkSpecSnapshot().ChannelID = empty, want derived channel")
		}

		oldActor, err := taskpkg.DeriveAgentSessionActorContext("sess-old", workspaceID)
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext(old) error = %v", err)
		}
		claimed, err := firstManager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:                taskpkg.ScopeWorkspace,
			WorkspaceID:          workspaceID,
			ClaimerSessionID:     "sess-old",
			ParticipationChannel: runChannel,
			LeaseDuration:        time.Second,
			Now:                  base,
		}, oldActor)
		if err != nil {
			t.Fatalf("ClaimNextRun(old) error = %v", err)
		}
		if got, want := claimed.Run.ID, execution.Run.ID; got != want {
			t.Fatalf("old claim run = %q, want %q", got, want)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("first Close() error = %v", err)
		}

		second := openSchedulerGlobalDB(t, dbPath)
		secondManager := newSchedulerTaskManager(t, second)
		waker := &fakeWaker{}
		scheduler := newTestScheduler(
			t,
			integrationTaskSource{manager: secondManager, store: second},
			&fakeSessionSource{sessions: []SessionSnapshot{
				integrationSessionSnapshot(
					"sess-new",
					workspaceID,
					runChannel,
					"active",
					false,
					nil,
					base.Add(2*time.Second),
				),
			}},
			waker,
			WithClock(clockwork.NewFakeClockAt(base.Add(2*time.Second))),
		)

		result, err := scheduler.RunOnce(ctx)
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.RecoveredLeases != 1 {
			t.Fatalf("RecoveredLeases = %d, want 1 (result %#v)", result.RecoveredLeases, result)
		}
		if !slices.Contains(result.RecoveredRunIDs, execution.Run.ID) {
			t.Fatalf("RecoveredRunIDs = %v, want %q", result.RecoveredRunIDs, execution.Run.ID)
		}
		if got := len(waker.targetsSnapshot()); got != 1 {
			t.Fatalf("wake targets after recovery = %d, want 1", got)
		}

		events, err := second.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: execution.Task.ID})
		if err != nil {
			t.Fatalf("ListTaskEvents() error = %v", err)
		}
		if !schedulerIntegrationHasEvent(events, "task.run_lease_expired") {
			t.Fatalf("event types = %v, want task.run_lease_expired", schedulerIntegrationEventTypes(events))
		}
		for _, event := range events {
			if strings.HasPrefix(event.EventType, "scheduler.") {
				t.Fatalf("unexpected scheduler hook-like event persisted: %#v", event)
			}
		}

		newActor, err := taskpkg.DeriveAgentSessionActorContext("sess-new", workspaceID)
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext(new) error = %v", err)
		}
		claim, err := secondManager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:                taskpkg.ScopeWorkspace,
			WorkspaceID:          workspaceID,
			ClaimerSessionID:     "sess-new",
			ParticipationChannel: runChannel,
			LeaseDuration:        time.Minute,
			Now:                  base.Add(3 * time.Second),
		}, newActor)
		if err != nil {
			t.Fatalf("ClaimNextRun(new) error = %v", err)
		}
		if got, want := claim.Run.ID, execution.Run.ID; got != want {
			t.Fatalf("new claim run = %q, want %q", got, want)
		}
	})
}

func TestSchedulerRecoversExpiredImmutableParticipationLeaseIntegration(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should recover an expired immutable participation lease and preserve reclaim semantics",
		func(t *testing.T) {
			ctx := testutil.Context(t)
			base := time.Date(2027, 4, 28, 9, 46, 36, 0, time.UTC)
			db := openSchedulerGlobalDB(t, filepath.Join(t.TempDir(), "agh.db"))
			workspaceID := registerSchedulerWorkspace(
				t,
				db,
				"participation-expiry",
				filepath.Join(t.TempDir(), "workspace"),
			)
			manager := newSchedulerTaskManagerWithOptions(
				t,
				db,
			)

			channelTimestamp := base.Add(-3 * time.Second)
			if err := db.WriteNetworkChannel(ctx, store.NetworkChannelEntry{
				Channel:     "scope-direct-history",
				WorkspaceID: workspaceID,
				Purpose:     "Immutable participation lease expiry recovery validation",
				CreatedBy:   "founder",
				CreatedAt:   channelTimestamp,
				UpdatedAt:   channelTimestamp,
			}); err != nil {
				t.Fatalf("WriteNetworkChannel() error = %v", err)
			}

			operator, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "agh task start")
			if err != nil {
				t.Fatalf("DeriveHumanActorContext() error = %v", err)
			}
			taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: workspaceID,
				Title:       "Immutable participation lease expiry recovery",
			}, operator)
			if err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			execution, err := manager.StartTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{
				NetworkParticipation: schedulerNamedParticipation("scope-direct-history"),
			}, operator)
			if err != nil {
				t.Fatalf("StartTask() error = %v", err)
			}
			if got, want := execution.Run.NetworkSpecSnapshot().ChannelID, "scope-direct-history"; got != want {
				t.Fatalf("execution.Run.NetworkSpecSnapshot().ChannelID = %q, want %q", got, want)
			}

			oldActor, err := taskpkg.DeriveAgentSessionActorContext("sess-old", workspaceID)
			if err != nil {
				t.Fatalf("DeriveAgentSessionActorContext(old) error = %v", err)
			}
			firstClaim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				Scope:                taskpkg.ScopeWorkspace,
				WorkspaceID:          workspaceID,
				ClaimerSessionID:     "sess-old",
				ParticipationChannel: "scope-direct-history",
				LeaseDuration:        time.Second,
				Now:                  base,
			}, oldActor)
			if err != nil {
				t.Fatalf("ClaimNextRun(old) error = %v", err)
			}
			if got, want := firstClaim.Run.ID, execution.Run.ID; got != want {
				t.Fatalf("firstClaim.Run.ID = %q, want %q", got, want)
			}
			if got, want := firstClaim.Run.NetworkSpecSnapshot().ChannelID, "scope-direct-history"; got != want {
				t.Fatalf("firstClaim.Run.NetworkSpecSnapshot().ChannelID = %q, want %q", got, want)
			}
			if firstClaim.ClaimToken == "" {
				t.Fatal("firstClaim.ClaimToken = empty, want raw claim token")
			}

			waker := &fakeWaker{}
			scheduler := newTestScheduler(
				t,
				integrationTaskSource{manager: manager, store: db},
				&fakeSessionSource{},
				waker,
				WithClock(clockwork.NewFakeClockAt(base.Add(12*time.Second))),
			)

			result, err := scheduler.RunOnce(ctx)
			if err != nil {
				t.Fatalf("RunOnce() error = %v", err)
			}
			if got, want := result.RecoveredLeases, 1; got != want {
				t.Fatalf("RecoveredLeases = %d, want %d (result %#v)", got, want, result)
			}
			if !slices.Contains(result.RecoveredRunIDs, execution.Run.ID) {
				t.Fatalf("RecoveredRunIDs = %v, want %q", result.RecoveredRunIDs, execution.Run.ID)
			}
			if got := len(waker.targetsSnapshot()); got != 0 {
				t.Fatalf("wake targets after participation recovery = %d, want 0", got)
			}

			if _, err := manager.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
				RunID:         execution.Run.ID,
				ClaimToken:    firstClaim.ClaimToken,
				LeaseDuration: time.Minute,
				Now:           base.Add(13 * time.Second),
			}, oldActor); !errors.Is(err, taskpkg.ErrInvalidClaimToken) {
				t.Fatalf(
					"HeartbeatRunLease(stale recovered lease) error = %v, want %v",
					err,
					taskpkg.ErrInvalidClaimToken,
				)
			}

			newActor, err := taskpkg.DeriveAgentSessionActorContext("sess-new", workspaceID)
			if err != nil {
				t.Fatalf("DeriveAgentSessionActorContext(new) error = %v", err)
			}
			secondClaim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				Scope:                taskpkg.ScopeWorkspace,
				WorkspaceID:          workspaceID,
				ClaimerSessionID:     "sess-new",
				ParticipationChannel: "scope-direct-history",
				LeaseDuration:        time.Minute,
				Now:                  base.Add(14 * time.Second),
			}, newActor)
			if err != nil {
				t.Fatalf("ClaimNextRun(new) error = %v", err)
			}
			if got, want := secondClaim.Run.ID, execution.Run.ID; got != want {
				t.Fatalf("secondClaim.Run.ID = %q, want %q", got, want)
			}
			if got, want := secondClaim.Run.SessionID, "sess-new"; got != want {
				t.Fatalf("secondClaim.Run.SessionID = %q, want %q", got, want)
			}
			if got, want := secondClaim.Run.NetworkSpecSnapshot().ChannelID, "scope-direct-history"; got != want {
				t.Fatalf("secondClaim.Run.NetworkSpecSnapshot().ChannelID = %q, want %q", got, want)
			}
			if secondClaim.ClaimToken == "" {
				t.Fatal("secondClaim.ClaimToken = empty, want raw claim token")
			}

			completed, err := manager.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
				RunID:      secondClaim.Run.ID,
				ClaimToken: secondClaim.ClaimToken,
				Result: taskpkg.RunResult{
					Value: []byte(`{"ok":true,"path":"scheduler-participation-expiry"}`),
				},
			}, newActor)
			if err != nil {
				t.Fatalf("CompleteRunLease() error = %v", err)
			}
			if got, want := completed.Status, taskpkg.TaskRunStatusCompleted; got != want {
				t.Fatalf("completed.Status = %q, want %q", got, want)
			}
			if got, want := completed.NetworkSpecSnapshot().ChannelID, "scope-direct-history"; got != want {
				t.Fatalf("completed.NetworkSpecSnapshot().ChannelID = %q, want %q", got, want)
			}

			storedTask, err := db.GetTask(ctx, taskRecord.ID)
			if err != nil {
				t.Fatalf("GetTask() error = %v", err)
			}
			if got, want := storedTask.Status, taskpkg.TaskStatusCompleted; got != want {
				t.Fatalf("storedTask.Status = %q, want %q", got, want)
			}

			events, err := db.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: taskRecord.ID})
			if err != nil {
				t.Fatalf("ListTaskEvents() error = %v", err)
			}
			eventCounts := map[string]int{}
			for _, event := range events {
				eventCounts[event.EventType]++
			}
			if got, want := eventCounts["task.run_claimed"], 2; got != want {
				t.Fatalf(
					"eventCounts[task.run_claimed] = %d, want %d (events=%#v)",
					got,
					want,
					schedulerIntegrationEventTypes(events),
				)
			}
			if got, want := eventCounts["task.run_lease_expired"], 1; got != want {
				t.Fatalf(
					"eventCounts[task.run_lease_expired] = %d, want %d (events=%#v)",
					got,
					want,
					schedulerIntegrationEventTypes(events),
				)
			}
			if got, want := eventCounts["task.run.completed"], 1; got != want {
				t.Fatalf(
					"eventCounts[task.run.completed] = %d, want %d (events=%#v)",
					got,
					want,
					schedulerIntegrationEventTypes(events),
				)
			}
		},
	)
}

func TestSchedulerRequeuesDeadWorkerLeaseAndWakesReplacementIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should release a dead worker lease before waking a replacement session", func(t *testing.T) {
		ctx := testutil.Context(t)
		base := time.Date(2026, 5, 19, 11, 0, 0, 0, time.UTC)
		db := openSchedulerGlobalDB(t, filepath.Join(t.TempDir(), "agh.db"))
		workspaceID := registerSchedulerWorkspace(t, db, "dead-worker", filepath.Join(t.TempDir(), "workspace"))
		manager := newSchedulerTaskManager(t, db)
		execution := createSchedulerTaskRun(t, ctx, manager, workspaceID, "Dead worker recovery")
		runChannel := execution.Run.NetworkSpecSnapshot().ChannelID
		if runChannel == "" {
			t.Fatal("execution.Run.NetworkSpecSnapshot().ChannelID = empty, want derived channel")
		}

		deadActor, err := taskpkg.DeriveAgentSessionActorContext("sess-dead-worker", workspaceID)
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext(dead) error = %v", err)
		}
		firstClaim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:                taskpkg.ScopeWorkspace,
			WorkspaceID:          workspaceID,
			ClaimerSessionID:     "sess-dead-worker",
			ParticipationChannel: runChannel,
			LeaseDuration:        time.Minute,
			Now:                  base,
		}, deadActor)
		if err != nil {
			t.Fatalf("ClaimNextRun(dead) error = %v", err)
		}
		if got, want := firstClaim.Run.ID, execution.Run.ID; got != want {
			t.Fatalf("firstClaim.Run.ID = %q, want %q", got, want)
		}

		daemonActor, err := taskpkg.DeriveDaemonActorContext("spawn-reaper", "daemon.spawn_reaper")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		released, err := manager.ReleaseSessionRunLeases(ctx, taskpkg.SessionLeaseRelease{
			SessionID: "sess-dead-worker",
			Reason:    "worker_died",
			Now:       base.Add(10 * time.Second),
		}, daemonActor)
		if err != nil {
			t.Fatalf("ReleaseSessionRunLeases() error = %v", err)
		}
		if got, want := len(released), 1; got != want {
			t.Fatalf("len(ReleaseSessionRunLeases()) = %d, want %d", got, want)
		}
		if got, want := released[0].Run.Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("released[0].Run.Status = %q, want %q", got, want)
		}
		if released[0].Run.SessionID != "" || released[0].Run.ClaimTokenHash != "" {
			t.Fatalf("released[0].Run = %#v, want queued and unowned", released[0].Run)
		}
		if released[0].PreviousClaimTokenHash == "" || released[0].PreviousSessionID != "sess-dead-worker" {
			t.Fatalf("released[0] = %#v, want previous dead-worker ownership snapshot", released[0])
		}
		if _, err := manager.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
			RunID:         firstClaim.Run.ID,
			ClaimToken:    firstClaim.ClaimToken,
			LeaseDuration: time.Minute,
			Now:           base.Add(11 * time.Second),
		}, deadActor); !errors.Is(err, taskpkg.ErrInvalidClaimToken) {
			t.Fatalf(
				"HeartbeatRunLease(dead token after release) error = %v, want %v",
				err,
				taskpkg.ErrInvalidClaimToken,
			)
		}

		waker := &fakeWaker{}
		scheduler := newTestScheduler(
			t,
			integrationTaskSource{manager: manager, store: db},
			&fakeSessionSource{sessions: []SessionSnapshot{
				integrationSessionSnapshot(
					"sess-replacement",
					workspaceID,
					runChannel,
					"active",
					false,
					nil,
					base.Add(12*time.Second),
				),
			}},
			waker,
			WithClock(clockwork.NewFakeClockAt(base.Add(12*time.Second))),
		)
		result, err := scheduler.RunOnce(ctx)
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.RecoveredLeases != 0 || result.WakeSucceeded != 1 {
			t.Fatalf("RunOnce() result = %#v, want no expired recovery and one wake", result)
		}
		targets := waker.targetsSnapshot()
		if got, want := len(targets), 1; got != want {
			t.Fatalf("wake targets = %d, want %d", got, want)
		}
		if got, want := targets[0].Session.ID, "sess-replacement"; got != want {
			t.Fatalf("wake target session = %q, want %q", got, want)
		}
		if got, want := targets[0].Work.Run.ID, execution.Run.ID; got != want {
			t.Fatalf("wake target run = %q, want %q", got, want)
		}

		replacementActor, err := taskpkg.DeriveAgentSessionActorContext("sess-replacement", workspaceID)
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext(replacement) error = %v", err)
		}
		secondClaim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:                taskpkg.ScopeWorkspace,
			WorkspaceID:          workspaceID,
			ClaimerSessionID:     "sess-replacement",
			ParticipationChannel: runChannel,
			LeaseDuration:        time.Minute,
			Now:                  base.Add(13 * time.Second),
		}, replacementActor)
		if err != nil {
			t.Fatalf("ClaimNextRun(replacement) error = %v", err)
		}
		if got, want := secondClaim.Run.ID, execution.Run.ID; got != want {
			t.Fatalf("secondClaim.Run.ID = %q, want %q", got, want)
		}
		if got, want := secondClaim.Run.SessionID, "sess-replacement"; got != want {
			t.Fatalf("secondClaim.Run.SessionID = %q, want %q", got, want)
		}
	})
}

func TestSchedulerHoldsSerialBacklogBehindCompatibleCapacityIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should wait through repeated busy cycles and wake the queued run after release", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		base := time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
		db := openSchedulerGlobalDB(t, filepath.Join(t.TempDir(), "agh.db"))
		workspaceID := registerSchedulerWorkspace(t, db, "serial-capacity", filepath.Join(t.TempDir(), "workspace"))
		manager := newSchedulerTaskManagerWithOptions(
			t,
			db,
			taskpkg.WithManagerNow(func() time.Time { return base }),
		)
		activeExecution := createSchedulerTaskRun(t, ctx, manager, workspaceID, "Active serial work")
		queuedExecution := createSchedulerTaskRun(t, ctx, manager, workspaceID, "Queued serial work")
		activeChannel := activeExecution.Run.NetworkSpecSnapshot().ChannelID
		queuedChannel := queuedExecution.Run.NetworkSpecSnapshot().ChannelID
		actor, err := taskpkg.DeriveAgentSessionActorContext("sess-serial", workspaceID)
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext() error = %v", err)
		}
		activeClaim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:                taskpkg.ScopeWorkspace,
			WorkspaceID:          workspaceID,
			ClaimerSessionID:     "sess-serial",
			ParticipationChannel: activeChannel,
			LeaseDuration:        time.Hour,
			Now:                  base.Add(time.Second),
		}, actor)
		if err != nil {
			t.Fatalf("ClaimNextRun(active) error = %v", err)
		}
		if got, want := activeClaim.Run.ID, activeExecution.Run.ID; got != want {
			t.Fatalf("active claim run = %q, want %q", got, want)
		}

		sessions := &fakeSessionSource{sessions: []SessionSnapshot{
			integrationSessionSnapshot(
				"sess-serial",
				workspaceID,
				queuedChannel,
				"active",
				false,
				nil,
				base,
			),
		}}
		waker := &fakeWaker{}
		escalator := &fakeEscalationActor{}
		scheduler := newTestScheduler(
			t,
			integrationTaskSource{manager: manager, store: db},
			sessions,
			waker,
			WithClock(clockwork.NewFakeClockAt(base.Add(10*time.Minute))),
			WithEscalationActor(escalator),
			WithStarvationStore(db),
			WithStarvationThresholds(StarvationThresholds{
				FanOutAfter:         1,
				SpawnAfter:          2,
				EventAfter:          3,
				NeedsAttentionAfter: 4,
				MinQueuedAge:        time.Second,
			}),
		)

		for cycle := 1; cycle <= 12; cycle++ {
			result, err := scheduler.RunOnce(ctx)
			if err != nil {
				t.Fatalf("RunOnce(busy cycle %d) error = %v", cycle, err)
			}
			if result.CapacityWaitingRuns != 1 || result.StarvedRuns != 0 || result.NoMatchRuns != 0 {
				t.Fatalf("busy cycle %d result = %#v, want one capacity wait only", cycle, result)
			}
		}
		if got := len(waker.targetsSnapshot()); got != 0 {
			t.Fatalf("wake targets while capacity busy = %d, want 0", got)
		}
		if len(escalator.spawns()) != 0 || len(escalator.emitted()) != 0 || len(escalator.attention()) != 0 {
			t.Fatalf("busy serial backlog produced convergence side effects: %#v", escalator)
		}
		if _, ok, err := db.LoadRunStarvation(ctx, queuedExecution.Run.ID); err != nil || ok {
			t.Fatalf("LoadRunStarvation(busy) = (ok %t, err %v), want (false, nil)", ok, err)
		}

		if _, err := manager.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			RunID:      activeClaim.Run.ID,
			ClaimToken: activeClaim.ClaimToken,
			Result:     taskpkg.RunResult{Value: []byte(`{"serial":true}`)},
		}, actor); err != nil {
			t.Fatalf("CompleteRunLease(active) error = %v", err)
		}
		released, err := scheduler.RunOnce(ctx)
		if err != nil {
			t.Fatalf("RunOnce(after release) error = %v", err)
		}
		if released.CapacityWaitingRuns != 0 || released.WakeSucceeded != 1 {
			t.Fatalf("after-release result = %#v, want one wake and no capacity wait", released)
		}
		targets := waker.targetsSnapshot()
		if got, want := targets[len(targets)-1].Work.Run.ID, queuedExecution.Run.ID; got != want {
			t.Fatalf("after-release wake run = %q, want %q", got, want)
		}
		claim, err := manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:                taskpkg.ScopeWorkspace,
			WorkspaceID:          workspaceID,
			ClaimerSessionID:     "sess-serial",
			ParticipationChannel: queuedChannel,
			LeaseDuration:        time.Hour,
			Now:                  base.Add(11 * time.Minute),
		}, actor)
		if err != nil {
			t.Fatalf("ClaimNextRun(queued) error = %v", err)
		}
		if got, want := claim.Run.ID, queuedExecution.Run.ID; got != want {
			t.Fatalf("queued claim run = %q, want %q", got, want)
		}
	})
}

func TestSchedulerNoEligibleSessionDoesNotClaimIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should leave a queued run untouched when no eligible session exists", func(t *testing.T) {
		ctx := testutil.Context(t)
		base := time.Date(2026, 4, 26, 16, 0, 0, 0, time.UTC)
		db := openSchedulerGlobalDB(t, filepath.Join(t.TempDir(), "agh.db"))
		workspaceID := registerSchedulerWorkspace(t, db, "no-eligible", filepath.Join(t.TempDir(), "workspace"))
		manager := newSchedulerTaskManager(t, db)
		execution := createSchedulerTaskRun(t, ctx, manager, workspaceID, "No eligible")
		waker := &fakeWaker{}
		scheduler := newTestScheduler(
			t,
			integrationTaskSource{manager: manager, store: db},
			&fakeSessionSource{sessions: []SessionSnapshot{
				sessionSnapshot("sess-other", "ws-other", "active", false, nil, base),
			}},
			waker,
			WithClock(clockwork.NewFakeClockAt(base)),
		)

		result, err := scheduler.RunOnce(ctx)
		if err != nil {
			t.Fatalf("RunOnce() error = %v", err)
		}
		if result.NoMatchRuns != 1 || result.WakeAttempts != 0 {
			t.Fatalf("scheduler result = %#v, want one no-match and no wake attempts", result)
		}
		if got := len(waker.targetsSnapshot()); got != 0 {
			t.Fatalf("wake targets = %d, want 0", got)
		}
		stored, err := db.GetTaskRun(ctx, execution.Run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if stored.Status != taskpkg.TaskRunStatusQueued || stored.SessionID != "" || stored.ClaimTokenHash != "" {
			t.Fatalf("stored run = %#v, want queued with no owner or claim token", stored)
		}

		otherActor, err := taskpkg.DeriveAgentSessionActorContext("sess-other", "ws-other")
		if err != nil {
			t.Fatalf("DeriveAgentSessionActorContext(other) error = %v", err)
		}
		_, err = manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeWorkspace,
			WorkspaceID:      "ws-other",
			ClaimerSessionID: "sess-other",
			Now:              base.Add(time.Second),
		}, otherActor)
		if !errors.Is(err, taskpkg.ErrNoClaimableRun) {
			t.Fatalf("ClaimNextRun(wrong workspace) error = %v, want ErrNoClaimableRun", err)
		}
	})
}

type integrationTaskSource struct {
	manager *taskpkg.Service
	store   taskpkg.Store
}

func (s integrationTaskSource) PendingRuns(ctx context.Context) ([]RunSnapshot, error) {
	runs, err := s.store.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued})
	if err != nil {
		return nil, err
	}
	return s.joinRuns(ctx, runs)
}

func (s integrationTaskSource) ActiveRuns(ctx context.Context) ([]taskpkg.Run, error) {
	return s.store.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{
		taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning,
	})
}

func (s integrationTaskSource) GetRunStatus(
	ctx context.Context,
	runID string,
) (taskpkg.RunStatus, bool, error) {
	run, err := s.store.GetTaskRun(ctx, runID)
	if err != nil {
		if errors.Is(err, taskpkg.ErrTaskRunNotFound) {
			return taskpkg.TaskRunStatusUnknown, false, nil
		}
		return taskpkg.TaskRunStatusUnknown, false, err
	}
	return run.Status, true, nil
}

func (s integrationTaskSource) RecoverExpiredRunLeases(
	ctx context.Context,
	recovery taskpkg.ExpiredLeaseRecovery,
	actor taskpkg.ActorContext,
) ([]taskpkg.ExpiredLeaseRecoveryResult, error) {
	return s.manager.RecoverExpiredRunLeases(ctx, recovery, actor)
}

func (s integrationTaskSource) ExpireTaskBlocks(
	ctx context.Context,
	now time.Time,
	actor taskpkg.ActorContext,
) (taskpkg.ExpireTaskBlocksResult, error) {
	return s.manager.ExpireTaskBlocks(ctx, now, actor)
}

func (s integrationTaskSource) joinRuns(ctx context.Context, runs []taskpkg.Run) ([]RunSnapshot, error) {
	work := make([]RunSnapshot, 0, len(runs))
	for _, run := range runs {
		taskRecord, err := s.store.GetTask(ctx, run.TaskID)
		if err != nil {
			return nil, err
		}
		work = append(work, RunSnapshot{Task: taskRecord, Run: run})
	}
	return work, nil
}

func openSchedulerGlobalDB(t *testing.T, path string) *globaldb.GlobalDB {
	t.Helper()

	db, err := globaldb.OpenGlobalDB(testutil.Context(t), path)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return db
}

func newSchedulerTaskManager(t *testing.T, store taskpkg.Store) *taskpkg.Service {
	t.Helper()
	return newSchedulerTaskManagerWithOptions(t, store)
}

func newSchedulerTaskManagerWithOptions(t *testing.T, store taskpkg.Store, opts ...taskpkg.Option) *taskpkg.Service {
	t.Helper()

	managerOptions := []taskpkg.Option{
		taskpkg.WithStore(store),
		taskpkg.WithParticipationResolver(schedulerParticipationResolver{}),
	}
	managerOptions = append(managerOptions, opts...)
	manager, err := taskpkg.NewManager(managerOptions...)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	return manager
}

func integrationSessionSnapshot(
	id string,
	workspaceID string,
	channel string,
	state string,
	prompting bool,
	capabilities []string,
	createdAt time.Time,
) SessionSnapshot {
	return SessionSnapshot{
		ID:              id,
		WorkspaceID:     workspaceID,
		Channel:         channel,
		State:           state,
		Prompting:       prompting,
		Capabilities:    append([]string(nil), capabilities...),
		CapabilityState: CapabilityStateKnown,
		CreatedAt:       createdAt,
	}
}

func registerSchedulerWorkspace(t *testing.T, db *globaldb.GlobalDB, name string, rootDir string) string {
	t.Helper()

	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", rootDir, err)
	}
	workspace := aghworkspace.Workspace{
		ID:        "ws-" + strings.ReplaceAll(name, " ", "-"),
		RootDir:   rootDir,
		Name:      name,
		CreatedAt: time.Date(2026, 4, 26, 8, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 26, 8, 0, 0, 0, time.UTC),
	}
	if err := db.InsertWorkspace(testutil.Context(t), workspace); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	return workspace.ID
}

func createSchedulerTaskRun(
	t *testing.T,
	ctx context.Context,
	manager *taskpkg.Service,
	workspaceID string,
	title string,
) *taskpkg.Execution {
	t.Helper()

	actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "agh task start")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	taskRecord, err := manager.CreateTask(ctx, taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: workspaceID,
		Title:       title,
	}, actor)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	execution, err := manager.StartTask(ctx, taskRecord.ID, taskpkg.ExecutionRequest{
		NetworkParticipation: schedulerNamedParticipation("scheduler-" + taskRecord.ID),
	}, actor)
	if err != nil {
		t.Fatalf("StartTask() error = %v", err)
	}
	return execution
}

type schedulerParticipationResolver struct{}

func (schedulerParticipationResolver) Resolve(
	_ context.Context,
	input participation.ResolveInput,
) (participation.Spec, error) {
	if input.Request == nil || input.Request.Mode == nil || *input.Request.Mode == participation.ModeLocal {
		return participation.LocalSpec(), nil
	}
	bounds, err := participation.ResolveBounds(
		input.Request.Bounds,
		schedulerParticipationDefaults(),
		participation.Limits{},
	)
	if err != nil {
		return participation.Spec{}, err
	}
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     input.WorkspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       strings.TrimSpace(*input.Request.ChannelID),
		Source:          participation.SourceExplicitRequest,
		Bounds:          bounds,
	}, nil
}

func schedulerParticipationDefaults() participation.Bounds {
	return participation.Bounds{
		MaxWakes:         4,
		MaxWakeWallTime:  "30s",
		MaxTotalWallTime: "2m",
		MaxInputTokens:   4096,
		MaxOutputTokens:  4096,
		MaxWakeDepth:     4,
		CoalesceWindow:   "250ms",
	}
}

func schedulerNamedParticipation(channel string) *participation.Request {
	mode := participation.ModeLive
	strategy := participation.StrategyNamed
	channel = strings.TrimSpace(channel)
	return &participation.Request{
		Mode:            &mode,
		ChannelStrategy: &strategy,
		ChannelID:       &channel,
	}
}

func schedulerIntegrationHasEvent(events []taskpkg.Event, want string) bool {
	for _, event := range events {
		if event.EventType == want {
			return true
		}
	}
	return false
}

func schedulerIntegrationEventTypes(events []taskpkg.Event) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.EventType)
	}
	slices.Sort(types)
	return types
}
