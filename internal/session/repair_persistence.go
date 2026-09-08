package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) persistRepairActions(
	ctx context.Context,
	recorder EventRecorder,
	meta store.SessionMeta,
	actions []RepairAction,
) ([]RepairAction, error) {
	type preparedRepairAction struct {
		action      RepairAction
		agentEvent  acp.AgentEvent
		storedEvent store.SessionEvent
	}

	prepared := make([]preparedRepairAction, 0, len(actions))
	for _, action := range actions {
		event, err := m.repairActionEvent(meta, action)
		if err != nil {
			return nil, err
		}
		content, err := marshalAgentEvent(event)
		if err != nil {
			return nil, err
		}
		eventID, err := m.newRepairEventID()
		if err != nil {
			return nil, fmt.Errorf("session: generate repair event id: %w", err)
		}
		action.EventID = eventID
		prepared = append(prepared, preparedRepairAction{
			action:     action,
			agentEvent: event,
			storedEvent: store.SessionEvent{
				ID:        eventID,
				TurnID:    event.TurnID,
				Type:      event.Type,
				AgentName: strings.TrimSpace(meta.AgentName),
				Content:   content,
				Timestamp: event.Timestamp,
			},
		})
	}

	persisted := make([]RepairAction, 0, len(prepared))
	for index := range prepared {
		item := &prepared[index]
		if err := recorder.Record(ctx, item.storedEvent); err != nil {
			return persisted, fmt.Errorf("session: persist repair event for %q: %w", strings.TrimSpace(meta.ID), err)
		}

		item.action.Persisted = true
		persisted = append(persisted, item.action)
		m.notifyRepairEvent(ctx, meta, item.agentEvent)
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
		event.Title = firstTrimmedNonEmpty(action.ToolName, "interrupted tool result")
		event.Error = repairInterruptedToolMessage
		event.Raw = raw
	case RepairActionAppendTerminalError:
		event.Type = acp.EventTypeError
		event.Error = repairTerminalErrorMessage
		event.StopReason = string(sessionMetaStopReason(&meta))
		event.Failure = store.CloneSessionFailure(meta.Failure)
	default:
		return acp.AgentEvent{}, fmt.Errorf("session: unknown repair action %q", action.Code)
	}
	return event, nil
}

func (m *Manager) notifyRepairEvent(ctx context.Context, meta store.SessionMeta, event acp.AgentEvent) {
	if m == nil || m.notifier == nil {
		return
	}
	m.notifyAgentEventFromInfo(ctx, m.sessionInfoFromMeta(ctx, meta), event)
}

func repairACPSessionID(meta store.SessionMeta) string {
	if meta.ACPSessionID == nil {
		return ""
	}
	return strings.TrimSpace(*meta.ACPSessionID)
}
