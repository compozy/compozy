package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func taskFromGenerated(row *sqlcgen.GetTaskRow) (taskpkg.Task, error) {
	record := taskpkg.Task{
		ID:    row.ID,
		Title: row.Title,
		CreatedBy: taskpkg.ActorIdentity{
			Ref: row.CreatedByRef,
		},
		Origin: taskpkg.Origin{
			Ref: row.OriginRef,
		},
	}
	fields := taskScanFields{
		identifier:           row.Identifier,
		scope:                row.Scope,
		workspaceID:          row.WorkspaceID,
		parentTaskID:         row.ParentTaskID,
		description:          row.Description,
		priority:             row.Priority,
		maxAttempts:          int(row.MaxAttempts),
		autoEnqueueOnReady:   int(row.AutoEnqueueOnReady),
		status:               row.Status,
		approvalPolicy:       row.ApprovalPolicy,
		approvalState:        row.ApprovalState,
		ownerKind:            row.OwnerKind,
		ownerRef:             row.OwnerRef,
		createdByKind:        row.CreatedByKind,
		originKind:           row.OriginKind,
		createdAtRaw:         row.CreatedAt,
		updatedAtRaw:         row.UpdatedAt,
		closedAtRaw:          row.ClosedAt,
		currentRunID:         row.CurrentRunID,
		latestEventSeq:       row.LatestEventSeq,
		paused:               int(row.Paused),
		pausedBy:             row.PausedBy,
		pausedAtRaw:          row.PausedAt,
		pausedReason:         row.PausedReason,
		needsAttentionReason: row.NeedsAttentionReason,
		needsAttentionAtRaw:  row.NeedsAttentionAt,
		needsAttentionByKind: row.NeedsAttentionByKind,
		needsAttentionByRef:  row.NeedsAttentionByRef,
		wakeCreator:          int(row.WakeCreator),
		metadataJSON:         row.MetadataJson,
	}
	return taskRecordFromFields(record, &fields)
}

func taskRunFromGenerated(row *sqlcgen.GetTaskRunRow) (taskpkg.Run, error) {
	attempt, err := taskAttemptFromInt64(row.Attempt)
	if err != nil {
		return taskpkg.Run{}, err
	}
	recoveryCount, err := taskAttemptFromInt64(row.RecoveryCount)
	if err != nil {
		return taskpkg.Run{}, err
	}
	run := taskpkg.Run{
		ID:            row.ID,
		TaskID:        taskNullStringValue(row.TaskID),
		WorkspaceID:   taskNullStringValue(row.WorkspaceID),
		Attempt:       attempt,
		RecoveryCount: recoveryCount,
		Origin: taskpkg.Origin{
			Ref: row.OriginRef,
		},
	}
	fields := taskRunScanFields{
		status:                 row.Status,
		runKind:                row.RunKind,
		loopRunID:              row.LoopRunID,
		previousRunID:          row.PreviousRunID,
		failureKind:            row.FailureKind,
		claimedByKind:          row.ClaimedByKind,
		claimedByRef:           row.ClaimedByRef,
		sessionID:              row.SessionID,
		originKind:             row.OriginKind,
		idempotencyKey:         row.IdempotencyKey,
		networkSpecJSON:        row.NetworkSpecJson,
		networkMode:            row.NetworkMode,
		networkChannel:         row.NetworkChannel,
		networkSource:          row.NetworkSource,
		designationGroupID:     row.DesignationGroupID,
		claimToken:             nullableTaskString(row.ClaimToken),
		claimTokenHash:         row.ClaimTokenHash,
		leaseUntilRaw:          row.LeaseUntil,
		heartbeatAtRaw:         row.HeartbeatAt,
		queuedAtRaw:            row.QueuedAt,
		claimedAtRaw:           row.ClaimedAt,
		startedAtRaw:           row.StartedAt,
		endedAtRaw:             row.EndedAt,
		tokensUsed:             row.TokensUsed,
		runErr:                 row.Error,
		metadataJSON:           row.MetadataJson,
		resultJSON:             row.ResultJson,
		reviewRequired:         row.ReviewRequired,
		reviewRequestRound:     int(row.ReviewRequestRound),
		reviewPolicySnapshot:   row.ReviewPolicySnapshot,
		reviewRequestID:        row.ReviewRequestID,
		parentRunID:            row.ParentRunID,
		reviewID:               row.ReviewID,
		reviewRound:            int(row.ReviewRound),
		continuationReason:     row.ContinuationReason,
		missingWorkJSON:        row.MissingWorkJson,
		nextRoundGuidance:      row.NextRoundGuidance,
		networkWakeID:          row.NetworkWakeID,
		networkTargetSessionID: row.NetworkTargetSessionID,
		networkOwnerKey:        row.NetworkOwnerKey,
	}
	return fields.record(run)
}

