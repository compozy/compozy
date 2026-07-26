package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/automation"
	automationmodel "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/events"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

type Job = automation.Job
type JobListQuery = automation.JobListQuery
type Trigger = automation.Trigger
type TriggerListQuery = automation.TriggerListQuery
type Run = automation.Run
type RunQuery = automation.RunQuery
type SchedulerState = automation.SchedulerState
type SchedulerClaim = automation.SchedulerClaim
type JobEnabledOverlay = automation.JobEnabledOverlay
type TriggerEnabledOverlay = automation.TriggerEnabledOverlay

func automationNamedParticipation(channelID string) *participation.Request {
	mode := participation.ModeLive
	strategy := participation.StrategyNamed
	return &participation.Request{
		Mode:            &mode,
		ChannelStrategy: &strategy,
		ChannelID:       &channelID,
	}
}

func assertAutomationNamedParticipation(
	t *testing.T,
	request *participation.Request,
	channelID string,
) {
	t.Helper()
	if request == nil || request.Mode == nil || *request.Mode != participation.ModeLive {
		t.Fatalf("network participation = %#v, want live", request)
	}
	if request.ChannelStrategy == nil || *request.ChannelStrategy != participation.StrategyNamed {
		t.Fatalf("network participation strategy = %#v, want named", request.ChannelStrategy)
	}
	if request.ChannelID == nil || *request.ChannelID != channelID {
		t.Fatalf("network participation channel_id = %#v, want %q", request.ChannelID, channelID)
	}
}

func TestOpenGlobalDBCreatesAutomationSchemaAndIndexes(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	assertTablesPresent(
		t,
		globalDB.db,
		"automation_jobs",
		"automation_triggers",
		"automation_runs",
		"automation_watch_events",
		"automation_scheduler_state",
		"automation_job_overlays",
		"automation_trigger_overlays",
		"automation_suggestions",
	)
	assertTableColumns(t, globalDB.db, "automation_jobs", []string{
		"id",
		"scope",
		"name",
		"agent_name",
		"workspace_id",
		"prompt",
		"schedule",
		"task",
		"enabled",
		"retry",
		"fire_limit",
		"source",
		"target_kind",
		"loop_workspace_id",
		"loop_name",
		"loop_inputs",
		"loop_input_mapping",
		"loop_network_participation",
		"created_at",
		"updated_at",
	})
	assertTableColumns(t, globalDB.db, "automation_triggers", []string{
		"id",
		"scope",
		"name",
		"agent_name",
		"workspace_id",
		"prompt",
		"event",
		"filter",
		"enabled",
		"retry",
		"fire_limit",
		"source",
		"webhook_id",
		"endpoint_slug",
		"webhook_secret_ref",
		"target_kind",
		"loop_workspace_id",
		"loop_name",
		"loop_inputs",
		"loop_input_mapping",
		"loop_network_participation",
		"created_at",
		"updated_at",
	})
	assertTableColumns(t, globalDB.db, "automation_runs", []string{
		"id",
		"job_id",
		"trigger_id",
		"session_id",
		"task_id",
		"task_run_id",
		"status",
		"attempt",
		"started_at",
		"ended_at",
		"error",
		"loop_run_id",
		"fire_id",
		"scheduled_at",
		"delivery_error",
		"delivery_error_at",
		"network_participation",
		"metadata_json",
	})
	assertTableColumns(t, globalDB.db, "automation_scheduler_state", []string{
		"job_id",
		"next_run_at",
		"last_run_at",
		"last_scheduled_at",
		"last_fire_id",
		"schedule_hash",
		"catch_up_policy",
		"misfire_grace_seconds",
		"consecutive_resume_failures",
		"last_misfire_at",
		"misfire_count",
		"updated_at",
	})
	assertIndexesPresent(t, globalDB.db, "automation_jobs",
		"uq_automation_jobs_global_name",
		"uq_automation_jobs_workspace_name",
		"idx_automation_jobs_enabled",
		"idx_automation_jobs_loop_target",
	)
	assertIndexesPresent(t, globalDB.db, "automation_triggers",
		"uq_automation_triggers_global_name",
		"uq_automation_triggers_workspace_name",
		"uq_automation_triggers_webhook_id",
		"idx_automation_triggers_enabled",
		"idx_automation_triggers_event",
		"idx_automation_triggers_loop_target",
	)
	assertIndexesPresent(t, globalDB.db, "automation_runs",
		"idx_automation_runs_job",
		"idx_automation_runs_trigger",
		"idx_automation_runs_loop_run",
		"idx_automation_runs_status",
		"idx_automation_runs_started",
		"uq_automation_runs_fire_id",
	)
	assertIndexesPresent(
		t,
		globalDB.db,
		"automation_watch_events",
		"idx_automation_watch_events_workspace_status_seq",
	)
	assertIndexesPresent(t, globalDB.db, "automation_scheduler_state",
		"idx_automation_scheduler_next_run",
		"idx_automation_scheduler_misfire",
	)
	assertTableColumns(t, globalDB.db, "automation_suggestions", []string{
		"id",
		"workspace_id",
		"source",
		"dedup_key",
		"status",
		"payload",
		"created_at",
		"resolved_at",
	})
	assertIndexesPresent(
		t,
		globalDB.db,
		"automation_suggestions",
		"automation_suggestions_workspace_id_dedup_key",
		"idx_automation_suggestions_workspace_status",
	)
	assertTableSQLContains(
		t,
		globalDB.db,
		"automation_scheduler_state",
		"'skip_missed', 'coalesce', 'replay', 'run_once_on_catchup'",
	)
}

func TestGlobalDBSchedulerCatchUpPolicyMigration(t *testing.T) {
	t.Run("Should preserve scheduler state while hard-cutting catch-up policy values", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		previousDB, err := openGlobalMigrationPrefixDatabase(t, path, previousAutomationMigrationStream(t))
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(previous schema) error = %v", err)
		}
		ctx := testutil.Context(t)
		updatedAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
		if _, err := previousDB.ExecContext(
			ctx,
			`INSERT INTO automation_scheduler_state (
			job_id, last_fire_id, schedule_hash, catch_up_policy,
			misfire_grace_seconds, misfire_count, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			"job-migration-cursor",
			"fire-before-upgrade",
			"hash-before-upgrade",
			"skip",
			45,
			3,
			store.FormatTimestamp(updatedAt),
		); err != nil {
			t.Fatalf("seed previous scheduler state error = %v", err)
		}
		if err := previousDB.Close(); err != nil {
			t.Fatalf("previousDB.Close() error = %v", err)
		}

		globalDB, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(upgrade) error = %v", err)
		}
		ctx = testutil.Context(t)
		t.Cleanup(func() {
			if err := globalDB.Close(ctx); err != nil {
				t.Errorf("GlobalDB.Close() error = %v", err)
			}
		})

		state, err := globalDB.GetSchedulerState(ctx, "job-migration-cursor")
		if err != nil {
			t.Fatalf("GetSchedulerState() after upgrade error = %v", err)
		}
		if got, want := state.CatchUpPolicy, automation.SchedulerCatchUpPolicySkipMissed; got != want {
			t.Fatalf("GetSchedulerState().CatchUpPolicy = %q, want %q", got, want)
		}
		if got, want := state.LastFireID, "fire-before-upgrade"; got != want {
			t.Fatalf("GetSchedulerState().LastFireID = %q, want %q", got, want)
		}
		if got, want := state.MisfireGraceSeconds, 45; got != want {
			t.Fatalf("GetSchedulerState().MisfireGraceSeconds = %d, want %d", got, want)
		}
		if got, want := state.MisfireCount, 3; got != want {
			t.Fatalf("GetSchedulerState().MisfireCount = %d, want %d", got, want)
		}

		state.CatchUpPolicy = automation.SchedulerCatchUpPolicyRunOnce
		state.UpdatedAt = updatedAt.Add(time.Minute)
		if _, err := globalDB.SaveSchedulerState(ctx, state); err != nil {
			t.Fatalf("SaveSchedulerState(run once policy) error = %v", err)
		}
		status, err := store.Status(ctx, globalDB.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(global) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
	})
}

func previousAutomationMigrationStream(t *testing.T) store.MigrationStream {
	t.Helper()

	return globalMigrationPrefix(t,
		"00001_baseline.sql",
		"00002_schema.sql",
		"00003_schema.sql",
		"00004_schema.sql",
		"00005_schema.sql",
		"00006_schema.sql",
		"00007_schema.sql",
		"00008_network_participation.sql",
		"00009_automation_network_participation.sql",
		"00010_network_participation_hardening.sql",
		"00011_schema.sql",
		"00012_schema.sql",
		"00013_network_subscription_hard_cut.sql",
		"00014_network_task_status_projections.sql",
	)
}

func TestGlobalDBCreateJobScopeAwareUniqueness(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "automation-uniqueness-a", t.TempDir())
	workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "automation-uniqueness-b", t.TempDir())

	if _, err := globalDB.CreateJob(
		testutil.Context(t),
		automationJobForTest(automation.AutomationScopeGlobal, "daily-report", "", automation.JobSourceDynamic),
	); err != nil {
		t.Fatalf("CreateJob(global) error = %v", err)
	}
	if _, err := globalDB.CreateJob(
		testutil.Context(t),
		automationJobForTest(
			automation.AutomationScopeWorkspace,
			"daily-report",
			workspaceA,
			automation.JobSourceDynamic,
		),
	); err != nil {
		t.Fatalf("CreateJob(workspace same name) error = %v", err)
	}
	if _, err := globalDB.CreateJob(
		testutil.Context(t),
		automationJobForTest(
			automation.AutomationScopeWorkspace,
			"daily-report",
			workspaceB,
			automation.JobSourceDynamic,
		),
	); err != nil {
		t.Fatalf("CreateJob(second workspace same name) error = %v", err)
	}

	if _, err := globalDB.CreateJob(
		testutil.Context(t),
		automationJobForTest(automation.AutomationScopeGlobal, "daily-report", "", automation.JobSourceDynamic),
	); !errors.Is(
		err,
		automation.ErrJobNameTaken,
	) {
		t.Fatalf("CreateJob(duplicate global) error = %v, want ErrJobNameTaken", err)
	}
	if _, err := globalDB.CreateJob(
		testutil.Context(t),
		automationJobForTest(
			automation.AutomationScopeWorkspace,
			"daily-report",
			workspaceA,
			automation.JobSourceDynamic,
		),
	); !errors.Is(
		err,
		automation.ErrJobNameTaken,
	) {
		t.Fatalf("CreateJob(duplicate workspace) error = %v, want ErrJobNameTaken", err)
	}
}

func TestGlobalDBGetTriggerByWebhookIDUsesStableID(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	trigger := automationWebhookTriggerForTest(
		automation.AutomationScopeGlobal,
		"deploy-review",
		"",
		automation.JobSourceDynamic,
	)
	trigger.WebhookID = "wbh_stable-001"
	trigger.EndpointSlug = "deploy-review-v1"

	created, err := globalDB.CreateTrigger(testutil.Context(t), trigger)
	if err != nil {
		t.Fatalf("CreateTrigger() error = %v", err)
	}

	created.EndpointSlug = "deploy-review-renamed"
	updated, err := globalDB.UpdateTrigger(testutil.Context(t), created)
	if err != nil {
		t.Fatalf("UpdateTrigger() error = %v", err)
	}

	lookedUp, err := globalDB.GetTriggerByWebhookID(testutil.Context(t), "wbh_stable-001")
	if err != nil {
		t.Fatalf("GetTriggerByWebhookID() error = %v", err)
	}
	if got, want := lookedUp.ID, updated.ID; got != want {
		t.Fatalf("trigger.ID = %q, want %q", got, want)
	}
	if got, want := lookedUp.EndpointSlug, "deploy-review-renamed"; got != want {
		t.Fatalf("trigger.EndpointSlug = %q, want %q", got, want)
	}
	if got, want := lookedUp.WebhookID, "wbh_stable-001"; got != want {
		t.Fatalf("trigger.WebhookID = %q, want %q", got, want)
	}
}

func TestGlobalDBTriggerWebhookSecretRefPersistsWithoutLegacyTable(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	trigger := automationWebhookTriggerForTest(
		automation.AutomationScopeGlobal,
		"deploy-review",
		"",
		automation.JobSourceDynamic,
	)
	trigger.WebhookSecretRef = "vault:automation/triggers/deploy-review/webhook-secret"

	created, err := globalDB.CreateTrigger(testutil.Context(t), trigger)
	if err != nil {
		t.Fatalf("CreateTrigger() error = %v", err)
	}
	got, err := globalDB.GetTrigger(testutil.Context(t), created.ID)
	if err != nil {
		t.Fatalf("GetTrigger() error = %v", err)
	}
	if got, want := got.WebhookSecretRef, trigger.WebhookSecretRef; got != want {
		t.Fatalf("trigger.WebhookSecretRef = %q, want %q", got, want)
	}

	var legacyTableCount int
	if err := globalDB.db.QueryRowContext(
		testutil.Context(t),
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'automation_trigger_webhook_secrets'`,
	).Scan(&legacyTableCount); err != nil {
		t.Fatalf("QueryRowContext(legacy automation secret table) error = %v", err)
	}
	if legacyTableCount != 0 {
		t.Fatalf("legacy automation_trigger_webhook_secrets table exists, want absent")
	}
}

