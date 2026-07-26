package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

func scanTaskRunRecord(scanner rowScanner) (taskpkg.Run, error) {
	var run taskpkg.Run
	var taskID sql.NullString
	var workspaceID sql.NullString
	var fields taskRunScanFields
	if err := scanner.Scan(
		&run.ID,
		&taskID,
		&workspaceID,
		&fields.runKind,
		&fields.loopRunID,
		&fields.status,
		&run.Attempt,
		&run.RecoveryCount,
		&fields.previousRunID,
		&fields.failureKind,
		&fields.claimedByKind,
		&fields.claimedByRef,
		&fields.sessionID,
		&fields.originKind,
		&run.Origin.Ref,
		&fields.idempotencyKey,
		&fields.networkSpecJSON,
		&fields.networkMode,
		&fields.networkChannel,
		&fields.networkSource,
		&fields.designationGroupID,
		&fields.claimToken,
		&fields.claimTokenHash,
		&fields.leaseUntilRaw,
		&fields.heartbeatAtRaw,
		&fields.queuedAtRaw,
		&fields.claimedAtRaw,
		&fields.startedAtRaw,
		&fields.endedAtRaw,
		&fields.tokensUsed,
		&fields.runErr,
		&fields.metadataJSON,
		&fields.resultJSON,
		&fields.reviewRequired,
		&fields.reviewRequestRound,
		&fields.reviewPolicySnapshot,
		&fields.reviewRequestID,
		&fields.parentRunID,
		&fields.reviewID,
		&fields.reviewRound,
		&fields.continuationReason,
		&fields.missingWorkJSON,
		&fields.nextRoundGuidance,
		&fields.networkWakeID,
		&fields.networkTargetSessionID,
		&fields.networkOwnerKey,
	); err != nil {
		return taskpkg.Run{}, fmt.Errorf("store: scan task run: %w", err)
	}
	run.TaskID = taskNullStringValue(taskID)
	run.WorkspaceID = taskNullStringValue(workspaceID)
	return (&fields).record(run)
}

type taskRunScanFields struct {
	status                 string
	runKind                string
	loopRunID              sql.NullString
	previousRunID          sql.NullString
	failureKind            string
	claimedByKind          sql.NullString
	claimedByRef           sql.NullString
	sessionID              sql.NullString
	originKind             string
	idempotencyKey         sql.NullString
	networkSpecJSON        string
	networkMode            string
	networkChannel         sql.NullString
	networkSource          string
	designationGroupID     string
	claimToken             sql.NullString
	claimTokenHash         sql.NullString
	leaseUntilRaw          sql.NullString
	heartbeatAtRaw         sql.NullString
	queuedAtRaw            string
	claimedAtRaw           sql.NullString
	startedAtRaw           sql.NullString
	endedAtRaw             sql.NullString
	tokensUsed             int64
	runErr                 sql.NullString
	metadataJSON           sql.NullString
	resultJSON             sql.NullString
	reviewRequired         bool
	reviewRequestRound     int
	reviewPolicySnapshot   string
	reviewRequestID        sql.NullString
	parentRunID            sql.NullString
	reviewID               sql.NullString
	reviewRound            int
	continuationReason     string
	missingWorkJSON        string
	nextRoundGuidance      string
	networkWakeID          sql.NullString
	networkTargetSessionID sql.NullString
	networkOwnerKey        sql.NullString
}

func (fields *taskRunScanFields) record(run taskpkg.Run) (taskpkg.Run, error) {
	assignScannedTaskRunRecord(
		&run,
		fields.runKind,
		fields.loopRunID,
		fields.status,
		fields.previousRunID,
		fields.failureKind,
		fields.claimedByKind,
		fields.claimedByRef,
		fields.sessionID,
		fields.originKind,
		fields.idempotencyKey,
		fields.designationGroupID,
		fields.claimToken,
		fields.claimTokenHash,
		fields.runErr,
	)
	networkSpec, err := decodeParticipationSnapshot(
		run.WorkspaceID,
		fields.networkSpecJSON,
		fields.networkMode,
		fields.networkChannel,
		fields.networkSource,
	)
	if err != nil {
		return taskpkg.Run{}, err
	}
	run.SetNetworkState(
		networkSpec,
		taskNullStringValue(fields.networkWakeID),
		taskNullStringValue(fields.networkTargetSessionID),
		taskNullStringValue(fields.networkOwnerKey),
	)
	if err := assignTaskRunTimestamps(
		&run,
		fields.queuedAtRaw,
		fields.claimedAtRaw,
		fields.startedAtRaw,
		fields.endedAtRaw,
		fields.leaseUntilRaw,
		fields.heartbeatAtRaw,
	); err != nil {
		return taskpkg.Run{}, err
	}
	if err := assignTaskMetadata(&run.Metadata, fields.metadataJSON, "task_run.metadata_json"); err != nil {
		return taskpkg.Run{}, err
	}
	if err := assignTaskMetadata(&run.Result, fields.resultJSON, "task_run.result_json"); err != nil {
		return taskpkg.Run{}, err
	}
	run.TokensUsed = fields.tokensUsed
	run.Review = taskRunReviewLineageFromScan(fields)
	run = normalizeTaskRunRecord(run)
	if err := run.Validate(); err != nil {
		return taskpkg.Run{}, err
	}

	return run, nil
}

