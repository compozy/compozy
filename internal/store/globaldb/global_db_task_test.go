package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func TestOpenGlobalDBCreatesTaskSchemaAndIndexes(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	assertTablesPresent(
		t,
		globalDB.db,
		"tasks",
		"task_blocks",
		"task_block_recurrences",
		"task_triage_state",
		"task_runs",
		"task_run_reviews",
		"task_run_required_capabilities",
		"task_run_preferred_capabilities",
		"task_dependencies",
		"task_events",
		"task_run_idempotency",
	)
	assertTableColumns(t, globalDB.db, "tasks", []string{
		"id",
		"identifier",
		"scope",
		"workspace_id",
		"parent_task_id",
		"title",
		"description",
		"priority",
		"max_attempts",
		"status",
		"approval_policy",
		"approval_state",
		"owner_kind",
		"owner_ref",
		"created_by_kind",
		"created_by_ref",
		"origin_kind",
		"origin_ref",
		"created_at",
		"updated_at",
		"closed_at",
		"metadata_json",
		"current_run_id",
		"paused",
		"paused_by",
		"paused_at",
		"paused_reason",
		"max_runtime_seconds",
		"spawn_failure_count",
		"last_spawn_error",
		"review_policy",
		"review_max_rounds",
		"review_round",
		"last_review_id",
		"last_review_outcome",
		"review_circuit_opened_at",
		"review_circuit_reason",
		"auto_enqueue_on_ready",
		"needs_attention_reason",
		"needs_attention_at",
		"needs_attention_by_kind",
		"needs_attention_by_ref",
		"wake_creator",
	})
	assertTableColumns(t, globalDB.db, "task_blocks", []string{
		"id",
		"workspace_id",
		"task_id",
		"kind",
		"reason",
		"details_json",
		"created_by_kind",
		"created_by_ref",
		"created_at",
		"expires_at",
		"cleared_at",
		"cleared_by_kind",
		"cleared_by_ref",
		"clear_note",
	})
	assertTableColumns(t, globalDB.db, "task_block_recurrences", []string{
		"task_id",
		"kind",
		"count",
		"updated_at",
	})
	assertTableColumns(t, globalDB.db, "task_triage_state", []string{
		"task_id",
		"actor_kind",
		"actor_id",
		"is_read",
		"archived",
		"dismissed",
		"last_seen_activity_at",
		"updated_at",
	})
	assertTableColumns(t, globalDB.db, "task_runs", []string{
		"id",
		"task_id",
		"workspace_id",
		"status",
		"attempt",
		"recovery_count",
		"previous_run_id",
		"failure_kind",
		"claimed_by_kind",
		"claimed_by_ref",
		"session_id",
		"origin_kind",
		"origin_ref",
		"idempotency_key",
		"network_spec_json",
		"network_mode",
		"network_channel",
		"network_source",
		"designation_group_id",
		"queued_at",
		"claimed_at",
		"started_at",
		"ended_at",
		"error",
		"metadata_json",
		"result_json",
		"summary",
		"claimed_agent_name",
		"claimed_peer_id",
		"terminalized_by_session_id",
		"terminalized_by_agent_name",
		"terminalized_by_peer_id",
		"terminalized_by_actor_kind",
		"terminalized_by_actor_ref",
		"review_required",
		"review_request_round",
		"review_policy_snapshot",
		"review_request_id",
		"parent_run_id",
		"review_id",
		"review_round",
		"continuation_reason",
		"missing_work_json",
		"next_round_guidance",
		"claim_token",
		"claim_token_hash",
		"lease_until",
		"heartbeat_at",
		"run_kind",
		"loop_run_id",
		"tokens_used",
		"network_wake_id",
		"network_target_session_id",
		"network_owner_key",
	})
	assertTableColumns(t, globalDB.db, "task_run_required_capabilities", []string{
		"run_id",
		"capability_id",
	})
	assertTableColumns(t, globalDB.db, "task_run_preferred_capabilities", []string{
		"run_id",
		"capability_id",
	})
	assertTableColumns(t, globalDB.db, "task_dependencies", []string{
		"task_id",
		"depends_on_task_id",
		"kind",
		"created_at",
	})
	assertTableColumns(t, globalDB.db, "task_events", []string{
		"id",
		"event_seq",
		"task_id",
		"run_id",
		"event_type",
		"actor_kind",
		"actor_id",
		"origin_kind",
		"origin_ref",
		"payload_json",
		"timestamp",
	})
	assertTableColumns(t, globalDB.db, "task_run_idempotency", []string{
		"idempotency_key",
		"origin_kind",
		"origin_ref",
		"run_id",
		"created_at",
	})
	assertIndexesPresent(t, globalDB.db, "tasks",
		"idx_tasks_scope",
		"idx_tasks_workspace",
		"idx_tasks_status",
		"idx_tasks_priority",
		"idx_tasks_approval_state",
		"idx_tasks_parent",
		"idx_tasks_owner",
		"idx_tasks_current_run",
		"idx_tasks_paused",
		"idx_tasks_review_policy",
		"idx_tasks_review_round",
		"idx_tasks_created_by",
	)
	assertIndexesPresent(t, globalDB.db, "task_blocks",
		"idx_task_blocks_open",
		"idx_task_blocks_expiry",
	)
	assertIndexesPresent(t, globalDB.db, "task_triage_state",
		"idx_task_triage_task",
		"idx_task_triage_actor",
	)
	assertIndexesPresent(t, globalDB.db, "task_runs",
		"idx_task_runs_task",
		"idx_task_runs_task_status",
		"idx_task_runs_status",
		"idx_task_runs_previous",
		"idx_task_runs_session",
		"idx_task_runs_channel",
		"idx_task_runs_pending_claim",
		"idx_task_runs_active_lease_recovery",
		"idx_task_runs_session_status",
		"idx_task_runs_parent_run",
		"idx_task_runs_review_request",
		"uq_task_runs_review_id",
		"idx_task_runs_task_review_round",
		"idx_task_runs_designation_group",
		"idx_task_runs_target_session",
		"uq_task_runs_active_loop_coordinator",
	)
	assertIndexesPresent(t, globalDB.db, "task_run_required_capabilities",
		"idx_task_run_required_capabilities_capability",
	)
	assertIndexesPresent(t, globalDB.db, "task_run_preferred_capabilities",
		"idx_task_run_preferred_capabilities_capability",
	)
	assertIndexesPresent(t, globalDB.db, "task_dependencies",
		"idx_task_dependencies_task",
		"idx_task_dependencies_depends_on",
	)
	assertIndexesPresent(t, globalDB.db, "task_events",
		"idx_task_events_task",
		"idx_task_events_run",
		"idx_task_events_type",
		"uq_task_events_event_seq",
		"idx_task_events_task_seq",
		"idx_task_events_type_seq",
	)
	assertIndexesPresent(t, globalDB.db, "task_run_idempotency",
		"idx_task_run_idempotency_run",
	)
	assertIndexSQLContains(t, globalDB.db, "idx_task_blocks_open", "WHERE cleared_at IS NULL")
	assertIndexSQLContains(t, globalDB.db, "idx_task_blocks_expiry", "expires_at IS NOT NULL")
	assertIndexSQLContains(t, globalDB.db, "idx_task_runs_target_session", "run_kind = 'network_wake'")
	assertIndexSQLContains(t, globalDB.db, "uq_task_runs_active_loop_coordinator", "run_kind = 'coordinator'")
	assertIndexSQLContains(
		t,
		globalDB.db,
		"uq_task_runs_active_loop_coordinator",
		"status IN ('queued', 'claimed', 'starting', 'running')",
	)
	assertTableSQLContains(t, globalDB.db, "task_blocks", "CHECK (kind IN ('needs_input','capability','transient'))")
	assertTableSQLContains(t, globalDB.db, "task_blocks", "CHECK (length(reason) > 0)")
	assertTasksStatusAcceptsNeedsAttention(t, globalDB, "task-schema-needs-attention")
	assertTaskRunTokensUsedRejectsNegativeValues(t, globalDB)
}

func assertTaskRunTokensUsedRejectsNegativeValues(t *testing.T, globalDB *GlobalDB) {
	t.Helper()

	ctx := testutil.Context(t)
	taskRecord := taskRecordForTest("task-schema-tokens-used")
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask(tokens constraint) error = %v", err)
	}
	run := taskRunForTest("run-schema-tokens-used", taskRecord.ID)
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun(tokens constraint) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`UPDATE task_runs SET tokens_used = -1 WHERE id = ?`,
		run.ID,
	); err == nil || !strings.Contains(err.Error(), "CHECK constraint failed") {
		t.Fatalf("negative tokens_used update error = %v, want check constraint failure", err)
	}
}

func TestGlobalDBTaskRunsCoordinatorExclusivityIndex(t *testing.T) {
	t.Parallel()

	t.Run("Should reject concurrent active coordinators for one loop run", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		taskRecord := taskRecordForTest("task-loop-coordinator-index")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		insertLoopRunForCoordinatorIndexTest(ctx, t, globalDB.db, "loop-run-coordinator-index")

		errs := make(chan error, 2)
		start := make(chan struct{})
		for _, runID := range []string{"run-loop-coordinator-a", "run-loop-coordinator-b"} {
			go func(runID string) {
				<-start
				errs <- insertTaskRunForCoordinatorIndexTest(
					ctx,
					globalDB.db,
					runID,
					taskRecord.ID,
					"loop-run-coordinator-index",
					"coordinator",
					"queued",
				)
			}(runID)
		}
		close(start)

		successes := 0
		uniqueFailures := 0
		for range 2 {
			err := <-errs
			if err == nil {
				successes++
				continue
			}
			if strings.Contains(err.Error(), "UNIQUE constraint failed: task_runs.loop_run_id") {
				uniqueFailures++
				continue
			}
			t.Fatalf("insert active coordinator error = %v, want unique constraint failure or success", err)
		}
		if successes != 1 || uniqueFailures != 1 {
			t.Fatalf("active coordinator inserts: successes=%d uniqueFailures=%d, want 1/1", successes, uniqueFailures)
		}

		if err := insertTaskRunForCoordinatorIndexTest(
			ctx,
			globalDB.db,
			"run-loop-worker-same-loop",
			taskRecord.ID,
			"loop-run-coordinator-index",
			"worker",
			"queued",
		); err != nil {
			t.Fatalf("insert worker with same loop_run_id error = %v, want partial index to ignore workers", err)
		}
		if err := insertTaskRunForCoordinatorIndexTest(
			ctx,
			globalDB.db,
			"run-loop-coordinator-terminal",
			taskRecord.ID,
			"loop-run-coordinator-index",
			"coordinator",
			"completed",
		); err != nil {
			t.Fatalf("insert terminal coordinator error = %v, want partial index to ignore terminal runs", err)
		}
	})
}

func TestOpenGlobalDBTaskBlockSchemaSurvivesRestart(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve task block schema across double boot", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		dbPath := filepath.Join(t.TempDir(), GlobalDatabaseName)
		first, err := OpenGlobalDB(ctx, dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := first.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(first cleanup) error = %v", closeErr)
			}
		})
		assertTaskBlockingSchema(t, first.db)
		assertTasksStatusAcceptsNeedsAttention(t, first, "task-block-first-boot")
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		second, err := OpenGlobalDB(ctx, dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(second) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := second.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(second) error = %v", closeErr)
			}
		})
		assertTaskBlockingSchema(t, second.db)
		assertTasksStatusAcceptsNeedsAttention(t, second, "task-block-second-boot")
	})
}

func TestOpenGlobalDBTaskRunClaimIndexesSupportPlannedScans(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	assertQueryPlanUsesIndex(
		t,
		globalDB.db,
		`SELECT id
		 FROM task_runs
		 WHERE status = ? AND (lease_until IS NULL OR lease_until <= ?)
		 ORDER BY queued_at ASC, id ASC`,
		"idx_task_runs_pending_claim",
		taskpkg.TaskRunStatusQueued.String(),
		"2026-04-26T12:00:00Z",
	)
	assertQueryPlanUsesIndex(
		t,
		globalDB.db,
		`SELECT id
		 FROM task_runs
		 WHERE status = ? AND lease_until <= ? AND heartbeat_at <= ?`,
		"idx_task_runs_active_lease_recovery",
		taskpkg.TaskRunStatusClaimed.String(),
		"2026-04-26T12:00:00Z",
		"2026-04-26T11:59:00Z",
	)
	assertQueryPlanUsesIndex(
		t,
		globalDB.db,
		`SELECT run_id
		 FROM task_run_required_capabilities
		 WHERE capability_id = ?`,
		"idx_task_run_required_capabilities_capability",
		"golang",
	)
	assertQueryPlanUsesIndex(
		t,
		globalDB.db,
		`SELECT run_id
		 FROM task_run_preferred_capabilities
		 WHERE capability_id = ?`,
		"idx_task_run_preferred_capabilities_capability",
		"codex",
	)
}

func TestGlobalDBTaskRoundTripPreservesNullableFields(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"task-roundtrip-workspace",
		filepath.Join(t.TempDir(), "workspace"),
	)

	parent := taskRecordForTest("task-parent")
	parent.Metadata = json.RawMessage(`{"kind":"global"}`)
	if err := globalDB.CreateTask(testutil.Context(t), parent); err != nil {
		t.Fatalf("CreateTask(parent) error = %v", err)
	}

	child := taskRecordForTest("task-child")
	child.Scope = taskpkg.ScopeWorkspace
	child.WorkspaceID = workspaceID
	child.ParentTaskID = parent.ID
	child.Priority = taskpkg.PriorityUrgent
	child.MaxAttempts = 5
	child.AutoEnqueueOnReady = true
	child.ApprovalPolicy = taskpkg.ApprovalPolicyManual
	child.ApprovalState = taskpkg.ApprovalStateApproved
	child.Owner = ownershipForTest(taskpkg.OwnerKindHuman, "alice")
	child.Metadata = json.RawMessage(`{"kind":"workspace"}`)
	if err := globalDB.CreateTask(testutil.Context(t), child); err != nil {
		t.Fatalf("CreateTask(child) error = %v", err)
	}

	gotParent, err := globalDB.GetTask(testutil.Context(t), parent.ID)
	if err != nil {
		t.Fatalf("GetTask(parent) error = %v", err)
	}
	assertTaskEqual(t, gotParent, parent)
	if gotParent.WorkspaceID != "" {
		t.Fatalf("GetTask(parent).WorkspaceID = %q, want empty", gotParent.WorkspaceID)
	}
	if gotParent.ParentTaskID != "" {
		t.Fatalf("GetTask(parent).ParentTaskID = %q, want empty", gotParent.ParentTaskID)
	}
	if gotParent.Owner != nil {
		t.Fatalf("GetTask(parent).Owner = %#v, want nil", gotParent.Owner)
	}

	gotChild, err := globalDB.GetTask(testutil.Context(t), child.ID)
	if err != nil {
		t.Fatalf("GetTask(child) error = %v", err)
	}
	assertTaskEqual(t, gotChild, child)

	child.Title = "Updated child"
	child.Description = "Updated description"
	child.Priority = taskpkg.PriorityHigh
	child.MaxAttempts = 4
	child.Status = taskpkg.TaskStatusInProgress
	child.ApprovalPolicy = taskpkg.ApprovalPolicyNone
	child.ApprovalState = taskpkg.ApprovalStateNotRequired
	child.Owner = ownershipForTest(taskpkg.OwnerKindAgentSession, "sess-1")
	child.Metadata = json.RawMessage(`{"kind":"updated"}`)
	child.UpdatedAt = child.UpdatedAt.Add(2 * time.Minute)
	if err := globalDB.UpdateTask(testutil.Context(t), child, coordinatorActorContextForTest()); err != nil {
		t.Fatalf("UpdateTask(child) error = %v", err)
	}
	gotChild, err = globalDB.GetTask(testutil.Context(t), child.ID)
	if err != nil {
		t.Fatalf("GetTask(updated child) error = %v", err)
	}
	assertTaskEqual(t, gotChild, child)

	summaries, err := globalDB.ListTasks(testutil.Context(t), taskpkg.Query{ParentTaskID: parent.ID})
	if err != nil {
		t.Fatalf("ListTasks(parent filter) error = %v", err)
	}
	if got, want := len(summaries), 1; got != want {
		t.Fatalf("len(ListTasks(parent filter)) = %d, want %d", got, want)
	}
	assertTaskSummaryMatchesTask(t, &summaries[0], child)

	children, err := globalDB.CountDirectChildren(testutil.Context(t), parent.ID)
	if err != nil {
		t.Fatalf("CountDirectChildren() error = %v", err)
	}
	if got, want := children, 1; got != want {
		t.Fatalf("CountDirectChildren() = %d, want %d", got, want)
	}
}

func TestGlobalDBDeleteTaskMapsChildConstraintToValidationError(t *testing.T) {
	t.Parallel()

	t.Run("ShouldMapChildConstraintFailuresToTaskValidationErrors", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)

		parent := taskRecordForTest("task-parent-delete")
		if err := globalDB.CreateTask(testutil.Context(t), parent); err != nil {
			t.Fatalf("CreateTask(parent) error = %v", err)
		}

		child := taskRecordForTest("task-child-delete")
		child.ParentTaskID = parent.ID
		if err := globalDB.CreateTask(testutil.Context(t), child); err != nil {
			t.Fatalf("CreateTask(child) error = %v", err)
		}

		err := globalDB.DeleteTask(testutil.Context(t), parent.ID)
		if !errors.Is(err, taskpkg.ErrValidation) {
			t.Fatalf("DeleteTask(parent) error = %v, want %v", err, taskpkg.ErrValidation)
		}
		if strings.Contains(strings.ToLower(err.Error()), "foreign key constraint failed") {
			t.Fatalf("DeleteTask(parent) error = %q, want mapped task validation error", err.Error())
		}
	})
}

