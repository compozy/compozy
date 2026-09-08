//go:build integration

package automation

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
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
		runs, err := db.ListRuns(ctx, RunQuery{
			ReadScope: store.ReadScope{ProfileID: job.ProfileID},
			JobID:     job.ID,
		})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		return len(runs) > 0 && runs[0].Status == RunCompleted
	})

	runs, err := db.ListRuns(ctx, RunQuery{
		ReadScope: store.ReadScope{ProfileID: job.ProfileID},
		JobID:     job.ID,
	})
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
		runs, err := db.ListRuns(ctx, RunQuery{
			ReadScope: store.ReadScope{ProfileID: job.ProfileID},
			JobID:     job.ID,
		})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		return len(runs) > 0 && runs[0].Status == RunCancelled
	})

	runs, err := db.ListRuns(ctx, RunQuery{
		ReadScope: store.ReadScope{ProfileID: job.ProfileID},
		JobID:     job.ID,
	})
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

	runs, err := db.ListRuns(ctx, RunQuery{
		ReadScope: store.ReadScope{ProfileID: job.ProfileID},
		JobID:     job.ID,
	})
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

// Invariant: a busy gate retains the original fire across restart or retires it
// on schedule replacement. Owner: durable automation scheduler integration suite.
func TestSchedulerIntegrationDeferredFire(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{
		"one-shot restart", "recurring restart", "schedule replacement", "past replacement", "gate release",
		"draining restart", "cancelled restart", "deadline restart", "recurring draining restart",
	} {
		t.Run("Should preserve fire ownership on "+scenario, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.Context(t)
			path := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
			db, err := globaldb.OpenGlobalDB(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			base := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
			clock := clockwork.NewFakeClockAt(base)
			started, release := make(chan struct{}, 1), make(chan struct{})
			admissionErr := map[string]error{
				"draining restart": admission.ErrDraining, "cancelled restart": context.Canceled,
				"deadline restart": context.DeadlineExceeded, "recurring draining restart": admission.ErrDraining,
			}[scenario]
			creator := newRecordingSessionCreator(
				sessionAttemptPlan{promptStarted: started, promptRelease: release},
				sessionAttemptPlan{createErr: admissionErr},
			)
			dispatcher := newTestDispatcher(
				t,
				creator,
				db,
				WithDispatcherMaxConcurrent(1),
				WithDispatcherNow(clock.Now),
			)
			blocker, err := db.CreateJob(ctx, testJob(AutomationScopeGlobal, "capacity-holder", ""))
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				_, err := dispatcher.Dispatch(ctx, DispatchRequest{Kind: DispatchKindSchedule, Job: &blocker})
				done <- err
			}()
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			job := testJob(AutomationScopeGlobal, "deferred-job", "")
			fireAt := base.Add(time.Minute)
			job.Schedule = &ScheduleSpec{Mode: ScheduleModeAt, Time: fireAt.Format(time.RFC3339)}
			if scenario == "recurring restart" || scenario == "recurring draining restart" {
				job.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1m"}
			}
			job, err = db.CreateJob(ctx, job)
			if err != nil {
				t.Fatal(err)
			}
			scheduler := newTestScheduler(t, dispatcher, WithSchedulerStore(db), WithSchedulerClock(clock))
			if _, err := scheduler.Register(ctx, job); err != nil {
				t.Fatal(err)
			}
			clock.Advance(time.Minute)
			if err := scheduler.executeScheduledJob(ctx, job.ID); err != nil {
				t.Fatal(err)
			}
			deferred, err := db.GetSchedulerState(ctx, job.ID)
			if err != nil {
				t.Fatal(err)
			}
			if deferred.DeferredUntil == nil || deferred.LastScheduledAt == nil ||
				!deferred.LastScheduledAt.Equal(fireAt) {
				t.Fatalf("missing durable deferral: %+v", deferred)
			}
			originalID := scheduledRunID(job.ID, fireAt)
			if scenario == "schedule replacement" || scenario == "past replacement" {
				job.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1h"}
				if scenario == "past replacement" {
					job.Schedule = &ScheduleSpec{Mode: ScheduleModeAt, Time: base.Format(time.RFC3339)}
				}
				if _, err := scheduler.Update(ctx, job); err != nil {
					t.Fatal(err)
				}
				run, err := db.GetRun(ctx, originalID)
				if err != nil {
					t.Fatal(err)
				}
				if run.Status != RunCancelled {
					t.Fatalf("superseded status = %s", run.Status)
				}
				_, err = db.SetScheduledDeferral(
					ctx,
					SchedulerClaim{JobID: job.ID, FireID: deferred.LastFireID, ScheduleHash: deferred.ScheduleHash},
					deferred.DeferredUntil,
				)
				if !errors.Is(err, ErrScheduledFireAlreadyClaimed) {
					t.Fatalf("stale deferral error = %v", err)
				}
			}
			if scenario == "gate release" {
				if err := scheduler.Start(ctx); err != nil {
					t.Fatal(err)
				}
				waitForTimers(t, clock, 1)
			}
			close(release)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if scenario == "schedule replacement" || scenario == "past replacement" {
				return
			}
			if admissionErr != nil {
				if err := scheduler.executeScheduledJob(ctx, job.ID); err != nil {
					t.Fatal(err)
				}
				run, err := db.GetRun(ctx, originalID)
				if err != nil {
					t.Fatal(err)
				}
				if run.Status != RunScheduled || run.EndedAt != nil || run.Error != "" || run.SessionID != "" {
					t.Fatalf("unstarted fire was consumed by admission failure: %+v", run)
				}
				if err := scheduler.Start(ctx); err != nil {
					t.Fatal(err)
				}
				waitForTimers(t, clock, 1)
				if got := len(creator.createCalls()); got != 2 {
					t.Fatalf("admission retry bypassed deferred timer: create calls = %d", got)
				}
			}
			if scenario != "gate release" {
				if err := scheduler.Stop(ctx); err != nil {
					t.Fatal(err)
				}
				if err := db.Close(ctx); err != nil {
					t.Fatal(err)
				}
				db, err = globaldb.OpenGlobalDB(ctx, path)
				if err != nil {
					t.Fatal(err)
				}
				clock = clockwork.NewFakeClockAt(fireAt.Add(10 * time.Minute))
				dispatcher = newTestDispatcher(t, newRecordingSessionCreator(), db, WithDispatcherNow(clock.Now))
				scheduler = newTestScheduler(t, dispatcher, WithSchedulerStore(db), WithSchedulerClock(clock))
				state, err := scheduler.Register(ctx, job)
				if err != nil {
					t.Fatal(err)
				}
				if !state.Registered {
					t.Fatal("deferred fire was discarded on restart")
				}
				if err := scheduler.Start(ctx); err != nil {
					t.Fatal(err)
				}
			}
			waitUntil(t, 3*time.Second, 10*time.Millisecond, func() bool {
				run, err := db.GetRun(ctx, originalID)
				return err == nil && run.Status == RunCompleted
			})
			if err := scheduler.Stop(ctx); err != nil {
				t.Fatal(err)
			}
			runs, err := db.ListRuns(ctx, RunQuery{ReadScope: store.ReadScope{ProfileID: job.ProfileID}, JobID: job.ID})
			if err != nil {
				t.Fatal(err)
			}
			completions := 0
			for _, run := range runs {
				if run.Status == RunCompleted {
					completions++
					if run.ID != originalID {
						t.Fatalf("replacement fire ran: %s", run.ID)
					}
				}
			}
			if completions != 1 {
				t.Fatalf("completed fires = %d", completions)
			}
			state, err := db.GetSchedulerState(ctx, job.ID)
			if err != nil {
				t.Fatal(err)
			}
			if state.DeferredUntil != nil {
				t.Fatal("completed fire remains deferred")
			}
		})
	}
}
