//go:build integration

package globaldb

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestGoalTurnRuntimeLifecycleIntegration(t *testing.T) {
	t.Run("Should keep one prepared prompt identity per control epoch", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-prepare-once")
		ctx := testutil.Context(t)
		firstRequest := goal.PreparePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         "queue:goal-prompt-first",
			PromptID:             "goal-prompt-first",
			PromptKind:           "work",
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			Message:              "Advance the durable Goal",
			PreparedAt:           now,
		}
		first, err := globalDB.PrepareGoalPrompt(ctx, firstRequest)
		if err != nil {
			t.Fatalf("PrepareGoalPrompt(first) error = %v", err)
		}
		repeated, err := globalDB.PrepareGoalPrompt(ctx, firstRequest)
		if err != nil {
			t.Fatalf("PrepareGoalPrompt(repeated) error = %v", err)
		}
		if repeated != first {
			t.Fatalf("repeated ticket = %#v, want %#v", repeated, first)
		}
		secondRequest := firstRequest
		secondRequest.QueueEntryID = "queue:goal-prompt-second"
		secondRequest.PromptID = "goal-prompt-second"
		if _, err := globalDB.PrepareGoalPrompt(ctx, secondRequest); err == nil {
			t.Fatal("PrepareGoalPrompt(second identity) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalControlStale)
		}
		var queueRows int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM session_input_queue WHERE loop_run_id = ?`,
			string(key.LoopRunID),
		).Scan(&queueRows); err != nil {
			t.Fatalf("count prepared Goal prompts error = %v", err)
		}
		if queueRows != 1 {
			t.Fatalf("prepared queue rows = %d, want 1", queueRows)
		}
		checkpoint, err := globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint() error = %v", err)
		}
		if checkpoint.PromptID != first.PromptID || checkpoint.QueueEntryID != first.QueueEntryID {
			t.Fatalf("prepared checkpoint = %#v, want first ticket", checkpoint)
		}
	})

	t.Run("Should preserve unknown versus reported-zero usage through the ready-slot claim", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name     string
			reported bool
		}{
			{name: "Should keep unknown usage absent", reported: false},
			{name: "Should keep a reported zero present", reported: true},
		}
		for index, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				runID := "run-goal-usage-nullability-" + strconv.Itoa(index)
				globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, runID)
				promptID := "goal-prompt-usage-nullability-" + strconv.Itoa(index)
				ticket, err := globalDB.PrepareGoalPrompt(testutil.Context(t), goal.PreparePromptRequest{
					Key: key, ExpectedControlEpoch: 1, TaskRunID: taskRunID,
					QueueEntryID: "queue:" + promptID, PromptID: promptID, PromptKind: "work",
					SessionID: "session-goal-runtime", BindingHandle: "goal:runtime", BindingEpoch: 1,
					UsageBaseTokens: 0, UsageBaseReported: tc.reported,
					Message: "Advance the durable Goal", PreparedAt: now,
				})
				if err != nil {
					t.Fatalf("PrepareGoalPrompt() error = %v", err)
				}
				decision := flushGoalRuntimeBudget(
					t, globalDB, key, taskRunID, goal.BudgetBeforeWork, promptID, 0,
				)
				if _, err := globalDB.ClaimPreparedWorkPrompt(testutil.Context(t), goal.ClaimPreparedWorkPromptRequest{
					Key: key, ExpectedControlEpoch: 1, TaskRunID: taskRunID,
					QueueEntryID: ticket.QueueEntryID, PromptID: promptID,
					SessionID: "session-goal-runtime", BindingHandle: "goal:runtime", BindingEpoch: 1,
					UsageBaseTokens: 0, UsageBaseReported: tc.reported, BudgetDecision: decision,
					ActorKind: "daemon", ActorID: "loop-action",
				}); err != nil {
					t.Fatalf("ClaimPreparedWorkPrompt() error = %v", err)
				}
				entry, err := globalDB.GetSessionInputQueueEntryByID(testutil.Context(t), ticket.QueueEntryID)
				if err != nil {
					t.Fatalf("GetSessionInputQueueEntryByID() error = %v", err)
				}
				if tc.reported && (entry.OperationUsageBaseTokens == nil || *entry.OperationUsageBaseTokens != 0) {
					t.Fatalf("reported-zero usage base = %#v, want pointer to zero", entry.OperationUsageBaseTokens)
				}
				if !tc.reported && entry.OperationUsageBaseTokens != nil {
					t.Fatalf("unknown usage base = %#v, want nil", entry.OperationUsageBaseTokens)
				}
			})
		}
	})

	t.Run("Should advance only a proven pre-submit rejection and deduplicate it", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-safe-rejection")
		promptID := "goal-prompt-safe-rejection"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		request := goal.RejectPreparedPromptRequest{
			Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1,
			TaskRunID: taskRunID, QueueEntryID: ticket.QueueEntryID, PromptID: promptID,
			ReasonCode: looppkg.ReasonCodeGoalPromptRequestFailed,
		}
		if err := globalDB.RejectPreparedGoalPrompt(testutil.Context(t), request); err != nil {
			t.Fatalf("RejectPreparedGoalPrompt() error = %v", err)
		}
		if err := globalDB.RejectPreparedGoalPrompt(testutil.Context(t), request); err != nil {
			t.Fatalf("RejectPreparedGoalPrompt(idempotent) error = %v", err)
		}

		entry, err := globalDB.GetSessionInputQueueEntryByID(testutil.Context(t), ticket.QueueEntryID)
		if err != nil {
			t.Fatalf("GetSessionInputQueueEntryByID() error = %v", err)
		}
		if entry.Status != store.SessionInputQueueStatusFailed ||
			entry.TerminalKind != string(looppkg.ActionPromptOutcomeRejectedBeforeSubmit) ||
			entry.TerminalReasonCode != string(looppkg.ReasonCodeGoalPromptRequestFailed) ||
			entry.DispatchTokenHash != "" {
			t.Fatalf("rejected queue entry = %#v", entry)
		}
		checkpoint, err := globalDB.LoadCheckpoint(testutil.Context(t), key)
		if err != nil {
			t.Fatalf("LoadCheckpoint() error = %v", err)
		}
		if checkpoint.Phase != "preparing" || checkpoint.PromptAttempt != 1 ||
			checkpoint.QueueEntryID != "" || checkpoint.PromptID != "" {
			t.Fatalf("rejected checkpoint = %#v", checkpoint)
		}
	})

	t.Run("Should recover one exact terminal without exposing the raw dispatch token", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-terminal-recovery")
		promptID := "goal-prompt-terminal-recovery"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		decision := flushGoalRuntimeBudget(
			t, globalDB, key, taskRunID, goal.BudgetBeforeWork, promptID, 0,
		)
		if _, err := globalDB.ClaimPreparedWorkPrompt(testutil.Context(t), goal.ClaimPreparedWorkPromptRequest{
			Key: key, ExpectedControlEpoch: 1, TaskRunID: taskRunID,
			QueueEntryID: ticket.QueueEntryID, PromptID: promptID,
			SessionID: "session-goal-runtime", BindingHandle: "goal:runtime", BindingEpoch: 1,
			BudgetDecision: decision, ActorKind: "daemon", ActorID: "loop-action",
		}); err != nil {
			t.Fatalf("ClaimPreparedWorkPrompt() error = %v", err)
		}
		result := looppkg.ActionPromptResult{
			PromptID: promptID, Outcome: looppkg.ActionPromptOutcomeCompleted,
			StopReason: looppkg.ActionStopEndTurn, EventStartSeq: 10, EventEndSeq: 12,
		}
		request := goal.RecoverPromptTerminalRequest{
			Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1,
			TaskRunID: taskRunID, QueueEntryID: ticket.QueueEntryID, PromptID: promptID,
			SessionID: "session-goal-runtime", BindingHandle: "goal:runtime",
			Result: result, TerminalAt: now.Add(time.Second),
		}
		if err := globalDB.RecoverGoalPromptTerminal(testutil.Context(t), request); err != nil {
			t.Fatalf("RecoverGoalPromptTerminal() error = %v", err)
		}
		if err := globalDB.RecoverGoalPromptTerminal(testutil.Context(t), request); err != nil {
			t.Fatalf("RecoverGoalPromptTerminal(idempotent) error = %v", err)
		}
		entry, err := globalDB.GetSessionInputQueueEntryByID(testutil.Context(t), ticket.QueueEntryID)
		if err != nil {
			t.Fatalf("GetSessionInputQueueEntryByID() error = %v", err)
		}
		if entry.TerminalKind != string(looppkg.ActionPromptOutcomeCompleted) ||
			entry.TerminalStopReason != string(looppkg.ActionStopEndTurn) ||
			entry.TerminalEventStartSeq == nil || *entry.TerminalEventStartSeq != 10 ||
			entry.TerminalEventEndSeq == nil || *entry.TerminalEventEndSeq != 12 ||
			entry.DispatchTokenHash == "" {
			t.Fatalf("recovered queue entry = %#v", entry)
		}
		conflict := request
		conflict.Result.StopReason = looppkg.ActionStopMaxTokens
		if err := globalDB.RecoverGoalPromptTerminal(
			testutil.Context(t),
			conflict,
		); !errors.Is(
			err,
			looppkg.ErrTransitionConflict,
		) {
			t.Fatalf("RecoverGoalPromptTerminal(conflict) error = %v, want transition conflict", err)
		}
	})

	t.Run("Should bind a denied budget fence to the exact decision", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name   string
			mutate func(*goal.BudgetDecision)
		}{
			{
				name: "Should reject a different cause",
				mutate: func(decision *goal.BudgetDecision) {
					decision.Cause = looppkg.ReasonCodeGoalTurnsExhausted
				},
			},
			{
				name: "Should reject a different disposition",
				mutate: func(decision *goal.BudgetDecision) {
					decision.Disposition = looppkg.ActionDispositionNeedsApproval
				},
			},
		}
		for index, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				runID := "run-goal-fence-decision-" + strconv.Itoa(index)
				globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, runID)
				ctx := testutil.Context(t)
				if _, err := globalDB.db.ExecContext(
					ctx,
					`UPDATE loop_runs SET budget_tokens = 1, budget_on_exceeded = 'halt' WHERE id = ?`,
					string(key.LoopRunID),
				); err != nil {
					t.Fatalf("configure denied Goal budget error = %v", err)
				}
				promptID := "goal-prompt-fence-decision-" + strconv.Itoa(index)
				ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
				denied := flushGoalRuntimeBudget(
					t, globalDB, key, taskRunID, goal.BudgetBeforeWork, promptID, 1,
				)
				if denied.Allowed || denied.Cause != looppkg.ReasonCodeGoalBudgetFenced ||
					denied.Disposition != looppkg.ActionDispositionExhausted {
					t.Fatalf("denied Goal decision = %#v", denied)
				}
				tc.mutate(&denied)
				err := globalDB.FencePreparedPrompt(ctx, goal.FencePreparedPromptRequest{
					Key:                  key,
					ExpectedControlEpoch: 1,
					ExpectedBindingEpoch: 1,
					TaskRunID:            taskRunID,
					QueueEntryID:         ticket.QueueEntryID,
					PromptID:             promptID,
					Outcome:              looppkg.ActionPromptOutcomeBudgetFenced,
					Disposition:          looppkg.ActionDispositionExhausted,
					Cause:                looppkg.ReasonCodeGoalBudgetFenced,
					BudgetDecision:       denied,
				})
				if err == nil {
					t.Fatal("FencePreparedPrompt(mismatched decision) error = nil")
				}
				requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalPromptFenced)
			})
		}
	})

	t.Run("Should never return a terminal prepared ticket", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-terminal-ticket")
		ctx := testutil.Context(t)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET budget_tokens = 1, budget_on_exceeded = 'halt' WHERE id = ?`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("configure terminal-ticket budget error = %v", err)
		}
		promptID := "goal-prompt-terminal-ticket"
		request := goal.PreparePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         "queue:" + promptID,
			PromptID:             promptID,
			PromptKind:           "work",
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			Message:              "Advance the durable Goal",
			PreparedAt:           now,
		}
		ticket, err := globalDB.PrepareGoalPrompt(ctx, request)
		if err != nil {
			t.Fatalf("PrepareGoalPrompt() error = %v", err)
		}
		denied := flushGoalRuntimeBudget(
			t, globalDB, key, taskRunID, goal.BudgetBeforeWork, promptID, 1,
		)
		if err := globalDB.FencePreparedPrompt(ctx, goal.FencePreparedPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Outcome:              looppkg.ActionPromptOutcomeBudgetFenced,
			Disposition:          looppkg.ActionDispositionExhausted,
			Cause:                looppkg.ReasonCodeGoalBudgetFenced,
			BudgetDecision:       denied,
		}); err != nil {
			t.Fatalf("FencePreparedPrompt() error = %v", err)
		}
		if _, err := globalDB.PrepareGoalPrompt(ctx, request); err == nil {
			t.Fatal("PrepareGoalPrompt(terminal row) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalControlStale)
		}
	})

	t.Run("Should attribute a control-fenced prompt to the Goal control actor", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-control-fence-actor")
		ctx := testutil.Context(t)
		promptID := "goal-prompt-control-fence-actor"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		if err := globalDB.FencePreparedPrompt(ctx, goal.FencePreparedPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Outcome:              looppkg.ActionPromptOutcomeControlFenced,
			Disposition:          looppkg.ActionDispositionPaused,
			Cause:                looppkg.ReasonCodeGoalPromptFenced,
		}); err != nil {
			t.Fatalf("FencePreparedPrompt(control) error = %v", err)
		}
		events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			WorkspaceID: key.WorkspaceID,
			RunID:       key.LoopRunID,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents() error = %v", err)
		}
		statusEvent := loopEventPayloadForKind(t, events, loopRunEventGoalStatusChanged)
		if statusEvent["actor_kind"] != "system" || statusEvent["actor_id"] != "goal-control" {
			t.Fatalf("control-fenced Goal status actor = %#v", statusEvent)
		}
	})

	t.Run("Should linearize one work claim before effect and settle its authoritative judge", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-runtime")
		ctx := testutil.Context(t)
		promptID := "goal-prompt-runtime"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		decision := flushGoalRuntimeBudget(t, globalDB, key, taskRunID, goal.BudgetBeforeWork, promptID, 0)
		claimed, err := globalDB.ClaimPreparedWorkPrompt(ctx, goal.ClaimPreparedWorkPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			UsageBaseTokens:      0,
			BudgetDecision:       decision,
			ActorKind:            "daemon",
			ActorID:              "loop-action",
		})
		if err != nil {
			t.Fatalf("ClaimPreparedWorkPrompt() error = %v", err)
		}
		if claimed.DispatchToken == "" || claimed.Checkpoint.TurnsUsed != 1 ||
			claimed.Checkpoint.Phase != "prompting" {
			t.Fatalf("claimed = %#v", claimed)
		}
		wrongWorkspaceKey := key
		wrongWorkspaceKey.WorkspaceID = "ws-other"
		if err := globalDB.RecordGoalDriverAttached(ctx, goal.DriverAttachmentRequest{
			Key:                  wrongWorkspaceKey,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			DriverTurnID:         promptID,
			AttachedAt:           now.Add(time.Second),
		}); err == nil {
			t.Fatal("RecordGoalDriverAttached(wrong workspace) error = nil")
		}
		if err := globalDB.FinalizeGoalPrompt(ctx, goal.FinalizePromptRequest{
			Key:                  wrongWorkspaceKey,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			Result: looppkg.ActionPromptResult{
				PromptID:   promptID,
				Outcome:    looppkg.ActionPromptOutcomeCompleted,
				StopReason: looppkg.ActionStopEndTurn,
			},
			TerminalAt: now.Add(2 * time.Second),
		}); err == nil {
			t.Fatal("FinalizeGoalPrompt(wrong workspace) error = nil")
		}
		if err := globalDB.RecordGoalDriverAttached(ctx, goal.DriverAttachmentRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			DriverTurnID:         promptID,
			AttachedAt:           now.Add(time.Second),
		}); err != nil {
			t.Fatalf("RecordGoalDriverAttached() error = %v", err)
		}
		promptResult := looppkg.ActionPromptResult{
			PromptID:       promptID,
			Outcome:        looppkg.ActionPromptOutcomeCompleted,
			StopReason:     looppkg.ActionStopEndTurn,
			TokensUsed:     5,
			TokensReported: true,
		}
		finalizeRequest := goal.FinalizePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			Result:               promptResult,
			TerminalAt:           now.Add(2 * time.Second),
		}
		if err := globalDB.FinalizeGoalPrompt(ctx, finalizeRequest); err != nil {
			t.Fatalf("FinalizeGoalPrompt() error = %v", err)
		}
		conflictingRange := finalizeRequest
		conflictingRange.Result.EventEndSeq = 1
		if err := globalDB.FinalizeGoalPrompt(ctx, conflictingRange); !errors.Is(err, looppkg.ErrTransitionConflict) {
			t.Fatalf("FinalizeGoalPrompt(conflicting event range) error = %v, want transition conflict", err)
		}
		judgeDecision := flushGoalRuntimeBudget(
			t, globalDB, key, taskRunID, goal.BudgetBeforeJudge, "judge-runtime", 5,
		)
		attempt, err := globalDB.BeginJudgeAttempt(ctx, goal.BeginJudgeAttemptRequest{
			AttemptID:            "judge-runtime",
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			PromptID:             promptID,
			Turn:                 1,
			JudgeDigest:          "judge-digest-runtime",
			UsageBaseTokens:      5,
			BudgetDecision:       judgeDecision,
		})
		if err != nil || attempt.Status != "running" {
			t.Fatalf("BeginJudgeAttempt() = %#v, %v", attempt, err)
		}
		verdict := gate.Verdict{Outcome: gate.VerdictOutcomeApproved}
		if _, err := globalDB.CompleteJudgeAttempt(ctx, goal.CompleteJudgeAttemptRequest{
			AttemptID:            attempt.AttemptID,
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			PromptID:             promptID,
			Verdict:              verdict,
			TokensUsed:           0,
			TokensReported:       false,
		}); err != nil {
			t.Fatalf("CompleteJudgeAttempt() error = %v", err)
		}
		completeReq := goal.CompleteTurnRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Result:               promptResult,
			Verdict:              &verdict,
			DispatchActorKind:    "daemon",
			DispatchActorID:      "loop-action",
		}
		completed, err := globalDB.CompleteTurn(ctx, completeReq)
		if err != nil {
			t.Fatalf("CompleteTurn() error = %v", err)
		}
		if completed.Status != "complete" || completed.Phase != "terminal" || completed.TurnsUsed != 1 {
			t.Fatalf("completed checkpoint = %#v", completed)
		}
		binding, err := globalDB.GetSessionBindingAttempt(ctx, goal.BindingKey{
			WorkspaceID: key.WorkspaceID, LoopRunID: key.LoopRunID, Handle: "goal:runtime",
		}, 1)
		if err != nil {
			t.Fatalf("GetSessionBindingAttempt(terminal) error = %v", err)
		}
		cleanups, err := globalDB.ClaimGoalSessionCleanup(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionCleanup(terminal) error = %v", err)
		}
		if binding.State != goal.BindingStateClosed || len(cleanups) != 1 ||
			cleanups[0].SessionID != "session-goal-runtime" ||
			cleanups[0].Cause != goal.SessionCleanupCauseTerminal {
			t.Fatalf("terminal binding/cleanup = %#v/%#v", binding, cleanups)
		}
		assertGoalRuntimeTurn(t, globalDB, key, promptID, "completed", "end_turn")
		assertGoalRuntimeEventCount(t, globalDB, key.LoopRunID, loopRunEventGoalTurnStarted, 1)
		assertGoalRuntimeEventCount(t, globalDB, key.LoopRunID, loopRunEventGoalTurnCompleted, 1)
		events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			WorkspaceID: key.WorkspaceID,
			RunID:       key.LoopRunID,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents() error = %v", err)
		}
		startedEvent := loopEventPayloadForKind(t, events, loopRunEventGoalTurnStarted)
		for field, want := range map[string]any{
			"generation":     float64(1),
			"node_id":        "converge",
			"item_index":     float64(0),
			"turn":           float64(1),
			"prompt_attempt": float64(0),
			"prompt_id":      promptID,
			"session_id":     "session-goal-runtime",
			"binding_handle": "goal:runtime",
			"binding_epoch":  float64(1),
			"actor_kind":     "daemon",
			"actor_id":       "loop-action",
		} {
			if got := startedEvent[field]; got != want {
				t.Fatalf("started event %s = %#v, want %#v; payload=%#v", field, got, want, startedEvent)
			}
		}
		completedEvent := loopEventPayloadForKind(t, events, loopRunEventGoalTurnCompleted)
		for field, want := range map[string]any{
			"generation":      float64(1),
			"node_id":         "converge",
			"item_index":      float64(0),
			"turn":            float64(1),
			"prompt_attempt":  float64(0),
			"prompt_id":       promptID,
			"seq":             float64(1),
			"session_id":      "session-goal-runtime",
			"binding_handle":  "goal:runtime",
			"binding_epoch":   float64(1),
			"result_status":   "completed",
			"stop_reason":     "end_turn",
			"verdict_outcome": "approved",
			"tokens_used":     float64(5),
			"actor_kind":      "daemon",
			"actor_id":        "loop-action",
		} {
			if got := completedEvent[field]; got != want {
				t.Fatalf("completed event %s = %#v, want %#v; payload=%#v", field, got, want, completedEvent)
			}
		}
		if completedEvent["reason_code"] != nil || completedEvent["evidence_ref"] != nil {
			t.Fatalf("completed nullable fields = reason:%#v evidence:%#v, want nil; payload=%#v",
				completedEvent["reason_code"], completedEvent["evidence_ref"], completedEvent)
		}
		blockingIssues, ok := completedEvent["blocking_issues"].([]any)
		if !ok || len(blockingIssues) != 0 {
			t.Fatalf("completed blocking_issues = %#v, want empty array", completedEvent["blocking_issues"])
		}
		if _, err := globalDB.CompleteTurn(ctx, completeReq); err != nil {
			t.Fatalf("CompleteTurn(idempotent) error = %v", err)
		}
		cleanups, err = globalDB.ClaimGoalSessionCleanup(ctx, 10)
		if err != nil || len(cleanups) != 1 {
			t.Fatalf("terminal cleanup replay = %#v, %v", cleanups, err)
		}
		conflict := completeReq
		conflict.Verdict = &gate.Verdict{Outcome: gate.VerdictOutcomeRejected}
		if _, err := globalDB.CompleteTurn(ctx, conflict); !errors.Is(err, looppkg.ErrTransitionConflict) {
			t.Fatalf("CompleteTurn(conflicting) error = %v, want transition conflict", err)
		}
		wrongOwner := completeReq
		wrongOwner.TaskRunID = "other-task-run"
		if _, err := globalDB.CompleteTurn(ctx, wrongOwner); !errors.Is(err, looppkg.ErrTransitionConflict) {
			t.Fatalf("CompleteTurn(wrong owner) error = %v, want transition conflict", err)
		}
		assertGoalRuntimeEventCount(t, globalDB, key.LoopRunID, loopRunEventGoalTurnCompleted, 1)
	})

	t.Run("Should co-commit blocked terminal cleanup only for run-owned bindings", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name        string
			ownership   goal.BindingOwnership
			wantCleanup int
		}{
			{name: "run owned", ownership: goal.BindingOwnershipRunOwned, wantCleanup: 1},
			{name: "borrowed origin", ownership: goal.BindingOwnershipOriginBorrowed, wantCleanup: 0},
		} {
			t.Run("Should settle "+tc.name, func(t *testing.T) {
				t.Parallel()

				runID := "run-goal-blocked-cleanup-" + strings.ReplaceAll(tc.name, " ", "-")
				globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, runID)
				ctx := testutil.Context(t)
				if tc.ownership == goal.BindingOwnershipOriginBorrowed {
					if _, err := globalDB.db.ExecContext(
						ctx,
						`UPDATE loop_session_bindings SET ownership = 'origin-borrowed'
						 WHERE loop_run_id = ? AND handle = 'goal:runtime' AND binding_epoch = 1`,
						string(key.LoopRunID),
					); err != nil {
						t.Fatalf("set borrowed binding ownership error = %v", err)
					}
				}
				promptID := "goal-prompt-blocked-cleanup"
				ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
				decision := flushGoalRuntimeBudget(
					t, globalDB, key, taskRunID, goal.BudgetBeforeWork, promptID, 0,
				)
				claimed, err := globalDB.ClaimPreparedWorkPrompt(ctx, goal.ClaimPreparedWorkPromptRequest{
					Key: key, ExpectedControlEpoch: 1, TaskRunID: taskRunID,
					QueueEntryID: ticket.QueueEntryID, PromptID: promptID,
					SessionID: "session-goal-runtime", BindingHandle: "goal:runtime", BindingEpoch: 1,
					BudgetDecision: decision, ActorKind: "daemon", ActorID: "loop-action",
				})
				if err != nil {
					t.Fatalf("ClaimPreparedWorkPrompt() error = %v", err)
				}
				terminal := looppkg.ActionPromptResult{
					PromptID: promptID, Outcome: looppkg.ActionPromptOutcomeCompleted,
					StopReason: looppkg.ActionStopEndTurn,
				}
				if err := globalDB.FinalizeGoalPrompt(ctx, goal.FinalizePromptRequest{
					Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, TaskRunID: taskRunID,
					QueueEntryID: ticket.QueueEntryID, PromptID: promptID,
					SessionID: "session-goal-runtime", BindingHandle: "goal:runtime",
					DispatchToken: claimed.DispatchToken, Result: terminal, TerminalAt: now.Add(time.Second),
				}); err != nil {
					t.Fatalf("FinalizeGoalPrompt() error = %v", err)
				}
				verdict := gate.Verdict{Outcome: gate.VerdictOutcomeBlocked}
				settled, err := globalDB.CompleteTurn(ctx, goal.CompleteTurnRequest{
					Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, TaskRunID: taskRunID,
					QueueEntryID: ticket.QueueEntryID, PromptID: promptID, Result: terminal, Verdict: &verdict,
					DispatchActorKind: "daemon", DispatchActorID: "loop-action",
				})
				if err != nil {
					t.Fatalf("CompleteTurn(blocked) error = %v", err)
				}
				cleanups, err := globalDB.ClaimGoalSessionCleanup(ctx, 10)
				if err != nil {
					t.Fatalf("ClaimGoalSessionCleanup() error = %v", err)
				}
				if settled.Status != "blocked" || settled.Phase != "terminal" || len(cleanups) != tc.wantCleanup {
					t.Fatalf("blocked settlement/cleanup = %#v/%#v", settled, cleanups)
				}
			})
		}
	})

	t.Run("Should reject a prepared claim after its binding is no longer active", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-closed-binding")
		ctx := testutil.Context(t)
		promptID := "goal-prompt-closed-binding"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		decision := flushGoalRuntimeBudget(
			t,
			globalDB,
			key,
			taskRunID,
			goal.BudgetBeforeWork,
			promptID,
			0,
		)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_session_bindings SET state = 'closed', closed_at = ?
			 WHERE loop_run_id = ? AND handle = ? AND binding_epoch = 1 AND state = 'active'`,
			store.FormatTimestamp(now.Add(time.Second)),
			string(key.LoopRunID),
			"goal:runtime",
		); err != nil {
			t.Fatalf("close Goal binding before claim error = %v", err)
		}
		_, err := globalDB.ClaimPreparedWorkPrompt(ctx, goal.ClaimPreparedWorkPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			BudgetDecision:       decision,
			ActorKind:            "daemon",
			ActorID:              "loop-action",
		})
		if err == nil {
			t.Fatal("ClaimPreparedWorkPrompt(closed binding) error = nil")
		}
		var turnCount int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM loop_goal_turns WHERE loop_run_id = ?`,
			string(key.LoopRunID),
		).Scan(&turnCount); err != nil {
			t.Fatalf("count turns after closed binding error = %v", err)
		}
		if turnCount != 0 {
			t.Fatalf("turns after closed binding = %d, want 0", turnCount)
		}
	})

	t.Run("Should reject driver and terminal evidence after the active binding changes", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-late-binding")
		ctx := testutil.Context(t)
		promptID := "goal-prompt-late-binding"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		decision := flushGoalRuntimeBudget(
			t,
			globalDB,
			key,
			taskRunID,
			goal.BudgetBeforeWork,
			promptID,
			0,
		)
		claimed, err := globalDB.ClaimPreparedWorkPrompt(ctx, goal.ClaimPreparedWorkPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			BudgetDecision:       decision,
			ActorKind:            "daemon",
			ActorID:              "loop-action",
		})
		if err != nil {
			t.Fatalf("ClaimPreparedWorkPrompt() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_session_bindings SET state = 'closed', closed_at = ?
			 WHERE loop_run_id = ? AND handle = ? AND binding_epoch = 1 AND state = 'active'`,
			store.FormatTimestamp(now.Add(time.Second)),
			string(key.LoopRunID),
			"goal:runtime",
		); err != nil {
			t.Fatalf("close Goal binding after claim error = %v", err)
		}
		attachment := goal.DriverAttachmentRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			DriverTurnID:         promptID,
			AttachedAt:           now.Add(2 * time.Second),
		}
		if err := globalDB.RecordGoalDriverAttached(ctx, attachment); err == nil {
			t.Fatal("RecordGoalDriverAttached(closed binding) error = nil")
		}
		if err := globalDB.FinalizeGoalPrompt(ctx, goal.FinalizePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			Result: looppkg.ActionPromptResult{
				PromptID:   promptID,
				Outcome:    looppkg.ActionPromptOutcomeCompleted,
				StopReason: looppkg.ActionStopEndTurn,
			},
			TerminalAt: now.Add(3 * time.Second),
		}); err == nil {
			t.Fatal("FinalizeGoalPrompt(closed binding) error = nil")
		}
		if _, err := globalDB.RecordReportIntent(ctx, goal.RecordReportIntentRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			PromptID:             promptID,
			Status:               "blocked",
			EvidenceRef:          "evidence:closed-binding",
			ActorKind:            "agent",
			ActorID:              "session-goal-runtime",
		}); err == nil {
			t.Fatal("RecordReportIntent(closed binding) error = nil")
		}
		if err := globalDB.MarkAmbiguous(ctx, goal.AmbiguousRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Cause:                looppkg.ReasonCodeGoalRecoveryAmbiguous,
		}); err == nil {
			t.Fatal("MarkAmbiguous(closed binding) error = nil")
		}
		var resultStatus *string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT result_status FROM loop_goal_turns WHERE loop_run_id = ? AND prompt_id = ?`,
			string(key.LoopRunID),
			promptID,
		).Scan(&resultStatus); err != nil {
			t.Fatalf("load late-binding turn result error = %v", err)
		}
		if resultStatus != nil {
			t.Fatalf("late-binding turn result = %v, want open", resultStatus)
		}
	})

	t.Run("Should settle compaction only while its exact binding remains active", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-compact-binding")
		ctx := testutil.Context(t)
		promptID := "goal-prompt-compact-binding"
		usageSequence := int64(41)
		usageBaselineUsed := int64(9)
		observed, err := globalDB.RecordContextUsage(ctx, goal.RecordContextUsageRequest{
			Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, ExpectedPhase: "idle",
			SessionID: "session-goal-runtime", BindingHandle: "goal:runtime",
			Usage: goal.ContextUsage{
				Known: true, Used: usageBaselineUsed, Size: 10, Sequence: usageSequence, ReportedAt: now,
			},
		})
		if err != nil {
			t.Fatalf("RecordContextUsage(compact floor) error = %v", err)
		}
		ticket, err := globalDB.PrepareGoalPrompt(ctx, goal.PreparePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         "queue:" + promptID,
			PromptID:             promptID,
			PromptKind:           "compact",
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			ContextUsageSequence: &usageSequence,
			ContextUsageUsed:     &usageBaselineUsed,
			Message:              "Compact the durable Goal context",
			PreparedAt:           now,
		})
		if err != nil {
			t.Fatalf("PrepareGoalPrompt(compact) error = %v", err)
		}
		if observed.ContextState != "known" || observed.UsageSequence == nil ||
			*observed.UsageSequence != usageSequence {
			t.Fatalf("observed context checkpoint = %#v", observed)
		}
		decision := flushGoalRuntimeBudget(
			t, globalDB, key, taskRunID, goal.BudgetBeforeCompact, promptID, 0,
		)
		claimed, err := globalDB.ClaimPreparedCompaction(ctx, goal.ClaimPreparedCompactionRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			UsageSequence:        &usageSequence,
			BudgetDecision:       decision,
		})
		if err != nil {
			t.Fatalf("ClaimPreparedCompaction() error = %v", err)
		}
		if claimed.Checkpoint.Phase != "compacting" || claimed.DispatchToken == "" ||
			claimed.Checkpoint.ContextState != "known" || claimed.Checkpoint.UsagePendingAfterSequence != nil ||
			claimed.Checkpoint.CompactionBaselineUsed == nil ||
			*claimed.Checkpoint.CompactionBaselineUsed != usageBaselineUsed {
			t.Fatalf("claimed compaction = %#v", claimed)
		}
		if err := globalDB.RecordGoalDriverAttached(ctx, goal.DriverAttachmentRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			DriverTurnID:         promptID,
			AttachedAt:           now.Add(time.Second),
		}); err != nil {
			t.Fatalf("RecordGoalDriverAttached(compact) error = %v", err)
		}
		terminal := looppkg.ActionPromptResult{
			PromptID:   promptID,
			Outcome:    looppkg.ActionPromptOutcomeCompleted,
			StopReason: looppkg.ActionStopEndTurn,
		}
		if err := globalDB.FinalizeGoalPrompt(ctx, goal.FinalizePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			Result:               terminal,
			TerminalAt:           now.Add(2 * time.Second),
		}); err != nil {
			t.Fatalf("FinalizeGoalPrompt(compact) error = %v", err)
		}
		path := globalDB.Path()
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close(before compaction recovery) error = %v", err)
		}
		globalDB, err = OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(compaction recovery) error = %v", err)
		}
		globalDB.now = func() time.Time { return now.Add(3 * time.Second) }
		t.Cleanup(func() {
			if err := globalDB.Close(context.Background()); err != nil {
				t.Errorf("Close(recovered compaction DB) error = %v", err)
			}
		})
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_session_bindings SET state = 'closed', closed_at = ?
			 WHERE loop_run_id = ? AND handle = 'goal:runtime' AND binding_epoch = 1`,
			store.FormatTimestamp(now.Add(3*time.Second)),
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("close compact binding error = %v", err)
		}
		result := goal.CompactionResult{
			PromptResult: terminal,
			Outcome:      goal.CompactionSucceeded,
		}
		complete := goal.CompleteCompactionRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Result:               result,
		}
		if _, err := globalDB.CompleteCompaction(ctx, complete); err == nil {
			t.Fatal("CompleteCompaction(closed binding) error = nil")
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_session_bindings SET state = 'active', closed_at = NULL
			 WHERE loop_run_id = ? AND handle = 'goal:runtime' AND binding_epoch = 1`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("reactivate compact binding error = %v", err)
		}
		completed, err := globalDB.CompleteCompaction(ctx, complete)
		if err != nil {
			t.Fatalf("CompleteCompaction() error = %v", err)
		}
		if completed.Phase != "idle" || completed.ContextState != "pending" ||
			completed.UsagePendingAfterSequence == nil || *completed.UsagePendingAfterSequence != usageSequence ||
			completed.CompactionBaselineUsed == nil || *completed.CompactionBaselineUsed != usageBaselineUsed {
			t.Fatalf("completed compaction checkpoint = %#v", completed)
		}
		stale, err := globalDB.RecordContextUsage(ctx, goal.RecordContextUsageRequest{
			Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, ExpectedPhase: "idle",
			SessionID: "session-goal-runtime", BindingHandle: "goal:runtime",
			Usage: goal.ContextUsage{
				Known: true, Used: usageBaselineUsed, Size: 10, Sequence: usageSequence,
				ReportedAt: now.Add(4 * time.Second),
			},
		})
		if err != nil || stale.ContextState != "pending" {
			t.Fatalf("RecordContextUsage(stale after recovery) = %#v, %v", stale, err)
		}
		fresh, err := globalDB.RecordContextUsage(ctx, goal.RecordContextUsageRequest{
			Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, ExpectedPhase: "idle",
			SessionID: "session-goal-runtime", BindingHandle: "goal:runtime",
			Usage: goal.ContextUsage{
				Known: true, Used: 4, Size: 10, Sequence: usageSequence + 1,
				ReportedAt: now.Add(5 * time.Second),
			},
		})
		if err != nil || fresh.ContextState != "known" || fresh.CompactionBaselineUsed != nil ||
			fresh.CompactionRecoveryRequired {
			t.Fatalf("RecordContextUsage(fresh lower after recovery) = %#v, %v", fresh, err)
		}
		var turns int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM loop_goal_turns WHERE loop_run_id = ?`,
			string(key.LoopRunID),
		).Scan(&turns); err != nil {
			t.Fatalf("count compact Goal turns error = %v", err)
		}
		if turns != 0 {
			t.Fatalf("compact Goal turns = %d, want 0", turns)
		}
	})

	t.Run("Should preserve known context after an ineffective compaction", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-compact-ineffective")
		usageSequence := int64(52)
		if _, err := globalDB.RecordContextUsage(testutil.Context(t), goal.RecordContextUsageRequest{
			Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, ExpectedPhase: "idle",
			SessionID: "session-goal-runtime", BindingHandle: "goal:runtime",
			Usage: goal.ContextUsage{
				Known: true, Used: 9, Size: 10, Sequence: usageSequence, ReportedAt: now,
			},
		}); err != nil {
			t.Fatalf("RecordContextUsage() error = %v", err)
		}
		ticket, terminal := finalizeGoalRuntimeCompaction(
			t,
			globalDB,
			key,
			taskRunID,
			"goal-prompt-compact-ineffective",
			usageSequence,
			now,
		)
		usageBaselineUsed := int64(9)
		completed, err := globalDB.CompleteCompaction(testutil.Context(t), goal.CompleteCompactionRequest{
			Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, TaskRunID: taskRunID,
			QueueEntryID: ticket.QueueEntryID, PromptID: ticket.PromptID,
			Result: goal.CompactionResult{
				PromptResult: terminal, Outcome: goal.CompactionIneffective,
				UsageSequence: &usageSequence, UsageBaselineUsed: &usageBaselineUsed,
			},
		})
		if err != nil {
			t.Fatalf("CompleteCompaction(ineffective) error = %v", err)
		}
		if completed.ContextState != "known" || completed.UsageSequence == nil ||
			*completed.UsageSequence != usageSequence || completed.UsagePendingAfterSequence != nil {
			t.Fatalf("ineffective compaction context = %#v", completed)
		}
	})

	t.Run("Should preserve a nil compaction baseline through successful settlement", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-compact-null-baseline")
		ctx := testutil.Context(t)
		promptID := "goal-prompt-compact-null-baseline"
		ticket, err := globalDB.PrepareGoalPrompt(ctx, goal.PreparePromptRequest{
			Key: key, ExpectedControlEpoch: 1, TaskRunID: taskRunID,
			QueueEntryID: "queue:" + promptID, PromptID: promptID, PromptKind: "compact",
			SessionID: "session-goal-runtime", BindingHandle: "goal:runtime", BindingEpoch: 1,
			Message: "Compact without prior telemetry", PreparedAt: now,
		})
		if err != nil {
			t.Fatalf("PrepareGoalPrompt(nil baseline) error = %v", err)
		}
		decision := flushGoalRuntimeBudget(
			t, globalDB, key, taskRunID, goal.BudgetBeforeCompact, promptID, 0,
		)
		claimed, err := globalDB.ClaimPreparedCompaction(ctx, goal.ClaimPreparedCompactionRequest{
			Key: key, ExpectedControlEpoch: 1, TaskRunID: taskRunID,
			QueueEntryID: ticket.QueueEntryID, PromptID: promptID,
			SessionID: "session-goal-runtime", BindingHandle: "goal:runtime", BindingEpoch: 1,
			BudgetDecision: decision,
		})
		if err != nil {
			t.Fatalf("ClaimPreparedCompaction(nil baseline) error = %v", err)
		}
		if claimed.Checkpoint.UsageSequence != nil || claimed.Checkpoint.CompactionBaselineUsed != nil {
			t.Fatalf("claimed nil-baseline compaction = %#v", claimed.Checkpoint)
		}
		if err := globalDB.RecordGoalDriverAttached(ctx, goal.DriverAttachmentRequest{
			Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, TaskRunID: taskRunID,
			QueueEntryID: ticket.QueueEntryID, PromptID: promptID,
			SessionID: "session-goal-runtime", BindingHandle: "goal:runtime",
			DispatchToken: claimed.DispatchToken, DriverTurnID: promptID, AttachedAt: now.Add(time.Second),
		}); err != nil {
			t.Fatalf("RecordGoalDriverAttached(nil baseline) error = %v", err)
		}
		terminal := looppkg.ActionPromptResult{
			PromptID: promptID, Outcome: looppkg.ActionPromptOutcomeCompleted, StopReason: looppkg.ActionStopEndTurn,
		}
		if err := globalDB.FinalizeGoalPrompt(ctx, goal.FinalizePromptRequest{
			Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, TaskRunID: taskRunID,
			QueueEntryID: ticket.QueueEntryID, PromptID: promptID,
			SessionID: "session-goal-runtime", BindingHandle: "goal:runtime",
			DispatchToken: claimed.DispatchToken, Result: terminal, TerminalAt: now.Add(2 * time.Second),
		}); err != nil {
			t.Fatalf("FinalizeGoalPrompt(nil baseline) error = %v", err)
		}
		completed, err := globalDB.CompleteCompaction(ctx, goal.CompleteCompactionRequest{
			Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, TaskRunID: taskRunID,
			QueueEntryID: ticket.QueueEntryID, PromptID: promptID,
			Result: goal.CompactionResult{PromptResult: terminal, Outcome: goal.CompactionSucceeded},
		})
		if err != nil {
			t.Fatalf("CompleteCompaction(nil baseline) error = %v", err)
		}
		if completed.ContextState != "pending" || completed.UsageSequence != nil ||
			completed.UsagePendingAfterSequence != nil || completed.CompactionBaselineUsed != nil {
			t.Fatalf("completed nil-baseline compaction = %#v", completed)
		}
	})

	t.Run("Should let control revoke a prepared prompt before claim without a turn", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-revoke-prepared")
		ctx := testutil.Context(t)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET origin_kind = 'session', origin_session_id = ? WHERE id = ?`,
			"session-goal-runtime",
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("set revoke-prepared session origin error = %v", err)
		}
		promptID := "goal-prompt-revoke-prepared"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		decision := flushGoalRuntimeBudget(
			t,
			globalDB,
			key,
			taskRunID,
			goal.BudgetBeforeWork,
			promptID,
			0,
		)
		revokeRequest := goal.RevokePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Disposition:          looppkg.ActionDispositionPaused,
			Status:               "paused",
			Cause:                looppkg.ReasonCodeGoalControlRevokedInFlight,
			ActorKind:            "user",
			ActorID:              "operator:stop",
			ProjectionCause:      goal.SessionOutboxCauseStatus,
			RevokedAt:            now.Add(time.Second),
		}
		revoked, err := globalDB.RevokeGoalPrompt(ctx, revokeRequest)
		if err != nil {
			t.Fatalf("RevokeGoalPrompt(prepared) error = %v", err)
		}
		if revoked.ControlEpoch != 2 || revoked.Phase != "terminal" || revoked.Status != "paused" {
			t.Fatalf("revoked prepared checkpoint = %#v, want epoch 2 terminal paused", revoked)
		}
		if _, err := globalDB.ClaimPreparedWorkPrompt(ctx, goal.ClaimPreparedWorkPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			BudgetDecision:       decision,
			ActorKind:            "daemon",
			ActorID:              "loop-action",
		}); err == nil {
			t.Fatal("ClaimPreparedWorkPrompt(after revoke) error = nil")
		}
		var turnCount int
		var terminalKind, terminalReason *string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT terminal_kind, terminal_reason_code FROM session_input_queue WHERE id = ?`,
			ticket.QueueEntryID,
		).Scan(&terminalKind, &terminalReason); err != nil {
			t.Fatalf("load revoked prepared queue error = %v", err)
		}
		if terminalKind == nil || *terminalKind != "control-fenced" || terminalReason == nil ||
			*terminalReason != string(looppkg.ReasonCodeGoalControlRevokedInFlight) {
			t.Fatalf("revoked prepared terminal = kind:%v reason:%v", terminalKind, terminalReason)
		}
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM loop_goal_turns WHERE loop_run_id = ?`,
			string(key.LoopRunID),
		).Scan(&turnCount); err != nil {
			t.Fatalf("count revoked prepared turns error = %v", err)
		}
		if turnCount != 0 {
			t.Fatalf("revoked prepared turns = %d, want 0", turnCount)
		}
		pending, err := globalDB.ClaimGoalSessionOutbox(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionOutbox(revoked prepared) error = %v", err)
		}
		if len(pending) != 1 || pending[0].Cause != goal.SessionOutboxCauseStatus {
			t.Fatalf("revoked prepared outbox = %#v, want one status projection", pending)
		}
	})

	t.Run(
		"Should atomically stop the Run and revoke its claimed Goal prompt with the authenticated actor",
		func(t *testing.T) {
			t.Parallel()

			globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-operator-stop")
			ctx := testutil.Context(t)
			promptID := "goal-prompt-operator-stop"
			ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
			decision := flushGoalRuntimeBudget(
				t, globalDB, key, taskRunID, goal.BudgetBeforeWork, promptID, 0,
			)
			claimed, err := globalDB.ClaimPreparedWorkPrompt(ctx, goal.ClaimPreparedWorkPromptRequest{
				Key: key, ExpectedControlEpoch: 1, TaskRunID: taskRunID,
				QueueEntryID: ticket.QueueEntryID, PromptID: promptID,
				SessionID: "session-goal-runtime", BindingHandle: "goal:runtime", BindingEpoch: 1,
				BudgetDecision: decision, ActorKind: "daemon", ActorID: "loop-action",
			})
			if err != nil {
				t.Fatalf("ClaimPreparedWorkPrompt() error = %v", err)
			}
			actor, err := taskpkg.DeriveHumanActorContext("operator-stop", taskpkg.OriginKindCLI, "cli")
			if err != nil {
				t.Fatalf("DeriveHumanActorContext() error = %v", err)
			}
			stoppedAt := now.Add(time.Second)
			result, err := globalDB.StopGoalRun(ctx, looppkg.GoalRunStopRequest{
				WorkspaceID: key.WorkspaceID, RunID: key.LoopRunID,
				ExpectedStatus: looppkg.StatusRunning, Actor: actor, StoppedAt: stoppedAt,
			})
			if err != nil {
				t.Fatalf("StopGoalRun() error = %v", err)
			}
			if len(result.RevokedPromptLeases) != 1 {
				t.Fatalf("revoked prompt leases = %#v, want one", result.RevokedPromptLeases)
			}
			lease := result.RevokedPromptLeases[0]
			if lease.QueueEntryID != ticket.QueueEntryID || lease.SessionID != "session-goal-runtime" ||
				lease.OwnerKind != "goal" || lease.LoopRunID != string(key.LoopRunID) ||
				lease.TaskRunID != taskRunID || lease.RunGeneration != key.Generation ||
				lease.ControlEpoch != 1 || lease.BindingEpoch != 1 || lease.PromptID != promptID ||
				lease.PromptKind != "work" {
				t.Fatalf("revoked prompt lease = %#v", lease)
			}
			run, err := globalDB.GetLoopRun(ctx, key.WorkspaceID, key.LoopRunID)
			if err != nil {
				t.Fatalf("GetLoopRun() error = %v", err)
			}
			if run.Status != looppkg.StatusFailed {
				t.Fatalf("stopped Run status = %q, want failed", run.Status)
			}
			checkpoint, err := globalDB.LoadCheckpoint(ctx, key)
			if err != nil {
				t.Fatalf("LoadCheckpoint() error = %v", err)
			}
			if checkpoint.ControlEpoch != 2 || checkpoint.Phase != "terminal" || checkpoint.Status != "paused" ||
				checkpoint.ControlActorKind != string(actor.Actor.Kind.Normalize()) ||
				checkpoint.ControlActorID != actor.Actor.Ref {
				t.Fatalf("stopped checkpoint = %#v", checkpoint)
			}
			assertGoalRuntimeTurn(t, globalDB, key, promptID, "ambiguous", "")
			events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
				WorkspaceID: key.WorkspaceID,
				RunID:       key.LoopRunID,
			})
			if err != nil {
				t.Fatalf("ListLoopRunEvents() error = %v", err)
			}
			statusEvent := loopEventPayloadForKind(t, events, loopRunEventGoalStatusChanged)
			if statusEvent["actor_kind"] != string(actor.Actor.Kind.Normalize()) ||
				statusEvent["actor_id"] != actor.Actor.Ref ||
				statusEvent["cause"] != string(looppkg.ReasonCodeGoalControlRevokedInFlight) {
				t.Fatalf("stopped Goal status event = %#v", statusEvent)
			}
			if err := globalDB.FinalizeGoalPrompt(ctx, goal.FinalizePromptRequest{
				Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1,
				TaskRunID: taskRunID, QueueEntryID: ticket.QueueEntryID, PromptID: promptID,
				SessionID: "session-goal-runtime", BindingHandle: "goal:runtime",
				DispatchToken: claimed.DispatchToken,
				Result: looppkg.ActionPromptResult{
					PromptID: promptID, Outcome: looppkg.ActionPromptOutcomeCompleted,
					StopReason: looppkg.ActionStopEndTurn,
				},
				TerminalAt: stoppedAt.Add(time.Second),
			}); err == nil {
				t.Fatal("FinalizeGoalPrompt(after operator Stop) error = nil")
			}

			var eventsBefore, turnsBefore int
			if err := globalDB.db.QueryRowContext(
				ctx,
				`SELECT COUNT(*) FROM loop_run_events WHERE loop_run_id = ?`,
				string(key.LoopRunID),
			).Scan(&eventsBefore); err != nil {
				t.Fatalf("count events before Stop replay error = %v", err)
			}
			if err := globalDB.db.QueryRowContext(
				ctx,
				`SELECT COUNT(*) FROM loop_goal_turns WHERE loop_run_id = ?`,
				string(key.LoopRunID),
			).Scan(&turnsBefore); err != nil {
				t.Fatalf("count turns before Stop replay error = %v", err)
			}

			const replayCount = 8
			replayErrors := make(chan error, replayCount)
			var replayWG sync.WaitGroup
			for range replayCount {
				replayWG.Add(1)
				go func() {
					defer replayWG.Done()
					_, replayErr := globalDB.StopGoalRun(ctx, looppkg.GoalRunStopRequest{
						WorkspaceID:    key.WorkspaceID,
						RunID:          key.LoopRunID,
						ExpectedStatus: looppkg.StatusRunning,
						Actor:          actor,
						StoppedAt:      stoppedAt.Add(2 * time.Second),
					})
					replayErrors <- replayErr
				}()
			}
			replayWG.Wait()
			close(replayErrors)
			for replayErr := range replayErrors {
				if replayErr == nil {
					t.Fatal("StopGoalRun(concurrent replay) error = nil")
				}
			}

			var eventsAfter, turnsAfter int
			if err := globalDB.db.QueryRowContext(
				ctx,
				`SELECT COUNT(*) FROM loop_run_events WHERE loop_run_id = ?`,
				string(key.LoopRunID),
			).Scan(&eventsAfter); err != nil {
				t.Fatalf("count events after Stop replay error = %v", err)
			}
			if err := globalDB.db.QueryRowContext(
				ctx,
				`SELECT COUNT(*) FROM loop_goal_turns WHERE loop_run_id = ?`,
				string(key.LoopRunID),
			).Scan(&turnsAfter); err != nil {
				t.Fatalf("count turns after Stop replay error = %v", err)
			}
			if eventsAfter != eventsBefore || turnsAfter != turnsBefore {
				t.Fatalf(
					"concurrent Stop replay changed events/turns from %d/%d to %d/%d",
					eventsBefore,
					turnsBefore,
					eventsAfter,
					turnsAfter,
				)
			}
		},
	)

	t.Run("Should co-commit a clear tombstone and an unbound session projection", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-revoke-clear")
		ctx := testutil.Context(t)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET origin_kind = 'session', origin_session_id = ? WHERE id = ?`,
			"session-goal-runtime",
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("set clear session origin error = %v", err)
		}
		promptID := "goal-prompt-revoke-clear"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		if _, err := globalDB.RevokeGoalPrompt(ctx, goal.RevokePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Disposition:          looppkg.ActionDispositionPaused,
			Status:               "paused",
			Cause:                looppkg.ReasonCodeGoalControlRevokedInFlight,
			ActorKind:            "user",
			ActorID:              "operator:clear",
			ProjectionCause:      goal.SessionOutboxCauseClear,
			RevokedAt:            now.Add(time.Second),
		}); err != nil {
			t.Fatalf("RevokeGoalPrompt(clear) error = %v", err)
		}
		var clearedAt *string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT goal_cleared_at FROM loop_runs WHERE id = ?`,
			string(key.LoopRunID),
		).Scan(&clearedAt); err != nil {
			t.Fatalf("load Goal clear tombstone error = %v", err)
		}
		if clearedAt == nil {
			t.Fatal("Goal clear tombstone = nil")
		}
		pending, err := globalDB.ClaimGoalSessionOutbox(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionOutbox(clear) error = %v", err)
		}
		if len(pending) != 1 || pending[0].Cause != goal.SessionOutboxCauseClear ||
			pending[0].BoundSessionID != nil {
			t.Fatalf("clear outbox = %#v, want one unbound clear projection", pending)
		}
	})

	t.Run("Should let control terminalize one already claimed turn and reject late evidence", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-revoke-claimed")
		ctx := testutil.Context(t)
		promptID := "goal-prompt-revoke-claimed"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		decision := flushGoalRuntimeBudget(
			t,
			globalDB,
			key,
			taskRunID,
			goal.BudgetBeforeWork,
			promptID,
			0,
		)
		claimed, err := globalDB.ClaimPreparedWorkPrompt(ctx, goal.ClaimPreparedWorkPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			BudgetDecision:       decision,
			ActorKind:            "daemon",
			ActorID:              "loop-action",
		})
		if err != nil {
			t.Fatalf("ClaimPreparedWorkPrompt() error = %v", err)
		}
		revokeRequest := goal.RevokePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Disposition:          looppkg.ActionDispositionPaused,
			Status:               "paused",
			Cause:                looppkg.ReasonCodeGoalControlRevokedInFlight,
			ActorKind:            "user",
			ActorID:              "operator:clear",
			ProjectionCause:      goal.SessionOutboxCauseStatus,
			RevokedAt:            now.Add(time.Second),
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`CREATE TRIGGER fail_goal_revoke_event
			 BEFORE INSERT ON loop_run_events
			 WHEN NEW.kind = 'goal_turn_completed'
			 BEGIN SELECT RAISE(ABORT, 'forced Goal revoke event failure'); END`,
		); err != nil {
			t.Fatalf("create Goal revoke failure trigger error = %v", err)
		}
		if _, err := globalDB.RevokeGoalPrompt(ctx, revokeRequest); err == nil {
			t.Fatal("RevokeGoalPrompt(forced event failure) error = nil")
		}
		checkpointAfterRollback, err := globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint(after revoke rollback) error = %v", err)
		}
		if checkpointAfterRollback.ControlEpoch != 1 || checkpointAfterRollback.Phase != "prompting" ||
			checkpointAfterRollback.Status != "active" {
			t.Fatalf("checkpoint after revoke rollback = %#v, want epoch 1 prompting active", checkpointAfterRollback)
		}
		bindingAfterRollback, err := globalDB.GetActiveSessionBinding(ctx, goal.BindingKey{
			WorkspaceID: key.WorkspaceID,
			LoopRunID:   key.LoopRunID,
			Handle:      "goal:runtime",
		})
		if err != nil || bindingAfterRollback.BindingEpoch != 1 {
			t.Fatalf("active binding after revoke rollback = %#v, %v", bindingAfterRollback, err)
		}
		var rollbackResult *string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT result_status FROM loop_goal_turns WHERE loop_run_id = ? AND prompt_id = ?`,
			string(key.LoopRunID),
			promptID,
		).Scan(&rollbackResult); err != nil {
			t.Fatalf("load Goal turn after revoke rollback error = %v", err)
		}
		if rollbackResult != nil {
			t.Fatalf("Goal turn after revoke rollback = %v, want open", rollbackResult)
		}
		if _, err := globalDB.db.ExecContext(ctx, `DROP TRIGGER fail_goal_revoke_event`); err != nil {
			t.Fatalf("drop Goal revoke failure trigger error = %v", err)
		}

		revoked, err := globalDB.RevokeGoalPrompt(ctx, revokeRequest)
		if err != nil {
			t.Fatalf("RevokeGoalPrompt(claimed) error = %v", err)
		}
		if revoked.ControlEpoch != 2 || revoked.Phase != "terminal" || revoked.Status != "paused" {
			t.Fatalf("revoked claimed checkpoint = %#v, want epoch 2 terminal paused", revoked)
		}
		assertGoalRuntimeTurn(t, globalDB, key, promptID, "ambiguous", "")
		assertGoalRuntimeEventCount(t, globalDB, key.LoopRunID, loopRunEventGoalTurnCompleted, 1)
		lateAttachment := goal.DriverAttachmentRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			DriverTurnID:         promptID,
			AttachedAt:           now.Add(2 * time.Second),
		}
		if err := globalDB.RecordGoalDriverAttached(ctx, lateAttachment); err == nil {
			t.Fatal("RecordGoalDriverAttached(after revoke) error = nil")
		}
		if err := globalDB.FinalizeGoalPrompt(ctx, goal.FinalizePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			Result: looppkg.ActionPromptResult{
				PromptID:   promptID,
				Outcome:    looppkg.ActionPromptOutcomeCompleted,
				StopReason: looppkg.ActionStopEndTurn,
			},
			TerminalAt: now.Add(3 * time.Second),
		}); err == nil {
			t.Fatal("FinalizeGoalPrompt(after revoke) error = nil")
		}
	})

	t.Run("Should preserve an exact queue terminal when control revokes before settlement", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-revoke-terminal")
		ctx := testutil.Context(t)
		promptID := "goal-prompt-revoke-terminal"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		decision := flushGoalRuntimeBudget(
			t, globalDB, key, taskRunID, goal.BudgetBeforeWork, promptID, 0,
		)
		claimed, err := globalDB.ClaimPreparedWorkPrompt(ctx, goal.ClaimPreparedWorkPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			BudgetDecision:       decision,
			ActorKind:            "daemon",
			ActorID:              "loop-action",
		})
		if err != nil {
			t.Fatalf("ClaimPreparedWorkPrompt() error = %v", err)
		}
		if err := globalDB.RecordGoalDriverAttached(ctx, goal.DriverAttachmentRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			DriverTurnID:         promptID,
			AttachedAt:           now.Add(time.Second),
		}); err != nil {
			t.Fatalf("RecordGoalDriverAttached() error = %v", err)
		}
		terminalAt := now.Add(2 * time.Second)
		terminal := looppkg.ActionPromptResult{
			PromptID:       promptID,
			Outcome:        looppkg.ActionPromptOutcomeCompleted,
			StopReason:     looppkg.ActionStopMaxTokens,
			EventStartSeq:  10,
			EventEndSeq:    12,
			TokensUsed:     7,
			TokensReported: true,
		}
		if err := globalDB.FinalizeGoalPrompt(ctx, goal.FinalizePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			Result:               terminal,
			TerminalAt:           terminalAt,
		}); err != nil {
			t.Fatalf("FinalizeGoalPrompt() error = %v", err)
		}
		revoked, err := globalDB.RevokeGoalPrompt(ctx, goal.RevokePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Disposition:          looppkg.ActionDispositionPaused,
			Status:               "paused",
			Cause:                looppkg.ReasonCodeGoalControlRevokedInFlight,
			ActorKind:            "user",
			ActorID:              "operator:replace",
			ProjectionCause:      goal.SessionOutboxCauseReplace,
			RevokedAt:            now.Add(3 * time.Second),
		})
		if err != nil {
			t.Fatalf("RevokeGoalPrompt(exact terminal) error = %v", err)
		}
		if revoked.ControlEpoch != 2 || revoked.Status != "paused" || revoked.Phase != "terminal" {
			t.Fatalf("revoked exact-terminal checkpoint = %#v", revoked)
		}
		var resultStatus string
		var stopReason string
		var reasonCode, verdictOutcome *string
		var tokensUsed int64
		var endedAt string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT result_status, stop_reason, reason_code, verdict_outcome, tokens_used, ended_at
			 FROM loop_goal_turns WHERE loop_run_id = ? AND prompt_id = ?`,
			string(key.LoopRunID),
			promptID,
		).Scan(
			&resultStatus,
			&stopReason,
			&reasonCode,
			&verdictOutcome,
			&tokensUsed,
			&endedAt,
		); err != nil {
			t.Fatalf("load exact revoked turn terminal error = %v", err)
		}
		parsedEndedAt, err := parseGoalTimestampValue(endedAt, "exact revoked turn ended_at")
		if err != nil {
			t.Fatalf("parse exact revoked turn ended_at error = %v", err)
		}
		if resultStatus != string(looppkg.ActionPromptOutcomeCompleted) ||
			stopReason != string(looppkg.ActionStopMaxTokens) || reasonCode != nil ||
			verdictOutcome != nil || tokensUsed != 7 || !parsedEndedAt.Equal(terminalAt) {
			t.Fatalf(
				"exact revoked turn = result:%q stop:%q reason:%v verdict:%v tokens:%d ended:%s",
				resultStatus,
				stopReason,
				reasonCode,
				verdictOutcome,
				tokensUsed,
				parsedEndedAt,
			)
		}
		var eventStartSeq, eventEndSeq int64
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT terminal_event_start_seq, terminal_event_end_seq
			 FROM session_input_queue WHERE id = ?`,
			ticket.QueueEntryID,
		).Scan(&eventStartSeq, &eventEndSeq); err != nil {
			t.Fatalf("load exact revoked queue event range error = %v", err)
		}
		if eventStartSeq != 10 || eventEndSeq != 12 {
			t.Fatalf("exact revoked queue event range = %d..%d", eventStartSeq, eventEndSeq)
		}
	})

	t.Run("Should finalize a claimed operation as ambiguous once without another prompt", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-ambiguous")
		promptID := "goal-prompt-ambiguous"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		decision := flushGoalRuntimeBudget(t, globalDB, key, taskRunID, goal.BudgetBeforeWork, promptID, 0)
		claimed, err := globalDB.ClaimPreparedWorkPrompt(testutil.Context(t), goal.ClaimPreparedWorkPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			BudgetDecision:       decision,
			ActorKind:            "daemon",
			ActorID:              "loop-action",
		})
		if err != nil || claimed.DispatchToken == "" {
			t.Fatalf("ClaimPreparedWorkPrompt() = %#v, %v", claimed, err)
		}
		ambiguous := goal.AmbiguousRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Cause:                looppkg.ReasonCodeGoalRecoveryAmbiguous,
		}
		if err := globalDB.MarkAmbiguous(testutil.Context(t), ambiguous); err != nil {
			t.Fatalf("MarkAmbiguous(first) error = %v", err)
		}
		if err := globalDB.MarkAmbiguous(testutil.Context(t), ambiguous); err != nil {
			t.Fatalf("MarkAmbiguous(second) error = %v", err)
		}
		checkpoint, err := globalDB.LoadCheckpoint(testutil.Context(t), key)
		if err != nil {
			t.Fatalf("LoadCheckpoint() error = %v", err)
		}
		if checkpoint.Status != "paused" || checkpoint.Phase != "awaiting_control" ||
			checkpoint.ControlCause != looppkg.ReasonCodeGoalRecoveryAmbiguous {
			t.Fatalf("ambiguous checkpoint = %#v", checkpoint)
		}
		assertGoalRuntimeTurn(t, globalDB, key, promptID, "ambiguous", "")
		var count int
		if err := globalDB.db.QueryRowContext(
			testutil.Context(t),
			`SELECT COUNT(*) FROM loop_goal_turns WHERE loop_run_id = ?`,
			string(key.LoopRunID),
		).Scan(&count); err != nil {
			t.Fatalf("count Goal turns error = %v", err)
		}
		if count != 1 {
			t.Fatalf("Goal turns = %d, want 1", count)
		}
		assertGoalRuntimeEventCount(
			t,
			globalDB,
			key.LoopRunID,
			loopRunEventGoalTurnCompleted,
			1,
		)
	})

	t.Run("Should co-commit a session-origin status event and outbox row", func(t *testing.T) {
		t.Parallel()

		globalDB, key, _, now := seedGoalTurnRuntime(t, "run-goal-status-outbox")
		ctx := testutil.Context(t)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs
			 SET origin_kind = 'session', origin_session_id = ?
			 WHERE id = ? AND workspace_id = ?`,
			"session-goal-runtime",
			string(key.LoopRunID),
			string(key.WorkspaceID),
		); err != nil {
			t.Fatalf("set session-origin Goal run error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`CREATE TRIGGER fail_goal_status_outbox
			 BEFORE INSERT ON loop_goal_session_outbox
			 BEGIN SELECT RAISE(ABORT, 'forced goal outbox failure'); END`,
		); err != nil {
			t.Fatalf("create outbox failure trigger error = %v", err)
		}
		checkpoint, err := globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint(before status control) error = %v", err)
		}
		request := goal.ControlCheckpointRequest{
			Key:                  key,
			ExpectedControlEpoch: checkpoint.ControlEpoch,
			ExpectedBindingEpoch: checkpoint.BindingEpoch,
			ExpectedPhase:        checkpoint.Phase,
			TaskRunID:            checkpoint.TaskRunID,
			QueueEntryID:         checkpoint.QueueEntryID,
			PromptID:             checkpoint.PromptID,
			Disposition:          looppkg.ActionDispositionPaused,
			Status:               "paused",
			Cause:                looppkg.ReasonCodeGoalBudgetFenced,
			ActorKind:            "human",
			ActorID:              "operator:status",
		}
		if err := globalDB.CheckpointControl(ctx, request); err == nil {
			t.Fatal("CheckpointControl(forced outbox failure) error = nil")
		}
		checkpoint, err = globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint(after rollback) error = %v", err)
		}
		if checkpoint.Status != "active" || checkpoint.Phase != "idle" {
			t.Fatalf("checkpoint after outbox rollback = %#v, want active/idle", checkpoint)
		}
		assertGoalRuntimeEventCount(t, globalDB, key.LoopRunID, loopRunEventGoalStatusChanged, 0)

		if _, err := globalDB.db.ExecContext(ctx, `DROP TRIGGER fail_goal_status_outbox`); err != nil {
			t.Fatalf("drop outbox failure trigger error = %v", err)
		}
		globalDB.now = func() time.Time { return now.Add(time.Minute) }
		if err := globalDB.CheckpointControl(ctx, request); err != nil {
			t.Fatalf("CheckpointControl() error = %v", err)
		}
		assertGoalRuntimeEventCount(t, globalDB, key.LoopRunID, loopRunEventGoalStatusChanged, 1)
		pending, err := globalDB.ClaimGoalSessionOutbox(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionOutbox() error = %v", err)
		}
		if len(pending) != 1 || pending[0].Cause != goal.SessionOutboxCauseStatus ||
			pending[0].OriginSessionID != "session-goal-runtime" ||
			pending[0].BoundSessionID == nil || *pending[0].BoundSessionID != "session-goal-runtime" {
			t.Fatalf("status outbox = %#v, want one session-origin status projection", pending)
		}
	})

	t.Run("Should settle an in-flight turn with separate dispatch and pause actors", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-pause-boundary")
		ctx := testutil.Context(t)
		promptID := "goal-prompt-pause-boundary"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		decision := flushGoalRuntimeBudget(t, globalDB, key, taskRunID, goal.BudgetBeforeWork, promptID, 0)
		claimed, err := globalDB.ClaimPreparedWorkPrompt(ctx, goal.ClaimPreparedWorkPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			BudgetDecision:       decision,
			ActorKind:            "daemon",
			ActorID:              "loop-action",
		})
		if err != nil {
			t.Fatalf("ClaimPreparedWorkPrompt() error = %v", err)
		}
		pauseActor := operatorActorContextForTest("user:pause-operator")
		if err := globalDB.SetLoopRunPauseRequested(
			ctx,
			key.WorkspaceID,
			key.LoopRunID,
			true,
			pauseActor,
		); err != nil {
			t.Fatalf("SetLoopRunPauseRequested() error = %v", err)
		}
		checkpoint, err := globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint(pause intent) error = %v", err)
		}
		if checkpoint.ControlActorKind != string(pauseActor.Actor.Kind.Normalize()) ||
			checkpoint.ControlActorID != pauseActor.Actor.Ref {
			t.Fatalf("checkpoint pause actor = %q/%q", checkpoint.ControlActorKind, checkpoint.ControlActorID)
		}
		terminal := looppkg.ActionPromptResult{
			PromptID:   promptID,
			Outcome:    looppkg.ActionPromptOutcomeCompleted,
			StopReason: looppkg.ActionStopEndTurn,
		}
		if err := globalDB.FinalizeGoalPrompt(ctx, goal.FinalizePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			Result:               terminal,
			TerminalAt:           now.Add(time.Second),
		}); err != nil {
			t.Fatalf("FinalizeGoalPrompt() error = %v", err)
		}
		judgeDecision := flushGoalRuntimeBudget(
			t,
			globalDB,
			key,
			taskRunID,
			goal.BudgetBeforeJudge,
			"judge-after-pause",
			0,
		)
		if _, err := globalDB.BeginJudgeAttempt(ctx, goal.BeginJudgeAttemptRequest{
			AttemptID:            "judge-after-pause",
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			PromptID:             promptID,
			Turn:                 1,
			JudgeDigest:          "judge-after-pause-digest",
			BudgetDecision:       judgeDecision,
		}); err == nil {
			t.Fatal("BeginJudgeAttempt(after Pause) error = nil")
		} else {
			requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalPromptFenced)
		}
		settled, err := globalDB.CompleteTurn(ctx, goal.CompleteTurnRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Result:               terminal,
			DispatchActorKind:    "daemon",
			DispatchActorID:      "loop-action",
		})
		if err != nil {
			t.Fatalf("CompleteTurn(pause boundary) error = %v", err)
		}
		if settled.Phase != "awaiting_control" || settled.Status != "paused" ||
			settled.ControlCause != looppkg.ReasonCode(looppkg.TransitionCausePauseBoundary) ||
			settled.ControlActorKind != string(pauseActor.Actor.Kind.Normalize()) ||
			settled.ControlActorID != pauseActor.Actor.Ref {
			t.Fatalf("settled pause checkpoint = %#v", settled)
		}
		var turnActorKind, turnActorID string
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT actor_kind, actor_id FROM loop_goal_turns WHERE loop_run_id = ? AND prompt_id = ?`,
			string(key.LoopRunID),
			promptID,
		).Scan(&turnActorKind, &turnActorID); err != nil {
			t.Fatalf("query paused Goal turn actor error = %v", err)
		}
		if turnActorKind != "daemon" || turnActorID != "loop-action" {
			t.Fatalf("paused turn actor = %q/%q, want dispatch actor", turnActorKind, turnActorID)
		}
		events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			WorkspaceID: key.WorkspaceID,
			RunID:       key.LoopRunID,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents() error = %v", err)
		}
		turnEvent := loopEventPayloadForKind(t, events, loopRunEventGoalTurnCompleted)
		statusEvent := loopEventPayloadForKind(t, events, loopRunEventGoalStatusChanged)
		if turnEvent["actor_kind"] != "daemon" || turnEvent["actor_id"] != "loop-action" {
			t.Fatalf("turn event actor = %#v", turnEvent)
		}
		if statusEvent["actor_kind"] != string(pauseActor.Actor.Kind.Normalize()) ||
			statusEvent["actor_id"] != pauseActor.Actor.Ref {
			t.Fatalf("status event actor = %#v", statusEvent)
		}
	})

	t.Run("Should settle one winning checkpoint when pause report completion and budget race", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-boundary-race")
		ctx := testutil.Context(t)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET budget_tokens = 5, budget_on_exceeded = 'halt' WHERE id = ?`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("configure raced Goal budget error = %v", err)
		}
		promptID := "goal-prompt-boundary-race"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		initialBudget := flushGoalRuntimeBudget(
			t,
			globalDB,
			key,
			taskRunID,
			goal.BudgetBeforeWork,
			promptID,
			0,
		)
		claimed, err := globalDB.ClaimPreparedWorkPrompt(ctx, goal.ClaimPreparedWorkPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			BudgetDecision:       initialBudget,
			ActorKind:            "daemon",
			ActorID:              "loop-action",
		})
		if err != nil {
			t.Fatalf("ClaimPreparedWorkPrompt() error = %v", err)
		}
		reportReq := goal.RecordReportIntentRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			PromptID:             promptID,
			Status:               "blocked",
			EvidenceRef:          "evidence:boundary-race",
			ActorKind:            "agent",
			ActorID:              "goal-reporter",
		}
		if _, err := globalDB.RecordReportIntent(ctx, reportReq); err != nil {
			t.Fatalf("RecordReportIntent(seed) error = %v", err)
		}
		terminal := looppkg.ActionPromptResult{
			PromptID:       promptID,
			Outcome:        looppkg.ActionPromptOutcomeCompleted,
			StopReason:     looppkg.ActionStopEndTurn,
			TokensUsed:     5,
			TokensReported: true,
		}
		pauseActor := operatorActorContextForTest("user:boundary-race")
		budgetSnapshot := goalBudgetSnapshotForTest(t, globalDB, goal.BudgetBoundarySnapshot{
			Key:                 key,
			TaskRunID:           taskRunID,
			Boundary:            goal.BudgetAfterWork,
			Phase:               "boundary-race",
			Turn:                1,
			OperationID:         promptID,
			OperationBaseTokens: 0,
			LiveTokensUsed:      5,
			TokensReported:      true,
		})
		type boundaryRaceResult struct {
			kind     string
			decision goal.BudgetDecision
			err      error
		}
		const contendersPerKind = 4
		start := make(chan struct{})
		results := make(chan boundaryRaceResult, contendersPerKind*4)
		var wait sync.WaitGroup
		for range contendersPerKind {
			wait.Add(4)
			go func() {
				defer wait.Done()
				<-start
				err := globalDB.SetLoopRunPauseRequested(
					context.Background(), key.WorkspaceID, key.LoopRunID, true, pauseActor,
				)
				results <- boundaryRaceResult{kind: "pause", err: err}
			}()
			go func() {
				defer wait.Done()
				<-start
				_, err := globalDB.RecordReportIntent(context.Background(), reportReq)
				results <- boundaryRaceResult{kind: "report", err: err}
			}()
			go func() {
				defer wait.Done()
				<-start
				err := globalDB.FinalizeGoalPrompt(context.Background(), goal.FinalizePromptRequest{
					Key:                  key,
					ExpectedControlEpoch: 1,
					ExpectedBindingEpoch: 1,
					TaskRunID:            taskRunID,
					QueueEntryID:         ticket.QueueEntryID,
					PromptID:             promptID,
					SessionID:            "session-goal-runtime",
					BindingHandle:        "goal:runtime",
					DispatchToken:        claimed.DispatchToken,
					Result:               terminal,
					TerminalAt:           now.Add(time.Second),
				})
				results <- boundaryRaceResult{kind: "completion", err: err}
			}()
			go func() {
				defer wait.Done()
				<-start
				decision, err := globalDB.FlushAndCheck(context.Background(), budgetSnapshot)
				results <- boundaryRaceResult{kind: "budget", decision: decision, err: err}
			}()
		}
		close(start)
		wait.Wait()
		close(results)
		for result := range results {
			switch result.kind {
			case "pause", "completion":
				if result.err != nil {
					t.Errorf("%s contender error = %v", result.kind, result.err)
				}
			case "report":
				if result.err != nil {
					requireGoalReasonCode(t, result.err, looppkg.ReasonCodeGoalNotActive)
				}
			case "budget":
				if result.err != nil {
					requireGoalReasonCode(t, result.err, looppkg.ReasonCodeGoalControlStale)
					continue
				}
				if result.decision.Allowed || result.decision.Disposition != looppkg.ActionDispositionExhausted {
					t.Errorf("budget contender = %#v, %v", result.decision, result.err)
				}
			default:
				t.Errorf("unexpected boundary race result kind %q", result.kind)
			}
		}

		settleReq := goal.CompleteTurnRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Result:               terminal,
			DispatchActorKind:    "daemon",
			DispatchActorID:      "loop-action",
		}
		const settlementContenders = 8
		settlementErrors := make(chan error, settlementContenders)
		wait.Add(settlementContenders)
		for range settlementContenders {
			go func() {
				defer wait.Done()
				_, err := globalDB.CompleteTurn(context.Background(), settleReq)
				settlementErrors <- err
			}()
		}
		wait.Wait()
		close(settlementErrors)
		for err := range settlementErrors {
			if err != nil {
				t.Errorf("CompleteTurn(race) error = %v", err)
			}
		}
		checkpoint, err := globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint(boundary race) error = %v", err)
		}
		if checkpoint.ControlEpoch != 1 || checkpoint.Phase != "terminal" ||
			checkpoint.Status != "blocked" ||
			checkpoint.ControlCause != looppkg.ReasonCodeGoalReportedBlocked ||
			checkpoint.ReportIntent != nil {
			t.Fatalf("winning boundary checkpoint = %#v", checkpoint)
		}
		var pauseRequested int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT pause_requested FROM loop_runs WHERE id = ?`,
			string(key.LoopRunID),
		).Scan(&pauseRequested); err != nil {
			t.Fatalf("query raced pause state error = %v", err)
		}
		if pauseRequested != 0 {
			t.Fatalf("pause_requested = %d, want losing Pause cleared", pauseRequested)
		}
		assertGoalRuntimeEventCount(t, globalDB, key.LoopRunID, loopRunEventGoalTurnCompleted, 1)
		assertGoalRuntimeEventCount(t, globalDB, key.LoopRunID, loopRunEventGoalStatusChanged, 1)
		events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			WorkspaceID: key.WorkspaceID,
			RunID:       key.LoopRunID,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents(boundary race) error = %v", err)
		}
		turnEvent := loopEventPayloadForKind(t, events, loopRunEventGoalTurnCompleted)
		statusEvent := loopEventPayloadForKind(t, events, loopRunEventGoalStatusChanged)
		if turnEvent["actor_kind"] != "daemon" || turnEvent["actor_id"] != "loop-action" {
			t.Fatalf("raced turn event actor = %#v", turnEvent)
		}
		if statusEvent["actor_kind"] != reportReq.ActorKind ||
			statusEvent["actor_id"] != reportReq.ActorID {
			t.Fatalf("raced status event actor = %#v", statusEvent)
		}
	})

	t.Run("Should allow only one concurrent claim for a prepared logical prompt", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-claim-race")
		promptID := "goal-prompt-race"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		decision := flushGoalRuntimeBudget(t, globalDB, key, taskRunID, goal.BudgetBeforeWork, promptID, 0)
		const workers = 8
		var wg sync.WaitGroup
		var mu sync.Mutex
		successes := 0
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := globalDB.ClaimPreparedWorkPrompt(context.Background(), goal.ClaimPreparedWorkPromptRequest{
					Key:                  key,
					ExpectedControlEpoch: 1,
					TaskRunID:            taskRunID,
					QueueEntryID:         ticket.QueueEntryID,
					PromptID:             promptID,
					SessionID:            "session-goal-runtime",
					BindingHandle:        "goal:runtime",
					BindingEpoch:         1,
					BudgetDecision:       decision,
					ActorKind:            "daemon",
					ActorID:              "loop-action",
				})
				if err == nil {
					mu.Lock()
					successes++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if successes != 1 {
			t.Fatalf("successful claims = %d, want 1", successes)
		}
		var count int
		if err := globalDB.db.QueryRowContext(
			testutil.Context(t),
			`SELECT COUNT(*) FROM loop_goal_turns WHERE loop_run_id = ?`,
			string(key.LoopRunID),
		).Scan(&count); err != nil {
			t.Fatalf("count raced Goal turns error = %v", err)
		}
		if count != 1 {
			t.Fatalf("raced Goal turns = %d, want 1", count)
		}
	})

	t.Run("Should reject a stale ready-session budget decision before allocating a turn", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-stale-ready-budget")
		ctx := testutil.Context(t)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET budget_tokens = 1, budget_on_exceeded = 'halt' WHERE id = ?`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("configure stale ready budget error = %v", err)
		}
		promptID := "goal-prompt-stale-ready-budget"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		allowed := flushGoalRuntimeBudget(
			t,
			globalDB,
			key,
			taskRunID,
			goal.BudgetBeforeWork,
			promptID,
			0,
		)
		if !allowed.Allowed {
			t.Fatalf("initial budget decision = %#v, want allowed", allowed)
		}
		crossed := flushGoalRuntimeBudget(
			t,
			globalDB,
			key,
			taskRunID,
			goal.BudgetBeforeWork,
			promptID,
			1,
		)
		if crossed.Allowed || crossed.Disposition != looppkg.ActionDispositionExhausted {
			t.Fatalf("crossed budget decision = %#v", crossed)
		}
		_, err := globalDB.ClaimPreparedWorkPrompt(ctx, goal.ClaimPreparedWorkPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			BudgetDecision:       allowed,
			ActorKind:            "daemon",
			ActorID:              "loop-action",
		})
		if err == nil {
			t.Fatal("ClaimPreparedWorkPrompt(stale budget) error = nil")
		}
		requireGoalReasonCode(t, err, looppkg.ReasonCodeGoalPromptFenced)
		var turnCount int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM loop_goal_turns WHERE loop_run_id = ?`,
			string(key.LoopRunID),
		).Scan(&turnCount); err != nil {
			t.Fatalf("count stale-budget Goal turns error = %v", err)
		}
		if turnCount != 0 {
			t.Fatalf("stale-budget Goal turns = %d, want none", turnCount)
		}
		var status string
		var dispatchable int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT status, dispatchable FROM session_input_queue WHERE id = ?`,
			ticket.QueueEntryID,
		).Scan(&status, &dispatchable); err != nil {
			t.Fatalf("query stale-budget queue entry error = %v", err)
		}
		if status != store.SessionInputQueueStatusQueued || dispatchable != 0 {
			t.Fatalf("stale-budget queue state = %q/%d, want queued/non-dispatchable", status, dispatchable)
		}
	})

	t.Run("Should bind a pre-submit budget approval to one work turn and consume it at settlement", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-budget-reentry")
		ctx := testutil.Context(t)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs
			 SET budget_tokens = 1, budget_on_exceeded = 'escalate'
			 WHERE id = ?`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("configure Goal budget error = %v", err)
		}
		metadata := `{"generation":1,"node_id":"converge","item_index":0,"goal_segment_epoch":1}`
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE task_runs SET tokens_used = 1, metadata_json = ? WHERE id = ?`,
			metadata,
			taskRunID,
		); err != nil {
			t.Fatalf("configure initial Goal segment error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_goal_checkpoints SET turns_used = 2 WHERE loop_run_id = ?`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("configure Goal turns error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status, task_run_id,
				goal_status, goal_turns_used, goal_turn_limit
			) VALUES (?, 1, 'converge', 0, 'enqueued', ?, 'active', 2, 3)`,
			string(key.LoopRunID),
			taskRunID,
		); err != nil {
			t.Fatalf("insert Goal budget output error = %v", err)
		}
		promptID := "goal-prompt-budget-reentry"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		denied, err := globalDB.FlushAndCheck(ctx, goalBudgetSnapshotForTest(t, globalDB, goal.BudgetBoundarySnapshot{
			Key:                 key,
			TaskRunID:           taskRunID,
			Boundary:            goal.BudgetBeforeWork,
			Phase:               "pre-submit",
			Turn:                3,
			OperationID:         promptID,
			OperationBaseTokens: 1,
			LiveTokensUsed:      1,
			TokensReported:      true,
		}))
		if err != nil || denied.Allowed || denied.Disposition != looppkg.ActionDispositionNeedsApproval {
			t.Fatalf("FlushAndCheck(denied) = %#v, %v", denied, err)
		}
		if err := globalDB.FencePreparedPrompt(ctx, goal.FencePreparedPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 1,
			ExpectedBindingEpoch: 1,
			TaskRunID:            taskRunID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Outcome:              looppkg.ActionPromptOutcomeBudgetFenced,
			Disposition:          looppkg.ActionDispositionNeedsApproval,
			Cause:                looppkg.ReasonCodeGoalBudgetFenced,
			BudgetDecision:       denied,
		}); err != nil {
			t.Fatalf("FencePreparedPrompt() error = %v", err)
		}
		gateID := looppkg.SyntheticGoalGateID(
			key.NodeID,
			key.Generation,
			key.ItemIndex,
			looppkg.ReasonCodeGoalBudgetFenced,
		)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_generation_outputs SET status = 'awaiting_goal' WHERE loop_run_id = ?`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("settle Goal budget output error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET status = 'needs-approval', active_gate_id = ? WHERE id = ?`,
			string(gateID),
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("settle Goal budget Run error = %v", err)
		}

		state, found, err := globalDB.LoadAwaitingGoalControl(ctx, key.WorkspaceID, key.LoopRunID)
		if err != nil || !found {
			t.Fatalf("LoadAwaitingGoalControl() = %#v, %t, %v", state, found, err)
		}
		if state.OpenTurn {
			t.Fatal("pre-submit Goal fence reported an open turn")
		}
		reactivated, err := globalDB.ReactivateGoalRun(ctx, looppkg.GoalReactivationRequest{
			State: state,
			Kind:  looppkg.GoalGrantBudget,
			Scope: looppkg.GoalGrantScopeWorkAndSettle,
			Actor: operatorActorContextForTest("user:budget-approver"),
		})
		if err != nil {
			t.Fatalf("ReactivateGoalRun() error = %v", err)
		}
		checkpoint, err := globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint(after approval) error = %v", err)
		}
		if checkpoint.Phase != "queued" || checkpoint.ControlEpoch != 2 ||
			checkpoint.ControlGrant == nil || checkpoint.ControlGrant.Consumed ||
			checkpoint.ControlGrant.Turn != 3 ||
			checkpoint.ControlGrant.Scope != goal.ControlGrantScopeWorkAndSettle {
			t.Fatalf("approved checkpoint = %#v", checkpoint)
		}
		allowed, err := globalDB.FlushAndCheck(ctx, goalBudgetSnapshotForTest(t, globalDB, goal.BudgetBoundarySnapshot{
			Key:                 key,
			TaskRunID:           reactivated.Run.ID,
			Boundary:            goal.BudgetBeforeWork,
			Phase:               "ready-slot",
			Turn:                3,
			OperationID:         promptID,
			OperationBaseTokens: 0,
			LiveTokensUsed:      0,
		}))
		if err != nil || !allowed.Allowed || allowed.GrantID != checkpoint.ControlGrant.ID {
			t.Fatalf("FlushAndCheck(approved) = %#v, %v", allowed, err)
		}
		claimed, err := globalDB.ClaimPreparedWorkPrompt(ctx, goal.ClaimPreparedWorkPromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 2,
			TaskRunID:            reactivated.Run.ID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			BindingEpoch:         1,
			BudgetDecision:       allowed,
			ActorKind:            "daemon",
			ActorID:              "loop-action",
		})
		if err != nil {
			t.Fatalf("ClaimPreparedWorkPrompt(approved) error = %v", err)
		}
		terminal := looppkg.ActionPromptResult{
			PromptID:   promptID,
			Outcome:    looppkg.ActionPromptOutcomeCompleted,
			StopReason: looppkg.ActionStopMaxTokens,
		}
		if err := globalDB.FinalizeGoalPrompt(ctx, goal.FinalizePromptRequest{
			Key:                  key,
			ExpectedControlEpoch: 2,
			ExpectedBindingEpoch: 1,
			TaskRunID:            reactivated.Run.ID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			SessionID:            "session-goal-runtime",
			BindingHandle:        "goal:runtime",
			DispatchToken:        claimed.DispatchToken,
			Result:               terminal,
			TerminalAt:           now.Add(2 * time.Second),
		}); err != nil {
			t.Fatalf("FinalizeGoalPrompt(approved) error = %v", err)
		}
		settled, err := globalDB.CompleteTurn(ctx, goal.CompleteTurnRequest{
			Key:                  key,
			ExpectedControlEpoch: 2,
			ExpectedBindingEpoch: 1,
			TaskRunID:            reactivated.Run.ID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Result:               terminal,
			DispatchActorKind:    "daemon",
			DispatchActorID:      "loop-action",
		})
		if err != nil {
			t.Fatalf("CompleteTurn(approved) error = %v", err)
		}
		if settled.ControlGrant == nil || !settled.ControlGrant.Consumed || settled.TurnsUsed != 3 {
			t.Fatalf("settled approved turn = %#v", settled)
		}
	})

	t.Run("Should bind a post-work budget approval to settlement without another work turn", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, now := seedGoalTurnRuntime(t, "run-goal-budget-settle")
		ctx := testutil.Context(t)
		metadata := `{"generation":1,"node_id":"converge","item_index":0,"goal_segment_epoch":1}`
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE task_runs SET tokens_used = 1, metadata_json = ? WHERE id = ?`,
			metadata,
			taskRunID,
		); err != nil {
			t.Fatalf("configure settling Goal segment error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs
			 SET budget_tokens = 1, budget_on_exceeded = 'escalate'
			 WHERE id = ?`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("configure settling Goal budget error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status, task_run_id,
				goal_status, goal_turns_used, goal_turn_limit
			) VALUES (?, 1, 'converge', 0, 'awaiting_goal', ?, 'budget-limited', 1, 3)`,
			string(key.LoopRunID),
			taskRunID,
		); err != nil {
			t.Fatalf("insert settling Goal output error = %v", err)
		}
		promptID := "goal-prompt-budget-settle"
		ticket := prepareGoalRuntimePrompt(t, globalDB, key, taskRunID, promptID, now)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE session_input_queue
			 SET status = 'sent', terminal_kind = 'completed', terminal_stop_reason = 'end_turn',
			     terminal_tokens_reported = 0, terminal_at = ?, updated_at = ?
			 WHERE id = ?`,
			store.FormatTimestamp(now.Add(time.Second)),
			store.FormatTimestamp(now.Add(time.Second)),
			ticket.QueueEntryID,
		); err != nil {
			t.Fatalf("terminalize settling Goal prompt error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_goal_turns (
				loop_run_id, seq, generation, node_id, item_index, turn,
				session_id, binding_handle, binding_epoch, prompt_id, prompt_attempt,
				usage_base_tokens, actor_kind, actor_id, started_at
			) VALUES (?, 1, 1, 'converge', 0, 1, 'session-goal-runtime', 'goal:runtime', 1, ?, 0, 0,
			          'daemon', 'loop-action', ?)`,
			string(key.LoopRunID),
			promptID,
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatalf("insert open settling Goal turn error = %v", err)
		}
		gateID := looppkg.SyntheticGoalGateID(
			key.NodeID,
			key.Generation,
			key.ItemIndex,
			looppkg.ReasonCodeGoalBudgetFenced,
		)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_goal_checkpoints
			 SET phase = 'awaiting_control', goal_status = 'budget-limited',
			     control_cause = 'goal_budget_fenced', turns_used = 1
			 WHERE loop_run_id = ?`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("checkpoint settling Goal control error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET status = 'needs-approval', active_gate_id = ? WHERE id = ?`,
			string(gateID),
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("settle pending Goal Run error = %v", err)
		}

		state, found, err := globalDB.LoadAwaitingGoalControl(ctx, key.WorkspaceID, key.LoopRunID)
		if err != nil || !found || !state.OpenTurn {
			t.Fatalf("LoadAwaitingGoalControl(open turn) = %#v, %t, %v", state, found, err)
		}
		reactivated, err := globalDB.ReactivateGoalRun(ctx, looppkg.GoalReactivationRequest{
			State: state,
			Kind:  looppkg.GoalGrantBudget,
			Scope: looppkg.GoalGrantScopeSettleCurrent,
			Actor: operatorActorContextForTest("user:settlement-approver"),
		})
		if err != nil {
			t.Fatalf("ReactivateGoalRun(settle-current) error = %v", err)
		}
		checkpoint, err := globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint(settle-current) error = %v", err)
		}
		if checkpoint.Phase != "judging" || checkpoint.ControlGrant == nil ||
			checkpoint.ControlGrant.Turn != 1 || checkpoint.ControlGrant.Consumed ||
			checkpoint.ControlGrant.Scope != goal.ControlGrantScopeSettleCurrent {
			t.Fatalf("settle-current checkpoint = %#v", checkpoint)
		}
		decision, err := globalDB.FlushAndCheck(ctx, goalBudgetSnapshotForTest(t, globalDB, goal.BudgetBoundarySnapshot{
			Key:                 key,
			TaskRunID:           reactivated.Run.ID,
			Boundary:            goal.BudgetBeforeJudge,
			Phase:               "judging",
			Turn:                1,
			OperationID:         "judge-budget-settle",
			OperationBaseTokens: 0,
			LiveTokensUsed:      0,
		}))
		if err != nil || !decision.Allowed || decision.GrantScope != goal.ControlGrantScopeSettleCurrent {
			t.Fatalf("FlushAndCheck(settle-current) = %#v, %v", decision, err)
		}
		attempt, err := globalDB.BeginJudgeAttempt(ctx, goal.BeginJudgeAttemptRequest{
			AttemptID:            "judge-budget-settle",
			Key:                  key,
			ExpectedControlEpoch: 2,
			ExpectedBindingEpoch: 1,
			TaskRunID:            reactivated.Run.ID,
			PromptID:             promptID,
			Turn:                 1,
			JudgeDigest:          "judge-budget-settle-digest",
			BudgetDecision:       decision,
		})
		if err != nil {
			t.Fatalf("BeginJudgeAttempt(settle-current) error = %v", err)
		}
		verdict := gate.Verdict{Outcome: gate.VerdictOutcomeRejected}
		if _, err := globalDB.CompleteJudgeAttempt(ctx, goal.CompleteJudgeAttemptRequest{
			AttemptID:            attempt.AttemptID,
			Key:                  key,
			ExpectedControlEpoch: 2,
			ExpectedBindingEpoch: 1,
			TaskRunID:            reactivated.Run.ID,
			PromptID:             promptID,
			Verdict:              verdict,
		}); err != nil {
			t.Fatalf("CompleteJudgeAttempt(settle-current) error = %v", err)
		}
		terminal := looppkg.ActionPromptResult{
			PromptID:   promptID,
			Outcome:    looppkg.ActionPromptOutcomeCompleted,
			StopReason: looppkg.ActionStopEndTurn,
		}
		settled, err := globalDB.CompleteTurn(ctx, goal.CompleteTurnRequest{
			Key:                  key,
			ExpectedControlEpoch: 2,
			ExpectedBindingEpoch: 1,
			TaskRunID:            reactivated.Run.ID,
			QueueEntryID:         ticket.QueueEntryID,
			PromptID:             promptID,
			Result:               terminal,
			Verdict:              &verdict,
			DispatchActorKind:    "daemon",
			DispatchActorID:      "loop-action",
		})
		if err != nil {
			t.Fatalf("CompleteTurn(settle-current) error = %v", err)
		}
		if settled.ControlGrant == nil || !settled.ControlGrant.Consumed || settled.TurnsUsed != 1 {
			t.Fatalf("settled current turn = %#v", settled)
		}
	})

	t.Run("Should increment a turn-exhaustion limit once under concurrent approvals", func(t *testing.T) {
		t.Parallel()

		globalDB, key, taskRunID, _ := seedGoalTurnRuntime(t, "run-goal-turn-extension")
		ctx := testutil.Context(t)
		metadata := `{"generation":1,"node_id":"converge","item_index":0,"goal_segment_epoch":1}`
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE task_runs SET metadata_json = ? WHERE id = ?`,
			metadata,
			taskRunID,
		); err != nil {
			t.Fatalf("configure exhausted Goal segment error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO loop_generation_outputs (
				loop_run_id, generation, node_id, item_index, status, task_run_id,
				goal_status, goal_turns_used, goal_turn_limit
			) VALUES (?, 1, 'converge', 0, 'awaiting_goal', ?, 'budget-limited', 3, 3)`,
			string(key.LoopRunID),
			taskRunID,
		); err != nil {
			t.Fatalf("insert exhausted Goal output error = %v", err)
		}
		gateID := looppkg.SyntheticGoalGateID(
			key.NodeID,
			key.Generation,
			key.ItemIndex,
			looppkg.ReasonCodeGoalTurnsExhausted,
		)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_goal_checkpoints
			 SET phase = 'awaiting_control', goal_status = 'budget-limited',
			     control_cause = 'goal_turns_exhausted', turns_used = 3, turn_limit = 3
			 WHERE loop_run_id = ?`,
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("checkpoint exhausted Goal error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET status = 'needs-approval', active_gate_id = ? WHERE id = ?`,
			string(gateID),
			string(key.LoopRunID),
		); err != nil {
			t.Fatalf("settle exhausted Goal Run error = %v", err)
		}
		state, found, err := globalDB.LoadAwaitingGoalControl(ctx, key.WorkspaceID, key.LoopRunID)
		if err != nil || !found {
			t.Fatalf("LoadAwaitingGoalControl(exhausted) = %#v, %t, %v", state, found, err)
		}
		req := looppkg.GoalReactivationRequest{
			State:         state,
			Kind:          looppkg.GoalGrantTurnExtension,
			Scope:         looppkg.GoalGrantScopeTurnLimit,
			TurnIncrement: 3,
			Actor:         operatorActorContextForTest("user:turn-approver"),
		}
		const contenders = 8
		results := make(chan looppkg.GoalReactivationResult, contenders)
		errorsCh := make(chan error, contenders)
		var wait sync.WaitGroup
		wait.Add(contenders)
		for range contenders {
			go func() {
				defer wait.Done()
				result, err := globalDB.ReactivateGoalRun(ctx, req)
				results <- result
				errorsCh <- err
			}()
		}
		wait.Wait()
		close(results)
		close(errorsCh)
		for err := range errorsCh {
			if err != nil {
				t.Errorf("ReactivateGoalRun(turn extension) error = %v", err)
			}
		}
		successorRunID := looppkg.GoalSegmentRunID(key.LoopRunID, 1, key.NodeID, 0, 2)
		for result := range results {
			if result.Run.ID != successorRunID || result.ControlEpoch != 2 || result.GrantID != 1 {
				t.Errorf("turn-extension result = %#v", result)
			}
		}
		checkpoint, err := globalDB.LoadCheckpoint(ctx, key)
		if err != nil {
			t.Fatalf("LoadCheckpoint(turn extension) error = %v", err)
		}
		if checkpoint.TurnLimit != 6 || checkpoint.TurnsUsed != 3 || checkpoint.ControlEpoch != 2 ||
			checkpoint.ControlGrant == nil || !checkpoint.ControlGrant.Consumed ||
			checkpoint.ControlGrant.Kind != goal.ControlGrantTurnExtension ||
			checkpoint.ControlGrant.Scope != goal.ControlGrantScopeTurnLimit ||
			checkpoint.ControlGrant.Turn != 3 {
			t.Fatalf("turn-extension checkpoint = %#v", checkpoint)
		}
		var successorCount int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM task_runs WHERE id = ?`,
			successorRunID,
		).Scan(&successorCount); err != nil {
			t.Fatalf("count turn-extension successor error = %v", err)
		}
		if successorCount != 1 {
			t.Fatalf("turn-extension successors = %d, want 1", successorCount)
		}
	})
}

func TestGoalTurnReaderIntegration(t *testing.T) {
	t.Run("Should paginate one run wide sequence without skips or duplicates", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-reader")
		insertGoalSchemaLoopRun(t, globalDB, "run-goal-reader", "ws-goal-reader", "catalog", nil)
		startedAt := time.Date(2026, 7, 10, 18, 0, 0, 0, time.UTC)
		rows := []struct {
			seq        int64
			generation int
			nodeID     string
			itemIndex  int
			turn       int
		}{
			{seq: 1, generation: 1, nodeID: "alpha", itemIndex: 0, turn: 1},
			{seq: 2, generation: 1, nodeID: "beta", itemIndex: 0, turn: 1},
			{seq: 3, generation: 1, nodeID: "alpha", itemIndex: 1, turn: 1},
			{seq: 4, generation: 2, nodeID: "alpha", itemIndex: 0, turn: 1},
			{seq: 5, generation: 1, nodeID: "alpha", itemIndex: 0, turn: 2},
		}
		for _, row := range rows {
			insertGoalTurnReaderRow(
				t,
				globalDB,
				"run-goal-reader",
				row.seq,
				row.generation,
				row.nodeID,
				row.itemIndex,
				row.turn,
				startedAt.Add(time.Duration(row.seq)*time.Minute),
			)
		}

		ctx := testutil.Context(t)
		query := goal.TurnQuery{
			WorkspaceID: "ws-goal-reader",
			LoopRunID:   "run-goal-reader",
			Limit:       2,
		}
		var gotSeqs []int64
		for {
			page, err := globalDB.ListGoalTurns(ctx, query)
			if err != nil {
				t.Fatalf("ListGoalTurns(after_seq=%d) error = %v", query.AfterSeq, err)
			}
			for _, turn := range page.Turns {
				gotSeqs = append(gotSeqs, turn.Seq)
				if turn.Key.WorkspaceID != query.WorkspaceID || turn.Key.LoopRunID != query.LoopRunID {
					t.Fatalf("turn key = %#v, want workspace/run %q/%q", turn.Key, query.WorkspaceID, query.LoopRunID)
				}
				if turn.BlockingIssues == nil {
					t.Fatalf("turn %d blocking issues = nil, want empty slice", turn.Seq)
				}
			}
			if page.NextAfterSeq == nil {
				break
			}
			query.AfterSeq = *page.NextAfterSeq
		}
		wantSeqs := []int64{1, 2, 3, 4, 5}
		if len(gotSeqs) != len(wantSeqs) {
			t.Fatalf("paginated seqs = %v, want %v", gotSeqs, wantSeqs)
		}
		for index := range wantSeqs {
			if gotSeqs[index] != wantSeqs[index] {
				t.Fatalf("paginated seqs = %v, want %v", gotSeqs, wantSeqs)
			}
		}

		itemIndex := 0
		filtered, err := globalDB.ListGoalTurns(ctx, goal.TurnQuery{
			WorkspaceID: "ws-goal-reader",
			LoopRunID:   "run-goal-reader",
			NodeID:      "alpha",
			ItemIndex:   &itemIndex,
		})
		if err != nil {
			t.Fatalf("ListGoalTurns(filtered) error = %v", err)
		}
		filteredSeqs := make([]int64, 0, len(filtered.Turns))
		for _, turn := range filtered.Turns {
			filteredSeqs = append(filteredSeqs, turn.Seq)
		}
		wantFiltered := []int64{1, 4, 5}
		if len(filteredSeqs) != len(wantFiltered) {
			t.Fatalf("filtered seqs = %v, want %v", filteredSeqs, wantFiltered)
		}
		for index := range wantFiltered {
			if filteredSeqs[index] != wantFiltered[index] {
				t.Fatalf("filtered seqs = %v, want %v", filteredSeqs, wantFiltered)
			}
		}

		turn, err := globalDB.GetGoalTurnByPromptID(ctx, goal.TurnKey{
			WorkspaceID: "ws-goal-reader",
			LoopRunID:   "run-goal-reader",
			Generation:  2,
			NodeID:      "alpha",
		}, "prompt-goal-reader-4")
		if err != nil {
			t.Fatalf("GetGoalTurnByPromptID() error = %v", err)
		}
		if turn.Seq != 4 || turn.Key.Generation != 2 || turn.Key.NodeID != "alpha" {
			t.Fatalf("turn = %#v, want seq 4 generation 2 node alpha", turn)
		}
	})

	t.Run("Should enforce workspace ownership and return deterministic empty results", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-reader", "ws-foreign")
		insertGoalSchemaLoopRun(t, globalDB, "run-goal-reader-empty", "ws-goal-reader", "catalog", nil)
		ctx := testutil.Context(t)

		page, err := globalDB.ListGoalTurns(ctx, goal.TurnQuery{
			WorkspaceID: "ws-goal-reader",
			LoopRunID:   "run-goal-reader-empty",
		})
		if err != nil {
			t.Fatalf("ListGoalTurns(empty) error = %v", err)
		}
		if page.Turns == nil || len(page.Turns) != 0 || page.NextAfterSeq != nil {
			t.Fatalf("empty page = %#v, want non-nil empty turns and nil cursor", page)
		}

		_, err = globalDB.ListGoalTurns(ctx, goal.TurnQuery{
			WorkspaceID: "ws-foreign",
			LoopRunID:   "run-goal-reader-empty",
		})
		if !errors.Is(err, looppkg.ErrRunNotFound) {
			t.Fatalf("ListGoalTurns(foreign workspace) error = %v, want %v", err, looppkg.ErrRunNotFound)
		}

		_, err = globalDB.GetGoalTurnByPromptID(ctx, goal.TurnKey{
			WorkspaceID: "ws-goal-reader",
			LoopRunID:   "run-goal-reader-empty",
			Generation:  1,
			NodeID:      "alpha",
		}, "missing-prompt")
		if !errors.Is(err, goal.ErrTurnNotFound) {
			t.Fatalf("GetGoalTurnByPromptID(missing) error = %v, want %v", err, goal.ErrTurnNotFound)
		}
	})
}

func insertGoalTurnReaderRow(
	t *testing.T,
	globalDB *GlobalDB,
	runID string,
	seq int64,
	generation int,
	nodeID string,
	itemIndex int,
	turn int,
	startedAt time.Time,
) {
	t.Helper()

	if _, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO loop_goal_turns (
			loop_run_id, seq, generation, node_id, item_index, turn,
			session_id, binding_handle, binding_epoch, prompt_id, prompt_attempt,
			usage_base_tokens, blocking_json, actor_kind, actor_id, started_at
		) VALUES (?, ?, ?, ?, ?, ?, 'session-goal-reader', 'goal:reader', 1, ?, 0, 0, '[]', 'system', 'reader-test', ?)`,
		runID,
		seq,
		generation,
		nodeID,
		itemIndex,
		turn,
		"prompt-goal-reader-"+strconv.FormatInt(seq, 10),
		startedAt,
	); err != nil {
		t.Fatalf("insert Goal turn seq %d error = %v", seq, err)
	}
}