func TestDeleteTaskTransactionStoreDelegatesTaskStateReadsAndMutations(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)

	parent := taskRecordForTest("task-tx-parent")
	if err := globalDB.CreateTask(ctx, parent); err != nil {
		t.Fatalf("CreateTask(parent) error = %v", err)
	}
	child := taskRecordForTest("task-tx-child")
	child.ParentTaskID = parent.ID
	if err := globalDB.CreateTask(ctx, child); err != nil {
		t.Fatalf("CreateTask(child) error = %v", err)
	}
	dependency := taskpkg.Dependency{
		TaskID:          child.ID,
		DependsOnTaskID: parent.ID,
		Kind:            taskpkg.DependencyKindBlocks,
		CreatedAt:       child.CreatedAt.Add(time.Minute),
	}
	if err := globalDB.CreateDependency(ctx, dependency); err != nil {
		t.Fatalf("CreateDependency() error = %v", err)
	}
	run := taskRunForTest("run-tx-child", child.ID)
	if err := globalDB.CreateTaskRun(ctx, run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}

	txStore := &deleteTaskTxStore{tasks: globalDB.TaskRepo, exec: globalDB.db}
	gotChild, err := txStore.GetTask(ctx, child.ID)
	if err != nil {
		t.Fatalf("txStore.GetTask() error = %v", err)
	}
	assertTaskEqual(t, gotChild, child)

	child.Title = "Updated by transaction store"
	child.UpdatedAt = child.UpdatedAt.Add(2 * time.Minute)
	if err := txStore.UpdateTask(ctx, child, coordinatorActorContextForTest()); err != nil {
		t.Fatalf("txStore.UpdateTask() error = %v", err)
	}
	updatedChild, err := globalDB.GetTask(ctx, child.ID)
	if err != nil {
		t.Fatalf("GetTask(updated child) error = %v", err)
	}
	assertTaskEqual(t, updatedChild, child)

	children, err := txStore.CountDirectChildren(ctx, parent.ID)
	if err != nil {
		t.Fatalf("txStore.CountDirectChildren() error = %v", err)
	}
	if got, want := children, 1; got != want {
		t.Fatalf("txStore.CountDirectChildren() = %d, want %d", got, want)
	}
	dependencies, err := txStore.ListDependencies(ctx, child.ID)
	if err != nil {
		t.Fatalf("txStore.ListDependencies() error = %v", err)
	}
	if got, want := len(dependencies), 1; got != want {
		t.Fatalf("len(txStore.ListDependencies()) = %d, want %d", got, want)
	}
	dependents, err := txStore.ListDependents(ctx, parent.ID)
	if err != nil {
		t.Fatalf("txStore.ListDependents() error = %v", err)
	}
	if got, want := len(dependents), 1; got != want {
		t.Fatalf("len(txStore.ListDependents()) = %d, want %d", got, want)
	}
	runs, err := txStore.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: child.ID})
	if err != nil {
		t.Fatalf("txStore.ListTaskRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(txStore.ListTaskRuns()) = %d, want %d", got, want)
	}
	assertTaskRunEqual(t, runs[0], run)

	if err := txStore.DeleteTask(ctx, child.ID); err != nil {
		t.Fatalf("txStore.DeleteTask() error = %v", err)
	}
	if _, err := globalDB.GetTask(ctx, child.ID); !errors.Is(err, taskpkg.ErrTaskNotFound) {
		t.Fatalf("GetTask(deleted child) error = %v, want ErrTaskNotFound", err)
	}
}

func TestGlobalDBCreateAndUpdateTaskRejectInvalidScopeBindings(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"invalid-scope-workspace",
		filepath.Join(t.TempDir(), "workspace"),
	)

	t.Run("Should reject creating global task with workspace", func(t *testing.T) {
		t.Parallel()

		record := taskRecordForTest("task-invalid-create-global")
		record.WorkspaceID = workspaceID

		err := globalDB.CreateTask(testutil.Context(t), record)
		if !errors.Is(err, taskpkg.ErrInvalidScopeBinding) {
			t.Fatalf("CreateTask(global with workspace) error = %v, want ErrInvalidScopeBinding", err)
		}
	})

	t.Run("Should reject creating workspace task without workspace", func(t *testing.T) {
		t.Parallel()

		record := taskRecordForTest("task-invalid-create-workspace")
		record.Scope = taskpkg.ScopeWorkspace

		err := globalDB.CreateTask(testutil.Context(t), record)
		if !errors.Is(err, taskpkg.ErrInvalidScopeBinding) {
			t.Fatalf("CreateTask(workspace without workspace_id) error = %v, want ErrInvalidScopeBinding", err)
		}
	})

	t.Run("Should reject updating global task with workspace", func(t *testing.T) {
		t.Parallel()

		record := taskRecordForTest("task-invalid-update-global")
		if err := globalDB.CreateTask(testutil.Context(t), record); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		record.WorkspaceID = workspaceID
		record.UpdatedAt = record.UpdatedAt.Add(time.Minute)
		err := globalDB.UpdateTask(testutil.Context(t), record, coordinatorActorContextForTest())
		if !errors.Is(err, taskpkg.ErrInvalidScopeBinding) {
			t.Fatalf("UpdateTask(global with workspace) error = %v, want ErrInvalidScopeBinding", err)
		}
	})

	t.Run("Should reject updating workspace task without workspace", func(t *testing.T) {
		t.Parallel()

		record := taskRecordForTest("task-invalid-update-workspace")
		record.Scope = taskpkg.ScopeWorkspace
		record.WorkspaceID = workspaceID
		if err := globalDB.CreateTask(testutil.Context(t), record); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		record.WorkspaceID = ""
		record.UpdatedAt = record.UpdatedAt.Add(time.Minute)
		err := globalDB.UpdateTask(testutil.Context(t), record, coordinatorActorContextForTest())
		if !errors.Is(err, taskpkg.ErrInvalidScopeBinding) {
			t.Fatalf("UpdateTask(workspace without workspace_id) error = %v, want ErrInvalidScopeBinding", err)
		}
	})
}

func TestGlobalDBListTasksFilters(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	workspaceA := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"task-filter-a",
		filepath.Join(t.TempDir(), "workspace-a"),
	)
	workspaceB := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"task-filter-b",
		filepath.Join(t.TempDir(), "workspace-b"),
	)

	globalTask := taskRecordForTest("task-filter-global")
	globalTask.Status = taskpkg.TaskStatusPending

	readyTask := taskRecordForTest("task-filter-ready")
	readyTask.CreatedAt = readyTask.CreatedAt.Add(time.Minute)
	readyTask.UpdatedAt = readyTask.UpdatedAt.Add(time.Minute)
	readyTask.Scope = taskpkg.ScopeWorkspace
	readyTask.WorkspaceID = workspaceA
	readyTask.Status = taskpkg.TaskStatusReady
	readyTask.Priority = taskpkg.PriorityHigh
	readyTask.ApprovalPolicy = taskpkg.ApprovalPolicyManual
	readyTask.ApprovalState = taskpkg.ApprovalStateApproved
	readyTask.Owner = ownershipForTest(taskpkg.OwnerKindHuman, "alice")

	childTask := taskRecordForTest("task-filter-child")
	childTask.CreatedAt = childTask.CreatedAt.Add(2 * time.Minute)
	childTask.UpdatedAt = childTask.UpdatedAt.Add(2 * time.Minute)
	childTask.Scope = taskpkg.ScopeWorkspace
	childTask.WorkspaceID = workspaceB
	childTask.ParentTaskID = globalTask.ID
	childTask.Status = taskpkg.TaskStatusBlocked
	childTask.Priority = taskpkg.PriorityUrgent
	childTask.ApprovalPolicy = taskpkg.ApprovalPolicyManual
	childTask.ApprovalState = taskpkg.ApprovalStatePending
	childTask.Owner = ownershipForTest(taskpkg.OwnerKindPool, "backlog")

	for _, record := range []taskpkg.Task{globalTask, readyTask, childTask} {
		if err := globalDB.CreateTask(testutil.Context(t), record); err != nil {
			t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
		}
	}

	for _, tc := range []struct {
		name  string
		query taskpkg.Query
		want  []string
	}{
		{
			name:  "scope",
			query: taskpkg.Query{Scope: taskpkg.ScopeGlobal},
			want:  []string{globalTask.ID},
		},
		{
			name:  "workspace",
			query: taskpkg.Query{WorkspaceID: workspaceA},
			want:  []string{readyTask.ID},
		},
		{
			name:  "status",
			query: taskpkg.Query{Status: taskpkg.TaskStatusReady},
			want:  []string{readyTask.ID},
		},
		{
			name:  "priority",
			query: taskpkg.Query{Priority: taskpkg.PriorityUrgent},
			want:  []string{childTask.ID},
		},
		{
			name:  "approval state",
			query: taskpkg.Query{ApprovalState: taskpkg.ApprovalStatePending},
			want:  []string{childTask.ID},
		},
		{
			name:  "parent",
			query: taskpkg.Query{ParentTaskID: globalTask.ID},
			want:  []string{childTask.ID},
		},
		{
			name:  "owner kind",
			query: taskpkg.Query{OwnerKind: taskpkg.OwnerKindHuman},
			want:  []string{readyTask.ID},
		},
		{
			name:  "owner ref",
			query: taskpkg.Query{OwnerRef: "alice"},
			want:  []string{readyTask.ID},
		},
		{
			name:  "limit",
			query: taskpkg.Query{Limit: 2},
			want:  []string{childTask.ID, readyTask.ID},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			summaries, err := globalDB.ListTasks(testutil.Context(t), tc.query)
			if err != nil {
				t.Fatalf("ListTasks(%s) error = %v", tc.name, err)
			}
			gotIDs := taskSummaryIDs(summaries)
			if !testutil.EqualStringSlices(gotIDs, tc.want) {
				t.Fatalf("ListTasks(%s) ids = %#v, want %#v", tc.name, gotIDs, tc.want)
			}
		})
	}
}

func TestGlobalDBListTasksFiltersByCreatedBy(t *testing.T) {
	t.Parallel()

	t.Run("Should return only workspace scoped tasks matching created by filters", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "created-by-a", filepath.Join(t.TempDir(), "a"))
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "created-by-b", filepath.Join(t.TempDir(), "b"))

		agentA := workspaceTaskRecordForTest("task-created-by-agent-a", workspaceA)
		agentA.CreatedBy = taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-created-by"}
		agentB := workspaceTaskRecordForTest("task-created-by-agent-b", workspaceB)
		agentB.CreatedBy = taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-created-by"}
		humanA := workspaceTaskRecordForTest("task-created-by-human-a", workspaceA)
		humanA.CreatedBy = taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:created-by"}

		for _, record := range []taskpkg.Task{agentA, agentB, humanA} {
			if err := globalDB.CreateTask(ctx, record); err != nil {
				t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
			}
		}

		for _, tc := range []struct {
			name  string
			query taskpkg.Query
			want  []string
		}{
			{
				name: "Should filter by created by kind inside one workspace",
				query: taskpkg.Query{
					WorkspaceID:   workspaceA,
					CreatedByKind: taskpkg.ActorKindAgentSession,
				},
				want: []string{agentA.ID},
			},
			{
				name: "Should filter by created by ref inside one workspace",
				query: taskpkg.Query{
					WorkspaceID:  workspaceB,
					CreatedByRef: "sess-created-by",
				},
				want: []string{agentB.ID},
			},
			{
				name: "Should combine created by kind and ref",
				query: taskpkg.Query{
					WorkspaceID:   workspaceA,
					CreatedByKind: taskpkg.ActorKindHuman,
					CreatedByRef:  "user:created-by",
				},
				want: []string{humanA.ID},
			},
			{
				name: "Should return empty when no created by filter matches",
				query: taskpkg.Query{
					WorkspaceID:   workspaceA,
					CreatedByKind: taskpkg.ActorKindDaemon,
					CreatedByRef:  "daemon",
				},
				want: nil,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				summaries, err := globalDB.ListTasks(testutil.Context(t), tc.query)
				if err != nil {
					t.Fatalf("ListTasks(%#v) error = %v", tc.query, err)
				}
				gotIDs := taskSummaryIDs(summaries)
				if !testutil.EqualStringSlices(gotIDs, tc.want) {
					t.Fatalf("ListTasks(%#v) ids = %#v, want %#v", tc.query, gotIDs, tc.want)
				}
			})
		}
	})
}