func assignScannedTaskRecord(
	record *taskpkg.Task,
	identifier sql.NullString,
	scope string,
	workspaceID sql.NullString,
	parentTaskID sql.NullString,
	description sql.NullString,
	priority string,
	maxAttempts int,
	status string,
	approvalPolicy string,
	approvalState string,
	ownerKind sql.NullString,
	ownerRef sql.NullString,
	createdByKind string,
	originKind string,
) {
	record.Identifier = taskNullStringValue(identifier)
	record.Scope = taskpkg.Scope(strings.TrimSpace(scope))
	record.WorkspaceID = taskNullStringValue(workspaceID)
	record.ParentTaskID = taskNullStringValue(parentTaskID)
	record.Description = taskNullStringValue(description)
	record.Priority = taskpkg.Priority(strings.TrimSpace(priority))
	record.MaxAttempts = maxAttempts
	record.Status = taskpkg.Status(strings.TrimSpace(status))
	record.ApprovalPolicy = taskpkg.ApprovalPolicy(strings.TrimSpace(approvalPolicy))
	record.ApprovalState = taskpkg.ApprovalState(strings.TrimSpace(approvalState))
	record.CreatedBy.Kind = taskpkg.ActorKind(strings.TrimSpace(createdByKind))
	record.Origin.Kind = taskpkg.OriginKind(strings.TrimSpace(originKind))
	if ownerKind.Valid || ownerRef.Valid {
		record.Owner = &taskpkg.Ownership{
			Kind: taskpkg.OwnerKind(strings.TrimSpace(ownerKind.String)),
			Ref:  strings.TrimSpace(ownerRef.String),
		}
	}
}

func assignTaskRecordTimestamps(
	record *taskpkg.Task,
	createdAtRaw string,
	updatedAtRaw string,
	closedAtRaw sql.NullString,
) error {
	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return err
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return err
	}
	record.CreatedAt = createdAt
	record.UpdatedAt = updatedAt
	if !closedAtRaw.Valid {
		return nil
	}
	closedAt, err := store.ParseTimestamp(closedAtRaw.String)
	if err != nil {
		return err
	}
	record.ClosedAt = closedAt
	return nil
}

func assignScannedTaskRunRecord(
	run *taskpkg.Run,
	runKind string,
	loopRunID sql.NullString,
	status string,
	previousRunID sql.NullString,
	failureKind string,
	claimedByKind sql.NullString,
	claimedByRef sql.NullString,
	sessionID sql.NullString,
	originKind string,
	idempotencyKey sql.NullString,
	designationGroupID string,
	_ sql.NullString,
	claimTokenHash sql.NullString,
	runErr sql.NullString,
) {
	run.RunKind = taskpkg.ParseRunKind(runKind)
	run.LoopRunID = taskNullStringValue(loopRunID)
	run.Status = taskpkg.ParseRunStatus(status)
	run.PreviousRunID = taskNullStringValue(previousRunID)
	run.FailureKind = strings.TrimSpace(failureKind)
	if claimedByKind.Valid || claimedByRef.Valid {
		run.ClaimedBy = &taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKind(strings.TrimSpace(claimedByKind.String)),
			Ref:  strings.TrimSpace(claimedByRef.String),
		}
	}
	run.SessionID = taskNullStringValue(sessionID)
	run.Origin.Kind = taskpkg.OriginKind(strings.TrimSpace(originKind))
	run.IdempotencyKey = taskNullStringValue(idempotencyKey)
	run.DesignationGroupID = strings.TrimSpace(designationGroupID)
	run.ClaimTokenHash = taskNullStringValue(claimTokenHash)
	run.Error = taskNullStringValue(runErr)
}

func assignTaskRunTimestamps(
	run *taskpkg.Run,
	queuedAtRaw string,
	claimedAtRaw sql.NullString,
	startedAtRaw sql.NullString,
	endedAtRaw sql.NullString,
	leaseUntilRaw sql.NullString,
	heartbeatAtRaw sql.NullString,
) error {
	queuedAt, err := store.ParseTimestamp(queuedAtRaw)
	if err != nil {
		return err
	}
	run.QueuedAt = queuedAt
	if err := assignNullableTaskTimestamp(&run.ClaimedAt, claimedAtRaw); err != nil {
		return err
	}
	if err := assignNullableTaskTimestamp(&run.StartedAt, startedAtRaw); err != nil {
		return err
	}
	if err := assignNullableTaskTimestamp(&run.EndedAt, endedAtRaw); err != nil {
		return err
	}
	if err := assignNullableTaskTimestamp(&run.LeaseUntil, leaseUntilRaw); err != nil {
		return err
	}
	return assignNullableTaskTimestamp(&run.HeartbeatAt, heartbeatAtRaw)
}

func assignNullableTaskTimestamp(target *time.Time, raw sql.NullString) error {
	if !raw.Valid {
		return nil
	}
	parsed, err := store.ParseTimestamp(raw.String)
	if err != nil {
		return err
	}
	*target = parsed
	return nil
}

func assignTaskMetadata(target *json.RawMessage, raw sql.NullString, field string) error {
	metadata, err := decodeTaskJSON(raw, field)
	if err != nil {
		return err
	}
	*target = metadata
	return nil
}
