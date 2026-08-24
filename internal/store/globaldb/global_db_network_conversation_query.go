package globaldb

import (
	"context"
	"database/sql"

	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (g *NetworkRepo) lookupNetworkConversationMessageCursor(
	ctx context.Context,
	ref store.NetworkConversationRef,
	messageID string,
	query store.NetworkConversationMessageQuery,
) (networkMessageCursor, error) {
	cursorQuery := query
	cursorQuery.BeforeMessageID = ""
	cursorQuery.AfterMessageID = ""
	where, args := networkConversationMessageFilterClauses(ref, cursorQuery)
	where = append([]string{"network_timeline_log.message_id = ?"}, where...)
	args = append([]any{strings.TrimSpace(messageID)}, args...)

	var cursor networkMessageCursor
	// dynamic-sql: the cursor must reuse the container-specific and optional message filters from the page query.
	if err := g.db.QueryRowContext(
		ctx,
		store.AppendWhere(`SELECT network_timeline_log.sequence `+networkTimelineOwnerFromClause(), where),
		args...,
	).Scan(&cursor.Sequence); err != nil {
		return networkMessageCursor{}, fmt.Errorf(
			"%w: network conversation message %q not found: %w",
			store.ErrNetworkCursorInvalid,
			strings.TrimSpace(messageID),
			err,
		)
	}
	return cursor, nil
}

func networkConversationMessageSelect() string {
	return `SELECT
		owner_profile.id, owner_profile.name, owner_profile.color, COALESCE(owner_profile.icon, ''),
		COALESCE(owner_profile.emoji, ''), owner_profile.archived_at IS NOT NULL,
		network_timeline_log.sequence, network_timeline_log.message_id,
		network_timeline_log.session_id, network_timeline_log.workspace_id,
		network_timeline_log.channel, network_timeline_log.surface,
		network_timeline_log.thread_id, network_timeline_log.direct_id,
		network_timeline_log.direction, network_timeline_log.peer_from,
		network_timeline_log.peer_to, network_timeline_log.kind,
		network_timeline_log.work_id, network_timeline_log.reply_to,
		network_timeline_log.trace_id, network_timeline_log.causation_id,
		network_timeline_log.intent, network_timeline_log.text,
		network_timeline_log.preview_text, network_timeline_log.mentions_json,
		network_timeline_log.ext_json, network_timeline_log.body_json,
		COALESCE((
			SELECT MAX(audit.size)
			FROM network_audit_log AS audit
			WHERE audit.workspace_id = network_timeline_log.workspace_id
				AND audit.message_id = network_timeline_log.message_id
		), 0),
		network_timeline_log.timestamp
	` + networkTimelineOwnerFromClause()
}

func networkConversationMessageFilterClauses(
	ref store.NetworkConversationRef,
	query store.NetworkConversationMessageQuery,
) ([]string, []any) {
	where, scopeArgs := store.BuildClauses(store.ReadScopeClause("owner_profile.id", query.ReadScope))
	where = append(where,
		"network_timeline_log.workspace_id = ?",
		"network_timeline_log.channel = ?",
		"network_timeline_log.surface = ?",
	)
	args := []any{ref.WorkspaceID, ref.Channel, ref.Surface}
	args = append(scopeArgs, args...)
	if ref.Surface == store.NetworkSurfaceThread {
		where = append(where, "network_timeline_log.thread_id = ?")
		args = append(args, ref.ThreadID)
	} else {
		where = append(where, "network_timeline_log.direct_id = ?")
		args = append(args, ref.DirectID)
	}
	if strings.TrimSpace(query.Kind) != "" {
		where = append(where, "network_timeline_log.kind = ?")
		args = append(args, strings.TrimSpace(query.Kind))
	}
	if strings.TrimSpace(query.WorkID) != "" {
		where = append(where, "network_timeline_log.work_id = ?")
		args = append(args, strings.TrimSpace(query.WorkID))
	}
	return where, args
}

func normalizeNetworkConversationRef(ref store.NetworkConversationRef) store.NetworkConversationRef {
	return store.NetworkConversationRef{
		WorkspaceID: strings.TrimSpace(ref.WorkspaceID),
		Channel:     strings.TrimSpace(ref.Channel),
		Surface:     strings.TrimSpace(ref.Surface),
		ThreadID:    strings.TrimSpace(ref.ThreadID),
		DirectID:    strings.TrimSpace(ref.DirectID),
	}
}

func normalizeNetworkChannelRef(ref store.NetworkChannelRef) store.NetworkChannelRef {
	return store.NetworkChannelRef{
		WorkspaceID: strings.TrimSpace(ref.WorkspaceID),
		Channel:     strings.TrimSpace(ref.Channel),
	}
}

func scanNetworkThreadSummaries(rows *sql.Rows) ([]store.NetworkThreadSummary, error) {
	summaries := make([]store.NetworkThreadSummary, 0)
	for rows.Next() {
		summary, err := scanNetworkThreadSummary(rows)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate network thread rows: %w", err)
	}
	return summaries, nil
}

func scanNetworkThreadSummary(scanner rowScanner) (store.NetworkThreadSummary, error) {
	var (
		summary     store.NetworkThreadSummary
		title       sql.NullString
		sessionID   sql.NullString
		lastPreview sql.NullString
		openedRaw   string
		activityRaw string
	)
	if err := scanner.Scan(
		&summary.ProfileID,
		&summary.ProfileName,
		&summary.ProfileColor,
		&summary.ProfileIcon,
		&summary.ProfileEmoji,
		&summary.ProfileArchived,
		&summary.WorkspaceID,
		&summary.Channel,
		&summary.ThreadID,
		&summary.RootMessageID,
		&title,
		&summary.OpenedByPeerID,
		&sessionID,
		&openedRaw,
		&summary.OpenedSequence,
		&activityRaw,
		&summary.LastActivitySequence,
		&summary.MessageCount,
		&summary.ParticipantCount,
		&summary.OpenWorkCount,
		&summary.DeliveredCount,
		&summary.PromptSizeBytes,
		&summary.EstimatedPromptTokens,
		&lastPreview,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.NetworkThreadSummary{}, fmt.Errorf(
				"%w: network thread: %w",
				store.ErrNetworkConversationNotFound,
				err,
			)
		}
		return store.NetworkThreadSummary{}, fmt.Errorf("store: scan network thread summary: %w", err)
	}
	if value := store.NullString(title); value != nil {
		summary.Title = *value
	}
	if value := store.NullString(sessionID); value != nil {
		summary.OpenedSessionID = *value
	}
	if value := store.NullString(lastPreview); value != nil {
		summary.LastMessagePreview = *value
	}
	openedAt, err := store.ParseTimestamp(openedRaw)
	if err != nil {
		return store.NetworkThreadSummary{}, fmt.Errorf("store: parse network thread opened_at: %w", err)
	}
	activityAt, err := store.ParseTimestamp(activityRaw)
	if err != nil {
		return store.NetworkThreadSummary{}, fmt.Errorf("store: parse network thread last_activity_at: %w", err)
	}
	summary.OpenedAt = openedAt
	summary.LastActivityAt = activityAt
	if err := summary.Validate(); err != nil {
		return store.NetworkThreadSummary{}, err
	}
	return summary, nil
}