func TestGlobalDBTaskBlocksCRUD(t *testing.T) {
	t.Parallel()

	t.Run("Should create clear and list task blocks with workspace isolation", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now }

		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "task-blocks-a", filepath.Join(t.TempDir(), "a"))
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "task-blocks-b", filepath.Join(t.TempDir(), "b"))
		taskRecord := workspaceTaskRecordForTest("task-blocks-crud", workspaceA)
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		openBlock := taskBlockRecordForTest("block-open", taskRecord.ID, taskpkg.BlockKindNeedsInput, now)
		openBlock.WorkspaceID = "caller-supplied-workspace-is-ignored"
		createdOpenResult, err := globalDB.CreateTaskBlock(ctx, taskpkg.CreateTaskBlockMutation{
			Actor:           coordinatorActorContextForTest(),
			Block:           openBlock,
			RecurrenceLimit: 2,
		})
		if err != nil {
			t.Fatalf("CreateTaskBlock(open) error = %v", err)
		}
		createdOpen := createdOpenResult.Block
		if got, want := createdOpen.WorkspaceID, workspaceA; got != want {
			t.Fatalf("created open workspace_id = %q, want %q", got, want)
		}
		if got, want := string(createdOpen.Details), string(openBlock.Details); got != want {
			t.Fatalf("created open details = %s, want %s", got, want)
		}

		blockToClear := taskBlockRecordForTest(
			"block-clear",
			taskRecord.ID,
			taskpkg.BlockKindTransient,
			now.Add(time.Minute),
		)
		createdClearResult, err := globalDB.CreateTaskBlock(ctx, taskpkg.CreateTaskBlockMutation{
			Actor:           coordinatorActorContextForTest(),
			Block:           blockToClear,
			RecurrenceLimit: 2,
		})
		if err != nil {
			t.Fatalf("CreateTaskBlock(clear) error = %v", err)
		}
		createdClear := createdClearResult.Block
		clearAt := now.Add(2 * time.Minute)
		cleared, err := globalDB.ClearTaskBlock(ctx, taskpkg.ClearTaskBlockMutation{
			TaskID:    taskRecord.ID,
			BlockID:   createdClear.ID,
			ClearedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:resolver"},
			ClearedAt: clearAt,
			ClearNote: "resolved by operator",
			Actor:     operatorActorContextForTest("user:resolver"),
		})
		if err != nil {
			t.Fatalf("ClearTaskBlock(first) error = %v", err)
		}
		if got, want := cleared.ClearedBy, (taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:resolver"}); got != want {
			t.Fatalf("cleared_by = %#v, want %#v", got, want)
		}
		if !cleared.ClearedAt.Equal(clearAt) {
			t.Fatalf("cleared_at = %v, want %v", cleared.ClearedAt, clearAt)
		}
		if got, want := cleared.ClearNote, "resolved by operator"; got != want {
			t.Fatalf("clear_note = %q, want %q", got, want)
		}

		if _, err := globalDB.ClearTaskBlock(ctx, taskpkg.ClearTaskBlockMutation{
			TaskID:    taskRecord.ID,
			BlockID:   createdClear.ID,
			ClearedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:resolver"},
			ClearedAt: clearAt.Add(time.Minute),
			Actor:     operatorActorContextForTest("user:resolver"),
		}); !errors.Is(err, taskpkg.ErrConflict) {
			t.Fatalf("ClearTaskBlock(second) error = %v, want %v", err, taskpkg.ErrConflict)
		}

		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO task_blocks (
			id, workspace_id, task_id, kind, reason, details_json, created_by_kind, created_by_ref,
			created_at, expires_at, cleared_at, cleared_by_kind, cleared_by_ref, clear_note
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, NULL, NULL, NULL)`,
			"block-wrong-workspace",
			workspaceB,
			taskRecord.ID,
			string(taskpkg.BlockKindNeedsInput),
			"wrong workspace row",
			nil,
			string(taskpkg.ActorKindHuman),
			"user:wrong-workspace",
			store.FormatTimestamp(now.Add(3*time.Minute)),
		); err != nil {
			t.Fatalf("seed wrong workspace task block error = %v", err)
		}

		openOnly, err := globalDB.ListTaskBlocks(ctx, taskRecord.ID, false)
		if err != nil {
			t.Fatalf("ListTaskBlocks(open-only) error = %v", err)
		}
		assertTaskBlockIDs(t, openOnly, []string{createdOpen.ID})

		withCleared, err := globalDB.ListTaskBlocks(ctx, taskRecord.ID, true)
		if err != nil {
			t.Fatalf("ListTaskBlocks(include cleared) error = %v", err)
		}
		assertTaskBlockIDs(t, withCleared, []string{createdOpen.ID, createdClear.ID})
	})
}

func TestGlobalDBTaskBlockRecurrences(t *testing.T) {
	t.Parallel()

	t.Run("Should upsert increment read and reset task block recurrence counters", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"task-block-recurrences",
			filepath.Join(t.TempDir(), "workspace"),
		)
		taskRecord := workspaceTaskRecordForTest("task-block-recurrences", workspaceID)
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		absent, err := globalDB.GetTaskBlockRecurrence(ctx, taskRecord.ID, taskpkg.BlockKindNeedsInput)
		if err != nil {
			t.Fatalf("GetTaskBlockRecurrence(absent) error = %v", err)
		}
		if absent.Count != 0 || absent.TaskID != taskRecord.ID || absent.Kind != taskpkg.BlockKindNeedsInput {
			t.Fatalf("absent recurrence = %#v, want zero counter for task/kind", absent)
		}

		firstAt := time.Date(2026, 7, 3, 11, 0, 0, 0, time.UTC)
		stored, err := globalDB.UpsertTaskBlockRecurrence(ctx, taskpkg.BlockRecurrence{
			TaskID:    taskRecord.ID,
			Kind:      taskpkg.BlockKindNeedsInput,
			Count:     2,
			UpdatedAt: firstAt,
		})
		if err != nil {
			t.Fatalf("UpsertTaskBlockRecurrence(first) error = %v", err)
		}
		assertTaskBlockRecurrence(t, stored, taskRecord.ID, taskpkg.BlockKindNeedsInput, 2, firstAt)

		secondAt := firstAt.Add(time.Minute)
		stored, err = globalDB.UpsertTaskBlockRecurrence(ctx, taskpkg.BlockRecurrence{
			TaskID:    taskRecord.ID,
			Kind:      taskpkg.BlockKindNeedsInput,
			Count:     5,
			UpdatedAt: secondAt,
		})
		if err != nil {
			t.Fatalf("UpsertTaskBlockRecurrence(second) error = %v", err)
		}
		assertTaskBlockRecurrence(t, stored, taskRecord.ID, taskpkg.BlockKindNeedsInput, 5, secondAt)

		incremented, err := globalDB.IncrementTaskBlockRecurrence(
			ctx,
			taskRecord.ID,
			taskpkg.BlockKindCapability,
			secondAt.Add(time.Minute),
		)
		if err != nil {
			t.Fatalf("IncrementTaskBlockRecurrence(first capability) error = %v", err)
		}
		assertTaskBlockRecurrence(
			t,
			incremented,
			taskRecord.ID,
			taskpkg.BlockKindCapability,
			1,
			secondAt.Add(time.Minute),
		)
		incremented, err = globalDB.IncrementTaskBlockRecurrence(
			ctx,
			taskRecord.ID,
			taskpkg.BlockKindCapability,
			secondAt.Add(2*time.Minute),
		)
		if err != nil {
			t.Fatalf("IncrementTaskBlockRecurrence(second capability) error = %v", err)
		}
		assertTaskBlockRecurrence(
			t,
			incremented,
			taskRecord.ID,
			taskpkg.BlockKindCapability,
			2,
			secondAt.Add(2*time.Minute),
		)

		if err := globalDB.ResetTaskBlockRecurrences(ctx, taskRecord.ID); err != nil {
			t.Fatalf("ResetTaskBlockRecurrences() error = %v", err)
		}
		for _, kind := range []taskpkg.BlockKind{taskpkg.BlockKindNeedsInput, taskpkg.BlockKindCapability} {
			reset, err := globalDB.GetTaskBlockRecurrence(ctx, taskRecord.ID, kind)
			if err != nil {
				t.Fatalf("GetTaskBlockRecurrence(%s after reset) error = %v", kind, err)
			}
			if reset.Count != 0 {
				t.Fatalf("recurrence count for %s after reset = %d, want 0", kind, reset.Count)
			}
		}
		if _, err := globalDB.GetTaskBlockRecurrence(
			ctx,
			"task-block-recurrences-missing",
			taskpkg.BlockKindNeedsInput,
		); !errors.Is(err, taskpkg.ErrTaskNotFound) {
			t.Fatalf("GetTaskBlockRecurrence(missing task) error = %v, want %v", err, taskpkg.ErrTaskNotFound)
		}
		if err := globalDB.ResetTaskBlockRecurrences(
			ctx,
			"task-block-recurrences-missing",
		); !errors.Is(err, taskpkg.ErrTaskNotFound) {
			t.Fatalf("ResetTaskBlockRecurrences(missing task) error = %v, want %v", err, taskpkg.ErrTaskNotFound)
		}
	})
}

func TestGlobalDBTaskNeedsAttentionAndWakeCreator(t *testing.T) {
	t.Parallel()

	t.Run("Should write clear attention metadata and preserve wake creator state", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"task-attention",
			filepath.Join(t.TempDir(), "workspace"),
		)
		taskRecord := workspaceTaskRecordForTest("task-attention", workspaceID)
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		markedAt := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
		marked, err := globalDB.MarkTaskNeedsAttention(ctx, taskpkg.NeedsAttentionMutation{
			Origin:   coordinatorActorContextForTest().Origin,
			TaskID:   taskRecord.ID,
			Reason:   "creator input required",
			Actor:    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "scheduler"},
			MarkedAt: markedAt,
		})
		if err != nil {
			t.Fatalf("MarkTaskNeedsAttention() error = %v", err)
		}
		if marked.NeedsAttention == nil {
			t.Fatal("NeedsAttention = nil, want escalation metadata")
		}
		if got, want := marked.NeedsAttention.Reason, "creator input required"; got != want {
			t.Fatalf("NeedsAttention.Reason = %q, want %q", got, want)
		}
		if !marked.NeedsAttention.At.Equal(markedAt) {
			t.Fatalf("NeedsAttention.At = %v, want %v", marked.NeedsAttention.At, markedAt)
		}
		if got, want := marked.NeedsAttention.By, (taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "scheduler"}); got != want {
			t.Fatalf("NeedsAttention.By = %#v, want %#v", got, want)
		}
		if !marked.WakeCreator {
			t.Fatal("WakeCreator = false after create, want default true")
		}

		wakeDisabledAt := markedAt.Add(time.Minute)
		wakeDisabled, err := globalDB.SetTaskWakeCreator(ctx, taskpkg.WakeCreatorMutation{
			TaskID:      taskRecord.ID,
			WakeCreator: false,
			UpdatedAt:   wakeDisabledAt,
		})
		if err != nil {
			t.Fatalf("SetTaskWakeCreator(false) error = %v", err)
		}
		if wakeDisabled.WakeCreator {
			t.Fatal("WakeCreator = true, want false")
		}
		if !wakeDisabled.UpdatedAt.Equal(wakeDisabledAt) {
			t.Fatalf("UpdatedAt after wake update = %v, want %v", wakeDisabled.UpdatedAt, wakeDisabledAt)
		}

		clearedAt := markedAt.Add(2 * time.Minute)
		cleared, err := globalDB.ClearTaskNeedsAttention(ctx, taskpkg.NeedsAttentionClearMutation{
			TaskID:    taskRecord.ID,
			Actor:     operatorActorContextForTest("operator"),
			ClearedAt: clearedAt,
		})
		if err != nil {
			t.Fatalf("ClearTaskNeedsAttention() error = %v", err)
		}
		if cleared.Task.NeedsAttention != nil {
			t.Fatalf("cleared NeedsAttention = %#v, want nil", cleared.Task.NeedsAttention)
		}
		if cleared.Task.WakeCreator {
			t.Fatal("WakeCreator = true after attention clear, want unchanged false")
		}
		if !cleared.Task.UpdatedAt.Equal(clearedAt) {
			t.Fatalf("UpdatedAt after attention clear = %v, want %v", cleared.Task.UpdatedAt, clearedAt)
		}
		if got, want := cleared.Event.Event.EventType, string(hookspkg.HookTaskRecovered); got != want {
			t.Fatalf("recovered event type = %q, want %q", got, want)
		}
		persistedEvent, err := globalDB.GetTaskEventRecord(ctx, cleared.Event.Event.ID)
		if err != nil {
			t.Fatalf("GetTaskEventRecord(recovered) error = %v", err)
		}
		if !reflect.DeepEqual(persistedEvent, cleared.Event) {
			t.Fatalf("persisted recovered event = %#v, want %#v", persistedEvent, cleared.Event)
		}
		_, err = globalDB.ClearTaskNeedsAttention(ctx, taskpkg.NeedsAttentionClearMutation{
			TaskID:    taskRecord.ID,
			Actor:     operatorActorContextForTest("operator"),
			ClearedAt: clearedAt.Add(time.Minute),
		})
		if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			t.Fatalf("ClearTaskNeedsAttention(second) error = %v, want ErrInvalidStatusTransition", err)
		}
	})
}

func TestGlobalDBBlockTaskAndReleaseRun(t *testing.T) {
	t.Parallel()

	t.Run("Should insert first block without recurrence and release run in one transaction", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 3, 13, 0, 0, 0, time.UTC)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"block-release-success",
			filepath.Join(t.TempDir(), "workspace"),
		)
		taskRecord := workspaceTaskRecordForTest("task-block-release-success", workspaceID)
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		rawToken := "agh_claim_block_release_success"
		leased := storeLeasedTaskRunForBlockTest(
			ctx,
			t,
			globalDB,
			taskRecord.ID,
			"run-block-release-success",
			"sess-block-release-success",
			rawToken,
			now.Add(10*time.Minute),
		)

		result, err := globalDB.BlockTaskAndReleaseRun(ctx, taskpkg.BlockTaskAndReleaseRunMutation{
			Actor: coordinatorActorContextForTest(),
			Block: taskpkg.TaskBlock{
				ID:      "block-release-success",
				TaskID:  taskRecord.ID,
				Kind:    taskpkg.BlockKindCapability,
				Reason:  "missing gpu capability",
				Details: json.RawMessage(`{"capability_id":"gpu"}`),
				CreatedBy: taskpkg.ActorIdentity{
					Kind: taskpkg.ActorKindAgentSession,
					Ref:  "sess-block-release-success",
				},
				CreatedAt: now,
			},
			RunID:      leased.ID,
			ClaimToken: rawToken,
			Now:        now,
		})
		if err != nil {
			t.Fatalf("BlockTaskAndReleaseRun() error = %v", err)
		}
		if got, want := result.ReleaseReason, "blocked"; got != want {
			t.Fatalf("ReleaseReason = %q, want %q", got, want)
		}
		if got, want := result.Block.WorkspaceID, workspaceID; got != want {
			t.Fatalf("Block.WorkspaceID = %q, want %q", got, want)
		}
		if got, want := result.Recurrence.Count, 0; got != want {
			t.Fatalf("Recurrence.Count = %d, want %d", got, want)
		}
		if got, want := result.Run.Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("Run.Status = %q, want %q", got, want)
		}
		if got, want := result.Run.Attempt, leased.Attempt; got != want {
			t.Fatalf("Run.Attempt = %d, want unchanged %d", got, want)
		}
		if result.Run.SessionID != "" || result.Run.ClaimedBy != nil ||
			result.Run.ClaimTokenHash != "" || !result.Run.LeaseUntil.IsZero() {
			t.Fatalf("released run retained lease fields: %#v", result.Run)
		}
		if got, want := result.PreviousRun.ID, leased.ID; got != want {
			t.Fatalf("PreviousRun.ID = %q, want %q", got, want)
		}
		if got, want := result.ClaimTokenHash, leased.ClaimTokenHash; got != want {
			t.Fatalf("ClaimTokenHash = %q, want %q", got, want)
		}

		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if storedTask.CurrentRunID != "" {
			t.Fatalf("CurrentRunID = %q, want cleared", storedTask.CurrentRunID)
		}
		blocks, err := globalDB.ListTaskBlocks(ctx, taskRecord.ID, false)
		if err != nil {
			t.Fatalf("ListTaskBlocks() error = %v", err)
		}
		assertTaskBlockIDs(t, blocks, []string{result.Block.ID})
	})

	t.Run("Should block and release global task runs", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 3, 14, 0, 0, 0, time.UTC)
		taskRecord := taskRecordForTest("task-global-block-release")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		rawToken := "agh_claim_global_block_release"
		leased := storeLeasedTaskRunForBlockTest(
			ctx,
			t,
			globalDB,
			taskRecord.ID,
			"run-global-block-release",
			"sess-global-block-release",
			rawToken,
			now.Add(10*time.Minute),
		)

		result, err := globalDB.BlockTaskAndReleaseRun(ctx, taskpkg.BlockTaskAndReleaseRunMutation{
			Actor: coordinatorActorContextForTest(),
			Block: taskpkg.TaskBlock{
				ID:     "block-global-release",
				TaskID: taskRecord.ID,
				Kind:   taskpkg.BlockKindNeedsInput,
				Reason: "global task needs input",
				CreatedBy: taskpkg.ActorIdentity{
					Kind: taskpkg.ActorKindAgentSession,
					Ref:  "sess-global-block-release",
				},
				CreatedAt: now,
			},
			RunID:      leased.ID,
			ClaimToken: rawToken,
			Now:        now,
		})
		if err != nil {
			t.Fatalf("BlockTaskAndReleaseRun(global) error = %v", err)
		}
		if got, want := result.Block.WorkspaceID, ""; got != want {
			t.Fatalf("Block.WorkspaceID = %q, want global empty workspace", got)
		}
		if got, want := result.Run.Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("Run.Status = %q, want %q", got, want)
		}
		if got, want := result.Run.Attempt, leased.Attempt; got != want {
			t.Fatalf("Run.Attempt = %d, want unchanged %d", got, want)
		}
		open, err := globalDB.HasOpenTaskBlocks(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("HasOpenTaskBlocks(open global) error = %v", err)
		}
		if !open {
			t.Fatal("HasOpenTaskBlocks(open global) = false, want true")
		}
		blocks, err := globalDB.ListTaskBlocks(ctx, taskRecord.ID, false)
		if err != nil {
			t.Fatalf("ListTaskBlocks(global open) error = %v", err)
		}
		assertTaskBlockIDs(t, blocks, []string{result.Block.ID})

		cleared, err := globalDB.ClearTaskBlock(ctx, taskpkg.ClearTaskBlockMutation{
			TaskID:    taskRecord.ID,
			BlockID:   result.Block.ID,
			ClearedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:operator"},
			ClearedAt: now.Add(time.Minute),
			Actor:     operatorActorContextForTest("user:operator"),
		})
		if err != nil {
			t.Fatalf("ClearTaskBlock(global) error = %v", err)
		}
		if cleared.WorkspaceID != "" || cleared.ClearedAt.IsZero() {
			t.Fatalf("cleared global block = %#v, want empty workspace and clear stamp", cleared)
		}
		open, err = globalDB.HasOpenTaskBlocks(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("HasOpenTaskBlocks(cleared global) error = %v", err)
		}
		if open {
			t.Fatal("HasOpenTaskBlocks(cleared global) = true, want false")
		}
	})

	t.Run("Should serialize expired lease recovery against block release", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 3, 15, 0, 0, 0, time.UTC)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"block-release-recovery-race",
			filepath.Join(t.TempDir(), "workspace"),
		)

		for attempt := range 20 {
			taskID := fmt.Sprintf("task-block-release-recovery-race-%02d", attempt)
			runID := fmt.Sprintf("run-block-release-recovery-race-%02d", attempt)
			blockID := fmt.Sprintf("block-release-recovery-race-%02d", attempt)
			sessionID := fmt.Sprintf("sess-block-release-recovery-race-%02d", attempt)
			rawToken := fmt.Sprintf("agh_claim_block_release_recovery_race_%02d", attempt)

			taskRecord := workspaceTaskRecordForTest(taskID, workspaceID)
			taskRecord.Status = taskpkg.TaskStatusReady
			if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
				t.Fatalf("CreateTask(%q) error = %v", taskID, err)
			}
			leased := storeLeasedTaskRunForBlockTest(
				ctx,
				t,
				globalDB,
				taskRecord.ID,
				runID,
				sessionID,
				rawToken,
				now.Add(time.Minute),
			)

			start := make(chan struct{})
			var wg sync.WaitGroup
			var blockResult taskpkg.BlockTaskAndReleaseRunResult
			var blockErr error
			var recovered []taskpkg.ExpiredLeaseRecoveryResult
			var recoverErr error
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				blockResult, blockErr = globalDB.BlockTaskAndReleaseRun(ctx, taskpkg.BlockTaskAndReleaseRunMutation{
					Actor: coordinatorActorContextForTest(),
					Block: taskpkg.TaskBlock{
						ID:      blockID,
						TaskID:  taskRecord.ID,
						Kind:    taskpkg.BlockKindNeedsInput,
						Reason:  "creator input required during recovery race",
						Details: json.RawMessage(`{"race":"recovery"}`),
						CreatedBy: taskpkg.ActorIdentity{
							Kind: taskpkg.ActorKindAgentSession,
							Ref:  sessionID,
						},
						CreatedAt: now,
					},
					RunID:      leased.ID,
					ClaimToken: rawToken,
					Now:        now,
				})
			}()
			go func() {
				defer wg.Done()
				<-start
				recovered, recoverErr = globalDB.RecoverExpiredRunLeases(ctx, taskpkg.ExpiredLeaseRecovery{
					Now:    now.Add(2 * time.Minute),
					Reason: string(taskpkg.AutonomyLeaseExpired),
					Limit:  10,
				})
			}()
			close(start)
			wg.Wait()

			if recoverErr != nil {
				t.Fatalf("RecoverExpiredRunLeases(%d) error = %v", attempt, recoverErr)
			}
			if blockErr != nil &&
				!errors.Is(blockErr, taskpkg.ErrInvalidClaimToken) &&
				!errors.Is(blockErr, taskpkg.ErrInvalidStatusTransition) &&
				!errors.Is(blockErr, taskpkg.ErrLeaseExpired) {
				t.Fatalf("BlockTaskAndReleaseRun(%d) error = %v, want nil or lease race conflict", attempt, blockErr)
			}
			blockSucceeded := blockErr == nil
			recoverSucceeded := len(recovered) > 0
			if blockSucceeded && recoverSucceeded {
				t.Fatalf(
					"attempt %d double-released run: block=%#v recovered=%#v",
					attempt,
					blockResult.Run,
					recovered,
				)
			}
			if !blockSucceeded && !recoverSucceeded {
				t.Fatalf("attempt %d neither block-release nor recovery won: blockErr=%v", attempt, blockErr)
			}

			blocks, err := globalDB.ListTaskBlocks(ctx, taskRecord.ID, true)
			if err != nil {
				t.Fatalf("ListTaskBlocks(%d) error = %v", attempt, err)
			}
			switch {
			case blockSucceeded:
				assertTaskBlockIDs(t, blocks, []string{blockResult.Block.ID})
			case len(blocks) != 0:
				t.Fatalf("attempt %d blocks = %#v, want none when recovery won", attempt, blocks)
			}
			storedRun, err := globalDB.GetTaskRun(ctx, leased.ID)
			if err != nil {
				t.Fatalf("GetTaskRun(%d) error = %v", attempt, err)
			}
			if storedRun.Status != taskpkg.TaskRunStatusQueued ||
				storedRun.SessionID != "" ||
				storedRun.ClaimTokenHash != "" ||
				!storedRun.LeaseUntil.IsZero() {
				t.Fatalf("attempt %d stored run = %#v, want queued with cleared lease", attempt, storedRun)
			}
		}
	})

	t.Run("Should reject claim token mismatch without changing run block or recurrence rows", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 3, 14, 0, 0, 0, time.UTC)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"block-release-token-mismatch",
			filepath.Join(t.TempDir(), "workspace"),
		)
		taskRecord := workspaceTaskRecordForTest("task-block-release-token-mismatch", workspaceID)
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		leased := storeLeasedTaskRunForBlockTest(
			ctx,
			t,
			globalDB,
			taskRecord.ID,
			"run-block-release-token-mismatch",
			"sess-block-release-token-mismatch",
			"agh_claim_block_release_token_mismatch",
			now.Add(10*time.Minute),
		)

		_, err := globalDB.BlockTaskAndReleaseRun(ctx, taskpkg.BlockTaskAndReleaseRunMutation{
			Actor: coordinatorActorContextForTest(),
			Block: taskpkg.TaskBlock{
				ID:     "block-token-mismatch",
				TaskID: taskRecord.ID,
				Kind:   taskpkg.BlockKindNeedsInput,
				Reason: "needs creator input",
				CreatedBy: taskpkg.ActorIdentity{
					Kind: taskpkg.ActorKindAgentSession,
					Ref:  "sess-block-release-token-mismatch",
				},
				CreatedAt: now,
			},
			RunID:      leased.ID,
			ClaimToken: "agh_claim_wrong_token",
			Now:        now,
		})
		if !errors.Is(err, taskpkg.ErrInvalidClaimToken) {
			t.Fatalf("BlockTaskAndReleaseRun(wrong token) error = %v, want %v", err, taskpkg.ErrInvalidClaimToken)
		}

		blocks, listErr := globalDB.ListTaskBlocks(ctx, taskRecord.ID, true)
		if listErr != nil {
			t.Fatalf("ListTaskBlocks(after rejection) error = %v", listErr)
		}
		if len(blocks) != 0 {
			t.Fatalf("ListTaskBlocks(after rejection) = %#v, want empty", blocks)
		}
		recurrence, recurrenceErr := globalDB.GetTaskBlockRecurrence(ctx, taskRecord.ID, taskpkg.BlockKindNeedsInput)
		if recurrenceErr != nil {
			t.Fatalf("GetTaskBlockRecurrence(after rejection) error = %v", recurrenceErr)
		}
		if recurrence.Count != 0 {
			t.Fatalf("recurrence count after rejection = %d, want 0", recurrence.Count)
		}
		storedRun, getRunErr := globalDB.GetTaskRun(ctx, leased.ID)
		if getRunErr != nil {
			t.Fatalf("GetTaskRun(after rejection) error = %v", getRunErr)
		}
		if got, want := storedRun.Status, taskpkg.TaskRunStatusClaimed; got != want {
			t.Fatalf("stored run status = %q, want %q", got, want)
		}
		if got, want := storedRun.SessionID, leased.SessionID; got != want {
			t.Fatalf("stored run session = %q, want %q", got, want)
		}
		if got, want := storedRun.ClaimTokenHash, leased.ClaimTokenHash; got != want {
			t.Fatalf("stored run claim hash = %q, want %q", got, want)
		}
		storedTask, getTaskErr := globalDB.GetTask(ctx, taskRecord.ID)
		if getTaskErr != nil {
			t.Fatalf("GetTask(after rejection) error = %v", getTaskErr)
		}
		if got, want := storedTask.CurrentRunID, leased.ID; got != want {
			t.Fatalf("CurrentRunID after rejection = %q, want %q", got, want)
		}
	})
}

func TestGlobalDBListTasksSearchAndActivityOrdering(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	alpha := taskRecordForTest("task-search-alpha")
	alpha.Title = "Alpha planning"
	alpha.Identifier = "OPS-100"
	alpha.UpdatedAt = alpha.UpdatedAt.Add(time.Minute)

	beta := taskRecordForTest("task-search-beta")
	beta.Title = "Beta rollout"
	beta.Identifier = "OPS-200"
	beta.CreatedAt = beta.CreatedAt.Add(2 * time.Minute)
	beta.UpdatedAt = beta.UpdatedAt.Add(2 * time.Minute)

	for _, record := range []taskpkg.Task{alpha, beta} {
		if err := globalDB.CreateTask(testutil.Context(t), record); err != nil {
			t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
		}
	}
	if err := globalDB.CreateTaskRun(testutil.Context(t), taskpkg.Run{
		ID:        "run-search-beta",
		TaskID:    beta.ID,
		Status:    taskpkg.TaskRunStatusRunning,
		Attempt:   1,
		Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler"},
		QueuedAt:  time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		StartedAt: time.Date(2026, 4, 17, 12, 5, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}

	byTitle, err := globalDB.ListTasks(testutil.Context(t), taskpkg.Query{Search: "alpha"})
	if err != nil {
		t.Fatalf("ListTasks(search title) error = %v", err)
	}
	if got, want := orderedTaskSummaryIDs(byTitle), []string{alpha.ID}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("ListTasks(search title) ids = %#v, want %#v", got, want)
	}

	byIdentifier, err := globalDB.ListTasks(testutil.Context(t), taskpkg.Query{Search: "ops-200"})
	if err != nil {
		t.Fatalf("ListTasks(search identifier) error = %v", err)
	}
	if got, want := orderedTaskSummaryIDs(byIdentifier), []string{beta.ID}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("ListTasks(search identifier) ids = %#v, want %#v", got, want)
	}

	all, err := globalDB.ListTasks(testutil.Context(t), taskpkg.Query{})
	if err != nil {
		t.Fatalf("ListTasks(all) error = %v", err)
	}
	if got, want := orderedTaskSummaryIDs(all), []string{beta.ID, alpha.ID}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("ListTasks(all) order = %#v, want %#v", got, want)
	}
}

func TestGlobalDBTaskCatalogPaginationAndFacets(t *testing.T) {
	t.Parallel()

	t.Run("Should filter and count the complete catalog before the page cut", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		base := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		for index := range 125 {
			record := taskRecordForTest(fmt.Sprintf("task-catalog-%03d", index))
			record.UpdatedAt = base.Add(time.Duration(index) * time.Second)
			record.Priority = taskpkg.PriorityLow
			if index%2 == 0 {
				record.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "user:alice"}
			} else {
				record.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "user:bob"}
			}
			if index == 3 {
				record.Status = taskpkg.TaskStatusDraft
			}
			if index == 7 {
				record.Status = taskpkg.TaskStatusBlocked
				record.Priority = taskpkg.PriorityUrgent
			}
			if index == 121 {
				record.Title = "Needle beyond the former prefix"
				record.Priority = taskpkg.PriorityHigh
			}
			if err := globalDB.CreateTask(ctx, record); err != nil {
				t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
			}
		}

		page, err := globalDB.ListTaskCatalog(ctx, taskpkg.CatalogQuery{
			Scope:         taskpkg.CatalogScopeGlobal,
			Search:        "needle",
			IncludeDrafts: true,
			Limit:         5,
		})
		if err != nil {
			t.Fatalf("ListTaskCatalog(search) error = %v", err)
		}
		if got, want := page.Total, 1; got != want {
			t.Fatalf("ListTaskCatalog(search).Total = %d, want %d", got, want)
		}
		if got, want := orderedTaskSummaryIDs(page.Tasks), []string{"task-catalog-121"}; !testutil.EqualStringSlices(
			got,
			want,
		) {
			t.Fatalf("ListTaskCatalog(search) ids = %#v, want %#v", got, want)
		}
		if got, want := page.StatusFacets, []taskpkg.CatalogStatusFacet{{
			Status: taskpkg.TaskStatusReady,
			Count:  1,
		}}; !equalTaskCatalogStatusFacets(got, want) {
			t.Fatalf("ListTaskCatalog(search).StatusFacets = %#v, want %#v", got, want)
		}

		counting := &countingTaskCatalogExecutor{taskSQLExecutor: globalDB.db}
		normalized, err := taskpkg.NormalizeCatalogQuery(taskpkg.CatalogQuery{
			Scope:         taskpkg.CatalogScopeGlobal,
			IncludeDrafts: true,
			Limit:         10,
		})
		if err != nil {
			t.Fatalf("NormalizeCatalogQuery() error = %v", err)
		}
		countedPage, err := listTaskCatalogWithExecutor(ctx, counting, normalized)
		if err != nil {
			t.Fatalf("listTaskCatalogWithExecutor() error = %v", err)
		}
		if got, want := counting.reads, 2; got != want {
			t.Fatalf("task catalog read statements = %d, want constant %d", got, want)
		}
		if got, want := countedPage.Total, 125; got != want {
			t.Fatalf("listTaskCatalogWithExecutor().Total = %d, want %d", got, want)
		}
	})

	t.Run("Should page without gaps after the captured anchor changes", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		base := time.Date(2026, 7, 10, 13, 0, 0, 0, time.UTC)
		for index := range 17 {
			record := taskRecordForTest(fmt.Sprintf("task-cursor-%02d", index))
			record.UpdatedAt = base.Add(time.Duration(index) * time.Second)
			record.Priority = []taskpkg.Priority{
				taskpkg.PriorityLow,
				taskpkg.PriorityMedium,
				taskpkg.PriorityHigh,
				taskpkg.PriorityUrgent,
			}[index%4]
			if err := globalDB.CreateTask(ctx, record); err != nil {
				t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
			}
		}

		query := taskpkg.CatalogQuery{
			Scope:         taskpkg.CatalogScopeGlobal,
			IncludeDrafts: true,
			Sort:          taskpkg.CatalogSortPriority,
			Limit:         4,
		}
		first, err := globalDB.ListTaskCatalog(ctx, query)
		if err != nil {
			t.Fatalf("ListTaskCatalog(first) error = %v", err)
		}
		if !first.HasMore || first.NextCursor == "" {
			t.Fatalf("ListTaskCatalog(first) page = %#v, want continuation", first)
		}
		anchorID := first.Tasks[len(first.Tasks)-1].ID
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE tasks SET updated_at = ? WHERE id = ?`,
			store.FormatTimestamp(base.Add(24*time.Hour)),
			anchorID,
		); err != nil {
			t.Fatalf("move catalog anchor error = %v", err)
		}

		seen := make(map[string]struct{}, 17)
		for _, item := range first.Tasks {
			seen[item.ID] = struct{}{}
		}
		query.Cursor = first.NextCursor
		for query.Cursor != "" {
			page, pageErr := globalDB.ListTaskCatalog(ctx, query)
			if pageErr != nil {
				t.Fatalf("ListTaskCatalog(next) error = %v", pageErr)
			}
			for _, item := range page.Tasks {
				if _, duplicate := seen[item.ID]; duplicate {
					t.Fatalf("ListTaskCatalog() duplicated task %q", item.ID)
				}
				seen[item.ID] = struct{}{}
			}
			query.Cursor = page.NextCursor
		}
		if got, want := len(seen), 17; got != want {
			t.Fatalf("unique paged tasks = %d, want %d", got, want)
		}

		mismatch := query
		mismatch.Cursor = first.NextCursor
		mismatch.Search = "different"
		if _, err := globalDB.ListTaskCatalog(ctx, mismatch); !errors.Is(err, taskpkg.ErrCatalogCursorInvalid) {
			t.Fatalf("ListTaskCatalog(mismatched cursor) error = %v, want %v", err, taskpkg.ErrCatalogCursorInvalid)
		}
	})

	t.Run("Should derive canonical status before filters and self-filtered facets", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		base := time.Date(2026, 7, 10, 14, 0, 0, 0, time.UTC)

		running := taskRecordForTest("task-canonical-running")
		running.Status = taskpkg.TaskStatusReady
		running.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "user:alice"}
		running.UpdatedAt = base
		if err := globalDB.CreateTask(ctx, running); err != nil {
			t.Fatalf("CreateTask(running) error = %v", err)
		}
		run := taskRunForTest("run-canonical-running", running.ID)
		run.QueuedAt = base.Add(time.Minute)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun(running) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE task_runs
			 SET status = 'running', claimed_by_kind = 'daemon', claimed_by_ref = 'scheduler',
			     session_id = 'sess-canonical-running', claimed_at = ?, started_at = ?
			 WHERE id = ?`,
			store.FormatTimestamp(base.Add(2*time.Minute)),
			store.FormatTimestamp(base.Add(3*time.Minute)),
			run.ID,
		); err != nil {
			t.Fatalf("promote canonical running run error = %v", err)
		}

		paused := taskRecordForTest("task-canonical-paused")
		paused.Status = taskpkg.TaskStatusReady
		paused.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "user:alice"}
		paused.Paused = true
		paused.PausedBy = "user:alice"
		paused.PausedAt = base.Add(4 * time.Minute)
		paused.PausedReason = "operator pause"
		paused.UpdatedAt = base.Add(4 * time.Minute)
		if err := globalDB.CreateTask(ctx, paused); err != nil {
			t.Fatalf("CreateTask(paused) error = %v", err)
		}

		dependency := taskRecordForTest("task-canonical-dependency")
		dependency.Status = taskpkg.TaskStatusPending
		dependency.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "user:bob"}
		dependency.UpdatedAt = base.Add(5 * time.Minute)
		if err := globalDB.CreateTask(ctx, dependency); err != nil {
			t.Fatalf("CreateTask(dependency) error = %v", err)
		}
		dependent := taskRecordForTest("task-canonical-dependent")
		dependent.Status = taskpkg.TaskStatusReady
		dependent.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "user:bob"}
		dependent.UpdatedAt = base.Add(6 * time.Minute)
		if err := globalDB.CreateTask(ctx, dependent); err != nil {
			t.Fatalf("CreateTask(dependent) error = %v", err)
		}
		if err := globalDB.CreateDependency(ctx, taskpkg.Dependency{
			TaskID:          dependent.ID,
			DependsOnTaskID: dependency.ID,
			Kind:            taskpkg.DependencyKindBlocks,
			CreatedAt:       base.Add(7 * time.Minute),
		}); err != nil {
			t.Fatalf("CreateDependency() error = %v", err)
		}
		completedDependency := taskRecordForTest("task-canonical-completed-dependency")
		completedDependency.Status = taskpkg.TaskStatusReady
		completedDependency.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "user:bob"}
		completedDependency.UpdatedAt = base.Add(8 * time.Minute)
		if err := globalDB.CreateTask(ctx, completedDependency); err != nil {
			t.Fatalf("CreateTask(completed dependency) error = %v", err)
		}
		completedRun := taskRunForTest("run-canonical-completed-dependency", completedDependency.ID)
		completedRun.Status = taskpkg.TaskRunStatusCompleted
		completedRun.StartedAt = base.Add(9 * time.Minute)
		completedRun.EndedAt = base.Add(10 * time.Minute)
		if err := globalDB.CreateTaskRun(ctx, completedRun); err != nil {
			t.Fatalf("CreateTaskRun(completed dependency) error = %v", err)
		}
		released := taskRecordForTest("task-canonical-released-dependent")
		released.Title = "Released by canonical dependency completion"
		released.Status = taskpkg.TaskStatusReady
		released.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "user:bob"}
		released.UpdatedAt = base.Add(11 * time.Minute)
		if err := globalDB.CreateTask(ctx, released); err != nil {
			t.Fatalf("CreateTask(released dependent) error = %v", err)
		}
		if err := globalDB.CreateDependency(ctx, taskpkg.Dependency{
			TaskID:          released.ID,
			DependsOnTaskID: completedDependency.ID,
			Kind:            taskpkg.DependencyKindBlocks,
			CreatedAt:       base.Add(12 * time.Minute),
		}); err != nil {
			t.Fatalf("CreateDependency(released) error = %v", err)
		}

		inProgress, err := globalDB.ListTaskCatalog(ctx, taskpkg.CatalogQuery{
			Scope:         taskpkg.CatalogScopeGlobal,
			Status:        taskpkg.TaskStatusInProgress,
			IncludeDrafts: true,
		})
		if err != nil {
			t.Fatalf("ListTaskCatalog(in_progress) error = %v", err)
		}
		if got, want := orderedTaskSummaryIDs(inProgress.Tasks), []string{running.ID}; !testutil.EqualStringSlices(
			got,
			want,
		) {
			t.Fatalf("in-progress ids = %#v, want %#v", got, want)
		}
		if got, want := inProgress.StatusFacets, []taskpkg.CatalogStatusFacet{{
			Status: taskpkg.TaskStatusInProgress,
			Count:  1,
		}}; !equalTaskCatalogStatusFacets(got, want) {
			t.Fatalf("in-progress status facets = %#v, want %#v", got, want)
		}

		blocked, err := globalDB.ListTaskCatalog(ctx, taskpkg.CatalogQuery{
			Scope:         taskpkg.CatalogScopeGlobal,
			Status:        taskpkg.TaskStatusBlocked,
			IncludeDrafts: true,
		})
		if err != nil {
			t.Fatalf("ListTaskCatalog(blocked) error = %v", err)
		}
		if got, want := blocked.Total, 2; got != want {
			t.Fatalf("blocked total = %d, want %d", got, want)
		}
		if got, want := blocked.StatusFacets, []taskpkg.CatalogStatusFacet{{
			Status: taskpkg.TaskStatusBlocked,
			Count:  2,
		}}; !equalTaskCatalogStatusFacets(got, want) {
			t.Fatalf("blocked status facets = %#v, want %#v", got, want)
		}

		alice, err := globalDB.ListTaskCatalog(ctx, taskpkg.CatalogQuery{
			Scope:         taskpkg.CatalogScopeGlobal,
			OwnerKind:     taskpkg.OwnerKindHuman,
			OwnerRef:      "user:alice",
			IncludeDrafts: true,
		})
		if err != nil {
			t.Fatalf("ListTaskCatalog(owner) error = %v", err)
		}
		if got, want := alice.Total, 2; got != want {
			t.Fatalf("alice total = %d, want %d", got, want)
		}
		if got, want := alice.OwnerFacets, []taskpkg.CatalogOwnerFacet{{
			Owner: taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "user:alice"},
			Count: 2,
		}}; len(got) != len(want) || got[0] != want[0] {
			t.Fatalf("alice owner facets = %#v, want %#v", got, want)
		}

		releasedPage, err := globalDB.ListTaskCatalog(ctx, taskpkg.CatalogQuery{
			Scope:         taskpkg.CatalogScopeGlobal,
			Search:        "released by canonical dependency completion",
			IncludeDrafts: true,
		})
		if err != nil {
			t.Fatalf("ListTaskCatalog(released dependency) error = %v", err)
		}
		if got, want := releasedPage.Total, 1; got != want {
			t.Fatalf("released dependent total = %d, want %d", got, want)
		}
		if got, want := releasedPage.Tasks[0].Status, taskpkg.TaskStatusReady; got != want {
			t.Fatalf("released dependent status = %q, want %q", got, want)
		}
		manager, err := taskpkg.NewManager(taskpkg.WithStore(globalDB))
		if err != nil {
			t.Fatalf("task.NewManager() error = %v", err)
		}
		actor, err := taskpkg.DeriveHumanActorContext("user:alice", taskpkg.OriginKindCLI, "catalog parity")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}
		enriched, err := manager.ListTasks(ctx, taskpkg.Query{Search: released.Title}, actor)
		if err != nil {
			t.Fatalf("ListTasks(enriched parity) error = %v", err)
		}
		if len(enriched) != 1 || enriched[0].Status != releasedPage.Tasks[0].Status {
			t.Fatalf(
				"catalog/enriched parity = %#v / %#v, want matching canonical status",
				releasedPage.Tasks,
				enriched,
			)
		}
	})

	t.Run("Should seek scoped history rows through task indexes", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "catalog-plan", t.TempDir())
		normalized, err := taskpkg.NormalizeCatalogQuery(taskpkg.CatalogQuery{
			Scope:         taskpkg.CatalogScopeWorkspace,
			WorkspaceID:   workspaceID,
			IncludeDrafts: true,
			Limit:         10,
		})
		if err != nil {
			t.Fatalf("NormalizeCatalogQuery() error = %v", err)
		}
		baseWhere, baseArgs := taskCatalogBaseFilter(normalized)
		where, filterArgs := taskCatalogFilter(normalized)
		statement := taskCatalogStatement(
			"SELECT "+taskCatalogSelectColumns+" FROM catalog",
			baseWhere,
			where,
		) + taskCatalogOrderBy(normalized.Sort) + taskCatalogLimitClause(normalized.Limit+1)
		args := append(append([]any(nil), baseArgs...), filterArgs...)
		rows, err := globalDB.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+statement, args...)
		if err != nil {
			t.Fatalf("EXPLAIN QUERY PLAN task catalog error = %v", err)
		}
		defer func() {
			if closeErr := rows.Close(); closeErr != nil {
				t.Fatalf("close task catalog query plan rows error = %v", closeErr)
			}
		}()

		details := make([]string, 0)
		for rows.Next() {
			var id int
			var parent int
			var unused int
			var detail string
			if scanErr := rows.Scan(&id, &parent, &unused, &detail); scanErr != nil {
				t.Fatalf("scan task catalog query plan error = %v", scanErr)
			}
			details = append(details, detail)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate task catalog query plan error = %v", err)
		}
		plan := strings.Join(details, "\n")
		for _, indexedSeek := range []string{
			"SEARCH tr USING INDEX idx_task_runs_task",
			"SEARCH te USING INDEX idx_task_events_task",
			"idx_task_dependencies_task",
		} {
			if !strings.Contains(plan, indexedSeek) {
				t.Fatalf("task catalog query plan missing %q:\n%s", indexedSeek, plan)
			}
		}
		for _, globalScan := range []string{"SCAN task_runs", "SCAN task_events", "SCAN task_dependencies"} {
			if strings.Contains(plan, globalScan) {
				t.Fatalf("task catalog query plan contains global history scan %q:\n%s", globalScan, plan)
			}
		}
	})

	t.Run("Should treat percent and underscore as literal search text", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		for _, record := range []taskpkg.Task{
			func() taskpkg.Task {
				record := taskRecordForTest("task-search-literal")
				record.Title = "Investigate literal %_ marker"
				record.Status = taskpkg.TaskStatusReady
				record.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "user:alice"}
				return record
			}(),
			func() taskpkg.Task {
				record := taskRecordForTest("task-search-wildcard-decoy")
				record.Title = "Investigate ordinary marker"
				record.Status = taskpkg.TaskStatusReady
				record.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "user:alice"}
				return record
			}(),
		} {
			if err := globalDB.CreateTask(ctx, record); err != nil {
				t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
			}
		}
		catalog, err := globalDB.ListTaskCatalog(ctx, taskpkg.CatalogQuery{
			Scope:         taskpkg.CatalogScopeGlobal,
			Search:        "%_",
			IncludeDrafts: true,
		})
		if err != nil {
			t.Fatalf("ListTaskCatalog(literal search) error = %v", err)
		}
		got := orderedTaskSummaryIDs(catalog.Tasks)
		want := []string{"task-search-literal"}
		if !testutil.EqualStringSlices(got, want) {
			t.Fatalf("literal catalog search ids = %#v, want %#v", got, want)
		}
		inbox, err := globalDB.ListTaskInbox(ctx, taskpkg.InboxQuery{
			Scope:  taskpkg.CatalogScopeGlobal,
			Search: "%_",
		}, taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:alice"})
		if err != nil {
			t.Fatalf("ListTaskInbox(literal search) error = %v", err)
		}
		if got, want := inbox.Total, 1; got != want {
			t.Fatalf("literal inbox search total = %d, want %d", got, want)
		}
	})

	t.Run("Should bind cursors to the resolved workspace boundary", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "catalog-a", t.TempDir())
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "catalog-b", t.TempDir())
		for index, workspaceID := range []string{workspaceA, workspaceA, workspaceB, workspaceB} {
			record := workspaceTaskRecordForTest(fmt.Sprintf("task-workspace-%d", index), workspaceID)
			record.UpdatedAt = record.UpdatedAt.Add(time.Duration(index) * time.Minute)
			if err := globalDB.CreateTask(ctx, record); err != nil {
				t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
			}
		}

		first, err := globalDB.ListTaskCatalog(ctx, taskpkg.CatalogQuery{
			Scope:         taskpkg.CatalogScopeWorkspace,
			WorkspaceID:   workspaceA,
			IncludeDrafts: true,
			Limit:         1,
		})
		if err != nil {
			t.Fatalf("ListTaskCatalog(workspace A) error = %v", err)
		}
		if first.NextCursor == "" {
			t.Fatal("ListTaskCatalog(workspace A).NextCursor = empty, want continuation")
		}
		_, err = globalDB.ListTaskCatalog(ctx, taskpkg.CatalogQuery{
			Scope:         taskpkg.CatalogScopeWorkspace,
			WorkspaceID:   workspaceB,
			IncludeDrafts: true,
			Cursor:        first.NextCursor,
			Limit:         1,
		})
		if !errors.Is(err, taskpkg.ErrCatalogCursorInvalid) {
			t.Fatalf(
				"ListTaskCatalog(cross-workspace cursor) error = %v, want %v",
				err,
				taskpkg.ErrCatalogCursorInvalid,
			)
		}
	})
}

func TestGlobalDBTaskInboxUsesTwoStatementPaging(t *testing.T) {
	t.Parallel()

	t.Run("Should compute exact metadata and one page with two reads", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		record := taskRecordForTest("task-inbox-two-reads")
		record.Status = taskpkg.TaskStatusReady
		record.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "user:alice"}
		if err := globalDB.CreateTask(ctx, record); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		actor := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:alice"}
		query, normalizedActor, err := taskpkg.NormalizeInboxQuery(taskpkg.InboxQuery{
			Scope: taskpkg.CatalogScopeGlobal,
			Limit: 10,
		}, actor)
		if err != nil {
			t.Fatalf("NormalizeInboxQuery() error = %v", err)
		}
		counting := &countingTaskCatalogExecutor{taskSQLExecutor: globalDB.db}
		page, err := listTaskInboxWithExecutor(ctx, counting, query, normalizedActor)
		if err != nil {
			t.Fatalf("listTaskInboxWithExecutor() error = %v", err)
		}
		if got, want := counting.reads, 2; got != want {
			t.Fatalf("task inbox read statements = %d, want constant %d", got, want)
		}
		if got, want := page.Total, 1; got != want {
			t.Fatalf("task inbox total = %d, want %d", got, want)
		}
	})
}

func TestGlobalDBTaskRunRoundTripAndFilters(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a Live snapshot bound to a different owner workspace", func(t *testing.T) {
		t.Parallel()

		liveSpec := participation.Spec{
			Version:         participation.SpecVersion,
			Mode:            participation.ModeLive,
			WorkspaceID:     "ws-owner-a",
			ChannelStrategy: participation.StrategyNamed,
			ChannelID:       "ops",
			Source:          participation.SourceExplicitRequest,
			Bounds: participation.Bounds{
				MaxWakes:         1,
				MaxWakeWallTime:  "30s",
				MaxTotalWallTime: "1m",
				MaxInputTokens:   1024,
				MaxOutputTokens:  512,
				MaxWakeDepth:     1,
				CoalesceWindow:   "250ms",
			},
		}

		if _, err := encodeParticipationSnapshot("ws-owner-b", liveSpec); err == nil ||
			!strings.Contains(err.Error(), "does not match owner workspace") {
			t.Fatalf("encodeParticipationSnapshot(mismatched workspace) error = %v, want workspace mismatch", err)
		}

		encoded, err := encodeParticipationSnapshot(liveSpec.WorkspaceID, liveSpec)
		if err != nil {
			t.Fatalf("encodeParticipationSnapshot(matching workspace) error = %v", err)
		}
		if _, err := decodeParticipationSnapshot(
			"ws-owner-b",
			encoded.JSON,
			encoded.Mode,
			encoded.Channel,
			encoded.Source,
		); err == nil || !strings.Contains(err.Error(), "does not match owner workspace") {
			t.Fatalf("decodeParticipationSnapshot(mismatched workspace) error = %v, want workspace mismatch", err)
		}
	})

	t.Run("Should join task-run scan and rows-close errors", func(t *testing.T) {
		t.Parallel()

		globalDB := openQueryRowsCloseErrorGlobalDB(t)
		_, err := globalDB.ListTaskRuns(
			testutil.Context(t),
			taskpkg.RunQuery{TaskID: "task-close-error"},
		)
		if !errors.Is(err, errQueryRowsClose) || !strings.Contains(err.Error(), "scan task run") {
			t.Fatalf("ListTaskRuns() error = %v, want joined scan and rows-close errors", err)
		}
	})

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"task-run-roundtrip",
		filepath.Join(t.TempDir(), "task-run-roundtrip"),
	)
	taskRecord := taskRecordForTest("task-run-roundtrip")
	if err := globalDB.CreateTask(testutil.Context(t), taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	queuedRun := taskRunForTest("run-queued", taskRecord.ID)
	queuedRun.WorkspaceID = workspaceID
	queuedRun.Metadata = json.RawMessage(`{"schema":"agh.harness.detached.v1","owner_session_id":"sess-owner"}`)
	queuedRun.NetworkSpec = participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     workspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       "finance",
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         3,
			MaxWakeWallTime:  "2m",
			MaxTotalWallTime: "10m",
			MaxInputTokens:   4096,
			MaxOutputTokens:  2048,
			MaxWakeDepth:     2,
			CoalesceWindow:   "500ms",
		},
	}
	if err := globalDB.CreateTaskRun(testutil.Context(t), queuedRun); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}

	storedQueued, err := globalDB.GetTaskRun(testutil.Context(t), queuedRun.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(queued) error = %v", err)
	}
	if storedQueued.SessionID != "" {
		t.Fatalf("GetTaskRun(queued).SessionID = %q, want empty", storedQueued.SessionID)
	}
	if storedQueued.ClaimedBy != nil {
		t.Fatalf("GetTaskRun(queued).ClaimedBy = %#v, want nil", storedQueued.ClaimedBy)
	}

	runningRun := queuedRun
	runningRun.Status = taskpkg.TaskRunStatusRunning
	runningRun.ClaimedBy = actorForTest(taskpkg.ActorKindDaemon, "scheduler")
	runningRun.SessionID = "sess-task-run"
	runningRun.ClaimedAt = queuedRun.QueuedAt.Add(30 * time.Second)
	runningRun.StartedAt = queuedRun.QueuedAt.Add(time.Minute)
	runningRun.ClaimTokenHash = "sha256:" + strings.Repeat("a", 64)
	runningRun.LeaseUntil = runningRun.ClaimedAt.Add(10 * time.Minute)
	runningRun.HeartbeatAt = runningRun.ClaimedAt.Add(15 * time.Second)
	runningRun.RequiredCapabilities = []string{"golang", "sqlite"}
	runningRun.PreferredCapabilities = []string{"claude", "codex"}
	if err := globalDB.UpdateTaskRun(testutil.Context(t), runningRun); err != nil {
		t.Fatalf("UpdateTaskRun(running) error = %v", err)
	}

	runsByTask, err := globalDB.ListTaskRuns(testutil.Context(t), taskpkg.RunQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskRuns(task) error = %v", err)
	}
	if got, want := len(runsByTask), 1; got != want {
		t.Fatalf("len(ListTaskRuns(task)) = %d, want %d", got, want)
	}
	assertTaskRunEqual(t, runsByTask[0], runningRun)

	runsBySession, err := globalDB.ListTaskRuns(testutil.Context(t), taskpkg.RunQuery{SessionID: "sess-task-run"})
	if err != nil {
		t.Fatalf("ListTaskRuns(session) error = %v", err)
	}
	if got, want := len(runsBySession), 1; got != want {
		t.Fatalf("len(ListTaskRuns(session)) = %d, want %d", got, want)
	}

	runsByStatus, err := globalDB.ListTaskRunsByStatus(
		testutil.Context(t),
		[]taskpkg.RunStatus{taskpkg.TaskRunStatusRunning},
	)
	if err != nil {
		t.Fatalf("ListTaskRunsByStatus() error = %v", err)
	}
	if got, want := len(runsByStatus), 1; got != want {
		t.Fatalf("len(ListTaskRunsByStatus()) = %d, want %d", got, want)
	}

	activeBindings, err := globalDB.CountActiveSessionBindings(testutil.Context(t), "sess-task-run")
	if err != nil {
		t.Fatalf("CountActiveSessionBindings(running) error = %v", err)
	}
	if got, want := activeBindings, 1; got != want {
		t.Fatalf("CountActiveSessionBindings(running) = %d, want %d", got, want)
	}

	completedRun := runningRun
	completedRun.Status = taskpkg.TaskRunStatusCompleted
	completedRun.EndedAt = runningRun.StartedAt.Add(5 * time.Minute)
	completedRun.Result = json.RawMessage(`{"ok":true}`)
	if err := globalDB.UpdateTaskRun(testutil.Context(t), completedRun); err != nil {
		t.Fatalf("UpdateTaskRun(completed) error = %v", err)
	}

	storedCompleted, err := globalDB.GetTaskRun(testutil.Context(t), completedRun.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(completed) error = %v", err)
	}
	assertTaskRunEqual(t, storedCompleted, completedRun)

	activeBindings, err = globalDB.CountActiveSessionBindings(testutil.Context(t), "sess-task-run")
	if err != nil {
		t.Fatalf("CountActiveSessionBindings(completed) error = %v", err)
	}
	if got, want := activeBindings, 0; got != want {
		t.Fatalf("CountActiveSessionBindings(completed) = %d, want %d", got, want)
	}

	newerRun := taskRunForTest("run-newer-channel", taskRecord.ID)
	newerRun.Attempt = 2
	newerRun.PreviousRunID = completedRun.ID
	newerRun.WorkspaceID = workspaceID
	newerRun.QueuedAt = completedRun.EndedAt.Add(time.Minute)
	newerRun.NetworkSpec = completedRun.NetworkSpec
	newerRun.NetworkSpec.ChannelID = "operations"
	if err := globalDB.CreateTaskRun(testutil.Context(t), newerRun); err != nil {
		t.Fatalf("CreateTaskRun(newer channel) error = %v", err)
	}

	financeRuns, err := globalDB.ListTaskRuns(testutil.Context(t), taskpkg.RunQuery{
		TaskID:               taskRecord.ID,
		ParticipationChannel: "finance",
		Limit:                1,
	})
	if err != nil {
		t.Fatalf("ListTaskRuns(participation channel) error = %v", err)
	}
	if got, want := len(financeRuns), 1; got != want || financeRuns[0].ID != completedRun.ID {
		t.Fatalf(
			"ListTaskRuns(participation channel) = %#v, want completed finance run before limit",
			financeRuns,
		)
	}

	if _, err := globalDB.ListTaskRuns(testutil.Context(t), taskpkg.RunQuery{
		TaskID:               taskRecord.ID,
		ParticipationChannel: "not valid",
	}); !errors.Is(err, taskpkg.ErrValidation) {
		t.Fatalf("ListTaskRuns(invalid participation channel) error = %v, want %v", err, taskpkg.ErrValidation)
	}
}

func TestGlobalDBTaskRunForceOperations(t *testing.T) {
	t.Parallel()

	t.Run("Should force release claimed run with snapshot fencing", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		taskRecord := taskRecordForTest("task-force-release")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-force-release", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		claimed := run
		claimed.Status = taskpkg.TaskRunStatusClaimed
		claimed.ClaimedBy = actorForTest(taskpkg.ActorKindAgentSession, "sess-worker")
		claimed.SessionID = "sess-worker"
		claimed.ClaimTokenHash = "sha256:" + strings.Repeat("a", 64)
		claimed.LeaseUntil = claimed.QueuedAt.Add(10 * time.Minute)
		claimed.HeartbeatAt = claimed.QueuedAt.Add(time.Minute)
		claimed.ClaimedAt = claimed.QueuedAt.Add(time.Second)
		if err := globalDB.UpdateTaskRun(ctx, claimed); err != nil {
			t.Fatalf("UpdateTaskRun(claimed) error = %v", err)
		}

		result, err := globalDB.ForceReleaseTaskRun(ctx, taskpkg.ForceReleaseRunMutation{
			RunID: claimed.ID,
			Now:   claimed.QueuedAt.Add(2 * time.Minute),
		})
		if err != nil {
			t.Fatalf("ForceReleaseTaskRun() error = %v", err)
		}
		if got, want := result.Previous.Status, taskpkg.TaskRunStatusClaimed; got != want {
			t.Fatalf("Previous.Status = %q, want %q", got, want)
		}
		if got, want := result.Run.Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("Run.Status = %q, want %q", got, want)
		}
		if result.Run.SessionID != "" || result.Run.ClaimedBy != nil ||
			result.Run.ClaimTokenHash != "" || !result.Run.LeaseUntil.IsZero() {
			t.Fatalf("ForceReleaseTaskRun() retained lease fields: %#v", result.Run)
		}
	})

	t.Run("Should force fail queued run with operator failure kind", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		taskRecord := taskRecordForTest("task-force-fail")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-force-fail", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		if _, err := globalDB.ForceFailTaskRun(ctx, taskpkg.ForceFailRunMutation{
			RunID:  run.ID,
			Reason: " ",
		}); !errors.Is(err, taskpkg.ErrValidation) {
			t.Fatalf("ForceFailTaskRun(empty reason) error = %v, want %v", err, taskpkg.ErrValidation)
		}

		result, err := globalDB.ForceFailTaskRun(ctx, taskpkg.ForceFailRunMutation{
			RunID:  run.ID,
			Reason: "operator recovery",
			Now:    run.QueuedAt.Add(3 * time.Minute),
		})
		if err != nil {
			t.Fatalf("ForceFailTaskRun() error = %v", err)
		}
		if got, want := result.Run.Status, taskpkg.TaskRunStatusFailed; got != want {
			t.Fatalf("Run.Status = %q, want %q", got, want)
		}
		if got, want := result.Run.FailureKind, taskpkg.FailureKindOperatorForced; got != want {
			t.Fatalf("Run.FailureKind = %q, want %q", got, want)
		}
		if got, want := result.Run.Error, "operator recovery"; got != want {
			t.Fatalf("Run.Error = %q, want %q", got, want)
		}
	})

	t.Run("Should retry failed run once and preserve source row", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		taskRecord := taskRecordForTest("task-force-retry")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		source := taskRunForTest("run-force-retry-source", taskRecord.ID)
		source.Status = taskpkg.TaskRunStatusFailed
		source.Error = "failed before retry"
		source.EndedAt = source.QueuedAt.Add(time.Minute)
		if err := globalDB.CreateTaskRun(ctx, source); err != nil {
			t.Fatalf("CreateTaskRun(source) error = %v", err)
		}

		result, err := globalDB.RetryTaskRun(ctx, taskpkg.RetryRunMutation{
			SourceRunID: source.ID,
			NewRunID:    "run-force-retry-child",
			Origin:      taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "task.retry"},
			Metadata:    json.RawMessage("{\"source\":\"operator\"}"),
			QueuedAt:    source.QueuedAt.Add(2 * time.Minute),
		})
		if err != nil {
			t.Fatalf("RetryTaskRun() error = %v", err)
		}
		if result.PreviousRun.ID != source.ID || result.PreviousRun.Status != taskpkg.TaskRunStatusFailed {
			t.Fatalf("PreviousRun = %#v, want unchanged failed source", result.PreviousRun)
		}
		if result.Run.PreviousRunID != source.ID || result.Run.Status != taskpkg.TaskRunStatusQueued {
			t.Fatalf("Run = %#v, want queued retry linked to source", result.Run)
		}
		storedSource, err := globalDB.GetTaskRun(ctx, source.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(source) error = %v", err)
		}
		if storedSource.Status != taskpkg.TaskRunStatusFailed || storedSource.PreviousRunID != "" {
			t.Fatalf("stored source = %#v, want failed source unchanged", storedSource)
		}
		if _, err := globalDB.RetryTaskRun(ctx, taskpkg.RetryRunMutation{
			SourceRunID: source.ID,
			NewRunID:    "run-force-retry-duplicate",
			Origin:      taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "task.retry"},
			QueuedAt:    source.QueuedAt.Add(3 * time.Minute),
		}); !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			t.Fatalf("RetryTaskRun(duplicate) error = %v, want %v", err, taskpkg.ErrInvalidStatusTransition)
		}
	})
}

func TestGlobalDBReserveQueuedRunDeduplicatesConcurrentIdempotentRequests(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	taskRecord := taskRecordForTest("task-run-reserve-idempotent")
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	origin := taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler"}
	queuedAt := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	metadata := json.RawMessage(`{"schema":"agh.harness.detached.v1","wake_target":{"session_id":"sess-wake"}}`)
	type reserveResult struct {
		task     taskpkg.Task
		run      taskpkg.Run
		existing bool
		err      error
	}

	results := make([]reserveResult, 2)
	runIDs := []string{"run-reserved-a", "run-reserved-b"}
	var wg sync.WaitGroup
	wg.Add(len(results))
	for idx := range results {
		go func(i int) {
			defer wg.Done()
			taskCopy, runCopy, existing, err := globalDB.ReserveQueuedRun(
				ctx,
				queuedRunReservationForTest(
					taskRecord.ID,
					runIDs[i],
					"dup-key",
					origin,
					metadata,
					queuedAt,
				),
			)
			results[i] = reserveResult{
				task:     taskCopy,
				run:      runCopy,
				existing: existing,
				err:      err,
			}
		}(idx)
	}
	wg.Wait()

	for idx, result := range results {
		if result.err != nil {
			t.Fatalf("ReserveQueuedRun(%d) error = %v", idx, result.err)
		}
		if got, want := result.task.ID, taskRecord.ID; got != want {
			t.Fatalf("ReserveQueuedRun(%d) task id = %q, want %q", idx, got, want)
		}
		if got, want := result.run.TaskID, taskRecord.ID; got != want {
			t.Fatalf("ReserveQueuedRun(%d) run task id = %q, want %q", idx, got, want)
		}
		if got, want := result.run.IdempotencyKey, "dup-key"; got != want {
			t.Fatalf("ReserveQueuedRun(%d) idempotency key = %q, want %q", idx, got, want)
		}
		if got, want := result.run.Attempt, int32(1); got != want {
			t.Fatalf("ReserveQueuedRun(%d) attempt = %d, want %d", idx, got, want)
		}
		if got, want := string(result.run.Metadata), string(metadata); got != want {
			t.Fatalf("ReserveQueuedRun(%d) metadata = %s, want %s", idx, got, want)
		}
	}

	if results[0].run.ID != results[1].run.ID {
		t.Fatalf("ReserveQueuedRun() run ids = [%q %q], want same run", results[0].run.ID, results[1].run.ID)
	}

	existingCount := 0
	for _, result := range results {
		if result.existing {
			existingCount++
		}
	}
	if got, want := existingCount, 1; got != want {
		t.Fatalf("existing result count = %d, want %d", got, want)
	}

	runs, err := globalDB.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
	if err != nil {
		t.Fatalf("ListTaskRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(ListTaskRuns()) = %d, want %d", got, want)
	}
	if got, want := runs[0].ID, results[0].run.ID; got != want {
		t.Fatalf("stored run id = %q, want %q", got, want)
	}

	storedRun, err := globalDB.GetTaskRunByIdempotencyKey(ctx, "dup-key", origin)
	if err != nil {
		t.Fatalf("GetTaskRunByIdempotencyKey() error = %v", err)
	}
	if got, want := storedRun.ID, results[0].run.ID; got != want {
		t.Fatalf("GetTaskRunByIdempotencyKey() id = %q, want %q", got, want)
	}
	if got, want := string(storedRun.Metadata), string(metadata); got != want {
		t.Fatalf("GetTaskRunByIdempotencyKey() metadata = %s, want %s", got, want)
	}
}

func TestGlobalDBReserveQueuedRunPersistsResolvedNetworkSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("Should persist the resolved Network snapshot with a queued reservation", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"network-snapshot",
			filepath.Join(t.TempDir(), "network-snapshot"),
		)
		taskRecord := taskRecordForTest("task-run-reserve-network-snapshot")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		resolved := participation.Spec{
			Version:         participation.SpecVersion,
			Mode:            participation.ModeLive,
			WorkspaceID:     workspaceID,
			ChannelStrategy: participation.StrategyNamed,
			ChannelID:       "ops",
			Source:          participation.SourceExplicitRequest,
			Bounds: participation.Bounds{
				MaxWakes:         3,
				MaxWakeWallTime:  "30s",
				MaxTotalWallTime: "2m",
				MaxInputTokens:   4096,
				MaxOutputTokens:  2048,
				MaxWakeDepth:     2,
				CoalesceWindow:   "250ms",
			},
		}
		reservation := queuedRunReservationForTest(
			taskRecord.ID,
			"run-reserve-network-snapshot",
			"network-snapshot-key",
			taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler"},
			nil,
			time.Date(2026, 7, 13, 13, 0, 0, 0, time.UTC),
		)
		reservation.NetworkSpec = resolved
		_, run, existing, err := globalDB.ReserveQueuedRun(ctx, reservation)
		if err != nil {
			t.Fatalf("ReserveQueuedRun() error = %v", err)
		}
		if existing {
			t.Fatal("ReserveQueuedRun() existing = true, want false")
		}
		if !reflect.DeepEqual(run.NetworkSpec, resolved) {
			t.Fatalf("ReserveQueuedRun().NetworkSpec = %#v, want %#v", run.NetworkSpec, resolved)
		}

		var (
			rawSnapshot string
			mode        string
			channel     sql.NullString
			source      string
		)
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT network_spec_json, network_mode, network_channel, network_source
		 FROM task_runs WHERE id = ?`,
			run.ID,
		).Scan(&rawSnapshot, &mode, &channel, &source); err != nil {
			t.Fatalf("query reserved task-run snapshot error = %v", err)
		}
		stored, err := decodeParticipationSnapshot(workspaceID, rawSnapshot, mode, channel, source)
		if err != nil {
			t.Fatalf("decodeParticipationSnapshot() error = %v", err)
		}
		if !reflect.DeepEqual(stored, resolved) {
			t.Fatalf("stored NetworkSpec = %#v, want %#v", stored, resolved)
		}
		var idempotencyRows int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM task_run_idempotency WHERE run_id = ?`,
			run.ID,
		).Scan(&idempotencyRows); err != nil {
			t.Fatalf("count reserved idempotency rows error = %v", err)
		}
		if idempotencyRows != 1 {
			t.Fatalf("reserved idempotency row count = %d, want 1", idempotencyRows)
		}
	})
}

func TestGlobalDBReserveQueuedRunRejectsConcurrentOpenRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		status    taskpkg.RunStatus
		configure func(taskpkg.Run) taskpkg.Run
	}{
		{
			name:   "Should reject another queued reservation while a queued run exists",
			status: taskpkg.TaskRunStatusQueued,
			configure: func(run taskpkg.Run) taskpkg.Run {
				return run
			},
		},
		{
			name:   "Should reject another queued reservation while a claimed run exists",
			status: taskpkg.TaskRunStatusClaimed,
			configure: func(run taskpkg.Run) taskpkg.Run {
				run.ClaimedBy = actorForTest(taskpkg.ActorKindDaemon, "scheduler")
				run.ClaimedAt = run.QueuedAt.Add(30 * time.Second)
				return run
			},
		},
		{
			name:   "Should reject another queued reservation while a starting run exists",
			status: taskpkg.TaskRunStatusStarting,
			configure: func(run taskpkg.Run) taskpkg.Run {
				run.ClaimedBy = actorForTest(taskpkg.ActorKindDaemon, "scheduler")
				run.ClaimedAt = run.QueuedAt.Add(30 * time.Second)
				run.SessionID = "sess-open-starting"
				run.StartedAt = run.QueuedAt.Add(time.Minute)
				return run
			},
		},
		{
			name:   "Should reject another queued reservation while a running run exists",
			status: taskpkg.TaskRunStatusRunning,
			configure: func(run taskpkg.Run) taskpkg.Run {
				run.ClaimedBy = actorForTest(taskpkg.ActorKindDaemon, "scheduler")
				run.ClaimedAt = run.QueuedAt.Add(30 * time.Second)
				run.SessionID = "sess-open-running"
				run.StartedAt = run.QueuedAt.Add(time.Minute)
				return run
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			globalDB := openTestGlobalDB(t)
			ctx := testutil.Context(t)
			taskRecord := taskRecordForTest("task-run-reserve-open-guard")
			if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}

			origin := taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler"}
			queuedAt := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
			_, firstRun, existing, err := globalDB.ReserveQueuedRun(
				ctx,
				queuedRunReservationForTest(
					taskRecord.ID,
					"run-reserved-open-a",
					"open-key",
					origin,
					nil,
					queuedAt,
				),
			)
			if err != nil {
				t.Fatalf("ReserveQueuedRun(first) error = %v", err)
			}
			if existing {
				t.Fatal("ReserveQueuedRun(first) existing = true, want false")
			}

			storedFirstRun, err := globalDB.GetTaskRun(ctx, firstRun.ID)
			if err != nil {
				t.Fatalf("GetTaskRun(first) error = %v", err)
			}
			storedFirstRun.Status = tt.status
			storedFirstRun = tt.configure(storedFirstRun)
			if err := globalDB.UpdateTaskRun(ctx, storedFirstRun); err != nil {
				t.Fatalf("UpdateTaskRun(%s) error = %v", tt.status, err)
			}

			_, duplicateRun, duplicateExisting, err := globalDB.ReserveQueuedRun(
				ctx,
				queuedRunReservationForTest(
					taskRecord.ID,
					"run-reserved-open-duplicate",
					"open-key",
					origin,
					nil,
					queuedAt.Add(time.Second),
				),
			)
			if err != nil {
				t.Fatalf("ReserveQueuedRun(idempotent duplicate) error = %v", err)
			}
			if !duplicateExisting {
				t.Fatal("ReserveQueuedRun(idempotent duplicate) existing = false, want true")
			}
			if got, want := duplicateRun.ID, firstRun.ID; got != want {
				t.Fatalf("ReserveQueuedRun(idempotent duplicate).ID = %q, want %q", got, want)
			}

			_, secondRun, secondExisting, err := globalDB.ReserveQueuedRun(
				ctx,
				queuedRunReservationForTest(
					taskRecord.ID,
					"run-reserved-open-b",
					"new-open-key",
					origin,
					nil,
					queuedAt.Add(2*time.Second),
				),
			)
			if secondExisting {
				t.Fatal("ReserveQueuedRun(second) existing = true, want false")
			}
			if secondRun.ID != "" {
				t.Fatalf("ReserveQueuedRun(second) run = %#v, want zero value", secondRun)
			}
			if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
				t.Fatalf("ReserveQueuedRun(second) error = %v, want %v", err, taskpkg.ErrInvalidStatusTransition)
			}

			runs, err := globalDB.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
			if err != nil {
				t.Fatalf("ListTaskRuns() error = %v", err)
			}
			if got, want := len(runs), 1; got != want {
				t.Fatalf("len(ListTaskRuns()) = %d, want %d", got, want)
			}
			if got, want := runs[0].ID, firstRun.ID; got != want {
				t.Fatalf("stored run id = %q, want %q", got, want)
			}
		})
	}
}

func TestGlobalDBReserveQueuedRunAllowsDesignatedSiblingRuns(t *testing.T) {
	t.Parallel()

	t.Run("Should allow designated sibling runs and reject conflicting reservations", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		taskRecord := taskRecordForTest("task-run-designated-siblings")
		taskRecord.MaxAttempts = 5
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		origin := taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "fanout"}
		queuedAt := time.Date(2026, 7, 1, 16, 0, 0, 0, time.UTC)
		_, firstRun, firstExisting, err := globalDB.ReserveQueuedRun(
			ctx,
			queuedRunReservationForTest(
				taskRecord.ID,
				"run-designated-sibling-a",
				"designated-key-a",
				origin,
				nil,
				queuedAt,
				"designation-group-a",
			),
		)
		if err != nil {
			t.Fatalf("ReserveQueuedRun(first) error = %v", err)
		}
		if firstExisting {
			t.Fatal("ReserveQueuedRun(first) existing = true, want false")
		}
		_, secondRun, secondExisting, err := globalDB.ReserveQueuedRun(
			ctx,
			queuedRunReservationForTest(
				taskRecord.ID,
				"run-designated-sibling-b",
				"designated-key-b",
				origin,
				nil,
				queuedAt.Add(time.Second),
				"designation-group-a",
			),
		)
		if err != nil {
			t.Fatalf("ReserveQueuedRun(second same group) error = %v", err)
		}
		if secondExisting {
			t.Fatal("ReserveQueuedRun(second same group) existing = true, want false")
		}
		if firstRun.ID == secondRun.ID {
			t.Fatalf("designated sibling run IDs are equal: %q", firstRun.ID)
		}
		for _, run := range []taskpkg.Run{firstRun, secondRun} {
			if got, want := run.DesignationGroupID, "designation-group-a"; got != want {
				t.Fatalf("DesignationGroupID(%s) = %q, want %q", run.ID, got, want)
			}
		}

		_, undesignatedRun, undesignatedExisting, err := globalDB.ReserveQueuedRun(
			ctx,
			queuedRunReservationForTest(
				taskRecord.ID,
				"run-designated-sibling-undesignated",
				"designated-key-undesignated",
				origin,
				nil,
				queuedAt.Add(2*time.Second),
			),
		)
		if undesignatedExisting {
			t.Fatal("ReserveQueuedRun(undesignated) existing = true, want false")
		}
		if undesignatedRun.ID != "" {
			t.Fatalf("ReserveQueuedRun(undesignated) run = %#v, want zero value", undesignatedRun)
		}
		if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			t.Fatalf("ReserveQueuedRun(undesignated) error = %v, want %v", err, taskpkg.ErrInvalidStatusTransition)
		}

		_, otherGroupRun, otherGroupExisting, err := globalDB.ReserveQueuedRun(
			ctx,
			queuedRunReservationForTest(
				taskRecord.ID,
				"run-designated-sibling-other-group",
				"designated-key-other-group",
				origin,
				nil,
				queuedAt.Add(3*time.Second),
				"designation-group-b",
			),
		)
		if otherGroupExisting {
			t.Fatal("ReserveQueuedRun(other group) existing = true, want false")
		}
		if otherGroupRun.ID != "" {
			t.Fatalf("ReserveQueuedRun(other group) run = %#v, want zero value", otherGroupRun)
		}
		if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			t.Fatalf("ReserveQueuedRun(other group) error = %v, want %v", err, taskpkg.ErrInvalidStatusTransition)
		}

		runs, err := globalDB.ListTaskRuns(ctx, taskpkg.RunQuery{
			TaskID:             taskRecord.ID,
			DesignationGroupID: "designation-group-a",
		})
		if err != nil {
			t.Fatalf("ListTaskRuns(designation group) error = %v", err)
		}
		if got, want := len(runs), 2; got != want {
			t.Fatalf("len(ListTaskRuns(designation group)) = %d, want %d", got, want)
		}
	})
}

func TestGlobalDBUpdateTaskRunRejectsSessionRebinding(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	taskRecord := taskRecordForTest("task-run-rebinding")
	if err := globalDB.CreateTask(testutil.Context(t), taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	run := taskRunForTest("run-rebinding", taskRecord.ID)
	run.Status = taskpkg.TaskRunStatusRunning
	run.SessionID = "sess-1"
	run.StartedAt = run.QueuedAt.Add(time.Minute)
	if err := globalDB.CreateTaskRun(testutil.Context(t), run); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}

	run.SessionID = "sess-2"
	err := globalDB.UpdateTaskRun(testutil.Context(t), run)
	if !errors.Is(err, taskpkg.ErrSessionAlreadyBound) {
		t.Fatalf("UpdateTaskRun(rebind) error = %v, want ErrSessionAlreadyBound", err)
	}
}

func TestGlobalDBUpdateTaskRunRejectsImmutableIdentityRewrite(t *testing.T) {
	t.Parallel()

	t.Run("Should keep the anchored run and current projection unchanged", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-run-immutable-identity")
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		run := taskRunForTest("run-immutable-identity", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		run.Status = taskpkg.TaskRunStatusRunning
		run.StartedAt = run.QueuedAt.Add(time.Minute)
		if err := globalDB.UpdateTaskRun(ctx, run); err != nil {
			t.Fatalf("UpdateTaskRun(running) error = %v", err)
		}

		rewritten := run
		rewritten.TaskID = ""
		rewritten.WorkspaceID = "ws-wake"
		rewritten.RunKind = taskpkg.RunKindNetworkWake
		rewritten.SetNetworkState(
			participation.LocalSpec(),
			"wake-immutable-identity",
			"sess-target",
			"session:sess-target",
		)
		err := globalDB.UpdateTaskRun(ctx, rewritten)
		if !errors.Is(err, taskpkg.ErrImmutableField) {
			t.Fatalf("UpdateTaskRun(identity rewrite) error = %v, want ErrImmutableField", err)
		}

		storedRun, err := globalDB.GetTaskRun(ctx, run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if storedRun.TaskID != taskRecord.ID || storedRun.RunKind.Normalize() != taskpkg.RunKindWorker {
			t.Fatalf("stored run identity = task %q kind %q, want anchored worker", storedRun.TaskID, storedRun.RunKind)
		}
		storedTask, err := globalDB.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask() error = %v", err)
		}
		if got, want := storedTask.CurrentRunID, run.ID; got != want {
			t.Fatalf("stored task current_run_id = %q, want %q", got, want)
		}
	})
}

func TestGlobalDBUpdateTaskRunAllowsManagedStartSessionTransfer(t *testing.T) {
	t.Parallel()

	t.Run("Should transfer a claimed start run to its dedicated managed session once", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-run-managed-start-transfer")
		if err := globalDB.CreateTask(testutil.Context(t), taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		run := taskRunForTest("run-managed-start-transfer", taskRecord.ID)
		run.Status = taskpkg.TaskRunStatusStarting
		run.ClaimedBy = &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-claimant"}
		run.SessionID = "sess-claimant"
		run.ClaimedAt = run.QueuedAt.Add(time.Minute)
		if err := globalDB.CreateTaskRun(testutil.Context(t), run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}

		run.SessionID = "sess-dedicated"
		if err := globalDB.UpdateTaskRun(testutil.Context(t), run); err != nil {
			t.Fatalf("UpdateTaskRun(managed transfer) error = %v", err)
		}
		stored, err := globalDB.GetTaskRun(testutil.Context(t), run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got, want := stored.SessionID, "sess-dedicated"; got != want {
			t.Fatalf("stored.SessionID = %q, want %q", got, want)
		}

		run.SessionID = "sess-other"
		err = globalDB.UpdateTaskRun(testutil.Context(t), run)
		if !errors.Is(err, taskpkg.ErrSessionAlreadyBound) {
			t.Fatalf("UpdateTaskRun(rebind after transfer) error = %v, want ErrSessionAlreadyBound", err)
		}
	})
}

func TestGlobalDBUpdateTaskRunAllowsQueuedSessionRelease(t *testing.T) {
	t.Parallel()

	t.Run("Should release queued session when requeued", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-run-queued-release")
		if err := globalDB.CreateTask(testutil.Context(t), taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		run := taskRunForTest("run-queued-release", taskRecord.ID)
		run.Status = taskpkg.TaskRunStatusClaimed
		run.ClaimedBy = &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-queued-release"}
		run.SessionID = "sess-queued-release"
		run.ClaimedAt = run.QueuedAt.Add(time.Minute)
		if err := globalDB.CreateTaskRun(testutil.Context(t), run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}

		run.Status = taskpkg.TaskRunStatusQueued
		run.ClaimedBy = nil
		run.SessionID = ""
		run.ClaimedAt = time.Time{}
		err := globalDB.UpdateTaskRun(testutil.Context(t), run)
		if err != nil {
			t.Fatalf("UpdateTaskRun(requeue release) error = %v", err)
		}

		stored, err := globalDB.GetTaskRun(testutil.Context(t), run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(requeued) error = %v", err)
		}
		if got, want := stored.Status, taskpkg.TaskRunStatusQueued; got != want {
			t.Fatalf("stored.Status = %q, want %q", got, want)
		}
		if stored.SessionID != "" || stored.ClaimedBy != nil || !stored.ClaimedAt.IsZero() {
			t.Fatalf(
				"stored lease fields = session %q claimed_by %#v claimed_at %v, want released",
				stored.SessionID,
				stored.ClaimedBy,
				stored.ClaimedAt,
			)
		}
	})
}

func TestGlobalDBUpdateTaskRunRejectsActiveSessionClear(t *testing.T) {
	t.Parallel()

	t.Run("Should reject clearing session binding for active runs", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		taskRecord := taskRecordForTest("task-run-active-clear")
		if err := globalDB.CreateTask(testutil.Context(t), taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}

		run := taskRunForTest("run-active-clear", taskRecord.ID)
		run.Status = taskpkg.TaskRunStatusRunning
		run.SessionID = "sess-active-clear"
		run.StartedAt = run.QueuedAt.Add(time.Minute)
		if err := globalDB.CreateTaskRun(testutil.Context(t), run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}

		run.SessionID = ""
		err := globalDB.UpdateTaskRun(testutil.Context(t), run)
		if !errors.Is(err, taskpkg.ErrSessionAlreadyBound) {
			t.Fatalf("UpdateTaskRun(active clear) error = %v, want ErrSessionAlreadyBound", err)
		}
	})
}

func TestGlobalDBTaskAndRunReferenceErrors(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	_, err := globalDB.GetTask(testutil.Context(t), "missing-task")
	if !errors.Is(err, taskpkg.ErrTaskNotFound) {
		t.Fatalf("GetTask(missing) error = %v, want ErrTaskNotFound", err)
	}

	_, err = globalDB.GetTaskRun(testutil.Context(t), "missing-run")
	if !errors.Is(err, taskpkg.ErrTaskRunNotFound) {
		t.Fatalf("GetTaskRun(missing) error = %v, want ErrTaskRunNotFound", err)
	}

	workspaceTask := taskRecordForTest("task-missing-workspace")
	workspaceTask.Scope = taskpkg.ScopeWorkspace
	workspaceTask.WorkspaceID = "ws-missing"
	err = globalDB.CreateTask(testutil.Context(t), workspaceTask)
	if !errors.Is(err, aghworkspace.ErrWorkspaceNotFound) {
		t.Fatalf("CreateTask(missing workspace) error = %v, want ErrWorkspaceNotFound", err)
	}

	childTask := taskRecordForTest("task-missing-parent")
	childTask.ParentTaskID = "missing-parent"
	err = globalDB.CreateTask(testutil.Context(t), childTask)
	if !errors.Is(err, taskpkg.ErrTaskNotFound) {
		t.Fatalf("CreateTask(missing parent) error = %v, want ErrTaskNotFound", err)
	}

	run := taskRunForTest("run-missing-task", "missing-task")
	err = globalDB.CreateTaskRun(testutil.Context(t), run)
	if !errors.Is(err, taskpkg.ErrTaskNotFound) {
		t.Fatalf("CreateTaskRun(missing task) error = %v, want ErrTaskNotFound", err)
	}
}

func TestTaskNormalizationDefaultsAndHelpers(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	globalDB.now = func() time.Time {
		return time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC)
	}

	record := taskRecordForTest("task-defaults")
	record.CreatedAt = time.Time{}
	record.UpdatedAt = time.Time{}
	record.Owner = ownershipForTest(taskpkg.OwnerKindHuman, " alice ")
	normalizedTask, err := globalDB.normalizeTaskForCreate(record)
	if err != nil {
		t.Fatalf("normalizeTaskForCreate() error = %v", err)
	}
	if !normalizedTask.CreatedAt.Equal(globalDB.now()) {
		t.Fatalf("normalizeTaskForCreate().CreatedAt = %v, want %v", normalizedTask.CreatedAt, globalDB.now())
	}
	if !normalizedTask.UpdatedAt.Equal(globalDB.now()) {
		t.Fatalf("normalizeTaskForCreate().UpdatedAt = %v, want %v", normalizedTask.UpdatedAt, globalDB.now())
	}
	if normalizedTask.Owner == nil || normalizedTask.Owner.Ref != "alice" {
		t.Fatalf("normalizeTaskForCreate().Owner = %#v, want trimmed owner", normalizedTask.Owner)
	}

	updateRecord := taskRecordForTest("task-update-default")
	updateRecord.UpdatedAt = time.Time{}
	normalizedUpdate, err := globalDB.normalizeTaskForUpdate(updateRecord)
	if err != nil {
		t.Fatalf("normalizeTaskForUpdate() error = %v", err)
	}
	if !normalizedUpdate.UpdatedAt.Equal(globalDB.now()) {
		t.Fatalf("normalizeTaskForUpdate().UpdatedAt = %v, want %v", normalizedUpdate.UpdatedAt, globalDB.now())
	}

	run := taskRunForTest("run-defaults", "task-defaults")
	run.Attempt = 0
	run.QueuedAt = time.Time{}
	normalizedRun, err := globalDB.normalizeTaskRunForCreate(run)
	if err != nil {
		t.Fatalf("normalizeTaskRunForCreate() error = %v", err)
	}
	if got, want := normalizedRun.Attempt, int32(1); got != want {
		t.Fatalf("normalizeTaskRunForCreate().Attempt = %d, want %d", got, want)
	}
	if !normalizedRun.QueuedAt.Equal(globalDB.now()) {
		t.Fatalf("normalizeTaskRunForCreate().QueuedAt = %v, want %v", normalizedRun.QueuedAt, globalDB.now())
	}

	runs, err := globalDB.ListTaskRunsByStatus(testutil.Context(t), nil)
	if err != nil {
		t.Fatalf("ListTaskRunsByStatus(nil) error = %v", err)
	}
	if got := len(runs); got != 0 {
		t.Fatalf("len(ListTaskRunsByStatus(nil)) = %d, want 0", got)
	}

	if _, err := requireTaskValue("", "task id"); err == nil {
		t.Fatal("requireTaskValue(empty) error = nil, want non-nil")
	}

	decoded, err := decodeTaskJSON(sqlNullStringForTest(`{"ok":true}`), "test")
	if err != nil {
		t.Fatalf("decodeTaskJSON(valid) error = %v", err)
	}
	if got, want := string(decoded), `{"ok":true}`; got != want {
		t.Fatalf("decodeTaskJSON(valid) = %q, want %q", got, want)
	}
	if _, err := decodeTaskJSON(sqlNullStringForTest(`{"ok":`), "test"); err == nil {
		t.Fatal("decodeTaskJSON(invalid) error = nil, want non-nil")
	}
}

func TestGlobalDBTaskTriageStateRoundTripAndActorIsolation(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	taskRecord := taskRecordForTest("task-triage-roundtrip")
	if err := globalDB.CreateTask(testutil.Context(t), taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	aliceState := taskpkg.TriageState{
		TaskID:             taskRecord.ID,
		Actor:              taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:alice"},
		Read:               true,
		Archived:           true,
		Dismissed:          false,
		LastSeenActivityAt: taskRecord.UpdatedAt.Add(5 * time.Minute),
		UpdatedAt:          taskRecord.UpdatedAt.Add(6 * time.Minute),
	}
	if err := globalDB.UpsertTaskTriageState(testutil.Context(t), aliceState); err != nil {
		t.Fatalf("UpsertTaskTriageState(alice) error = %v", err)
	}

	bobState := taskpkg.TriageState{
		TaskID:    taskRecord.ID,
		Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:bob"},
		Read:      false,
		Archived:  false,
		Dismissed: true,
		UpdatedAt: taskRecord.UpdatedAt.Add(7 * time.Minute),
	}
	if err := globalDB.UpsertTaskTriageState(testutil.Context(t), bobState); err != nil {
		t.Fatalf("UpsertTaskTriageState(bob) error = %v", err)
	}

	storedAlice, err := globalDB.GetTaskTriageState(testutil.Context(t), taskRecord.ID, aliceState.Actor)
	if err != nil {
		t.Fatalf("GetTaskTriageState(alice) error = %v", err)
	}
	if storedAlice != aliceState {
		t.Fatalf("storedAlice = %#v, want %#v", storedAlice, aliceState)
	}

	storedBob, err := globalDB.GetTaskTriageState(testutil.Context(t), taskRecord.ID, bobState.Actor)
	if err != nil {
		t.Fatalf("GetTaskTriageState(bob) error = %v", err)
	}
	if storedBob != bobState {
		t.Fatalf("storedBob = %#v, want %#v", storedBob, bobState)
	}

	aliceState.Archived = false
	aliceState.Dismissed = true
	aliceState.UpdatedAt = aliceState.UpdatedAt.Add(time.Minute)
	if err := globalDB.UpsertTaskTriageState(testutil.Context(t), aliceState); err != nil {
		t.Fatalf("UpsertTaskTriageState(alice update) error = %v", err)
	}

	updatedAlice, err := globalDB.GetTaskTriageState(testutil.Context(t), taskRecord.ID, aliceState.Actor)
	if err != nil {
		t.Fatalf("GetTaskTriageState(updated alice) error = %v", err)
	}
	if updatedAlice != aliceState {
		t.Fatalf("updatedAlice = %#v, want %#v", updatedAlice, aliceState)
	}

	if _, err := globalDB.GetTaskTriageState(
		testutil.Context(t),
		taskRecord.ID,
		taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:charlie"},
	); !errors.Is(err, taskpkg.ErrTaskTriageStateNotFound) {
		t.Fatalf("GetTaskTriageState(missing) error = %v, want %v", err, taskpkg.ErrTaskTriageStateNotFound)
	}
}

func TestGlobalDBListTaskTriageStatesFiltersByActorAndOrdersByUpdate(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	firstTask := taskRecordForTest("task-triage-list-first")
	secondTask := taskRecordForTest("task-triage-list-second")
	secondTask.UpdatedAt = secondTask.UpdatedAt.Add(2 * time.Minute)
	if err := globalDB.CreateTask(testutil.Context(t), firstTask); err != nil {
		t.Fatalf("CreateTask(firstTask) error = %v", err)
	}
	if err := globalDB.CreateTask(testutil.Context(t), secondTask); err != nil {
		t.Fatalf("CreateTask(secondTask) error = %v", err)
	}

	alice := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:alice"}
	aliceFirst := taskpkg.TriageState{
		TaskID:             firstTask.ID,
		Actor:              alice,
		Read:               true,
		LastSeenActivityAt: firstTask.UpdatedAt,
		UpdatedAt:          firstTask.UpdatedAt.Add(5 * time.Minute),
	}
	aliceSecond := taskpkg.TriageState{
		TaskID:             secondTask.ID,
		Actor:              alice,
		Archived:           true,
		LastSeenActivityAt: secondTask.UpdatedAt,
		UpdatedAt:          secondTask.UpdatedAt.Add(8 * time.Minute),
	}
	bob := taskpkg.TriageState{
		TaskID:    secondTask.ID,
		Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:bob"},
		Dismissed: true,
		UpdatedAt: secondTask.UpdatedAt.Add(9 * time.Minute),
	}
	for _, state := range []taskpkg.TriageState{aliceFirst, aliceSecond, bob} {
		if err := globalDB.UpsertTaskTriageState(testutil.Context(t), state); err != nil {
			t.Fatalf("UpsertTaskTriageState(%q/%q) error = %v", state.Actor.Kind, state.Actor.Ref, err)
		}
	}

	aliceStates, err := globalDB.ListTaskTriageStates(testutil.Context(t), alice)
	if err != nil {
		t.Fatalf("ListTaskTriageStates(alice) error = %v", err)
	}
	if got, want := len(aliceStates), 2; got != want {
		t.Fatalf("len(ListTaskTriageStates(alice)) = %d, want %d", got, want)
	}
	if got, want := []string{
		aliceStates[0].TaskID,
		aliceStates[1].TaskID,
	}, []string{
		secondTask.ID,
		firstTask.ID,
	}; !testutil.EqualStringSlices(
		got,
		want,
	) {
		t.Fatalf("alice task ids = %#v, want %#v", got, want)
	}
	if aliceStates[0] != aliceSecond {
		t.Fatalf("aliceStates[0] = %#v, want %#v", aliceStates[0], aliceSecond)
	}
	if aliceStates[1] != aliceFirst {
		t.Fatalf("aliceStates[1] = %#v, want %#v", aliceStates[1], aliceFirst)
	}

	bobStates, err := globalDB.ListTaskTriageStates(
		testutil.Context(t),
		taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user:bob"},
	)
	if err != nil {
		t.Fatalf("ListTaskTriageStates(bob) error = %v", err)
	}
	if got, want := len(bobStates), 1; got != want {
		t.Fatalf("len(ListTaskTriageStates(bob)) = %d, want %d", got, want)
	}
	if bobStates[0] != bob {
		t.Fatalf("bobStates[0] = %#v, want %#v", bobStates[0], bob)
	}
}

func TestGlobalDBRecoverTaskRun(t *testing.T) {
	t.Parallel()

	t.Run("Should terminalize a needs_attention run and queue a linked child", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
		taskRecord := taskRecordForTest("task-recover")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		source := taskRunForTest("run-recover-source", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, source); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE task_runs SET status = 'needs_attention' WHERE id = ?`,
			source.ID,
		); err != nil {
			t.Fatalf("escalate to needs_attention error = %v", err)
		}

		result, err := globalDB.RecoverTaskRun(ctx, taskpkg.RecoverRunMutation{
			SourceRunID: source.ID,
			NewRunID:    "run-recover-child",
			Origin:      taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler"},
			Reason:      "operator unblocked",
			QueuedAt:    now,
		})
		if err != nil {
			t.Fatalf("RecoverTaskRun() error = %v", err)
		}
		if result.PreviousRun.Status.Normalize() != taskpkg.TaskRunStatusFailed {
			t.Fatalf("PreviousRun.Status = %q, want failed", result.PreviousRun.Status)
		}
		if result.Run.Status.Normalize() != taskpkg.TaskRunStatusQueued {
			t.Fatalf("Run.Status = %q, want queued", result.Run.Status)
		}
		if result.Run.PreviousRunID != source.ID {
			t.Fatalf("Run.PreviousRunID = %q, want %q", result.Run.PreviousRunID, source.ID)
		}
		if result.Run.Attempt != source.Attempt+1 {
			t.Fatalf("Run.Attempt = %d, want %d", result.Run.Attempt, source.Attempt+1)
		}
		if !result.Run.LeaseUntil.IsZero() {
			t.Fatalf("child carries lease state: lease=%v", result.Run.LeaseUntil)
		}
		stored, err := globalDB.GetTaskRun(ctx, source.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(source) error = %v", err)
		}
		if stored.Status.Normalize() != taskpkg.TaskRunStatusFailed {
			t.Fatalf("source status = %q, want failed (terminalized in the same tx)", stored.Status)
		}
	})

	t.Run("Should reject recovering a non-needs_attention run", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		taskRecord := taskRecordForTest("task-recover-reject")
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		source := taskRunForTest("run-recover-reject", taskRecord.ID)
		if err := globalDB.CreateTaskRun(ctx, source); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		if _, err := globalDB.RecoverTaskRun(ctx, taskpkg.RecoverRunMutation{
			SourceRunID: source.ID,
			NewRunID:    "run-recover-reject-child",
			Origin:      taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler"},
			QueuedAt:    time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC),
		}); !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			t.Fatalf("RecoverTaskRun(queued) error = %v, want %v", err, taskpkg.ErrInvalidStatusTransition)
		}
	})
}

