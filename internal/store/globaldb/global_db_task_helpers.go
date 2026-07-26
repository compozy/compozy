package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
)

func normalizeTaskRecord(record taskpkg.Task) taskpkg.Task {
	normalized := record
	normalized.ID = strings.TrimSpace(normalized.ID)
	normalized.Identifier = strings.TrimSpace(normalized.Identifier)
	normalized.Scope = normalized.Scope.Normalize()
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.ParentTaskID = strings.TrimSpace(normalized.ParentTaskID)
	normalized.CurrentRunID = strings.TrimSpace(normalized.CurrentRunID)
	normalized.Title = strings.TrimSpace(normalized.Title)
	normalized.Description = strings.TrimSpace(normalized.Description)
	normalized.PausedBy = strings.TrimSpace(normalized.PausedBy)
	normalized.PausedReason = strings.TrimSpace(normalized.PausedReason)
	if normalized.NeedsAttention != nil {
		attention := *normalized.NeedsAttention
		attention.Reason = strings.TrimSpace(attention.Reason)
		attention.By.Kind = attention.By.Kind.Normalize()
		attention.By.Ref = strings.TrimSpace(attention.By.Ref)
		if !attention.At.IsZero() {
			attention.At = attention.At.UTC()
		}
		if attention.Reason == "" && attention.At.IsZero() && attention.By.IsZero() {
			normalized.NeedsAttention = nil
		} else {
			normalized.NeedsAttention = &attention
		}
	}
	normalized.Status = normalized.Status.Normalize()
	normalized.CreatedBy.Kind = normalized.CreatedBy.Kind.Normalize()
	normalized.CreatedBy.Ref = strings.TrimSpace(normalized.CreatedBy.Ref)
	normalized.Origin.Kind = normalized.Origin.Kind.Normalize()
	normalized.Origin.Ref = strings.TrimSpace(normalized.Origin.Ref)
	if normalized.Owner != nil {
		owner := *normalized.Owner
		owner.Kind = owner.Kind.Normalize()
		owner.Ref = strings.TrimSpace(owner.Ref)
		if owner.IsZero() {
			normalized.Owner = nil
		} else {
			normalized.Owner = &owner
		}
	}
	normalized.Metadata = normalizeTaskJSON(normalized.Metadata)
	normalized.Priority = taskpkg.DefaultPriority
	if record.Priority.Normalize() != "" {
		normalized.Priority = record.Priority.Normalize()
	}
	normalized.MaxAttempts = taskpkg.DefaultTaskMaxAttempts
	if record.MaxAttempts != 0 {
		normalized.MaxAttempts = record.MaxAttempts
	}
	normalized.ApprovalPolicy = taskpkg.DefaultApprovalPolicy
	if record.ApprovalPolicy.Normalize() != "" {
		normalized.ApprovalPolicy = record.ApprovalPolicy.Normalize()
	}
	normalized.ApprovalState = taskpkg.ApprovalStateNotRequired
	if record.ApprovalState.Normalize() != "" {
		normalized.ApprovalState = record.ApprovalState.Normalize()
	} else if normalized.ApprovalPolicy == taskpkg.ApprovalPolicyManual {
		normalized.ApprovalState = taskpkg.ApprovalStatePending
	}
	if !normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = normalized.CreatedAt.UTC()
	}
	if !normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.UpdatedAt.UTC()
	}
	if !normalized.ClosedAt.IsZero() {
		normalized.ClosedAt = normalized.ClosedAt.UTC()
	}
	if !normalized.PausedAt.IsZero() {
		normalized.PausedAt = normalized.PausedAt.UTC()
	}
	return normalized
}

