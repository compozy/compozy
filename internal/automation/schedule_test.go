package automation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
	"github.com/jonboulle/clockwork"
)

func TestSchedulerCronStateUsesDeterministicNextRun(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 8, 30, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	scheduler := newTestScheduler(t, newStubScheduleDispatcher(), WithSchedulerClock(fakeClock))

	job := testJob(AutomationScopeGlobal, "cron-next-run", "")
	job.Schedule = &ScheduleSpec{
		Mode: ScheduleModeCron,
		Expr: "0 9 * * *",
	}

	state, err := scheduler.Register(context.Background(), job)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if !state.Registered {
		t.Fatal("Register().Registered = false, want true")
	}
	if state.NextRun == nil {
		t.Fatal("Register().NextRun = nil, want populated")
	}

	want := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	if got := *state.NextRun; !got.Equal(want) {
		t.Fatalf("Register().NextRun = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestSchedulerCronSkipsSpringForwardMissingWallTime(t *testing.T) {
	t.Run(
		"Should skip the nonexistent spring-forward wall time and fire once at the next valid local time",
		func(t *testing.T) {
			t.Parallel()

			location, err := time.LoadLocation("America/New_York")
			if err != nil {
				t.Fatalf("LoadLocation() error = %v", err)
			}
			baseTime := time.Date(2026, 3, 8, 1, 55, 0, 0, location)
			fakeClock := clockwork.NewFakeClockAt(baseTime)
			store := newMemorySchedulerStore()
			dispatcher := newStubScheduleDispatcher()
			scheduler := newTestScheduler(
				t,
				dispatcher,
				WithSchedulerClock(fakeClock),
				WithSchedulerStore(store),
				WithSchedulerLocation(location),
			)

			job := testJob(AutomationScopeGlobal, "spring-forward", "")
			job.Schedule = &ScheduleSpec{
				Mode: ScheduleModeCron,
				Expr: "30 2 * * *",
			}

			state, err := scheduler.Register(context.Background(), job)
			if err != nil {
				t.Fatalf("Register() error = %v", err)
			}
			wantNext := time.Date(2026, 3, 9, 2, 30, 0, 0, location)
			if state.NextRun == nil || !state.NextRun.Equal(wantNext) {
				t.Fatalf("Register().NextRun = %v, want %s", state.NextRun, wantNext.Format(time.RFC3339))
			}
			if err := scheduler.Start(testutil.Context(t)); err != nil {
				t.Fatalf("Start() error = %v", err)
			}

			waitForTimers(t, fakeClock, 1)
			afterSpringForwardGap := time.Date(2026, 3, 8, 4, 0, 0, 0, location)
			fakeClock.Advance(afterSpringForwardGap.Sub(baseTime))
			dispatcher.assertDispatchCount(t, 0)

			oneSecondBeforeNextRun := wantNext.Add(-1 * time.Second)
			fakeClock.Advance(oneSecondBeforeNextRun.Sub(afterSpringForwardGap))
			dispatcher.assertDispatchCount(t, 0)

			fakeClock.Advance(time.Second)
			dispatcher.waitForDispatchCount(t, 1, 2*time.Second)
			dispatcher.waitForCompletionCount(t, 1, 2*time.Second)

			storedRuns := store.runsForJob(job.ID)
			if got, want := len(storedRuns), 1; got != want {
				t.Fatalf("runsForJob() length = %d, want %d", got, want)
			}
			if storedRuns[0].ScheduledAt == nil || !storedRuns[0].ScheduledAt.Equal(wantNext) {
				t.Fatalf(
					"stored run ScheduledAt = %v, want %s",
					storedRuns[0].ScheduledAt,
					wantNext.Format(time.RFC3339),
				)
			}
			schedulerState, err := store.GetSchedulerState(context.Background(), job.ID)
			if err != nil {
				t.Fatalf("GetSchedulerState() error = %v", err)
			}
			wantFollowingRun := time.Date(2026, 3, 10, 2, 30, 0, 0, location)
			if schedulerState.NextRunAt == nil || !schedulerState.NextRunAt.Equal(wantFollowingRun) {
				t.Fatalf(
					"NextRunAt after first fire = %v, want %s",
					schedulerState.NextRunAt,
					wantFollowingRun.Format(time.RFC3339),
				)
			}
		},
	)
}

func TestSchedulerEveryStateUsesIntervalSemantics(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	scheduler := newTestScheduler(t, newStubScheduleDispatcher(), WithSchedulerClock(fakeClock))

	job := testJob(AutomationScopeGlobal, "every-next-run", "")
	job.Schedule = &ScheduleSpec{
		Mode:     ScheduleModeEvery,
		Interval: "30m",
	}

	state, err := scheduler.Register(context.Background(), job)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if state.NextRun == nil {
		t.Fatal("Register().NextRun = nil, want populated")
	}

	want := baseTime.Add(30 * time.Minute)
	if got := *state.NextRun; !got.Equal(want) {
		t.Fatalf("Register().NextRun = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestScheduledFireIsCatchUpShouldSeparateMisfiresFromTimerJitter(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

	t.Run("Should not mark a normally delayed timer tick as catch-up", func(t *testing.T) {
		t.Parallel()

		scheduledAt := base
		claimedAt := scheduledAt.Add(50 * time.Millisecond)
		state := SchedulerState{CatchUpPolicy: SchedulerCatchUpPolicyCoalesce}

		if scheduledFireIsCatchUp(scheduledAt, claimedAt, state) {
			t.Fatal("scheduledFireIsCatchUp() = true, want false for normal timer jitter")
		}
	})

	t.Run("Should mark a coalesced missed instant as catch-up", func(t *testing.T) {
		t.Parallel()

		scheduledAt := base.Add(-1 * time.Minute)
		claimedAt := scheduledAt
		state := SchedulerState{
			CatchUpPolicy: SchedulerCatchUpPolicyCoalesce,
			LastMisfireAt: timePointer(base),
			MisfireCount:  1,
		}

		if !scheduledFireIsCatchUp(scheduledAt, claimedAt, state) {
			t.Fatal("scheduledFireIsCatchUp() = false, want true for coalesced missed fire")
		}
	})

	t.Run("Should not let an old misfire mark a future on-time fire as catch-up", func(t *testing.T) {
		t.Parallel()

		scheduledAt := base.Add(time.Minute)
		claimedAt := scheduledAt.Add(50 * time.Millisecond)
		state := SchedulerState{
			CatchUpPolicy: SchedulerCatchUpPolicyReplay,
			LastMisfireAt: timePointer(base),
			MisfireCount:  2,
		}

		if scheduledFireIsCatchUp(scheduledAt, claimedAt, state) {
			t.Fatal("scheduledFireIsCatchUp() = true, want false after old misfire was drained")
		}
	})
}

func TestSchedulerAtJobUnregistersAfterFiringOnce(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	dispatcher := newStubScheduleDispatcher()
	scheduler := newTestScheduler(t, dispatcher, WithSchedulerClock(fakeClock))

	job := testJob(AutomationScopeGlobal, "at-once", "")
	job.Schedule = &ScheduleSpec{
		Mode: ScheduleModeAt,
		Time: baseTime.Add(1 * time.Minute).Format(time.RFC3339),
	}

	state, err := scheduler.Register(context.Background(), job)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if !state.Registered {
		t.Fatal("Register().Registered = false, want true")
	}
	if err := scheduler.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	waitForTimers(t, fakeClock, 1)
	fakeClock.Advance(1 * time.Minute)
	dispatcher.waitForDispatchCount(t, 1, 2*time.Second)
	dispatcher.waitForCompletionCount(t, 1, 2*time.Second)

	if _, err := scheduler.State(job.ID); !errors.Is(err, ErrScheduledJobNotFound) {
		t.Fatalf("State() error = %v, want ErrScheduledJobNotFound", err)
	}
	if got := len(scheduler.States()); got != 0 {
		t.Fatalf("len(States()) = %d, want 0", got)
	}
}

func TestSchedulerSingletonPreventsOverlap(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	dispatcher := newStubScheduleDispatcher()
	dispatcher.blockNextDispatch()
	scheduler := newTestScheduler(t, dispatcher, WithSchedulerClock(fakeClock))

	job := testJob(AutomationScopeGlobal, "singleton", "")
	job.Schedule = &ScheduleSpec{
		Mode:     ScheduleModeEvery,
		Interval: "1s",
	}

	if _, err := scheduler.Register(context.Background(), job); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := scheduler.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	waitForTimers(t, fakeClock, 1)
	fakeClock.Advance(1 * time.Second)
	dispatcher.waitForDispatchCount(t, 1, 2*time.Second)

	fakeClock.Advance(1 * time.Second)
	dispatcher.assertDispatchCount(t, 1)

	dispatcher.releaseBlockedDispatch()
	dispatcher.waitForCompletionCount(t, 1, 2*time.Second)
	waitForTimers(t, fakeClock, 1)
	fakeClock.Advance(0)
	dispatcher.waitForDispatchCount(t, 2, 2*time.Second)
}

func TestSchedulerAdvancesDurableCursorBeforeDispatch(t *testing.T) {
	t.Run("Should advance the durable cursor before dispatch", func(t *testing.T) {
		t.Parallel()

		baseTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
		fakeClock := clockwork.NewFakeClockAt(baseTime)
		store := newMemorySchedulerStore()
		dispatcher := newStubScheduleDispatcher()
		dispatcher.onDispatch = func(req DispatchRequest) {
			if req.ReservedRun == nil {
				t.Fatal("Dispatch() ReservedRun = nil, want durable run reservation")
			}
			state, err := store.GetSchedulerState(context.Background(), req.Job.ID)
			if err != nil {
				t.Fatalf("GetSchedulerState() error = %v", err)
			}
			if state.NextRunAt == nil || !state.NextRunAt.After(*req.ReservedRun.ScheduledAt) {
				t.Fatalf("durable cursor was not advanced before dispatch: state=%#v run=%#v", state, req.ReservedRun)
			}
			assertNamedParticipation(t, req.ReservedRun.NetworkParticipation, "scheduled-task")
		}
		scheduler := newTestScheduler(t, dispatcher, WithSchedulerClock(fakeClock), WithSchedulerStore(store))

		job := testJob(AutomationScopeWorkspace, "pre-dispatch-advance", "ws-scheduled-task")
		job.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1m"}
		job.AgentName = ""
		job.Prompt = ""
		job.Task = &JobTaskConfig{
			Title:                "Run scheduled task",
			NetworkParticipation: testNamedParticipation("scheduled-task"),
		}
		if _, err := scheduler.Register(context.Background(), job); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if err := scheduler.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		waitForTimers(t, fakeClock, 1)
		fakeClock.Advance(time.Minute)
		dispatcher.waitForDispatchCount(t, 1, 2*time.Second)
	})
}

func TestSchedulerDefersNextRunAfterFireLimit(t *testing.T) {
	t.Run("Should defer the next run to the fire-limit retry time", func(t *testing.T) {
		t.Parallel()

		baseTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
		scheduledAt := baseTime.Add(time.Minute)
		retryAt := baseTime.Add(47 * time.Minute)
		fakeClock := clockwork.NewFakeClockAt(baseTime)
		store := newMemorySchedulerStore()
		dispatcher := newStubScheduleDispatcher()
		dispatcher.dispatchResult = &FireLimitError{
			Count:   12,
			Limit:   12,
			Window:  time.Hour,
			RetryAt: retryAt,
		}
		scheduler := newTestScheduler(
			t,
			dispatcher,
			WithSchedulerClock(fakeClock),
			WithSchedulerStore(store),
		)

		job := testJob(AutomationScopeGlobal, "fire-limit-deferral", "")
		job.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1m"}
		if _, err := scheduler.Register(context.Background(), job); err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if err := scheduler.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		waitForTimers(t, fakeClock, 1)
		fakeClock.Advance(time.Minute)
		dispatcher.waitForDispatchCount(t, 1, 2*time.Second)
		dispatcher.waitForCompletionCount(t, 1, 2*time.Second)
		store.waitForNextRunAt(t, job.ID, retryAt, 2*time.Second)

		state, err := store.GetSchedulerState(context.Background(), job.ID)
		if err != nil {
			t.Fatalf("GetSchedulerState() error = %v", err)
		}
		if state.NextRunAt == nil || !state.NextRunAt.Equal(retryAt) {
			t.Fatalf("state.NextRunAt = %v, want %s", state.NextRunAt, retryAt.Format(time.RFC3339))
		}
		if got := store.deliveryErrorForRun(scheduledRunID(job.ID, scheduledAt)); got != "" {
			t.Fatalf("deliveryErrorForRun() = %q, want empty", got)
		}

		runtimeState, err := scheduler.State(job.ID)
		if err != nil {
			t.Fatalf("State() error = %v", err)
		}
		if runtimeState.NextRun == nil || !runtimeState.NextRun.Equal(retryAt) {
			t.Fatalf("State().NextRun = %v, want %s", runtimeState.NextRun, retryAt.Format(time.RFC3339))
		}
	})
}

func TestSchedulerReconcilesMissedRunsWithSkipPolicy(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 5, 0, 0, time.UTC)
	missedAt := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	store := newMemorySchedulerStore()
	job := testJob(AutomationScopeGlobal, "missed-skip", "")
	job.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1m"}
	_, err := store.SaveSchedulerState(context.Background(), SchedulerState{
		JobID:         job.ID,
		NextRunAt:     &missedAt,
		ScheduleHash:  scheduleHash(job.Schedule),
		CatchUpPolicy: SchedulerCatchUpPolicySkipMissed,
		UpdatedAt:     missedAt,
	})
	if err != nil {
		t.Fatalf("SaveSchedulerState() error = %v", err)
	}

	dispatcher := newStubScheduleDispatcher()
	scheduler := newTestScheduler(t, dispatcher, WithSchedulerClock(fakeClock), WithSchedulerStore(store))
	state, err := scheduler.Register(context.Background(), job)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	wantNext := baseTime.Add(time.Minute)
	if state.NextRun == nil || !state.NextRun.Equal(wantNext) {
		t.Fatalf("Register().NextRun = %v, want %s", state.NextRun, wantNext.Format(time.RFC3339))
	}
	if got, want := state.MisfireCount, 1; got != want {
		t.Fatalf("Register().MisfireCount = %d, want %d", got, want)
	}
	if err := scheduler.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	waitForTimers(t, fakeClock, 1)
	dispatcher.assertDispatchCount(t, 0)
	fakeClock.Advance(time.Minute)
	dispatcher.waitForDispatchCount(t, 1, 2*time.Second)
}

func TestSchedulerReconcilesMissedRunsWithCoalescePolicy(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 5, 0, 0, time.UTC)
	missedAt := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	store := newMemorySchedulerStore()
	job := testJob(AutomationScopeGlobal, "missed-coalesce", "")
	job.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1m"}
	if _, err := store.SaveSchedulerState(context.Background(), SchedulerState{
		JobID:         job.ID,
		NextRunAt:     &missedAt,
		ScheduleHash:  scheduleHash(job.Schedule),
		CatchUpPolicy: SchedulerCatchUpPolicyCoalesce,
		UpdatedAt:     missedAt,
	}); err != nil {
		t.Fatalf("SaveSchedulerState() error = %v", err)
	}

	dispatcher := newStubScheduleDispatcher()
	scheduler := newTestScheduler(t, dispatcher, WithSchedulerClock(fakeClock), WithSchedulerStore(store))
	state, err := scheduler.Register(context.Background(), job)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if state.NextRun == nil || !state.NextRun.Equal(baseTime) {
		t.Fatalf("Register().NextRun = %v, want coalesced due %s", state.NextRun, baseTime.Format(time.RFC3339))
	}
	if err := scheduler.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	waitForTimers(t, fakeClock, 1)
	dispatcher.waitForDispatchCount(t, 1, 2*time.Second)
	calls := dispatcher.callSnapshot()
	if got := len(calls); got != 1 {
		t.Fatalf("dispatch calls = %d, want 1", got)
	}
	if calls[0].ScheduledAt == nil || !calls[0].ScheduledAt.Equal(baseTime) {
		t.Fatalf("Dispatch().ScheduledAt = %v, want %s", calls[0].ScheduledAt, baseTime.Format(time.RFC3339))
	}
	if !calls[0].CatchUp || calls[0].CatchUpPolicy != SchedulerCatchUpPolicyCoalesce {
		t.Fatalf("Dispatch() catch-up = %v/%q, want true/coalesce", calls[0].CatchUp, calls[0].CatchUpPolicy)
	}
}

func TestSchedulerCoalescesLongCronDowntime(t *testing.T) {
	t.Run("Should dispatch exactly one catch-up after more than ten thousand missed cron fires", func(t *testing.T) {
		t.Parallel()

		baseTime := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
		missedAt := baseTime.Add(-20_001 * time.Minute)
		fakeClock := clockwork.NewFakeClockAt(baseTime)
		store := newMemorySchedulerStore()
		job := testJob(AutomationScopeGlobal, "long-cron-coalesce", "")
		job.Schedule = &ScheduleSpec{
			Mode:          ScheduleModeCron,
			Expr:          "* * * * *",
			CatchUpPolicy: SchedulerCatchUpPolicyCoalesce,
		}
		if _, err := store.SaveSchedulerState(context.Background(), SchedulerState{
			JobID:         job.ID,
			NextRunAt:     &missedAt,
			ScheduleHash:  scheduleHash(job.Schedule),
			CatchUpPolicy: SchedulerCatchUpPolicyCoalesce,
			UpdatedAt:     missedAt,
		}); err != nil {
			t.Fatalf("SaveSchedulerState() error = %v", err)
		}

		dispatcher := newStubScheduleDispatcher()
		scheduler := newTestScheduler(
			t,
			dispatcher,
			WithSchedulerClock(fakeClock),
			WithSchedulerStore(store),
		)
		state, err := scheduler.Register(context.Background(), job)
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
		if state.NextRun == nil || !state.NextRun.Equal(baseTime) {
			t.Fatalf("Register().NextRun = %v, want latest missed fire %s", state.NextRun, baseTime)
		}
		if err := scheduler.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		dispatcher.waitForDispatchCount(t, 1, 2*time.Second)
		dispatcher.waitForCompletionCount(t, 1, 2*time.Second)
		dispatcher.assertDispatchCount(t, 1)
		stored, err := store.GetSchedulerState(context.Background(), job.ID)
		if err != nil {
			t.Fatalf("GetSchedulerState() error = %v", err)
		}
		wantNext := baseTime.Add(time.Minute)
		if stored.NextRunAt == nil || !stored.NextRunAt.Equal(wantNext) {
			t.Fatalf("NextRunAt = %v, want %s", stored.NextRunAt, wantNext)
		}
	})
}

func TestSchedulerReconcilesMissedRunsWithReplayPolicy(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 3, 0, 0, time.UTC)
	missedAt := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	store := newMemorySchedulerStore()
	job := testJob(AutomationScopeGlobal, "missed-replay", "")
	job.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1m"}
	if _, err := store.SaveSchedulerState(context.Background(), SchedulerState{
		JobID:         job.ID,
		NextRunAt:     &missedAt,
		ScheduleHash:  scheduleHash(job.Schedule),
		CatchUpPolicy: SchedulerCatchUpPolicyReplay,
		UpdatedAt:     missedAt,
	}); err != nil {
		t.Fatalf("SaveSchedulerState() error = %v", err)
	}

	dispatcher := newStubScheduleDispatcher()
	dispatcher.onDispatch = func(req DispatchRequest) {
		if req.ReservedRun != nil {
			store.setRunStatus(req.ReservedRun.ID, RunCompleted, fakeClock.Now())
		}
	}
	scheduler := newTestScheduler(t, dispatcher, WithSchedulerClock(fakeClock), WithSchedulerStore(store))
	state, err := scheduler.Register(context.Background(), job)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if state.NextRun == nil || !state.NextRun.Equal(missedAt) {
		t.Fatalf("Register().NextRun = %v, want first missed %s", state.NextRun, missedAt.Format(time.RFC3339))
	}
	if err := scheduler.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	waitForTimers(t, fakeClock, 1)
	dispatcher.waitForDispatchCount(t, 4, 2*time.Second)
	calls := dispatcher.callSnapshot()
	if got := len(calls); got < 4 {
		t.Fatalf("dispatch calls = %d, want at least 4", got)
	}
	for idx := range 4 {
		want := missedAt.Add(time.Duration(idx) * time.Minute)
		if calls[idx].ScheduledAt == nil || !calls[idx].ScheduledAt.Equal(want) {
			t.Fatalf("Dispatch(%d).ScheduledAt = %v, want %s", idx, calls[idx].ScheduledAt, want.Format(time.RFC3339))
		}
		if !calls[idx].CatchUp || calls[idx].CatchUpPolicy != SchedulerCatchUpPolicyReplay {
			t.Fatalf(
				"Dispatch(%d) catch-up = %v/%q, want true/replay",
				idx,
				calls[idx].CatchUp,
				calls[idx].CatchUpPolicy,
			)
		}
	}
}

func TestSchedulerReconcilesMissedRunOnceWithinGrace(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 5, 0, 0, time.UTC)
	missedAt := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	store := newMemorySchedulerStore()
	job := testJob(AutomationScopeGlobal, "missed-run-once", "")
	job.Schedule = &ScheduleSpec{
		Mode:                ScheduleModeEvery,
		Interval:            "1m",
		CatchUpPolicy:       SchedulerCatchUpPolicyRunOnce,
		MisfireGraceSeconds: 10 * 60,
	}
	if _, err := store.SaveSchedulerState(context.Background(), SchedulerState{
		JobID:               job.ID,
		NextRunAt:           &missedAt,
		ScheduleHash:        scheduleHash(job.Schedule),
		CatchUpPolicy:       SchedulerCatchUpPolicyRunOnce,
		MisfireGraceSeconds: 10 * 60,
		UpdatedAt:           missedAt,
	}); err != nil {
		t.Fatalf("SaveSchedulerState() error = %v", err)
	}

	dispatcher := newStubScheduleDispatcher()
	scheduler := newTestScheduler(t, dispatcher, WithSchedulerClock(fakeClock), WithSchedulerStore(store))
	state, err := scheduler.Register(context.Background(), job)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if got, want := state.CatchUpPolicy, SchedulerCatchUpPolicyRunOnce; got != want {
		t.Fatalf("Register().CatchUpPolicy = %q, want %q", got, want)
	}
	if got, want := state.MisfireGraceSeconds, 10*60; got != want {
		t.Fatalf("Register().MisfireGraceSeconds = %d, want %d", got, want)
	}
	if err := scheduler.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	dispatcher.waitForDispatchCount(t, 1, 2*time.Second)
	dispatcher.waitForCompletionCount(t, 1, 2*time.Second)
	dispatcher.assertDispatchCount(t, 1)
	calls := dispatcher.callSnapshot()
	if !calls[0].CatchUp || calls[0].CatchUpPolicy != SchedulerCatchUpPolicyRunOnce {
		t.Fatalf(
			"Dispatch() catch-up = %v/%q, want true/%q",
			calls[0].CatchUp,
			calls[0].CatchUpPolicy,
			SchedulerCatchUpPolicyRunOnce,
		)
	}
	stored, err := store.GetSchedulerState(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetSchedulerState() error = %v", err)
	}
	wantNext := baseTime.Add(time.Minute)
	if stored.NextRunAt == nil || !stored.NextRunAt.Equal(wantNext) {
		t.Fatalf("NextRunAt = %v, want %s", stored.NextRunAt, wantNext.Format(time.RFC3339))
	}
}

