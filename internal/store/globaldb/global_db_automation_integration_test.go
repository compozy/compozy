//go:build integration

package globaldb

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGlobalDBAutomationPersistenceSurvivesReopen(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), GlobalDatabaseName)

	first, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(first) error = %v", err)
	}

	workspaceID := registerWorkspaceForGlobalTests(t, first, "automation-reopen-workspace", t.TempDir())
	job, err := first.CreateJob(
		ctx,
		automationJobForTest(
			automation.AutomationScopeWorkspace,
			"persisted-job",
			workspaceID,
			automation.JobSourceConfig,
		),
	)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	trigger, err := first.CreateTrigger(
		ctx,
		automationWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"persisted-trigger",
			workspaceID,
			automation.JobSourceConfig,
		),
	)
	if err != nil {
		t.Fatalf("CreateTrigger() error = %v", err)
	}
	if _, err := first.SetJobEnabledOverlay(ctx, JobEnabledOverlay{JobID: job.ID, EnabledOverride: false}); err != nil {
		t.Fatalf("SetJobEnabledOverlay() error = %v", err)
	}
	if _, err := first.SetTriggerEnabledOverlay(
		ctx,
		TriggerEnabledOverlay{TriggerID: trigger.ID, EnabledOverride: false},
	); err != nil {
		t.Fatalf("SetTriggerEnabledOverlay() error = %v", err)
	}

	runStartedAt := time.Date(2026, 4, 10, 19, 0, 0, 0, time.UTC)
	run, err := first.CreateRun(ctx, automationRunForJob(job.ID, automation.RunCompleted, 1, runStartedAt))
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	if err := first.Close(ctx); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	second, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(second) error = %v", err)
	}
	t.Cleanup(func() {
		if err := second.Close(ctx); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
	})

	jobs, err := second.ListJobs(ctx, JobListQuery{ReadScope: automationAllProfiles})
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if got, want := len(jobs.Jobs), 1; got != want {
		t.Fatalf("len(jobs) = %d, want %d", got, want)
	}

	reloadedTrigger, err := second.GetTriggerByWebhookID(ctx, trigger.WebhookID)
	if err != nil {
		t.Fatalf("GetTriggerByWebhookID() error = %v", err)
	}
	if got, want := reloadedTrigger.ID, trigger.ID; got != want {
		t.Fatalf("trigger.ID = %q, want %q", got, want)
	}

	jobOverlay, err := second.GetJobEnabledOverlay(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJobEnabledOverlay() error = %v", err)
	}
	if jobOverlay.EnabledOverride {
		t.Fatal("GetJobEnabledOverlay().EnabledOverride = true, want false")
	}
	triggerOverlay, err := second.GetTriggerEnabledOverlay(ctx, trigger.ID)
	if err != nil {
		t.Fatalf("GetTriggerEnabledOverlay() error = %v", err)
	}
	if triggerOverlay.EnabledOverride {
		t.Fatal("GetTriggerEnabledOverlay().EnabledOverride = true, want false")
	}

	storedRun, err := second.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if storedRun.StartedAt == nil || !storedRun.StartedAt.Equal(runStartedAt) {
		t.Fatalf("GetRun().StartedAt = %#v, want %v", storedRun.StartedAt, runStartedAt)
	}
}

func TestGlobalDBRunWindowQueriesSurviveReopen(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), GlobalDatabaseName)

	first, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(first) error = %v", err)
	}

	workspaceID := registerWorkspaceForGlobalTests(t, first, "automation-window-workspace", t.TempDir())
	job, err := first.CreateJob(
		ctx,
		automationJobForTest(
			automation.AutomationScopeWorkspace,
			"window-job",
			workspaceID,
			automation.JobSourceDynamic,
		),
	)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	base := time.Date(2026, 4, 10, 21, 0, 0, 0, time.UTC)
	for _, startedAt := range []time.Time{
		base.Add(-50 * time.Minute),
		base.Add(-10 * time.Minute),
		base.Add(-5 * time.Minute),
	} {
		if _, err := first.CreateRun(
			ctx,
			automationRunForJob(job.ID, automation.RunCompleted, 1, startedAt),
		); err != nil {
			t.Fatalf("CreateRun(%v) error = %v", startedAt, err)
		}
	}

	if err := first.Close(ctx); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	second, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(second) error = %v", err)
	}
	t.Cleanup(func() {
		if err := second.Close(ctx); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
	})

	count, err := second.CountRuns(ctx, RunQuery{ReadScope: automationAllProfiles,
		JobID: job.ID,
		Since: base.Add(-15 * time.Minute),
		Until: base,
	})
	if err != nil {
		t.Fatalf("CountRuns() error = %v", err)
	}
	if got, want := count, int64(2); got != want {
		t.Fatalf("CountRuns() = %d, want %d", got, want)
	}

	runs, err := second.ListRuns(ctx, RunQuery{ReadScope: automationAllProfiles,
		JobID: job.ID,
		Since: base.Add(-15 * time.Minute),
		Until: base,
	})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if got, want := len(runs), 2; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
}

