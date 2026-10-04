package extensionpkg

import (
	"strings"
	"time"

	apicontract "github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func taskSummaryPayloadFromSummary(record *taskpkg.Summary) apicontract.TaskSummaryPayload {
	if record == nil {
		return apicontract.TaskSummaryPayload{}
	}

	return apicontract.TaskSummaryPayload{
		ID:                   record.ID,
		ProfileID:            record.ProfileID,
		Identifier:           record.Identifier,
		Scope:                record.Scope,
		WorkspaceID:          record.WorkspaceID,
		ParentTaskID:         record.ParentTaskID,
		Title:                taskpkg.RedactClaimTokens(strings.TrimSpace(record.Title)),
		Priority:             record.Priority,
		MaxAttempts:          record.MaxAttempts,
		AutoEnqueueOnReady:   record.AutoEnqueueOnReady,
		Status:               record.Status,
		ApprovalPolicy:       record.ApprovalPolicy,
		ApprovalState:        record.ApprovalState,
		Draft:                record.Draft,
		Owner:                cloneOwnership(record.Owner),
		CurrentRunID:         record.CurrentRunID,
		LatestEventSeq:       record.LatestEventSeq,
		Paused:               record.Paused,
		PausedBy:             record.PausedBy,
		PausedAt:             optionalTime(record.PausedAt),
		PausedReason:         taskpkg.RedactClaimTokens(strings.TrimSpace(record.PausedReason)),
		EffectivePaused:      record.EffectivePaused,
		PausedByTaskID:       record.PausedByTaskID,
		BlockedReasons:       blockedReasonsPayload(record.BlockedReasons),
		NeedsAttention:       recordNeedsAttention(record.NeedsAttention, record.Status),
		NeedsAttentionReason: needsAttentionReason(record.NeedsAttention),
		NeedsAttentionAt:     needsAttentionAt(record.NeedsAttention),
		NeedsAttentionBy:     needsAttentionBy(record.NeedsAttention),
		WakeCreator:          record.WakeCreator,
		CreatedBy:            record.CreatedBy,
		Origin:               record.Origin,
		CreatedAt:            record.CreatedAt,
		UpdatedAt:            record.UpdatedAt,
		ClosedAt:             optionalTime(record.ClosedAt),
		ChildCount:           int(record.ChildCount),
		DependencyCount:      int(record.DependencyCount),
		Dependencies:         taskDependencyReferencePayloadsFromReferences(record.Dependencies),
		ActiveRun:            taskRunSummaryPayloadFromSummary(record.ActiveRun),
		LastActivityAt:       optionalTime(record.LastActivityAt),
	}
}

func taskPayloadFromTask(record *taskpkg.Task) apicontract.TaskPayload {
	if record == nil {
		return apicontract.TaskPayload{}
	}

	return apicontract.TaskPayload{
		ID:                 record.ID,
		ProfileID:          record.ProfileID,
		Identifier:         record.Identifier,
		Scope:              record.Scope,
		WorkspaceID:        record.WorkspaceID,
		ParentTaskID:       record.ParentTaskID,
		Title:              taskpkg.RedactClaimTokens(strings.TrimSpace(record.Title)),
		Description:        taskpkg.RedactClaimTokens(strings.TrimSpace(record.Description)),
		Priority:           record.Priority,
		MaxAttempts:        record.MaxAttempts,
		AutoEnqueueOnReady: record.AutoEnqueueOnReady,
		Status:             record.Status,
		ApprovalPolicy:     record.ApprovalPolicy,
		ApprovalState:      record.ApprovalState,
		Draft:              record.Status.Normalize() == taskpkg.TaskStatusDraft,
		Owner:              cloneOwnership(record.Owner),
		CurrentRunID:       record.CurrentRunID,
		LatestEventSeq:     record.LatestEventSeq,
		Paused:             record.Paused,
		PausedBy:           record.PausedBy,
		PausedAt:           optionalTime(record.PausedAt),
		PausedReason:       taskpkg.RedactClaimTokens(strings.TrimSpace(record.PausedReason)),
		EffectivePaused:    record.Paused,
		PausedByTaskID: func() string {
			if record.Paused {
				return record.ID
			}
			return ""
		}(),
		NeedsAttention:       recordNeedsAttention(record.NeedsAttention, record.Status),
		NeedsAttentionReason: needsAttentionReason(record.NeedsAttention),
		NeedsAttentionAt:     needsAttentionAt(record.NeedsAttention),
		NeedsAttentionBy:     needsAttentionBy(record.NeedsAttention),
		WakeCreator:          record.WakeCreator,
		CreatedBy:            record.CreatedBy,
		Origin:               record.Origin,
		CreatedAt:            record.CreatedAt,
		UpdatedAt:            record.UpdatedAt,
		ClosedAt:             optionalTime(record.ClosedAt),
		Metadata:             taskpkg.RedactClaimTokenJSON(record.Metadata),
	}
}

func blockedReasonsPayload(reasons *[]taskpkg.BlockedReason) []taskpkg.BlockedReason {
	if reasons == nil || len(*reasons) == 0 {
		return nil
	}
	cloned := make([]taskpkg.BlockedReason, len(*reasons))
	for idx, reason := range *reasons {
		cloned[idx] = reason
		cloned[idx].Reason = taskpkg.RedactClaimTokens(strings.TrimSpace(reason.Reason))
	}
	return cloned
}

func recordNeedsAttention(attention *taskpkg.NeedsAttention, status taskpkg.Status) bool {
	return attention != nil || status.Normalize() == taskpkg.TaskStatusNeedsAttention
}

func needsAttentionReason(attention *taskpkg.NeedsAttention) string {
	if attention == nil {
		return ""
	}
	return taskpkg.RedactClaimTokens(strings.TrimSpace(attention.Reason))
}

func needsAttentionAt(attention *taskpkg.NeedsAttention) *time.Time {
	if attention == nil {
		return nil
	}
	return optionalTime(attention.At)
}

func needsAttentionBy(attention *taskpkg.NeedsAttention) *taskpkg.ActorIdentity {
	if attention == nil || attention.By.IsZero() {
		return nil
	}
	actor := attention.By
	return &actor
}

func taskRunPayloadFromRun(run *taskpkg.Run) apicontract.TaskRunPayload {
	if run == nil {
		return apicontract.TaskRunPayload{}
	}

	return apicontract.TaskRunPayload{
		ID:        run.ID,
		ProfileID: run.ProfileID,
		TaskID:    run.TaskID,
		Status:    run.Status,
		// Run attempts are int32, which converts exactly to int on every Go architecture.
		Attempt:        int(run.Attempt),
		ClaimedBy:      cloneActorIdentity(run.ClaimedBy),
		SessionID:      run.SessionID,
		Origin:         run.Origin,
		IdempotencyKey: run.IdempotencyKey,
		QueuedAt:       run.QueuedAt,
		ClaimedAt:      optionalTime(run.ClaimedAt),
		StartedAt:      optionalTime(run.StartedAt),
		EndedAt:        optionalTime(run.EndedAt),
		Error:          taskpkg.RedactClaimTokens(run.Error),
		Metadata:       taskpkg.RedactClaimTokenJSON(run.Metadata),
		Result:         taskpkg.RedactClaimTokenJSON(run.ResultValue()),
	}
}
