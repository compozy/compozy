package globaldb

import (
	"errors"
	"testing"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBGoalBudgetGuardShouldMapExceededPolicyAndGrantScope(t *testing.T) {
	tests := []struct {
		name            string
		onExceeded      dsl.BudgetExceeded
		grant           *goal.ControlGrant
		wantAllowed     bool
		wantDisposition looppkg.ActionDisposition
		wantGrantID     int64
	}{
		{
			name:            "halt",
			onExceeded:      dsl.BudgetExceededHalt,
			wantDisposition: looppkg.ActionDispositionExhausted,
		},
		{
			name:            "escalate",
			onExceeded:      dsl.BudgetExceededEscalate,
			wantDisposition: looppkg.ActionDispositionNeedsApproval,
		},
		{
			name:       "work and settle grant",
			onExceeded: dsl.BudgetExceededEscalate,
			grant: &goal.ControlGrant{
				ID:       7,
				Kind:     goal.ControlGrantBudget,
				Cause:    looppkg.ReasonCodeGoalBudgetFenced,
				Turn:     1,
				Scope:    goal.ControlGrantScopeWorkAndSettle,
				Consumed: false,
			},
			wantAllowed: true,
			wantGrantID: 7,
		},
	}
	for _, tc := range tests {
		t.Run("Should enforce "+tc.name, func(t *testing.T) {
			t.Parallel()

			globalDB, key, taskRunID, now := seedGoalBudgetGuardTest(t, tc.onExceeded, 5, tc.grant)
			snapshot := goalBudgetSnapshotForTest(t, globalDB, goal.BudgetBoundarySnapshot{
				Key:                 key,
				TaskRunID:           taskRunID,
				Boundary:            goal.BudgetBeforeWork,
				Phase:               "preparing",
				Turn:                1,
				OperationID:         "goal-prompt:1",
				OperationBaseTokens: 0,
				LiveTokensUsed:      5,
				TokensReported:      true,
			})
			decision, err := globalDB.FlushAndCheck(testutil.Context(t), snapshot)
			if err != nil {
				t.Fatalf("FlushAndCheck() error = %v", err)
			}
			if decision.Allowed != tc.wantAllowed || decision.Disposition != tc.wantDisposition ||
				decision.GrantID != tc.wantGrantID {
				t.Fatalf("decision = %#v", decision)
			}
			if decision.BudgetVersion != 1 || !decision.ValidUntil.After(now) {
				t.Fatalf("decision version/expiry = %d/%v", decision.BudgetVersion, decision.ValidUntil)
			}
			if !tc.wantAllowed && decision.Cause != looppkg.ReasonCodeGoalBudgetFenced {
				t.Fatalf("decision cause = %q", decision.Cause)
			}
		})
	}
}

func TestGlobalDBGoalBudgetGuardShouldMaxFlushCumulativeUsageWithoutHeartbeat(t *testing.T) {
	t.Run("Should preserve the highest reported task total and unknown token truth", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, _ := seedGoalBudgetGuardTest(
			t,
			dsl.BudgetExceededHalt,
			100,
			nil,
		)
		ctx := testutil.Context(t)
		checks := []struct {
			live     int64
			reported bool
			want     int64
		}{
			{live: 10, reported: true, want: 10},
			{live: 7, reported: true, want: 10},
			{live: 999, reported: false, want: 10},
		}
		for index, check := range checks {
			snapshot := goalBudgetSnapshotForTest(t, globalDB, goal.BudgetBoundarySnapshot{
				Key:                 key,
				TaskRunID:           taskRunID,
				Boundary:            goal.BudgetAfterWork,
				Phase:               "prompting",
				Turn:                1,
				OperationID:         "goal-prompt:1",
				OperationBaseTokens: 0,
				LiveTokensUsed:      check.live,
				TokensReported:      check.reported,
			})
			decision, err := globalDB.FlushAndCheck(ctx, snapshot)
			if err != nil {
				t.Fatalf("FlushAndCheck(%d) error = %v", index, err)
			}
			if !decision.Allowed || decision.BudgetVersion != int64(index+1) {
				t.Fatalf("decision[%d] = %#v", index, decision)
			}
			var taskTokens, loopTokens int64
			if err := globalDB.db.QueryRowContext(
				ctx,
				`SELECT tokens_used FROM task_runs WHERE id = ?`,
				taskRunID,
			).Scan(&taskTokens); err != nil {
				t.Fatalf("query task tokens error = %v", err)
			}
			if err := globalDB.db.QueryRowContext(
				ctx,
				`SELECT tokens_used FROM loop_runs WHERE id = ?`,
				string(key.LoopRunID),
			).Scan(&loopTokens); err != nil {
				t.Fatalf("query Loop tokens error = %v", err)
			}
			if taskTokens != check.want || loopTokens != check.want {
				t.Fatalf("tokens[%d] = task:%d loop:%d, want %d", index, taskTokens, loopTokens, check.want)
			}
		}
	})
}

