package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func goalSettlementEvidenceRef(
	ctx context.Context,
	exec taskSQLExecutor,
	checkpoint goal.Checkpoint,
	req goal.CompleteTurnRequest,
) (string, error) {
	if checkpoint.ReportIntent != nil && checkpoint.ReportIntent.PromptID == req.PromptID &&
		checkpoint.ReportIntent.BindingEpoch == req.ExpectedBindingEpoch {
		return strings.TrimSpace(checkpoint.ReportIntent.EvidenceRef), nil
	}
	if checkpoint.JudgeAttemptID == "" {
		return "", nil
	}
	evidence, err := sqlcgen.New(exec).GetGoalSettlementEvidenceRef(ctx, sqlcgen.GetGoalSettlementEvidenceRefParams{
		AttemptID: checkpoint.JudgeAttemptID, LoopRunID: string(req.Key.LoopRunID),
	})
	if err != nil {
		return "", fmt.Errorf("store: load Goal judge evidence for settlement: %w", err)
	}
	if evidence.Valid {
		return strings.TrimSpace(evidence.String), nil
	}
	return "", nil
}

func resolveGoalTurnSettlement(
	ctx context.Context,
	exec taskSQLExecutor,
	checkpoint goal.Checkpoint,
	req goal.CompleteTurnRequest,
) (goalTurnSettlement, error) {
	settlement := goalTurnSettlement{
		phase:          goalCheckpointPhaseIdle,
		status:         goalStatusActive,
		actorKind:      "system",
		actorID:        "goal-executor",
		brokenStreak:   checkpoint.BrokenStreak,
		recoveryStreak: checkpoint.RecoveryStreak,
	}
	if req.Result.StopReason == looppkg.ActionStopMaxTokens {
		settlement.recoveryStreak++
	}
	if req.Verdict != nil {
		switch req.Verdict.Outcome {
		case gate.VerdictOutcomeError, gate.VerdictOutcomeTimeout, gate.VerdictOutcomeInvalidOutput:
			settlement.brokenStreak++
		default:
			settlement.brokenStreak = 0
		}
	}
	switch {
	case checkpoint.ReportIntent != nil && checkpoint.ReportIntent.PromptID == req.PromptID &&
		checkpoint.ReportIntent.BindingEpoch == req.ExpectedBindingEpoch &&
		checkpoint.ReportIntent.Status == goalReportStatusBlocked:
		settlement.phase = goalCheckpointPhaseTerminal
		settlement.status = goalStatusBlocked
		settlement.cause = looppkg.ReasonCodeGoalReportedBlocked
		settlement.actorKind = checkpoint.ReportIntent.ActorKind
		settlement.actorID = checkpoint.ReportIntent.ActorID
	case req.Verdict != nil && req.Verdict.Outcome == gate.VerdictOutcomeApproved:
		settlement.phase = goalCheckpointPhaseTerminal
		settlement.status = goalStatusComplete
	case req.Verdict != nil && req.Verdict.Outcome == gate.VerdictOutcomeBlocked:
		settlement.phase = goalCheckpointPhaseTerminal
		settlement.status = goalStatusBlocked
		settlement.cause = looppkg.ReasonCodeGoalReportedBlocked
	}
	if settlement.phase != goalCheckpointPhaseTerminal {
		pauseRequested, actorKind, actorID, requestedAt, err := pendingGoalPauseActor(
			ctx,
			exec,
			req.Key.LoopRunID,
		)
		if err != nil {
			return goalTurnSettlement{}, err
		}
		if pauseRequested {
			settlement.phase = goalCheckpointPhaseAwaitingControl
			settlement.status = goalStatusPaused
			settlement.cause = looppkg.ReasonCode(looppkg.TransitionCausePauseBoundary)
			settlement.actorKind = actorKind
			settlement.actorID = actorID
			settlement.pauseRequested = true
			settlement.pauseRequestedAt = requestedAt
		}
	}
	if checkpoint.ControlGrant != nil && !checkpoint.ControlGrant.Consumed &&
		checkpoint.ControlGrant.Kind == goal.ControlGrantBudget &&
		checkpoint.ControlGrant.Turn == checkpoint.TurnsUsed {
		settlement.consumeGrant = true
	}
	return settlement, nil
}

func pendingGoalPauseActor(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
) (bool, string, string, time.Time, error) {
	row, err := sqlcgen.New(exec).GetPendingGoalPauseActor(ctx, string(runID))
	if err != nil {
		return false, "", "", time.Time{}, fmt.Errorf("store: load Goal pause actor: %w", err)
	}
	if row.PauseRequested == 0 {
		return false, "", "", time.Time{}, nil
	}
	if !row.ControlActorKind.Valid || !row.ControlActorID.Valid ||
		strings.TrimSpace(row.ControlActorKind.String) == "" ||
		strings.TrimSpace(row.ControlActorID.String) == "" {
		return false, "", "", time.Time{}, fmt.Errorf(
			"%w: pending Goal pause actor is incomplete",
			looppkg.ErrValidation,
		)
	}
	parsedRequestedAt, err := parseGoalTimestampValue(
		goalOptionalTimeValue(row.ControlRequestedAt),
		"loop pause control_requested_at",
	)
	if err != nil {
		return false, "", "", time.Time{}, err
	}
	return true, strings.TrimSpace(
			row.ControlActorKind.String,
		), strings.TrimSpace(
			row.ControlActorID.String,
		), parsedRequestedAt, nil
}

