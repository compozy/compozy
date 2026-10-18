package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (h *HostAPIHandler) handleObserveHealth(ctx context.Context, _ json.RawMessage) (any, error) {
	if h.observer == nil {
		return nil, errors.New("extension: observer is not configured")
	}
	return h.observer.Health(ctx)
}

func (h *HostAPIHandler) handleListLogs(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.observer == nil {
		return nil, errors.New("extension: observer is not configured")
	}

	var params hostAPIListLogsParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	workspaceID, err := h.resolveRequiredWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	profileID := hostAPIProfileID(ctx)

	events, err := h.observer.QueryEvents(ctx, store.EventSummaryQuery{
		ReadScope:     store.ReadScope{ProfileID: profileID},
		WorkspaceID:   workspaceID,
		SessionID:     strings.TrimSpace(params.SessionID),
		AgentName:     strings.TrimSpace(params.AgentName),
		Type:          strings.TrimSpace(params.Type),
		RunID:         strings.TrimSpace(params.RunID),
		ActorKind:     strings.TrimSpace(params.ActorKind),
		ActorID:       strings.TrimSpace(params.ActorID),
		Provider:      strings.TrimSpace(params.Provider),
		Outcome:       strings.TrimSpace(params.Outcome),
		Component:     strings.TrimSpace(params.Component),
		ErrorOnly:     params.ErrorOnly,
		AfterSequence: params.AfterSequence,
		Since:         params.Since,
		Limit:         params.Limit,
	})
	if err != nil {
		return nil, err
	}

	result := make([]hostAPISessionEvent, 0, len(events))
	for _, event := range events {
		result = append(result, hostAPISessionEvent{
			Type:      event.Type,
			Timestamp: event.Timestamp,
			Data: map[string]any{
				hostAPIWorkspaceIDKey: event.WorkspaceID,
				hostAPISessionIDKey:   event.SessionID,
				hostAPIAgentNameKey:   event.AgentName,
				manifestSummaryKey:    event.Summary,
			},
		})
	}

	return result, nil
}