func taskRunFromStatusGenerated(row *sqlcgen.ListTaskRunsByStatusRow) (taskpkg.Run, error) {
	attempt, err := taskAttemptFromInt64(row.Attempt)
	if err != nil {
		return taskpkg.Run{}, err
	}
	recoveryCount, err := taskAttemptFromInt64(row.RecoveryCount)
	if err != nil {
		return taskpkg.Run{}, err
	}
	run := taskpkg.Run{
		ID:            row.ID,
		TaskID:        taskNullStringValue(row.TaskID),
		WorkspaceID:   taskNullStringValue(row.WorkspaceID),
		Attempt:       attempt,
		RecoveryCount: recoveryCount,
		Origin:        taskpkg.Origin{Ref: row.OriginRef},
	}
	fields := taskRunScanFields{
		status:                 row.Status,
		runKind:                row.RunKind,
		loopRunID:              row.LoopRunID,
		previousRunID:          row.PreviousRunID,
		failureKind:            row.FailureKind,
		claimedByKind:          row.ClaimedByKind,
		claimedByRef:           row.ClaimedByRef,
		sessionID:              row.SessionID,
		originKind:             row.OriginKind,
		idempotencyKey:         row.IdempotencyKey,
		networkSpecJSON:        row.NetworkSpecJson,
		networkMode:            row.NetworkMode,
		networkChannel:         row.NetworkChannel,
		networkSource:          row.NetworkSource,
		designationGroupID:     row.DesignationGroupID,
		claimToken:             nullableTaskString(row.ClaimToken),
		claimTokenHash:         row.ClaimTokenHash,
		leaseUntilRaw:          row.LeaseUntil,
		heartbeatAtRaw:         row.HeartbeatAt,
		queuedAtRaw:            row.QueuedAt,
		claimedAtRaw:           row.ClaimedAt,
		startedAtRaw:           row.StartedAt,
		endedAtRaw:             row.EndedAt,
		tokensUsed:             row.TokensUsed,
		runErr:                 row.Error,
		metadataJSON:           row.MetadataJson,
		resultJSON:             row.ResultJson,
		reviewRequired:         row.ReviewRequired,
		reviewRequestRound:     int(row.ReviewRequestRound),
		reviewPolicySnapshot:   row.ReviewPolicySnapshot,
		reviewRequestID:        row.ReviewRequestID,
		parentRunID:            row.ParentRunID,
		reviewID:               row.ReviewID,
		reviewRound:            int(row.ReviewRound),
		continuationReason:     row.ContinuationReason,
		missingWorkJSON:        row.MissingWorkJson,
		nextRoundGuidance:      row.NextRoundGuidance,
		networkWakeID:          row.NetworkWakeID,
		networkTargetSessionID: row.NetworkTargetSessionID,
		networkOwnerKey:        row.NetworkOwnerKey,
	}
	return fields.record(run)
}

func taskAttemptFromInt64(value int64) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("store: task run attempt %d exceeds int32 range", value)
	}
	return int32(value), nil
}

func insertTaskParams(record taskpkg.Task) sqlcgen.InsertTaskParams {
	return sqlcgen.InsertTaskParams{
		ID:                 record.ID,
		Identifier:         nullableTaskString(record.Identifier),
		Scope:              string(record.Scope),
		WorkspaceID:        nullableTaskString(record.WorkspaceID),
		ParentTaskID:       nullableTaskString(record.ParentTaskID),
		Title:              record.Title,
		Description:        nullableTaskString(record.Description),
		Priority:           string(record.Priority),
		MaxAttempts:        int64(record.MaxAttempts),
		AutoEnqueueOnReady: boolInt64(record.AutoEnqueueOnReady),
		Status:             string(record.Status),
		ApprovalPolicy:     string(record.ApprovalPolicy),
		ApprovalState:      string(record.ApprovalState),
		OwnerKind:          nullableTaskOwnerKind(record.Owner),
		OwnerRef:           nullableTaskOwnerRef(record.Owner),
		CreatedByKind:      string(record.CreatedBy.Kind),
		CreatedByRef:       record.CreatedBy.Ref,
		OriginKind:         string(record.Origin.Kind),
		OriginRef:          record.Origin.Ref,
		CreatedAt:          store.FormatTimestamp(record.CreatedAt),
		UpdatedAt:          store.FormatTimestamp(record.UpdatedAt),
		ClosedAt:           nullableTaskTime(record.ClosedAt),
		Paused:             boolInt64(record.Paused),
		PausedBy:           record.PausedBy,
		PausedAt:           nullableTaskTime(record.PausedAt),
		PausedReason:       record.PausedReason,
		WakeCreator:        boolInt64(record.WakeCreator),
		MetadataJson:       nullableTaskRawJSON(record.Metadata),
	}
}

