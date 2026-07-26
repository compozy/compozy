//go:build integration

package globaldb

import (
	"testing"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/testutil"
)

func TestGoalJudgeAttemptLifecycleIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should persist before evaluator effect and complete once with reported zero tokens", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-judge")
		insertGoalSchemaLoopRun(t, globalDB, "run-judge", "ws-goal-judge", "catalog", nil)
		now := time.Date(2026, 7, 10, 18, 0, 0, 0, time.UTC)
		key := goal.TurnKey{
			WorkspaceID: "ws-goal-judge",
			LoopRunID:   "run-judge",
			Generation:  1,
			NodeID:      "converge",
		}
		seedActiveGoalBindingForTest(
			t,
			globalDB,
			"run-judge",
			"ws-goal-judge",
			"goal:judge",
			3,
			"session-judge",
			now,
		)
		if _, err := globalDB.CreateCheckpoint(testutil.Context(t), goal.CreateCheckpointRequest{
			Checkpoint: goal.Checkpoint{
				Key:               key,
				ControlEpoch:      2,
				Phase:             "judging",
				Status:            "active",
				TurnsUsed:         1,
				TurnLimit:         10,
				TaskRunID:         "task-run-judge",
				QueueEntryID:      "queue-judge",
				PromptID:          "prompt-judge",
				PromptKind:        "work",
				SessionID:         "session-judge",
				BindingHandle:     "goal:judge",
				BindingEpoch:      3,
				ContextState:      "unknown",
				ContextNudgeRatio: 0.8,
				UpdatedAt:         now,
			},
		}); err != nil {
			t.Fatalf("CreateCheckpoint(judge) error = %v", err)
		}
		begin := goal.BeginJudgeAttemptRequest{
			AttemptID:            "judge-attempt-1",
			Key:                  key,
			ExpectedControlEpoch: 2,
			ExpectedBindingEpoch: 3,
			TaskRunID:            "task-run-judge",
			PromptID:             "prompt-judge",
			Turn:                 1,
			JudgeDigest:          "judge-digest-v1",
			UsageBaseTokens:      17,
			BudgetDecision: goal.BudgetDecision{
				Allowed:       true,
				BudgetVersion: 0,
				ValidUntil:    time.Now().UTC().Add(time.Minute),
			},
		}
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`UPDATE loop_session_bindings SET state = 'closed', closed_at = ?
			 WHERE loop_run_id = 'run-judge' AND handle = 'goal:judge' AND binding_epoch = 3`,
			now.Add(time.Second).Format(time.RFC3339Nano),
		); err != nil {
			t.Fatalf("close judge binding before begin error = %v", err)
		}
		if _, err := globalDB.BeginJudgeAttempt(testutil.Context(t), begin); err == nil {
			t.Fatal("BeginJudgeAttempt(closed binding) error = nil")
		}
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`UPDATE loop_session_bindings SET state = 'active', closed_at = NULL
			 WHERE loop_run_id = 'run-judge' AND handle = 'goal:judge' AND binding_epoch = 3`,
		); err != nil {
			t.Fatalf("reactivate judge binding before begin error = %v", err)
		}
		started, err := globalDB.BeginJudgeAttempt(testutil.Context(t), begin)
		if err != nil {
			t.Fatalf("BeginJudgeAttempt(first) error = %v", err)
		}
		repeated, err := globalDB.BeginJudgeAttempt(testutil.Context(t), begin)
		if err != nil {
			t.Fatalf("BeginJudgeAttempt(second) error = %v", err)
		}
		if started.AttemptID != repeated.AttemptID || started.Status != "running" ||
			started.UsageBaseTokens != 17 {
			t.Fatalf("judge starts = %#v/%#v, want same running attempt", started, repeated)
		}

		verdict := gate.Verdict{
			Outcome: gate.VerdictOutcomeRejected,
			BlockingIssues: []gate.BlockingIssue{
				{ID: "verify", Note: "verification still fails"},
			},
		}
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`UPDATE loop_session_bindings SET state = 'closed', closed_at = ?
			 WHERE loop_run_id = 'run-judge' AND handle = 'goal:judge' AND binding_epoch = 3`,
			now.Add(2*time.Second).Format(time.RFC3339Nano),
		); err != nil {
			t.Fatalf("close judge binding before completion error = %v", err)
		}
		if _, err := globalDB.CompleteJudgeAttempt(
			testutil.Context(t),
			goal.CompleteJudgeAttemptRequest{
				AttemptID:            "judge-attempt-1",
				Key:                  key,
				ExpectedControlEpoch: 2,
				ExpectedBindingEpoch: 3,
				TaskRunID:            "task-run-judge",
				PromptID:             "prompt-judge",
				Verdict:              verdict,
				TokensReported:       true,
			},
		); err == nil {
			t.Fatal("CompleteJudgeAttempt(closed binding) error = nil")
		}
		if _, err := globalDB.db.ExecContext(
			testutil.Context(t),
			`UPDATE loop_session_bindings SET state = 'active', closed_at = NULL
			 WHERE loop_run_id = 'run-judge' AND handle = 'goal:judge' AND binding_epoch = 3`,
		); err != nil {
			t.Fatalf("reactivate judge binding before completion error = %v", err)
		}
		completed, err := globalDB.CompleteJudgeAttempt(
			testutil.Context(t),
			goal.CompleteJudgeAttemptRequest{
				AttemptID:            "judge-attempt-1",
				Key:                  key,
				ExpectedControlEpoch: 2,
				ExpectedBindingEpoch: 3,
				TaskRunID:            "task-run-judge",
				PromptID:             "prompt-judge",
				Verdict:              verdict,
				TokensUsed:           0,
				TokensReported:       true,
			},
		)
		if err != nil {
			t.Fatalf("CompleteJudgeAttempt() error = %v", err)
		}
		if completed.Status != "completed" || completed.Outcome != string(gate.VerdictOutcomeRejected) ||
			!completed.TokensReported || completed.TokensUsed != 0 || len(completed.BlockingIssues) != 1 {
			t.Fatalf("completed judge = %#v, want rejected with reported zero", completed)
		}
		loaded, err := globalDB.GetJudgeAttempt(testutil.Context(t), key, "judge-attempt-1")
		if err != nil {
			t.Fatalf("GetJudgeAttempt() error = %v", err)
		}
		if loaded.CompletedAt == nil || !loaded.TokensReported || loaded.TokensUsed != 0 {
			t.Fatalf("loaded judge = %#v, want durable terminal token truth", loaded)
		}

		stale := goal.CompleteJudgeAttemptRequest{
			AttemptID:            "judge-attempt-1",
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 3,
			TaskRunID:            "task-run-judge",
			PromptID:             "prompt-judge",
			Verdict:              verdict,
		}
		if _, err := globalDB.CompleteJudgeAttempt(testutil.Context(t), stale); err == nil {
			t.Fatal("CompleteJudgeAttempt(stale control) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalControlStale)
		}
	})
}

