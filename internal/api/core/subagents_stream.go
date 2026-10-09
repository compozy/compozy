package core

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func (h *BaseHandlers) subscribeSubagents(ctx context.Context, sessionID string) (<-chan session.SubagentUpdate, func(), error) {
	subscriber, ok := h.Subagents.(session.SubagentUpdateSubscriber)
	if !ok {
		return nil, func() {}, nil
	}
	return subscriber.SubscribeSubagentUpdates(ctx, sessionID)
}

func (h *BaseHandlers) writeSubagentsSnapshot(ctx context.Context, writer FlushWriter, sessionID string, info *session.Info) error {
	if h.Subagents == nil {
		return nil
	}
	workspaceID := ""
	if info != nil {
		workspaceID = info.WorkspaceID
	}
	page, err := h.Subagents.List(ctx, store.SubagentListQuery{WorkspaceID: workspaceID, ParentSessionID: sessionID, Limit: 200})
	if err != nil {
		return err
	}
	return WriteSSE(writer, SSEMessage{Name: contract.SessionStreamEventSubagentsSnapshot,
		Data: contract.SubagentsSnapshotEvent{SessionID: sessionID, Subagents: subagentPagePayloads(page)}})
}

func writeSubagentUpdate(writer FlushWriter, sessionID string, update session.SubagentUpdate) error {
	if update.ParentSessionID != sessionID {
		return nil
	}
	return WriteSSE(writer, SSEMessage{Name: contract.SessionStreamEventSubagentUpdated,
		Data: contract.SubagentUpdatedEvent{SessionID: sessionID, Subagent: contract.SubagentFromDomain(update.Subagent)}})
}
