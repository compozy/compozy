package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func updateNetworkChannelProjection(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) error {
	queries := sqlcgen.New(exec)
	if err := queries.EnsureNetworkChannelProjection(ctx, sqlcgen.EnsureNetworkChannelProjectionParams{
		WorkspaceID: entry.WorkspaceID,
		Channel:     entry.Channel,
	}); err != nil {
		return fmt.Errorf("store: ensure network channel projection: %w", err)
	}

	switch {
	case strings.TrimSpace(entry.Kind) == store.NetworkKindGreet:
		if err := updateNetworkChannelPresenceProjection(ctx, exec, entry); err != nil {
			return err
		}
	case networkProjectionMessageIsPublic(entry):
		if err := updateNetworkChannelMessageProjection(ctx, exec, entry); err != nil {
			return err
		}
	}
	return updateNetworkChannelParticipants(ctx, exec, entry)
}

func updateNetworkChannelPresenceProjection(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) error {
	timestamp := store.FormatTimestamp(entry.Timestamp)
	if err := sqlcgen.New(exec).IncrementNetworkChannelPresence(
		ctx,
		sqlcgen.IncrementNetworkChannelPresenceParams{
			LastPresenceAt:       sql.NullString{String: timestamp, Valid: true},
			LastPresenceSequence: entry.Sequence,
			WorkspaceID:          entry.WorkspaceID,
			Channel:              entry.Channel,
		},
	); err != nil {
		return fmt.Errorf("store: update network channel presence projection: %w", err)
	}
	return nil
}

func updateNetworkChannelMessageProjection(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) error {
	timestamp := store.FormatTimestamp(entry.Timestamp)
	messageID := strings.TrimSpace(entry.MessageID)
	preview := strings.TrimSpace(entry.PreviewText)
	if preview == "" {
		preview = strings.TrimSpace(entry.Text)
	}
	queries := sqlcgen.New(exec)
	if err := queries.IncrementNetworkChannelMessage(ctx, sqlcgen.IncrementNetworkChannelMessageParams{
		LastMessagePreview:   preview,
		LastMessageID:        messageID,
		LastMessageSequence:  entry.Sequence,
		LastActivityAt:       sql.NullString{String: timestamp, Valid: true},
		LastActivitySequence: entry.Sequence,
		WorkspaceID:          entry.WorkspaceID,
		Channel:              entry.Channel,
	}); err != nil {
		return fmt.Errorf("store: update network channel message projection: %w", err)
	}
	if err := queries.IncrementNetworkChannelKind(ctx, sqlcgen.IncrementNetworkChannelKindParams{
		WorkspaceID: entry.WorkspaceID,
		Channel:     entry.Channel,
		Kind:        strings.TrimSpace(entry.Kind),
	}); err != nil {
		return fmt.Errorf("store: update network channel kind projection: %w", err)
	}
	return nil
}

func updateNetworkChannelParticipants(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) error {
	sessionID := strings.TrimSpace(entry.SessionID)
	if sessionID == "" {
		return nil
	}
	queries := sqlcgen.New(exec)
	rowsAffected, err := queries.InsertNetworkChannelParticipant(
		ctx,
		sqlcgen.InsertNetworkChannelParticipantParams{
			WorkspaceID: entry.WorkspaceID,
			Channel:     entry.Channel,
			SessionID:   sessionID,
		},
	)
	if err != nil {
		return fmt.Errorf("store: insert network channel participant: %w", err)
	}
	if rowsAffected == 0 {
		return nil
	}
	if err := queries.IncrementNetworkChannelParticipantCount(
		ctx,
		sqlcgen.IncrementNetworkChannelParticipantCountParams{
			WorkspaceID: entry.WorkspaceID,
			Channel:     entry.Channel,
		},
	); err != nil {
		return fmt.Errorf("store: increment network channel participant projection: %w", err)
	}
	return nil
}

func networkProjectionMessageIsPublic(entry store.NetworkConversationMessage) bool {
	return strings.TrimSpace(entry.Kind) != store.NetworkKindGreet &&
		strings.TrimSpace(entry.PeerTo) == "" &&
		strings.TrimSpace(entry.DirectID) == "" &&
		strings.TrimSpace(entry.Surface) != store.NetworkSurfaceDirect
}