func TestSchedulerResetsExplicitReliabilityToTargetDefault(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	store := newMemorySchedulerStore()
	job := testJob(AutomationScopeGlobal, "target-default-reset", "")
	previousSchedule := &ScheduleSpec{
		Mode:                ScheduleModeEvery,
		Interval:            "1m",
		CatchUpPolicy:       SchedulerCatchUpPolicyReplay,
		MisfireGraceSeconds: 30,
	}
	job.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1m"}
	nextRun := baseTime.Add(time.Minute)
	if _, err := store.SaveSchedulerState(context.Background(), SchedulerState{
		JobID:               job.ID,
		NextRunAt:           &nextRun,
		ScheduleHash:        scheduleHash(previousSchedule),
		CatchUpPolicy:       SchedulerCatchUpPolicyReplay,
		MisfireGraceSeconds: 30,
		UpdatedAt:           baseTime.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SaveSchedulerState() error = %v", err)
	}

	scheduler := newTestScheduler(
		t,
		newStubScheduleDispatcher(),
		WithSchedulerClock(fakeClock),
		WithSchedulerStore(store),
		WithSchedulerCatchUpPolicyResolver(func(context.Context, Job) (SchedulerCatchUpPolicy, error) {
			return SchedulerCatchUpPolicyCoalesce, nil
		}),
	)
	state, err := scheduler.Register(context.Background(), job)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if got, want := state.CatchUpPolicy, SchedulerCatchUpPolicyCoalesce; got != want {
		t.Fatalf("Register().CatchUpPolicy = %q, want target default %q", got, want)
	}
	if got := state.MisfireGraceSeconds; got != 0 {
		t.Fatalf("Register().MisfireGraceSeconds = %d, want daemon jitter default 0", got)
	}
}

func TestSchedulerRecordsGraceExceededSkip(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 5, 0, 0, time.UTC)
	missedAt := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	store := newMemorySchedulerStore()
	job := testJob(AutomationScopeGlobal, "missed-grace-skip", "")
	job.Schedule = &ScheduleSpec{
		Mode:                ScheduleModeEvery,
		Interval:            "1m",
		CatchUpPolicy:       SchedulerCatchUpPolicySkipMissed,
		MisfireGraceSeconds: 30,
	}
	if _, err := store.SaveSchedulerState(context.Background(), SchedulerState{
		JobID:               job.ID,
		NextRunAt:           &missedAt,
		ScheduleHash:        scheduleHash(job.Schedule),
		CatchUpPolicy:       SchedulerCatchUpPolicySkipMissed,
		MisfireGraceSeconds: 30,
		UpdatedAt:           missedAt,
	}); err != nil {
		t.Fatalf("SaveSchedulerState() error = %v", err)
	}

	dispatcher := newStubScheduleDispatcher()
	scheduler := newTestScheduler(t, dispatcher, WithSchedulerClock(fakeClock), WithSchedulerStore(store))
	if _, err := scheduler.Register(context.Background(), job); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	dispatcher.assertDispatchCount(t, 0)
	runs := store.runsForJob(job.ID)
	if got, want := len(runs), 1; got != want {
		t.Fatalf("runsForJob() length = %d, want %d", got, want)
	}
	if got, want := runs[0].Status, RunCancelled; got != want {
		t.Fatalf("skip run status = %q, want %q", got, want)
	}
	if got, want := runs[0].Metadata[SchedulerSkipReasonMetadataKey], string(
		SchedulerSkipReasonGraceExceeded,
	); got != want {
		t.Fatalf("skip reason = %#v, want %q", got, want)
	}
	state, err := store.GetSchedulerState(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetSchedulerState() error = %v", err)
	}
	if got, want := state.MisfireCount, 1; got != want {
		t.Fatalf("MisfireCount = %d, want %d", got, want)
	}
	wantNext := baseTime.Add(time.Minute)
	if state.NextRunAt == nil || !state.NextRunAt.Equal(wantNext) {
		t.Fatalf("NextRunAt = %v, want %s", state.NextRunAt, wantNext.Format(time.RFC3339))
	}
}