func TestGlobalDBJobEnabledOverlayDoesNotMutateDefinition(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-overlay-workspace", t.TempDir())

	configJob := automationJobForTest(
		automation.AutomationScopeWorkspace,
		"config-job",
		workspaceID,
		automation.JobSourceConfig,
	)
	configJob.Enabled = true
	created, err := globalDB.CreateJob(testutil.Context(t), configJob)
	if err != nil {
		t.Fatalf("CreateJob(config) error = %v", err)
	}

	overlay, err := globalDB.SetJobEnabledOverlay(testutil.Context(t), JobEnabledOverlay{
		JobID:           created.ID,
		EnabledOverride: false,
	})
	if err != nil {
		t.Fatalf("SetJobEnabledOverlay() error = %v", err)
	}

	reloaded, err := globalDB.GetJob(testutil.Context(t), created.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if !reloaded.Enabled {
		t.Fatal("GetJob().Enabled = false, want definition payload unchanged")
	}

	storedOverlay, err := globalDB.GetJobEnabledOverlay(testutil.Context(t), created.ID)
	if err != nil {
		t.Fatalf("GetJobEnabledOverlay() error = %v", err)
	}
	if storedOverlay.EnabledOverride {
		t.Fatal("GetJobEnabledOverlay().EnabledOverride = true, want false")
	}
	if overlay.UpdatedAt.IsZero() {
		t.Fatal("SetJobEnabledOverlay().UpdatedAt = zero, want populated")
	}

	overlays, err := globalDB.ListJobEnabledOverlays(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListJobEnabledOverlays() error = %v", err)
	}
	if got, want := len(overlays), 1; got != want {
		t.Fatalf("len(ListJobEnabledOverlays()) = %d, want %d", got, want)
	}

	if _, err := globalDB.SetJobEnabledOverlay(testutil.Context(t), JobEnabledOverlay{
		JobID:           "resource-backed-job",
		EnabledOverride: false,
	}); err != nil {
		t.Fatalf("SetJobEnabledOverlay(resource-backed) error = %v", err)
	}

	if err := globalDB.DeleteJobEnabledOverlay(testutil.Context(t), created.ID); err != nil {
		t.Fatalf("DeleteJobEnabledOverlay() error = %v", err)
	}
	if _, err := globalDB.GetJobEnabledOverlay(
		testutil.Context(t),
		created.ID,
	); !errors.Is(
		err,
		automation.ErrJobOverlayNotFound,
	) {
		t.Fatalf("GetJobEnabledOverlay(after delete) error = %v, want ErrJobOverlayNotFound", err)
	}
}

func TestGlobalDBTriggerEnabledOverlayDoesNotMutateDefinition(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-trigger-overlay-workspace", t.TempDir())

	configTrigger := automationWebhookTriggerForTest(
		automation.AutomationScopeWorkspace,
		"config-trigger",
		workspaceID,
		automation.JobSourceConfig,
	)
	configTrigger.Enabled = true
	created, err := globalDB.CreateTrigger(testutil.Context(t), configTrigger)
	if err != nil {
		t.Fatalf("CreateTrigger(config) error = %v", err)
	}

	if _, err := globalDB.SetTriggerEnabledOverlay(testutil.Context(t), TriggerEnabledOverlay{
		TriggerID:       created.ID,
		EnabledOverride: false,
	}); err != nil {
		t.Fatalf("SetTriggerEnabledOverlay() error = %v", err)
	}

	reloaded, err := globalDB.GetTrigger(testutil.Context(t), created.ID)
	if err != nil {
		t.Fatalf("GetTrigger() error = %v", err)
	}
	if !reloaded.Enabled {
		t.Fatal("GetTrigger().Enabled = false, want definition payload unchanged")
	}

	storedOverlay, err := globalDB.GetTriggerEnabledOverlay(testutil.Context(t), created.ID)
	if err != nil {
		t.Fatalf("GetTriggerEnabledOverlay() error = %v", err)
	}
	if storedOverlay.EnabledOverride {
		t.Fatal("GetTriggerEnabledOverlay().EnabledOverride = true, want false")
	}

	overlays, err := globalDB.ListTriggerEnabledOverlays(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListTriggerEnabledOverlays() error = %v", err)
	}
	if got, want := len(overlays), 1; got != want {
		t.Fatalf("len(ListTriggerEnabledOverlays()) = %d, want %d", got, want)
	}

	if _, err := globalDB.SetTriggerEnabledOverlay(testutil.Context(t), TriggerEnabledOverlay{
		TriggerID:       "resource-backed-trigger",
		EnabledOverride: false,
	}); err != nil {
		t.Fatalf("SetTriggerEnabledOverlay(resource-backed) error = %v", err)
	}

	if err := globalDB.DeleteTriggerEnabledOverlay(testutil.Context(t), created.ID); err != nil {
		t.Fatalf("DeleteTriggerEnabledOverlay() error = %v", err)
	}
	if _, err := globalDB.GetTriggerEnabledOverlay(
		testutil.Context(t),
		created.ID,
	); !errors.Is(
		err,
		automation.ErrTriggerOverlayNotFound,
	) {
		t.Fatalf("GetTriggerEnabledOverlay(after delete) error = %v, want ErrTriggerOverlayNotFound", err)
	}
}

func TestGlobalDBTriggerUniquenessAndWebhookIDConstraints(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-trigger-constraints", t.TempDir())

	globalWebhook := automationWebhookTriggerForTest(
		automation.AutomationScopeGlobal,
		"deploy-review",
		"",
		automation.JobSourceDynamic,
	)
	globalWebhook.WebhookID = "wbh_stable-webhook-a"
	if _, err := globalDB.CreateTrigger(testutil.Context(t), globalWebhook); err != nil {
		t.Fatalf("CreateTrigger(globalWebhook) error = %v", err)
	}

	workspaceTrigger := automationWebhookTriggerForTest(
		automation.AutomationScopeWorkspace,
		"deploy-review",
		workspaceID,
		automation.JobSourceDynamic,
	)
	workspaceTrigger.WebhookID = "wbh_stable-webhook-b"
	if _, err := globalDB.CreateTrigger(testutil.Context(t), workspaceTrigger); err != nil {
		t.Fatalf("CreateTrigger(workspace same name) error = %v", err)
	}

	duplicateName := automationWebhookTriggerForTest(
		automation.AutomationScopeGlobal,
		"deploy-review",
		"",
		automation.JobSourceDynamic,
	)
	duplicateName.WebhookID = "wbh_stable-webhook-c"
	if _, err := globalDB.CreateTrigger(
		testutil.Context(t),
		duplicateName,
	); !errors.Is(
		err,
		automation.ErrTriggerNameTaken,
	) {
		t.Fatalf("CreateTrigger(duplicate name) error = %v, want ErrTriggerNameTaken", err)
	}

	duplicateWebhook := automationWebhookTriggerForTest(
		automation.AutomationScopeWorkspace,
		"another-name",
		workspaceID,
		automation.JobSourceDynamic,
	)
	duplicateWebhook.WebhookID = "wbh_stable-webhook-a"
	if _, err := globalDB.CreateTrigger(
		testutil.Context(t),
		duplicateWebhook,
	); !errors.Is(
		err,
		automation.ErrTriggerWebhookIDTaken,
	) {
		t.Fatalf("CreateTrigger(duplicate webhook) error = %v, want ErrTriggerWebhookIDTaken", err)
	}

	filtered, err := globalDB.ListTriggers(testutil.Context(t), TriggerListQuery{
		Scope:  automation.AutomationScopeWorkspace,
		Event:  "webhook",
		Source: automation.JobSourceDynamic,
	})
	if err != nil {
		t.Fatalf("ListTriggers(filtered) error = %v", err)
	}
	if got, want := len(filtered.Triggers), 1; got != want {
		t.Fatalf("len(ListTriggers(filtered)) = %d, want %d", got, want)
	}
}

func TestGlobalDBAutomationListsSearchPageAndIsolateWorkspaces(t *testing.T) {
	t.Parallel()

	t.Run("Should update and delete catalog projections with rich definitions", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		job, err := globalDB.CreateJob(ctx, automationJobForTest(
			automation.AutomationScopeGlobal,
			"projection-job",
			"",
			automation.JobSourceDynamic,
		))
		if err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}
		job.Schedule.Interval = "45m"
		if _, err := globalDB.UpdateJob(ctx, job); err != nil {
			t.Fatalf("UpdateJob() error = %v", err)
		}
		jobs, err := globalDB.ListJobs(ctx, JobListQuery{Search: "45M", Limit: 10})
		if err != nil {
			t.Fatalf("ListJobs(updated schedule) error = %v", err)
		}
		if len(jobs.Jobs) != 1 || jobs.Jobs[0].ID != job.ID {
			t.Fatalf("ListJobs(updated schedule) = %#v, want updated job", jobs)
		}

		trigger := automationNonWebhookTriggerForTest(
			automation.AutomationScopeGlobal,
			"projection-trigger",
			"",
			automation.JobSourceDynamic,
		)
		trigger, err = globalDB.CreateTrigger(ctx, trigger)
		if err != nil {
			t.Fatalf("CreateTrigger() error = %v", err)
		}
		trigger.Filter = map[string]string{"data.team": "projection-updated"}
		if _, err := globalDB.UpdateTrigger(ctx, trigger); err != nil {
			t.Fatalf("UpdateTrigger() error = %v", err)
		}
		triggers, err := globalDB.ListTriggers(ctx, TriggerListQuery{
			Search: "PROJECTION-UPDATED",
			Limit:  10,
		})
		if err != nil {
			t.Fatalf("ListTriggers(updated filter) error = %v", err)
		}
		if len(triggers.Triggers) != 1 || triggers.Triggers[0].ID != trigger.ID {
			t.Fatalf("ListTriggers(updated filter) = %#v, want updated trigger", triggers)
		}
		if err := globalDB.DeleteJob(ctx, job.ID); err != nil {
			t.Fatalf("DeleteJob() error = %v", err)
		}
		if err := globalDB.DeleteTrigger(ctx, trigger.ID); err != nil {
			t.Fatalf("DeleteTrigger() error = %v", err)
		}
		jobs, err = globalDB.ListJobs(ctx, JobListQuery{Search: "45m", Limit: 10})
		if err != nil {
			t.Fatalf("ListJobs(after delete) error = %v", err)
		}
		triggers, err = globalDB.ListTriggers(ctx, TriggerListQuery{
			Search: "projection-updated",
			Limit:  10,
		})
		if err != nil {
			t.Fatalf("ListTriggers(after delete) error = %v", err)
		}
		if jobs.Total != 0 || triggers.Total != 0 {
			t.Fatalf("catalog totals after delete = jobs %d triggers %d, want zero", jobs.Total, triggers.Total)
		}
	})

	t.Run("Should walk unfiltered job and trigger cursors", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		for index := range 3 {
			if _, err := globalDB.CreateJob(ctx, automationJobForTest(
				automation.AutomationScopeGlobal,
				fmt.Sprintf("unfiltered-job-%d", index),
				"",
				automation.JobSourceDynamic,
			)); err != nil {
				t.Fatalf("CreateJob(%d) error = %v", index, err)
			}
			if _, err := globalDB.CreateTrigger(ctx, automationNonWebhookTriggerForTest(
				automation.AutomationScopeGlobal,
				fmt.Sprintf("unfiltered-trigger-%d", index),
				"",
				automation.JobSourceDynamic,
			)); err != nil {
				t.Fatalf("CreateTrigger(%d) error = %v", index, err)
			}
		}
		jobs, err := globalDB.ListJobs(ctx, JobListQuery{Limit: 1})
		if err != nil {
			t.Fatalf("ListJobs(first) error = %v", err)
		}
		firstJobID := jobs.Jobs[0].ID
		jobs, err = globalDB.ListJobs(ctx, JobListQuery{Limit: 1, Cursor: jobs.NextCursor})
		if err != nil {
			t.Fatalf("ListJobs(second) error = %v", err)
		}
		if len(jobs.Jobs) != 1 || jobs.Jobs[0].ID == firstJobID {
			t.Fatalf("ListJobs(second).Jobs = %#v, want one different job", jobs.Jobs)
		}
		triggers, err := globalDB.ListTriggers(ctx, TriggerListQuery{Limit: 1})
		if err != nil {
			t.Fatalf("ListTriggers(first) error = %v", err)
		}
		firstTriggerID := triggers.Triggers[0].ID
		triggers, err = globalDB.ListTriggers(
			ctx,
			TriggerListQuery{Limit: 1, Cursor: triggers.NextCursor},
		)
		if err != nil {
			t.Fatalf("ListTriggers(second) error = %v", err)
		}
		if len(triggers.Triggers) != 1 || triggers.Triggers[0].ID == firstTriggerID {
			t.Fatalf("ListTriggers(second).Triggers = %#v, want one different trigger", triggers.Triggers)
		}
	})

	t.Run("Should page effective jobs before hydrating rich records", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-catalog-jobs", t.TempDir())
		otherWorkspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"automation-catalog-jobs-other",
			t.TempDir(),
		)
		ctx := testutil.Context(t)
		jobsToSeed := make([]Job, 0, 203)
		for index := range 202 {
			jobsToSeed = append(jobsToSeed, automationJobForTest(
				automation.AutomationScopeWorkspace,
				fmt.Sprintf("needle-job-%03d", index),
				workspaceID,
				automation.JobSourceConfig,
			))
		}
		jobsToSeed = append(jobsToSeed, automationJobForTest(
			automation.AutomationScopeWorkspace,
			"needle-job-foreign",
			otherWorkspaceID,
			automation.JobSourceConfig,
		))
		seeded := seedAutomationCatalogDefinitionsForTest(t, globalDB, jobsToSeed, nil)
		expected := seeded.jobs[:202]
		foreign := seeded.jobs[202]
		if _, err := globalDB.SetJobEnabledOverlay(ctx, JobEnabledOverlay{
			JobID:           expected[10].ID,
			EnabledOverride: false,
		}); err != nil {
			t.Fatalf("SetJobEnabledOverlay() error = %v", err)
		}
		expected = append(expected[:10], expected[11:]...)

		poison := strings.Repeat("{", 1<<20)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE automation_jobs SET retry = ?, task = ? WHERE id = ?`,
			poison,
			poison,
			expected[len(expected)-1].ID,
		); err != nil {
			t.Fatalf("poison off-page job rich fields error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE automation_jobs SET retry = ? WHERE id = ?`,
			poison,
			foreign.ID,
		); err != nil {
			t.Fatalf("poison foreign job rich fields error = %v", err)
		}

		automation.SortJobsForList(expected)
		enabled := true
		query := JobListQuery{
			Scope:       automation.AutomationScopeWorkspace,
			WorkspaceID: workspaceID,
			Enabled:     &enabled,
			Search:      "NEEDLE",
			Limit:       5,
		}
		page, err := globalDB.ListJobs(ctx, query)
		if err != nil {
			t.Fatalf("ListJobs(first page) error = %v", err)
		}
		if got, want := page.Total, len(expected); got != want {
			t.Fatalf("ListJobs().Total = %d, want %d", got, want)
		}
		if got, want := len(page.Jobs), query.Limit; got != want {
			t.Fatalf("len(ListJobs().Jobs) = %d, want %d", got, want)
		}
		if !page.HasMore || page.NextCursor == "" {
			t.Fatalf("ListJobs() pagination = %#v, want another page", page)
		}
		for index, job := range page.Jobs {
			if got, want := job.ID, expected[index].ID; got != want {
				t.Fatalf("ListJobs().Jobs[%d].ID = %q, want %q", index, got, want)
			}
		}
		counting := &countingAutomationCatalogExecutor{automationCatalogExecutor: globalDB.db}
		countingQuery := query
		countingQuery.Limit = automation.MaxListLimit
		if _, err := listAutomationJobCatalog(
			ctx,
			counting,
			automationmodel.JobListQueryWithDefaultLimit(countingQuery),
		); err != nil {
			t.Fatalf("listAutomationJobCatalog(counting) error = %v", err)
		}
		assertAutomationCatalogReadCardinality(t, counting, automation.MaxListLimit)

		query.Cursor = page.NextCursor
		next, err := globalDB.ListJobs(ctx, query)
		if err != nil {
			t.Fatalf("ListJobs(second page) error = %v", err)
		}
		if got, want := next.Jobs[0].ID, expected[query.Limit].ID; got != want {
			t.Fatalf("ListJobs(second page).Jobs[0].ID = %q, want %q", got, want)
		}
	})

	t.Run("Should search trigger filters before hydrating rich records", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-catalog-triggers", t.TempDir())
		otherWorkspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"automation-catalog-triggers-other",
			t.TempDir(),
		)
		ctx := testutil.Context(t)
		triggersToSeed := make([]Trigger, 0, 5)
		for index := range 4 {
			trigger := automationNonWebhookTriggerForTest(
				automation.AutomationScopeWorkspace,
				fmt.Sprintf("trigger-%03d", index),
				workspaceID,
				automation.JobSourceConfig,
			)
			trigger.Filter["data.team"] = "operations-needle"
			triggersToSeed = append(triggersToSeed, trigger)
		}
		foreign := automationNonWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"trigger-foreign",
			otherWorkspaceID,
			automation.JobSourceConfig,
		)
		foreign.Filter["data.team"] = "operations-needle"
		triggersToSeed = append(triggersToSeed, foreign)
		seeded := seedAutomationCatalogDefinitionsForTest(t, globalDB, nil, triggersToSeed)
		expected := seeded.triggers[:4]
		foreign = seeded.triggers[4]
		if _, err := globalDB.SetTriggerEnabledOverlay(ctx, TriggerEnabledOverlay{
			TriggerID:       expected[3].ID,
			EnabledOverride: false,
		}); err != nil {
			t.Fatalf("SetTriggerEnabledOverlay() error = %v", err)
		}
		expected = expected[:3]

		poison := strings.Repeat("{", 1<<20)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE automation_triggers SET retry = ?, loop_inputs = ? WHERE id = ?`,
			poison,
			poison,
			expected[len(expected)-1].ID,
		); err != nil {
			t.Fatalf("poison off-page trigger rich fields error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE automation_triggers SET retry = ? WHERE id = ?`,
			poison,
			foreign.ID,
		); err != nil {
			t.Fatalf("poison foreign trigger rich fields error = %v", err)
		}

		automation.SortTriggersForList(expected)
		enabled := true
		query := TriggerListQuery{
			Scope:       automation.AutomationScopeWorkspace,
			WorkspaceID: workspaceID,
			Enabled:     &enabled,
			Search:      "OPERATIONS-NEEDLE",
			Limit:       1,
		}
		page, err := globalDB.ListTriggers(ctx, query)
		if err != nil {
			t.Fatalf("ListTriggers(first page) error = %v", err)
		}
		if got, want := page.Total, len(expected); got != want {
			t.Fatalf("ListTriggers().Total = %d, want %d", got, want)
		}
		if got, want := len(page.Triggers), query.Limit; got != want {
			t.Fatalf("len(ListTriggers().Triggers) = %d, want %d", got, want)
		}
		if !page.HasMore || page.NextCursor == "" {
			t.Fatalf("ListTriggers() pagination = %#v, want another page", page)
		}
		for index, trigger := range page.Triggers {
			if got, want := trigger.ID, expected[index].ID; got != want {
				t.Fatalf("ListTriggers().Triggers[%d].ID = %q, want %q", index, got, want)
			}
		}
		counting := &countingAutomationCatalogExecutor{automationCatalogExecutor: globalDB.db}
		countingQuery := query
		countingQuery.Limit = 2
		if _, err := listAutomationTriggerCatalog(
			ctx,
			counting,
			automationmodel.TriggerListQueryWithDefaultLimit(countingQuery),
		); err != nil {
			t.Fatalf("listAutomationTriggerCatalog(counting) error = %v", err)
		}
		assertAutomationCatalogReadCardinality(t, counting, 2)

		query.Cursor = page.NextCursor
		next, err := globalDB.ListTriggers(ctx, query)
		if err != nil {
			t.Fatalf("ListTriggers(second page) error = %v", err)
		}
		if got, want := next.Triggers[0].ID, expected[query.Limit].ID; got != want {
			t.Fatalf("ListTriggers(second page).Triggers[0].ID = %q, want %q", got, want)
		}
	})

	t.Run("Should page matching jobs without gaps or workspace leaks", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-list-jobs", t.TempDir())
		otherWorkspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-list-jobs-other", t.TempDir())
		ctx := testutil.Context(t)
		sources := []automation.JobSource{
			automation.JobSourceDynamic,
			automation.JobSourcePackage,
			automation.JobSourceConfig,
		}
		expected := make([]Job, 0, 17)
		for index := range 17 {
			job := automationJobForTest(
				automation.AutomationScopeWorkspace,
				fmt.Sprintf("needle-job-%02d", 16-index),
				workspaceID,
				sources[index%len(sources)],
			)
			created, err := globalDB.CreateJob(ctx, job)
			if err != nil {
				t.Fatalf("CreateJob(%d) error = %v", index, err)
			}
			expected = append(expected, created)
		}
		disabled := automationJobForTest(
			automation.AutomationScopeWorkspace,
			"needle-job-disabled",
			workspaceID,
			automation.JobSourceConfig,
		)
		disabled.Enabled = false
		if _, err := globalDB.CreateJob(ctx, disabled); err != nil {
			t.Fatalf("CreateJob(disabled) error = %v", err)
		}
		if _, err := globalDB.CreateJob(ctx, automationJobForTest(
			automation.AutomationScopeWorkspace,
			"needle-job-other-workspace",
			otherWorkspaceID,
			automation.JobSourceConfig,
		)); err != nil {
			t.Fatalf("CreateJob(other workspace) error = %v", err)
		}

		automation.SortJobsForList(expected)
		enabled := true
		query := JobListQuery{
			Scope:       automation.AutomationScopeWorkspace,
			WorkspaceID: workspaceID,
			Enabled:     &enabled,
			Search:      "  NeEdLe  ",
			Limit:       5,
		}
		walkedIDs := make([]string, 0, len(expected))
		for {
			page, err := globalDB.ListJobs(ctx, query)
			if err != nil {
				t.Fatalf("ListJobs(cursor=%q) error = %v", query.Cursor, err)
			}
			if got, want := page.Total, len(expected); got != want {
				t.Fatalf("ListJobs().Total = %d, want %d", got, want)
			}
			for _, job := range page.Jobs {
				walkedIDs = append(walkedIDs, job.ID)
			}
			if !page.HasMore {
				break
			}
			if page.NextCursor == "" {
				t.Fatal("ListJobs().NextCursor = empty while HasMore is true")
			}
			query.Cursor = page.NextCursor
		}
		if got, want := len(walkedIDs), len(expected); got != want {
			t.Fatalf("walked job count = %d, want %d", got, want)
		}
		for index, job := range expected {
			if got, want := walkedIDs[index], job.ID; got != want {
				t.Fatalf("walked job %d = %q, want %q", index, got, want)
			}
		}
		query.Search = "different"
		if _, err := globalDB.ListJobs(ctx, query); !errors.Is(err, automation.ErrListCursorInvalid) {
			t.Fatalf("ListJobs(changed cursor query) error = %v, want ErrListCursorInvalid", err)
		}
	})

	t.Run("Should search trigger filter values before paging", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-list-triggers", t.TempDir())
		otherWorkspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-list-triggers-other", t.TempDir())
		ctx := testutil.Context(t)
		sources := []automation.JobSource{
			automation.JobSourcePackage,
			automation.JobSourceDynamic,
			automation.JobSourceConfig,
		}
		expected := make([]Trigger, 0, 13)
		for index := range 13 {
			trigger := automationNonWebhookTriggerForTest(
				automation.AutomationScopeWorkspace,
				fmt.Sprintf("trigger-%02d", 12-index),
				workspaceID,
				sources[index%len(sources)],
			)
			trigger.Filter["data.team"] = "operations-needle"
			created, err := globalDB.CreateTrigger(ctx, trigger)
			if err != nil {
				t.Fatalf("CreateTrigger(%d) error = %v", index, err)
			}
			expected = append(expected, created)
		}
		disabled := automationNonWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"trigger-disabled",
			workspaceID,
			automation.JobSourceConfig,
		)
		disabled.Enabled = false
		disabled.Filter["data.team"] = "operations-needle"
		if _, err := globalDB.CreateTrigger(ctx, disabled); err != nil {
			t.Fatalf("CreateTrigger(disabled) error = %v", err)
		}
		other := automationNonWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"trigger-other-workspace",
			otherWorkspaceID,
			automation.JobSourceConfig,
		)
		other.Filter["data.team"] = "operations-needle"
		if _, err := globalDB.CreateTrigger(ctx, other); err != nil {
			t.Fatalf("CreateTrigger(other workspace) error = %v", err)
		}

		automation.SortTriggersForList(expected)
		enabled := true
		query := TriggerListQuery{
			Scope:       automation.AutomationScopeWorkspace,
			WorkspaceID: workspaceID,
			Enabled:     &enabled,
			Search:      "OPERATIONS-NEEDLE",
			Limit:       4,
		}
		walkedIDs := make([]string, 0, len(expected))
		for {
			page, err := globalDB.ListTriggers(ctx, query)
			if err != nil {
				t.Fatalf("ListTriggers(cursor=%q) error = %v", query.Cursor, err)
			}
			if got, want := page.Total, len(expected); got != want {
				t.Fatalf("ListTriggers().Total = %d, want %d", got, want)
			}
			for _, trigger := range page.Triggers {
				walkedIDs = append(walkedIDs, trigger.ID)
			}
			if !page.HasMore {
				break
			}
			query.Cursor = page.NextCursor
		}
		if got, want := len(walkedIDs), len(expected); got != want {
			t.Fatalf("walked trigger count = %d, want %d", got, want)
		}
		for index, trigger := range expected {
			if got, want := walkedIDs[index], trigger.ID; got != want {
				t.Fatalf("walked trigger %d = %q, want %q", index, got, want)
			}
		}
	})
}

