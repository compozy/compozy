package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const (
	loopRunEventNodeRunning             = string(looppkg.RunEventNodeRunning)
	loopRunEventNodeSucceeded           = string(looppkg.RunEventNodeSucceeded)
	loopRunEventNodeFailed              = string(looppkg.RunEventNodeFailed)
	loopRunEventNodeQuarantined         = string(looppkg.RunEventNodeQuarantined)
	loopRunEventNodeRequeued            = string(looppkg.RunEventNodeRequeued)
	loopRunEventNodePaused              = string(looppkg.RunEventNodePaused)
	loopRunEventNodeResumed             = string(looppkg.RunEventNodeResumed)
	loopRunEventNodeWaitStarted         = string(looppkg.RunEventNodeWaitStarted)
	loopRunEventNodeWaitResumed         = string(looppkg.RunEventNodeWaitResumed)
	loopRunEventDuplicateSuppressed     = string(looppkg.RunEventDuplicateSuppressed)
	loopRunEventNodeCanceled            = string(looppkg.RunEventNodeCanceled)
	loopRunEventNodeKilled              = string(looppkg.RunEventNodeKilled)
	loopRunEventNodeAttentionFlagged    = string(looppkg.RunEventNodeAttentionFlagged)
	loopRunEventNodeAttentionCleared    = string(looppkg.RunEventNodeAttentionCleared)
	loopRunEventTargetBreakerTransition = string(looppkg.RunEventTargetBreaker)
	loopRunEventGateVerdict             = string(looppkg.RunEventGateVerdict)
	loopRunEventGenerationStarted       = string(looppkg.RunEventGenerationStarted)
	loopRunEventChannelMsg              = string(looppkg.RunEventChannelMsg)
	loopRunEventTokenTick               = string(looppkg.RunEventTokenTick)
	loopRunEventNeedsApproval           = string(looppkg.RunEventNeedsApproval)
	loopRunEventStatusChanged           = string(looppkg.RunEventStatusChanged)
	loopRunEventGoalTurnStarted         = string(looppkg.RunEventGoalTurnStarted)
	loopRunEventGoalTurnCompleted       = string(looppkg.RunEventGoalTurnCompleted)
	loopRunEventGoalStatusChanged       = string(looppkg.RunEventGoalStatusChanged)
	loopRunEventRuntimeApplied          = string(looppkg.RunEventRuntimeApplied)
	loopRunEventPredicateDiagnostic     = string(looppkg.RunEventPredicateDiagnostic)
	loopRunEventRouteTaken              = string(looppkg.RunEventRouteTaken)
	loopRunEventNodeRetryScheduled      = string(looppkg.RunEventNodeRetryScheduled)
	loopRunEventStaleScheduleDropped    = string(looppkg.RunEventStaleScheduleDropped)
	loopRunEventLateArrival             = string(looppkg.RunEventLateArrival)
	loopRunEventEffectResults           = string(looppkg.RunEventEffectResults)
	loopRunEventCustomEvent             = string(looppkg.RunEventCustomEvent)
	loopRunEventRequestOpened           = string(looppkg.RunEventRequestOpened)
	loopRunEventRequestAnswered         = string(looppkg.RunEventRequestAnswered)
	loopRunEventRequestExpired          = string(looppkg.RunEventRequestExpired)
	loopRunEventRequestCanceled         = string(looppkg.RunEventRequestCanceled)
	loopRunEventNodeAmended             = string(looppkg.RunEventNodeAmended)
	loopRunEventBranchPruned            = string(looppkg.RunEventBranchPruned)
	loopRunEventRunForked               = string(looppkg.RunEventRunForked)

	maxLoopRunEventPayloadBytes = 16 * 1024
	loopTokenTickMinDelta       = 2000
	loopTokenTickMinInterval    = 5 * time.Second

	loopRunEventPayloadKeyGeneration    = "generation"
	loopRunEventPayloadKeyKind          = "kind"
	loopRunEventPayloadKeyPrompt        = "prompt"
	loopRunEventPayloadKeyExpiresAt     = "expires_at"
	loopRunEventPayloadKeyAttentionFlag = "attention_flag"
	loopRunEventPayloadKeyFrom          = "from"
	loopRunEventPayloadKeyFailure       = "failure"
	loopRunEventPayloadKeyTo            = "to"
	loopRunEventPayloadKeyCause         = "cause"
	loopRunEventPayloadKeyActorKind     = "actor_kind"
	loopRunEventPayloadKeyActorID       = "actor_id"
	loopRunEventPayloadKeyItemIndex     = "item_index"
	loopRunEventPayloadKeyNodeID        = "node_id"
	loopRunEventPayloadKeyPromptID      = "prompt_id"
	loopRunEventPayloadKeyReason        = "reason"
	loopRunEventPayloadKeyRole          = "role"
	loopRunEventPayloadKeyStopReason    = "stop_reason"
	loopRunEventPayloadKeySummary       = "summary"
	loopRunEventPayloadKeyStatus        = "status"
	loopRunEventPayloadKeyTaskID        = "task_id"
	loopRunEventPayloadKeyTaskRunID     = "task_run_id"
	loopRunEventPayloadKeyTarget        = "target"
	loopRunEventPayloadKeyTerminal      = "terminal"
	loopRunEventPayloadKeyText          = "text"
	loopRunEventPayloadKeyTitle         = "title"
	loopRunEventPayloadKeyType          = "type"
	loopRunEventPayloadKeyValue         = "value"
	loopRunEventPayloadKeyVerdict       = "verdict"
	loopRunEventPayloadKeyIssuedEpoch   = "issued_epoch"
	loopRunEventPayloadKeyCurrentEpoch  = "current_epoch"
	loopRunEventPayloadKeyScheduleKind  = "schedule_kind"
	loopRunEventPayloadKeyMode          = "mode"
	loopRunEventPayloadKeyWaitKind      = "wait_kind"
	loopRunEventPayloadKeyExpired       = "expired"
	loopRunEventPayloadKeyRoute         = "route"
	loopRunEventPayloadKeyDefault       = "default"
	loopRunEventPayloadKeyMatchedWhen   = "matched_when"
	loopRunEventVerdictRevise           = "revise"
	loopRunApprovalFactLabelKey         = "label"
	loopRunNodeOutputRunning            = "running"
	loopRetryScheduleKind               = "retry"
	loopGenerationOutputRetrying        = "retrying"
	loopGenerationOutputAwaitingGoal    = "awaiting_goal"
	loopWaitClaimedByScheduler          = "scheduler"
	loopWaitClaimedByTimer              = "timer"
	loopWaitClaimedByEvent              = "event"
	loopWaitClaimedByExpiry             = "expiry"
)

func appendLoopRunStatusEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	ws looppkg.WorkspaceID,
	from looppkg.Status,
	to looppkg.Status,
	cause looppkg.TransitionCause,
	at time.Time,
) error {
	return appendLoopRunStatusEventWithFailure(ctx, exec, runID, ws, from, to, cause, nil, at)
}

func appendLoopRunStatusEventWithFailure(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	ws looppkg.WorkspaceID,
	from looppkg.Status,
	to looppkg.Status,
	cause looppkg.TransitionCause,
	failure *taskpkg.CoordinatorFailure,
	at time.Time,
) error {
	return appendLoopRunStatusEventWithFailureAndEffects(
		ctx, exec, runID, ws, from, to, cause, failure, nil, at,
	)
}

func appendLoopRunStatusEventWithFailureAndEffects(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	ws looppkg.WorkspaceID,
	from looppkg.Status,
	to looppkg.Status,
	cause looppkg.TransitionCause,
	failure *taskpkg.CoordinatorFailure,
	effects []looppkg.RenderedEffectIntent,
	at time.Time,
) error {
	if from == to {
		return nil
	}
	payload := map[string]any{
		loopRunEventPayloadKeyFrom:   string(from),
		loopRunEventPayloadKeyTo:     string(to),
		loopRunEventPayloadKeyStatus: string(to),
		loopRunEventPayloadKeyCause:  string(cause),
	}
	if to.Terminal() {
		var completionState string
		if err := exec.QueryRowContext(ctx, `SELECT completion_state FROM loop_runs WHERE id = ?`,
			runID).Scan(&completionState); err != nil {
			return fmt.Errorf("store: load Loop completion state for status event: %w", err)
		}
		payload["completion_state"] = completionState
	}
	if failure != nil {
		payload[loopRunEventPayloadKeyFailure] = failure
	}
	eventID, _, err := appendLoopRunEventWithIdentity(
		ctx, exec, runID, ws, loopRunEventStatusChanged, payload, at,
	)
	if err != nil {
		return err
	}
	if len(effects) == 0 {
		return nil
	}
	run := looppkg.Run{ID: runID, WorkspaceID: ws}
	return insertLoopEffectIntentsWithExecutor(ctx, exec, run, eventID, effects, at)
}

func appendLoopRunEventWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	ws looppkg.WorkspaceID,
	kind string,
	payload any,
	at time.Time,
) error {
	_, _, err := appendLoopRunEventWithIdentity(ctx, exec, runID, ws, kind, payload, at)
	return err
}

func appendLoopRunEventWithSequence(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	ws looppkg.WorkspaceID,
	kind string,
	payload any,
	at time.Time,
) (int64, error) {
	_, seq, err := appendLoopRunEventWithIdentity(ctx, exec, runID, ws, kind, payload, at)
	return seq, err
}

func appendLoopRunEventWithIdentity(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	ws looppkg.WorkspaceID,
	kind string,
	payload any,
	at time.Time,
) (string, int64, error) {
	runID = looppkg.RunID(strings.TrimSpace(string(runID)))
	ws = looppkg.WorkspaceID(strings.TrimSpace(string(ws)))
	kind = strings.TrimSpace(kind)
	if runID == "" {
		return "", 0, fmt.Errorf("%w: loop event run_id is required", looppkg.ErrValidation)
	}
	if ws == "" {
		return "", 0, fmt.Errorf("%w: loop event workspace_id is required", looppkg.ErrValidation)
	}
	if !loopRunEventKindValid(kind) {
		return "", 0, fmt.Errorf("%w: loop run event kind is invalid: %q", looppkg.ErrValidation, kind)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	payloadJSON, err := normalizeLoopRunEventPayload(kind, payload)
	if err != nil {
		return "", 0, err
	}
	seq, err := nextLoopRunEventSequence(ctx, exec, runID)
	if err != nil {
		return "", 0, err
	}
	eventID, err := store.NewID("loopevt")
	if err != nil {
		return "", 0, fmt.Errorf("store: generate loop run event id: %w", err)
	}
	err = sqlcgen.New(exec).InsertLoopRunEvent(ctx, sqlcgen.InsertLoopRunEventParams{
		ID: eventID, LoopRunID: string(runID), WorkspaceID: string(ws),
		Seq: seq, Kind: kind, PayloadJson: string(payloadJSON), At: at.UTC(),
	})
	if err != nil {
		return "", 0, fmt.Errorf("store: insert loop run event %q: %w", kind, err)
	}
	return eventID, seq, nil
}

func nextLoopRunEventSequence(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
) (int64, error) {
	next, err := sqlcgen.New(exec).NextLoopRunEventSequence(ctx, string(runID))
	if err != nil {
		return 0, fmt.Errorf("store: select next loop run event sequence: %w", err)
	}
	return next, nil
}

func loopRunEventKindValid(kind string) bool {
	switch kind {
	case loopRunEventNodeRunning,
		loopRunEventNodeSucceeded,
		loopRunEventNodeFailed,
		loopRunEventNodeQuarantined,
		loopRunEventNodeRequeued,
		loopRunEventNodePaused,
		loopRunEventNodeResumed,
		loopRunEventNodeWaitStarted,
		loopRunEventNodeWaitResumed,
		loopRunEventDuplicateSuppressed,
		loopRunEventNodeCanceled,
		loopRunEventNodeKilled,
		loopRunEventNodeAttentionFlagged,
		loopRunEventNodeAttentionCleared,
		loopRunEventTargetBreakerTransition,
		loopRunEventGateVerdict,
		loopRunEventGenerationStarted,
		loopRunEventChannelMsg,
		loopRunEventTokenTick,
		loopRunEventNeedsApproval,
		loopRunEventStatusChanged,
		loopRunEventGoalTurnStarted,
		loopRunEventGoalTurnCompleted,
		loopRunEventGoalStatusChanged,
		loopRunEventRuntimeApplied,
		loopRunEventPredicateDiagnostic,
		loopRunEventRouteTaken,
		loopRunEventNodeRetryScheduled,
		loopRunEventStaleScheduleDropped,
		loopRunEventLateArrival,
		loopRunEventEffectResults,
		loopRunEventCustomEvent,
		loopRunEventRequestOpened,
		loopRunEventRequestAnswered,
		loopRunEventRequestExpired,
		loopRunEventRequestCanceled,
		loopRunEventNodeAmended,
		loopRunEventBranchPruned,
		loopRunEventRunForked:
		return true
	default:
		return false
	}
}