func updateGoalCheckpointAfterTurn(
	ctx context.Context,
	exec taskSQLExecutor,
	checkpoint goal.Checkpoint,
	settlement goalTurnSettlement,
	now time.Time,
) error {
	controlActorKind := goalNullableString("")
	controlActorID := goalNullableString("")
	controlRequestedAt := sql.NullString{}
	if settlement.pauseRequested {
		controlActorKind = goalNullableString(settlement.actorKind)
		controlActorID = goalNullableString(settlement.actorID)
		controlRequestedAt = goalNullableTimestampString(&settlement.pauseRequestedAt)
	}
	affected, err := sqlcgen.New(exec).UpdateGoalCheckpointAfterTurn(ctx, sqlcgen.UpdateGoalCheckpointAfterTurnParams{
		Phase:              settlement.phase,
		GoalStatus:         settlement.status,
		ControlCause:       goalNullableString(string(settlement.cause)),
		BrokenStreak:       int64(settlement.brokenStreak),
		RecoveryStreak:     int64(settlement.recoveryStreak),
		ControlActorKind:   controlActorKind,
		ControlActorID:     controlActorID,
		ControlRequestedAt: controlRequestedAt,
		ConsumeGrant:       boolToInt(settlement.consumeGrant),
		UpdatedAt:          store.FormatTimestamp(now),
		LoopRunID:          string(checkpoint.Key.LoopRunID),
		Generation:         int64(checkpoint.Key.Generation),
		NodeID:             string(checkpoint.Key.NodeID),
		ItemIndex:          int64(checkpoint.Key.ItemIndex),
		ControlEpoch:       checkpoint.ControlEpoch,
		BindingEpoch:       goalNullableInt64(&checkpoint.BindingEpoch),
		TaskRunID:          goalNullableString(checkpoint.TaskRunID),
		QueueEntryID:       goalNullableString(checkpoint.QueueEntryID),
		PromptID:           goalNullableString(checkpoint.PromptID),
	})
	if err != nil {
		return fmt.Errorf("store: advance Goal checkpoint after turn: %w", err)
	}
	return requireGoalAffectedCount(affected, "advance Goal checkpoint after turn")
}

func appendGoalTurnCompletedEvent(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.TurnKey,
	promptID string,
	at time.Time,
) error {
	row, err := sqlcgen.New(exec).GetGoalTurnByPromptID(ctx, sqlcgen.GetGoalTurnByPromptIDParams{
		LoopRunID: string(key.LoopRunID), Generation: int64(key.Generation), NodeID: string(key.NodeID),
		ItemIndex: int64(key.ItemIndex), PromptID: strings.TrimSpace(promptID), WorkspaceID: string(key.WorkspaceID),
	})
	if err != nil {
		return fmt.Errorf("store: load completed Goal turn event payload: %w", err)
	}
	turn, err := goalTurnFromGenerated(row, key.WorkspaceID, key.LoopRunID)
	if err != nil {
		return fmt.Errorf("store: decode completed Goal turn event payload: %w", err)
	}
	if turn.ResultStatus == nil || turn.EndedAt == nil {
		return fmt.Errorf("%w: Goal turn event requires a terminal row", looppkg.ErrTransitionConflict)
	}
	return appendLoopRunEventWithExecutor(
		ctx,
		exec,
		key.LoopRunID,
		key.WorkspaceID,
		loopRunEventGoalTurnCompleted,
		map[string]any{
			loopRunEventPayloadKeyGeneration:     turn.Key.Generation,
			loopRunEventPayloadKeyNodeID:         string(turn.Key.NodeID),
			loopRunEventPayloadKeyItemIndex:      turn.Key.ItemIndex,
			"turn":                               turn.Turn,
			columnPromptAttempt:                  turn.PromptAttempt,
			loopRunEventPayloadKeyPromptID:       turn.PromptID,
			"seq":                                turn.Seq,
			columnSessionID:                      turn.SessionID,
			"binding_handle":                     turn.BindingHandle,
			columnBindingEpoch:                   turn.BindingEpoch,
			"result_status":                      *turn.ResultStatus,
			loopRunEventPayloadKeyStopReason:     goalTurnEventOptionalStopReason(turn.StopReason),
			"reason_code":                        goalTurnEventOptionalReasonCode(turn.ReasonCode),
			"verdict_outcome":                    goalTurnEventOptionalVerdict(turn.VerdictOutcome),
			loopRunEventPayloadKeyBlockingIssues: turn.BlockingIssues,
			"evidence_ref":                       goalTurnEventOptionalString(turn.EvidenceRef),
			"tokens_used":                        goalTurnEventOptionalInt64(turn.TokensUsed),
			loopRunEventPayloadKeyActorKind:      turn.ActorKind,
			loopRunEventPayloadKeyActorID:        turn.ActorID,
		},
		at,
	)
}

func goalTurnEventOptionalStopReason(value *looppkg.ActionStopReason) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func goalTurnEventOptionalReasonCode(value *looppkg.ReasonCode) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func goalTurnEventOptionalVerdict(value *gate.VerdictOutcome) any {
	if value == nil {
		return nil
	}
	return string(*value)
}

func goalTurnEventOptionalString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func goalTurnEventOptionalInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}
