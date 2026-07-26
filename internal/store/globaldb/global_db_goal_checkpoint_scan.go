package globaldb

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
)

type goalCheckpointScanFields struct {
	workspaceID                loop.WorkspaceID
	controlActorKind           sql.NullString
	controlActorID             sql.NullString
	controlRequestedAt         any
	controlCause               sql.NullString
	taskRunID                  sql.NullString
	queueEntryID               sql.NullString
	promptID                   sql.NullString
	promptKind                 sql.NullString
	usageSequence              sql.NullInt64
	usagePendingAfterSequence  sql.NullInt64
	compactionBaselineUsed     sql.NullInt64
	compactionRecoveryRequired int
	sessionID                  sql.NullString
	bindingHandle              sql.NullString
	bindingEpoch               sql.NullInt64
	controlGrantKind           sql.NullString
	controlGrantCause          sql.NullString
	controlGrantTurn           sql.NullInt64
	controlGrantScope          sql.NullString
	controlGrantConsumed       int
	judgeAttemptID             sql.NullString
	cancelPromptID             sql.NullString
	cancelCause                sql.NullString
	cancelRequestedAt          any
	reportPromptID             sql.NullString
	reportStatus               sql.NullString
	reportEvidenceRef          sql.NullString
	reportBindingEpoch         sql.NullInt64
	reportActorKind            sql.NullString
	reportActorID              sql.NullString
	reportRecordedAt           any
	updatedAt                  any
}

func (fields *goalCheckpointScanFields) apply(checkpoint *goal.Checkpoint) error {
	checkpoint.Key.WorkspaceID = fields.workspaceID
	checkpoint.ControlActorKind = strings.TrimSpace(fields.controlActorKind.String)
	checkpoint.ControlActorID = strings.TrimSpace(fields.controlActorID.String)
	checkpoint.ControlCause = loop.ReasonCode(strings.TrimSpace(fields.controlCause.String))
	checkpoint.TaskRunID = strings.TrimSpace(fields.taskRunID.String)
	checkpoint.QueueEntryID = strings.TrimSpace(fields.queueEntryID.String)
	checkpoint.PromptID = strings.TrimSpace(fields.promptID.String)
	checkpoint.PromptKind = strings.TrimSpace(fields.promptKind.String)
	checkpoint.SessionID = strings.TrimSpace(fields.sessionID.String)
	checkpoint.BindingHandle = strings.TrimSpace(fields.bindingHandle.String)
	checkpoint.JudgeAttemptID = strings.TrimSpace(fields.judgeAttemptID.String)
	if fields.bindingEpoch.Valid {
		checkpoint.BindingEpoch = fields.bindingEpoch.Int64
	}
	if fields.usageSequence.Valid {
		value := fields.usageSequence.Int64
		checkpoint.UsageSequence = &value
	}
	if fields.usagePendingAfterSequence.Valid {
		value := fields.usagePendingAfterSequence.Int64
		checkpoint.UsagePendingAfterSequence = &value
	}
	if fields.compactionBaselineUsed.Valid {
		value := fields.compactionBaselineUsed.Int64
		checkpoint.CompactionBaselineUsed = &value
	}
	checkpoint.CompactionRecoveryRequired = fields.compactionRecoveryRequired != 0
	var err error
	if checkpoint.ControlRequestedAt, err = parseOptionalGoalTimestampValue(
		fields.controlRequestedAt,
		"checkpoint control_requested_at",
	); err != nil {
		return err
	}
	if err := fields.applyGrant(checkpoint); err != nil {
		return err
	}
	if err := fields.applyCancel(checkpoint); err != nil {
		return err
	}
	if err := fields.applyReport(checkpoint); err != nil {
		return err
	}
	checkpoint.UpdatedAt, err = parseGoalTimestampValue(fields.updatedAt, "checkpoint updated_at")
	return err
}

func (fields *goalCheckpointScanFields) applyGrant(checkpoint *goal.Checkpoint) error {
	if checkpoint.ControlGrant == nil || checkpoint.ControlGrant.ID == 0 {
		checkpoint.ControlGrant = nil
		return nil
	}
	checkpoint.ControlGrant.Kind = goal.ControlGrantKind(strings.TrimSpace(fields.controlGrantKind.String))
	checkpoint.ControlGrant.Cause = loop.ReasonCode(strings.TrimSpace(fields.controlGrantCause.String))
	checkpoint.ControlGrant.Scope = goal.ControlGrantScope(strings.TrimSpace(fields.controlGrantScope.String))
	checkpoint.ControlGrant.Consumed = fields.controlGrantConsumed != 0
	if !fields.controlGrantTurn.Valid {
		return fmt.Errorf("store: goal checkpoint grant %d has no turn", checkpoint.ControlGrant.ID)
	}
	checkpoint.ControlGrant.Turn = int(fields.controlGrantTurn.Int64)
	return nil
}

func (fields *goalCheckpointScanFields) applyCancel(checkpoint *goal.Checkpoint) error {
	if !fields.cancelCause.Valid {
		return nil
	}
	requestedAt, err := parseGoalTimestampValue(fields.cancelRequestedAt, "checkpoint cancel requested_at")
	if err != nil {
		return err
	}
	checkpoint.CompactionCancel = &goal.CompactionCancelIntent{
		PromptID:    strings.TrimSpace(fields.cancelPromptID.String),
		Cause:       strings.TrimSpace(fields.cancelCause.String),
		RequestedAt: requestedAt,
	}
	return nil
}

func (fields *goalCheckpointScanFields) applyReport(checkpoint *goal.Checkpoint) error {
	if !fields.reportStatus.Valid {
		return nil
	}
	recordedAt, err := parseGoalTimestampValue(fields.reportRecordedAt, "checkpoint report recorded_at")
	if err != nil {
		return err
	}
	checkpoint.ReportIntent = &goal.ReportIntent{
		PromptID:     strings.TrimSpace(fields.reportPromptID.String),
		Status:       strings.TrimSpace(fields.reportStatus.String),
		EvidenceRef:  strings.TrimSpace(fields.reportEvidenceRef.String),
		BindingEpoch: fields.reportBindingEpoch.Int64,
		ActorKind:    strings.TrimSpace(fields.reportActorKind.String),
		ActorID:      strings.TrimSpace(fields.reportActorID.String),
		RecordedAt:   recordedAt,
	}
	return nil
}
