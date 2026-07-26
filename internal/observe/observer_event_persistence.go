package observe

import (
	"context"

	"strings"

	"time"

	"github.com/compozy/agh/internal/acp"

	"github.com/compozy/agh/internal/store"
)

func observedEventTimestamp(event acp.AgentEvent, now func() time.Time) time.Time {
	if !event.Timestamp.IsZero() {
		return event.Timestamp
	}
	return now()
}

func (o *Observer) writeObservedEventSummary(
	ctx context.Context,
	sessionID string,
	snapshot observedSession,
	event acp.AgentEvent,
	timestamp time.Time,
) error {
	correlation := event.Normalize()
	return o.registry.WriteEventSummary(ctx, store.EventSummary{
		SessionID:        sessionID,
		WorkspaceID:      snapshot.workspaceID,
		Type:             strings.TrimSpace(event.Type),
		AgentName:        snapshot.agentName,
		Content:          observedEventContent(event),
		EventCorrelation: correlation,
		ParentSessionID:  snapshot.parentSessionID,
		RootSessionID:    snapshot.rootSessionID,
		SpawnDepth:       snapshot.spawnDepth,
		Summary:          summarizeEvent(event),
		Timestamp:        timestamp,
	})
}

func (o *Observer) writeObservedPermissionLog(
	ctx context.Context,
	sessionID string,
	snapshot observedSession,
	event acp.AgentEvent,
	timestamp time.Time,
) error {
	if strings.TrimSpace(event.Type) != acp.EventTypePermission {
		return nil
	}

	policyUsed := strings.TrimSpace(snapshot.permissionMode)
	if policyUsed == "" {
		o.logger.Warn(
			"observe: skipped permission log without resolved policy",
			"session_id",
			sessionID,
			"agent_name",
			snapshot.agentName,
			"workspace_id",
			snapshot.workspaceID,
		)
		return nil
	}
	if strings.TrimSpace(event.Decision) == "" {
		return nil
	}

	return o.registry.WritePermissionLog(ctx, store.PermissionLogEntry{
		SessionID:  sessionID,
		AgentName:  snapshot.agentName,
		Action:     strings.TrimSpace(event.Action),
		Resource:   strings.TrimSpace(event.Resource),
		Decision:   strings.TrimSpace(event.Decision),
		PolicyUsed: policyUsed,
		Timestamp:  timestamp,
	})
}

func (o *Observer) logObservedEventFailure(
	message string,
	sessionID string,
	snapshot observedSession,
	event acp.AgentEvent,
	err error,
) {
	o.logger.Warn(
		message,
		"session_id",
		sessionID,
		"agent_name",
		snapshot.agentName,
		"workspace_id",
		snapshot.workspaceID,
		"event_type",
		event.Type,
		"error",
		err,
	)
}

func normalizeObservedAgentEvent(payload any) (acp.AgentEvent, bool) {
	switch event := payload.(type) {
	case acp.AgentEvent:
		return event, true
	case *acp.AgentEvent:
		if event == nil {
			return acp.AgentEvent{}, false
		}
		return *event, true
	default:
		return acp.AgentEvent{}, false
	}
}
