//go:build integration

package globaldb

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
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

	t.Run("Should enforce immutable lineage and paired best-state constraints", func(t *testing.T) {
		t.Parallel()

		const (
			originConstraint = "origin IN"
			parentConstraint = "parent_generation >= 0 AND parent_generation < generation"
			bestConstraint   = "best_generation IS NULL AND best_score IS NULL"
		)
		cases := []struct {
			name              string
			query             string
			expectedErrorPart string
			includeCreatedAt  bool
		}{
			{
				name:              "Should reject an unknown generation origin",
				query:             `INSERT INTO loop_generations (loop_run_id, generation, parent_generation, origin, created_at) VALUES (?, 2, 1, 'unknown', ?)`,
				expectedErrorPart: originConstraint,
				includeCreatedAt:  true,
			},
			{
				name:              "Should reject a non-predecessor generation parent",
				query:             `INSERT INTO loop_generations (loop_run_id, generation, parent_generation, origin, created_at) VALUES (?, 2, 2, 'reattempt', ?)`,
				expectedErrorPart: parentConstraint,
				includeCreatedAt:  true,
			},
			{
				name:              "Should reject a half-set best state",
				query:             `UPDATE loop_runs SET best_generation = 1, best_score = NULL WHERE id = ?`,
				expectedErrorPart: bestConstraint,
			},
			{
				name:              "Should reject an out-of-range best generation",
				query:             `UPDATE loop_runs SET best_generation = 2, best_score = 0.9 WHERE id = ?`,
				expectedErrorPart: bestConstraint,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				globalDB := openLoopTestGlobalDB(t)
				ctx := testutil.Context(t)
				now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
				run := testLoopRun("looprun-schema-constraints", now, looppkg.StatusRunning)
				created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
				if err != nil {
					t.Fatalf("CreateLoopRunForStart() error = %v", err)
				}
				args := []any{string(created.ID)}
				if tc.includeCreatedAt {
					args = append(args, now)
				}
				if _, err := globalDB.db.ExecContext(ctx, tc.query, args...); err == nil {
					t.Fatal("invalid schema state accepted, want constraint error")
				} else if !strings.Contains(err.Error(), tc.expectedErrorPart) {
					t.Fatalf("constraint error = %v, want substring %q", err, tc.expectedErrorPart)
				}
			})
		}
	})

	t.Run("Should upgrade v38 loop rows and preserve lifecycle defaults after reopen", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00039_schema.sql"),
		)
		if err != nil {
			t.Fatalf("openGlobalMigrationPrefixDatabase() error = %v", err)
		}
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
		prefixGlobalDB := &GlobalDB{db: prefixDB, path: path, now: func() time.Time { return now }}
		prefixGlobalDB.initializeRepositories(openConfig{})
		workspaceID := registerWorkspaceForGlobalTests(t, prefixGlobalDB, "loop-lifecycle-upgrade", t.TempDir())
		if _, err := prefixDB.ExecContext(
			ctx,
			`INSERT INTO loop_runs (
				id, workspace_id, loop_name, status, generation, reattempt_strategy,
				last_progress_at, inputs_json, created_at, started_at
			) VALUES (?, ?, 'delivery', 'running', 1, 'failed_only', ?, '{}', ?, ?)`,
			"run-v38",
			workspaceID,
			now,
			now,
			now,
		); err != nil {
			t.Fatalf("insert v38 loop run error = %v", err)
		}
		if _, err := prefixDB.ExecContext(
			ctx,
			`INSERT INTO loop_generations (
				loop_run_id, generation, parent_generation, origin, created_at
			) VALUES ('run-v38', 1, 0, 'initial', ?)`,
			now,
		); err != nil {
			t.Fatalf("insert v38 generation error = %v", err)
		}
		if _, err := prefixDB.ExecContext(
			ctx,
			`INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status, output_ref, task_run_id,
				resolved_runtime_json, goal_status, goal_turns_used, goal_turn_limit
			) VALUES ('run-v38', 1, 'goal', 0, 'succeeded', 'output-v38', 'task-v38',
				'{"model":"pinned"}', 'complete', 4, 9)`,
		); err != nil {
			t.Fatalf("insert v38 generation output error = %v", err)
		}
		if _, err := prefixDB.ExecContext(
			ctx,
			`INSERT INTO dead_entities (workspace_id, kind, entity_id, reason, marked_at)
			 VALUES (?, 'mcp_sidecar', 'legacy-sidecar', 'legacy reason', ?)`,
			workspaceID,
			now,
		); err != nil {
			t.Fatalf("insert v38 dead entity error = %v", err)
		}
		if _, err := prefixDB.ExecContext(
			ctx,
			`INSERT INTO loop_run_events (id, loop_run_id, workspace_id, seq, kind, payload_json, at)
			 VALUES ('event-v38', 'run-v38', ?, 1, 'legacy', '{}', ?)`,
			workspaceID,
			now,
		); err != nil {
			t.Fatalf("insert v38 event error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("prefixDB.Close() error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("openGlobalMigrationUpgrade() error = %v", err)
		}
		assertTablesPresent(
			t,
			upgraded.db,
			"loop_node_controls",
			"loop_node_attempts",
			"loop_node_waits",
			"loop_admission_claims",
			"loop_effect_outbox",
		)
		assertTableHasColumn(t, upgraded.db, "loop_runs", "cancel_requested")
		assertTableHasColumn(t, upgraded.db, "loop_runs", "cancel_kind")
		assertTableHasColumn(t, upgraded.db, "loop_run_events", "delivery_key")
		assertMigratedLoopLifecycleRows(t, upgraded.db, workspaceID)
		if _, err := upgraded.db.ExecContext(
			ctx,
			`INSERT INTO loop_generations (
				loop_run_id, generation, parent_generation, origin, created_at
			) VALUES ('run-v38', 2, 1, 'requeue', ?)`,
			now,
		); err != nil {
			t.Fatalf("insert requeue generation after upgrade error = %v", err)
		}
		if _, err := upgraded.db.ExecContext(
			ctx,
			`INSERT INTO dead_entities (workspace_id, kind, entity_id, reason, marked_at)
			 VALUES (?, 'loop_target', 'delivery:target', 'breaker open', ?)`,
			workspaceID,
			now,
		); err != nil {
			t.Fatalf("insert loop_target dead entity after upgrade error = %v", err)
		}
		if _, err := upgraded.db.ExecContext(
			ctx,
			`INSERT INTO dead_entities (workspace_id, kind, entity_id, reason, marked_at)
			 VALUES (?, 'invalid', 'bad-target', 'invalid', ?)`,
			workspaceID,
			now,
		); err == nil {
			t.Fatal("invalid dead entity kind accepted after upgrade")
		}
		status, err := store.Status(ctx, upgraded.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(upgraded) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("Close(upgraded) error = %v", err)
		}

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopened) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(reopened cleanup) error = %v", closeErr)
			}
		})
		assertMigratedLoopLifecycleRows(t, reopened.db, workspaceID)
		reopenedStatus, err := store.Status(ctx, reopened.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(reopened) error = %v", err)
		}
		assertCompleteMigrationStream(t, reopenedStatus, MigrationStream())
	})

	t.Run("Should scope non-null delivery keys while allowing legacy null keys", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC)
		for _, runID := range []string{"delivery-key-run-a", "delivery-key-run-b"} {
			if _, err := globalDB.CreateLoopRunForStart(
				ctx,
				testLoopRun(runID, now, looppkg.StatusRunning),
				dsl.ConcurrencyAllow,
			); err != nil {
				t.Fatalf("CreateLoopRunForStart(%s) error = %v", runID, err)
			}
		}
		insertEvent := func(id, runID string, seq int, deliveryKey any) error {
			_, execErr := globalDB.db.ExecContext(
				ctx,
				`INSERT INTO loop_run_events (
					id, loop_run_id, workspace_id, seq, kind, payload_json, at, delivery_key
				) VALUES (?, ?, 'ws-1', ?, 'test', '{}', ?, ?)`,
				id,
				runID,
				seq,
				now,
				deliveryKey,
			)
			return execErr
		}
		if err := insertEvent("event-null-1", "delivery-key-run-a", 1, nil); err != nil {
			t.Fatalf("insert first NULL delivery key error = %v", err)
		}
		if err := insertEvent("event-null-2", "delivery-key-run-a", 2, nil); err != nil {
			t.Fatalf("insert second NULL delivery key error = %v", err)
		}
		if err := insertEvent("event-key-a", "delivery-key-run-a", 3, "delivery-1"); err != nil {
			t.Fatalf("insert scoped delivery key error = %v", err)
		}
		if err := insertEvent("event-key-duplicate", "delivery-key-run-a", 4, "delivery-1"); err == nil {
			t.Fatal("duplicate delivery key accepted within one run")
		}
		if err := insertEvent("event-key-b", "delivery-key-run-b", 1, "delivery-1"); err != nil {
			t.Fatalf("insert same delivery key in another run error = %v", err)
		}
	})

	t.Run("Should cascade run-owned lifecycle rows and retain admission claims", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
		created, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("cascade-lifecycle", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		assertForeignKeysEnabled(t, globalDB.db)
		inserts := []struct {
			name  string
			query string
			args  []any
		}{
			{
				name:  "node control",
				query: `INSERT INTO loop_node_controls (loop_run_id, node_id, updated_at) VALUES (?, 'node', ?)`,
				args:  []any{created.ID, now},
			},
			{
				name: "node attempt",
				query: `INSERT INTO loop_node_attempts (
					loop_run_id, generation, node_id, attempt, disposition, started_at
				) VALUES (?, 1, 'node', 1, 'succeeded', ?)`,
				args: []any{created.ID, now},
			},
			{
				name: "node wait",
				query: `INSERT INTO loop_node_waits (
					loop_run_id, generation, node_id, kind, created_at
				) VALUES (?, 1, 'node', 'timer', ?)`,
				args: []any{created.ID, now},
			},
			{
				name: "effect outbox",
				query: `INSERT INTO loop_effect_outbox (
					loop_run_id, delivery_id, source_event_id, trigger, generation, entry_index, entry_json, created_at
				) VALUES (?, 'delivery', 'source-event', 'on_error', 1, 0, '{}', ?)`,
				args: []any{created.ID, now},
			},
			{
				name: "admission claim",
				query: `INSERT INTO loop_admission_claims (
					workspace_id, loop_name, source_key, event_key, loop_run_id, claimed_at
				) VALUES ('ws-1', 'delivery', 'source', 'event', ?, ?)`,
				args: []any{created.ID, now},
			},
		}
		for _, insert := range inserts {
			if _, err := globalDB.db.ExecContext(ctx, insert.query, insert.args...); err != nil {
				t.Fatalf("insert %s error = %v", insert.name, err)
			}
		}
		if _, err := globalDB.db.ExecContext(ctx, `DELETE FROM loop_runs WHERE id = ?`, created.ID); err != nil {
			t.Fatalf("delete loop run error = %v", err)
		}
		for _, table := range []string{
			"loop_node_controls",
			"loop_node_attempts",
			"loop_node_waits",
			"loop_effect_outbox",
		} {
			assertLoopLifecycleRowCount(t, globalDB.db, table, string(created.ID), 0)
		}
		assertLoopLifecycleRowCount(t, globalDB.db, "loop_admission_claims", string(created.ID), 1)
	})
}