func TestGlobalDBGoalBudgetGuardShouldRejectStaleCheckpointOwner(t *testing.T) {
	t.Run("Should leave usage and budget version unchanged after the segment loses ownership", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, _ := seedGoalBudgetGuardTest(t, dsl.BudgetExceededHalt, 100, nil)
		ctx := testutil.Context(t)
		snapshot := goalBudgetSnapshotForTest(t, globalDB, goal.BudgetBoundarySnapshot{
			Key:                 key,
			TaskRunID:           taskRunID,
			Boundary:            goal.BudgetAfterWork,
			Phase:               "stale-segment",
			Turn:                1,
			OperationID:         "goal-stale-segment",
			OperationBaseTokens: 0,
			LiveTokensUsed:      25,
			TokensReported:      true,
		})
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_goal_checkpoints SET phase = 'awaiting_control', goal_status = 'paused'
			 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?`,
			string(key.LoopRunID),
			key.Generation,
			string(key.NodeID),
			key.ItemIndex,
		); err != nil {
			t.Fatalf("move checkpoint owner error = %v", err)
		}
		if _, err := globalDB.FlushAndCheck(ctx, snapshot); err == nil {
			t.Fatal("FlushAndCheck(stale owner) error = nil")
		} else {
			var reason *looppkg.ReasonError
			if !errors.As(err, &reason) || reason.Code != looppkg.ReasonCodeGoalControlStale {
				t.Fatalf("FlushAndCheck(stale owner) error = %v, want goal_control_stale", err)
			}
		}
		var taskTokens, loopTokens, budgetVersion int64
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT task_runs.tokens_used, loop_runs.tokens_used, loop_runs.budget_version
			 FROM task_runs JOIN loop_runs ON loop_runs.id = task_runs.loop_run_id
			 WHERE task_runs.id = ?`,
			taskRunID,
		).Scan(&taskTokens, &loopTokens, &budgetVersion); err != nil {
			t.Fatalf("read stale-owner accounting error = %v", err)
		}
		if taskTokens != 0 || loopTokens != 0 || budgetVersion != 0 {
			t.Fatalf(
				"stale-owner accounting = task:%d loop:%d version:%d, want all zero",
				taskTokens,
				loopTokens,
				budgetVersion,
			)
		}
	})
}

func TestGlobalDBGoalBudgetGuardShouldFenceWallClockAcrossQueuedAndMidTurnBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		boundary goal.BudgetBoundary
		phase    string
	}{
		{name: "queued pre-submit", boundary: goal.BudgetBeforeWork, phase: "ready-slot"},
		{name: "mid-turn settlement", boundary: goal.BudgetAfterWork, phase: "prompting"},
	}
	for _, tc := range tests {
		t.Run("Should fence "+tc.name+" after the wall deadline", func(t *testing.T) {
			t.Parallel()

			globalDB, key, taskRunID, _ := seedGoalBudgetGuardTest(
				t,
				dsl.BudgetExceededHalt,
				0,
				nil,
			)
			ctx := testutil.Context(t)
			databaseNow, err := goalBudgetDatabaseNow(ctx, globalDB.db)
			if err != nil {
				t.Fatalf("goalBudgetDatabaseNow() error = %v", err)
			}
			if _, err := globalDB.db.ExecContext(
				ctx,
				`UPDATE loop_runs SET budget_wall_sec = 1, started_at = ? WHERE id = ?`,
				store.FormatTimestamp(databaseNow.Add(-2*time.Second)),
				string(key.LoopRunID),
			); err != nil {
				t.Fatalf("configure Goal wall budget error = %v", err)
			}
			snapshot := goalBudgetSnapshotForTest(t, globalDB, goal.BudgetBoundarySnapshot{
				Key:                 key,
				TaskRunID:           taskRunID,
				Boundary:            tc.boundary,
				Phase:               tc.phase,
				Turn:                1,
				OperationID:         "goal-wall-boundary",
				OperationBaseTokens: 0,
				LiveTokensUsed:      0,
				TokensReported:      false,
			})
			decision, err := globalDB.FlushAndCheck(ctx, snapshot)
			if err != nil {
				t.Fatalf("FlushAndCheck() error = %v", err)
			}
			if decision.Allowed || decision.Disposition != looppkg.ActionDispositionExhausted ||
				decision.Cause != looppkg.ReasonCodeGoalBudgetFenced {
				t.Fatalf("wall-clock decision = %#v", decision)
			}
		})
	}
}

