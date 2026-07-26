package core

import (
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskSummaryPayloadsFromSummaries converts task summaries into shared payloads.
func TaskSummaryPayloadsFromSummaries(tasks []taskpkg.Summary) []contract.TaskSummaryPayload {
	payloads := make([]contract.TaskSummaryPayload, 0, len(tasks))
	for idx := range tasks {
		payloads = append(payloads, TaskSummaryPayloadFromSummary(&tasks[idx]))
	}
	return payloads
}

// TaskSummaryPayloadFromSummary converts one task summary into the shared payload.
func TaskSummaryPayloadFromSummary(record *taskpkg.Summary) contract.TaskSummaryPayload {
	if record == nil {
		return contract.TaskSummaryPayload{}
	}

	return contract.TaskSummaryPayload{
		ID:                           record.ID,
		Identifier:                   record.Identifier,
		Scope:                        record.Scope,
		WorkspaceID:                  record.WorkspaceID,
		ParentTaskID:                 record.ParentTaskID,
		ResolvedNetworkParticipation: resolvedParticipationFromRunSummary(record.ActiveRun),
		Title:                        taskpkg.RedactClaimTokens(strings.TrimSpace(record.Title)),
		Priority:                     record.Priority,
		MaxAttempts:                  record.MaxAttempts,
		AutoEnqueueOnReady:           record.AutoEnqueueOnReady,
		Status:                       record.Status,
		ApprovalPolicy:               record.ApprovalPolicy,
		ApprovalState:                record.ApprovalState,
		Draft:                        record.Draft,
		Owner:                        cloneOwnership(record.Owner),
		CurrentRunID:                 record.CurrentRunID,
		LatestEventSeq:               record.LatestEventSeq,
		Paused:                       record.Paused,
		PausedBy:                     record.PausedBy,
		PausedAt:                     optionalTime(record.PausedAt),
		PausedReason:                 taskpkg.RedactClaimTokens(strings.TrimSpace(record.PausedReason)),
		EffectivePaused:              record.EffectivePaused,
		PausedByTaskID:               record.PausedByTaskID,
		BlockedReasons:               blockedReasonsPayload(record.BlockedReasons),
		NeedsAttention:               recordNeedsAttention(record.NeedsAttention, record.Status),
		NeedsAttentionReason:         needsAttentionReason(record.NeedsAttention),
		NeedsAttentionAt:             needsAttentionAt(record.NeedsAttention),
		NeedsAttentionBy:             needsAttentionBy(record.NeedsAttention),
		WakeCreator:                  record.WakeCreator,
		CreatedBy:                    record.CreatedBy,
		Origin:                       record.Origin,
		CreatedAt:                    record.CreatedAt,
		UpdatedAt:                    record.UpdatedAt,
		ClosedAt:                     optionalTime(record.ClosedAt),
		ChildCount:                   int(record.ChildCount),
		DependencyCount:              int(record.DependencyCount),
		Dependencies:                 TaskDependencyReferencePayloadsFromReferences(record.Dependencies),
		ActiveRun:                    TaskRunSummaryPayloadFromSummary(record.ActiveRun),
		LastActivityAt:               optionalTime(record.LastActivityAt),
	}
}

// TaskPayloadFromTask converts one task record into the shared payload.
func TaskPayloadFromTask(record *taskpkg.Task) contract.TaskPayload {
	if record == nil {
		return contract.TaskPayload{}
	}

	return contract.TaskPayload{
		ID:                 record.ID,
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

// TaskBlockPayloadsFromBlocks converts task-block records into shared payloads.
func TaskBlockPayloadsFromBlocks(blocks []taskpkg.TaskBlock) []contract.TaskBlockPayload {
	if len(blocks) == 0 {
		return nil
	}
	payloads := make([]contract.TaskBlockPayload, 0, len(blocks))
	for _, block := range blocks {
		payloads = append(payloads, TaskBlockPayloadFromBlock(block))
	}
	return payloads
}

// TaskBlockPayloadFromBlock converts one task-block record into the shared payload.
func TaskBlockPayloadFromBlock(block taskpkg.TaskBlock) contract.TaskBlockPayload {
	payload := contract.TaskBlockPayload{
		ID:          strings.TrimSpace(block.ID),
		TaskID:      strings.TrimSpace(block.TaskID),
		WorkspaceID: strings.TrimSpace(block.WorkspaceID),
		Kind:        block.Kind.Normalize(),
		Reason:      taskpkg.RedactClaimTokens(strings.TrimSpace(block.Reason)),
		Details:     taskpkg.RedactClaimTokenJSON(block.Details),
		CreatedAt:   block.CreatedAt,
		CreatedBy:   block.CreatedBy,
		ExpiresAt:   optionalTime(block.ExpiresAt),
		ClearedAt:   optionalTime(block.ClearedAt),
		ClearNote:   taskpkg.RedactClaimTokens(strings.TrimSpace(block.ClearNote)),
	}
	if !block.ClearedBy.IsZero() {
		clearedBy := block.ClearedBy
		payload.ClearedBy = &clearedBy
	}
	return payload
}

// TaskDependencyPayloadsFromDependencies converts dependency records into shared payloads.
func TaskDependencyPayloadsFromDependencies(dependencies []taskpkg.Dependency) []contract.TaskDependencyPayload {
	payloads := make([]contract.TaskDependencyPayload, 0, len(dependencies))
	for _, dependency := range dependencies {
		payloads = append(payloads, contract.TaskDependencyPayload{
			TaskID:          dependency.TaskID,
			DependsOnTaskID: dependency.DependsOnTaskID,
			Kind:            dependency.Kind,
			CreatedAt:       dependency.CreatedAt,
		})
	}
	return payloads
}

// TaskRunPayloadsFromRuns converts task runs into shared payloads.
func TaskRunPayloadsFromRuns(runs []taskpkg.Run) []contract.TaskRunPayload {
	payloads := make([]contract.TaskRunPayload, 0, len(runs))
	for _, run := range runs {
		payloads = append(payloads, TaskRunPayloadFromRun(&run))
	}
	return payloads
}

// TaskExecutionResponseFromExecution converts one task execution-boundary result.
func TaskExecutionResponseFromExecution(execution *taskpkg.Execution) contract.TaskExecutionResponse {
	if execution == nil {
		return contract.TaskExecutionResponse{}
	}
	return contract.TaskExecutionResponse{
		Task: TaskPayloadFromTask(&execution.Task),
		Run:  TaskRunPayloadFromRun(&execution.Run),
	}
}

// RetryTaskRunResponseFromResult converts one retry result into the shared payload.
func RetryTaskRunResponseFromResult(result *taskpkg.RetryRunResult) contract.RetryTaskRunResponse {
	if result == nil {
		return contract.RetryTaskRunResponse{}
	}
	return contract.RetryTaskRunResponse{
		PreviousRun: TaskRunPayloadFromRun(&result.PreviousRun),
		Run:         TaskRunPayloadFromRun(&result.Run),
	}
}

// BulkForceTaskRunResponseFromResult converts per-row bulk force outcomes into shared payloads.
func BulkForceTaskRunResponseFromResult(
	result taskpkg.BulkForceRunResult,
	maskInternalErrors bool,
) contract.BulkForceTaskRunResponse {
	items := make([]contract.BulkForceTaskRunItemPayload, 0, len(result.Items))
	for _, item := range result.Items {
		payload := contract.BulkForceTaskRunItemPayload{
			RunID: item.RunID,
			OK:    item.OK,
			Run:   optionalTaskRunPayload(item.Run),
		}
		if item.Err != nil {
			errorPayload := ErrorPayloadForStatus(StatusForTaskError(item.Err), item.Err, maskInternalErrors)
			payload.Error = &errorPayload
		}
		items = append(items, payload)
	}
	return contract.BulkForceTaskRunResponse{Results: items}
}

func optionalTaskRunPayload(run *taskpkg.Run) *contract.TaskRunPayload {
	if run == nil {
		return nil
	}
	payload := TaskRunPayloadFromRun(run)
	return &payload
}

// TaskEventPayloadsFromEvents converts task events into shared payloads.
func TaskEventPayloadsFromEvents(events []taskpkg.Event) []contract.TaskEventPayload {
	payloads := make([]contract.TaskEventPayload, 0, len(events))
	for _, event := range events {
		payloads = append(payloads, contract.TaskEventPayload{
			ID:        event.ID,
			TaskID:    event.TaskID,
			RunID:     event.RunID,
			EventType: event.EventType,
			Actor:     event.Actor,
			Origin:    event.Origin,
			Payload:   taskpkg.RedactClaimTokenJSON(event.Payload),
			Timestamp: event.Timestamp,
		})
	}
	return payloads
}

// TaskDetailPayloadFromView converts one expanded task view into the shared payload.
func TaskDetailPayloadFromView(view *taskpkg.View) contract.TaskDetailPayload {
	if view == nil {
		return contract.TaskDetailPayload{}
	}

	summary := TaskSummaryPayloadFromSummary(&view.Summary)
	taskRecord := TaskPayloadFromTask(&view.Task)
	taskRecord.EffectivePaused = summary.EffectivePaused
	taskRecord.PausedByTaskID = summary.PausedByTaskID
	taskRecord.BlockedReasons = summary.BlockedReasons

	return contract.TaskDetailPayload{
		Summary:              summary,
		Task:                 taskRecord,
		Children:             TaskSummaryPayloadsFromSummaries(view.Children),
		Dependencies:         TaskDependencyPayloadsFromDependencies(view.Dependencies),
		DependencyReferences: TaskDependencyReferencePayloadsFromReferences(view.DependencyReferences),
		Runs:                 TaskRunPayloadsFromRuns(view.Runs),
		Events:               TaskEventPayloadsFromEvents(view.Events),
	}
}