func TestSchedulerRecordsDeliveryErrorWithoutRollingBackCursor(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	store := newMemorySchedulerStore()
	dispatchErr := errors.New("dispatcher unavailable")
	dispatcher := newStubScheduleDispatcher()
	dispatcher.dispatchResult = dispatchErr
	scheduler := newTestScheduler(t, dispatcher, WithSchedulerClock(fakeClock), WithSchedulerStore(store))

	job := testJob(AutomationScopeGlobal, "delivery-error", "")
	job.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1m"}
	if _, err := scheduler.Register(context.Background(), job); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := scheduler.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	waitForTimers(t, fakeClock, 1)
	fakeClock.Advance(time.Minute)
	dispatcher.waitForDispatchCount(t, 1, 2*time.Second)
	dispatcher.waitForCompletionCount(t, 1, 2*time.Second)

	state, err := store.GetSchedulerState(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetSchedulerState() error = %v", err)
	}
	wantNext := baseTime.Add(2 * time.Minute)
	if state.NextRunAt == nil || !state.NextRunAt.Equal(wantNext) {
		t.Fatalf("NextRunAt after delivery error = %v, want %s", state.NextRunAt, wantNext.Format(time.RFC3339))
	}
	if got := store.waitForDeliveryError(
		t,
		scheduledRunID(job.ID, baseTime.Add(time.Minute)),
		2*time.Second,
	); got != dispatchErr.Error() {
		t.Fatalf("delivery error = %q, want %q", got, dispatchErr.Error())
	}
}