func TestGoalJudgeAmbiguityIntegration(t *testing.T) {
	t.Run("Should terminalize one running attempt and its open turn without evaluator replay", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-judge-ambiguous")
		insertGoalSchemaLoopRun(
			t,
			globalDB,
			"run-judge-ambiguous",
			"ws-goal-judge-ambiguous",
			"catalog",
			nil,
		)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 10, 18, 30, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now.Add(time.Second) }
		key := goal.TurnKey{
			WorkspaceID: "ws-goal-judge-ambiguous",
			LoopRunID:   "run-judge-ambiguous",
			Generation:  1,
			NodeID:      "converge",
		}
		seedActiveGoalBindingForTest(
			t,
			globalDB,
			"run-judge-ambiguous",
			"ws-goal-judge-ambiguous",
			"goal:judge-ambiguous",
			3,
			"session-judge-ambiguous",
			now,
		)
		if _, err := globalDB.CreateCheckpoint(ctx, goal.CreateCheckpointRequest{
			Checkpoint: goal.Checkpoint{
				Key:               key,
				ControlEpoch:      2,
				Phase:             "judging",
				Status:            "active",
				TurnsUsed:         1,
				TurnLimit:         10,
				TaskRunID:         "task-run-judge-ambiguous",
				QueueEntryID:      "queue-judge-ambiguous",
				PromptID:          "prompt-judge-ambiguous",
				PromptKind:        "work",
				SessionID:         "session-judge-ambiguous",
				BindingHandle:     "goal:judge-ambiguous",
				BindingEpoch:      3,
				JudgeAttemptID:    "judge-attempt-ambiguous",
				ContextState:      "unknown",
				ContextNudgeRatio: 0.8,
				UpdatedAt:         now,
			},
		}); err != nil {
			t.Fatalf("CreateCheckpoint(judge ambiguous) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_goal_turns (
				loop_run_id, seq, generation, node_id, item_index, turn,
				session_id, binding_handle, binding_epoch, prompt_id, prompt_attempt,
				usage_base_tokens, actor_kind, actor_id, started_at
			) VALUES (?, 1, 1, 'converge', 0, 1, ?, ?, 3, ?, 0, 0, 'daemon', 'loop-action', ?)`,
			string(key.LoopRunID),
			"session-judge-ambiguous",
			"goal:judge-ambiguous",
			"prompt-judge-ambiguous",
			now.Format(time.RFC3339Nano),
		); err != nil {
			t.Fatalf("insert open Goal turn error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_goal_judge_attempts (
				attempt_id, loop_run_id, generation, node_id, item_index, turn,
				judge_digest, status, blocking_json, usage_base_tokens, started_at
			) VALUES (?, ?, 1, 'converge', 0, 1, 'judge-digest-v1', 'running', '[]', 0, ?)`,
			"judge-attempt-ambiguous",
			string(key.LoopRunID),
			now.Format(time.RFC3339Nano),
		); err != nil {
			t.Fatalf("insert running Goal judge error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status,
				goal_status, goal_turns_used, goal_turn_limit
			) VALUES (?, 1, 'converge', 0, 'running', 'active', 1, 10)`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("insert Goal judge projection error = %v", err)
		}

		request := goal.MarkJudgeAmbiguousRequest{
			AttemptID:            "judge-attempt-ambiguous",
			Key:                  key,
			ExpectedControlEpoch: 2,
			ExpectedBindingEpoch: 3,
			TaskRunID:            "task-run-judge-ambiguous",
			PromptID:             "prompt-judge-ambiguous",
			Cause:                string(looppkg.ReasonCodeGoalRecoveryAmbiguous),
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_session_bindings SET state = 'closed', closed_at = ?
			 WHERE loop_run_id = 'run-judge-ambiguous'
			   AND handle = 'goal:judge-ambiguous' AND binding_epoch = 3`,
			now.Add(time.Second).Format(time.RFC3339Nano),
		); err != nil {
			t.Fatalf("close ambiguous judge binding error = %v", err)
		}
		if err := globalDB.MarkJudgeAmbiguous(ctx, request); err == nil {
			t.Fatal("MarkJudgeAmbiguous(closed binding) error = nil")
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_session_bindings SET state = 'active', closed_at = NULL
			 WHERE loop_run_id = 'run-judge-ambiguous'
			   AND handle = 'goal:judge-ambiguous' AND binding_epoch = 3`,
		); err != nil {
			t.Fatalf("reactivate ambiguous judge binding error = %v", err)
		}
		err := globalDB.MarkJudgeAmbiguous(ctx, request)
		if err != nil {
			t.Fatalf("MarkJudgeAmbiguous() error = %v", err)
		}
		if err := globalDB.MarkJudgeAmbiguous(ctx, request); err != nil {
			t.Fatalf("MarkJudgeAmbiguous(idempotent retry) error = %v", err)
		}
		attempt, err := globalDB.GetJudgeAttempt(ctx, key, "judge-attempt-ambiguous")
		if err != nil {
			t.Fatalf("GetJudgeAttempt() error = %v", err)
		}
		if attempt.Status != "ambiguous" || attempt.CompletedAt == nil || attempt.Outcome != "" {
			t.Fatalf("ambiguous attempt = %#v", attempt)
		}
		checkpoint, err := globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint() error = %v", err)
		}
		if checkpoint.Phase != "awaiting_control" || checkpoint.Status != "paused" ||
			checkpoint.ControlCause != looppkg.ReasonCodeGoalRecoveryAmbiguous {
			t.Fatalf("ambiguous judge checkpoint = %#v", checkpoint)
		}
		assertGoalRuntimeTurn(t, globalDB, key, "prompt-judge-ambiguous", "ambiguous", "")
		var projectedStatus string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT goal_status FROM loop_generation_outputs
			 WHERE loop_run_id = ? AND generation = 1 AND node_id = 'converge' AND item_index = 0`,
			string(key.LoopRunID),
		).Scan(&projectedStatus); err != nil {
			t.Fatalf("load Goal judge projection error = %v", err)
		}
		if projectedStatus != "paused" {
			t.Fatalf("Goal judge projection = %q, want paused", projectedStatus)
		}
		assertGoalRuntimeEventCount(t, globalDB, key.LoopRunID, loopRunEventGoalTurnCompleted, 1)
		assertGoalRuntimeEventCount(t, globalDB, key.LoopRunID, loopRunEventGoalStatusChanged, 1)
	})
}
