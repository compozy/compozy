package core

import (
	"encoding/json"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskDesignationRollupPayloadsFromStore converts persisted designation rollups into shared payloads.
func TaskDesignationRollupPayloadsFromStore(
	rollups []store.TaskDesignationRollup,
) []contract.TaskDesignationRollupPayload {
	if len(rollups) == 0 {
		return nil
	}
	payloads := make([]contract.TaskDesignationRollupPayload, 0, len(rollups))
	for _, rollup := range rollups {
		payloads = append(payloads, contract.TaskDesignationRollupPayload{
			DesignationGroupID: strings.TrimSpace(rollup.DesignationGroupID),
			TaskID:             strings.TrimSpace(rollup.TaskID),
			Summary:            taskpkg.RedactClaimTokenJSON(rollup.SummaryJSON),
			CreatedAt:          rollup.CreatedAt,
		})
	}
	return payloads
}

// TaskInspectPayloadFromView converts one inspect view into the shared payload.
func TaskInspectPayloadFromView(view *taskpkg.InspectView) contract.TaskInspectPayload {
	if view == nil {
		return contract.TaskInspectPayload{}
	}

	return contract.TaskInspectPayload{
		Target:       string(view.Target),
		Task:         TaskSummaryPayloadFromSummary(&view.Task),
		CurrentRun:   TaskInspectRunPayloadFromSummary(view.CurrentRun),
		BoundSession: TaskInspectSessionPayloadFromSummary(view.BoundSession),
		RecentRuns:   TaskInspectRunPayloadsFromSummaries(view.RecentRuns),
		RecentEvents: TaskInspectEventPayloadsFromSummaries(view.RecentEvents),
		Scheduler:    TaskInspectSchedulerPayloadFromState(view.Scheduler),
		Diagnostics:  append([]contract.DiagnosticItem(nil), view.Diagnostics...),
		NextAction:   string(view.NextAction),
		AsOf:         view.AsOf,
	}
}

// TaskInspectRunPayloadFromSummary converts an inspect run summary into the shared payload.
func TaskInspectRunPayloadFromSummary(summary *taskpkg.InspectRunSummary) *contract.TaskInspectRunPayload {
	if summary == nil {
		return nil
	}
	return &contract.TaskInspectRunPayload{
		RunID:                   summary.RunID,
		TaskID:                  summary.TaskID,
		Status:                  summary.Status,
		ClaimTokenHashTruncated: summary.ClaimTokenHashTruncated,
		LeaseUntil:              optionalTime(summary.LeaseUntil),
		HeartbeatAt:             optionalTime(summary.HeartbeatAt),
		HeartbeatAgeSeconds:     cloneInt64Ptr(summary.HeartbeatAgeSeconds),
		Retries:                 summary.Retries,
		LastErrorSummary:        taskpkg.RedactClaimTokens(strings.TrimSpace(summary.LastErrorSummary)),
		FailureKind:             summary.FailureKind,
		BoundSessionID:          summary.BoundSessionID,
		StartedAt:               optionalTime(summary.StartedAt),
		EndedAt:                 optionalTime(summary.EndedAt),
		PreviousRunID:           summary.PreviousRunID,
		QueuedAt:                summary.QueuedAt,
		Attempt:                 summary.Attempt,
	}
}

// TaskInspectRunPayloadsFromSummaries converts inspect run summaries into shared payloads.
func TaskInspectRunPayloadsFromSummaries(
	summaries []taskpkg.InspectRunSummary,
) []contract.TaskInspectRunPayload {
	payloads := make([]contract.TaskInspectRunPayload, 0, len(summaries))
	for idx := range summaries {
		payload := TaskInspectRunPayloadFromSummary(&summaries[idx])
		if payload != nil {
			payloads = append(payloads, *payload)
		}
	}
	return payloads
}

// TaskInspectSessionPayloadFromSummary converts an inspect session summary into the shared payload.
func TaskInspectSessionPayloadFromSummary(
	summary *taskpkg.InspectSessionSummary,
) *contract.TaskInspectSessionPayload {
	if summary == nil {
		return nil
	}
	return &contract.TaskInspectSessionPayload{
		SessionID:      summary.SessionID,
		State:          summary.State,
		AgentName:      summary.AgentName,
		ProviderName:   summary.ProviderName,
		WorkspaceID:    summary.WorkspaceID,
		StartedAt:      optionalTime(summary.StartedAt),
		LastActivityAt: optionalTime(summary.LastActivityAt),
		StopReason:     taskpkg.RedactClaimTokens(strings.TrimSpace(summary.StopReason)),
		FailureKind:    summary.FailureKind,
	}
}

// TaskInspectEventPayloadsFromSummaries converts inspect event summaries into shared payloads.
func TaskInspectEventPayloadsFromSummaries(
	summaries []taskpkg.InspectEventSummary,
) []contract.TaskInspectEventPayload {
	payloads := make([]contract.TaskInspectEventPayload, 0, len(summaries))
	for _, summary := range summaries {
		payloads = append(payloads, contract.TaskInspectEventPayload{
			ID:        summary.ID,
			Type:      summary.Type,
			SessionID: summary.SessionID,
			TaskID:    summary.TaskID,
			RunID:     summary.RunID,
			Outcome:   summary.Outcome,
			Summary:   taskpkg.RedactClaimTokens(strings.TrimSpace(summary.Summary)),
			Timestamp: summary.Timestamp,
		})
	}
	return payloads
}

// TaskInspectSchedulerPayloadFromState converts scheduler state into the shared payload.
func TaskInspectSchedulerPayloadFromState(state taskpkg.InspectSchedulerState) contract.TaskInspectSchedulerPayload {
	return contract.TaskInspectSchedulerPayload{
		Paused:    state.Paused,
		PausedBy:  state.PausedBy,
		PausedAt:  optionalTime(state.PausedAt),
		Reason:    taskpkg.RedactClaimTokens(strings.TrimSpace(state.Reason)),
		UpdatedAt: optionalTime(state.UpdatedAt),
	}
}

func cloneOwnership(source *taskpkg.Ownership) *taskpkg.Ownership {
	if source == nil {
		return nil
	}
	return &taskpkg.Ownership{
		Kind: source.Kind.Normalize(),
		Ref:  strings.TrimSpace(source.Ref),
	}
}

func cloneActorIdentity(source *taskpkg.ActorIdentity) *taskpkg.ActorIdentity {
	if source == nil {
		return nil
	}
	return &taskpkg.ActorIdentity{
		Kind: source.Kind.Normalize(),
		Ref:  strings.TrimSpace(source.Ref),
	}
}

func trimStringPtr(source *string) *string {
	if source == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*source)
	return &trimmed
}

func normalizePriorityPtr(source *taskpkg.Priority) *taskpkg.Priority {
	if source == nil {
		return nil
	}
	normalized := source.Normalize()
	return &normalized
}

func normalizeApprovalPolicyPtr(source *taskpkg.ApprovalPolicy) *taskpkg.ApprovalPolicy {
	if source == nil {
		return nil
	}
	normalized := source.Normalize()
	return &normalized
}

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	cloned := value
	return &cloned
}

func cloneRawMessagePtr(source *json.RawMessage) *json.RawMessage {
	if source == nil {
		return nil
	}
	copyValue := cloneRawMessage(*source)
	return &copyValue
}