func TestSchedulerRestartAfterClaimDoesNotDuplicateAlreadyClaimedFire(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	firstClock := clockwork.NewFakeClockAt(baseTime)
	store := newMemorySchedulerStore()
	firstDispatcher := newStubScheduleDispatcher()
	firstDispatcher.blockNextDispatch()
	firstScheduler := newTestScheduler(t, firstDispatcher, WithSchedulerClock(firstClock), WithSchedulerStore(store))

	job := testJob(AutomationScopeGlobal, "restart-window", "")
	job.Schedule = &ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1m"}
	if _, err := firstScheduler.Register(context.Background(), job); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}

	fireAt := baseTime.Add(time.Minute)
	firstClock.Advance(time.Minute)
	dispatchCtx, cancelDispatch := context.WithCancel(testutil.Context(t))
	defer cancelDispatch()
	firstErrCh := make(chan error, 1)
	go func() {
		firstErrCh <- firstScheduler.executeScheduledJob(dispatchCtx, job.ID)
	}()
	firstDispatcher.waitForDispatchCount(t, 1, 2*time.Second)

	claimedState, err := store.GetSchedulerState(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("GetSchedulerState(after first claim) error = %v", err)
	}
	wantNext := baseTime.Add(2 * time.Minute)
	if claimedState.LastScheduledAt == nil || !claimedState.LastScheduledAt.Equal(fireAt) {
		t.Fatalf("LastScheduledAt after claim = %v, want %s", claimedState.LastScheduledAt, fireAt.Format(time.RFC3339))
	}
	if claimedState.NextRunAt == nil || !claimedState.NextRunAt.Equal(wantNext) {
		t.Fatalf("NextRunAt after claim = %v, want %s", claimedState.NextRunAt, wantNext.Format(time.RFC3339))
	}

	secondClock := clockwork.NewFakeClockAt(fireAt.Add(10 * time.Second))
	secondDispatcher := newStubScheduleDispatcher()
	secondScheduler := newTestScheduler(t, secondDispatcher, WithSchedulerClock(secondClock), WithSchedulerStore(store))
	restartedState, err := secondScheduler.Register(context.Background(), job)
	if err != nil {
		t.Fatalf("second Register() error = %v", err)
	}
	if restartedState.NextRun == nil || !restartedState.NextRun.Equal(wantNext) {
		t.Fatalf("restarted NextRun = %v, want %s", restartedState.NextRun, wantNext.Format(time.RFC3339))
	}
	if err := secondScheduler.Start(testutil.Context(t)); err != nil {
		t.Fatalf("second Start() error = %v", err)
	}

	waitForTimers(t, secondClock, 1)
	secondDispatcher.assertDispatchCount(t, 0)
	secondClock.Advance(50 * time.Second)
	secondDispatcher.assertDispatchCount(t, 0)
	store.waitForNextRunAt(t, job.ID, baseTime.Add(3*time.Minute), 2*time.Second)
	skippedRuns := store.runsForJob(job.ID)
	if got, want := len(skippedRuns), 2; got != want {
		t.Fatalf("runs after overlap = %d, want %d", got, want)
	}
	if got, want := skippedRuns[1].Metadata[SchedulerSkipReasonMetadataKey], string(
		SchedulerSkipReasonSelfOverlap,
	); got != want {
		t.Fatalf("overlap skip reason = %#v, want %q", got, want)
	}

	cancelDispatch()
	select {
	case err := <-firstErrCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("first executeScheduledJob() error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first executeScheduledJob() did not exit after cancellation")
	}
	store.setRunStatus(scheduledRunID(job.ID, fireAt), RunCancelled, secondClock.Now())
	secondClock.Advance(time.Minute)
	secondDispatcher.waitForDispatchCount(t, 1, 2*time.Second)
}

