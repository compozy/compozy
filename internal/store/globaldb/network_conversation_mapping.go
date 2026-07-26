package globaldb

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func networkThreadFromGenerated(row sqlcgen.GetNetworkThreadRow, queryErr error) (store.NetworkThreadSummary, error) {
	if queryErr != nil {
		if errors.Is(queryErr, sql.ErrNoRows) {
			return store.NetworkThreadSummary{}, fmt.Errorf(
				"%w: network thread: %w",
				store.ErrNetworkConversationNotFound,
				queryErr,
			)
		}
		return store.NetworkThreadSummary{}, fmt.Errorf("store: get network thread: %w", queryErr)
	}
	openedAt, err := store.ParseTimestamp(row.OpenedAt)
	if err != nil {
		return store.NetworkThreadSummary{}, fmt.Errorf("store: parse network thread opened_at: %w", err)
	}
	lastActivityAt, err := store.ParseTimestamp(row.LastActivityAt)
	if err != nil {
		return store.NetworkThreadSummary{}, fmt.Errorf("store: parse network thread last_activity_at: %w", err)
	}
	summary := store.NetworkThreadSummary{
		WorkspaceID: row.WorkspaceID, Channel: row.Channel, ThreadID: row.ThreadID,
		RootMessageID: row.RootMessageID, Title: row.Title, OpenedByPeerID: row.OpenedByPeerID,
		OpenedSessionID: row.OpenedSessionID, OpenedAt: openedAt, OpenedSequence: row.OpenedSequence,
		LastActivityAt: lastActivityAt, LastActivitySequence: row.LastActivitySequence,
		MessageCount: int(row.MessageCount), ParticipantCount: int(row.ParticipantCount),
		OpenWorkCount: int(row.OpenWorkCount), DeliveredCount: row.DeliveredCount,
		PromptSizeBytes: row.PromptSizeBytes, EstimatedPromptTokens: row.EstimatedPromptTokens,
		LastMessagePreview: row.LastMessagePreview,
	}
	if err := summary.Validate(); err != nil {
		return store.NetworkThreadSummary{}, err
	}
	return summary, nil
}

func networkDirectRoomFromGenerated(
	row sqlcgen.GetNetworkDirectRoomRow,
	queryErr error,
) (store.NetworkDirectRoomSummary, error) {
	if queryErr != nil {
		if errors.Is(queryErr, sql.ErrNoRows) {
			return store.NetworkDirectRoomSummary{}, fmt.Errorf(
				"%w: network direct room: %w",
				store.ErrNetworkConversationNotFound,
				queryErr,
			)
		}
		return store.NetworkDirectRoomSummary{}, fmt.Errorf("store: get network direct room: %w", queryErr)
	}
	openedAt, err := store.ParseTimestamp(row.OpenedAt)
	if err != nil {
		return store.NetworkDirectRoomSummary{}, fmt.Errorf("store: parse network direct room opened_at: %w", err)
	}
	lastActivityAt, err := store.ParseTimestamp(row.LastActivityAt)
	if err != nil {
		return store.NetworkDirectRoomSummary{}, fmt.Errorf(
			"store: parse network direct room last_activity_at: %w",
			err,
		)
	}
	summary := store.NetworkDirectRoomSummary{
		WorkspaceID: row.WorkspaceID, Channel: row.Channel, DirectID: row.DirectID,
		SessionA: row.SessionA, SessionB: row.SessionB, OpenedAt: openedAt, OpenedSequence: row.OpenedSequence,
		LastActivityAt: lastActivityAt, LastActivitySequence: row.LastActivitySequence,
		MessageCount: int(row.MessageCount), OpenWorkCount: int(row.OpenWorkCount),
		LastMessagePreview: row.LastMessagePreview,
	}
	if err := summary.Validate(); err != nil {
		return store.NetworkDirectRoomSummary{}, err
	}
	return summary, nil
}