func normalizeTaskRunRecord(run taskpkg.Run) taskpkg.Run {
	normalized := run
	normalized.ID = strings.TrimSpace(normalized.ID)
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.RunKind = normalized.RunKind.Normalize()
	if normalized.RunKind == taskpkg.RunKindUnknown {
		normalized.RunKind = taskpkg.RunKindWorker
	}
	normalized.LoopRunID = strings.TrimSpace(normalized.LoopRunID)
	normalized.Status = normalized.Status.Normalize()
	normalized.PreviousRunID = strings.TrimSpace(normalized.PreviousRunID)
	normalized.FailureKind = strings.TrimSpace(normalized.FailureKind)
	if normalized.ClaimedBy != nil {
		claimedBy := *normalized.ClaimedBy
		claimedBy.Kind = claimedBy.Kind.Normalize()
		claimedBy.Ref = strings.TrimSpace(claimedBy.Ref)
		normalized.ClaimedBy = &claimedBy
	}
	normalized.SessionID = strings.TrimSpace(normalized.SessionID)
	normalized.Origin.Kind = normalized.Origin.Kind.Normalize()
	normalized.Origin.Ref = strings.TrimSpace(normalized.Origin.Ref)
	normalized.IdempotencyKey = strings.TrimSpace(normalized.IdempotencyKey)
	wakeID, targetSessionID, ownerKey := normalized.NetworkWakeCorrelation()
	normalized.SetNetworkState(
		normalized.NetworkSpecSnapshot(),
		wakeID,
		targetSessionID,
		ownerKey,
	)
	normalized.DesignationGroupID = strings.TrimSpace(normalized.DesignationGroupID)
	normalized.ClaimTokenHash = strings.TrimSpace(normalized.ClaimTokenHash)
	normalized.RequiredCapabilities = normalizeTaskRunCapabilityIDs(normalized.RequiredCapabilities)
	normalized.PreferredCapabilities = normalizeTaskRunCapabilityIDs(
		normalized.PreferredCapabilities,
	)
	normalized.Review = normalizeTaskRunReviewLineage(normalized.Review)
	normalized.Error = strings.TrimSpace(normalized.Error)
	normalized.Metadata = normalizeTaskJSON(normalized.Metadata)
	normalized.Result = normalizeTaskJSON(normalized.Result)
	if !normalized.QueuedAt.IsZero() {
		normalized.QueuedAt = normalized.QueuedAt.UTC()
	}
	if !normalized.ClaimedAt.IsZero() {
		normalized.ClaimedAt = normalized.ClaimedAt.UTC()
	}
	if !normalized.StartedAt.IsZero() {
		normalized.StartedAt = normalized.StartedAt.UTC()
	}
	if !normalized.EndedAt.IsZero() {
		normalized.EndedAt = normalized.EndedAt.UTC()
	}
	if !normalized.LeaseUntil.IsZero() {
		normalized.LeaseUntil = normalized.LeaseUntil.UTC()
	}
	if !normalized.HeartbeatAt.IsZero() {
		normalized.HeartbeatAt = normalized.HeartbeatAt.UTC()
	}
	return normalized
}

func taskRunReviewLineage(run taskpkg.Run) taskpkg.RunReviewLineage {
	lineage := normalizeTaskRunReviewLineage(run.Review)
	if lineage == nil {
		return taskpkg.RunReviewLineage{MissingWork: json.RawMessage("[]")}
	}
	return *lineage
}

func taskRunReviewLineageFromScan(fields *taskRunScanFields) *taskpkg.RunReviewLineage {
	lineage := &taskpkg.RunReviewLineage{
		Required:           fields.reviewRequired,
		RequestRound:       fields.reviewRequestRound,
		PolicySnapshot:     taskpkg.ReviewPolicy(strings.TrimSpace(fields.reviewPolicySnapshot)),
		RequestID:          taskNullStringValue(fields.reviewRequestID),
		ParentRunID:        taskNullStringValue(fields.parentRunID),
		ReviewID:           taskNullStringValue(fields.reviewID),
		ReviewRound:        fields.reviewRound,
		ContinuationReason: strings.TrimSpace(fields.continuationReason),
		MissingWork:        []byte(strings.TrimSpace(fields.missingWorkJSON)),
		NextRoundGuidance:  strings.TrimSpace(fields.nextRoundGuidance),
	}
	return normalizeTaskRunReviewLineage(lineage)
}

func normalizeTaskRunReviewLineage(lineage *taskpkg.RunReviewLineage) *taskpkg.RunReviewLineage {
	if lineage == nil {
		return nil
	}
	normalized := *lineage
	normalized.PolicySnapshot = normalized.PolicySnapshot.Normalize()
	normalized.RequestID = strings.TrimSpace(normalized.RequestID)
	normalized.ParentRunID = strings.TrimSpace(normalized.ParentRunID)
	normalized.ReviewID = strings.TrimSpace(normalized.ReviewID)
	normalized.ContinuationReason = strings.TrimSpace(normalized.ContinuationReason)
	normalized.MissingWork = normalizeTaskRunMissingWork(normalized.MissingWork)
	normalized.NextRoundGuidance = strings.TrimSpace(normalized.NextRoundGuidance)
	if isEmptyTaskRunReviewLineage(normalized) {
		return nil
	}
	return &normalized
}

func isEmptyTaskRunReviewLineage(lineage taskpkg.RunReviewLineage) bool {
	return !lineage.Required &&
		lineage.RequestRound == 0 &&
		lineage.PolicySnapshot == "" &&
		lineage.RequestID == "" &&
		lineage.ParentRunID == "" &&
		lineage.ReviewID == "" &&
		lineage.ReviewRound == 0 &&
		lineage.ContinuationReason == "" &&
		string(lineage.MissingWork) == "[]" &&
		lineage.NextRoundGuidance == ""
}