func TestGlobalDBAutomationSuggestionsEnforceConsentInvariants(t *testing.T) {
	t.Parallel()

	t.Run("Should append the v20 schema and preserve suggestions across reopen", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(t, path, automationSuggestionMigrationPrefix(t))
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(v19 prefix) error = %v", err)
		}
		ctx := testutil.Context(t)
		prefixClosed := false
		t.Cleanup(func() {
			if prefixClosed {
				return
			}
			if err := prefixDB.Close(); err != nil {
				t.Errorf("prefixDB.Close(cleanup) error = %v", err)
			}
		})
		exists, err := tableExists(ctx, prefixDB, "automation_suggestions")
		if err != nil {
			t.Fatalf("tableExists(automation_suggestions) error = %v", err)
		}
		if exists {
			t.Fatal("automation_suggestions exists at v19, want append-only v20 ownership")
		}
		prefixGlobalDB := &GlobalDB{db: prefixDB, path: path, now: time.Now}
		prefixGlobalDB.initializeRepositories(openConfig{})
		workspaceID := registerWorkspaceForGlobalTests(t, prefixGlobalDB, "suggestions-upgrade", t.TempDir())
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("prefixDB.Close() error = %v", err)
		}
		prefixClosed = true

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(v20 upgrade) error = %v", err)
		}
		ctx = testutil.Context(t)
		upgradedClosed := false
		t.Cleanup(func() {
			if upgradedClosed {
				return
			}
			if err := upgraded.Close(testutil.Context(t)); err != nil {
				t.Errorf("upgraded.Close(cleanup) error = %v", err)
			}
		})
		created, err := upgraded.CreateSuggestion(ctx, automationSuggestionForTest(
			"suggestion-upgrade",
			workspaceID,
			"catalog:v1:upgrade",
		), automation.DefaultSuggestionPendingCap)
		if err != nil {
			t.Fatalf("CreateSuggestion(after upgrade) error = %v", err)
		}
		status, err := store.Status(ctx, upgraded.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(v20) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("upgraded.Close() error = %v", err)
		}
		upgradedClosed = true

		reopened, err := OpenGlobalDB(testutil.Context(t), path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		ctx = testutil.Context(t)
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("reopened.Close() error = %v", err)
			}
		})
		stored, err := reopened.GetSuggestion(ctx, workspaceID, created.ID)
		if err != nil {
			t.Fatalf("GetSuggestion(reopen) error = %v", err)
		}
		if stored.ID != created.ID || stored.DedupKey != created.DedupKey ||
			stored.Status != automation.SuggestionStatusPending {
			t.Fatalf("GetSuggestion(reopen) = %#v, want durable pending suggestion %#v", stored, created)
		}
	})

	t.Run("Should preserve v20 payload rows through the hard column rename", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			automationSuggestionMigrationPrefix(t, "00020_schema.sql"),
		)
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(v20 prefix) error = %v", err)
		}
		ctx := testutil.Context(t)
		prefixClosed := false
		t.Cleanup(func() {
			if prefixClosed {
				return
			}
			if err := prefixDB.Close(); err != nil {
				t.Errorf("prefixDB.Close(cleanup) error = %v", err)
			}
		})
		prefixGlobalDB := &GlobalDB{db: prefixDB, path: path, now: time.Now}
		prefixGlobalDB.initializeRepositories(openConfig{})
		workspaceID := registerWorkspaceForGlobalTests(t, prefixGlobalDB, "suggestions-payload-rename", t.TempDir())
		created := automationSuggestionForTest(
			"suggestion-payload-rename",
			workspaceID,
			"catalog:v1:payload-rename",
		)
		created.CreatedAt = time.Date(2026, 7, 18, 13, 30, 0, 0, time.UTC)
		payload, err := json.Marshal(created.Payload)
		if err != nil {
			t.Fatalf("json.Marshal(payload) error = %v", err)
		}
		if _, err := prefixDB.ExecContext(
			ctx,
			`INSERT INTO automation_suggestions (
				id, workspace_id, source, dedup_key, status, payload_json, created_at, resolved_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, NULL)`,
			created.ID,
			created.WorkspaceID,
			created.Source,
			created.DedupKey,
			created.Status,
			string(payload),
			store.FormatTimestamp(created.CreatedAt),
		); err != nil {
			t.Fatalf("insert v20 suggestion error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("prefixDB.Close() error = %v", err)
		}
		prefixClosed = true

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(payload rename) error = %v", err)
		}
		ctx = testutil.Context(t)
		t.Cleanup(func() {
			if err := upgraded.Close(testutil.Context(t)); err != nil {
				t.Errorf("upgraded.Close() error = %v", err)
			}
		})
		stored, err := upgraded.GetSuggestion(ctx, workspaceID, created.ID)
		if err != nil {
			t.Fatalf("GetSuggestion(payload rename) error = %v", err)
		}
		if stored.ID != created.ID || stored.Payload.Prompt != created.Payload.Prompt ||
			stored.DedupKey != created.DedupKey {
			t.Fatalf("GetSuggestion(payload rename) = %#v, want preserved %#v", stored, created)
		}
		assertTableColumns(t, upgraded.db, "automation_suggestions", []string{
			"id", "workspace_id", "source", "dedup_key", "status", "payload", "created_at", "resolved_at",
		})
	})

	t.Run("Should reject a payload bound to another workspace before persistence", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "suggestions-boundary-a", t.TempDir())
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "suggestions-boundary-b", t.TempDir())
		suggestion := automationSuggestionForTest(
			"suggestion-cross-workspace",
			workspaceA,
			"catalog:v1:cross-workspace",
		)
		suggestion.Payload.WorkspaceID = workspaceB
		if _, err := globalDB.CreateSuggestion(
			testutil.Context(t),
			suggestion,
			automation.DefaultSuggestionPendingCap,
		); err == nil ||
			!strings.Contains(err.Error(), "payload.workspace_id") {
			t.Fatalf("CreateSuggestion(cross workspace) error = %v, want payload.workspace_id validation", err)
		}
		listed, err := globalDB.ListSuggestions(
			testutil.Context(t),
			workspaceA,
			automation.SuggestionStatusPending,
		)
		if err != nil {
			t.Fatalf("ListSuggestions(after rejection) error = %v", err)
		}
		if len(listed) != 0 {
			t.Fatalf("ListSuggestions(after rejection) = %#v, want empty", listed)
		}
	})

	t.Run("Should latch a dismissed dedup key and isolate workspace reads", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "suggestions-latch-a", t.TempDir())
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "suggestions-latch-b", t.TempDir())
		created, err := globalDB.CreateSuggestion(testutil.Context(t), automationSuggestionForTest(
			"suggestion-latched",
			workspaceA,
			"catalog:v1:latched",
		), automation.DefaultSuggestionPendingCap)
		if err != nil {
			t.Fatalf("CreateSuggestion() error = %v", err)
		}
		dismissed, err := globalDB.ResolveSuggestion(
			testutil.Context(t),
			workspaceA,
			created.ID,
			automation.SuggestionStatusDismissed,
		)
		if err != nil {
			t.Fatalf("ResolveSuggestion(dismissed) error = %v", err)
		}
		replayed, err := globalDB.CreateSuggestion(testutil.Context(t), automationSuggestionForTest(
			"suggestion-replayed",
			workspaceA,
			created.DedupKey,
		), automation.DefaultSuggestionPendingCap)
		if err != nil {
			t.Fatalf("CreateSuggestion(replay) error = %v", err)
		}
		if replayed.ID != dismissed.ID || replayed.Status != automation.SuggestionStatusDismissed {
			t.Fatalf("CreateSuggestion(replay) = %#v, want latched dismissed suggestion %#v", replayed, dismissed)
		}
		if _, err := globalDB.GetSuggestion(
			testutil.Context(t),
			workspaceB,
			created.ID,
		); !errors.Is(err, automation.ErrSuggestionNotFound) {
			t.Fatalf("GetSuggestion(foreign workspace) error = %v, want ErrSuggestionNotFound", err)
		}
		pending, err := globalDB.ListSuggestions(
			testutil.Context(t),
			workspaceA,
			automation.SuggestionStatusPending,
		)
		if err != nil {
			t.Fatalf("ListSuggestions(pending) error = %v", err)
		}
		if len(pending) != 0 {
			t.Fatalf("ListSuggestions(pending) = %#v, want empty after dismissal", pending)
		}
	})

	t.Run("Should record idempotent transition summaries with exact workspace content", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "suggestions-events", t.TempDir())
		cases := []struct {
			name    string
			status  automation.SuggestionStatus
			event   string
			jobID   string
			outcome string
		}{
			{
				name: "accepted", status: automation.SuggestionStatusAccepted,
				event: events.AutomationSuggestionAccepted, jobID: "job-suggestion-event",
				outcome: string(events.OutcomeFor(events.AutomationSuggestionAccepted)),
			},
			{
				name:    string(automation.SuggestionStatusDismissed),
				status:  automation.SuggestionStatusDismissed,
				event:   events.AutomationSuggestionDismissed,
				outcome: string(events.OutcomeFor(events.AutomationSuggestionDismissed)),
			},
		}
		for _, tc := range cases {
			t.Run("Should record "+tc.name, func(t *testing.T) {
				created, err := globalDB.CreateSuggestion(testutil.Context(t), automationSuggestionForTest(
					"suggestion-event-"+tc.name,
					workspaceID,
					"catalog:v1:event-"+tc.name,
				), automation.DefaultSuggestionPendingCap)
				if err != nil {
					t.Fatalf("CreateSuggestion() error = %v", err)
				}
				resolved, err := globalDB.ResolveSuggestion(
					testutil.Context(t),
					workspaceID,
					created.ID,
					tc.status,
				)
				if err != nil {
					t.Fatalf("ResolveSuggestion() error = %v", err)
				}
				for attempt := range 2 {
					if err := globalDB.RecordSuggestionTransition(
						testutil.Context(t),
						resolved,
						tc.jobID,
					); err != nil {
						t.Fatalf("RecordSuggestionTransition(attempt %d) error = %v", attempt, err)
					}
				}
				summaries, err := globalDB.ListEventSummaries(testutil.Context(t), EventSummaryQuery{
					WorkspaceID: workspaceID,
					Type:        tc.event,
					Limit:       10,
				})
				if err != nil {
					t.Fatalf("ListEventSummaries() error = %v", err)
				}
				if got, want := len(summaries), 1; got != want {
					t.Fatalf("len(ListEventSummaries()) = %d, want %d", got, want)
				}
				summary := summaries[0]
				if summary.WorkspaceID != workspaceID || summary.ActorKind != "automation_suggestion" ||
					summary.ActorID != resolved.ID || summary.Outcome != tc.outcome {
					t.Fatalf("transition summary = %#v, want exact workspace/actor/outcome", summary)
				}
				var content suggestionTransitionContent
				if err := json.Unmarshal(summary.Content, &content); err != nil {
					t.Fatalf("json.Unmarshal(summary.Content) error = %v", err)
				}
				if content.WorkspaceID != workspaceID || content.SuggestionID != resolved.ID ||
					content.Source != resolved.Source || content.DedupKey != resolved.DedupKey ||
					content.JobID != tc.jobID {
					t.Fatalf("transition content = %#v, want resolved suggestion and job %q", content, tc.jobID)
				}
			})
		}
	})

	t.Run("Should serialize the pending cap at five", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "suggestions-cap", t.TempDir())
		errs := make(chan error, automation.DefaultSuggestionPendingCap+1)
		var workers sync.WaitGroup
		for index := range automation.DefaultSuggestionPendingCap + 1 {
			workers.Go(func() {
				_, err := globalDB.CreateSuggestion(
					testutil.Context(t),
					automationSuggestionForTest(
						fmt.Sprintf("suggestion-cap-%d", index),
						workspaceID,
						fmt.Sprintf("catalog:v1:cap-%d", index),
					),
					automation.DefaultSuggestionPendingCap,
				)
				errs <- err
			})
		}
		workers.Wait()
		close(errs)

		created := 0
		capped := 0
		for err := range errs {
			switch {
			case err == nil:
				created++
			case errors.Is(err, automation.ErrSuggestionPendingCap):
				capped++
			default:
				t.Fatalf("CreateSuggestion(concurrent) unexpected error = %v", err)
			}
		}
		if created != automation.DefaultSuggestionPendingCap || capped != 1 {
			t.Fatalf(
				"concurrent results created/capped = %d/%d, want %d/1",
				created,
				capped,
				automation.DefaultSuggestionPendingCap,
			)
		}
	})

	t.Run("Should allow exactly one pending resolution", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "suggestions-cas", t.TempDir())
		created, err := globalDB.CreateSuggestion(testutil.Context(t), automationSuggestionForTest(
			"suggestion-cas",
			workspaceID,
			"catalog:v1:cas",
		), automation.DefaultSuggestionPendingCap)
		if err != nil {
			t.Fatalf("CreateSuggestion() error = %v", err)
		}

		statuses := []automation.SuggestionStatus{
			automation.SuggestionStatusAccepted,
			automation.SuggestionStatusDismissed,
		}
		errs := make(chan error, len(statuses))
		var workers sync.WaitGroup
		for _, status := range statuses {
			workers.Go(func() {
				_, resolveErr := globalDB.ResolveSuggestion(
					testutil.Context(t),
					workspaceID,
					created.ID,
					status,
				)
				errs <- resolveErr
			})
		}
		workers.Wait()
		close(errs)

		resolved := 0
		losers := 0
		for err := range errs {
			switch {
			case err == nil:
				resolved++
			case errors.Is(err, automation.ErrSuggestionResolved):
				losers++
			default:
				t.Fatalf("ResolveSuggestion(concurrent) unexpected error = %v", err)
			}
		}
		if resolved != 1 || losers != 1 {
			t.Fatalf("concurrent resolutions winner/loser = %d/%d, want 1/1", resolved, losers)
		}
	})
}