func updateTaskParams(record taskpkg.Task) sqlcgen.UpdateTaskParams {
	params := sqlcgen.UpdateTaskParams{
		Identifier:           nullableTaskString(record.Identifier),
		Scope:                string(record.Scope),
		WorkspaceID:          nullableTaskString(record.WorkspaceID),
		ParentTaskID:         nullableTaskString(record.ParentTaskID),
		Title:                record.Title,
		Description:          nullableTaskString(record.Description),
		Priority:             string(record.Priority),
		MaxAttempts:          int64(record.MaxAttempts),
		AutoEnqueueOnReady:   boolInt64(record.AutoEnqueueOnReady),
		ApprovalPolicy:       string(record.ApprovalPolicy),
		ApprovalState:        string(record.ApprovalState),
		OwnerKind:            nullableTaskOwnerKind(record.Owner),
		OwnerRef:             nullableTaskOwnerRef(record.Owner),
		CreatedByKind:        string(record.CreatedBy.Kind),
		CreatedByRef:         record.CreatedBy.Ref,
		OriginKind:           string(record.Origin.Kind),
		OriginRef:            record.Origin.Ref,
		CreatedAt:            store.FormatTimestamp(record.CreatedAt),
		UpdatedAt:            store.FormatTimestamp(record.UpdatedAt),
		ClosedAt:             nullableTaskTime(record.ClosedAt),
		Paused:               boolInt64(record.Paused),
		PausedBy:             record.PausedBy,
		PausedAt:             nullableTaskTime(record.PausedAt),
		PausedReason:         record.PausedReason,
		NeedsAttentionReason: nullableTaskString(taskNeedsAttentionReason(record.NeedsAttention)),
		NeedsAttentionAt:     nullableTaskTime(taskNeedsAttentionAt(record.NeedsAttention)),
		NeedsAttentionByKind: nullableTaskActorKind(taskNeedsAttentionActor(record.NeedsAttention)),
		NeedsAttentionByRef:  nullableTaskActorRef(taskNeedsAttentionActor(record.NeedsAttention)),
		WakeCreator:          boolInt64(record.WakeCreator),
		MetadataJson:         nullableTaskRawJSON(record.Metadata),
		ID:                   record.ID,
	}
	return params
}

func taskRunParams(run taskpkg.Run) (sqlcgen.InsertTaskRunParams, error) {
	lineage := taskRunReviewLineage(run)
	network, err := encodeParticipationSnapshot(run.WorkspaceID, run.NetworkSpecSnapshot())
	if err != nil {
		return sqlcgen.InsertTaskRunParams{}, err
	}
	wakeID, targetSessionID, ownerKey := run.NetworkWakeCorrelation()
	return sqlcgen.InsertTaskRunParams{
		ID:                     run.ID,
		TaskID:                 nullableTaskString(run.TaskID),
		WorkspaceID:            nullableTaskString(run.WorkspaceID),
		RunKind:                run.RunKind.String(),
		LoopRunID:              nullableTaskString(run.LoopRunID),
		Status:                 run.Status.String(),
		Attempt:                int64(run.Attempt),
		RecoveryCount:          int64(run.RecoveryCount),
		PreviousRunID:          nullableTaskString(run.PreviousRunID),
		FailureKind:            strings.TrimSpace(run.FailureKind),
		ClaimedByKind:          nullableTaskActorKind(run.ClaimedBy),
		ClaimedByRef:           nullableTaskActorRef(run.ClaimedBy),
		SessionID:              nullableTaskString(run.SessionID),
		OriginKind:             string(run.Origin.Kind),
		OriginRef:              run.Origin.Ref,
		IdempotencyKey:         nullableTaskString(run.IdempotencyKey),
		NetworkSpecJson:        network.JSON,
		NetworkMode:            network.Mode,
		NetworkChannel:         network.Channel,
		NetworkSource:          network.Source,
		DesignationGroupID:     strings.TrimSpace(run.DesignationGroupID),
		ClaimTokenHash:         nullableTaskString(run.ClaimTokenHash),
		LeaseUntil:             nullableTaskTime(run.LeaseUntil),
		HeartbeatAt:            nullableTaskTime(run.HeartbeatAt),
		QueuedAt:               store.FormatTimestamp(run.QueuedAt),
		ClaimedAt:              nullableTaskTime(run.ClaimedAt),
		StartedAt:              nullableTaskTime(run.StartedAt),
		EndedAt:                nullableTaskTime(run.EndedAt),
		TokensUsed:             run.TokensUsed,
		Error:                  nullableTaskString(run.Error),
		MetadataJson:           nullableTaskRawJSON(run.Metadata),
		ResultJson:             nullableTaskRawJSON(run.Result),
		ReviewRequired:         lineage.Required,
		ReviewRequestRound:     int64(lineage.RequestRound),
		ReviewPolicySnapshot:   string(lineage.PolicySnapshot),
		ReviewRequestID:        nullableTaskString(lineage.RequestID),
		ParentRunID:            nullableTaskString(lineage.ParentRunID),
		ReviewID:               nullableTaskString(lineage.ReviewID),
		ReviewRound:            int64(lineage.ReviewRound),
		ContinuationReason:     lineage.ContinuationReason,
		MissingWorkJson:        string(lineage.MissingWork),
		NextRoundGuidance:      lineage.NextRoundGuidance,
		NetworkWakeID:          nullableTaskString(wakeID),
		NetworkTargetSessionID: nullableTaskString(targetSessionID),
		NetworkOwnerKey:        nullableTaskString(ownerKey),
	}, nil
}

