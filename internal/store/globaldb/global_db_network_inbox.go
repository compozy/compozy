package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
)

var (
	_ store.NetworkInboxStore     = (*NetworkRepo)(nil)
	_ store.NetworkCausationStore = (*NetworkRepo)(nil)
)

// ListNetworkInbox returns immutable delivered decisions in acceptance order.
func (g *NetworkRepo) ListNetworkInbox(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	channel string,
	limit int,
) (entries []store.NetworkMessageEntry, err error) {
	if err := g.checkReady(ctx, "list network inbox"); err != nil {
		return nil, err
	}
	targetSessionID := strings.TrimSpace(sessionID)
	targetWorkspaceID := strings.TrimSpace(workspaceID)
	if targetWorkspaceID == "" {
		return nil, fmt.Errorf("store: network inbox workspace_id is required")
	}
	if targetSessionID == "" {
		return nil, fmt.Errorf("store: network inbox session_id is required")
	}
	if limit <= 0 {
		limit = store.NetworkMessageDefaultLimit
	}
	if limit > store.NetworkMessageMaxLimit {
		limit = store.NetworkMessageMaxLimit
	}
	query, args := networkInboxQuery(targetWorkspaceID, targetSessionID, channel, limit)
	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query network inbox: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close network inbox rows: %w", closeErr)
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
	reverseNetworkMessages(entries)
	return entries, nil
}

func networkInboxQuery(workspaceID string, sessionID string, channel string, limit int) (string, []any) {
	// dynamic-sql: only the optional, internally selected channel predicate changes the statement shape.
	query := `SELECT
		n.sequence,
		n.message_id,
		n.session_id,
		n.workspace_id,
		n.channel,
		n.surface,
		n.thread_id,
		n.direct_id,
		n.direction,
		n.peer_from,
		n.peer_to,
		n.kind,
		n.work_id,
		n.reply_to,
		n.trace_id,
		n.causation_id,
		n.intent,
		n.text,
		n.preview_text,
		n.mentions_json,
		n.ext_json,
		n.body_json,
		COALESCE((
			SELECT MAX(audit.size)
			FROM network_audit_log AS audit
			WHERE audit.workspace_id = n.workspace_id
				AND audit.message_id = n.message_id
		), 0),
		n.timestamp
	FROM network_message_dispositions AS d
	JOIN network_timeline_log AS n
		ON n.workspace_id = d.workspace_id
		AND n.message_id = d.message_id
	JOIN sessions AS s
		ON s.id = d.recipient_session_id
		AND s.workspace_id = d.workspace_id
	WHERE d.recipient_session_id = ?
		AND d.workspace_id = ?
		AND d.decision = ?`
	args := []any{sessionID, workspaceID, store.NetworkDispositionDeliver}
	if targetChannel := strings.TrimSpace(channel); targetChannel != "" {
		query += " AND n.channel = ?"
		args = append(args, targetChannel)
	}
	query += " ORDER BY d.acceptance_seq DESC, n.sequence DESC LIMIT ?"
	args = append(args, limit)
	return query, args
}

// ResolveNetworkCausation returns the next wake depth for one durable source.
func (g *NetworkRepo) ResolveNetworkCausation(
	ctx context.Context,
	workspaceID string,
	causationID string,
) (string, int, bool, error) {
	if err := g.checkReady(ctx, "resolve network causation"); err != nil {
		return "", 0, false, err
	}
	targetWorkspaceID := strings.TrimSpace(workspaceID)
	if targetWorkspaceID == "" {
		return "", 0, false, fmt.Errorf("store: network causation workspace_id is required")
	}
	targetCausationID := strings.TrimSpace(causationID)
	if targetCausationID == "" {
		return "", 0, false, fmt.Errorf("store: network causation message_id is required")
	}
	var rootID string
	var depth int
	err := g.db.QueryRowContext(
		ctx,
		`SELECT wake.root_id, wake.depth + 1
		 FROM network_wake_sources AS source
		 JOIN network_live_wakes AS wake
			ON wake.workspace_id = source.workspace_id
			AND wake.wake_id = source.wake_id
		 WHERE source.workspace_id = ? AND source.envelope_id = ?`,
		targetWorkspaceID,
		targetCausationID,
	).Scan(&rootID, &depth)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, false, nil
	}
	if err != nil {
		return "", 0, false, fmt.Errorf("store: query network causation: %w", err)
	}
	return strings.TrimSpace(rootID), depth, true, nil
}
