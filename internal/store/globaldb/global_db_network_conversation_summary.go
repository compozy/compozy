package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func upsertNetworkThreadParticipant(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) error {
	sessionID := strings.TrimSpace(entry.SessionID)
	if sessionID == "" {
		return nil
	}
	err := sqlcgen.New(exec).UpsertNetworkThreadParticipant(ctx, sqlcgen.UpsertNetworkThreadParticipantParams{
		WorkspaceID: entry.WorkspaceID, Channel: entry.Channel, ThreadID: entry.ThreadID,
		SessionID: sessionID, FirstMessageID: entry.MessageID,
		FirstSeenAt: store.FormatTimestamp(entry.Timestamp), LastSeenAt: store.FormatTimestamp(entry.Timestamp),
	})
	if err != nil {
		return fmt.Errorf("store: upsert network thread participant: %w", err)
	}
	return nil
}

func refreshNetworkConversationSummary(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) error {
	switch entry.Surface {
	case store.NetworkSurfaceThread:
		return refreshNetworkThreadSummary(ctx, exec, entry)
	case store.NetworkSurfaceDirect:
		return refreshNetworkDirectRoomSummary(ctx, exec, entry)
	default:
		return nil
	}
}

func refreshNetworkThreadSummary(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) error {
	latest, err := latestNetworkConversationMessage(
		ctx,
		exec,
		entry.WorkspaceID,
		entry.Channel,
		entry.Surface,
		entry.ThreadID,
	)
	if err != nil {
		return err
	}
	queries := sqlcgen.New(exec)
	messageCount, err := queries.CountNetworkThreadMessages(ctx, sqlcgen.CountNetworkThreadMessagesParams{
		WorkspaceID: entry.WorkspaceID, Channel: entry.Channel,
		ThreadID: nullableNetworkString(entry.ThreadID),
	})
	if err != nil {
		return fmt.Errorf("store: count network thread messages: %w", err)
	}
	participantCount, err := queries.CountNetworkThreadParticipants(
		ctx,
		sqlcgen.CountNetworkThreadParticipantsParams{
			WorkspaceID: entry.WorkspaceID, Channel: entry.Channel, ThreadID: entry.ThreadID,
		},
	)
	if err != nil {
		return fmt.Errorf("store: count network thread participants: %w", err)
	}
	openWorkCount, err := countOpenNetworkWork(
		ctx,
		exec,
		entry.WorkspaceID,
		entry.Channel,
		entry.Surface,
		entry.ThreadID,
		"",
	)
	if err != nil {
		return err
	}
	if err := queries.UpdateNetworkThreadSummary(ctx, sqlcgen.UpdateNetworkThreadSummaryParams{
		LastActivityAt: latest.timestamp, LastActivitySequence: latest.sequence,
		MessageCount: messageCount, ParticipantCount: participantCount, OpenWorkCount: int64(openWorkCount),
		LastMessagePreview: latest.preview, WorkspaceID: entry.WorkspaceID,
		Channel: entry.Channel, ThreadID: entry.ThreadID,
	}); err != nil {
		return fmt.Errorf("store: update network thread summary: %w", err)
	}
	return nil
}

func refreshNetworkDirectRoomSummary(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) error {
	latest, err := latestNetworkConversationMessage(
		ctx,
		exec,
		entry.WorkspaceID,
		entry.Channel,
		entry.Surface,
		entry.DirectID,
	)
	if err != nil {
		return err
	}
	queries := sqlcgen.New(exec)
	messageCount, err := queries.CountNetworkDirectMessages(ctx, sqlcgen.CountNetworkDirectMessagesParams{
		WorkspaceID: entry.WorkspaceID, Channel: entry.Channel,
		DirectID: nullableNetworkString(entry.DirectID),
	})
	if err != nil {
		return fmt.Errorf("store: count network direct messages: %w", err)
	}
	openWorkCount, err := countOpenNetworkWork(
		ctx,
		exec,
		entry.WorkspaceID,
		entry.Channel,
		entry.Surface,
		"",
		entry.DirectID,
	)
	if err != nil {
		return err
	}
	if err := queries.UpdateNetworkDirectRoomSummary(ctx, sqlcgen.UpdateNetworkDirectRoomSummaryParams{
		LastActivityAt: latest.timestamp, LastActivitySequence: latest.sequence,
		MessageCount: messageCount, OpenWorkCount: int64(openWorkCount), LastMessagePreview: latest.preview,
		WorkspaceID: entry.WorkspaceID, Channel: entry.Channel, DirectID: entry.DirectID,
	}); err != nil {
		return fmt.Errorf("store: update network direct room summary: %w", err)
	}
	return nil
}

