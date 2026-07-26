//go:build integration

package globaldb

import (
	"database/sql"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestGoalDurableStateSchemaConstraintsIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve null token truth and reject invalid turn terminal shapes", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-schema")
		insertGoalSchemaLoopRun(t, globalDB, "run-turns", "ws-goal-schema", "catalog", nil)
		ctx := testutil.Context(t)
		startedAt := store.FormatTimestamp(time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC))
		endedAt := store.FormatTimestamp(time.Date(2026, 7, 10, 10, 1, 0, 0, time.UTC))

		for _, row := range []struct {
			turn     int
			seq      int
			promptID string
			tokens   any
		}{
			{turn: 1, seq: 1, promptID: "prompt-null", tokens: nil},
			{turn: 2, seq: 2, promptID: "prompt-zero", tokens: 0},
		} {
			if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_goal_turns (
				loop_run_id, seq, generation, node_id, item_index, turn,
				session_id, binding_handle, binding_epoch, prompt_id,
				result_status, stop_reason, tokens_used, actor_kind, actor_id, started_at, ended_at
			) VALUES (?, ?, 1, 'converge', 0, ?, 'session-1', 'goal:handle', 1, ?,
				'completed', 'end_turn', ?, 'daemon', 'worker', ?, ?)`,
				"run-turns", row.seq, row.turn, row.promptID, row.tokens, startedAt, endedAt,
			); err != nil {
				t.Fatalf("insert valid turn %d error = %v", row.turn, err)
			}
		}
		var nullCount, zeroCount int
		if err := globalDB.db.QueryRowContext(ctx, `SELECT
			SUM(CASE WHEN tokens_used IS NULL THEN 1 ELSE 0 END),
			SUM(CASE WHEN tokens_used = 0 THEN 1 ELSE 0 END)
			FROM loop_goal_turns WHERE loop_run_id = ?`, "run-turns").Scan(&nullCount, &zeroCount); err != nil {
			t.Fatalf("query token truth error = %v", err)
		}
		if nullCount != 1 || zeroCount != 1 {
			t.Fatalf("token truth counts = null:%d zero:%d, want 1/1", nullCount, zeroCount)
		}

		invalidStatements := []struct {
			name string
			sql  string
		}{
			{
				name: "open turn with terminal fields",
				sql: `INSERT INTO loop_goal_turns (
					loop_run_id, seq, generation, node_id, turn, session_id, binding_handle,
					binding_epoch, prompt_id, stop_reason, actor_kind, actor_id, started_at
				) VALUES ('run-turns', 3, 1, 'bad_open', 1, 'session-1', 'goal:bad',
					1, 'bad-open', 'end_turn', 'daemon', 'worker', '` + startedAt + `')`,
			},
			{
				name: "completed turn without one stop reason",
				sql: `INSERT INTO loop_goal_turns (
					loop_run_id, seq, generation, node_id, turn, session_id, binding_handle,
					binding_epoch, prompt_id, result_status, actor_kind, actor_id, started_at, ended_at
				) VALUES ('run-turns', 4, 1, 'bad_complete', 1, 'session-1', 'goal:bad',
					1, 'bad-complete', 'completed', 'daemon', 'worker', '` + startedAt + `', '` + endedAt + `')`,
			},
			{
				name: "invalid result with ambiguous terminal shape",
				sql: `INSERT INTO loop_goal_turns (
					loop_run_id, seq, generation, node_id, turn, session_id, binding_handle,
					binding_epoch, prompt_id, result_status, stop_reason, reason_code,
					actor_kind, actor_id, started_at, ended_at
				) VALUES ('run-turns', 5, 1, 'bad_invalid', 1, 'session-1', 'goal:bad',
					1, 'bad-invalid', 'invalid-result', 'end_turn', 'goal_stop_reason_invalid',
					'daemon', 'worker', '` + startedAt + `', '` + endedAt + `')`,
			},
			{
				name: "ambiguous turn with wrong reason",
				sql: `INSERT INTO loop_goal_turns (
					loop_run_id, seq, generation, node_id, turn, session_id, binding_handle,
					binding_epoch, prompt_id, result_status, reason_code,
					actor_kind, actor_id, started_at, ended_at
				) VALUES ('run-turns', 6, 1, 'bad_ambiguous', 1, 'session-1', 'goal:bad',
					1, 'bad-ambiguous', 'ambiguous', 'unknown',
					'daemon', 'worker', '` + startedAt + `', '` + endedAt + `')`,
			},
			{
				name: "invalid blocking json",
				sql: `INSERT INTO loop_goal_turns (
					loop_run_id, seq, generation, node_id, turn, session_id, binding_handle,
					binding_epoch, prompt_id, blocking_json, actor_kind, actor_id, started_at
				) VALUES ('run-turns', 7, 1, 'bad_json', 1, 'session-1', 'goal:bad',
					1, 'bad-json', '{', 'daemon', 'worker', '` + startedAt + `')`,
			},
			{
				name: "negative token count",
				sql: `INSERT INTO loop_goal_turns (
					loop_run_id, seq, generation, node_id, turn, session_id, binding_handle,
					binding_epoch, prompt_id, tokens_used, actor_kind, actor_id, started_at
				) VALUES ('run-turns', 8, 1, 'bad_tokens', 1, 'session-1', 'goal:bad',
					1, 'bad-tokens', -1, 'daemon', 'worker', '` + startedAt + `')`,
			},
			{
				name: "end before start",
				sql: `INSERT INTO loop_goal_turns (
					loop_run_id, seq, generation, node_id, turn, session_id, binding_handle,
					binding_epoch, prompt_id, result_status, stop_reason,
					actor_kind, actor_id, started_at, ended_at
				) VALUES ('run-turns', 9, 1, 'bad_time', 1, 'session-1', 'goal:bad',
					1, 'bad-time', 'completed', 'end_turn', 'daemon', 'worker',
					'` + endedAt + `', '` + startedAt + `')`,
			},
		}
		for _, tc := range invalidStatements {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				if _, err := globalDB.db.ExecContext(ctx, tc.sql); err == nil {
					t.Fatalf("ExecContext(%s) error = nil, want constraint failure", tc.name)
				}
			})
		}
	})

	t.Run("Should enforce checkpoint judge binding queue run and outbox checks", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-controls")
		originSession := "origin-session"
		insertGoalSchemaLoopRun(t, globalDB, "run-controls", "ws-goal-controls", "session", &originSession)
		ctx := testutil.Context(t)
		now := store.FormatTimestamp(time.Date(2026, 7, 10, 11, 0, 0, 0, time.UTC))

		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_goal_checkpoints (
			loop_run_id, generation, node_id, item_index, phase, goal_status,
			turn_limit, context_state, usage_sequence, context_nudge_ratio,
			control_grant_id, control_grant_kind, control_grant_cause, control_grant_turn,
			control_grant_scope, control_grant_consumed,
			report_prompt_id, report_status, report_binding_epoch,
			report_actor_kind, report_actor_id, report_recorded_at, updated_at
		) VALUES ('run-controls', 1, 'converge', 0, 'awaiting_control', 'paused',
			10, 'known', 0, 0.8,
			1, 'budget', 'budget_exceeded', 1, 'work-and-settle', 0,
			'report-1', 'complete', 1, 'agent', 'session-1', ?, ?)`, now, now); err != nil {
			t.Fatalf("insert valid checkpoint error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_goal_judge_attempts (
			attempt_id, loop_run_id, generation, node_id, item_index, turn,
			judge_digest, status, usage_base_tokens, started_at
		) VALUES ('judge-1', 'run-controls', 1, 'converge', 0, 1,
			'judge-digest', 'running', 0, ?)`, now); err != nil {
			t.Fatalf("insert valid judge attempt error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_session_bindings (
			loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
			creation_profile_ref, policy_spec_digest, creation_digest, ownership, state,
			created_at, activated_at
		) VALUES ('run-controls', 'goal:origin', 1, 'binding-1', 'origin-session', 'ws-goal-controls',
			'profile-1', 'policy-1', 'creation-1', 'origin-borrowed', 'active', ?, ?)`, now, now); err != nil {
			t.Fatalf("insert valid binding error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_goal_session_outbox (
			event_id, workspace_id, origin_session_id, loop_run_id, cause, created_at
		) VALUES ('goal-snapshot:1', 'ws-goal-controls', 'origin-session', 'run-controls', 'start', ?)`, now); err != nil {
			t.Fatalf("insert valid outbox row error = %v", err)
		}

		invalidStatements := []struct {
			name string
			sql  string
		}{
			{
				name: "checkpoint nudge ratio above one",
				sql: `INSERT INTO loop_goal_checkpoints (
					loop_run_id, generation, node_id, phase, goal_status, turn_limit,
					context_nudge_ratio, updated_at
				) VALUES ('run-controls', 1, 'bad_ratio', 'idle', 'active', 1, 1.1, '` + now + `')`,
			},
			{
				name: "partial control grant",
				sql: `INSERT INTO loop_goal_checkpoints (
					loop_run_id, generation, node_id, phase, goal_status, turn_limit,
					context_nudge_ratio, control_grant_id, control_grant_kind, updated_at
				) VALUES ('run-controls', 1, 'bad_grant', 'idle', 'active', 1, 0.8, 1, 'budget', '` + now + `')`,
			},
			{
				name: "partial report intent",
				sql: `INSERT INTO loop_goal_checkpoints (
					loop_run_id, generation, node_id, phase, goal_status, turn_limit,
					context_nudge_ratio, report_status, updated_at
				) VALUES ('run-controls', 1, 'bad_report', 'idle', 'active', 1, 0.8, 'blocked', '` + now + `')`,
			},
			{
				name: "completed judge without outcome",
				sql: `INSERT INTO loop_goal_judge_attempts (
					attempt_id, loop_run_id, generation, node_id, turn, judge_digest,
					status, started_at, completed_at
				) VALUES ('judge-bad', 'run-controls', 1, 'bad_judge', 1, 'digest',
					'completed', '` + now + `', '` + now + `')`,
			},
			{
				name: "borrowed successor binding",
				sql: `INSERT INTO loop_session_bindings (
					loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
					creation_profile_ref, policy_spec_digest, creation_digest, ownership, state,
					created_at, activated_at
				) VALUES ('run-controls', 'goal:bad', 2, 'binding-bad', 'origin-session', 'ws-goal-controls',
					'profile-1', 'policy-1', 'creation-bad', 'origin-borrowed', 'active', '` + now + `', '` + now + `')`,
			},
			{
				name: "negative queue terminal tokens",
				sql: `INSERT INTO session_input_queue (
					id, session_id, status, mode, text, enqueued_at, updated_at, terminal_tokens_used
				) VALUES ('queue-bad', 'missing-session', 'queued', 'queue', 'prompt', '` + now + `', '` + now + `', -1)`,
			},
			{
				name: "unknown outbox cause",
				sql: `INSERT INTO loop_goal_session_outbox (
					event_id, workspace_id, origin_session_id, loop_run_id, cause, created_at
				) VALUES ('goal-snapshot:bad', 'ws-goal-controls', 'origin-session', 'run-controls', 'unknown', '` + now + `')`,
			},
			{
				name: "negative run budget version",
				sql:  `UPDATE loop_runs SET budget_version = -1 WHERE id = 'run-controls'`,
			},
		}
		for _, tc := range invalidStatements {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				if _, err := globalDB.db.ExecContext(ctx, tc.sql); err == nil {
					t.Fatalf("ExecContext(%s) error = nil, want constraint failure", tc.name)
				}
			})
		}
	})
}

func assertGoalDurableStateSchema(t *testing.T, globalDB *GlobalDB) {
	t.Helper()

	assertTablesPresent(t, globalDB.db,
		"loop_goal_turns",
		"loop_goal_checkpoints",
		"loop_goal_judge_attempts",
		"loop_session_bindings",
		"loop_goal_session_outbox",
		"loop_goal_session_cleanup",
		"loop_goal_binding_retry_witnesses",
	)
	assertTableColumns(t, globalDB.db, "loop_goal_turns", []string{
		"loop_run_id", "seq", "generation", "node_id", "item_index", "turn",
		"session_id", "binding_handle", "binding_epoch", "prompt_id", "prompt_attempt",
		"usage_base_tokens", "result_status", "stop_reason", "reason_code", "verdict_outcome",
		"blocking_json", "evidence_ref", "prompt_ref", "tokens_used", "actor_kind", "actor_id",
		"started_at", "ended_at",
	})
	assertTableColumns(t, globalDB.db, "loop_goal_checkpoints", []string{
		"loop_run_id", "generation", "node_id", "item_index", "control_epoch",
		"control_actor_kind", "control_actor_id", "control_requested_at", "phase", "goal_status",
		"turns_used", "turn_limit", "broken_streak", "recovery_streak", "task_run_id",
		"queue_entry_id", "prompt_id", "prompt_kind", "prompt_attempt", "context_state",
		"usage_sequence", "usage_pending_after_sequence", "session_id", "binding_handle",
		"binding_epoch", "context_nudge_ratio", "control_grant_id", "control_grant_kind",
		"control_grant_cause", "control_grant_turn", "control_grant_scope", "control_grant_consumed",
		"judge_attempt_id", "compaction_cancel_prompt_id", "compaction_cancel_cause",
		"compaction_cancel_requested_at", "report_prompt_id", "report_status", "report_evidence_ref",
		"report_binding_epoch", "report_actor_kind", "report_actor_id", "report_recorded_at", "updated_at",
		"control_cause", "compaction_baseline_used", "compaction_recovery_required",
	})
	assertTableColumns(t, globalDB.db, "loop_goal_judge_attempts", []string{
		"attempt_id", "loop_run_id", "generation", "node_id", "item_index", "turn",
		"judge_digest", "status", "outcome", "blocking_json", "evidence_ref", "tokens_used",
		"usage_base_tokens", "started_at", "completed_at",
	})
	assertTableColumns(t, globalDB.db, "loop_session_bindings", []string{
		"loop_run_id", "handle", "binding_epoch", "binding_attempt_id", "session_id", "workspace_id",
		"creation_profile_ref", "policy_spec_digest", "creation_digest", "ownership", "state",
		"failure_code", "created_at", "activated_at", "failed_at", "closed_at", "adopted_generation",
		"adoption_attempt_id",
	})
	assertTableColumns(t, globalDB.db, "loop_goal_session_outbox", []string{
		"id", "event_id", "workspace_id", "origin_session_id", "loop_run_id", "bound_session_id",
		"cause", "created_at", "delivered_at",
	})
	assertTableColumns(t, globalDB.db, "loop_goal_session_cleanup", []string{
		"id", "cleanup_id", "workspace_id", "loop_run_id", "handle", "binding_epoch",
		"session_id", "cause", "created_at", "completed_at",
	})
	assertTableColumns(t, globalDB.db, "loop_goal_binding_retry_witnesses", []string{
		"loop_run_id", "handle", "failed_binding_epoch", "request_digest", "created_at",
	})

	for _, column := range []string{"creation_digest", "policy_spec_digest", "creation_profile_ref"} {
		assertTableHasColumn(t, globalDB.db, "sessions", column)
	}
	for _, column := range []string{"goal_status", "goal_turns_used", "goal_turn_limit"} {
		assertTableHasColumn(t, globalDB.db, "loop_generation_outputs", column)
	}
	for _, column := range []string{
		"origin_kind", "origin_session_id", "goal_cleared_at", "budget_version", "goal_context_nudge_ratio",
		"origin_creation_profile_ref", "origin_policy_spec_digest", "origin_creation_digest",
	} {
		assertTableHasColumn(t, globalDB.db, "loop_runs", column)
	}
	for _, column := range []string{
		"loop_run_id", "owner_kind", "owner_epoch", "binding_epoch", "prompt_id", "prompt_kind",
		"operation_usage_base_tokens", "prompt_attempt", "dispatchable", "activated_at",
		"dispatch_token_hash", "fence_kind", "fence_disposition", "fence_reason_code", "fenced_at",
		"terminal_event_start_seq", "terminal_event_end_seq", "terminal_kind", "terminal_stop_reason",
		"terminal_disposition", "terminal_reason_code", "terminal_tokens_reported", "terminal_tokens_used",
		"terminal_at",
	} {
		assertTableHasColumn(t, globalDB.db, "session_input_queue", column)
	}

	assertIndexesPresent(t, globalDB.db, "loop_session_bindings", "uq_loop_session_bindings_active")
	assertIndexesPresent(t, globalDB.db, "loop_runs", "uq_loop_runs_active_session_goal")
	assertIndexesPresent(t, globalDB.db, "session_input_queue",
		"idx_session_input_queue_goal_owner", "uq_session_input_queue_goal_prompt")
	assertIndexesPresent(t, globalDB.db, "loop_goal_session_outbox", "idx_loop_goal_session_outbox_pending")
	assertIndexesPresent(t, globalDB.db, "loop_goal_session_cleanup", "idx_loop_goal_session_cleanup_pending")
	assertIndexSQLContains(t, globalDB.db, "uq_loop_session_bindings_active", "WHERE state='active'")
	assertIndexSQLContains(t, globalDB.db, "uq_loop_runs_active_session_goal", "origin_kind='session'")
	assertIndexSQLContains(t, globalDB.db, "uq_loop_runs_active_session_goal", "needs-approval")
	assertIndexSQLContains(t, globalDB.db, "uq_session_input_queue_goal_prompt", "WHERE prompt_id IS NOT NULL")
	assertIndexSQLContains(t, globalDB.db, "idx_loop_goal_session_outbox_pending", "WHERE delivered_at IS NULL")
	assertIndexSQLContains(t, globalDB.db, "idx_loop_goal_session_cleanup_pending", "WHERE completed_at IS NULL")

	assertTableSQLContains(t, globalDB.db, "loop_goal_turns", "UNIQUE (loop_run_id, seq)")
	assertTableSQLContains(t, globalDB.db, "loop_goal_turns", "goal_stop_reason_invalid")
	assertTableSQLContains(t, globalDB.db, "loop_goal_turns", "goal_control_revoked_in_flight")
	assertTableSQLContains(t, globalDB.db, "loop_goal_checkpoints", "control_grant_kind")
	assertTableSQLContains(t, globalDB.db, "loop_goal_checkpoints", "report_recorded_at")
	assertTableSQLContains(t, globalDB.db, "loop_goal_judge_attempts", "status IN ('running','completed','ambiguous')")
	assertTableSQLContains(t, globalDB.db, "loop_session_bindings", "ownership = 'run-owned'")
}

func insertGoalSchemaLoopRun(
	t *testing.T,
	globalDB *GlobalDB,
	runID string,
	workspaceID string,
	originKind string,
	originSessionID *string,
) {
	t.Helper()

	now := store.FormatTimestamp(time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC))
	var sessionID sql.NullString
	if originSessionID != nil {
		sessionID = sql.NullString{String: *originSessionID, Valid: true}
	}
	if _, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO loop_runs (
			id, workspace_id, loop_name, status, last_progress_at, inputs_json,
			origin_kind, origin_session_id
		) VALUES (?, ?, 'goal-schema', 'running', ?, '{}', ?, ?)`,
		runID,
		workspaceID,
		now,
		originKind,
		sessionID,
	); err != nil {
		t.Fatalf("insert loop run %q error = %v", runID, err)
	}
}
