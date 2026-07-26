package globaldb

import (
	"database/sql"
	"fmt"
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
)

func scanTaskRecord(scanner rowScanner) (taskpkg.Task, error) {
	record, fields, err := scanTaskRecordColumns(scanner)
	if err != nil {
		return taskpkg.Task{}, err
	}
	return taskRecordFromFields(record, &fields)
}

func taskRecordFromFields(record taskpkg.Task, fields *taskScanFields) (taskpkg.Task, error) {
	assignScannedTaskRecord(
		&record,
		fields.identifier,
		fields.scope,
		fields.workspaceID,
		fields.parentTaskID,
		fields.description,
		fields.priority,
		fields.maxAttempts,
		fields.status,
		fields.approvalPolicy,
		fields.approvalState,
		fields.ownerKind,
		fields.ownerRef,
		fields.createdByKind,
		fields.originKind,
	)
	record.CurrentRunID = strings.TrimSpace(fields.currentRunID.String)
	record.LatestEventSeq = fields.latestEventSeq
	record.AutoEnqueueOnReady = fields.autoEnqueueOnReady != 0
	record.Paused = fields.paused != 0
	record.PausedBy = strings.TrimSpace(fields.pausedBy)
	record.PausedReason = strings.TrimSpace(fields.pausedReason)
	attention := taskpkg.NeedsAttention{
		Reason: taskNullStringValue(fields.needsAttentionReason),
	}
	if fields.needsAttentionByKind.Valid || fields.needsAttentionByRef.Valid {
		attention.By = taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKind(strings.TrimSpace(fields.needsAttentionByKind.String)),
			Ref:  strings.TrimSpace(fields.needsAttentionByRef.String),
		}
	}
	record.WakeCreator = fields.wakeCreator != 0
	if err := assignTaskRecordTimestamps(
		&record,
		fields.createdAtRaw,
		fields.updatedAtRaw,
		fields.closedAtRaw,
	); err != nil {
		return taskpkg.Task{}, err
	}
	if err := assignTaskMetadata(&record.Metadata, fields.metadataJSON, "task.metadata_json"); err != nil {
		return taskpkg.Task{}, err
	}
	if err := assignNullableTaskTimestamp(&record.PausedAt, fields.pausedAtRaw); err != nil {
		return taskpkg.Task{}, err
	}
	if err := assignNullableTaskTimestamp(&attention.At, fields.needsAttentionAtRaw); err != nil {
		return taskpkg.Task{}, err
	}
	if attention.Reason != "" || !attention.At.IsZero() || !attention.By.IsZero() {
		record.NeedsAttention = &attention
	}
	record = normalizeTaskRecord(record)
	if err := record.Validate(); err != nil {
		return taskpkg.Task{}, err
	}

	return record, nil
}

type taskScanFields struct {
	identifier           sql.NullString
	scope                string
	workspaceID          sql.NullString
	parentTaskID         sql.NullString
	description          sql.NullString
	priority             string
	maxAttempts          int
	autoEnqueueOnReady   int
	status               string
	approvalPolicy       string
	approvalState        string
	ownerKind            sql.NullString
	ownerRef             sql.NullString
	createdByKind        string
	originKind           string
	createdAtRaw         string
	updatedAtRaw         string
	closedAtRaw          sql.NullString
	currentRunID         sql.NullString
	latestEventSeq       int64
	paused               int
	pausedBy             string
	pausedAtRaw          sql.NullString
	pausedReason         string
	needsAttentionReason sql.NullString
	needsAttentionAtRaw  sql.NullString
	needsAttentionByKind sql.NullString
	needsAttentionByRef  sql.NullString
	wakeCreator          int
	metadataJSON         sql.NullString
}

func scanTaskRecordColumns(scanner rowScanner) (taskpkg.Task, taskScanFields, error) {
	var record taskpkg.Task
	var fields taskScanFields
	if err := scanner.Scan(
		&record.ID,
		&fields.identifier,
		&fields.scope,
		&fields.workspaceID,
		&fields.parentTaskID,
		&record.Title,
		&fields.description,
		&fields.priority,
		&fields.maxAttempts,
		&fields.autoEnqueueOnReady,
		&fields.status,
		&fields.approvalPolicy,
		&fields.approvalState,
		&fields.ownerKind,
		&fields.ownerRef,
		&fields.createdByKind,
		&record.CreatedBy.Ref,
		&fields.originKind,
		&record.Origin.Ref,
		&fields.createdAtRaw,
		&fields.updatedAtRaw,
		&fields.closedAtRaw,
		&fields.currentRunID,
		&fields.latestEventSeq,
		&fields.paused,
		&fields.pausedBy,
		&fields.pausedAtRaw,
		&fields.pausedReason,
		&fields.needsAttentionReason,
		&fields.needsAttentionAtRaw,
		&fields.needsAttentionByKind,
		&fields.needsAttentionByRef,
		&fields.wakeCreator,
		&fields.metadataJSON,
	); err != nil {
		return taskpkg.Task{}, taskScanFields{}, fmt.Errorf("store: scan task: %w", err)
	}
	return record, fields, nil
}
