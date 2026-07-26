//go:build integration

package globaldb

import (
	"path/filepath"
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

func TestOpenGlobalDBBootstrapsLoopSchemaIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should create loop schema on fresh DB and preserve it after reopen", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)

		first, err := OpenGlobalDB(testutil.Context(t), path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}
		assertLoopRunStateSchema(t, first)
		if err := first.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		second, err := OpenGlobalDB(testutil.Context(t), path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(second) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := second.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(second cleanup) error = %v", closeErr)
			}
		})
		assertLoopRunStateSchema(t, second)
	})
}

func assertLoopRunStateSchema(t *testing.T, globalDB *GlobalDB) {
	t.Helper()

	assertTablesPresent(
		t,
		globalDB.db,
		"loop_runs",
		"loop_generation_outputs",
		"loop_output_blobs",
		"loop_run_events",
		"loop_definition_snapshots",
		"loop_gate_decisions",
		"loop_ui_annotations",
		"loop_config",
	)
	assertTableColumns(t, globalDB.db, "loop_runs", []string{
		"id",
		"workspace_id",
		"loop_name",
		"status",
		"generation",
		"reattempt_strategy",
		"last_progress_at",
		"budget_tokens",
		"budget_wall_sec",
		"budget_on_exceeded",
		"tokens_used",
		"parent_loop_run_id",
		"pause_requested",
		"inputs_json",
		"created_at",
		"iteration_cap",
		"started_by_kind",
		"started_by_ref",
		"started_origin_kind",
		"started_origin_ref",
		"started_at",
		"definition_version",
		"definition_digest",
		"active_gate_id",
		"active_human_criteria_json",
		"budget_approval_seq",
		"start_metadata_json",
		"origin_kind",
		"origin_session_id",
		"goal_cleared_at",
		"budget_version",
		"goal_context_nudge_ratio",
		"control_actor_kind",
		"control_actor_id",
		"control_requested_at",
		"origin_creation_profile_ref",
		"origin_policy_spec_digest",
		"origin_creation_digest",
		"network_spec_json",
		"network_mode",
		"network_channel",
		"network_source",
	})
	assertTableExcludesColumns(t, globalDB.db, "loop_runs", []string{"consecutive_failures"})
	assertTableColumns(t, globalDB.db, "loop_generation_outputs", []string{
		"loop_run_id",
		"generation",
		"node_id",
		"item_index",
		"status",
		"output_ref",
		"task_run_id",
		"child_loop_run_id",
		"goal_status",
		"goal_turns_used",
		"goal_turn_limit",
	})
	assertTableColumns(t, globalDB.db, "loop_output_blobs", []string{
		"output_ref",
		"payload_json",
		"byte_size",
		"created_at",
		"last_used_at",
	})
	assertTableColumns(t, globalDB.db, "loop_run_events", []string{
		"id",
		"loop_run_id",
		"workspace_id",
		"seq",
		"kind",
		"payload_json",
		"at",
	})
	assertTableColumns(t, globalDB.db, "loop_definition_snapshots", []string{
		"workspace_id",
		"definition_digest",
		"definition_version",
		"definition_json",
		"byte_size",
		"created_at",
		"last_used_at",
	})
	assertTableColumns(t, globalDB.db, "loop_gate_decisions", []string{
		"workspace_id",
		"loop_run_id",
		"generation",
		"gate_id",
		"criterion_id",
		"decision",
		"actor_kind",
		"actor_ref",
		"origin_kind",
		"origin_ref",
		"note",
		"decided_at",
	})
	assertTableColumns(t, globalDB.db, "loop_ui_annotations", []string{
		"workspace_id",
		"loop_name",
		"node_id",
		"x",
		"y",
	})
	assertTableColumns(t, globalDB.db, "loop_config", []string{
		"workspace_id",
		"loop_name",
		"human_gate_enabled",
		"reattempt_strategy",
		"enabled_checks_json",
		"iteration_cap",
		"budget_tokens",
		"budget_wall_sec",
		"budget_on_exceeded",
		"no_progress_window",
		"fan_out_width",
		"gate_max_revisions",
		"model_default_worker",
		"model_default_judge",
	})
	assertTableHasColumn(t, globalDB.db, "task_runs", "run_kind")
	assertTableHasColumn(t, globalDB.db, "task_runs", "loop_run_id")
	assertTableHasColumn(t, globalDB.db, "task_runs", "tokens_used")
	assertIndexesPresent(t, globalDB.db, "loop_run_events", "idx_loop_run_events_run_seq")
	assertIndexSQLContains(t, globalDB.db, "idx_loop_run_events_run_seq", "ON loop_run_events(loop_run_id, seq)")
	assertIndexesPresent(t, globalDB.db, "loop_gate_decisions", "idx_loop_gate_decisions_workspace_run")
	assertIndexSQLContains(
		t,
		globalDB.db,
		"idx_loop_gate_decisions_workspace_run",
		"ON loop_gate_decisions(workspace_id, loop_run_id, generation, gate_id)",
	)
	assertIndexesPresent(t, globalDB.db, "loop_runs", "idx_loop_runs_queue_order")
	assertIndexSQLContains(
		t,
		globalDB.db,
		"idx_loop_runs_queue_order",
		"ON `loop_runs` (`workspace_id`, `loop_name`, `status`, `created_at`, `id`)",
	)
	assertIndexesPresent(t, globalDB.db, "loop_runs", "idx_loop_runs_catalog")
	assertIndexSQLContains(
		t,
		globalDB.db,
		"idx_loop_runs_catalog",
		"ON `loop_runs` (`workspace_id`, `loop_name`, `created_at` DESC, `id` DESC, `status`)",
	)
	assertIndexesPresent(t, globalDB.db, "task_runs", "uq_task_runs_active_loop_coordinator")
	assertIndexesPresent(t, globalDB.db, "loop_generation_outputs", "idx_loop_generation_outputs_output_ref")
	assertIndexSQLContains(
		t,
		globalDB.db,
		"uq_task_runs_active_loop_coordinator",
		"ON `task_runs` (`loop_run_id`)",
	)
	assertIndexSQLContains(t, globalDB.db, "uq_task_runs_active_loop_coordinator", "run_kind = 'coordinator'")
	assertIndexSQLContains(
		t,
		globalDB.db,
		"idx_loop_generation_outputs_output_ref",
		"ON loop_generation_outputs(output_ref)",
	)
	assertIndexSQLContains(
		t,
		globalDB.db,
		"uq_task_runs_active_loop_coordinator",
		"status IN ('queued', 'claimed', 'starting', 'running')",
	)
	assertTableSQLContains(t, globalDB.db, "loop_generation_outputs", "REFERENCES loop_runs(id) ON DELETE CASCADE")
	assertTableSQLContains(
		t,
		globalDB.db,
		"loop_generation_outputs",
		"PRIMARY KEY (loop_run_id, generation, node_id, item_index)",
	)
	assertTableSQLContains(t, globalDB.db, "loop_output_blobs", "PRIMARY KEY")
	assertTableSQLContains(
		t,
		globalDB.db,
		"loop_definition_snapshots",
		"PRIMARY KEY (workspace_id, definition_digest)",
	)
	assertTableSQLContains(
		t,
		globalDB.db,
		"loop_gate_decisions",
		"REFERENCES loop_runs(id) ON DELETE CASCADE",
	)
	assertTableSQLContains(t, globalDB.db, "loop_ui_annotations", "PRIMARY KEY (workspace_id, loop_name, node_id)")
	assertTableSQLContains(t, globalDB.db, "loop_config", "PRIMARY KEY (workspace_id, loop_name)")
	assertGoalDurableStateSchema(t, globalDB)
}