func assertMigratedLoopLifecycleRows(t *testing.T, db *sql.DB, workspaceID string) {
	t.Helper()
	ctx := testutil.Context(t)
	var (
		status           string
		outputRef        sql.NullString
		taskRunID        sql.NullString
		resolvedRuntime  sql.NullString
		attempt          int
		nextAttemptAt    sql.NullTime
		firstScheduledAt sql.NullTime
		epoch            int
		goalStatus       sql.NullString
		goalTurnsUsed    sql.NullInt64
		goalTurnLimit    sql.NullInt64
	)
	if err := db.QueryRowContext(
		ctx,
		`SELECT status, output_ref, task_run_id, resolved_runtime_json, attempt,
		        next_attempt_at, first_scheduled_at, epoch, goal_status, goal_turns_used, goal_turn_limit
		 FROM loop_generation_outputs
		 WHERE loop_run_id = 'run-v38' AND generation = 1 AND node_id = 'goal' AND item_index = 0`,
	).Scan(
		&status,
		&outputRef,
		&taskRunID,
		&resolvedRuntime,
		&attempt,
		&nextAttemptAt,
		&firstScheduledAt,
		&epoch,
		&goalStatus,
		&goalTurnsUsed,
		&goalTurnLimit,
	); err != nil {
		t.Fatalf("query migrated generation output error = %v", err)
	}
	if status != "succeeded" || outputRef.String != "output-v38" || taskRunID.String != "task-v38" ||
		resolvedRuntime.String != `{"model":"pinned"}` || attempt != 1 || nextAttemptAt.Valid ||
		firstScheduledAt.Valid || epoch != 0 || goalStatus.String != "complete" ||
		goalTurnsUsed.Int64 != 4 || goalTurnLimit.Int64 != 9 {
		t.Fatalf("migrated generation output lost data or defaults")
	}
	var cancelRequested int
	var cancelKind string
	if err := db.QueryRowContext(
		ctx,
		`SELECT cancel_requested, cancel_kind FROM loop_runs WHERE id = 'run-v38'`,
	).Scan(&cancelRequested, &cancelKind); err != nil {
		t.Fatalf("query migrated cancel defaults error = %v", err)
	}
	if cancelRequested != 0 || cancelKind != "" {
		t.Fatalf("migrated cancel defaults = %d/%q, want 0/empty", cancelRequested, cancelKind)
	}
	var deliveryKey sql.NullString
	if err := db.QueryRowContext(
		ctx,
		`SELECT delivery_key FROM loop_run_events WHERE id = 'event-v38'`,
	).Scan(&deliveryKey); err != nil {
		t.Fatalf("query migrated legacy event error = %v", err)
	}
	if deliveryKey.Valid {
		t.Fatalf("legacy event delivery key = %q, want NULL", deliveryKey.String)
	}
	var deadCount int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM dead_entities
		 WHERE workspace_id = ? AND kind = 'mcp_sidecar' AND entity_id = 'legacy-sidecar'`,
		workspaceID,
	).Scan(&deadCount); err != nil {
		t.Fatalf("query migrated dead entity error = %v", err)
	}
	if deadCount != 1 {
		t.Fatalf("migrated dead entity count = %d, want 1", deadCount)
	}
}

func assertLoopLifecycleRowCount(t *testing.T, db *sql.DB, table, runID string, want int) {
	t.Helper()
	var got int
	query := "SELECT COUNT(*) FROM " + table + " WHERE loop_run_id = ?"
	if err := db.QueryRowContext(testutil.Context(t), query, runID).Scan(&got); err != nil {
		t.Fatalf("count %s rows error = %v", table, err)
	}
	if got != want {
		t.Fatalf("%s rows = %d, want %d", table, got, want)
	}
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
		"loop_gate_verdicts",
		"loop_generations",
		"loop_ui_annotations",
		"loop_config",
		"loop_node_controls",
		"loop_node_attempts",
		"loop_node_waits",
		"loop_admission_claims",
		"loop_effect_outbox",
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
		"best_generation",
		"best_score",
		"cancel_requested",
		"cancel_kind",
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
		"resolved_runtime_json",
		"attempt",
		"next_attempt_at",
		"first_scheduled_at",
		"epoch",
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
		"delivery_key",
	})
	assertTableColumns(t, globalDB.db, "loop_node_controls", []string{
		"loop_run_id", "node_id", "paused", "pause_actor_kind", "pause_actor_id", "pause_reason",
		"pause_rule_id", "pause_requested_at", "quarantined", "quarantine_entry_json", "quarantined_at",
		"attention_flag", "attention_reason", "cancel_state", "cancel_actor_kind", "cancel_actor_id",
		"cancel_reason", "cancel_requested_at", "last_evidence_at", "death_resume_streak", "revision", "updated_at",
	})
	assertTableColumns(t, globalDB.db, "loop_node_attempts", []string{
		"loop_run_id", "generation", "node_id", "item_index", "attempt", "failure_class", "failure_code",
		"cause", "hint", "target", "disposition", "started_at", "ended_at", "next_attempt_at",
	})
	assertTableColumns(t, globalDB.db, "loop_node_waits", []string{
		"loop_run_id", "generation", "node_id", "item_index", "kind", "resume_at", "next_escalation_at",
		"escalation_cursor", "claim_state", "claimed_by_kind", "claimed_by_id", "claimed_at", "admission_failures",
		"expect_json", "ahead_payload_json", "issued_epoch", "created_at",
	})
	assertTableColumns(t, globalDB.db, "loop_admission_claims", []string{
		"workspace_id", "loop_name", "source_key", "event_key", "loop_run_id", "claimed_at",
		"suppressed_count", "last_suppressed_at",
	})
	assertTableColumns(t, globalDB.db, "loop_effect_outbox", []string{
		"loop_run_id", "delivery_id", "source_event_id", "trigger", "generation", "node_id", "item_index",
		"entry_index", "entry_json", "state", "attempts", "created_at", "delivered_at",
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
	assertTableColumns(t, globalDB.db, "loop_gate_verdicts", []string{
		"loop_run_id",
		"generation",
		"gate_id",
		"item_index",
		"outcome",
		"score",
		"route_cause_rank",
		"blocking_issues_json",
		"criteria_json",
		"decided_at",
	})
	assertTableColumns(t, globalDB.db, "loop_generations", []string{
		"loop_run_id",
		"generation",
		"parent_generation",
		"origin",
		"created_at",
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
		"runtime_defaults_json",
		"runtime_rules_json",
	})
	assertTableHasColumn(t, globalDB.db, "task_runs", "run_kind")
	assertTableHasColumn(t, globalDB.db, "task_runs", "loop_run_id")
	assertTableHasColumn(t, globalDB.db, "task_runs", "tokens_used")
	assertIndexesPresent(t, globalDB.db, "loop_run_events", "idx_loop_run_events_run_seq")
	assertIndexesPresent(t, globalDB.db, "loop_run_events", "uq_loop_run_events_delivery")
	assertIndexesPresent(
		t,
		globalDB.db,
		"loop_node_controls",
		"idx_loop_node_controls_quarantined",
		"idx_loop_node_controls_attention",
	)
	assertIndexesPresent(
		t,
		globalDB.db,
		"loop_node_waits",
		"idx_loop_node_waits_due",
		"idx_loop_node_waits_ladder",
		"idx_loop_node_waits_state",
	)
	assertIndexesPresent(t, globalDB.db, "loop_effect_outbox", "idx_loop_effect_outbox_pending")
	assertIndexSQLContains(t, globalDB.db, "idx_loop_run_events_run_seq", "ON loop_run_events(loop_run_id, seq)")
	assertIndexSQLContains(t, globalDB.db, "uq_loop_run_events_delivery", "WHERE delivery_key IS NOT NULL")
	assertIndexesPresent(t, globalDB.db, "loop_gate_decisions", "idx_loop_gate_decisions_workspace_run")
	assertIndexesPresent(t, globalDB.db, "loop_gate_verdicts", "idx_loop_gate_verdicts_route_cause")
	assertIndexSQLContains(
		t,
		globalDB.db,
		"idx_loop_gate_verdicts_route_cause",
		"ON loop_gate_verdicts(loop_run_id, generation, route_cause_rank)",
	)
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
		"ON loop_runs(workspace_id, loop_name, status, created_at ASC, id ASC)",
	)
	assertIndexesPresent(t, globalDB.db, "loop_runs", "idx_loop_runs_catalog")
	assertIndexSQLContains(
		t,
		globalDB.db,
		"idx_loop_runs_catalog",
		"ON loop_runs(workspace_id, loop_name, created_at DESC, id DESC, status)",
	)
	assertIndexesPresent(t, globalDB.db, "task_runs", "uq_task_runs_active_loop_coordinator")
	assertIndexesPresent(
		t,
		globalDB.db,
		"loop_generation_outputs",
		"idx_loop_generation_outputs_output_ref",
		"idx_loop_generation_outputs_retry_due",
	)
	assertIndexSQLContains(
		t,
		globalDB.db,
		"uq_task_runs_active_loop_coordinator",
		"ON task_runs(loop_run_id)",
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
		"idx_loop_generation_outputs_retry_due",
		"WHERE status = 'retrying' AND next_attempt_at IS NOT NULL",
	)
	assertIndexSQLContains(
		t,
		globalDB.db,
		"uq_task_runs_active_loop_coordinator",
		"status IN ('queued', 'claimed', 'starting', 'running')",
	)
	assertTableSQLContains(t, globalDB.db, "loop_generation_outputs", "REFERENCES loop_runs(id) ON DELETE CASCADE")
	assertTableSQLContains(t, globalDB.db, "loop_generation_outputs", "'retrying'")
	assertTableSQLContains(t, globalDB.db, "loop_generation_outputs", "'quarantined'")
	assertTableSQLContains(t, globalDB.db, "loop_generation_outputs", "CHECK (attempt >= 1)")
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
	assertTableSQLContains(t, globalDB.db, "loop_gate_verdicts", "REFERENCES loop_runs(id) ON DELETE CASCADE")
	assertTableSQLContains(
		t,
		globalDB.db,
		"loop_gate_verdicts",
		"PRIMARY KEY (loop_run_id, generation, gate_id, item_index)",
	)
	assertTableSQLContains(t, globalDB.db, "loop_generations", "REFERENCES loop_runs(id) ON DELETE CASCADE")
	assertTableSQLContains(t, globalDB.db, "loop_generations", "PRIMARY KEY (loop_run_id, generation)")
	assertTableSQLContains(t, globalDB.db, "loop_generations", "'requeue'")
	assertTableSQLContains(
		t,
		globalDB.db,
		"loop_generations",
		"parent_generation >= 0 AND parent_generation < generation",
	)
	assertTableSQLContains(t, globalDB.db, "loop_runs", "best_generation IS NULL AND best_score IS NULL")
	assertTableSQLContains(t, globalDB.db, "loop_runs", "cancel_kind IN ('', 'cancel', 'kill')")
	assertTableSQLContains(t, globalDB.db, "dead_entities", "'loop_target'")
	for _, table := range []string{"loop_node_controls", "loop_node_attempts", "loop_node_waits", "loop_effect_outbox"} {
		assertTableSQLContains(t, globalDB.db, table, "REFERENCES loop_runs(id) ON DELETE CASCADE")
	}
	assertTableSQLContains(t, globalDB.db, "loop_ui_annotations", "PRIMARY KEY (workspace_id, loop_name, node_id)")
	assertTableSQLContains(t, globalDB.db, "loop_config", "PRIMARY KEY (workspace_id, loop_name)")
	assertGoalDurableStateSchema(t, globalDB)
}