func automationSuggestionMigrationPrefix(t *testing.T, tail ...string) store.MigrationStream {
	t.Helper()

	files := []string{
		"00001_baseline.sql",
		"00002_schema.sql",
		"00003_schema.sql",
		"00004_schema.sql",
		"00005_schema.sql",
		"00006_schema.sql",
		"00007_schema.sql",
		"00008_network_participation.sql",
		"00009_automation_network_participation.sql",
		"00010_network_participation_hardening.sql",
		"00011_schema.sql",
		"00012_schema.sql",
		"00013_network_subscription_hard_cut.sql",
		"00014_network_task_status_projections.sql",
		"00015_schema.sql",
		"00016_schema.sql",
		"00017_schema.sql",
		"00018_schema.sql",
		"00019_schema.sql",
	}
	files = append(files, tail...)
	return globalMigrationPrefix(t, files...)
}

func TestGlobalDBAutomationCatalogUsesWorkspaceOrderIndexes(t *testing.T) {
	t.Parallel()

	t.Run("Should seek job candidates in public order without a temporary sort", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		query := automationmodel.JobListQueryWithDefaultLimit(JobListQuery{
			WorkspaceID: "ws-plan",
			Limit:       25,
		})
		where, args := automationJobCatalogWhere(query)
		args = append(args, query.Limit+1)
		plan := automationCatalogQueryPlan(
			t,
			globalDB.db,
			`SELECT c.job_id, c.source, c.name`+automationJobCatalogBaseSQL+where+
				` ORDER BY c.source_rank, c.name, c.job_id LIMIT ?`,
			args...,
		)
		if !strings.Contains(plan, automationJobCatalogWorkspaceIndex) {
			t.Fatalf("job catalog query plan = %q, want %s", plan, automationJobCatalogWorkspaceIndex)
		}
		if strings.Contains(plan, "USE TEMP B-TREE FOR ORDER BY") {
			t.Fatalf("job catalog query plan = %q, want indexed order", plan)
		}
	})

	t.Run("Should seek trigger candidates in public order without a temporary sort", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		query := automationmodel.TriggerListQueryWithDefaultLimit(TriggerListQuery{
			WorkspaceID: "ws-plan",
			Limit:       25,
		})
		where, args := automationTriggerCatalogWhere(query)
		args = append(args, query.Limit+1)
		plan := automationCatalogQueryPlan(
			t,
			globalDB.db,
			`SELECT c.trigger_id, c.source, c.name`+automationTriggerCatalogBaseSQL+where+
				` ORDER BY c.source_rank, c.name, c.trigger_id LIMIT ?`,
			args...,
		)
		if !strings.Contains(plan, automationTriggerCatalogWorkspaceIndex) {
			t.Fatalf("trigger catalog query plan = %q, want %s", plan, automationTriggerCatalogWorkspaceIndex)
		}
		if strings.Contains(plan, "USE TEMP B-TREE FOR ORDER BY") {
			t.Fatalf("trigger catalog query plan = %q, want indexed order", plan)
		}
	})
}