func TestSchedulerDisableAndUnregisterRemoveFutureFires(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	dispatcher := newStubScheduleDispatcher()
	scheduler := newTestScheduler(t, dispatcher, WithSchedulerClock(fakeClock))

	disabledJob := testJob(AutomationScopeGlobal, "disabled", "")
	disabledJob.Schedule = &ScheduleSpec{
		Mode:     ScheduleModeEvery,
		Interval: "1s",
	}
	if _, err := scheduler.Register(context.Background(), disabledJob); err != nil {
		t.Fatalf("Register(disabledJob) error = %v", err)
	}

	unregisteredJob := testJob(AutomationScopeGlobal, "unregistered", "")
	unregisteredJob.Schedule = &ScheduleSpec{
		Mode:     ScheduleModeEvery,
		Interval: "1s",
	}
	if _, err := scheduler.Register(context.Background(), unregisteredJob); err != nil {
		t.Fatalf("Register(unregisteredJob) error = %v", err)
	}

	if err := scheduler.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	disabledJob.Enabled = false
	state, err := scheduler.Update(context.Background(), disabledJob)
	if err != nil {
		t.Fatalf("Update(disabledJob) error = %v", err)
	}
	if state.Registered {
		t.Fatal("Update(disabledJob).Registered = true, want false")
	}
	if err := scheduler.Unregister(context.Background(), unregisteredJob.ID); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}

	fakeClock.Advance(3 * time.Second)
	dispatcher.assertDispatchCount(t, 0)
}