func updateTaskRunParams(run taskpkg.Run) (sqlcgen.UpdateTaskRunParams, error) {
	insert, err := taskRunParams(run)
	if err != nil {
		return sqlcgen.UpdateTaskRunParams{}, err
	}
	return sqlcgen.UpdateTaskRunParams{
		TaskID:                 insert.TaskID,
		WorkspaceID:            insert.WorkspaceID,
		RunKind:                insert.RunKind,
		LoopRunID:              insert.LoopRunID,
		Status:                 insert.Status,
		Attempt:                insert.Attempt,
		RecoveryCount:          insert.RecoveryCount,
		PreviousRunID:          insert.PreviousRunID,
		FailureKind:            insert.FailureKind,
		ClaimedByKind:          insert.ClaimedByKind,
		ClaimedByRef:           insert.ClaimedByRef,
		SessionID:              insert.SessionID,
		OriginKind:             insert.OriginKind,
		OriginRef:              insert.OriginRef,
		IdempotencyKey:         insert.IdempotencyKey,
		NetworkSpecJson:        insert.NetworkSpecJson,
		NetworkMode:            insert.NetworkMode,
		NetworkChannel:         insert.NetworkChannel,
		NetworkSource:          insert.NetworkSource,
		DesignationGroupID:     insert.DesignationGroupID,
		ClaimTokenHash:         insert.ClaimTokenHash,
		LeaseUntil:             insert.LeaseUntil,
		HeartbeatAt:            insert.HeartbeatAt,
		QueuedAt:               insert.QueuedAt,
		ClaimedAt:              insert.ClaimedAt,
		StartedAt:              insert.StartedAt,
		EndedAt:                insert.EndedAt,
		TokensUsed:             insert.TokensUsed,
		Error:                  insert.Error,
		MetadataJson:           insert.MetadataJson,
		ResultJson:             insert.ResultJson,
		ReviewRequired:         insert.ReviewRequired,
		ReviewRequestRound:     insert.ReviewRequestRound,
		ReviewPolicySnapshot:   insert.ReviewPolicySnapshot,
		ReviewRequestID:        insert.ReviewRequestID,
		ParentRunID:            insert.ParentRunID,
		ReviewID:               insert.ReviewID,
		ReviewRound:            insert.ReviewRound,
		ContinuationReason:     insert.ContinuationReason,
		MissingWorkJson:        insert.MissingWorkJson,
		NextRoundGuidance:      insert.NextRoundGuidance,
		NetworkWakeID:          insert.NetworkWakeID,
		NetworkTargetSessionID: insert.NetworkTargetSessionID,
		NetworkOwnerKey:        insert.NetworkOwnerKey,
		ID:                     run.ID,
	}, nil
}

func nullableTaskString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	return sql.NullString{String: value, Valid: value != ""}
}

func nullableTaskTime(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(value), Valid: true}
}

func nullableTaskRawJSON(value json.RawMessage) sql.NullString {
	if len(value) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(value), Valid: true}
}

func nullableTaskOwnerKind(owner *taskpkg.Ownership) sql.NullString {
	if owner == nil {
		return sql.NullString{}
	}
	return nullableTaskString(string(owner.Kind))
}

func nullableTaskOwnerRef(owner *taskpkg.Ownership) sql.NullString {
	if owner == nil {
		return sql.NullString{}
	}
	return nullableTaskString(owner.Ref)
}

func nullableTaskActorKind(actor *taskpkg.ActorIdentity) sql.NullString {
	if actor == nil {
		return sql.NullString{}
	}
	return nullableTaskString(string(actor.Kind))
}

func nullableTaskActorRef(actor *taskpkg.ActorIdentity) sql.NullString {
	if actor == nil {
		return sql.NullString{}
	}
	return nullableTaskString(actor.Ref)
}

func boolInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
