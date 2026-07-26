package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func (g *NetworkRepo) normalizeDirectRoomEntry(
	entry store.NetworkDirectRoomEntry,
) (store.NetworkDirectRoomEntry, error) {
	now := g.now()
	directID, sessionA, sessionB, err := store.NetworkDirectRoomIdentity(
		entry.WorkspaceID,
		entry.Channel,
		entry.SessionA,
		entry.SessionB,
	)
	if err != nil {
		return store.NetworkDirectRoomEntry{}, err
	}
	if existing := strings.TrimSpace(entry.DirectID); existing != "" && existing != directID {
		return store.NetworkDirectRoomEntry{}, fmt.Errorf(
			"%w: direct_id=%q expected=%q",
			store.ErrNetworkDirectRoomCollision,
			existing,
			directID,
		)
	}
	normalized := store.NetworkDirectRoomEntry{
		WorkspaceID:    strings.TrimSpace(entry.WorkspaceID),
		Channel:        strings.TrimSpace(entry.Channel),
		DirectID:       directID,
		SessionA:       sessionA,
		SessionB:       sessionB,
		OpenedAt:       entry.OpenedAt,
		LastActivityAt: entry.LastActivityAt,
	}
	if normalized.OpenedAt.IsZero() {
		normalized.OpenedAt = now
	}
	if normalized.LastActivityAt.IsZero() {
		normalized.LastActivityAt = normalized.OpenedAt
	}
	if err := normalized.Validate(); err != nil {
		return store.NetworkDirectRoomEntry{}, err
	}
	return normalized, nil
}

func (g *NetworkRepo) normalizeConversationMessage(
	entry store.NetworkConversationMessage,
) (store.NetworkConversationMessage, error) {
	normalized := store.NetworkConversationMessage{
		MessageID:   strings.TrimSpace(entry.MessageID),
		SessionID:   strings.TrimSpace(entry.SessionID),
		WorkspaceID: strings.TrimSpace(entry.WorkspaceID),
		Channel:     strings.TrimSpace(entry.Channel),
		Surface:     strings.TrimSpace(entry.Surface),
		ThreadID:    strings.TrimSpace(entry.ThreadID),
		DirectID:    strings.TrimSpace(entry.DirectID),
		Direction:   entry.Direction,
		PeerFrom:    strings.TrimSpace(entry.PeerFrom),
		PeerTo:      strings.TrimSpace(entry.PeerTo),
		Kind:        strings.TrimSpace(entry.Kind),
		WorkID:      strings.TrimSpace(entry.WorkID),
		ReplyTo:     strings.TrimSpace(entry.ReplyTo),
		TraceID:     strings.TrimSpace(entry.TraceID),
		CausationID: strings.TrimSpace(entry.CausationID),
		Intent:      strings.TrimSpace(entry.Intent),
		Text:        strings.TrimSpace(entry.Text),
		PreviewText: strings.TrimSpace(entry.PreviewText),
		Mentions:    append([]string(nil), entry.Mentions...),
		ExtJSON:     append(json.RawMessage(nil), entry.ExtJSON...),
		Body:        append(json.RawMessage(nil), entry.Body...),
		Timestamp:   entry.Timestamp,
	}
	mentions, err := store.NormalizeNetworkPeerIDs(normalized.Mentions, "network message mentions")
	if err != nil {
		return store.NetworkConversationMessage{}, err
	}
	normalized.Mentions = mentions
	if normalized.PreviewText == "" {
		normalized.PreviewText = normalized.Text
	}
	if normalized.Timestamp.IsZero() {
		normalized.Timestamp = g.now()
	}
	normalized.Timestamp = normalized.Timestamp.UTC()
	if err := normalized.Validate(); err != nil {
		return store.NetworkConversationMessage{}, fmt.Errorf("store: validate network conversation message: %w", err)
	}
	if strings.TrimSpace(normalized.SessionID) == "" {
		return store.NetworkConversationMessage{}, fmt.Errorf(
			"store: network conversation message session_id is required",
		)
	}
	return normalized, nil
}