func TestGlobalDBAutomationRunReservation(t *testing.T) {
	t.Run("Should reserve at most the fire limit across independent database handles", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		dbPath := filepath.Join(t.TempDir(), GlobalDatabaseName)
		first, err := OpenGlobalDB(ctx, dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}
		t.Cleanup(func() {
			if err := first.Close(ctx); err != nil {
				t.Fatalf("Close(first) error = %v", err)
			}
		})

		job, err := first.CreateJob(
			ctx,
			automationJobForTest(
				automation.AutomationScopeGlobal,
				"atomic-fire-limit-job",
				"",
				automation.JobSourceDynamic,
			),
		)
		if err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}

		second, err := OpenGlobalDB(ctx, dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(second) error = %v", err)
		}
		t.Cleanup(func() {
			if err := second.Close(ctx); err != nil {
				t.Fatalf("Close(second) error = %v", err)
			}
		})

		now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
		window := time.Hour
		statuses := []automation.RunStatus{
			automation.RunScheduled,
			automation.RunRunning,
			automation.RunDelegated,
			automation.RunCompleted,
			automation.RunFailed,
		}
		reservations := []automation.RunReservation{
			{
				Run: automation.Run{
					ID:        "run-atomic-fire-limit-first",
					JobID:     job.ID,
					Status:    automation.RunScheduled,
					Attempt:   1,
					StartedAt: timePointer(now),
				},
				Since:           now.Add(-window),
				Until:           now,
				Window:          window,
				Limit:           1,
				CountedStatuses: statuses,
			},
			{
				Run: automation.Run{
					ID:        "run-atomic-fire-limit-second",
					JobID:     job.ID,
					Status:    automation.RunScheduled,
					Attempt:   1,
					StartedAt: timePointer(now),
				},
				Since:           now.Add(-window),
				Until:           now,
				Window:          window,
				Limit:           1,
				CountedStatuses: statuses,
			},
		}

		type outcome struct {
			result automation.RunReservationResult
			err    error
		}
		start := make(chan struct{})
		outcomes := make(chan outcome, len(reservations))
		for index, db := range []*GlobalDB{first, second} {
			reservation := reservations[index]
			go func() {
				<-start
				result, reserveErr := db.ReserveRun(ctx, reservation)
				outcomes <- outcome{result: result, err: reserveErr}
			}()
		}
		close(start)

		var (
			reservedCount int
			rejected      automation.RunReservationResult
		)
		for range reservations {
			got := <-outcomes
			if got.err != nil {
				t.Fatalf("ReserveRun() error = %v", got.err)
			}
			if got.result.Reserved {
				reservedCount++
				continue
			}
			rejected = got.result
		}
		if got, want := reservedCount, 1; got != want {
			t.Fatalf("reserved count = %d, want %d", got, want)
		}
		if got, want := rejected.Count, int64(1); got != want {
			t.Fatalf("rejected.Count = %d, want %d", got, want)
		}
		if got, want := rejected.RetryAt, now.Add(window); !got.Equal(want) {
			t.Fatalf("rejected.RetryAt = %s, want %s", got, want)
		}

		runs, err := first.ListRuns(ctx, automation.RunQuery{ReadScope: automationAllProfiles,
			JobID: job.ID,
			Since: now.Add(-window),
			Until: now,
		})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("len(ListRuns()) = %d, want %d", got, want)
		}

		casJob, err := first.CreateJob(
			ctx,
			automationJobForTest(
				automation.AutomationScopeGlobal,
				"atomic-existing-reservation-job",
				"",
				automation.JobSourceDynamic,
			),
		)
		if err != nil {
			t.Fatalf("CreateJob(existing reservation) error = %v", err)
		}
		existing, err := first.CreateRun(ctx, automation.Run{
			ID:        "run-atomic-existing-reservation",
			JobID:     casJob.ID,
			Status:    automation.RunScheduled,
			Attempt:   1,
			StartedAt: timePointer(now),
		})
		if err != nil {
			t.Fatalf("CreateRun(existing reservation) error = %v", err)
		}
		existingReservation := automation.RunReservation{
			Run:             existing,
			ExistingRunID:   existing.ID,
			ExpectedStatus:  automation.RunScheduled,
			ExpectedAttempt: 1,
			Since:           now.Add(-window),
			Until:           now,
			Window:          window,
			Limit:           1,
			CountedStatuses: statuses,
		}

		start = make(chan struct{})
		outcomes = make(chan outcome, 2)
		for _, db := range []*GlobalDB{first, second} {
			go func() {
				<-start
				result, reserveErr := db.ReserveRun(ctx, existingReservation)
				outcomes <- outcome{result: result, err: reserveErr}
			}()
		}
		close(start)

		reservedCount = 0
		conflictCount := 0
		for range 2 {
			got := <-outcomes
			switch {
			case got.err == nil && got.result.Reserved:
				reservedCount++
			case errors.Is(got.err, automation.ErrRunReservationConflict):
				conflictCount++
			default:
				t.Fatalf("ReserveRun(existing) = (%#v, %v), want one reservation or conflict", got.result, got.err)
			}
		}
		if got, want := reservedCount, 1; got != want {
			t.Fatalf("existing reserved count = %d, want %d", got, want)
		}
		if got, want := conflictCount, 1; got != want {
			t.Fatalf("existing conflict count = %d, want %d", got, want)
		}

		activated, err := first.GetRun(ctx, existing.ID)
		if err != nil {
			t.Fatalf("GetRun(activated) error = %v", err)
		}
		if got, want := activated.Status, automation.RunRunning; got != want {
			t.Fatalf("activated.Status = %q, want %q", got, want)
		}
	})
}