func TestSchedulerPastOneTimeJobIsSkippedWithoutRegistration(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	scheduler := newTestScheduler(t, newStubScheduleDispatcher(), WithSchedulerClock(fakeClock))

	job := testJob(AutomationScopeGlobal, "past-at", "")
	job.Schedule = &ScheduleSpec{
		Mode: ScheduleModeAt,
		Time: baseTime.Add(-1 * time.Minute).Format(time.RFC3339),
	}

	state, err := scheduler.Register(context.Background(), job)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if state.Registered {
		t.Fatal("Register().Registered = true, want false")
	}
	if got := len(scheduler.States()); got != 0 {
		t.Fatalf("len(States()) = %d, want 0", got)
	}
}

func TestNewSchedulerAppliesLocationAndStopTimeoutOptions(t *testing.T) {
	t.Parallel()

	if _, err := NewScheduler(nil); err == nil {
		t.Fatal("NewScheduler(nil dispatcher) error = nil, want non-nil")
	}

	dispatcher := newStubScheduleDispatcher()
	location := time.FixedZone("UTC-3", -3*60*60)
	scheduler, err := NewScheduler(
		dispatcher,
		WithSchedulerClock(clockwork.NewFakeClock()),
		WithSchedulerLocation(location),
		WithSchedulerStopTimeout(3*time.Second),
	)
	if err != nil {
		t.Fatalf("NewScheduler() error = %v", err)
	}
	t.Cleanup(func() {
		if err := scheduler.Stop(context.Background()); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})

	if got, want := scheduler.location, location; got != want {
		t.Fatalf("scheduler.location = %v, want %v", got, want)
	}
	if got, want := scheduler.stopTimeout, 3*time.Second; got != want {
		t.Fatalf("scheduler.stopTimeout = %s, want %s", got, want)
	}
}

func TestSchedulerStartAndShutdownLifecycleGuards(t *testing.T) {
	t.Parallel()

	scheduler := newTestScheduler(t, newStubScheduleDispatcher())

	cancelledCtx, cancel := context.WithCancel(testutil.Context(t))
	cancel()
	if err := scheduler.Start(cancelledCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Start(canceled) error = %v, want context.Canceled", err)
	}
	if err := scheduler.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := scheduler.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start(second) error = %v", err)
	}
	if err := scheduler.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := scheduler.Start(testutil.Context(t)); !errors.Is(err, ErrSchedulerStopped) {
		t.Fatalf("Start(after shutdown) error = %v, want ErrSchedulerStopped", err)
	}
}

func TestSchedulerRegisterAndLookupErrorPaths(t *testing.T) {
	t.Parallel()

	scheduler := newTestScheduler(t, newStubScheduleDispatcher())
	job := testJob(AutomationScopeGlobal, "duplicate", "")

	if _, err := scheduler.Register(context.Background(), job); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if _, err := scheduler.Register(context.Background(), job); !errors.Is(err, ErrScheduledJobAlreadyRegistered) {
		t.Fatalf("Register(duplicate) error = %v, want ErrScheduledJobAlreadyRegistered", err)
	}
	if _, err := scheduler.State("missing"); !errors.Is(err, ErrScheduledJobNotFound) {
		t.Fatalf("State(missing) error = %v, want ErrScheduledJobNotFound", err)
	}

	missingJob := testJob(AutomationScopeGlobal, "missing", "")
	missingJob.ID = "missing"
	if _, err := scheduler.Update(context.Background(), missingJob); !errors.Is(err, ErrScheduledJobNotFound) {
		t.Fatalf("Update(missing) error = %v, want ErrScheduledJobNotFound", err)
	}
	if err := scheduler.Unregister(context.Background(), "missing"); !errors.Is(err, ErrScheduledJobNotFound) {
		t.Fatalf("Unregister(missing) error = %v, want ErrScheduledJobNotFound", err)
	}
}