func resolveDirectRoomWithExecutor(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkDirectRoomEntry,
) (store.NetworkDirectRoomSummary, bool, error) {
	rowsAffected, err := sqlcgen.New(exec).InsertNetworkDirectRoom(ctx, sqlcgen.InsertNetworkDirectRoomParams{
		WorkspaceID: entry.WorkspaceID, Channel: entry.Channel, DirectID: entry.DirectID,
		SessionA: entry.SessionA, SessionB: entry.SessionB,
		OpenedAt:       store.FormatTimestamp(entry.OpenedAt),
		LastActivityAt: store.FormatTimestamp(entry.LastActivityAt),
	})
	if err != nil {
		return store.NetworkDirectRoomSummary{}, false, fmt.Errorf("store: insert network direct room: %w", err)
	}

	summary, err := getDirectRoomBySessionPairWithExecutor(
		ctx,
		exec,
		entry.WorkspaceID,
		entry.Channel,
		entry.SessionA,
		entry.SessionB,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.NetworkDirectRoomSummary{}, false, fmt.Errorf(
				"%w: direct_id=%q session_a=%q session_b=%q",
				store.ErrNetworkDirectRoomCollision,
				entry.DirectID,
				entry.SessionA,
				entry.SessionB,
			)
		}
		return store.NetworkDirectRoomSummary{}, false, err
	}
	if summary.DirectID != entry.DirectID {
		return store.NetworkDirectRoomSummary{}, false, fmt.Errorf(
			"%w: direct_id=%q expected=%q",
			store.ErrNetworkDirectRoomCollision,
			summary.DirectID,
			entry.DirectID,
		)
	}
	return summary, rowsAffected > 0, nil
}

func getDirectRoomBySessionPairWithExecutor(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	channel string,
	sessionA string,
	sessionB string,
) (store.NetworkDirectRoomSummary, error) {
	row, err := sqlcgen.New(exec).GetNetworkDirectRoomBySessionPair(
		ctx,
		sqlcgen.GetNetworkDirectRoomBySessionPairParams{
			WorkspaceID: workspaceID, Channel: channel, SessionA: sessionA, SessionB: sessionB,
		},
	)
	if err != nil {
		return store.NetworkDirectRoomSummary{}, err
	}
	openedAt, err := store.ParseTimestamp(row.OpenedAt)
	if err != nil {
		return store.NetworkDirectRoomSummary{}, fmt.Errorf("store: parse network direct opened_at: %w", err)
	}
	lastActivityAt, err := store.ParseTimestamp(row.LastActivityAt)
	if err != nil {
		return store.NetworkDirectRoomSummary{}, fmt.Errorf("store: parse network direct last_activity_at: %w", err)
	}
	return store.NetworkDirectRoomSummary{
		WorkspaceID: row.WorkspaceID, Channel: row.Channel, DirectID: row.DirectID,
		SessionA: row.SessionA, SessionB: row.SessionB, OpenedAt: openedAt,
		OpenedSequence: row.OpenedSequence, LastActivityAt: lastActivityAt,
		LastActivitySequence: row.LastActivitySequence, MessageCount: int(row.MessageCount),
		OpenWorkCount: int(row.OpenWorkCount), LastMessagePreview: row.LastMessagePreview,
	}, nil
}

func insertNetworkTimelineMessageWithExecutor(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) (bool, int64, error) {
	result, err := sqlcgen.New(exec).InsertNetworkTimelineMessage(
		ctx,
		sqlcgen.InsertNetworkTimelineMessageParams{
			MessageID:   entry.MessageID,
			SessionID:   nullableNetworkString(entry.SessionID),
			WorkspaceID: entry.WorkspaceID,
			Channel:     entry.Channel,
			Surface:     nullableNetworkString(entry.Surface),
			ThreadID:    nullableNetworkString(entry.ThreadID),
			DirectID:    nullableNetworkString(entry.DirectID),
			Direction:   entry.Direction,
			PeerFrom:    entry.PeerFrom,
			PeerTo:      nullableNetworkString(entry.PeerTo),
			Kind:        entry.Kind,
			WorkID:      nullableNetworkString(entry.WorkID),
			ReplyTo:     nullableNetworkString(entry.ReplyTo),
			TraceID:     nullableNetworkString(entry.TraceID),
			CausationID: nullableNetworkString(entry.CausationID),
			Intent:      nullableNetworkString(entry.Intent),
			Text:        nullableNetworkString(entry.Text),
			PreviewText: entry.PreviewText,
			MentionsJson: networkMentionsJSONString(
				entry.Mentions,
			),
			ExtJson:   networkMessageExtJSONString(entry.ExtJSON),
			BodyJson:  string(entry.Body),
			Timestamp: store.FormatTimestamp(entry.Timestamp),
		},
	)
	if err != nil {
		return false, 0, fmt.Errorf("store: insert network conversation message: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, 0, fmt.Errorf("store: inspect network conversation message insert: %w", err)
	}
	if rowsAffected == 0 {
		return false, 0, nil
	}
	sequence, err := result.LastInsertId()
	if err != nil {
		return false, 0, fmt.Errorf("store: inspect network conversation message sequence: %w", err)
	}
	if sequence <= 0 {
		return false, 0, fmt.Errorf("store: network conversation message sequence must be positive: %d", sequence)
	}
	entry.Sequence = sequence
	if err := updateNetworkChannelProjection(ctx, exec, entry); err != nil {
		return false, 0, err
	}
	return true, sequence, nil
}

func lookupNetworkMessageAcceptance(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	messageID string,
) (int64, time.Time, error) {
	// dynamic-sql: this exact identity lookup is intentionally scoped by workspace and message id.
	const query = `
SELECT sequence, timestamp
FROM network_timeline_log
WHERE workspace_id = ? AND message_id = ?`
	var sequence int64
	var timestampRaw string
	if err := exec.QueryRowContext(ctx, query, workspaceID, messageID).Scan(&sequence, &timestampRaw); err != nil {
		return 0, time.Time{}, fmt.Errorf("store: lookup network message acceptance: %w", err)
	}
	timestamp, err := store.ParseTimestamp(timestampRaw)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("store: parse network message acceptance timestamp: %w", err)
	}
	return sequence, timestamp, nil
}