func taskRecordForTest(id string) taskpkg.Task {
	now := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
	return taskpkg.Task{
		ID:             id,
		Identifier:     "identifier-" + id,
		Scope:          taskpkg.ScopeGlobal,
		Title:          "Task " + id,
		Description:    "Description for " + id,
		Priority:       taskpkg.DefaultPriority,
		MaxAttempts:    taskpkg.DefaultTaskMaxAttempts,
		Status:         taskpkg.TaskStatusPending,
		ApprovalPolicy: taskpkg.ApprovalPolicyNone,
		ApprovalState:  taskpkg.ApprovalStateNotRequired,
		CreatedBy: taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindHuman,
			Ref:  "user:alice",
		},
		Origin: taskpkg.Origin{
			Kind: taskpkg.OriginKindCLI,
			Ref:  "cli",
		},
		CreatedAt:   now,
		UpdatedAt:   now,
		WakeCreator: true,
	}
}

func workspaceTaskRecordForTest(id string, workspaceID string) taskpkg.Task {
	taskRecord := taskRecordForTest(id)
	taskRecord.Scope = taskpkg.ScopeWorkspace
	taskRecord.WorkspaceID = workspaceID
	return taskRecord
}

type countingTaskCatalogExecutor struct {
	taskSQLExecutor
	reads int
}

