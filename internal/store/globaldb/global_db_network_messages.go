package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

type networkMessageCursor struct {
	Sequence int64
}

type networkMessageNullableFields struct {
	sessionID    sql.NullString
	surface      sql.NullString
	threadID     sql.NullString
	directID     sql.NullString
	peerTo       sql.NullString
	workID       sql.NullString
	replyTo      sql.NullString
	traceID      sql.NullString
	causationID  sql.NullString
	intent       sql.NullString
	text         sql.NullString
	previewText  string
	mentionsRaw  string
	extRaw       string
	bodyRaw      string
	timestampRaw string
}

// WriteNetworkMessage stores one persisted network timeline envelope, ignoring duplicate message ids.
func (g *NetworkRepo) WriteNetworkMessage(ctx context.Context, entry store.NetworkMessageEntry) error {
	if err := g.checkReady(ctx, "write network message"); err != nil {
		return err
	}
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("store: validate network message entry: %w", err)
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = g.now()
	}

	return g.withNetworkImmediateTransaction(
		ctx,
		"write network message",
		func(exec networkSQLExecutor) error {
			inserted, sequence, err := insertNetworkTimelineMessageWithExecutor(ctx, exec, entry)
			if err != nil || !inserted {
				return err
			}
			entry.Sequence = sequence
			projection, err := deriveNetworkTimelineWorkProjection(ctx, exec, entry)
			if err != nil {
				return err
			}
			return persistNetworkTimelineWorkProjection(ctx, exec, entry, projection)
		},
	)
}

// ListNetworkMessages returns persisted network timeline rows filtered by the supplied options.
func (g *NetworkRepo) ListNetworkMessages(
	ctx context.Context,
	query store.NetworkMessageQuery,
) (entries []store.NetworkMessageEntry, err error) {
	if err := g.checkReady(ctx, "list network messages"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("store: validate network message query: %w", err)
	}

	sqlQuery, args, reverseResults, err := g.buildNetworkMessageListQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query network messages: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close network messages rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, closeErr)
				return
			}
			err = closeErr
		}
	}()

	entries, err = loadNetworkMessageEntries(rows)
	if err != nil {
		return nil, err
	}
	if reverseResults {
		reverseNetworkMessages(entries)
	}

	return entries, nil
}

