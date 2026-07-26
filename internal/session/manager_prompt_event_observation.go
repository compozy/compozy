package session

import (
	"context"
	"encoding/json"

	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

func (m *Manager) observeRecordAndNotifyPromptEvent(
	ctx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	loop *promptPumpLoopState,
	normalized acp.AgentEvent,
	runtimeEvent bool,
) error {
	if loop.activity != nil && !runtimeEvent {
		loop.activity.observeEvent(normalized)
	}
	if err := m.recordEvent(ctx, session, normalized); err != nil {
		return fmt.Errorf("session: record prompt event: %w", err)
	}
	loop.fileMutations.Observe(normalized)
	m.emitFileMutationMarkerBeforeTerminalNotification(ctx, session, turnState, loop, normalized)
	m.notifyManagedPromptEvent(ctx, session, turnState, normalized)
	m.scheduleCompactionFromUsage(session, normalized)
	if kind, summary, evidence, ok := promptTranscriptMarker(normalized); ok {
		m.emitTranscriptMarker(ctx, session, turnState.turnID, kind, summary, evidence)
	}
	return nil
}

func promptTranscriptMarker(event acp.AgentEvent) (string, string, map[string]any, bool) {
	summary := firstNonEmpty(event.Text, event.Error, runtimeActivityDetail(event.Runtime))
	evidence := map[string]any{
		"event_type": event.Type,
	}
	switch {
	case event.Type == acp.EventTypeUserMessage && event.Action == acp.PromptActionSteered:
		if queueEntryID := strings.TrimSpace(event.RequestID); queueEntryID != "" {
			evidence["queue_entry_id"] = queueEntryID
		}
		if generation := strings.TrimSpace(event.Decision); generation != "" {
			evidence[promptEvidenceQueueGenerationKey] = generation
		}
		return transcript.MarkerPromptSteered,
			firstNonEmpty(summary, "Steering input injected at tool result boundary."),
			evidence,
			true
	case event.Type == acp.EventTypeError && event.Failure != nil:
		failure := event.Failure.Normalize()
		evidence["failure_kind"] = string(failure.Kind)
		if failure.Kind == store.FailureProviderAuth || failure.Kind == store.FailurePermission {
			if eventHasMCPAuthReason(event) {
				return transcript.MarkerMCPAuthRequired,
					firstNonEmpty(summary, failure.Summary, "MCP authentication is required."),
					evidence,
					true
			}
			return transcript.MarkerProviderFailure,
				firstNonEmpty(summary, failure.Summary, "Provider authentication failed."),
				evidence,
				true
		}
		return transcript.MarkerProviderFailure,
			firstNonEmpty(summary, failure.Summary, "Provider failed."),
			evidence,
			true
	default:
		return "", "", nil, false
	}
}

func eventHasMCPAuthReason(event acp.AgentEvent) bool {
	if len(event.Raw) == 0 {
		return false
	}
	var payload any
	if err := json.Unmarshal(event.Raw, &payload); err != nil {
		return false
	}
	return jsonValueHasMCPAuthReason(payload, 0)
}

func jsonValueHasMCPAuthReason(value any, depth int) bool {
	if depth > maxMCPAuthReasonJSONDepth {
		return false
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if jsonReasonFieldHasMCPAuthReason(key, nested) {
				return true
			}
			if jsonValueHasMCPAuthReason(nested, depth+1) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(typed, func(nested any) bool {
			return jsonValueHasMCPAuthReason(nested, depth+1)
		})
	}
	return false
}

func jsonReasonFieldHasMCPAuthReason(key string, value any) bool {
	switch strings.TrimSpace(key) {
	case jsonReasonFieldReason, "reason_code", "reason_codes", "code", "diagnostic_code":
	default:
		return false
	}
	switch typed := value.(type) {
	case string:
		return mcpAuthReasonCode(typed)
	case []any:
		for _, item := range typed {
			if jsonReasonFieldHasMCPAuthReason(key, item) {
				return true
			}
		}
	}
	return false
}

func mcpAuthReasonCode(reason string) bool {
	switch strings.TrimSpace(reason) {
	case "mcp_auth_unconfigured",
		"mcp_auth_required",
		"mcp_auth_expired",
		"mcp_auth_invalid",
		"mcp_auth_refresh_failed":
		return true
	default:
		return false
	}
}

func runtimeActivityDetail(activity *acp.RuntimeActivity) string {
	if activity == nil {
		return ""
	}
	return activity.LastActivityDetail
}