func (e *countingTaskCatalogExecutor) QueryContext(
	ctx context.Context,
	query string,
	args ...any,
) (*sql.Rows, error) {
	e.reads++
	return e.taskSQLExecutor.QueryContext(ctx, query, args...)
}

func (e *countingTaskCatalogExecutor) QueryRowContext(
	ctx context.Context,
	query string,
	args ...any,
) *sql.Row {
	e.reads++
	return e.taskSQLExecutor.QueryRowContext(ctx, query, args...)
}

func equalTaskCatalogStatusFacets(
	left []taskpkg.CatalogStatusFacet,
	right []taskpkg.CatalogStatusFacet,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func taskBlockRecordForTest(
	id string,
	taskID string,
	kind taskpkg.BlockKind,
	createdAt time.Time,
) taskpkg.TaskBlock {
	return taskpkg.TaskBlock{
		ID:        id,
		TaskID:    taskID,
		Kind:      kind,
		Reason:    "blocked because " + id,
		Details:   json.RawMessage(fmt.Sprintf(`{"block_id":%q}`, id)),
		CreatedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-blocker"},
		CreatedAt: createdAt,
	}
}

func storeLeasedTaskRunForBlockTest(
	ctx context.Context,
	t *testing.T,
	globalDB *GlobalDB,
	taskID string,
	runID string,
	sessionID string,
	rawToken string,
	leaseUntil time.Time,
) taskpkg.Run {
	t.Helper()

	queued := taskRunForTest(runID, taskID)
	if err := globalDB.CreateTaskRun(ctx, queued); err != nil {
		t.Fatalf("CreateTaskRun(%q) error = %v", runID, err)
	}
	storedQueued, err := globalDB.GetTaskRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetTaskRun(%q) error = %v", runID, err)
	}
	leased := leasedRunForGlobalTest(t, runID, taskID, sessionID, rawToken, leaseUntil)
	leased.QueuedAt = queued.QueuedAt
	leased.WorkspaceID = storedQueued.WorkspaceID
	if err := globalDB.UpdateTaskRun(ctx, leased); err != nil {
		t.Fatalf("UpdateTaskRun(%q claimed) error = %v", runID, err)
	}
	return leased
}

