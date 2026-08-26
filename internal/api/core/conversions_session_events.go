package core

import (
	"encoding/json"

	"fmt"
	"log/slog"

	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"

	"github.com/compozy/compozy/internal/diagnostics"
	eventspkg "github.com/compozy/compozy/internal/events"

	"github.com/compozy/compozy/internal/session"

	ssepkg "github.com/compozy/compozy/internal/sse"
	"github.com/compozy/compozy/internal/store"

	"github.com/compozy/compozy/internal/workref"
)

// SessionFailurePayloadFromStore converts a stored failure diagnostic into the
// shared API payload.
func SessionFailurePayloadFromStore(failure *store.SessionFailure) *contract.SessionFailurePayload {
	if failure == nil {
		return nil
	}
	normalized := failure.Normalize()
	if normalized.IsZero() {
		return nil
	}
	return &contract.SessionFailurePayload{
		Kind:            normalized.Kind,
		ReasonCode:      normalized.ReasonCode,
		Summary:         diagnostics.RedactAndBound(normalized.Summary, maxDiagnosticPayloadBytes),
		CrashBundlePath: diagnostics.RedactAndBound(normalized.CrashBundlePath, maxDiagnosticPayloadBytes),
	}
}

// SessionEventPayloadFromEvent converts a session event into the shared payload.
func SessionEventPayloadFromEvent(event store.SessionEvent, info *session.Info) contract.SessionEventPayload {
	ref := workref.NewPath(sessionWorkspaceFromInfo(info))
	payload := contract.SessionEventPayload{
		ID:               event.ID,
		SessionID:        event.SessionID,
		Sequence:         event.Sequence,
		TurnID:           event.TurnID,
		Type:             event.Type,
		AgentName:        event.AgentName,
		WorkspaceID:      ref.WorkspaceID,
		WorkspacePath:    ref.WorkspacePath,
		EventCorrelation: sessionEventCorrelation(event),
		Content:          PayloadJSON(event.Content),
		Goal:             sessionEventGoalPromptMeta(event.Content),
		Timestamp:        event.Timestamp,
	}
	if info != nil && info.Lineage != nil {
		lineage := store.NormalizeSessionLineage(event.SessionID, info.Lineage)
		payload.ParentSessionID = lineage.ParentSessionID
		payload.RootSessionID = lineage.RootSessionID
		payload.SpawnDepth = lineage.SpawnDepth
	}
	if info != nil && event.Type == session.EventTypeSessionStopped {
		payload.StopReason = info.StopReason
		payload.StopDetail = info.StopDetail
		payload.Failure = SessionFailurePayloadFromStore(info.Failure)
	}
	return payload
}

// SessionRepairPayloadFromResult converts a session repair report into the shared payload.
func SessionRepairPayloadFromResult(result *session.RepairResult) contract.SessionRepairPayload {
	if result == nil {
		return contract.SessionRepairPayload{}
	}

	issues := make([]contract.SessionRepairIssuePayload, 0, len(result.Issues))
	for _, issue := range result.Issues {
		issues = append(issues, contract.SessionRepairIssuePayload{
			Code:     issue.Code,
			Severity: issue.Severity,
			TurnID:   issue.TurnID,
			EventID:  issue.EventID,
			Detail:   issue.Detail,
		})
	}

	actions := make([]contract.SessionRepairActionPayload, 0, len(result.Actions))
	for _, action := range result.Actions {
		actions = append(actions, contract.SessionRepairActionPayload{
			Code:       action.Code,
			TurnID:     action.TurnID,
			EventID:    action.EventID,
			ToolCallID: action.ToolCallID,
			ToolName:   action.ToolName,
			Persisted:  action.Persisted,
		})
	}

	return contract.SessionRepairPayload{
		SessionID: result.SessionID,
		Issues:    issues,
		Actions:   actions,
		Persisted: result.Persisted,
	}
}

// TokenUsagePayloadFromUsage converts token usage info into the shared payload.
func TokenUsagePayloadFromUsage(usage *acp.TokenUsage) *contract.TokenUsagePayload {
	if usage == nil {
		return nil
	}

	return &contract.TokenUsagePayload{
		TurnID:           usage.TurnID,
		InputTokens:      usage.InputTokens,
		OutputTokens:     usage.OutputTokens,
		TotalTokens:      usage.TotalTokens,
		ThoughtTokens:    usage.ThoughtTokens,
		CacheReadTokens:  usage.CacheReadTokens,
		CacheWriteTokens: usage.CacheWriteTokens,
		ContextUsed:      usage.ContextUsed,
		ContextSize:      usage.ContextSize,
		CostAmount:       usage.CostAmount,
		CostCurrency:     usage.CostCurrency,
		Timestamp:        usage.Timestamp,
	}
}

