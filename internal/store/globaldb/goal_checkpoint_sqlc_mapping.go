package globaldb

import (
	"database/sql"
	"time"

	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func goalCheckpointInsertParams(checkpoint goal.Checkpoint) sqlcgen.InsertGoalCheckpointParams {
	params := sqlcgen.InsertGoalCheckpointParams{
		LoopRunID:                  string(checkpoint.Key.LoopRunID),
		Generation:                 int64(checkpoint.Key.Generation),
		NodeID:                     string(checkpoint.Key.NodeID),
		ItemIndex:                  int64(checkpoint.Key.ItemIndex),
		ControlEpoch:               checkpoint.ControlEpoch,
		ControlActorKind:           goalNullableString(checkpoint.ControlActorKind),
		ControlActorID:             goalNullableString(checkpoint.ControlActorID),
		ControlRequestedAt:         goalNullableTimestampString(checkpoint.ControlRequestedAt),
		Phase:                      checkpoint.Phase,
		GoalStatus:                 checkpoint.Status,
		ControlCause:               goalNullableString(string(checkpoint.ControlCause)),
		TurnsUsed:                  int64(checkpoint.TurnsUsed),
		TurnLimit:                  int64(checkpoint.TurnLimit),
		BrokenStreak:               int64(checkpoint.BrokenStreak),
		RecoveryStreak:             int64(checkpoint.RecoveryStreak),
		TaskRunID:                  goalNullableString(checkpoint.TaskRunID),
		QueueEntryID:               goalNullableString(checkpoint.QueueEntryID),
		PromptID:                   goalNullableString(checkpoint.PromptID),
		PromptKind:                 goalNullableString(checkpoint.PromptKind),
		PromptAttempt:              int64(checkpoint.PromptAttempt),
		ContextState:               checkpoint.ContextState,
		UsageSequence:              goalNullableInt64(checkpoint.UsageSequence),
		UsagePendingAfterSequence:  goalNullableInt64(checkpoint.UsagePendingAfterSequence),
		CompactionBaselineUsed:     goalNullableInt64(checkpoint.CompactionBaselineUsed),
		CompactionRecoveryRequired: int64(boolToInt(checkpoint.CompactionRecoveryRequired)),
		SessionID: goalNullableString(
			checkpoint.SessionID,
		),
		BindingHandle: goalNullableString(checkpoint.BindingHandle),
		BindingEpoch: goalNullablePositiveInt64(
			checkpoint.BindingEpoch,
		),
		ContextNudgeRatio:    checkpoint.ContextNudgeRatio,
		JudgeAttemptID:       goalNullableString(checkpoint.JudgeAttemptID),
		ControlGrantConsumed: int64(goalGrantConsumed(checkpoint.ControlGrant)),
		UpdatedAt:            store.FormatTimestamp(checkpoint.UpdatedAt),
	}
	applyGoalGrantInsertParams(&params, checkpoint.ControlGrant)
	applyGoalCancelInsertParams(&params, checkpoint.CompactionCancel)
	applyGoalReportInsertParams(&params, checkpoint.ReportIntent)
	return params
}

func applyGoalGrantInsertParams(params *sqlcgen.InsertGoalCheckpointParams, grant *goal.ControlGrant) {
	if grant == nil {
		return
	}
	params.ControlGrantID = grant.ID
	params.ControlGrantKind = goalNullableString(string(grant.Kind))
	params.ControlGrantCause = goalNullableString(string(grant.Cause))
	turn := int64(grant.Turn)
	params.ControlGrantTurn = goalNullableInt64(&turn)
	params.ControlGrantScope = goalNullableString(string(grant.Scope))
	params.ControlGrantConsumed = int64(boolToInt(grant.Consumed))
}

func applyGoalCancelInsertParams(params *sqlcgen.InsertGoalCheckpointParams, intent *goal.CompactionCancelIntent) {
	if intent == nil {
		return
	}
	params.CompactionCancelPromptID = goalNullableString(intent.PromptID)
	params.CompactionCancelCause = goalNullableString(intent.Cause)
	params.CompactionCancelRequestedAt = goalNullableTimestampString(&intent.RequestedAt)
}

func applyGoalReportInsertParams(params *sqlcgen.InsertGoalCheckpointParams, intent *goal.ReportIntent) {
	if intent == nil {
		return
	}
	params.ReportPromptID = goalNullableString(intent.PromptID)
	params.ReportStatus = goalNullableString(intent.Status)
	params.ReportEvidenceRef = goalNullableString(intent.EvidenceRef)
	params.ReportBindingEpoch = goalNullableInt64(&intent.BindingEpoch)
	params.ReportActorKind = goalNullableString(intent.ActorKind)
	params.ReportActorID = goalNullableString(intent.ActorID)
	params.ReportRecordedAt = goalNullableTimestampString(&intent.RecordedAt)
}

func goalNullableTimestampString(value *time.Time) sql.NullString {
	if value == nil || value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(value.UTC()), Valid: true}
}