func taskRunForTest(id string, taskID string) taskpkg.Run {
	queuedAt := time.Date(2026, 4, 14, 13, 0, 0, 0, time.UTC)
	return taskpkg.Run{
		ID:              id,
		TaskID:          taskID,
		Status:          taskpkg.TaskRunStatusQueued,
		Attempt:         1,
		Origin:          taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "scheduler"},
		RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: participation.LocalSpec()},
		QueuedAt:        queuedAt,
	}
}

func insertLoopRunForCoordinatorIndexTest(ctx context.Context, t *testing.T, db *sql.DB, id string) {
	t.Helper()

	now := store.FormatTimestamp(time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC))
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO loop_runs (
			id, workspace_id, loop_name, status, last_progress_at, inputs_json
		) VALUES (?, ?, ?, ?, ?, ?)`,
		id,
		"ws-loop-index",
		"software-delivery",
		"running",
		now,
		`{}`,
	); err != nil {
		t.Fatalf("insert loop_run %q error = %v", id, err)
	}
}

func insertTaskRunForCoordinatorIndexTest(
	ctx context.Context,
	db *sql.DB,
	runID string,
	taskID string,
	loopRunID string,
	runKind string,
	status string,
) error {
	_, err := db.ExecContext(
		ctx,
		`INSERT INTO task_runs (
			id, task_id, status, attempt, origin_kind, origin_ref, queued_at,
			run_kind, loop_run_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		taskID,
		status,
		1,
		string(taskpkg.OriginKindDaemon),
		"loop-coordinator-index-test",
		store.FormatTimestamp(time.Date(2026, 7, 4, 12, 5, 0, 0, time.UTC)),
		runKind,
		loopRunID,
	)
	return err
}

