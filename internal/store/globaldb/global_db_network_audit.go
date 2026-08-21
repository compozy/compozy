package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// WriteNetworkAudit stores one network audit row.
func (g *NetworkRepo) WriteNetworkAudit(ctx context.Context, entry store.NetworkAuditEntry) error {
	if err := g.checkReady(ctx, "write network audit"); err != nil {
		return err
	}
	if err := entry.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(entry.ID) == "" {
		generatedID, err := store.NewID("naud")
		if err != nil {
			return fmt.Errorf("store: generate network audit id: %w", err)
		}
		entry.ID = generatedID
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = g.now()
	}

	return insertNetworkAuditWithExecutor(ctx, g.db, entry)
}

// ListNetworkAudit returns network audit rows filtered by the supplied options.
func (g *NetworkRepo) ListNetworkAudit(
	ctx context.Context,
	query store.NetworkAuditQuery,
) (entries []store.NetworkAuditEntry, err error) {
	if err := g.checkReady(ctx, "list network audit"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	// dynamic-sql: the optional audit dimensions, lower time bound, and caller limit change the statement shape.
	sqlQuery := `SELECT
		audit.profile_id, COALESCE(owner_profile.name, ''),
		COALESCE(owner_profile.color, ''), COALESCE(owner_profile.icon, ''),
		COALESCE(owner_profile.emoji, ''), COALESCE(owner_profile.archived_at IS NOT NULL, 0),
		audit.id, audit.session_id, audit.workspace_id, audit.direction, audit.kind, audit.channel,
		audit.surface, audit.thread_id, audit.direct_id, audit.work_id, audit.peer_from, audit.peer_to,
		audit.message_id, audit.reason, audit.size, audit.timestamp
	FROM network_audit_log AS audit
	JOIN profiles AS owner_profile ON owner_profile.id = audit.profile_id`
	where, args := store.BuildClauses(
		store.ReadScopeClause("audit.profile_id", query.ReadScope),
		store.StringClause("audit.workspace_id", query.WorkspaceID),
		store.StringClause("audit.session_id", query.SessionID),
		store.StringClause("audit.direction", query.Direction),
		store.StringClause("audit.kind", query.Kind),
		store.StringClause("audit.channel", query.Channel),
		store.StringClause("audit.surface", query.Surface),
		store.StringClause("audit.thread_id", query.ThreadID),
		store.StringClause("audit.direct_id", query.DirectID),
		store.StringClause("audit.work_id", query.WorkID),
		store.StringClause("audit.message_id", query.MessageID),
		store.TimeClause("audit.timestamp", ">=", query.Since),
	)
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += " ORDER BY audit.timestamp ASC, audit.id ASC"
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query network audit: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close network audit rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, closeErr)
				return
			}
			err = closeErr
		}
	}()

	entries = make([]store.NetworkAuditEntry, 0)
	for rows.Next() {
		entry, scanErr := scanNetworkAudit(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate network audit: %w", err)
	}

	return entries, nil
}

func scanNetworkAudit(scanner rowScanner) (store.NetworkAuditEntry, error) {
	var (
		entry        store.NetworkAuditEntry
		surface      sql.NullString
		threadID     sql.NullString
		directID     sql.NullString
		workID       sql.NullString
		peerTo       sql.NullString
		reason       sql.NullString
		timestampRaw string
	)
	if err := scanner.Scan(
		&entry.ProfileID,
		&entry.ProfileName,
		&entry.ProfileColor,
		&entry.ProfileIcon,
		&entry.ProfileEmoji,
		&entry.ProfileArchived,
		&entry.ID,
		&entry.SessionID,
		&entry.WorkspaceID,
		&entry.Direction,
		&entry.Kind,
		&entry.Channel,
		&surface,
		&threadID,
		&directID,
		&workID,
		&entry.PeerFrom,
		&peerTo,
		&entry.MessageID,
		&reason,
		&entry.Size,
		&timestampRaw,
	); err != nil {
		return store.NetworkAuditEntry{}, fmt.Errorf("store: scan network audit: %w", err)
	}

	if value := store.NullString(surface); value != nil {
		entry.Surface = *value
	}
	if value := store.NullString(threadID); value != nil {
		entry.ThreadID = *value
	}
	if value := store.NullString(directID); value != nil {
		entry.DirectID = *value
	}
	if value := store.NullString(workID); value != nil {
		entry.WorkID = *value
	}
	if value := store.NullString(peerTo); value != nil {
		entry.PeerTo = *value
	}
	if value := store.NullString(reason); value != nil {
		entry.Reason = *value
	}

	timestamp, err := store.ParseTimestamp(timestampRaw)
	if err != nil {
		return store.NetworkAuditEntry{}, fmt.Errorf("store: parse network audit timestamp: %w", err)
	}
	entry.Timestamp = timestamp
	return entry, nil
}

func insertNetworkAuditWithExecutor(ctx context.Context, exec networkSQLExecutor, entry store.NetworkAuditEntry) error {
	if err := entry.Validate(); err != nil {
		return fmt.Errorf("store: validate network audit entry: %w", err)
	}
	if err := sqlcgen.New(exec).InsertNetworkAudit(ctx, sqlcgen.InsertNetworkAuditParams{
		ID: entry.ID, ProfileID: entry.ProfileID, SessionID: entry.SessionID, WorkspaceID: entry.WorkspaceID,
		Direction: entry.Direction, Kind: entry.Kind, Channel: entry.Channel,
		Surface: nullableNetworkString(entry.Surface), ThreadID: nullableNetworkString(entry.ThreadID),
		DirectID: nullableNetworkString(entry.DirectID), WorkID: nullableNetworkString(entry.WorkID),
		PeerFrom: entry.PeerFrom, PeerTo: nullableNetworkString(entry.PeerTo), MessageID: entry.MessageID,
		Reason: nullableNetworkString(entry.Reason), Size: int64(entry.Size),
		Timestamp: store.FormatTimestamp(entry.Timestamp),
	}); err != nil {
		return fmt.Errorf("store: insert network audit entry: %w", err)
	}
	return nil
}

func nullableNetworkString(value string) sql.NullString {
	return store.SQLNullString(value)
}