func TestGlobalDBAutomationValidationAndDeleteBehavior(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	if _, err := globalDB.ListJobs(testutil.Context(t), JobListQuery{
		Scope:       automation.AutomationScopeGlobal,
		WorkspaceID: "ws-ignored",
	}); err == nil {
		t.Fatal("ListJobs(invalid scope/workspace filter) error = nil, want non-nil")
	}
	if _, err := globalDB.ListTriggers(testutil.Context(t), TriggerListQuery{
		Scope:       automation.AutomationScopeGlobal,
		WorkspaceID: "ws-ignored",
	}); err == nil {
		t.Fatal("ListTriggers(invalid scope/workspace filter) error = nil, want non-nil")
	}
	if _, err := globalDB.ListRuns(testutil.Context(t), RunQuery{
		Since: time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 4, 10, 11, 0, 0, 0, time.UTC),
	}); err == nil {
		t.Fatal("ListRuns(invalid window) error = nil, want non-nil")
	}
	if _, err := globalDB.CreateRun(testutil.Context(t), Run{Status: automation.RunCompleted, Attempt: 1}); err == nil {
		t.Fatal("CreateRun(without job or trigger) error = nil, want non-nil")
	}

	if _, err := globalDB.CreateJob(
		testutil.Context(t),
		automationJobForTest(
			automation.AutomationScopeWorkspace,
			"missing-workspace-job",
			"ws-missing",
			automation.JobSourceDynamic,
		),
	); !errors.Is(
		err,
		aghworkspace.ErrWorkspaceNotFound,
	) {
		t.Fatalf("CreateJob(missing workspace) error = %v, want ErrWorkspaceNotFound", err)
	}
	if _, err := globalDB.CreateTrigger(
		testutil.Context(t),
		automationWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"missing-workspace-trigger",
			"ws-missing",
			automation.JobSourceDynamic,
		),
	); !errors.Is(
		err,
		aghworkspace.ErrWorkspaceNotFound,
	) {
		t.Fatalf("CreateTrigger(missing workspace) error = %v, want ErrWorkspaceNotFound", err)
	}

	workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-delete-workspace", t.TempDir())
	job, err := globalDB.CreateJob(
		testutil.Context(t),
		automationJobForTest(
			automation.AutomationScopeWorkspace,
			"delete-job",
			workspaceID,
			automation.JobSourceDynamic,
		),
	)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	job.Prompt = "Updated automation prompt."
	job.Schedule = &automation.ScheduleSpec{
		Mode: automation.ScheduleModeCron,
		Expr: "0 * * * *",
	}
	job, err = globalDB.UpdateJob(testutil.Context(t), job)
	if err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}
	loadedJob, err := globalDB.GetJob(testutil.Context(t), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got, want := loadedJob.Prompt, "Updated automation prompt."; got != want {
		t.Fatalf("GetJob().Prompt = %q, want %q", got, want)
	}
	trigger, err := globalDB.CreateTrigger(
		testutil.Context(t),
		automationNonWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"delete-trigger",
			workspaceID,
			automation.JobSourceDynamic,
		),
	)
	if err != nil {
		t.Fatalf("CreateTrigger() error = %v", err)
	}
	run, err := globalDB.CreateRun(
		testutil.Context(t),
		automationRunForJob(job.ID, automation.RunRunning, 1, time.Date(2026, 4, 10, 22, 0, 0, 0, time.UTC)),
	)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	run.Status = automation.RunFailed
	run.Attempt = 2
	run.SessionID = "sess-updated"
	run.NetworkParticipation = automationNamedParticipation("ops-automation")
	run.Metadata = map[string]any{
		"catch_up":        true,
		"catch_up_policy": "coalesce",
		"reason":          "loop_concurrency_conflict",
	}
	updatedRun, err := globalDB.UpdateRun(testutil.Context(t), run)
	if err != nil {
		t.Fatalf("UpdateRun() error = %v", err)
	}
	if got, want := updatedRun.Status, automation.RunFailed; got != want {
		t.Fatalf("UpdateRun().Status = %q, want %q", got, want)
	}
	if got, want := updatedRun.SessionID, "sess-updated"; got != want {
		t.Fatalf("UpdateRun().SessionID = %q, want %q", got, want)
	}
	if got, want := updatedRun.Metadata["reason"], "loop_concurrency_conflict"; got != want {
		t.Fatalf("UpdateRun().Metadata[reason] = %#v, want %q", got, want)
	}
	loadedRun, err := globalDB.GetRun(testutil.Context(t), run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if got, want := loadedRun.Attempt, 2; got != want {
		t.Fatalf("GetRun().Attempt = %d, want %d", got, want)
	}
	if got, want := loadedRun.Metadata["catch_up_policy"], "coalesce"; got != want {
		t.Fatalf("GetRun().Metadata[catch_up_policy] = %#v, want %q", got, want)
	}
	assertAutomationNamedParticipation(t, loadedRun.NetworkParticipation, "ops-automation")

	jobs, err := globalDB.ListJobs(
		testutil.Context(t),
		JobListQuery{Scope: automation.AutomationScopeWorkspace, Source: automation.JobSourceDynamic},
	)
	if err != nil {
		t.Fatalf("ListJobs(filtered) error = %v", err)
	}
	if got, want := len(jobs.Jobs), 1; got != want {
		t.Fatalf("len(ListJobs(filtered)) = %d, want %d", got, want)
	}

	if err := globalDB.DeleteRun(testutil.Context(t), run.ID); err != nil {
		t.Fatalf("DeleteRun() error = %v", err)
	}
	if _, err := globalDB.GetRun(testutil.Context(t), run.ID); !errors.Is(err, automation.ErrRunNotFound) {
		t.Fatalf("GetRun(after delete) error = %v, want ErrRunNotFound", err)
	}
	if err := globalDB.DeleteRun(testutil.Context(t), run.ID); !errors.Is(err, automation.ErrRunNotFound) {
		t.Fatalf("DeleteRun(missing) error = %v, want ErrRunNotFound", err)
	}
	if err := globalDB.DeleteJob(testutil.Context(t), job.ID); err != nil {
		t.Fatalf("DeleteJob() error = %v", err)
	}
	if _, err := globalDB.GetJob(testutil.Context(t), job.ID); !errors.Is(err, automation.ErrJobNotFound) {
		t.Fatalf("GetJob(after delete) error = %v, want ErrJobNotFound", err)
	}
	if err := globalDB.DeleteJob(testutil.Context(t), job.ID); !errors.Is(err, automation.ErrJobNotFound) {
		t.Fatalf("DeleteJob(missing) error = %v, want ErrJobNotFound", err)
	}
	if err := globalDB.DeleteTrigger(testutil.Context(t), trigger.ID); err != nil {
		t.Fatalf("DeleteTrigger() error = %v", err)
	}
	if _, err := globalDB.GetTrigger(testutil.Context(t), trigger.ID); !errors.Is(err, automation.ErrTriggerNotFound) {
		t.Fatalf("GetTrigger(after delete) error = %v, want ErrTriggerNotFound", err)
	}
	if err := globalDB.DeleteTrigger(testutil.Context(t), trigger.ID); !errors.Is(err, automation.ErrTriggerNotFound) {
		t.Fatalf("DeleteTrigger(missing) error = %v, want ErrTriggerNotFound", err)
	}

	t.Run("Should map typed run uniqueness constraints to ErrRunAlreadyExists", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		job, err := db.CreateJob(ctx, automationJobForTest(
			automation.AutomationScopeGlobal,
			"run-uniqueness-job",
			"",
			automation.JobSourceDynamic,
		))
		if err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}
		base := time.Date(2026, 4, 10, 23, 0, 0, 0, time.UTC)
		first := automationRunForJob(job.ID, automation.RunRunning, 1, base)
		first.FireID = "fire-run-unique"
		first, err = db.CreateRun(ctx, first)
		if err != nil {
			t.Fatalf("CreateRun(first) error = %v", err)
		}

		duplicateID := automationRunForJob(job.ID, automation.RunRunning, 1, base.Add(time.Minute))
		duplicateID.ID = first.ID
		duplicateID.FireID = "fire-run-other"
		if _, err := db.CreateRun(ctx, duplicateID); !errors.Is(err, automation.ErrRunAlreadyExists) {
			t.Fatalf("CreateRun(duplicate id) error = %v, want ErrRunAlreadyExists", err)
		}

		duplicateFire := automationRunForJob(job.ID, automation.RunRunning, 1, base.Add(2*time.Minute))
		duplicateFire.FireID = first.FireID
		if _, err := db.CreateRun(ctx, duplicateFire); !errors.Is(err, automation.ErrRunAlreadyExists) {
			t.Fatalf("CreateRun(duplicate fire) error = %v, want ErrRunAlreadyExists", err)
		}
	})
}

