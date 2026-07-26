package session

import (
	"context"

	"fmt"

	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

func (m *Manager) persistRepairActions(
	ctx context.Context,
	recorder EventRecorder,
	meta store.SessionMeta,
	actions []RepairAction,
) ([]RepairAction, error) {
	persisted := make([]RepairAction, 0, len(actions))
	for _, action := range actions {
		event, err := m.repairActionEvent(meta, action)
		if err != nil {
			return persisted, err
		}
		content, err := marshalAgentEvent(event)
		if err != nil {
			return persisted, err
		}
		eventID := store.NewID("ev")
		if err := recorder.Record(ctx, store.SessionEvent{
			ID:        eventID,
			TurnID:    event.TurnID,
			Type:      event.Type,
			AgentName: strings.TrimSpace(meta.AgentName),
			Content:   content,
			Timestamp: event.Timestamp,
		}); err != nil {
			return persisted, fmt.Errorf("session: persist repair event for %q: %w", strings.TrimSpace(meta.ID), err)
		}

		action.EventID = eventID
		action.Persisted = true
		persisted = append(persisted, action)
		m.notifyRepairEvent(ctx, strings.TrimSpace(meta.ID), event)
	}
	return persisted, nil
}

func (m *Manager) repairActionEvent(meta store.SessionMeta, action RepairAction) (acp.AgentEvent, error) {
	now := m.now().UTC()
	event := acp.AgentEvent{
		SessionID: repairACPSessionID(meta),
		TurnID:    strings.TrimSpace(action.TurnID),
		Timestamp: now,
	}

	switch action.Code {
	case RepairActionAppendInterruptedToolResult:
		raw, err := interruptedToolResultRaw(action.ToolCallID, action.ToolName)
		if err != nil {
			return acp.AgentEvent{}, err
		}
		event.Type = acp.EventTypeToolResult
		event.ToolCallID = strings.TrimSpace(action.ToolCallID)
		event.Title = firstNonEmpty(action.ToolName, "interrupted tool result")
		event.Error = repairInterruptedToolMessage
		event.Raw = raw
	case RepairActionAppendTerminalError:
		event.Type = acp.EventTypeError
		event.Error = repairTerminalErrorMessage
		event.StopReason = string(sessionMetaStopReason(meta))
		event.Failure = store.CloneSessionFailure(meta.Failure)
	default:
		return acp.AgentEvent{}, fmt.Errorf("session: unknown repair action %q", action.Code)
	}
	return event, nil
}

func (m *Manager) notifyRepairEvent(ctx context.Context, sessionID string, event acp.AgentEvent) {
	if m == nil || m.notifier == nil {
		return
	}
	m.notifier.OnAgentEvent(ctx, sessionID, event)
}

func repairACPSessionID(meta store.SessionMeta) string {
	if meta.ACPSessionID == nil {
		return ""
	}
	return strings.TrimSpace(*meta.ACPSessionID)
}
