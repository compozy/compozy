package core

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func (h *BaseHandlers) subscribeSubagents(
	ctx context.Context,
	sessionID string,
) (<-chan session.SubagentUpdate, func(), error) {
	subscriber, ok := h.Subagents.(session.SubagentUpdateSubscriber)
	if !ok {
		return nil, func() {}, nil
	}
	return subscriber.SubscribeSubagentUpdates(ctx, sessionID)
}

func (h *BaseHandlers) writeSubagentsSnapshot(
	ctx context.Context,
	writer FlushWriter,
	sessionID string,
	info *session.Info,
) error {
	if h.Subagents == nil {
		return nil
	}
	workspaceID := ""
	if info != nil {
		workspaceID = info.WorkspaceID
	}
	// The snapshot replaces the client's whole roster, so it carries every row:
	// a row missing from it would read as an unconfirmed (still running) card.
	var all store.SubagentPage
	query := store.SubagentListQuery{
		WorkspaceID:     workspaceID,
		ParentSessionID: sessionID,
		Limit:           subagentSnapshotPageSize,
	}
	for {
		page, err := h.Subagents.List(ctx, query)
		if err != nil {
			return err
		}
		all.Items = append(all.Items, page.Items...)
		if page.NextCursor == "" {
			break
		}
		query.Cursor = page.NextCursor
	}
	return WriteSSE(writer, SSEMessage{Name: contract.SessionStreamEventSubagentsSnapshot,
		Data: contract.SubagentsSnapshotEvent{SessionID: sessionID, Subagents: subagentPagePayloads(all)}})
}

const subagentSnapshotPageSize = 200

func writeSubagentUpdate(writer FlushWriter, sessionID string, update *session.SubagentUpdate) error {
	if update.ParentSessionID != sessionID {
		return nil
	}
	return WriteSSE(writer, SSEMessage{
		Name: contract.SessionStreamEventSubagentUpdated,
		Data: contract.SubagentUpdatedEvent{
			SessionID: sessionID,
			Subagent:  contract.SubagentFromDomain(&update.Subagent),
		},
	})
}

func (h *BaseHandlers) streamSubagentUpdate(
	writer FlushWriter, sessionID string, update *session.SubagentUpdate, open bool,
) bool {
	if !open {
		return false
	}
	if err := writeSubagentUpdate(writer, sessionID, update); err != nil {
		h.writeTranscriptStreamError(writer, err)
		return false
	}
	return true
}