func ensureNetworkConversationContainer(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) (bool, error) {
	switch entry.Surface {
	case store.NetworkSurfaceThread:
		return ensureNetworkThreadWithExecutor(ctx, exec, entry)
	case store.NetworkSurfaceDirect:
		return ensureNetworkDirectRoomWithExecutor(ctx, exec, entry)
	default:
		return false, nil
	}
}

func ensureNetworkThreadWithExecutor(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) (bool, error) {
	rowsAffected, err := sqlcgen.New(exec).InsertNetworkThread(ctx, sqlcgen.InsertNetworkThreadParams{
		WorkspaceID: entry.WorkspaceID, Channel: entry.Channel, ThreadID: entry.ThreadID,
		RootMessageID: entry.MessageID, Title: entry.PreviewText, OpenedByPeerID: entry.PeerFrom,
		OpenedSessionID: entry.SessionID, OpenedAt: store.FormatTimestamp(entry.Timestamp),
		OpenedSequence: entry.Sequence, LastActivityAt: store.FormatTimestamp(entry.Timestamp),
		LastActivitySequence: entry.Sequence,
	})
	if err != nil {
		return false, fmt.Errorf("store: insert network thread: %w", err)
	}
	return rowsAffected > 0, nil
}

func ensureNetworkDirectRoomWithExecutor(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) (bool, error) {
	if strings.TrimSpace(entry.PeerTo) == "" {
		return false, fmt.Errorf("store: network direct message peer_to is required")
	}
	directID, sessionA, sessionB, err := store.NetworkDirectRoomIdentity(
		entry.WorkspaceID,
		entry.Channel,
		entry.PeerFrom,
		entry.PeerTo,
	)
	if err != nil {
		return false, err
	}
	if entry.DirectID != directID {
		return false, fmt.Errorf(
			"%w: direct_id=%q expected=%q",
			store.ErrNetworkDirectRoomCollision,
			entry.DirectID,
			directID,
		)
	}
	_, opened, err := resolveDirectRoomWithExecutor(ctx, exec, store.NetworkDirectRoomEntry{
		WorkspaceID:    entry.WorkspaceID,
		Channel:        entry.Channel,
		DirectID:       directID,
		SessionA:       sessionA,
		SessionB:       sessionB,
		OpenedAt:       entry.Timestamp,
		LastActivityAt: entry.Timestamp,
	})
	if err != nil {
		return opened, err
	}
	rowsAffected, err := sqlcgen.New(exec).InitializeNetworkDirectRoomSequence(
		ctx,
		sqlcgen.InitializeNetworkDirectRoomSequenceParams{
			OpenedSequence: entry.Sequence, LastActivitySequence: entry.Sequence,
			WorkspaceID: entry.WorkspaceID, Channel: entry.Channel, DirectID: entry.DirectID,
		},
	)
	if err != nil {
		return false, fmt.Errorf("store: initialize network direct room sequence: %w", err)
	}
	return opened || rowsAffected > 0, nil
}