func TestGlobalDBAutomationLoopTargetsPersistFilterAndCorrelateRuns(t *testing.T) {
	t.Parallel()

	t.Run("Should persist and filter loop-target jobs", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-loop-target-jobs", t.TempDir())
		loopJob := automationJobForTest(
			automation.AutomationScopeWorkspace,
			"loop-job",
			workspaceID,
			automation.JobSourceDynamic,
		)
		loopJob.AgentName = ""
		loopJob.Prompt = ""
		loopJob.TargetKind = automation.TargetKindLoop
		loopJob.LoopTarget = &automation.LoopTarget{
			WorkspaceID:          workspaceID,
			LoopName:             "triage",
			NetworkParticipation: automationNamedParticipation("loop-job-channel"),
			Inputs: map[string]any{
				"tasks": "task-ref",
			},
		}
		createdLoopJob, err := globalDB.CreateJob(ctx, loopJob)
		if err != nil {
			t.Fatalf("CreateJob(loop target) error = %v", err)
		}
		storedLoopJob, err := globalDB.GetJob(ctx, createdLoopJob.ID)
		if err != nil {
			t.Fatalf("GetJob(loop target) error = %v", err)
		}
		if storedLoopJob.LoopTarget == nil {
			t.Fatal("GetJob(loop target).LoopTarget = nil, want persisted target")
		}
		if got, want := storedLoopJob.TargetKind, automation.TargetKindLoop; got != want {
			t.Fatalf("GetJob(loop target).TargetKind = %q, want %q", got, want)
		}
		if got, want := storedLoopJob.LoopTarget.LoopName, "triage"; got != want {
			t.Fatalf("GetJob(loop target).LoopTarget.LoopName = %q, want %q", got, want)
		}
		assertAutomationNamedParticipation(
			t,
			storedLoopJob.LoopTarget.NetworkParticipation,
			"loop-job-channel",
		)
		agentJob := automationJobForTest(
			automation.AutomationScopeWorkspace,
			"agent-job",
			workspaceID,
			automation.JobSourceDynamic,
		)
		if _, err := globalDB.CreateJob(ctx, agentJob); err != nil {
			t.Fatalf("CreateJob(agent target) error = %v", err)
		}
		filteredJobs, err := globalDB.ListJobs(ctx, JobListQuery{
			Scope:       automation.AutomationScopeWorkspace,
			WorkspaceID: workspaceID,
			LoopName:    "triage",
		})
		if err != nil {
			t.Fatalf("ListJobs(loop filter) error = %v", err)
		}
		if got, want := len(filteredJobs.Jobs), 1; got != want {
			t.Fatalf("len(ListJobs(loop filter)) = %d, want %d", got, want)
		}
		if got, want := filteredJobs.Jobs[0].ID, createdLoopJob.ID; got != want {
			t.Fatalf("ListJobs(loop filter)[0].ID = %q, want %q", got, want)
		}
		if filteredJobs.Jobs[0].LoopTarget == nil {
			t.Fatal("ListJobs(loop filter)[0].LoopTarget = nil, want hydrated target")
		}
		assertAutomationNamedParticipation(
			t,
			filteredJobs.Jobs[0].LoopTarget.NetworkParticipation,
			"loop-job-channel",
		)
	})

	t.Run("Should persist and filter loop-target triggers", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-loop-target-triggers", t.TempDir())
		loopTrigger := automationWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"loop-trigger",
			workspaceID,
			automation.JobSourceDynamic,
		)
		loopTrigger.AgentName = ""
		loopTrigger.Prompt = ""
		loopTrigger.TargetKind = automation.TargetKindLoop
		loopTrigger.LoopTarget = &automation.LoopTarget{
			WorkspaceID:          workspaceID,
			LoopName:             "triage",
			NetworkParticipation: automationNamedParticipation("loop-trigger-channel"),
			InputMapping: map[string]string{
				"title": "{{ .trigger.payload.title }}",
			},
		}
		createdLoopTrigger, err := globalDB.CreateTrigger(ctx, loopTrigger)
		if err != nil {
			t.Fatalf("CreateTrigger(loop target) error = %v", err)
		}
		storedLoopTrigger, err := globalDB.GetTrigger(ctx, createdLoopTrigger.ID)
		if err != nil {
			t.Fatalf("GetTrigger(loop target) error = %v", err)
		}
		if storedLoopTrigger.LoopTarget == nil {
			t.Fatal("GetTrigger(loop target).LoopTarget = nil, want persisted target")
		}
		if got, want := storedLoopTrigger.LoopTarget.InputMapping["title"], "{{ .trigger.payload.title }}"; got != want {
			t.Fatalf("GetTrigger(loop target).LoopTarget.InputMapping[title] = %q, want %q", got, want)
		}
		assertAutomationNamedParticipation(
			t,
			storedLoopTrigger.LoopTarget.NetworkParticipation,
			"loop-trigger-channel",
		)
		agentTrigger := automationWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"agent-trigger",
			workspaceID,
			automation.JobSourceDynamic,
		)
		agentTrigger.WebhookID = "wbh_agent_trigger"
		agentTrigger.EndpointSlug = "endpoint-agent-trigger"
		if _, err := globalDB.CreateTrigger(ctx, agentTrigger); err != nil {
			t.Fatalf("CreateTrigger(agent target) error = %v", err)
		}
		filteredTriggers, err := globalDB.ListTriggers(ctx, TriggerListQuery{
			Scope:       automation.AutomationScopeWorkspace,
			WorkspaceID: workspaceID,
			LoopName:    "triage",
		})
		if err != nil {
			t.Fatalf("ListTriggers(loop filter) error = %v", err)
		}
		if got, want := len(filteredTriggers.Triggers), 1; got != want {
			t.Fatalf("len(ListTriggers(loop filter)) = %d, want %d", got, want)
		}
		if got, want := filteredTriggers.Triggers[0].ID, createdLoopTrigger.ID; got != want {
			t.Fatalf("ListTriggers(loop filter)[0].ID = %q, want %q", got, want)
		}
		if filteredTriggers.Triggers[0].LoopTarget == nil {
			t.Fatal("ListTriggers(loop filter)[0].LoopTarget = nil, want hydrated target")
		}
		assertAutomationNamedParticipation(
			t,
			filteredTriggers.Triggers[0].LoopTarget.NetworkParticipation,
			"loop-trigger-channel",
		)
	})

	t.Run("Should correlate delegated automation runs to loop runs", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-loop-target-runs", t.TempDir())
		loopJob := automationJobForTest(
			automation.AutomationScopeWorkspace,
			"loop-job",
			workspaceID,
			automation.JobSourceDynamic,
		)
		loopJob.AgentName = ""
		loopJob.Prompt = ""
		loopJob.TargetKind = automation.TargetKindLoop
		loopJob.LoopTarget = &automation.LoopTarget{WorkspaceID: workspaceID, LoopName: "triage"}
		createdLoopJob, err := globalDB.CreateJob(ctx, loopJob)
		if err != nil {
			t.Fatalf("CreateJob(loop target) error = %v", err)
		}
		startedAt := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
		loopRunSeed := testLoopRun("looprun-automation-delegated", startedAt, looppkg.StatusRunning)
		loopRunSeed.WorkspaceID = looppkg.WorkspaceID(workspaceID)
		loopRunSeed.LoopName = "triage"
		createdLoopRun, err := globalDB.CreateLoopRunForStart(ctx, loopRunSeed, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		delegatedRun, err := globalDB.CreateRun(ctx, Run{
			JobID:     createdLoopJob.ID,
			Status:    automation.RunDelegated,
			Attempt:   1,
			StartedAt: timePointer(startedAt),
			EndedAt:   timePointer(startedAt.Add(time.Second)),
			LoopRunID: string(createdLoopRun.ID),
		})
		if err != nil {
			t.Fatalf("CreateRun(loop delegated) error = %v", err)
		}
		loadedRun, err := globalDB.GetRun(ctx, delegatedRun.ID)
		if err != nil {
			t.Fatalf("GetRun(loop delegated) error = %v", err)
		}
		if got, want := loadedRun.LoopRunID, string(createdLoopRun.ID); got != want {
			t.Fatalf("GetRun(loop delegated).LoopRunID = %q, want %q", got, want)
		}
		if got := loadedRun.TaskID; got != "" {
			t.Fatalf("GetRun(loop delegated).TaskID = %q, want empty", got)
		}
	})

	t.Run("Should enforce XOR validation for delegated run targets", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-loop-target-xor", t.TempDir())
		loopJob := automationJobForTest(
			automation.AutomationScopeWorkspace,
			"loop-job",
			workspaceID,
			automation.JobSourceDynamic,
		)
		loopJob.AgentName = ""
		loopJob.Prompt = ""
		loopJob.TargetKind = automation.TargetKindLoop
		loopJob.LoopTarget = &automation.LoopTarget{WorkspaceID: workspaceID, LoopName: "triage"}
		createdLoopJob, err := globalDB.CreateJob(ctx, loopJob)
		if err != nil {
			t.Fatalf("CreateJob(loop target) error = %v", err)
		}
		startedAt := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
		loopRunSeed := testLoopRun("looprun-automation-xor", startedAt, looppkg.StatusRunning)
		loopRunSeed.WorkspaceID = looppkg.WorkspaceID(workspaceID)
		loopRunSeed.LoopName = "triage"
		createdLoopRun, err := globalDB.CreateLoopRunForStart(ctx, loopRunSeed, dsl.ConcurrencyAllow)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		invalidDelegated := automationRunForJob(
			createdLoopJob.ID,
			automation.RunDelegated,
			2,
			startedAt.Add(time.Minute),
		)
		invalidDelegated.TaskID = "task-1"
		invalidDelegated.TaskRunID = "task-run-1"
		invalidDelegated.LoopRunID = string(createdLoopRun.ID)
		if _, err := globalDB.CreateRun(ctx, invalidDelegated); err == nil {
			t.Fatal("CreateRun(delegated task and loop targets) error = nil, want XOR validation error")
		} else if !strings.Contains(err.Error(), "requires exactly one") {
			t.Fatalf("CreateRun(delegated task and loop targets) error = %v, want XOR validation", err)
		}
	})
}

func TestAutomationStoreHelperBranches(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	base := time.Date(2026, 4, 10, 23, 0, 0, 0, time.UTC)
	globalDB.now = func() time.Time { return base }

	job, err := globalDB.normalizeJobForCreate(Job{
		Scope:     automation.AutomationScopeGlobal,
		Name:      "helper-job",
		AgentName: "agent",
		Prompt:    "prompt",
		Schedule: &automation.ScheduleSpec{
			Mode:     automation.ScheduleModeEvery,
			Interval: "15m",
		},
		Enabled:   true,
		Retry:     automation.DefaultRetryConfig(),
		FireLimit: automation.DefaultFireLimitConfig(),
	})
	if err != nil {
		t.Fatalf("normalizeJobForCreate() error = %v", err)
	}
	if got, want := job.Source, automation.JobSourceDynamic; got != want {
		t.Fatalf("normalizeJobForCreate().Source = %q, want %q", got, want)
	}
	if job.ID == "" || !job.CreatedAt.Equal(base) || !job.UpdatedAt.Equal(base) {
		t.Fatalf("normalizeJobForCreate() = %#v, want generated id and base timestamps", job)
	}
	if _, err := globalDB.normalizeJobForUpdate(Job{}); err == nil {
		t.Fatal("normalizeJobForUpdate(empty) error = nil, want non-nil")
	}

	trigger, err := globalDB.normalizeTriggerForCreate(Trigger{
		Scope:            automation.AutomationScopeGlobal,
		Name:             "helper-trigger",
		AgentName:        "agent",
		Prompt:           `{{ index .Data "payload" }}`,
		Event:            "webhook",
		Enabled:          true,
		Retry:            automation.DefaultRetryConfig(),
		FireLimit:        automation.DefaultFireLimitConfig(),
		WebhookID:        "wbh_helper-webhook",
		WebhookSecretRef: "vault:automation/triggers/helper/webhook-secret",
	})
	if err != nil {
		t.Fatalf("normalizeTriggerForCreate() error = %v", err)
	}
	if got, want := trigger.Source, automation.JobSourceDynamic; got != want {
		t.Fatalf("normalizeTriggerForCreate().Source = %q, want %q", got, want)
	}
	if _, err := globalDB.normalizeTriggerForUpdate(Trigger{}); err == nil {
		t.Fatal("normalizeTriggerForUpdate(empty) error = nil, want non-nil")
	}

	run, err := globalDB.normalizeRunForCreate(Run{
		JobID:  "job-1",
		Status: automation.RunRunning,
	})
	if err != nil {
		t.Fatalf("normalizeRunForCreate() error = %v", err)
	}
	if got, want := run.Attempt, 1; got != want {
		t.Fatalf("normalizeRunForCreate().Attempt = %d, want %d", got, want)
	}
	if _, err := globalDB.normalizeRunForCreate(Run{
		JobID:   "job-1",
		Status:  automation.RunRunning,
		Attempt: -1,
	}); err == nil {
		t.Fatal("normalizeRunForCreate(negative attempt) error = nil, want non-nil")
	}
	if _, err := globalDB.normalizeRunForUpdate(Run{
		ID:      "run-1",
		JobID:   "job-1",
		Status:  automation.RunRunning,
		Attempt: 0,
	}); err == nil {
		t.Fatal("normalizeRunForUpdate(zero attempt) error = nil, want non-nil")
	}

	if _, err := requireAutomationID("", "automation id"); err == nil {
		t.Fatal("requireAutomationID(empty) error = nil, want non-nil")
	}
	if got, err := requireAutomationID(" job-1 ", "automation id"); err != nil || got != "job-1" {
		t.Fatalf("requireAutomationID(trimmed) = (%q, %v), want (job-1, nil)", got, err)
	}

	if err := validateAutomationRunQuery(RunQuery{Status: "bad-status"}); err == nil {
		t.Fatal("validateAutomationRunQuery(bad status) error = nil, want non-nil")
	}
	if err := validateAutomationRunRecord(Run{
		JobID:     "job-1",
		TriggerID: "trg-1",
		Status:    automation.RunRunning,
		Attempt:   1,
	}); err == nil {
		t.Fatal("validateAutomationRunRecord(both job and trigger) error = nil, want non-nil")
	}
	if err := validateAutomationRunRecord(Run{
		JobID:   "job-1",
		Status:  automation.RunDelegated,
		Attempt: 1,
		TaskID:  "task-1",
	}); err == nil {
		t.Fatal("validateAutomationRunRecord(delegated without task run) error = nil, want non-nil")
	}

	var schedule *automation.ScheduleSpec
	if err := decodeAutomationSchedule(
		sql.NullString{Valid: true, String: `{"mode":"cron","expr":"0 * * * *"}`},
		&schedule,
	); err != nil {
		t.Fatalf("decodeAutomationSchedule(valid) error = %v", err)
	}
	if schedule == nil || schedule.Mode != automation.ScheduleModeCron {
		t.Fatalf("decodeAutomationSchedule(valid) = %#v, want cron schedule", schedule)
	}
	if err := decodeAutomationSchedule(sql.NullString{Valid: true, String: `{`}, &schedule); err == nil {
		t.Fatal("decodeAutomationSchedule(invalid) error = nil, want non-nil")
	}

	var taskConfig *automation.JobTaskConfig
	if err := decodeAutomationTaskConfig(
		sql.NullString{
			Valid:  true,
			String: `{"title":"Review findings","network_participation":{"mode":"live","channel_strategy":"named","channel_id":"ops-automation"}}`,
		},
		&taskConfig,
	); err != nil {
		t.Fatalf("decodeAutomationTaskConfig(valid) error = %v", err)
	}
	if taskConfig == nil || taskConfig.Title != "Review findings" {
		t.Fatalf("decodeAutomationTaskConfig(valid) = %#v, want populated task config", taskConfig)
	}
	assertAutomationNamedParticipation(t, taskConfig.NetworkParticipation, "ops-automation")
	if err := decodeAutomationTaskConfig(sql.NullString{Valid: true, String: `{`}, &taskConfig); err == nil {
		t.Fatal("decodeAutomationTaskConfig(invalid) error = nil, want non-nil")
	}

	var filter map[string]string
	if err := decodeAutomationFilter(
		sql.NullString{Valid: true, String: `{"data.kind":"ready"}`},
		&filter,
	); err != nil {
		t.Fatalf("decodeAutomationFilter(valid) error = %v", err)
	}
	if got, want := filter["data.kind"], "ready"; got != want {
		t.Fatalf("decodeAutomationFilter(valid) = %#v, want ready value", filter)
	}
	if err := decodeAutomationFilter(sql.NullString{Valid: true, String: `{`}, &filter); err == nil {
		t.Fatal("decodeAutomationFilter(invalid) error = nil, want non-nil")
	}

	if got := nullableAutomationTimestamp(nil); got != nil {
		t.Fatalf("nullableAutomationTimestamp(nil) = %#v, want nil", got)
	}
	if got, want := nullableAutomationTimestamp(timePointer(base)), any(store.FormatTimestamp(base)); got != want {
		t.Fatalf("nullableAutomationTimestamp(value) = %#v, want %#v", got, want)
	}
}

func TestGlobalDBAutomationDefinitionsExposeSourceForLegacyRows(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-source-workspace", t.TempDir())
	job, err := globalDB.CreateJob(
		testutil.Context(t),
		automationJobForTest(
			automation.AutomationScopeWorkspace,
			"source-job",
			workspaceID,
			automation.JobSourceConfig,
		),
	)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	trigger, err := globalDB.CreateTrigger(
		testutil.Context(t),
		automationWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"source-trigger",
			workspaceID,
			automation.JobSourceConfig,
		),
	)
	if err != nil {
		t.Fatalf("CreateTrigger() error = %v", err)
	}

	storedJob, err := globalDB.GetJob(testutil.Context(t), job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got, want := storedJob.Source, automation.JobSourceConfig; got != want {
		t.Fatalf("GetJob().Source = %q, want %q", got, want)
	}
	storedTrigger, err := globalDB.GetTrigger(testutil.Context(t), trigger.ID)
	if err != nil {
		t.Fatalf("GetTrigger() error = %v", err)
	}
	if got, want := storedTrigger.Source, automation.JobSourceConfig; got != want {
		t.Fatalf("GetTrigger().Source = %q, want %q", got, want)
	}

	if _, err := globalDB.GetJob(
		testutil.Context(t),
		"missing-job",
	); !errors.Is(
		err,
		automation.ErrJobNotFound,
	) {
		t.Fatalf("GetJob(missing) error = %v, want ErrJobNotFound", err)
	}
	if _, err := globalDB.GetTrigger(
		testutil.Context(t),
		"missing-trigger",
	); !errors.Is(
		err,
		automation.ErrTriggerNotFound,
	) {
		t.Fatalf("GetTrigger(missing) error = %v, want ErrTriggerNotFound", err)
	}
}