func ownershipForTest(kind taskpkg.OwnerKind, ref string) *taskpkg.Ownership {
	return &taskpkg.Ownership{Kind: kind, Ref: ref}
}

func actorForTest(kind taskpkg.ActorKind, ref string) *taskpkg.ActorIdentity {
	return &taskpkg.ActorIdentity{Kind: kind, Ref: ref}
}

func assertTaskBlockIDs(t *testing.T, blocks []taskpkg.TaskBlock, want []string) {
	t.Helper()

	got := make([]string, 0, len(blocks))
	for _, block := range blocks {
		got = append(got, block.ID)
	}
	if !testutil.EqualStringSlices(got, want) {
		t.Fatalf("task block ids = %#v, want %#v", got, want)
	}
}

func assertTaskBlockRecurrence(
	t *testing.T,
	got taskpkg.BlockRecurrence,
	taskID string,
	kind taskpkg.BlockKind,
	count int,
	updatedAt time.Time,
) {
	t.Helper()

	if got.TaskID != taskID ||
		got.Kind != kind ||
		got.Count != count ||
		!got.UpdatedAt.Equal(updatedAt) {
		t.Fatalf(
			"task block recurrence = %#v, want task=%q kind=%q count=%d updated_at=%v",
			got,
			taskID,
			kind,
			count,
			updatedAt,
		)
	}
}

