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
	loopRunEventNodeRunning             = "node_running"
	loopRunEventNodeSucceeded           = "node_succeeded"
	loopRunEventNodeFailed              = "node_failed"
	loopRunEventNodeQuarantined         = "node_quarantined"
	loopRunEventNodeRequeued            = "node_requeued"
	loopRunEventNodePaused              = "node_paused"
	loopRunEventNodeResumed             = "node_resumed"
	loopRunEventNodeWaitStarted         = "node_wait_started"
	loopRunEventNodeWaitResumed         = "node_wait_resumed"
	loopRunEventDuplicateSuppressed     = "duplicate_suppressed"
	loopRunEventNodeCanceled            = "node_canceled"
	loopRunEventNodeKilled              = "node_killed"
	loopRunEventNodeAttentionFlagged    = "node_attention_flagged"
	loopRunEventNodeAttentionCleared    = "node_attention_cleared"
	loopRunEventTargetBreakerTransition = "target_breaker_transition"
	loopRunEventGateVerdict             = "gate_verdict"
	loopRunEventGenerationStarted       = "generation_started"
	loopRunEventChannelMsg              = "channel_msg"
	loopRunEventTokenTick               = "token_tick"
	loopRunEventNeedsApproval           = "needs_approval"
	loopRunEventStatusChanged           = "status_changed"
	loopRunEventGoalTurnStarted         = "goal_turn_started"
	loopRunEventGoalTurnCompleted       = "goal_turn_completed"
	loopRunEventGoalStatusChanged       = "goal_status_changed"
	loopRunEventRuntimeApplied          = "runtime_applied"
	loopRunEventPredicateDiagnostic     = "predicate_diagnostic"
	loopRunEventRouteTaken              = "route_taken"
	loopRunEventNodeRetryScheduled      = "node_retry_scheduled"
	loopRunEventStaleScheduleDropped    = "stale_schedule_dropped"
	loopRunEventLateArrival             = "late_arrival"
	loopRunEventEffectResults           = "effect_results"
	loopRunEventCustomEvent             = "custom_event"
	loopRunEventRequestOpened           = "request_opened"
	loopRunEventRequestAnswered         = "request_answered"
	loopRunEventRequestExpired          = "request_expired"
	loopRunEventRequestCanceled         = "request_canceled"
	loopRunEventNodeAmended             = "node_amended"

	maxLoopRunEventPayloadBytes = 16 * 1024
	loopTokenTickMinDelta       = 2000
	loopTokenTickMinInterval    = 5 * time.Second

	loopRunEventPayloadKeyGeneration    = "generation"
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
		loopRunEventNodeAmended:
		return true
	default:
		return false
	}
}