func TestSchedulerUpdateReplacesNextRunAndStatesAreSorted(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	fakeClock := clockwork.NewFakeClockAt(baseTime)
	scheduler := newTestScheduler(t, newStubScheduleDispatcher(), WithSchedulerClock(fakeClock))

	jobB := testJob(AutomationScopeGlobal, "b", "")
	jobB.ID = "job-b"
	jobA := testJob(AutomationScopeGlobal, "a", "")
	jobA.ID = "job-a"

	if _, err := scheduler.Register(context.Background(), jobB); err != nil {
		t.Fatalf("Register(jobB) error = %v", err)
	}
	if _, err := scheduler.Register(context.Background(), jobA); err != nil {
		t.Fatalf("Register(jobA) error = %v", err)
	}

	jobB.Schedule = &ScheduleSpec{
		Mode:     ScheduleModeEvery,
		Interval: "2h",
	}
	state, err := scheduler.Update(context.Background(), jobB)
	if err != nil {
		t.Fatalf("Update(jobB) error = %v", err)
	}
	if state.NextRun == nil {
		t.Fatal("Update(jobB).NextRun = nil, want populated")
	}
	if got, want := *state.NextRun, baseTime.Add(2*time.Hour); !got.Equal(want) {
		t.Fatalf("Update(jobB).NextRun = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}

	states := scheduler.States()
	if got, want := len(states), 2; got != want {
		t.Fatalf("len(States()) = %d, want %d", got, want)
	}
	if got, want := states[0].JobID, "job-a"; got != want {
		t.Fatalf("States()[0].JobID = %q, want %q", got, want)
	}
	if got, want := states[1].JobID, "job-b"; got != want {
		t.Fatalf("States()[1].JobID = %q, want %q", got, want)
	}
}

func TestNormalizeScheduledJobAndPredictNextRunValidation(t *testing.T) {
	t.Parallel()

	if _, err := normalizeScheduledJob(Job{}); err == nil {
		t.Fatal("normalizeScheduledJob(empty) error = nil, want non-nil")
	}

	location := time.UTC
	registeredAt := time.Date(2026, 4, 10, 12, 0, 0, 0, location)
	invalidJob := testJob(AutomationScopeGlobal, "invalid-next-run", "")
	invalidJob.Schedule = &ScheduleSpec{
		Mode:     ScheduleModeEvery,
		Interval: "bad",
	}
	if got := predictNextRun(invalidJob, registeredAt, location); !got.IsZero() {
		t.Fatalf("predictNextRun(invalid every) = %s, want zero", got.Format(time.RFC3339))
	}
}

type stubScheduleDispatcher struct {
	mu             sync.Mutex
	calls          []DispatchRequest
	blocked        bool
	releaseCh      chan struct{}
	dispatchedCh   chan struct{}
	completedCh    chan struct{}
	dispatchResult error
	onDispatch     func(DispatchRequest)
}

func newStubScheduleDispatcher() *stubScheduleDispatcher {
	return &stubScheduleDispatcher{
		dispatchedCh: make(chan struct{}, 32),
		completedCh:  make(chan struct{}, 32),
	}
}

func (d *stubScheduleDispatcher) Dispatch(ctx context.Context, req DispatchRequest) (*Run, error) {
	defer notify(d.completedCh)

	d.mu.Lock()
	d.calls = append(d.calls, req)
	releaseCh := d.releaseCh
	blocked := d.blocked
	dispatchResult := d.dispatchResult
	onDispatch := d.onDispatch
	d.mu.Unlock()

	if onDispatch != nil {
		onDispatch(req)
	}
	notify(d.dispatchedCh)
	if blocked && releaseCh != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-releaseCh:
		}
	}
	if dispatchResult != nil {
		if req.ReservedRun != nil {
			return req.ReservedRun, dispatchResult
		}
		return nil, dispatchResult
	}
	if req.ReservedRun != nil {
		run := *req.ReservedRun
		run.Status = RunCompleted
		return &run, nil
	}
	return &Run{
		ID:     fmt.Sprintf("run-%d", d.count()),
		JobID:  req.Job.ID,
		Status: RunCompleted,
	}, nil
}

func (d *stubScheduleDispatcher) blockNextDispatch() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.blocked = true
	d.releaseCh = make(chan struct{})
}

func (d *stubScheduleDispatcher) releaseBlockedDispatch() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.releaseCh != nil {
		close(d.releaseCh)
	}
	d.blocked = false
	d.releaseCh = nil
}

func (d *stubScheduleDispatcher) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.calls)
}

func (d *stubScheduleDispatcher) callSnapshot() []DispatchRequest {
	d.mu.Lock()
	defer d.mu.Unlock()
	calls := make([]DispatchRequest, len(d.calls))
	copy(calls, d.calls)
	return calls
}

func (d *stubScheduleDispatcher) assertDispatchCount(t *testing.T, want int) {
	t.Helper()
	if got := d.count(); got != want {
		t.Fatalf("dispatch count = %d, want %d", got, want)
	}
}

func (d *stubScheduleDispatcher) waitForDispatchCount(t *testing.T, want int, timeout time.Duration) {
	t.Helper()

	deadline := time.After(timeout)
	for {
		if got := d.count(); got >= want {
			return
		}

		select {
		case <-deadline:
			t.Fatalf("dispatch count did not reach %d within %s; got %d", want, timeout, d.count())
		case <-d.dispatchedCh:
		}
	}
}

func (d *stubScheduleDispatcher) waitForCompletionCount(t *testing.T, want int, timeout time.Duration) {
	t.Helper()

	completed := 0
	deadline := time.After(timeout)
	for completed < want {
		select {
		case <-deadline:
			t.Fatalf("dispatch completion count did not reach %d within %s; got %d", want, timeout, completed)
		case <-d.completedCh:
			completed++
		}
	}
}

type memorySchedulerStore struct {
	mu              sync.Mutex
	states          map[string]SchedulerState
	runs            map[string]Run
	deliveryErrors  map[string]string
	stateCh         chan struct{}
	deliveryErrorCh chan struct{}
}

func newMemorySchedulerStore() *memorySchedulerStore {
	return &memorySchedulerStore{
		states:          make(map[string]SchedulerState),
		runs:            make(map[string]Run),
		deliveryErrors:  make(map[string]string),
		stateCh:         make(chan struct{}, 32),
		deliveryErrorCh: make(chan struct{}, 32),
	}
}

func (s *memorySchedulerStore) GetSchedulerState(_ context.Context, jobID string) (SchedulerState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.states[strings.TrimSpace(jobID)]
	if !ok {
		return SchedulerState{}, ErrSchedulerStateNotFound
	}
	return cloneSchedulerStateForTest(state), nil
}

func (s *memorySchedulerStore) SaveSchedulerState(
	_ context.Context,
	state SchedulerState,
) (SchedulerState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state.JobID = strings.TrimSpace(state.JobID)
	if state.CatchUpPolicy == "" {
		state.CatchUpPolicy = SchedulerCatchUpPolicySkipMissed
	}
	s.states[state.JobID] = cloneSchedulerStateForTest(state)
	notify(s.stateCh)
	return cloneSchedulerStateForTest(state), nil
}