func TestAutomationJSONHelperFunctions(t *testing.T) {
	t.Parallel()

	encoded, err := encodeAutomationJSON(automation.DefaultFireLimitConfig(), "fire_limit")
	if err != nil {
		t.Fatalf("encodeAutomationJSON() error = %v", err)
	}
	if encoded == "" {
		t.Fatal("encodeAutomationJSON() = empty, want JSON")
	}

	optional, err := encodeOptionalAutomationJSON(map[string]string{"key": "value"}, false, "filter")
	if err != nil {
		t.Fatalf("encodeOptionalAutomationJSON(non-empty) error = %v", err)
	}
	if optional == nil {
		t.Fatal("encodeOptionalAutomationJSON(non-empty) = nil, want JSON payload")
	}
	optional, err = encodeOptionalAutomationJSON(map[string]string{}, true, "filter")
	if err != nil {
		t.Fatalf("encodeOptionalAutomationJSON(empty) error = %v", err)
	}
	if optional != nil {
		t.Fatalf("encodeOptionalAutomationJSON(empty) = %#v, want nil", optional)
	}

	var decoded automation.FireLimitConfig
	if err := decodeAutomationJSON(encoded, &decoded, "fire_limit"); err != nil {
		t.Fatalf("decodeAutomationJSON(valid) error = %v", err)
	}
	if got, want := decoded.Window, automation.DefaultFireLimitConfig().Window; got != want {
		t.Fatalf("decodeAutomationJSON(valid).Window = %q, want %q", got, want)
	}
	if err := decodeAutomationJSON("", &decoded, "fire_limit"); err == nil {
		t.Fatal("decodeAutomationJSON(empty) error = nil, want non-nil")
	}
}

func TestAutomationOverlayNormalizersAndQueryValidators(t *testing.T) {
	t.Parallel()

	if _, err := normalizeJobOverlay(JobEnabledOverlay{}, time.Date(2026, 4, 10, 23, 30, 0, 0, time.UTC)); err == nil {
		t.Fatal("normalizeJobOverlay(empty) error = nil, want non-nil")
	}
	if _, err := normalizeTriggerOverlay(
		TriggerEnabledOverlay{},
		time.Date(2026, 4, 10, 23, 30, 0, 0, time.UTC),
	); err == nil {
		t.Fatal("normalizeTriggerOverlay(empty) error = nil, want non-nil")
	}
	if err := automation.ValidateJobListQuery(JobListQuery{Source: "bad-source"}); err == nil {
		t.Fatal("validateAutomationJobListQuery(bad source) error = nil, want non-nil")
	}
	if err := automation.ValidateTriggerListQuery(TriggerListQuery{Source: "bad-source"}); err == nil {
		t.Fatal("validateAutomationTriggerListQuery(bad source) error = nil, want non-nil")
	}
}

func TestGlobalDBListRunsFiltersByJobTriggerStatusAndTimeWindow(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "automation-runs-workspace", t.TempDir())
	jobA, err := globalDB.CreateJob(
		testutil.Context(t),
		automationJobForTest(automation.AutomationScopeWorkspace, "job-a", workspaceID, automation.JobSourceDynamic),
	)
	if err != nil {
		t.Fatalf("CreateJob(jobA) error = %v", err)
	}
	jobB, err := globalDB.CreateJob(
		testutil.Context(t),
		automationJobForTest(automation.AutomationScopeWorkspace, "job-b", workspaceID, automation.JobSourceDynamic),
	)
	if err != nil {
		t.Fatalf("CreateJob(jobB) error = %v", err)
	}
	trigger, err := globalDB.CreateTrigger(
		testutil.Context(t),
		automationNonWebhookTriggerForTest(
			automation.AutomationScopeWorkspace,
			"trigger-a",
			workspaceID,
			automation.JobSourceDynamic,
		),
	)
	if err != nil {
		t.Fatalf("CreateTrigger() error = %v", err)
	}

	base := time.Date(2026, 4, 10, 20, 0, 0, 0, time.UTC)
	for _, run := range []Run{
		automationRunForJob(jobA.ID, automation.RunCompleted, 1, base.Add(-10*time.Minute)),
		automationRunForJob(jobA.ID, automation.RunFailed, 2, base.Add(-5*time.Minute)),
		automationRunForTrigger(trigger.ID, automation.RunCompleted, 1, base.Add(-2*time.Minute)),
		automationRunForJob(jobB.ID, automation.RunCompleted, 1, base.Add(-90*time.Minute)),
	} {
		if _, err := globalDB.CreateRun(testutil.Context(t), run); err != nil {
			t.Fatalf("CreateRun(%q) error = %v", run.Status, err)
		}
	}

	jobRuns, err := globalDB.ListRuns(testutil.Context(t), RunQuery{
		JobID:  jobA.ID,
		Status: automation.RunFailed,
		Since:  base.Add(-6 * time.Minute),
		Until:  base,
	})
	if err != nil {
		t.Fatalf("ListRuns(job filter) error = %v", err)
	}
	if got, want := len(jobRuns), 1; got != want {
		t.Fatalf("len(jobRuns) = %d, want %d", got, want)
	}
	if got, want := jobRuns[0].Attempt, 2; got != want {
		t.Fatalf("jobRuns[0].Attempt = %d, want %d", got, want)
	}

	triggerRuns, err := globalDB.ListRuns(testutil.Context(t), RunQuery{
		TriggerID: trigger.ID,
		Status:    automation.RunCompleted,
		Since:     base.Add(-5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("ListRuns(trigger filter) error = %v", err)
	}
	if got, want := len(triggerRuns), 1; got != want {
		t.Fatalf("len(triggerRuns) = %d, want %d", got, want)
	}
	if got, want := triggerRuns[0].TriggerID, trigger.ID; got != want {
		t.Fatalf("triggerRuns[0].TriggerID = %q, want %q", got, want)
	}

	count, err := globalDB.CountRuns(testutil.Context(t), RunQuery{
		JobID: jobA.ID,
		Since: base.Add(-30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("CountRuns() error = %v", err)
	}
	if got, want := count, int64(2); got != want {
		t.Fatalf("CountRuns() = %d, want %d", got, want)
	}
}

func TestGlobalDBSchedulerStateSaveClaimAndDeliveryError(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	job, err := globalDB.CreateJob(
		ctx,
		automationJobForTest(automation.AutomationScopeGlobal, "durable-scheduler", "", automation.JobSourceDynamic),
	)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	nextRun := time.Date(2026, 4, 11, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	saved, err := globalDB.SaveSchedulerState(ctx, SchedulerState{
		JobID:               job.ID,
		NextRunAt:           &nextRun,
		ScheduleHash:        "hash-v1",
		CatchUpPolicy:       automation.SchedulerCatchUpPolicySkipMissed,
		MisfireGraceSeconds: 30,
		UpdatedAt:           updatedAt,
	})
	if err != nil {
		t.Fatalf("SaveSchedulerState() error = %v", err)
	}
	if got, want := saved.NextRunAt, &nextRun; got == nil || !got.Equal(*want) {
		t.Fatalf("SaveSchedulerState().NextRunAt = %v, want %v", got, want)
	}

	claimedAt := time.Date(2026, 4, 11, 9, 0, 1, 0, time.UTC)
	nextAfterClaim := time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC)
	claim, err := globalDB.ClaimScheduledRun(ctx, SchedulerClaim{
		JobID:                job.ID,
		RunID:                "run-scheduler-claim",
		FireID:               "fire-scheduler-claim",
		ScheduledAt:          nextRun,
		NextRunAt:            &nextAfterClaim,
		ClaimedAt:            claimedAt,
		ScheduleHash:         "hash-v1",
		NetworkParticipation: automationNamedParticipation("scheduled-ops"),
	})
	if err != nil {
		t.Fatalf("ClaimScheduledRun() error = %v", err)
	}
	if got, want := claim.State.NextRunAt, &nextAfterClaim; got == nil || !got.Equal(*want) {
		t.Fatalf("ClaimScheduledRun().State.NextRunAt = %v, want %v", got, want)
	}
	if got, want := claim.State.LastScheduledAt, &nextRun; got == nil || !got.Equal(*want) {
		t.Fatalf("ClaimScheduledRun().State.LastScheduledAt = %v, want %v", got, want)
	}
	if got, want := claim.Run.FireID, "fire-scheduler-claim"; got != want {
		t.Fatalf("ClaimScheduledRun().Run.FireID = %q, want %q", got, want)
	}
	if got, want := claim.Run.ScheduledAt, &nextRun; got == nil || !got.Equal(*want) {
		t.Fatalf("ClaimScheduledRun().Run.ScheduledAt = %v, want %v", got, want)
	}
	assertAutomationNamedParticipation(t, claim.Run.NetworkParticipation, "scheduled-ops")

	_, duplicateErr := globalDB.ClaimScheduledRun(ctx, SchedulerClaim{
		JobID:        job.ID,
		RunID:        "run-scheduler-duplicate",
		FireID:       "fire-scheduler-claim",
		ScheduledAt:  nextRun,
		NextRunAt:    &nextAfterClaim,
		ClaimedAt:    claimedAt,
		ScheduleHash: "hash-v1",
	})
	if !errors.Is(duplicateErr, automation.ErrScheduledFireAlreadyClaimed) {
		t.Fatalf("ClaimScheduledRun(duplicate) error = %v, want ErrScheduledFireAlreadyClaimed", duplicateErr)
	}

	recordedRun, err := globalDB.RecordRunDeliveryError(ctx, claim.Run.ID, errors.New("dispatch transport unavailable"))
	if err != nil {
		t.Fatalf("RecordRunDeliveryError() error = %v", err)
	}
	if got, want := recordedRun.DeliveryError, "dispatch transport unavailable"; got != want {
		t.Fatalf("RecordRunDeliveryError().DeliveryError = %q, want %q", got, want)
	}
	if recordedRun.Error != "" {
		t.Fatalf("RecordRunDeliveryError().Error = %q, want empty normal execution error", recordedRun.Error)
	}

	count, err := globalDB.CountRuns(ctx, RunQuery{JobID: job.ID, ExcludeID: claim.Run.ID})
	if err != nil {
		t.Fatalf("CountRuns(exclude claimed run) error = %v", err)
	}
	if count != 0 {
		t.Fatalf("CountRuns(exclude claimed run) = %d, want 0", count)
	}
}

func TestGlobalDBSchedulerClaimSuppressesSelfOverlapForOneFire(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	job, err := globalDB.CreateJob(
		ctx,
		automationJobForTest(automation.AutomationScopeGlobal, "overlap-guard", "", automation.JobSourceDynamic),
	)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	base := time.Date(2026, 4, 11, 9, 0, 0, 0, time.UTC)
	active, err := globalDB.CreateRun(
		ctx,
		automationRunForJob(job.ID, automation.RunRunning, 1, base.Add(-time.Minute)),
	)
	if err != nil {
		t.Fatalf("CreateRun(active) error = %v", err)
	}
	if _, err := globalDB.SaveSchedulerState(ctx, SchedulerState{
		JobID:         job.ID,
		NextRunAt:     &base,
		ScheduleHash:  "hash-overlap",
		CatchUpPolicy: automation.SchedulerCatchUpPolicyRunOnce,
		UpdatedAt:     base.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SaveSchedulerState() error = %v", err)
	}

	next := base.Add(time.Minute)
	skipped, err := globalDB.ClaimScheduledRun(ctx, SchedulerClaim{
		JobID:        job.ID,
		RunID:        "run-overlap-skip",
		FireID:       "fire-overlap-skip",
		ScheduledAt:  base,
		NextRunAt:    &next,
		ClaimedAt:    base.Add(time.Second),
		ScheduleHash: "hash-overlap",
	})
	if err != nil {
		t.Fatalf("ClaimScheduledRun(overlap) error = %v", err)
	}
	if !skipped.Skipped || skipped.SkipReason != automation.SchedulerSkipReasonSelfOverlap {
		t.Fatalf("ClaimScheduledRun(overlap) = %#v, want self-overlap skip", skipped)
	}
	if got, want := skipped.Run.Status, automation.RunCancelled; got != want {
		t.Fatalf("overlap run status = %q, want %q", got, want)
	}
	if got, want := skipped.Run.Metadata[automation.SchedulerSkipReasonMetadataKey], string(
		automation.SchedulerSkipReasonSelfOverlap,
	); got != want {
		t.Fatalf("overlap metadata reason = %#v, want %q", got, want)
	}

	active.Status = automation.RunCompleted
	active.EndedAt = &next
	if _, err := globalDB.UpdateRun(ctx, active); err != nil {
		t.Fatalf("UpdateRun(active completed) error = %v", err)
	}
	nextAfterNormal := next.Add(time.Minute)
	normal, err := globalDB.ClaimScheduledRun(ctx, SchedulerClaim{
		JobID:        job.ID,
		RunID:        "run-after-overlap",
		FireID:       "fire-after-overlap",
		ScheduledAt:  next,
		NextRunAt:    &nextAfterNormal,
		ClaimedAt:    next.Add(time.Second),
		ScheduleHash: "hash-overlap",
	})
	if err != nil {
		t.Fatalf("ClaimScheduledRun(after overlap) error = %v", err)
	}
	if normal.Skipped {
		t.Fatalf("ClaimScheduledRun(after overlap).Skipped = true, want false: %#v", normal)
	}
	if got, want := normal.Run.Status, automation.RunScheduled; got != want {
		t.Fatalf("normal run status = %q, want %q", got, want)
	}

	_, duplicateErr := globalDB.ClaimScheduledRun(ctx, SchedulerClaim{
		JobID:        job.ID,
		RunID:        "run-after-overlap-duplicate",
		FireID:       normal.Run.FireID,
		ScheduledAt:  next,
		NextRunAt:    &nextAfterNormal,
		ClaimedAt:    next.Add(2 * time.Second),
		ScheduleHash: "hash-overlap",
	})
	if !errors.Is(duplicateErr, automation.ErrScheduledFireAlreadyClaimed) {
		t.Fatalf("ClaimScheduledRun(duplicate) error = %v, want ErrScheduledFireAlreadyClaimed", duplicateErr)
	}
}

func TestGlobalDBSchedulerClaimPreventsDuplicateAfterReopen(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
	first, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(first) error = %v", err)
	}

	job, err := first.CreateJob(
		ctx,
		automationJobForTest(automation.AutomationScopeGlobal, "durable-reopen", "", automation.JobSourceDynamic),
	)
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	scheduledAt := time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC)
	nextRunAt := scheduledAt.Add(24 * time.Hour)
	_, err = first.SaveSchedulerState(ctx, SchedulerState{
		JobID:         job.ID,
		NextRunAt:     &scheduledAt,
		ScheduleHash:  "hash-reopen",
		CatchUpPolicy: automation.SchedulerCatchUpPolicySkipMissed,
		UpdatedAt:     scheduledAt.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("SaveSchedulerState() error = %v", err)
	}

	claim, err := first.ClaimScheduledRun(ctx, SchedulerClaim{
		JobID:        job.ID,
		RunID:        "run-durable-reopen",
		FireID:       "fire-durable-reopen",
		ScheduledAt:  scheduledAt,
		NextRunAt:    &nextRunAt,
		ClaimedAt:    scheduledAt.Add(time.Second),
		ScheduleHash: "hash-reopen",
	})
	if err != nil {
		t.Fatalf("ClaimScheduledRun() error = %v", err)
	}
	if err := first.Close(ctx); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	second, err := OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB(second) error = %v", err)
	}
	t.Cleanup(func() {
		if err := second.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(second) error = %v", err)
		}
	})

	state, err := second.GetSchedulerState(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetSchedulerState() after reopen error = %v", err)
	}
	if got, want := state.LastFireID, claim.Run.FireID; got != want {
		t.Fatalf("GetSchedulerState().LastFireID = %q, want %q", got, want)
	}
	if got, want := state.NextRunAt, &nextRunAt; got == nil || !got.Equal(*want) {
		t.Fatalf("GetSchedulerState().NextRunAt = %v, want %v", got, want)
	}

	_, duplicateErr := second.ClaimScheduledRun(ctx, SchedulerClaim{
		JobID:        job.ID,
		RunID:        "run-durable-reopen-duplicate",
		FireID:       claim.Run.FireID,
		ScheduledAt:  scheduledAt,
		NextRunAt:    &nextRunAt,
		ClaimedAt:    scheduledAt.Add(2 * time.Second),
		ScheduleHash: "hash-reopen",
	})
	if !errors.Is(duplicateErr, automation.ErrScheduledFireAlreadyClaimed) {
		t.Fatalf(
			"ClaimScheduledRun(duplicate after reopen) error = %v, want ErrScheduledFireAlreadyClaimed",
			duplicateErr,
		)
	}

	runs, err := second.ListRuns(ctx, RunQuery{JobID: job.ID})
	if err != nil {
		t.Fatalf("ListRuns() after duplicate claim error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(ListRuns()) = %d, want %d", got, want)
	}
	if got, want := runs[0].FireID, claim.Run.FireID; got != want {
		t.Fatalf("runs[0].FireID = %q, want %q", got, want)
	}
}

type seededAutomationCatalogDefinitions struct {
	jobs     []Job
	triggers []Trigger
}

func seedAutomationCatalogDefinitionsForTest(
	t *testing.T,
	globalDB *GlobalDB,
	jobs []Job,
	triggers []Trigger,
) seededAutomationCatalogDefinitions {
	t.Helper()
	ctx := testutil.Context(t)
	tx, err := globalDB.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx(seed automation catalog) error = %v", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			t.Errorf("Rollback(seed automation catalog) error = %v", rollbackErr)
		}
	}()
	seeded := seededAutomationCatalogDefinitions{
		jobs:     make([]Job, 0, len(jobs)),
		triggers: make([]Trigger, 0, len(triggers)),
	}
	for index, job := range jobs {
		normalized, normalizeErr := globalDB.normalizeJobForCreate(job)
		if normalizeErr != nil {
			t.Fatalf("normalizeJobForCreate(seed %d) error = %v", index, normalizeErr)
		}
		if insertErr := globalDB.insertJob(ctx, tx, normalized); insertErr != nil {
			t.Fatalf("insertJob(seed %d) error = %v", index, insertErr)
		}
		if projectionErr := upsertAutomationJobCatalog(ctx, tx, normalized); projectionErr != nil {
			t.Fatalf("upsertAutomationJobCatalog(seed %d) error = %v", index, projectionErr)
		}
		seeded.jobs = append(seeded.jobs, normalized)
	}
	for index, trigger := range triggers {
		normalized, normalizeErr := globalDB.normalizeTriggerForCreate(trigger)
		if normalizeErr != nil {
			t.Fatalf("normalizeTriggerForCreate(seed %d) error = %v", index, normalizeErr)
		}
		if insertErr := globalDB.insertTrigger(ctx, tx, normalized); insertErr != nil {
			t.Fatalf("insertTrigger(seed %d) error = %v", index, insertErr)
		}
		if projectionErr := upsertAutomationTriggerCatalog(ctx, tx, normalized); projectionErr != nil {
			t.Fatalf("upsertAutomationTriggerCatalog(seed %d) error = %v", index, projectionErr)
		}
		seeded.triggers = append(seeded.triggers, normalized)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit(seed automation catalog) error = %v", err)
	}
	committed = true
	return seeded
}