func TestGlobalDBGoalBudgetGuardShouldIssueAndValidateWithDatabaseTime(t *testing.T) {
	t.Run("Should ignore an injected process clock when authorizing the next effect", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, _ := seedGoalBudgetGuardTest(t, dsl.BudgetExceededHalt, 100, nil)
		globalDB.now = func() time.Time {
			return time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		ctx := testutil.Context(t)
		databaseNow, err := goalBudgetDatabaseNow(ctx, globalDB.db)
		if err != nil {
			t.Fatalf("goalBudgetDatabaseNow() error = %v", err)
		}
		decision, err := globalDB.FlushAndCheck(ctx, goalBudgetSnapshotForTest(t, globalDB, goal.BudgetBoundarySnapshot{
			Key:                 key,
			TaskRunID:           taskRunID,
			Boundary:            goal.BudgetBeforeWork,
			Phase:               "ready-slot",
			Turn:                1,
			OperationID:         "goal-database-clock",
			OperationBaseTokens: 0,
			LiveTokensUsed:      0,
			TokensReported:      false,
		}))
		if err != nil {
			t.Fatalf("FlushAndCheck() error = %v", err)
		}
		minimumValidity := databaseNow.Add(500 * time.Millisecond)
		if !decision.ValidUntil.After(databaseNow) || decision.ValidUntil.Before(minimumValidity) {
			t.Fatalf(
				"decision.ValidUntil = %v, want database-time authorization window after %v",
				decision.ValidUntil,
				databaseNow,
			)
		}
	})
}

func goalBudgetSnapshotForTest(
	t *testing.T,
	globalDB *GlobalDB,
	snapshot goal.BudgetBoundarySnapshot,
) goal.BudgetBoundarySnapshot {
	t.Helper()

	checkpoint, err := globalDB.LoadCheckpoint(testutil.Context(t), snapshot.Key)
	if err != nil {
		t.Fatalf("LoadCheckpoint(budget owner) error = %v", err)
	}
	snapshot.ExpectedControlEpoch = checkpoint.ControlEpoch
	snapshot.ExpectedBindingEpoch = checkpoint.BindingEpoch
	snapshot.ExpectedPhase = checkpoint.Phase
	snapshot.ExpectedQueueEntryID = checkpoint.QueueEntryID
	snapshot.ExpectedPromptID = checkpoint.PromptID
	return snapshot
}

func seedGoalBudgetGuardTest(
	t *testing.T,
	onExceeded dsl.BudgetExceeded,
	budgetTokens int,
	grant *goal.ControlGrant,
) (*GlobalDB, goal.TurnKey, string, time.Time) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t)
	ctx := testutil.Context(t)
	now, err := goalBudgetDatabaseNow(ctx, globalDB.db)
	if err != nil {
		t.Fatalf("goalBudgetDatabaseNow() error = %v", err)
	}
	globalDB.now = func() time.Time { return now }
	seed := testLoopRun("loop-goal-budget", now, looppkg.StatusRunning)
	seed.BudgetTokens = budgetTokens
	seed.BudgetOnExceeded = onExceeded
	loopRun, err := globalDB.CreateLoopRunForStart(ctx, seed, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	createCompletedLoopWorkerRunForTest(ctx, t, globalDB, loopRun.ID, "goal-budget", 0, now)
	taskRunID := "run-goal-budget"
	key := goal.TurnKey{
		WorkspaceID: loopRun.WorkspaceID,
		LoopRunID:   loopRun.ID,
		Generation:  1,
		NodeID:      "converge",
	}
	if _, err := globalDB.CreateCheckpoint(ctx, goal.CreateCheckpointRequest{Checkpoint: goal.Checkpoint{
		Key:               key,
		ControlEpoch:      1,
		Phase:             "idle",
		Status:            "active",
		TurnLimit:         3,
		TaskRunID:         taskRunID,
		ContextState:      "unknown",
		ControlGrant:      grant,
		ContextNudgeRatio: 0.8,
		UpdatedAt:         now,
	}}); err != nil {
		t.Fatalf("CreateCheckpoint() error = %v", err)
	}
	return globalDB, key, taskRunID, now
}
