//go:build integration

package automation

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
	"github.com/jonboulle/clockwork"
)

func TestSchedulerIntegrationFastScheduleDispatchesThroughDispatcher(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openAutomationIntegrationDB(t, ctx)
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, db)
	scheduler := newTestScheduler(t, dispatcher)

	job, err := db.CreateJob(ctx, testJob(AutomationScopeGlobal, "integration-schedule", ""))
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	job.Schedule = &ScheduleSpec{
		Mode:     ScheduleModeEvery,
		Interval: "1s",
	}

	if _, err := scheduler.Register(ctx, job); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	waitUntil(t, 4*time.Second, 25*time.Millisecond, func() bool {
		runs, err := db.ListRuns(ctx, RunQuery{JobID: job.ID})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		return len(runs) > 0 && runs[0].Status == RunCompleted
	})

	runs, err := db.ListRuns(ctx, RunQuery{JobID: job.ID})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("ListRuns() = 0 runs, want at least one")
	}
	if got, want := runs[0].Status, RunCompleted; got != want {
		t.Fatalf("runs[0].Status = %q, want %q", got, want)
	}
	if got := len(creator.createCalls()); got == 0 {
		t.Fatal("Create() call count = 0, want at least one")
	}
}

func TestSchedulerIntegrationShutdownCancelsInflightDispatch(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openAutomationIntegrationDB(t, ctx)

	promptStarted := make(chan struct{}, 1)
	promptRelease := make(chan struct{})
	creator := newRecordingSessionCreator(sessionAttemptPlan{
		promptStarted: promptStarted,
		promptRelease: promptRelease,
	})
	dispatcher := newTestDispatcher(t, creator, db)
	scheduler := newTestScheduler(t, dispatcher)

	job, err := db.CreateJob(ctx, testJob(AutomationScopeGlobal, "integration-shutdown", ""))
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	job.Schedule = &ScheduleSpec{
		Mode:     ScheduleModeEvery,
		Interval: "1s",
	}

	if _, err := scheduler.Register(ctx, job); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	select {
	case <-promptStarted:
	case <-time.After(4 * time.Second):
		t.Fatal("scheduled dispatch did not reach Prompt() in time")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := scheduler.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	waitUntil(t, 4*time.Second, 25*time.Millisecond, func() bool {
		runs, err := db.ListRuns(ctx, RunQuery{JobID: job.ID})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		return len(runs) > 0 && runs[0].Status == RunCancelled
	})

	runs, err := db.ListRuns(ctx, RunQuery{JobID: job.ID})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("ListRuns() = 0 runs, want at least one")
	}
	if got, want := runs[0].Status, RunCancelled; got != want {
		t.Fatalf("runs[0].Status = %q, want %q", got, want)
	}
	if runs[0].StartedAt == nil {
		t.Fatal("runs[0].StartedAt = nil, want populated")
	}
	if runs[0].EndedAt == nil {
		t.Fatal("runs[0].EndedAt = nil, want populated")
	}
}

func TestSchedulerIntegrationRestartDowntimeRunsCatchUpExactlyOnce(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
	db, err := globaldb.OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(context.Background()); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	})

	baseTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	missedAt := baseTime.Add(time.Minute)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	job, err := db.CreateJob(ctx, testJob(AutomationScopeGlobal, "restart-catch-up", ""))
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	job.Schedule = &ScheduleSpec{
		Mode:                ScheduleModeEvery,
		Interval:            "1m",
		CatchUpPolicy:       SchedulerCatchUpPolicyRunOnce,
		MisfireGraceSeconds: 10 * 60,
	}
	job, err = db.UpdateJob(ctx, job)
	if err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}
	if _, err := db.SaveSchedulerState(ctx, SchedulerState{
		JobID:               job.ID,
		NextRunAt:           &missedAt,
		ScheduleHash:        scheduleHash(job.Schedule),
		CatchUpPolicy:       SchedulerCatchUpPolicyRunOnce,
		MisfireGraceSeconds: 10 * 60,
		UpdatedAt:           baseTime,
	}); err != nil {
		t.Fatalf("SaveSchedulerState() error = %v", err)
	}

	fakeClock.Advance(5 * time.Minute)
	dispatcher := newStubScheduleDispatcher()
	scheduler := newTestScheduler(
		t,
		dispatcher,
		WithSchedulerClock(fakeClock),
		WithSchedulerStore(db),
	)
	if _, err := scheduler.Register(ctx, job); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	dispatcher.waitForDispatchCount(t, 1, 2*time.Second)
	dispatcher.waitForCompletionCount(t, 1, 2*time.Second)
	dispatcher.assertDispatchCount(t, 1)

	runs, err := db.ListRuns(ctx, RunQuery{JobID: job.ID})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(ListRuns()) = %d, want %d", got, want)
	}
}

func waitUntil(t *testing.T, timeout time.Duration, interval time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if fn() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("condition not met within %s", timeout)
		case <-ticker.C:
		}
	}
}
