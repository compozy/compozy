//go:build integration

package automation

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"sync/atomic"
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
	if runs[0].WorkspaceID == nil || *runs[0].WorkspaceID != job.WorkspaceID || runs[0].Attribution != "recorded" {
		t.Fatalf("scheduled source attribution = %#v", runs[0])
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

	stopCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
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
		NextRunAt:           new(baseTime.Add(time.Minute)),
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

// Invariant: a cron's durable cursor follows explicit zone changes while upgrades preserve pending work.
func TestSchedulerIntegrationTimezoneReconciliation(t *testing.T) {
	t.Parallel()
	t.Run("Should retain an unstarted legacy reservation when adopting the zone identity", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openAutomationIntegrationDB(t, ctx)
		now := time.Date(2026, 10, 22, 8, 0, 0, 0, time.UTC)
		job := testJob(AutomationScopeGlobal, "legacy-reservation", "")
		job.Schedule = &ScheduleSpec{Mode: ScheduleModeCron, Expr: "30 8 * * *"}
		job, err := db.CreateJob(ctx, job)
		if err != nil {
			t.Fatal(err)
		}
		fireAt := now.Add(-23*time.Hour - 30*time.Minute)
		claim := SchedulerClaim{
			ProfileID:     job.ProfileID,
			JobID:         job.ID,
			RunID:         scheduledRunID(job.ID, fireAt),
			FireID:        scheduledFireID(job.ID, fireAt),
			ScheduledAt:   fireAt,
			ClaimedAt:     fireAt,
			NextRunAt:     new(now.Add(30 * time.Minute)),
			ScheduleHash:  scheduleHash(job.Schedule),
			CatchUpPolicy: SchedulerCatchUpPolicySkipMissed,
		}
		if _, err := db.ClaimScheduledRun(ctx, claim); err != nil {
			t.Fatal(err)
		}
		scheduler := newTestScheduler(
			t,
			newStubScheduleDispatcher(),
			WithSchedulerStore(db),
			WithSchedulerClock(clockwork.NewFakeClockAt(now)),
		)
		if _, err := scheduler.Register(ctx, job); err != nil {
			t.Fatal(err)
		}
		run, err := db.GetRun(ctx, claim.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status != RunScheduled || run.FireID != claim.FireID {
			t.Fatalf("legacy reservation changed on identity upgrade: %+v", run)
		}
		state, err := db.GetSchedulerState(ctx, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if state.LastFireID != claim.FireID || state.LastScheduledAt == nil || !state.LastScheduledAt.Equal(fireAt) {
			t.Fatalf("last fire changed on identity upgrade: %+v", state)
		}
	})
	for _, tc := range []struct {
		name       string
		zone       string
		legacy     bool
		overdue    bool
		wantHour   int
		mode       ScheduleMode
		skipMissed bool
	}{
		{name: "Should move an existing cron to the new zone", zone: "America/Sao_Paulo", wantHour: 11},
		{name: "Should preserve the cursor with the same zone", zone: "UTC", wantHour: 8},
		{name: "Should adopt a legacy cursor without moving it", zone: "UTC", legacy: true, wantHour: 8},
		{name: "Should retain legacy catch-up work", zone: "UTC", legacy: true, overdue: true, wantHour: 8},
		{name: "Should retain legacy skip-missed policy", zone: "UTC", legacy: true, overdue: true, skipMissed: true, wantHour: 8},
		{name: "Should preserve interval timing on a zone change", zone: "America/Sao_Paulo", mode: ScheduleModeEvery, wantHour: 8},
		{name: "Should preserve absolute timing on a zone change", zone: "America/Sao_Paulo", mode: ScheduleModeAt, wantHour: 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.Context(t)
			db := openAutomationIntegrationDB(t, ctx)
			base := time.Date(2026, 10, 22, 8, 0, 0, 0, time.UTC)
			clock := clockwork.NewFakeClockAt(base)
			job := testJob(AutomationScopeGlobal, "timezone-reconcile", "")
			job.Schedule = &ScheduleSpec{
				Mode:          ScheduleModeCron,
				Expr:          "30 8 * * *",
				CatchUpPolicy: SchedulerCatchUpPolicyReplay,
			}
			if tc.skipMissed {
				job.Schedule.CatchUpPolicy = SchedulerCatchUpPolicySkipMissed
			}
			if tc.mode == ScheduleModeEvery {
				job.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "30m"}
			}
			if tc.mode == ScheduleModeAt {
				job.Schedule = &ScheduleSpec{
					Mode: ScheduleModeAt,
					Time: base.Add(30 * time.Minute).Format(time.RFC3339),
				}
			}
			job, err := db.CreateJob(ctx, job)
			if err != nil {
				t.Fatal(err)
			}
			first := newTestScheduler(t, newStubScheduleDispatcher(), WithSchedulerStore(db), WithSchedulerClock(clock))
			if _, err := first.Register(ctx, job); err != nil {
				t.Fatal(err)
			}
			if err := first.Stop(ctx); err != nil {
				t.Fatal(err)
			}
			before, err := db.GetSchedulerState(ctx, job.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.legacy {
				before.ScheduleHash = scheduleHash(job.Schedule)
				if _, err := db.SaveSchedulerState(ctx, before); err != nil {
					t.Fatal(err)
				}
			}
			if tc.overdue {
				clock.Advance(31 * time.Minute)
			}
			zone, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			restarted := newTestScheduler(
				t,
				newStubScheduleDispatcher(),
				WithSchedulerStore(db),
				WithSchedulerClock(clock),
				WithSchedulerLocation(zone),
			)
			state, err := restarted.Register(ctx, job)
			if err != nil {
				t.Fatal(err)
			}
			want := time.Date(2026, 10, 22, tc.wantHour, 30, 0, 0, time.UTC)
			if tc.skipMissed {
				want = want.Add(24 * time.Hour)
			}
			if state.NextRun == nil || !state.NextRun.Equal(want) {
				t.Fatalf("next run = %v, want %s", state.NextRun, want)
			}
			if tc.legacy && !tc.overdue {
				if err := restarted.Stop(ctx); err != nil {
					t.Fatal(err)
				}
				changedZone, err := time.LoadLocation("America/Sao_Paulo")
				if err != nil {
					t.Fatal(err)
				}
				changed := newTestScheduler(
					t,
					newStubScheduleDispatcher(),
					WithSchedulerStore(db),
					WithSchedulerClock(clock),
					WithSchedulerLocation(changedZone),
				)
				state, err := changed.Register(ctx, job)
				if err != nil {
					t.Fatal(err)
				}
				want = want.Add(3 * time.Hour)
				if state.NextRun == nil || !state.NextRun.Equal(want) {
					t.Fatalf("post-upgrade zone edit next run = %v, want %s", state.NextRun, want)
				}
			}
		})
	}
}

// Invariant: a busy gate retains the original fire across restart or retires it
// on schedule replacement. Owner: durable automation scheduler integration suite.
func TestSchedulerIntegrationDeferredFire(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{
		"one-shot restart", "recurring restart", "schedule replacement", "past replacement", "gate release",
		"draining restart", "cancelled restart", "deadline restart", "recurring draining restart",
		"cron zone restart", "legacy cron restart",
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
			if scenario == "cron zone restart" || scenario == "legacy cron restart" {
				job.Schedule = &ScheduleSpec{Mode: ScheduleModeCron, Expr: "1 12 * * *"}
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
			if scenario == "legacy cron restart" {
				// Model the durable hash written by the previous release before upgrading the scheduler.
				deferred.ScheduleHash = scheduleHash(job.Schedule)
				if _, err := db.SaveSchedulerState(ctx, deferred); err != nil {
					t.Fatal(err)
				}
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
				location := time.UTC
				if scenario == "cron zone restart" {
					location, err = time.LoadLocation("America/Sao_Paulo")
					if err != nil {
						t.Fatal(err)
					}
				}
				scheduler = newTestScheduler(
					t,
					dispatcher,
					WithSchedulerStore(db),
					WithSchedulerClock(clock),
					WithSchedulerLocation(location),
				)
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
			if scenario == "cron zone restart" {
				wantNext := time.Date(2026, 4, 10, 15, 1, 0, 0, time.UTC)
				if state.NextRunAt == nil || !state.NextRunAt.Equal(wantNext) {
					t.Fatalf("next run after deferred completion = %v, want %s", state.NextRunAt, wantNext)
				}
			}
		})
	}
}

// Invariant: a busy SQLite claim backs off without advancing or duplicating the scheduled fire.
func TestSchedulerIntegrationSQLiteContention(t *testing.T) {
	t.Run("Should back off failed claims and dispatch the original fire once after unlock", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openAutomationIntegrationDB(t, ctx)
		base := time.Date(2026, time.October, 3, 12, 0, 0, 0, time.UTC)
		clock := clockwork.NewFakeClockAt(base)
		creator := newRecordingSessionCreator()
		dispatcher := newTestDispatcher(t, creator, db, WithDispatcherNow(clock.Now))
		logCounter := &schedulerFailureLogCounter{}
		scheduler := newTestScheduler(t, dispatcher, WithSchedulerStore(db), WithSchedulerClock(clock),
			WithSchedulerLogger(slog.New(slog.NewTextHandler(logCounter, &slog.HandlerOptions{Level: slog.LevelWarn}))))
		job := testJob(AutomationScopeGlobal, "contended-fire", "")
		fireAt := base.Add(time.Second)
		job.Schedule = &ScheduleSpec{Mode: ScheduleModeAt, Time: fireAt.Format(time.RFC3339)}
		job, err := db.CreateJob(ctx, job)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := scheduler.Register(ctx, job); err != nil {
			t.Fatal(err)
		}
		writer, err := db.DB().Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		locked := true
		t.Cleanup(func() {
			if locked {
				if _, err := writer.ExecContext(context.Background(), "ROLLBACK"); err != nil {
					t.Error(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := writer.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			t.Fatal(err)
		}
		if err := scheduler.Start(ctx); err != nil {
			t.Fatal(err)
		}
		waitForTimers(t, clock, 1)
		clock.Advance(time.Second)
		waitUntil(t, 2*time.Second, time.Millisecond, func() bool { return logCounter.count.Load() >= 1 })
		waitForTimers(t, clock, 1)
		if count := logCounter.count.Load(); count != 1 {
			t.Fatalf("failed claims before backoff = %d", count)
		}
		clock.Advance(time.Second)
		waitUntil(t, 2*time.Second, time.Millisecond, func() bool { return logCounter.count.Load() >= 2 })
		waitForTimers(t, clock, 1)
		if count := logCounter.count.Load(); count != 2 {
			t.Fatalf("failed claims after one retry = %d", count)
		}
		if _, err := writer.ExecContext(ctx, "ROLLBACK"); err != nil {
			t.Fatal(err)
		}
		locked = false
		clock.Advance(2 * time.Second)
		waitUntil(t, 2*time.Second, time.Millisecond, func() bool { return len(creator.createCalls()) == 1 })
		runs, err := db.ListRuns(ctx, RunQuery{ReadScope: store.ReadScope{ProfileID: job.ProfileID}, JobID: job.ID})
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) != 1 || runs[0].ID != scheduledRunID(job.ID, fireAt) {
			t.Fatalf("scheduled runs = %#v, want original fire exactly once", runs)
		}
	})
}

type schedulerFailureLogCounter struct{ count atomic.Int32 }

func (w *schedulerFailureLogCounter) Write(data []byte) (int, error) {
	w.count.Add(1)
	return len(data), nil
}
