package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// ListThreadParticipants returns sessions recorded in one public thread.
func (g *NetworkRepo) ListThreadParticipants(
	ctx context.Context,
	ref store.NetworkChannelRef,
	threadID string,
) (participants []store.NetworkThreadParticipant, err error) {
	if err := g.checkReady(ctx, "list network thread participants"); err != nil {
		return nil, err
	}
	conversationRef := store.NetworkConversationRef{
		WorkspaceID: ref.WorkspaceID,
		Channel:     ref.Channel,
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    strings.TrimSpace(threadID),
	}
	if err := conversationRef.Validate(); err != nil {
		return nil, err
	}

	rows, err := g.queries.ListNetworkThreadParticipants(ctx, sqlcgen.ListNetworkThreadParticipantsParams{
		WorkspaceID: conversationRef.WorkspaceID,
		Channel:     conversationRef.Channel,
		ThreadID:    conversationRef.ThreadID,
	})
	if err != nil {
		return nil, fmt.Errorf("store: query network thread participants: %w", err)
	}
	participants = make([]store.NetworkThreadParticipant, 0, len(rows))
	for _, row := range rows {
		firstSeenAt, parseErr := store.ParseTimestamp(row.FirstSeenAt)
		if parseErr != nil {
			return nil, fmt.Errorf("store: parse network thread participant first_seen_at: %w", parseErr)
		}
		lastSeenAt, parseErr := store.ParseTimestamp(row.LastSeenAt)
		if parseErr != nil {
			return nil, fmt.Errorf("store: parse network thread participant last_seen_at: %w", parseErr)
		}
		participants = append(participants, store.NetworkThreadParticipant{
			WorkspaceID: row.WorkspaceID, Channel: row.Channel, ThreadID: row.ThreadID,
			SessionID: row.SessionID, FirstMessageID: row.FirstMessageID,
			FirstSeenAt: firstSeenAt, LastSeenAt: lastSeenAt,
		})
	}
	return participants, nil
}

// UpdateNetworkThreadSessionTokenStats merges one delivered prompt into the thread/session aggregate.
func (g *NetworkRepo) UpdateNetworkThreadSessionTokenStats(
	ctx context.Context,
	update store.NetworkThreadSessionTokenStatsUpdate,
) error {
	if err := g.checkReady(ctx, "update network thread session token stats"); err != nil {
		return err
	}
	if err := update.Validate(); err != nil {
		return err
	}
	if update.DeliveredCount <= 0 {
		update.DeliveredCount = 1
	}
	if update.DeliveredAt.IsZero() {
		update.DeliveredAt = g.now()
	}
	if update.UpdatedAt.IsZero() {
		update.UpdatedAt = g.now()
	}

	if err := g.queries.UpsertNetworkThreadSessionTokenStats(
		ctx,
		sqlcgen.UpsertNetworkThreadSessionTokenStatsParams{
			WorkspaceID: strings.TrimSpace(update.WorkspaceID), Channel: strings.TrimSpace(update.Channel),
			ThreadID: strings.TrimSpace(update.ThreadID), SessionID: strings.TrimSpace(update.SessionID),
			DeliveredCount: update.DeliveredCount, PromptSizeBytes: update.PromptSizeBytes,
			EstimatedPromptTokens: update.EstimatedPromptTokens,
			FirstDeliveredAt:      store.FormatTimestamp(update.DeliveredAt),
			LastDeliveredAt:       store.FormatTimestamp(update.DeliveredAt),
			UpdatedAt:             store.FormatTimestamp(update.UpdatedAt),
		},
	); err != nil {
		return fmt.Errorf("store: upsert network thread session token stats: %w", err)
	}
	return nil
}

// ListNetworkThreadSessionTokenStats returns prompt-cost aggregates for one public thread.
func (g *NetworkRepo) ListNetworkThreadSessionTokenStats(
	ctx context.Context,
	query store.NetworkThreadSessionTokenStatsQuery,
) (stats []store.NetworkThreadSessionTokenStats, err error) {
	if err := g.checkReady(ctx, "list network thread session token stats"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// dynamic-sql: optional thread/session filters and the caller-provided limit change the statement shape.
	sqlQuery := `SELECT workspace_id, channel, thread_id, session_id, delivered_count, prompt_size_bytes,
		estimated_prompt_tokens, first_delivered_at, last_delivered_at, updated_at
		FROM network_thread_session_token_stats`
	where, args := store.BuildClauses(
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("channel", query.Channel),
		store.StringClause("thread_id", query.ThreadID),
		store.StringClause("session_id", query.SessionID),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY last_delivered_at DESC, session_id ASC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query network thread session token stats: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close network thread session token stats rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, closeErr)
				return
			}
			err = closeErr
		}
	}()

	stats = make([]store.NetworkThreadSessionTokenStats, 0)
	for rows.Next() {
		stat, scanErr := scanNetworkThreadSessionTokenStats(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate network thread session token stats: %w", err)
	}
	return stats, nil
}

func scanNetworkThreadSessionTokenStats(scanner rowScanner) (store.NetworkThreadSessionTokenStats, error) {
	var (
		stats     store.NetworkThreadSessionTokenStats
		firstRaw  string
		lastRaw   string
		updateRaw string
	)
	if err := scanner.Scan(
		&stats.WorkspaceID,
		&stats.Channel,
		&stats.ThreadID,
		&stats.SessionID,
		&stats.DeliveredCount,
		&stats.PromptSizeBytes,
		&stats.EstimatedPromptTokens,
		&firstRaw,
		&lastRaw,
		&updateRaw,
	); err != nil {
		return store.NetworkThreadSessionTokenStats{}, fmt.Errorf(
			"store: scan network thread session token stats: %w",
			err,
		)
	}
	firstDeliveredAt, err := store.ParseTimestamp(firstRaw)
	if err != nil {
		return store.NetworkThreadSessionTokenStats{}, fmt.Errorf(
			"store: parse network thread session token stats first_delivered_at: %w",
			err,
		)
	}
	lastDeliveredAt, err := store.ParseTimestamp(lastRaw)
	if err != nil {
		return store.NetworkThreadSessionTokenStats{}, fmt.Errorf(
			"store: parse network thread session token stats last_delivered_at: %w",
			err,
		)
	}
	updatedAt, err := store.ParseTimestamp(updateRaw)
	if err != nil {
		return store.NetworkThreadSessionTokenStats{}, fmt.Errorf(
			"store: parse network thread session token stats updated_at: %w",
			err,
		)
	}
	stats.FirstDeliveredAt = firstDeliveredAt
	stats.LastDeliveredAt = lastDeliveredAt
	stats.UpdatedAt = updatedAt
	return stats, nil
}
