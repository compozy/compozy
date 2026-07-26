//go:build integration

package globaldb

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/testutil"
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

	jobs, err := second.ListJobs(ctx, JobListQuery{})
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

	count, err := second.CountRuns(ctx, RunQuery{
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

	runs, err := second.ListRuns(ctx, RunQuery{
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