type latestNetworkMessage struct {
	sequence  int64
	timestamp string
	preview   string
}

func latestNetworkConversationMessage(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	channel string,
	surface string,
	containerID string,
) (latestNetworkMessage, error) {
	queries := sqlcgen.New(exec)
	if surface == store.NetworkSurfaceDirect {
		row, err := queries.GetLatestNetworkDirectMessage(ctx, sqlcgen.GetLatestNetworkDirectMessageParams{
			WorkspaceID: workspaceID, Channel: channel, DirectID: nullableNetworkString(containerID),
		})
		if err != nil {
			return latestNetworkMessage{}, fmt.Errorf("store: lookup latest network conversation message: %w", err)
		}
		return latestNetworkMessage{sequence: row.Sequence, timestamp: row.Timestamp, preview: row.PreviewText}, nil
	}
	row, err := queries.GetLatestNetworkThreadMessage(ctx, sqlcgen.GetLatestNetworkThreadMessageParams{
		WorkspaceID: workspaceID, Channel: channel, ThreadID: nullableNetworkString(containerID),
	})
	if err != nil {
		return latestNetworkMessage{}, fmt.Errorf("store: lookup latest network conversation message: %w", err)
	}
	return latestNetworkMessage{sequence: row.Sequence, timestamp: row.Timestamp, preview: row.PreviewText}, nil
}

func countOpenNetworkWork(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	channel string,
	surface string,
	threadID string,
	directID string,
) (int, error) {
	queries := sqlcgen.New(exec)
	if surface == store.NetworkSurfaceThread {
		count, err := queries.CountOpenNetworkThreadWork(ctx, sqlcgen.CountOpenNetworkThreadWorkParams{
			WorkspaceID: workspaceID, Channel: channel, ThreadID: nullableNetworkString(threadID),
			Completed: store.NetworkWorkStateCompleted, Failed: store.NetworkWorkStateFailed,
			Canceled: store.NetworkWorkStateCanceled,
		})
		if err != nil {
			return 0, fmt.Errorf("store: count open network work: %w", err)
		}
		return int(count), nil
	}
	count, err := queries.CountOpenNetworkDirectWork(ctx, sqlcgen.CountOpenNetworkDirectWorkParams{
		WorkspaceID: workspaceID, Channel: channel, DirectID: nullableNetworkString(directID),
		Completed: store.NetworkWorkStateCompleted, Failed: store.NetworkWorkStateFailed,
		Canceled: store.NetworkWorkStateCanceled,
	})
	if err != nil {
		return 0, fmt.Errorf("store: count open network work: %w", err)
	}
	return int(count), nil
}

func auditEntryForConversationMessage(entry store.NetworkConversationMessage) store.NetworkAuditEntry {
	return store.NetworkAuditEntry{
		ID:          store.NewID("naud"),
		SessionID:   entry.SessionID,
		WorkspaceID: entry.WorkspaceID,
		Direction:   entry.Direction,
		Kind:        entry.Kind,
		Channel:     entry.Channel,
		Surface:     entry.Surface,
		ThreadID:    entry.ThreadID,
		DirectID:    entry.DirectID,
		WorkID:      entry.WorkID,
		PeerFrom:    entry.PeerFrom,
		PeerTo:      entry.PeerTo,
		MessageID:   entry.MessageID,
		Size:        len(entry.Body),
		Timestamp:   entry.Timestamp,
	}
}
