package globaldb

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBReactivateGoalRunShouldEnqueueOneEpochScopedSuccessor(t *testing.T) {
	t.Run("Should make concurrent plain resumes idempotent in one transaction", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t)
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 10, 20, 0, 0, 0, time.UTC)
		loopRun, err := globalDB.CreateLoopRunForStart(
			ctx,
			testLoopRun("looprun-goal-reentry", now, looppkg.StatusRunning),
			dsl.ConcurrencyAllow,
		)
		if err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		taskRecord := taskRecordForTest(looppkg.GoalNodeTaskID(loopRun.ID, 1, "converge", 0))
		taskRecord.Status = taskpkg.TaskStatusReady
		if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		initialRunID := looppkg.GoalSegmentRunID(loopRun.ID, 1, "converge", 0, 1)
		metadata := json.RawMessage(
			`{"generation":1,"node_id":"converge","item_index":0,"goal_segment_epoch":1}`,
		)
		reservation := queuedRunReservationForTest(
			taskRecord.ID,
			initialRunID,
			looppkg.GoalSegmentIdempotencyKey(loopRun.ID, 1, "converge", 0, 1),
			taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "loop"},
			metadata,
			now,
		)
		reservation.RunKind = taskpkg.RunKindWorker
		reservation.LoopRunID = string(loopRun.ID)
		if _, _, _, err := globalDB.ReserveQueuedRun(ctx, reservation); err != nil {
			t.Fatalf("ReserveQueuedRun(initial Goal segment) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status, task_run_id,
				goal_status, goal_turns_used, goal_turn_limit
			) VALUES (?, 1, 'converge', 0, 'enqueued', ?, 'active', 2, 10)`,
			string(loopRun.ID),
			initialRunID,
		); err != nil {
			t.Fatalf("insert Goal generation output error = %v", err)
		}
		if err := globalDB.RegisterSession(ctx, SessionInfo{
			ID:          "session-goal-reentry",
			AgentName:   "codex",
			WorkspaceID: string(loopRun.WorkspaceID),
			State:       "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_session_bindings (
				loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
				creation_profile_ref, policy_spec_digest, creation_digest, ownership, state,
				created_at, activated_at
			) VALUES (?, 'goal:reentry', 1, 'binding-reentry-1', 'session-goal-reentry', ?,
				'profile-reentry', 'policy-reentry', 'creation-reentry', 'run-owned', 'active', ?, ?)`,
			string(loopRun.ID),
			string(loopRun.WorkspaceID),
			store.FormatTimestamp(now),
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatalf("insert active Goal reentry binding error = %v", err)
		}
		key := goalpkg.TurnKey{
			WorkspaceID: loopRun.WorkspaceID,
			LoopRunID:   loopRun.ID,
			Generation:  1,
			NodeID:      "converge",
		}
		if _, err := globalDB.CreateCheckpoint(ctx, goalpkg.CreateCheckpointRequest{
			Checkpoint: goalpkg.Checkpoint{
				Key:               key,
				ControlEpoch:      1,
				Phase:             "idle",
				Status:            "active",
				TurnsUsed:         2,
				TurnLimit:         10,
				TaskRunID:         initialRunID,
				SessionID:         "session-goal-reentry",
				BindingHandle:     "goal:reentry",
				BindingEpoch:      1,
				ContextState:      "unknown",
				ContextNudgeRatio: 0.8,
				UpdatedAt:         now,
			},
		}); err != nil {
			t.Fatalf("CreateCheckpoint() error = %v", err)
		}
		ticket, err := globalDB.PrepareGoalPrompt(ctx, goalpkg.PreparePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            initialRunID,
			QueueEntryID:         "queue-goal-reentry",
			PromptID:             "prompt-goal-reentry",
			PromptKind:           "work",
			SessionID:            "session-goal-reentry",
			BindingHandle:        "goal:reentry",
			BindingEpoch:         1,
			Message:              "Continue after a safe pause",
			PreparedAt:           now,
		})
		if err != nil {
			t.Fatalf("PrepareGoalPrompt() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE session_input_queue
			 SET fence_kind = 'control-fenced', fence_disposition = 'paused',
			     fence_reason_code = 'operator_resume', fenced_at = ?
			 WHERE id = ?`,
			now.Format(time.RFC3339Nano),
			ticket.QueueEntryID,
		); err != nil {
			t.Fatalf("fence prepared Goal prompt error = %v", err)
		}
		claim, err := globalDB.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
			Scope:            taskpkg.ScopeGlobal,
			ClaimerSessionID: "worker-goal-reentry",
			ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "worker"},
			LeaseDuration:    time.Minute,
			Now:              now,
		})
		if err != nil {
			t.Fatalf("ClaimNextRun() error = %v", err)
		}
		controlPayload, err := json.Marshal(looppkg.ActionControl{
			Disposition:  looppkg.ActionDispositionPaused,
			GoalStatus:   "paused",
			Cause:        "operator_pause",
			CheckpointID: "checkpoint-goal-reentry",
		})
		if err != nil {
			t.Fatalf("json.Marshal(control) error = %v", err)
		}
		if _, err := globalDB.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
			Actor:      coordinatorActorContextForTest(),
			RunID:      claim.Run.ID,
			ClaimToken: claim.ClaimToken,
			Result: taskpkg.RunResult{CoordinatorControl: &taskpkg.CoordinatorControlResult{
				Kind:    "loop_action",
				Payload: controlPayload,
			}},
			Now: now.Add(time.Second),
		}); err != nil {
			t.Fatalf("CompleteRunLease() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_generation_outputs SET status = 'awaiting_goal'
			 WHERE loop_run_id = ? AND task_run_id = ?`,
			string(loopRun.ID),
			initialRunID,
		); err != nil {
			t.Fatalf("settle Goal output error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET status = 'paused' WHERE id = ?`,
			string(loopRun.ID),
		); err != nil {
			t.Fatalf("settle paused Run error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_goal_checkpoints
			 SET phase = 'awaiting_control', goal_status = 'paused', control_cause = 'operator_resume',
			     updated_at = ?
			 WHERE loop_run_id = ? AND generation = 1 AND node_id = 'converge'`,
			now.Add(time.Second).Format(time.RFC3339Nano),
			string(loopRun.ID),
		); err != nil {
			t.Fatalf("settle paused Goal checkpoint error = %v", err)
		}

		state, found, err := globalDB.LoadAwaitingGoalControl(ctx, loopRun.WorkspaceID, loopRun.ID)
		if err != nil {
			t.Fatalf("LoadAwaitingGoalControl() error = %v", err)
		}
		if !found || state.ControlEpoch != 1 || state.Cause != "operator_resume" {
			t.Fatalf("awaiting state = %#v, found=%t", state, found)
		}
		actor := operatorActorContextForTest("user:goal-operator")
		req := looppkg.GoalReactivationRequest{
			State: state,
			Kind:  looppkg.GoalGrantPlainResume,
			Scope: looppkg.GoalGrantScopeReactivate,
			Actor: actor,
		}
		const attempts = 8
		results := make([]looppkg.GoalReactivationResult, attempts)
		errs := make([]error, attempts)
		var wg sync.WaitGroup
		wg.Add(attempts)
		for idx := range attempts {
			go func() {
				defer wg.Done()
				results[idx], errs[idx] = globalDB.ReactivateGoalRun(ctx, req)
			}()
		}
		wg.Wait()
		for idx, err := range errs {
			if err != nil {
				t.Fatalf("ReactivateGoalRun(attempt=%d) error = %v", idx, err)
			}
		}
		successorRunID := looppkg.GoalSegmentRunID(loopRun.ID, 1, "converge", 0, 2)
		for idx, result := range results {
			if result.Run.ID != successorRunID || result.ControlEpoch != 2 || result.GrantID != 1 {
				t.Fatalf("result[%d] = %#v, want one epoch-2 successor", idx, result)
			}
		}
		var successorCount int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM task_runs WHERE id = ?`,
			successorRunID,
		).Scan(&successorCount); err != nil {
			t.Fatalf("count successor runs error = %v", err)
		}
		if successorCount != 1 {
			t.Fatalf("successor run count = %d, want 1", successorCount)
		}
		successor, err := globalDB.GetTaskRun(ctx, successorRunID)
		if err != nil {
			t.Fatalf("GetTaskRun(successor) error = %v", err)
		}
		if successor.RunKind != taskpkg.RunKindWorker || !successor.IsLoopWorker() ||
			successor.LoopRunID != string(loopRun.ID) {
			t.Fatalf("successor activation identity = %#v", successor)
		}
		var successorMetadata struct {
			Generation       int    `json:"generation"`
			NodeID           string `json:"node_id"`
			ItemIndex        int    `json:"item_index"`
			GoalSegmentEpoch int64  `json:"goal_segment_epoch"`
		}
		if err := json.Unmarshal(successor.Metadata, &successorMetadata); err != nil {
			t.Fatalf("json.Unmarshal(successor metadata) error = %v", err)
		}
		if successorMetadata.Generation != 1 || successorMetadata.NodeID != "converge" ||
			successorMetadata.ItemIndex != 0 || successorMetadata.GoalSegmentEpoch != 2 {
			t.Fatalf("successor metadata = %#v", successorMetadata)
		}
		checkpoint, err := globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint() error = %v", err)
		}
		if checkpoint.ControlEpoch != 2 || checkpoint.TaskRunID != successorRunID ||
			checkpoint.ControlGrant == nil || !checkpoint.ControlGrant.Consumed {
			t.Fatalf("checkpoint after reentry = %#v", checkpoint)
		}
		var promptTaskRunID string
		var ownerEpoch int64
		var fenceKind *string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT task_run_id, owner_epoch, fence_kind FROM session_input_queue WHERE id = ?`,
			ticket.QueueEntryID,
		).Scan(&promptTaskRunID, &ownerEpoch, &fenceKind); err != nil {
			t.Fatalf("query reactivated Goal prompt error = %v", err)
		}
		if promptTaskRunID != successorRunID || ownerEpoch != 2 || fenceKind != nil {
			t.Fatalf("reactivated prompt = task:%q epoch:%d fence:%v", promptTaskRunID, ownerEpoch, fenceKind)
		}
		storedRun, err := globalDB.GetLoopRunByID(ctx, loopRun.ID)
		if err != nil {
			t.Fatalf("GetLoopRunByID() error = %v", err)
		}
		if storedRun.Status != looppkg.StatusRunning || storedRun.ActiveGateID != "" {
			t.Fatalf("Run after reentry = %#v", storedRun)
		}
	})
}