func assertTaskEqual(t *testing.T, got taskpkg.Task, want taskpkg.Task) {
	t.Helper()

	if got.ID != want.ID ||
		got.Identifier != want.Identifier ||
		got.Scope != want.Scope ||
		got.WorkspaceID != want.WorkspaceID ||
		got.ParentTaskID != want.ParentTaskID ||
		got.Title != want.Title ||
		got.Description != want.Description ||
		got.Priority != want.Priority ||
		got.MaxAttempts != want.MaxAttempts ||
		got.AutoEnqueueOnReady != want.AutoEnqueueOnReady ||
		got.Status != want.Status ||
		got.ApprovalPolicy != want.ApprovalPolicy ||
		got.ApprovalState != want.ApprovalState ||
		got.CurrentRunID != want.CurrentRunID ||
		got.WakeCreator != want.WakeCreator ||
		got.CreatedBy != want.CreatedBy ||
		got.Origin != want.Origin ||
		!got.CreatedAt.Equal(want.CreatedAt) ||
		!got.UpdatedAt.Equal(want.UpdatedAt) ||
		!got.ClosedAt.Equal(want.ClosedAt) ||
		string(got.Metadata) != string(want.Metadata) {
		t.Fatalf("task = %#v, want %#v", got, want)
	}
	assertOwnershipEqual(t, got.Owner, want.Owner)
	assertNeedsAttentionEqual(t, got.NeedsAttention, want.NeedsAttention)
}

func assertTaskSummaryMatchesTask(t *testing.T, got *taskpkg.Summary, want taskpkg.Task) {
	t.Helper()

	if got.ID != want.ID ||
		got.Identifier != want.Identifier ||
		got.Scope != want.Scope ||
		got.WorkspaceID != want.WorkspaceID ||
		got.ParentTaskID != want.ParentTaskID ||
		got.Title != want.Title ||
		got.Priority != want.Priority ||
		got.MaxAttempts != want.MaxAttempts ||
		got.AutoEnqueueOnReady != want.AutoEnqueueOnReady ||
		got.Status != want.Status ||
		got.ApprovalPolicy != want.ApprovalPolicy ||
		got.ApprovalState != want.ApprovalState ||
		got.CurrentRunID != want.CurrentRunID ||
		got.Draft != (want.Status == taskpkg.TaskStatusDraft) ||
		got.CreatedBy != want.CreatedBy ||
		got.Origin != want.Origin ||
		!got.CreatedAt.Equal(want.CreatedAt) ||
		!got.UpdatedAt.Equal(want.UpdatedAt) ||
		!got.ClosedAt.Equal(want.ClosedAt) {
		t.Fatalf("task summary = %#v, want task %#v", got, want)
	}
	assertOwnershipEqual(t, got.Owner, want.Owner)
}

func assertTaskBlockingSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	for _, column := range []string{
		"needs_attention_reason",
		"needs_attention_at",
		"needs_attention_by_kind",
		"needs_attention_by_ref",
		"wake_creator",
	} {
		assertTableHasColumn(t, db, "tasks", column)
	}
	assertIndexesPresent(t, db, "tasks",
		"idx_tasks_scope",
		"idx_tasks_workspace",
		"idx_tasks_status",
		"idx_tasks_priority",
		"idx_tasks_approval_state",
		"idx_tasks_parent",
		"idx_tasks_owner",
		"idx_tasks_current_run",
		"idx_tasks_paused",
		"idx_tasks_review_policy",
		"idx_tasks_review_round",
		"idx_tasks_created_by",
	)
	assertTablesPresent(t, db, "task_blocks", "task_block_recurrences")
	assertTableHasColumn(t, db, "task_blocks", "workspace_id")
	assertIndexesPresent(t, db, "task_blocks", "idx_task_blocks_open", "idx_task_blocks_expiry")
	assertIndexSQLContains(t, db, "idx_task_blocks_open", "WHERE cleared_at IS NULL")
	assertIndexSQLContains(t, db, "idx_task_blocks_expiry", "WHERE cleared_at IS NULL AND expires_at IS NOT NULL")
	assertTableSQLContains(t, db, "task_blocks", "CHECK (kind IN ('needs_input','capability','transient'))")
	assertTableSQLContains(t, db, "task_blocks", "CHECK (length(reason) > 0)")
	assertTableSQLContains(t, db, "task_block_recurrences", "PRIMARY KEY (task_id, kind)")
	assertTableHasColumn(t, db, "task_runs", "metadata_json")
	assertTableHasColumn(t, db, "task_events", "event_seq")
	assertIndexesPresent(
		t,
		db,
		"task_events",
		"uq_task_events_event_seq",
		"idx_task_events_task_seq",
		"idx_task_events_type_seq",
	)
}

func assertTasksStatusAcceptsNeedsAttention(t *testing.T, globalDB *GlobalDB, taskID string) {
	t.Helper()

	ctx := testutil.Context(t)
	record := taskRecordForTest(taskID)
	if err := globalDB.CreateTask(ctx, record); err != nil {
		t.Fatalf("CreateTask(%s) error = %v", taskID, err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`UPDATE tasks SET status = 'needs_attention' WHERE id = ?`,
		taskID,
	); err != nil {
		t.Fatalf("UPDATE tasks.status to needs_attention error = %v, want nil", err)
	}
}

func assertTableHasColumn(t *testing.T, db *sql.DB, table string, column string) {
	t.Helper()

	columns, err := tableColumns(testutil.Context(t), db, table)
	if err != nil {
		t.Fatalf("tableColumns(%s) error = %v", table, err)
	}
	if _, ok := columns[column]; !ok {
		t.Fatalf("tableColumns(%s) missing %q in %#v", table, column, columns)
	}
}

func assertTableSQLContains(t *testing.T, db *sql.DB, table string, want string) {
	t.Helper()

	sqlText := schemaObjectSQL(t, db, "table", table)
	if !strings.Contains(sqlText, want) {
		t.Fatalf("sqlite schema for table %s = %q, want substring %q", table, sqlText, want)
	}
}

func assertIndexSQLContains(t *testing.T, db *sql.DB, index string, want string) {
	t.Helper()

	sqlText := schemaObjectSQL(t, db, "index", index)
	if !strings.Contains(sqlText, want) {
		t.Fatalf("sqlite schema for index %s = %q, want substring %q", index, sqlText, want)
	}
}

func schemaObjectSQL(t *testing.T, db *sql.DB, objectType string, name string) string {
	t.Helper()

	var sqlText sql.NullString
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT sql FROM sqlite_master WHERE type = ? AND name = ?`,
		objectType,
		name,
	).Scan(&sqlText); err != nil {
		t.Fatalf("query sqlite_master %s %s error = %v", objectType, name, err)
	}
	if !sqlText.Valid {
		return ""
	}
	return strings.TrimSpace(sqlText.String)
}

func assertTaskRunEqual(t *testing.T, got taskpkg.Run, want taskpkg.Run) {
	t.Helper()

	if got.ID != want.ID ||
		got.TaskID != want.TaskID ||
		got.Status != want.Status ||
		got.Attempt != want.Attempt ||
		got.PreviousRunID != want.PreviousRunID ||
		got.FailureKind != want.FailureKind ||
		got.SessionID != want.SessionID ||
		got.Origin != want.Origin ||
		got.IdempotencyKey != want.IdempotencyKey ||
		got.NetworkSpec != want.NetworkSpec ||
		got.ClaimTokenHash != want.ClaimTokenHash ||
		!got.QueuedAt.Equal(want.QueuedAt) ||
		!got.ClaimedAt.Equal(want.ClaimedAt) ||
		!got.StartedAt.Equal(want.StartedAt) ||
		!got.EndedAt.Equal(want.EndedAt) ||
		!got.LeaseUntil.Equal(want.LeaseUntil) ||
		!got.HeartbeatAt.Equal(want.HeartbeatAt) ||
		got.Error != want.Error ||
		string(got.Metadata) != string(want.Metadata) ||
		string(got.Result) != string(want.Result) ||
		!testutil.EqualStringSlices(got.RequiredCapabilities, want.RequiredCapabilities) ||
		!testutil.EqualStringSlices(got.PreferredCapabilities, want.PreferredCapabilities) {
		t.Fatalf("task run = %#v, want %#v", got, want)
	}
	assertActorEqual(t, got.ClaimedBy, want.ClaimedBy)
}

func assertQueryPlanUsesIndex(t *testing.T, db *sql.DB, query string, indexName string, args ...any) {
	t.Helper()

	rows, err := db.QueryContext(testutil.Context(t), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN error = %v", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	details := make([]string, 0)
	for rows.Next() {
		var (
			id      int
			parent  int
			notUsed int
			detail  string
		)
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan query plan error = %v", err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate query plan error = %v", err)
	}
	joined := strings.Join(details, "\n")
	if !strings.Contains(joined, indexName) {
		t.Fatalf("query plan = %q, want index %q", joined, indexName)
	}
}

func assertOwnershipEqual(t *testing.T, got *taskpkg.Ownership, want *taskpkg.Ownership) {
	t.Helper()

	switch {
	case got == nil && want == nil:
		return
	case got == nil || want == nil:
		t.Fatalf("ownership = %#v, want %#v", got, want)
	case *got != *want:
		t.Fatalf("ownership = %#v, want %#v", *got, *want)
	}
}

func assertNeedsAttentionEqual(t *testing.T, got *taskpkg.NeedsAttention, want *taskpkg.NeedsAttention) {
	t.Helper()

	switch {
	case got == nil && want == nil:
		return
	case got == nil || want == nil:
		t.Fatalf("needs attention = %#v, want %#v", got, want)
	case got.Reason != want.Reason || !got.At.Equal(want.At) || got.By != want.By:
		t.Fatalf("needs attention = %#v, want %#v", *got, *want)
	}
}

func assertActorEqual(t *testing.T, got *taskpkg.ActorIdentity, want *taskpkg.ActorIdentity) {
	t.Helper()

	switch {
	case got == nil && want == nil:
		return
	case got == nil || want == nil:
		t.Fatalf("actor = %#v, want %#v", got, want)
	case *got != *want:
		t.Fatalf("actor = %#v, want %#v", *got, *want)
	}
}

func taskSummaryIDs(summaries []taskpkg.Summary) []string {
	ids := make([]string, 0, len(summaries))
	for idx := range summaries {
		ids = append(ids, summaries[idx].ID)
	}
	sort.Strings(ids)
	return ids
}

func orderedTaskSummaryIDs(summaries []taskpkg.Summary) []string {
	ids := make([]string, 0, len(summaries))
	for idx := range summaries {
		ids = append(ids, summaries[idx].ID)
	}
	return ids
}

func queuedRunReservationForTest(
	taskID string,
	runID string,
	idempotencyKey string,
	origin taskpkg.Origin,
	metadata json.RawMessage,
	queuedAt time.Time,
	designationGroupID ...string,
) taskpkg.QueueRunReservation {
	reservation := taskpkg.QueueRunReservation{
		TaskID:         taskID,
		RunID:          runID,
		IdempotencyKey: idempotencyKey,
		Origin:         origin,
		Metadata:       metadata,
		QueuedAt:       queuedAt,
	}
	for _, value := range designationGroupID {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			reservation.DesignationGroupID = trimmed
			break
		}
	}
	return reservation
}

func sqlNullStringForTest(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