func normalizeTaskRunMissingWork(raw json.RawMessage) json.RawMessage {
	normalized := normalizeTaskJSON(raw)
	if len(normalized) == 0 {
		return json.RawMessage("[]")
	}
	return normalized
}

func normalizeTaskRunCapabilityIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			normalized = append(normalized, trimmed)
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	return normalized
}

func normalizeTaskQuery(query taskpkg.Query) taskpkg.Query {
	normalized := query
	normalized.Scope = normalized.Scope.Normalize()
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.Status = normalized.Status.Normalize()
	normalized.Priority = normalized.Priority.Normalize()
	normalized.ApprovalState = normalized.ApprovalState.Normalize()
	normalized.OwnerKind = normalized.OwnerKind.Normalize()
	normalized.OwnerRef = strings.TrimSpace(normalized.OwnerRef)
	normalized.ParentTaskID = strings.TrimSpace(normalized.ParentTaskID)
	normalized.CreatedByKind = normalized.CreatedByKind.Normalize()
	normalized.CreatedByRef = strings.TrimSpace(normalized.CreatedByRef)
	normalized.Search = strings.TrimSpace(normalized.Search)
	return normalized
}

func normalizeTaskRunQuery(query taskpkg.RunQuery) taskpkg.RunQuery {
	normalized := query
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.Status = normalized.Status.Normalize()
	normalized.SessionID = strings.TrimSpace(normalized.SessionID)
	normalized.DesignationGroupID = strings.TrimSpace(normalized.DesignationGroupID)
	normalized.ParticipationChannel = strings.TrimSpace(normalized.ParticipationChannel)
	return normalized
}

func taskSummaryFromRecord(record taskpkg.Task) taskpkg.Summary {
	return taskpkg.Summary{
		ID:                 record.ID,
		Identifier:         record.Identifier,
		Scope:              record.Scope,
		WorkspaceID:        record.WorkspaceID,
		ParentTaskID:       record.ParentTaskID,
		Title:              record.Title,
		Priority:           record.Priority,
		MaxAttempts:        record.MaxAttempts,
		AutoEnqueueOnReady: record.AutoEnqueueOnReady,
		Status:             record.Status,
		ApprovalPolicy:     record.ApprovalPolicy,
		ApprovalState:      record.ApprovalState,
		Draft:              record.Status.Normalize() == taskpkg.TaskStatusDraft,
		NeedsAttention:     cloneNeedsAttention(record.NeedsAttention),
		Owner:              record.Owner,
		CurrentRunID:       record.CurrentRunID,
		LatestEventSeq:     record.LatestEventSeq,
		Paused:             record.Paused,
		WakeCreator:        record.WakeCreator,
		PausedBy:           record.PausedBy,
		PausedAt:           record.PausedAt,
		PausedReason:       record.PausedReason,
		EffectivePaused:    record.Paused,
		PausedByTaskID: func() string {
			if record.Paused {
				return record.ID
			}
			return ""
		}(),
		CreatedBy:      record.CreatedBy,
		Origin:         record.Origin,
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
		ClosedAt:       record.ClosedAt,
		LastActivityAt: record.UpdatedAt,
	}
}

func cloneNeedsAttention(attention *taskpkg.NeedsAttention) *taskpkg.NeedsAttention {
	if attention == nil {
		return nil
	}
	clone := *attention
	return &clone
}

func requireTaskValue(value string, label string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("store: %s is required", label)
	}
	return trimmed, nil
}

func taskBoolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func taskNeedsAttentionReason(attention *taskpkg.NeedsAttention) string {
	if attention == nil {
		return ""
	}
	return attention.Reason
}

func taskNeedsAttentionAt(attention *taskpkg.NeedsAttention) time.Time {
	if attention == nil {
		return time.Time{}
	}
	return attention.At
}

func taskNeedsAttentionActor(attention *taskpkg.NeedsAttention) *taskpkg.ActorIdentity {
	if attention == nil || attention.By.IsZero() {
		return nil
	}
	return &attention.By
}

func normalizeTaskJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil
	}
	return json.RawMessage(trimmed)
}

func decodeTaskJSON(raw sql.NullString, label string) (json.RawMessage, error) {
	if !raw.Valid {
		return nil, nil
	}
	trimmed := strings.TrimSpace(raw.String)
	if trimmed == "" {
		return nil, nil
	}
	value := json.RawMessage(trimmed)
	if !json.Valid(value) {
		return nil, fmt.Errorf("store: decode %s: invalid JSON", label)
	}
	return value, nil
}

func taskNullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}