func scanNetworkDirectRoomSummaries(rows *sql.Rows) ([]store.NetworkDirectRoomSummary, error) {
	summaries := make([]store.NetworkDirectRoomSummary, 0)
	for rows.Next() {
		summary, err := scanNetworkDirectRoomSummary(rows)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate network direct room rows: %w", err)
	}
	return summaries, nil
}

func scanNetworkDirectRoomSummary(scanner rowScanner) (store.NetworkDirectRoomSummary, error) {
	var (
		summary     store.NetworkDirectRoomSummary
		lastPreview sql.NullString
		openedRaw   string
		activityRaw string
	)
	if err := scanner.Scan(
		&summary.ProfileID,
		&summary.ProfileName,
		&summary.ProfileColor,
		&summary.ProfileIcon,
		&summary.ProfileEmoji,
		&summary.ProfileArchived,
		&summary.WorkspaceID,
		&summary.Channel,
		&summary.DirectID,
		&summary.SessionA,
		&summary.SessionB,
		&openedRaw,
		&summary.OpenedSequence,
		&activityRaw,
		&summary.LastActivitySequence,
		&summary.MessageCount,
		&summary.OpenWorkCount,
		&lastPreview,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.NetworkDirectRoomSummary{}, fmt.Errorf(
				"%w: network direct room: %w",
				store.ErrNetworkConversationNotFound,
				err,
			)
		}
		return store.NetworkDirectRoomSummary{}, fmt.Errorf("store: scan network direct room summary: %w", err)
	}
	if value := store.NullString(lastPreview); value != nil {
		summary.LastMessagePreview = *value
	}
	openedAt, err := store.ParseTimestamp(openedRaw)
	if err != nil {
		return store.NetworkDirectRoomSummary{}, fmt.Errorf("store: parse network direct room opened_at: %w", err)
	}
	activityAt, err := store.ParseTimestamp(activityRaw)
	if err != nil {
		return store.NetworkDirectRoomSummary{}, fmt.Errorf(
			"store: parse network direct room last_activity_at: %w",
			err,
		)
	}
	summary.OpenedAt = openedAt
	summary.LastActivityAt = activityAt
	if err := summary.Validate(); err != nil {
		return store.NetworkDirectRoomSummary{}, err
	}
	return summary, nil
}

func normalizeRequiredNetworkField(value string, label string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("store: %s is required", label)
	}
	return trimmed, nil
}

func validateNetworkWorkID(workID string) error {
	if strings.TrimSpace(workID) == "" {
		return fmt.Errorf("store: network work_id is required")
	}
	if len(workID) > 128 || strings.ContainsAny(workID, `/\`) || containsControlCharacterForGlobalDB(workID) {
		return fmt.Errorf("store: invalid network work_id %q", workID)
	}
	return nil
}

func containsControlCharacterForGlobalDB(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