// LogEventPayloadFromSummary converts an event summary into the shared logs payload.
func LogEventPayloadFromSummary(event store.EventSummary) contract.LogEventPayload {
	return contract.LogEventPayload{
		ID:               event.ID,
		ProfileID:        event.ProfileID,
		ProfileName:      event.ProfileName,
		ProfileColor:     event.ProfileColor,
		ProfileIcon:      event.ProfileIcon,
		ProfileEmoji:     event.ProfileEmoji,
		ProfileArchived:  event.ProfileArchived,
		SessionID:        event.SessionID,
		WorkspaceID:      event.WorkspaceID,
		Type:             event.Type,
		AgentName:        event.AgentName,
		Provider:         event.Provider,
		Component:        eventspkg.ComponentFor(event.Type),
		Outcome:          logEventOutcome(event),
		Content:          ssepkg.ScrubMemoryContextBytes(event.ContentValue()),
		EventCorrelation: event.Normalize(),
		ParentSessionID:  event.ParentSessionID,
		RootSessionID:    event.RootSessionID,
		SpawnDepth:       event.SpawnDepth,
		Summary:          ssepkg.ScrubMemoryContextString(event.Summary),
		Timestamp:        event.Timestamp,
	}
}

func logEventOutcome(event store.EventSummary) string {
	outcome := strings.TrimSpace(event.Outcome)
	if outcome != "" {
		return outcome
	}
	return string(eventspkg.OutcomeFor(event.Type))
}

func sessionEventCorrelation(event store.SessionEvent) store.EventCorrelation {
	decoded, err := decodeSessionEventCorrelation(event.Content)
	if err != nil {
		slog.Warn(
			"api: decode session event correlation failed",
			"event_id",
			strings.TrimSpace(event.ID),
			"session_id",
			strings.TrimSpace(event.SessionID),
			"type",
			strings.TrimSpace(event.Type),
			"turn_id",
			strings.TrimSpace(event.TurnID),
			"error",
			err,
		)
		return store.EventCorrelation{}
	}
	return decoded.Normalize()
}

type sessionEventCorrelationPayload struct {
	WorktreeID           string `json:"worktree_id"`
	TaskID               string `json:"task_id"`
	RunID                string `json:"run_id"`
	WorkflowID           string `json:"workflow_id"`
	ClaimTokenHash       string `json:"claim_token_hash"`
	LeaseUntil           string `json:"lease_until"`
	CoordinatorSessionID string `json:"coordinator_session_id"`
	SchedulerReason      string `json:"scheduler_reason"`
	HookEvent            string `json:"hook_event"`
	HookName             string `json:"hook_name"`
	ActorKind            string `json:"actor_kind"`
	ActorID              string `json:"actor_id"`
	ReleaseReason        string `json:"release_reason"`
}

func decodeSessionEventCorrelation(payload string) (store.EventCorrelation, error) {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return store.EventCorrelation{}, nil
	}

	var decoded sessionEventCorrelationPayload
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return store.EventCorrelation{}, fmt.Errorf("api: unmarshal session event correlation: %w", err)
	}

	leaseUntil, err := parseSessionEventCorrelationTimestamp(decoded.LeaseUntil)
	if err != nil {
		return store.EventCorrelation{}, fmt.Errorf("api: parse session event lease_until: %w", err)
	}

	return store.EventCorrelation{
		WorktreeID:           decoded.WorktreeID,
		TaskID:               decoded.TaskID,
		RunID:                decoded.RunID,
		WorkflowID:           decoded.WorkflowID,
		ClaimTokenHash:       decoded.ClaimTokenHash,
		LeaseUntil:           leaseUntil,
		CoordinatorSessionID: decoded.CoordinatorSessionID,
		SchedulerReason:      decoded.SchedulerReason,
		HookEvent:            decoded.HookEvent,
		HookName:             decoded.HookName,
		ActorKind:            decoded.ActorKind,
		ActorID:              decoded.ActorID,
		ReleaseReason:        decoded.ReleaseReason,
	}, nil
}

func parseSessionEventCorrelationTimestamp(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return nil, err
	}
	normalized := parsed.UTC()
	return &normalized, nil
}
