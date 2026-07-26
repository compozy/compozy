package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	loopRunEventNodeRunning       = "node_running"
	loopRunEventNodeSucceeded     = "node_succeeded"
	loopRunEventNodeFailed        = "node_failed"
	loopRunEventGateVerdict       = "gate_verdict"
	loopRunEventGenerationStarted = "generation_started"
	loopRunEventChannelMsg        = "channel_msg"
	loopRunEventTokenTick         = "token_tick"
	loopRunEventNeedsApproval     = "needs_approval"
	loopRunEventStatusChanged     = "status_changed"
	loopRunEventGoalTurnStarted   = "goal_turn_started"
	loopRunEventGoalTurnCompleted = "goal_turn_completed"
	loopRunEventGoalStatusChanged = "goal_status_changed"

	maxLoopRunEventPayloadBytes = 16 * 1024
	loopTokenTickMinDelta       = 2000
	loopTokenTickMinInterval    = 5 * time.Second

	loopRunEventPayloadKeyGeneration = "generation"
	loopRunEventPayloadKeyFrom       = "from"
	loopRunEventPayloadKeyFailure    = "failure"
	loopRunEventPayloadKeyTo         = "to"
	loopRunEventPayloadKeyCause      = "cause"
	loopRunEventPayloadKeyActorKind  = "actor_kind"
	loopRunEventPayloadKeyActorID    = "actor_id"
	loopRunEventPayloadKeyItemIndex  = "item_index"
	loopRunEventPayloadKeyNodeID     = "node_id"
	loopRunEventPayloadKeyPromptID   = "prompt_id"
	loopRunEventPayloadKeyReason     = "reason"
	loopRunEventPayloadKeyRole       = "role"
	loopRunEventPayloadKeyStopReason = "stop_reason"
	loopRunEventPayloadKeySummary    = "summary"
	loopRunEventPayloadKeyStatus     = "status"
	loopRunEventPayloadKeyTaskID     = "task_id"
	loopRunEventPayloadKeyTaskRunID  = "task_run_id"
	loopRunEventPayloadKeyTerminal   = "terminal"
	loopRunEventPayloadKeyText       = "text"
	loopRunEventPayloadKeyTitle      = "title"
	loopRunEventPayloadKeyType       = "type"
	loopRunEventPayloadKeyValue      = "value"
	loopRunEventPayloadKeyVerdict    = "verdict"
	loopRunEventVerdictRevise        = "revise"
	loopRunApprovalFactLabelKey      = "label"
	loopRunNodeOutputRunning         = "running"
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
	return appendLoopRunEventWithExecutor(ctx, exec, runID, ws, loopRunEventStatusChanged, payload, at)
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
	_, err := appendLoopRunEventWithSequence(ctx, exec, runID, ws, kind, payload, at)
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
	runID = looppkg.RunID(strings.TrimSpace(string(runID)))
	ws = looppkg.WorkspaceID(strings.TrimSpace(string(ws)))
	kind = strings.TrimSpace(kind)
	if runID == "" {
		return 0, fmt.Errorf("%w: loop event run_id is required", looppkg.ErrValidation)
	}
	if ws == "" {
		return 0, fmt.Errorf("%w: loop event workspace_id is required", looppkg.ErrValidation)
	}
	if !loopRunEventKindValid(kind) {
		return 0, fmt.Errorf("%w: loop run event kind is invalid: %q", looppkg.ErrValidation, kind)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	payloadJSON, err := normalizeLoopRunEventPayload(kind, payload)
	if err != nil {
		return 0, err
	}
	seq, err := nextLoopRunEventSequence(ctx, exec, runID)
	if err != nil {
		return 0, err
	}
	err = sqlcgen.New(exec).InsertLoopRunEvent(ctx, sqlcgen.InsertLoopRunEventParams{
		ID: store.NewID("loopevt"), LoopRunID: string(runID), WorkspaceID: string(ws),
		Seq: seq, Kind: kind, PayloadJson: string(payloadJSON), At: at.UTC(),
	})
	if err != nil {
		return 0, fmt.Errorf("store: insert loop run event %q: %w", kind, err)
	}
	return seq, nil
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
		loopRunEventGateVerdict,
		loopRunEventGenerationStarted,
		loopRunEventChannelMsg,
		loopRunEventTokenTick,
		loopRunEventNeedsApproval,
		loopRunEventStatusChanged,
		loopRunEventGoalTurnStarted,
		loopRunEventGoalTurnCompleted,
		loopRunEventGoalStatusChanged:
		return true
	default:
		return false
	}
}
