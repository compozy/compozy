package globaldb

import (
	"database/sql"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func goalSessionOutboxFromGenerated(row sqlcgen.LoopGoalSessionOutbox) goal.SessionOutboxEvent {
	event := goal.SessionOutboxEvent{
		ID: row.ID, EventID: row.EventID, WorkspaceID: loop.WorkspaceID(row.WorkspaceID),
		OriginSessionID: row.OriginSessionID, LoopRunID: loop.RunID(row.LoopRunID),
		Cause: goal.SessionOutboxCause(row.Cause), CreatedAt: row.CreatedAt.UTC(),
	}
	if row.BoundSessionID.Valid {
		value := row.BoundSessionID.String
		event.BoundSessionID = &value
	}
	if row.DeliveredAt.Valid {
		value := row.DeliveredAt.Time.UTC()
		event.DeliveredAt = &value
	}
	return event
}

func goalSessionCleanupFromGenerated(row sqlcgen.LoopGoalSessionCleanup) goal.SessionCleanupObligation {
	obligation := goal.SessionCleanupObligation{
		ID: row.ID, CleanupID: row.CleanupID, WorkspaceID: loop.WorkspaceID(row.WorkspaceID),
		LoopRunID: loop.RunID(row.LoopRunID), Handle: row.Handle, BindingEpoch: row.BindingEpoch,
		SessionID: row.SessionID, Cause: goal.SessionCleanupCause(row.Cause), CreatedAt: row.CreatedAt.UTC(),
	}
	if row.CompletedAt.Valid {
		value := row.CompletedAt.Time.UTC()
		obligation.CompletedAt = &value
	}
	return obligation
}

func goalCheckpointFromGenerated(
	row *sqlcgen.LoopGoalCheckpoint,
	workspaceID loop.WorkspaceID,
) (goal.Checkpoint, error) {
	checkpoint := goal.Checkpoint{
		Key: goal.TurnKey{
			WorkspaceID: workspaceID,
			LoopRunID:   loop.RunID(row.LoopRunID),
			Generation:  int(row.Generation),
			NodeID:      dsl.NodeID(row.NodeID),
			ItemIndex:   int(row.ItemIndex),
		},
		ControlEpoch:      row.ControlEpoch,
		Phase:             row.Phase,
		Status:            row.GoalStatus,
		TurnsUsed:         int(row.TurnsUsed),
		TurnLimit:         int(row.TurnLimit),
		BrokenStreak:      int(row.BrokenStreak),
		RecoveryStreak:    int(row.RecoveryStreak),
		PromptAttempt:     int(row.PromptAttempt),
		ContextState:      row.ContextState,
		ContextNudgeRatio: row.ContextNudgeRatio,
	}
	fields := goalCheckpointScanFields{
		workspaceID: workspaceID, controlActorKind: row.ControlActorKind, controlActorID: row.ControlActorID,
		controlRequestedAt: goalOptionalTimeValue(row.ControlRequestedAt), controlCause: row.ControlCause,
		taskRunID:    row.TaskRunID,
		queueEntryID: row.QueueEntryID, promptID: row.PromptID, promptKind: row.PromptKind,
		usageSequence: row.UsageSequence, usagePendingAfterSequence: row.UsagePendingAfterSequence,
		compactionBaselineUsed:     row.CompactionBaselineUsed,
		compactionRecoveryRequired: int(row.CompactionRecoveryRequired), sessionID: row.SessionID,
		bindingHandle: row.BindingHandle, bindingEpoch: row.BindingEpoch, controlGrantKind: row.ControlGrantKind,
		controlGrantCause: row.ControlGrantCause, controlGrantTurn: row.ControlGrantTurn,
		controlGrantScope: row.ControlGrantScope, controlGrantConsumed: int(row.ControlGrantConsumed),
		judgeAttemptID: row.JudgeAttemptID, cancelPromptID: row.CompactionCancelPromptID,
		cancelCause:       row.CompactionCancelCause,
		cancelRequestedAt: goalOptionalTimeValue(row.CompactionCancelRequestedAt),
		reportPromptID:    row.ReportPromptID, reportStatus: row.ReportStatus,
		reportEvidenceRef: row.ReportEvidenceRef, reportBindingEpoch: row.ReportBindingEpoch,
		reportActorKind: row.ReportActorKind, reportActorID: row.ReportActorID,
		reportRecordedAt: goalOptionalTimeValue(row.ReportRecordedAt), updatedAt: row.UpdatedAt,
	}
	if row.ControlGrantID > 0 {
		checkpoint.ControlGrant = &goal.ControlGrant{ID: row.ControlGrantID}
	}
	if err := fields.apply(&checkpoint); err != nil {
		return goal.Checkpoint{}, err
	}
	return checkpoint, nil
}

func goalOptionalTimeValue(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time.UTC()
}

func goalTurnFromGenerated(
	row sqlcgen.LoopGoalTurn,
	workspaceID loop.WorkspaceID,
	runID loop.RunID,
) (goal.Turn, error) {
	turn := goal.Turn{
		Seq: row.Seq, Key: goal.TurnKey{WorkspaceID: workspaceID, LoopRunID: runID, Generation: int(row.Generation),
			NodeID: dsl.NodeID(row.NodeID), ItemIndex: int(row.ItemIndex)},
		Turn: int(row.Turn), SessionID: row.SessionID, BindingHandle: row.BindingHandle,
		BindingEpoch: row.BindingEpoch, PromptID: row.PromptID, PromptAttempt: int(row.PromptAttempt),
		UsageBaseTokens: row.UsageBaseTokens, ActorKind: row.ActorKind, ActorID: row.ActorID,
	}
	fields := goalTurnScanFields{
		resultStatus: row.ResultStatus, stopReason: row.StopReason, reasonCode: row.ReasonCode,
		verdictOutcome: row.VerdictOutcome, blockingJSON: row.BlockingJson, evidenceRef: row.EvidenceRef,
		promptRef: row.PromptRef, tokensUsed: row.TokensUsed, startedAtRaw: row.StartedAt,
		endedAtRaw: goalOptionalTimeValue(row.EndedAt),
	}
	if err := fields.apply(&turn); err != nil {
		return goal.Turn{}, err
	}
	return turn, nil
}