func (s *memorySchedulerStore) ListSchedulerStates(_ context.Context) ([]SchedulerState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	states := make([]SchedulerState, 0, len(s.states))
	for _, state := range s.states {
		states = append(states, cloneSchedulerStateForTest(state))
	}
	return states, nil
}

func (s *memorySchedulerStore) DeleteSchedulerState(_ context.Context, jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, strings.TrimSpace(jobID))
	notify(s.stateCh)
	return nil
}

func (s *memorySchedulerStore) ClaimScheduledRun(
	_ context.Context,
	claim SchedulerClaim,
) (SchedulerClaimResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.states[claim.JobID]
	if current.LastFireID == claim.FireID {
		return SchedulerClaimResult{}, ErrScheduledFireAlreadyClaimed
	}
	skipReason := claim.SkipReason
	if skipReason == "" {
		for _, run := range s.runs {
			if run.JobID == claim.JobID && (run.Status == RunScheduled || run.Status == RunRunning) {
				skipReason = SchedulerSkipReasonSelfOverlap
				break
			}
		}
	}
	skipped := skipReason != ""
	next := current
	next.JobID = claim.JobID
	next.NextRunAt = cloneTimePointer(claim.NextRunAt)
	if !skipped {
		next.LastRunAt = timePointer(claim.ClaimedAt)
	}
	next.LastScheduledAt = timePointer(claim.ScheduledAt)
	next.LastFireID = claim.FireID
	next.ScheduleHash = claim.ScheduleHash
	next.CatchUpPolicy = schedulerCatchUpPolicyOrDefault(claim.CatchUpPolicy, current.CatchUpPolicy)
	next.MisfireGraceSeconds = claim.MisfireGraceSeconds
	if claim.CatchUpPolicy == "" && claim.MisfireGraceSeconds == 0 {
		next.MisfireGraceSeconds = current.MisfireGraceSeconds
	}
	if claim.Misfire {
		next.LastMisfireAt = timePointer(claim.ClaimedAt)
		next.MisfireCount++
	} else if !claim.CatchUp {
		next.LastMisfireAt = nil
		next.MisfireCount = 0
	}
	next.UpdatedAt = claim.ClaimedAt
	run := Run{
		ID:                   claim.RunID,
		JobID:                claim.JobID,
		FireID:               claim.FireID,
		Status:               RunScheduled,
		Attempt:              1,
		ScheduledAt:          timePointer(claim.ScheduledAt),
		StartedAt:            timePointer(claim.ClaimedAt),
		NetworkParticipation: cloneParticipationRequest(claim.NetworkParticipation),
	}
	if skipped {
		run.Status = RunCancelled
		run.EndedAt = timePointer(claim.ClaimedAt)
		run.Metadata = map[string]any{SchedulerSkipReasonMetadataKey: string(skipReason)}
	}
	s.states[claim.JobID] = cloneSchedulerStateForTest(next)
	s.runs[claim.RunID] = *cloneRun(&run)
	notify(s.stateCh)
	return SchedulerClaimResult{State: next, Run: run, Skipped: skipped, SkipReason: skipReason}, nil
}

func (s *memorySchedulerStore) setRunStatus(runID string, status RunStatus, endedAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.runs[strings.TrimSpace(runID)]
	run.Status = status
	run.EndedAt = timePointer(endedAt)
	s.runs[run.ID] = run
}

func (s *memorySchedulerStore) waitForNextRunAt(
	t *testing.T,
	jobID string,
	want time.Time,
	timeout time.Duration,
) {
	t.Helper()

	deadline := time.After(timeout)
	for {
		state, err := s.GetSchedulerState(context.Background(), jobID)
		if err == nil && state.NextRunAt != nil && state.NextRunAt.Equal(want) {
			return
		}

		select {
		case <-deadline:
			t.Fatalf("NextRunAt did not reach %s within %s; got %#v", want.Format(time.RFC3339), timeout, state)
		case <-s.stateCh:
		}
	}
}

func (s *memorySchedulerStore) RecordRunDeliveryError(
	_ context.Context,
	runID string,
	runErr error,
) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.runs[strings.TrimSpace(runID)]
	if runErr != nil {
		run.DeliveryError = runErr.Error()
		s.deliveryErrors[run.ID] = run.DeliveryError
		notify(s.deliveryErrorCh)
	}
	s.runs[run.ID] = run
	return run, nil
}

func (s *memorySchedulerStore) deliveryErrorForRun(runID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deliveryErrors[strings.TrimSpace(runID)]
}

func (s *memorySchedulerStore) waitForDeliveryError(t *testing.T, runID string, timeout time.Duration) string {
	t.Helper()

	deadline := time.After(timeout)
	for {
		if deliveryError := s.deliveryErrorForRun(runID); deliveryError != "" {
			return deliveryError
		}

		select {
		case <-deadline:
			return s.deliveryErrorForRun(runID)
		case <-s.deliveryErrorCh:
		}
	}
}

func (s *memorySchedulerStore) runsForJob(jobID string) []Run {
	s.mu.Lock()
	defer s.mu.Unlock()
	runs := make([]Run, 0, len(s.runs))
	for _, run := range s.runs {
		if run.JobID != strings.TrimSpace(jobID) {
			continue
		}
		runs = append(runs, *cloneRun(&run))
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].ScheduledAt == nil {
			return true
		}
		if runs[j].ScheduledAt == nil {
			return false
		}
		return runs[i].ScheduledAt.Before(*runs[j].ScheduledAt)
	})
	return runs
}

func cloneSchedulerStateForTest(state SchedulerState) SchedulerState {
	state.NextRunAt = cloneTimePointer(state.NextRunAt)
	state.LastRunAt = cloneTimePointer(state.LastRunAt)
	state.LastScheduledAt = cloneTimePointer(state.LastScheduledAt)
	state.LastMisfireAt = cloneTimePointer(state.LastMisfireAt)
	return state
}

func newTestScheduler(t *testing.T, dispatcher ScheduleDispatcher, opts ...SchedulerOption) *Scheduler {
	t.Helper()

	scheduler, err := NewScheduler(dispatcher, append([]SchedulerOption{
		WithSchedulerLogger(slog.Default()),
	}, opts...)...)
	if err != nil {
		t.Fatalf("NewScheduler() error = %v", err)
	}
	t.Cleanup(func() {
		if err := scheduler.Stop(context.Background()); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})
	return scheduler
}

func waitForTimers(t *testing.T, clock *clockwork.FakeClock, count int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
	defer cancel()
	if err := clock.BlockUntilContext(ctx, count); err != nil {
		t.Fatalf("BlockUntilContext() error = %v", err)
	}
}