func automationJobForTest(
	scope automation.Scope,
	name string,
	workspaceID string,
	source automation.JobSource,
) Job {
	createdAt := time.Date(2026, 4, 10, 18, 0, 0, 0, time.UTC)
	return Job{
		Scope:       scope,
		Name:        name,
		AgentName:   "researcher",
		WorkspaceID: workspaceID,
		Prompt:      "Summarize the latest automation state.",
		Schedule: &automation.ScheduleSpec{
			Mode:     automation.ScheduleModeEvery,
			Interval: "30m",
		},
		Enabled:   true,
		Retry:     automation.DefaultRetryConfig(),
		FireLimit: automation.DefaultFireLimitConfig(),
		Source:    source,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

func automationWebhookTriggerForTest(
	scope automation.Scope,
	name string,
	workspaceID string,
	source automation.JobSource,
) Trigger {
	createdAt := time.Date(2026, 4, 10, 18, 5, 0, 0, time.UTC)
	return Trigger{
		Scope:            scope,
		Name:             name,
		AgentName:        "reviewer",
		WorkspaceID:      workspaceID,
		Prompt:           `Review webhook payload {{ index .Data "payload" }}`,
		Event:            "webhook",
		Enabled:          true,
		Retry:            automation.DefaultRetryConfig(),
		FireLimit:        automation.DefaultFireLimitConfig(),
		Source:           source,
		WebhookID:        "wbh_default",
		EndpointSlug:     "endpoint-default",
		WebhookSecretRef: "vault:automation/triggers/test/webhook-secret",
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
	}
}

func automationNonWebhookTriggerForTest(
	scope automation.Scope,
	name string,
	workspaceID string,
	source automation.JobSource,
) Trigger {
	createdAt := time.Date(2026, 4, 10, 18, 10, 0, 0, time.UTC)
	return Trigger{
		Scope:       scope,
		Name:        name,
		AgentName:   "reviewer",
		WorkspaceID: workspaceID,
		Prompt:      `Summarize session {{ index .Data "session_id" }}`,
		Event:       "session.stopped",
		Filter: map[string]string{
			"data.stop_reason": "completed",
		},
		Enabled:   true,
		Retry:     automation.DefaultRetryConfig(),
		FireLimit: automation.DefaultFireLimitConfig(),
		Source:    source,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

func automationRunForJob(jobID string, status automation.RunStatus, attempt int, startedAt time.Time) Run {
	endedAt := startedAt.Add(time.Minute)
	return Run{
		JobID:     jobID,
		Status:    status,
		Attempt:   attempt,
		StartedAt: timePointer(startedAt),
		EndedAt:   timePointer(endedAt),
	}
}

func automationRunForTrigger(triggerID string, status automation.RunStatus, attempt int, startedAt time.Time) Run {
	endedAt := startedAt.Add(time.Minute)
	return Run{
		TriggerID: triggerID,
		Status:    status,
		Attempt:   attempt,
		StartedAt: timePointer(startedAt),
		EndedAt:   timePointer(endedAt),
	}
}

func automationSuggestionForTest(id string, workspaceID string, dedupKey string) automation.Suggestion {
	return automation.Suggestion{
		ID:          id,
		WorkspaceID: workspaceID,
		Source:      automation.SuggestionSourceCatalog,
		DedupKey:    dedupKey,
		Status:      automation.SuggestionStatusPending,
		Payload: Job{
			ID:          "job-" + id,
			Scope:       automation.AutomationScopeWorkspace,
			Name:        "Suggested job",
			TargetKind:  automation.TargetKindAgent,
			AgentName:   "general",
			WorkspaceID: workspaceID,
			Prompt:      "Review the workspace.",
			Schedule: &automation.ScheduleSpec{
				Mode:          automation.ScheduleModeCron,
				Expr:          "0 8 * * *",
				CatchUpPolicy: automation.SchedulerCatchUpPolicyRunOnce,
			},
			Enabled:   true,
			Retry:     automation.DefaultRetryConfig(),
			FireLimit: automation.DefaultFireLimitConfig(),
			Source:    automation.JobSourceDynamic,
		},
	}
}

func timePointer(value time.Time) *time.Time {
	timestamp := value
	return &timestamp
}

type countingAutomationCatalogExecutor struct {
	automationCatalogExecutor
	reads             int
	hydrationPayloads []string
}

func (e *countingAutomationCatalogExecutor) QueryContext(
	ctx context.Context,
	query string,
	args ...any,
) (*sql.Rows, error) {
	e.reads++
	if strings.Contains(query, "WITH requested(") && len(args) > 0 {
		if payload, ok := args[0].(string); ok {
			e.hydrationPayloads = append(e.hydrationPayloads, payload)
		}
	}
	return e.automationCatalogExecutor.QueryContext(ctx, query, args...)
}

func (e *countingAutomationCatalogExecutor) QueryRowContext(
	ctx context.Context,
	query string,
	args ...any,
) *sql.Row {
	e.reads++
	return e.automationCatalogExecutor.QueryRowContext(ctx, query, args...)
}

func assertAutomationCatalogReadCardinality(
	t *testing.T,
	executor *countingAutomationCatalogExecutor,
	wantHydrated int,
) {
	t.Helper()
	if got, want := executor.reads, 3; got != want {
		t.Fatalf("automation catalog read statements = %d, want constant %d", got, want)
	}
	if got, want := len(executor.hydrationPayloads), 1; got != want {
		t.Fatalf("automation catalog hydration batches = %d, want %d", got, want)
	}
	var ids []string
	if err := json.Unmarshal([]byte(executor.hydrationPayloads[0]), &ids); err != nil {
		t.Fatalf("json.Unmarshal(automation catalog hydration ids) error = %v", err)
	}
	if got := len(ids); got != wantHydrated {
		t.Fatalf("automation catalog hydrated ids = %d, want %d", got, wantHydrated)
	}
	if len(ids) > automation.MaxListLimit {
		t.Fatalf("automation catalog hydrated ids = %d, max %d", len(ids), automation.MaxListLimit)
	}
}

func automationCatalogQueryPlan(t *testing.T, db *sql.DB, statement string, args ...any) string {
	t.Helper()
	rows, err := db.QueryContext(testutil.Context(t), "EXPLAIN QUERY PLAN "+statement, args...)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN automation catalog error = %v", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Errorf("close automation catalog query plan rows error = %v", closeErr)
		}
	}()
	details := make([]string, 0)
	for rows.Next() {
		var id int
		var parent int
		var unused int
		var detail string
		if scanErr := rows.Scan(&id, &parent, &unused, &detail); scanErr != nil {
			t.Fatalf("scan automation catalog query plan error = %v", scanErr)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate automation catalog query plan error = %v", err)
	}
	return strings.Join(details, "\n")
}

func assertIndexesPresent(t *testing.T, db *sql.DB, table string, want ...string) {
	t.Helper()

	rows, err := db.QueryContext(testutil.Context(t), "PRAGMA index_list("+table+")")
	if err != nil {
		t.Fatalf("QueryContext(index_list %q) error = %v", table, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Errorf("rows.Close(index_list %q) error = %v", table, closeErr)
		}
	}()

	got := make(map[string]struct{})
	for rows.Next() {
		var (
			seq     int
			name    string
			unique  int
			origin  string
			partial int
		)
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("Scan(index_list %q) error = %v", table, err)
		}
		got[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err(index_list %q) error = %v", table, err)
	}

	for _, indexName := range want {
		if _, ok := got[indexName]; !ok {
			t.Fatalf("index %q missing from %s indexes %#v", indexName, table, got)
		}
	}
}