func seedGoalTurnRuntime(
	t *testing.T,
	runID string,
) (*GlobalDB, goal.TurnKey, string, time.Time) {
	t.Helper()

	globalDB := openLoopTestGlobalDB(t, "ws-goal-runtime")
	now := time.Date(2026, 7, 10, 16, 0, 0, 0, time.UTC)
	globalDB.now = func() time.Time { return now }
	registerGoalSessionIdentityForTest(
		t,
		globalDB,
		goalSessionInfoForTest("session-goal-runtime", "ws-goal-runtime", now),
		store.SessionCreationIdentity{
			CreationProfileRef: "profile-runtime",
			PolicySpecDigest:   "policy-runtime",
			CreationDigest:     "creation-runtime",
		},
	)
	insertGoalSchemaLoopRun(t, globalDB, runID, "ws-goal-runtime", "catalog", nil)
	if _, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO loop_session_bindings (
			loop_run_id, handle, binding_epoch, binding_attempt_id, session_id, workspace_id,
			creation_profile_ref, policy_spec_digest, creation_digest, ownership, state,
			created_at, activated_at
		) VALUES (?, 'goal:runtime', 1, ?, 'session-goal-runtime', 'ws-goal-runtime',
			'profile-runtime', 'policy-runtime', 'creation-runtime', 'run-owned', 'active', ?, ?)`,
		runID,
		"binding-attempt:"+runID,
		store.FormatTimestamp(now),
		store.FormatTimestamp(now),
	); err != nil {
		t.Fatalf("insert active Goal runtime binding error = %v", err)
	}
	createCompletedLoopWorkerRunForTest(
		testutil.Context(t), t, globalDB, looppkg.RunID(runID), "goal-runtime", 0, now,
	)
	taskRunID := "run-goal-runtime"
	key := goal.TurnKey{
		WorkspaceID: "ws-goal-runtime",
		LoopRunID:   looppkg.RunID(runID),
		Generation:  1,
		NodeID:      "converge",
	}
	if _, err := globalDB.CreateCheckpoint(testutil.Context(t), goal.CreateCheckpointRequest{
		Checkpoint: goal.Checkpoint{
			Key:               key,
			ControlEpoch:      1,
			Phase:             "idle",
			Status:            "active",
			TurnLimit:         3,
			TaskRunID:         taskRunID,
			SessionID:         "session-goal-runtime",
			BindingHandle:     "goal:runtime",
			BindingEpoch:      1,
			ContextState:      "unknown",
			ContextNudgeRatio: 0.8,
			UpdatedAt:         now,
		},
	}); err != nil {
		t.Fatalf("CreateCheckpoint() error = %v", err)
	}
	return globalDB, key, taskRunID, now
}

func TestGoalSessionProjectionReader(t *testing.T) {
	t.Parallel()

	t.Run("Should apply clear only after selecting the newest session Goal", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-snapshot")
		ctx := testutil.Context(t)
		sessionID := "session-goal-snapshot"
		insertGoalSchemaLoopRun(t, globalDB, "run-goal-older", "ws-goal-snapshot", "session", &sessionID)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET status = 'done' WHERE id = 'run-goal-older'`,
		); err != nil {
			t.Fatalf("terminalize older Goal run error = %v", err)
		}
		insertGoalSchemaLoopRun(t, globalDB, "run-goal-newer", "ws-goal-snapshot", "session", &sessionID)
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET goal_cleared_at = ? WHERE id = 'run-goal-newer'`,
			store.FormatTimestamp(time.Date(2026, 7, 10, 17, 0, 0, 0, time.UTC)),
		); err != nil {
			t.Fatalf("clear newer Goal run error = %v", err)
		}

		projection, err := globalDB.GetSessionGoalProjection(ctx, "ws-goal-snapshot", sessionID)
		if err != nil {
			t.Fatalf("GetSessionGoalProjection() error = %v", err)
		}
		if !projection.Found || !projection.Cleared || projection.RunID != "run-goal-newer" {
			t.Fatalf("GetSessionGoalProjection() = %#v, want newest cleared Run", projection)
		}

		foreign, err := globalDB.GetSessionGoalProjection(ctx, "ws-goal-foreign", sessionID)
		if err != nil {
			t.Fatalf("GetSessionGoalProjection(foreign) error = %v", err)
		}
		if foreign.Found {
			t.Fatalf("GetSessionGoalProjection(foreign) = %#v, want no projection", foreign)
		}
	})
}

func prepareGoalRuntimePrompt(
	t *testing.T,
	globalDB *GlobalDB,
	key goal.TurnKey,
	taskRunID string,
	promptID string,
	now time.Time,
) goal.PromptTicket {
	t.Helper()

	ticket, err := globalDB.PrepareGoalPrompt(testutil.Context(t), goal.PreparePromptRequest{
		Key:                  key,
		ExpectedControlEpoch: 1,
		TaskRunID:            taskRunID,
		QueueEntryID:         "queue:" + promptID,
		PromptID:             promptID,
		PromptKind:           "work",
		SessionID:            "session-goal-runtime",
		BindingHandle:        "goal:runtime",
		BindingEpoch:         1,
		Message:              "Advance the durable Goal",
		PreparedAt:           now,
	})
	if err != nil {
		t.Fatalf("PrepareGoalPrompt() error = %v", err)
	}
	return ticket
}

func finalizeGoalRuntimeCompaction(
	t *testing.T,
	globalDB *GlobalDB,
	key goal.TurnKey,
	taskRunID string,
	promptID string,
	usageSequence int64,
	now time.Time,
) (goal.PromptTicket, looppkg.ActionPromptResult) {
	t.Helper()
	ctx := testutil.Context(t)
	usageBaselineUsed := int64(9)
	ticket, err := globalDB.PrepareGoalPrompt(ctx, goal.PreparePromptRequest{
		Key: key, ExpectedControlEpoch: 1, TaskRunID: taskRunID, QueueEntryID: "queue:" + promptID,
		PromptID: promptID, PromptKind: "compact", SessionID: "session-goal-runtime",
		BindingHandle: "goal:runtime", BindingEpoch: 1,
		ContextUsageSequence: &usageSequence, ContextUsageUsed: &usageBaselineUsed,
		Message: "Compact the Goal context", PreparedAt: now,
	})
	if err != nil {
		t.Fatalf("PrepareGoalPrompt(compact) error = %v", err)
	}
	decision := flushGoalRuntimeBudget(t, globalDB, key, taskRunID, goal.BudgetBeforeCompact, promptID, 0)
	claimed, err := globalDB.ClaimPreparedCompaction(ctx, goal.ClaimPreparedCompactionRequest{
		Key: key, ExpectedControlEpoch: 1, TaskRunID: taskRunID, QueueEntryID: ticket.QueueEntryID,
		PromptID: promptID, SessionID: "session-goal-runtime", BindingHandle: "goal:runtime",
		BindingEpoch: 1, UsageSequence: &usageSequence, BudgetDecision: decision,
	})
	if err != nil {
		t.Fatalf("ClaimPreparedCompaction() error = %v", err)
	}
	if err := globalDB.RecordGoalDriverAttached(ctx, goal.DriverAttachmentRequest{
		Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, TaskRunID: taskRunID,
		QueueEntryID: ticket.QueueEntryID, PromptID: promptID, SessionID: "session-goal-runtime",
		BindingHandle: "goal:runtime", DispatchToken: claimed.DispatchToken, DriverTurnID: promptID,
		AttachedAt: now.Add(time.Second),
	}); err != nil {
		t.Fatalf("RecordGoalDriverAttached(compact) error = %v", err)
	}
	terminal := looppkg.ActionPromptResult{
		PromptID: promptID, Outcome: looppkg.ActionPromptOutcomeCompleted,
		StopReason: looppkg.ActionStopMaxTokens,
	}
	if err := globalDB.FinalizeGoalPrompt(ctx, goal.FinalizePromptRequest{
		Key: key, ExpectedControlEpoch: 1, ExpectedBindingEpoch: 1, TaskRunID: taskRunID,
		QueueEntryID: ticket.QueueEntryID, PromptID: promptID, SessionID: "session-goal-runtime",
		BindingHandle: "goal:runtime", DispatchToken: claimed.DispatchToken, Result: terminal,
		TerminalAt: now.Add(2 * time.Second),
	}); err != nil {
		t.Fatalf("FinalizeGoalPrompt(compact) error = %v", err)
	}
	return ticket, terminal
}

func flushGoalRuntimeBudget(
	t *testing.T,
	globalDB *GlobalDB,
	key goal.TurnKey,
	taskRunID string,
	boundary goal.BudgetBoundary,
	operationID string,
	liveTokens int64,
) goal.BudgetDecision {
	t.Helper()

	decision, err := globalDB.FlushAndCheck(
		testutil.Context(t),
		goalBudgetSnapshotForTest(t, globalDB, goal.BudgetBoundarySnapshot{
			Key:                 key,
			TaskRunID:           taskRunID,
			Boundary:            boundary,
			Phase:               "runtime-test",
			Turn:                1,
			OperationID:         operationID,
			OperationBaseTokens: 0,
			LiveTokensUsed:      liveTokens,
			TokensReported:      true,
		}),
	)
	if err != nil {
		t.Fatalf("FlushAndCheck(%s) error = %v", boundary, err)
	}
	return decision
}

func assertGoalRuntimeTurn(
	t *testing.T,
	globalDB *GlobalDB,
	key goal.TurnKey,
	promptID string,
	wantResult string,
	wantStop string,
) {
	t.Helper()

	var resultStatus string
	var stopReason *string
	if err := globalDB.db.QueryRowContext(
		testutil.Context(t),
		`SELECT result_status, stop_reason FROM loop_goal_turns
		 WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND prompt_id = ?`,
		string(key.LoopRunID),
		key.Generation,
		string(key.NodeID),
		promptID,
	).Scan(&resultStatus, &stopReason); err != nil {
		t.Fatalf("query Goal turn terminal error = %v", err)
	}
	if resultStatus != wantResult || (wantStop == "" && stopReason != nil) ||
		(wantStop != "" && (stopReason == nil || *stopReason != wantStop)) {
		t.Fatalf("Goal turn terminal = result:%q stop:%v", resultStatus, stopReason)
	}
}

func assertGoalRuntimeEventCount(
	t *testing.T,
	globalDB *GlobalDB,
	runID looppkg.RunID,
	kind string,
	want int,
) {
	t.Helper()

	var count int
	if err := globalDB.db.QueryRowContext(
		testutil.Context(t),
		`SELECT COUNT(*) FROM loop_run_events WHERE loop_run_id = ? AND kind = ?`,
		string(runID),
		kind,
	).Scan(&count); err != nil {
		t.Fatalf("count %s events error = %v", kind, err)
	}
	if count != want {
		t.Fatalf("%s events = %d, want %d", kind, count, want)
	}
}