func (g *NetworkRepo) buildNetworkMessageListQuery(
	ctx context.Context,
	query store.NetworkMessageQuery,
) (string, []any, bool, error) {
	// dynamic-sql: optional envelope filters, cursor direction, sort order, and limit change the statement shape.
	sqlQuery := `SELECT
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

	where, args := networkMessageFilterClauses(query, true)

	reverseResults := false
	switch {
	case strings.TrimSpace(query.BeforeMessageID) != "":
		cursor, cursorErr := g.lookupNetworkMessageCursor(ctx, query.BeforeMessageID, query)
		if cursorErr != nil {
			return "", nil, false, cursorErr
		}
		where = append(where, "sequence < ?")
		args = append(args, cursor.Sequence)
		reverseResults = true
	case strings.TrimSpace(query.AfterMessageID) != "":
		cursor, cursorErr := g.lookupNetworkMessageCursor(ctx, query.AfterMessageID, query)
		if cursorErr != nil {
			return "", nil, false, cursorErr
		}
		where = append(where, "sequence > ?")
		args = append(args, cursor.Sequence)
	default:
		reverseResults = query.Limit > 0
	}

	sqlQuery = store.AppendWhere(sqlQuery, where)
	if reverseResults {
		sqlQuery += " ORDER BY sequence DESC"
	} else {
		sqlQuery += " ORDER BY sequence ASC"
	}
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)
	return sqlQuery, args, reverseResults, nil
}

func networkMessageFilterClauses(query store.NetworkMessageQuery, includeMessageID bool) ([]string, []any) {
	messageID := ""
	if includeMessageID {
		messageID = query.MessageID
	}
	where, args := store.BuildClauses(
		store.ReadScopeClause("owner_profile.id", query.ReadScope),
		store.StringClause("network_timeline_log.workspace_id", query.WorkspaceID),
		store.StringClause("network_timeline_log.session_id", query.SessionID),
		store.StringClause("network_timeline_log.channel", query.Channel),
		store.StringClause("network_timeline_log.peer_from", query.PeerFrom),
		store.StringClause("network_timeline_log.peer_to", query.PeerTo),
		store.StringClause("network_timeline_log.kind", query.Kind),
		store.StringClause("network_timeline_log.direction", query.Direction),
		store.StringClause("network_timeline_log.message_id", messageID),
		store.TimeClause("network_timeline_log.timestamp", ">=", query.Since),
	)
	if peerID := strings.TrimSpace(query.PeerID); peerID != "" {
		where = append(where, "(network_timeline_log.peer_from = ? OR network_timeline_log.peer_to = ?)")
		args = append(args, peerID, peerID)
	}
	directed := `(COALESCE(TRIM(peer_to), '') <> ''
		OR COALESCE(TRIM(direct_id), '') <> ''
		OR COALESCE(TRIM(surface), '') = 'direct')`
	public := `(COALESCE(TRIM(peer_to), '') = ''
		AND COALESCE(TRIM(direct_id), '') = ''
		AND COALESCE(TRIM(surface), '') <> 'direct')`
	switch {
	case query.PublicOnly && query.IncludePresence:
		where = append(where, "(kind = 'greet' OR "+public+")")
	case query.PublicOnly:
		where = append(where, "kind <> 'greet' AND "+public)
	case query.DirectedOnly && query.IncludePresence:
		where = append(where, "(kind = 'greet' OR "+directed+")")
	case query.DirectedOnly:
		where = append(where, "kind <> 'greet' AND "+directed)
	}
	return where, args
}

func loadNetworkMessageEntries(rows *sql.Rows) ([]store.NetworkMessageEntry, error) {
	entries := make([]store.NetworkMessageEntry, 0)
	for rows.Next() {
		entry, err := scanNetworkMessage(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate network messages: %w", err)
	}
	return entries, nil
}

func reverseNetworkMessages(entries []store.NetworkMessageEntry) {
	for left, right := 0, len(entries)-1; left < right; left, right = left+1, right-1 {
		entries[left], entries[right] = entries[right], entries[left]
	}
}

func (g *NetworkRepo) lookupNetworkMessageCursor(
	ctx context.Context,
	messageID string,
	query store.NetworkMessageQuery,
) (networkMessageCursor, error) {
	trimmed := strings.TrimSpace(messageID)
	if trimmed == "" {
		return networkMessageCursor{}, nil
	}

	where, args := networkMessageFilterClauses(query, false)
	where = append([]string{"network_timeline_log.message_id = ?"}, where...)
	args = append([]any{trimmed}, args...)

	var cursor networkMessageCursor
	// dynamic-sql: the cursor must reuse every optional list filter to prove membership in the requested page.
	err := g.db.QueryRowContext(
		ctx,
		store.AppendWhere(`SELECT network_timeline_log.sequence `+networkTimelineOwnerFromClause(), where),
		args...,
	).Scan(&cursor.Sequence)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return networkMessageCursor{}, fmt.Errorf("store: network message cursor not found: %w", err)
		}
		return networkMessageCursor{}, fmt.Errorf("store: lookup network message cursor: %w", err)
	}
	return cursor, nil
}

func scanNetworkMessage(scanner rowScanner) (store.NetworkMessageEntry, error) {
	var (
		entry    store.NetworkMessageEntry
		nullable networkMessageNullableFields
	)
	if err := scanner.Scan(
		&entry.ProfileID,
		&entry.ProfileName,
		&entry.ProfileColor,
		&entry.ProfileIcon,
		&entry.ProfileEmoji,
		&entry.ProfileArchived,
		&entry.Sequence,
		&entry.MessageID,
		&nullable.sessionID,
		&entry.WorkspaceID,
		&entry.Channel,
		&nullable.surface,
		&nullable.threadID,
		&nullable.directID,
		&entry.Direction,
		&entry.PeerFrom,
		&nullable.peerTo,
		&entry.Kind,
		&nullable.workID,
		&nullable.replyTo,
		&nullable.traceID,
		&nullable.causationID,
		&nullable.intent,
		&nullable.text,
		&nullable.previewText,
		&nullable.mentionsRaw,
		&nullable.extRaw,
		&nullable.bodyRaw,
		&entry.SizeBytes,
		&nullable.timestampRaw,
	); err != nil {
		return store.NetworkMessageEntry{}, fmt.Errorf("store: scan network message: %w", err)
	}

	applyNetworkMessageNullableFields(&entry, nullable)

	timestamp, err := store.ParseTimestamp(nullable.timestampRaw)
	if err != nil {
		return store.NetworkMessageEntry{}, fmt.Errorf("store: parse network message timestamp: %w", err)
	}
	entry.Timestamp = timestamp
	return entry, nil
}

func networkTimelineOwnerFromClause() string {
	return `FROM network_timeline_log
	LEFT JOIN network_threads AS owner_thread
		ON owner_thread.workspace_id = network_timeline_log.workspace_id
		AND owner_thread.channel = network_timeline_log.channel
		AND owner_thread.thread_id = network_timeline_log.thread_id
	LEFT JOIN network_direct_rooms AS owner_direct
		ON owner_direct.workspace_id = network_timeline_log.workspace_id
		AND owner_direct.channel = network_timeline_log.channel
		AND owner_direct.direct_id = network_timeline_log.direct_id
	JOIN network_channels AS owner_channel
		ON owner_channel.workspace_id = network_timeline_log.workspace_id
		AND owner_channel.channel = network_timeline_log.channel
	JOIN profiles AS owner_profile
		ON owner_profile.id = COALESCE(owner_thread.profile_id, owner_direct.profile_id, owner_channel.profile_id)`
}

func applyNetworkMessageNullableFields(entry *store.NetworkMessageEntry, nullable networkMessageNullableFields) {
	if value := store.NullString(nullable.sessionID); value != nil {
		entry.SessionID = *value
	}
	if value := store.NullString(nullable.surface); value != nil {
		entry.Surface = *value
	}
	if value := store.NullString(nullable.threadID); value != nil {
		entry.ThreadID = *value
	}
	if value := store.NullString(nullable.directID); value != nil {
		entry.DirectID = *value
	}
	if value := store.NullString(nullable.peerTo); value != nil {
		entry.PeerTo = *value
	}
	if value := store.NullString(nullable.workID); value != nil {
		entry.WorkID = *value
	}
	if value := store.NullString(nullable.replyTo); value != nil {
		entry.ReplyTo = *value
	}
	if value := store.NullString(nullable.traceID); value != nil {
		entry.TraceID = *value
	}
	if value := store.NullString(nullable.causationID); value != nil {
		entry.CausationID = *value
	}
	if value := store.NullString(nullable.intent); value != nil {
		entry.Intent = *value
	}
	if value := store.NullString(nullable.text); value != nil {
		entry.Text = *value
	}
	entry.PreviewText = strings.TrimSpace(nullable.previewText)
	entry.Mentions = networkMentionsFromJSONString(nullable.mentionsRaw)
	entry.ExtJSON = []byte(networkMessageExtJSONString([]byte(nullable.extRaw)))
	entry.Body = []byte(nullable.bodyRaw)
}

func networkMessageExtJSONString(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "{}"
	}
	return trimmed
}

func networkMentionsJSONString(values []string) string {
	normalized, err := store.NormalizeNetworkPeerIDs(values, "mentions")
	if err != nil || len(normalized) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func networkMentionsFromJSONString(raw string) []string {
	var values []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &values); err != nil {
		return nil
	}
	normalized, err := store.NormalizeNetworkPeerIDs(values, "mentions")
	if err != nil {
		return nil
	}
	return normalized
}
