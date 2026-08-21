package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func getScopedNetworkWork(
	ctx context.Context,
	exec networkSQLExecutor,
	readScope store.ReadScope,
	workspaceID string,
	workID string,
) (store.NetworkWorkEntry, error) {
	statement := `SELECT nw.work_id, nw.profile_id, p.name, p.color, COALESCE(p.icon, ''),
		COALESCE(p.emoji, ''), p.archived_at IS NOT NULL, nw.workspace_id, nw.channel,
		nw.surface, nw.thread_id, nw.direct_id, nw.opened_by_session_id, nw.target_session_id,
		nw.state, nw.opened_at, nw.last_activity_at, nw.terminal_at
		FROM network_work AS nw
		JOIN profiles AS p ON p.id = nw.profile_id`
	where, args := store.BuildClauses(
		store.ReadScopeClause("nw.profile_id", readScope),
		store.StringClause("nw.workspace_id", workspaceID),
		store.StringClause("nw.work_id", workID),
	)
	return scanScopedNetworkWork(exec.QueryRowContext(ctx, store.AppendWhere(statement, where), args...))
}

func scanScopedNetworkWork(scanner rowScanner) (store.NetworkWorkEntry, error) {
	var (
		entry       store.NetworkWorkEntry
		threadID    sql.NullString
		directID    sql.NullString
		targetID    sql.NullString
		terminalRaw sql.NullString
		openedRaw   string
		activityRaw string
	)
	if err := scanner.Scan(
		&entry.WorkID, &entry.ProfileID, &entry.ProfileName, &entry.ProfileColor,
		&entry.ProfileIcon, &entry.ProfileEmoji, &entry.ProfileArchived, &entry.WorkspaceID,
		&entry.Channel, &entry.Surface, &threadID, &directID, &entry.OpenedBySessionID,
		&targetID, &entry.State, &openedRaw, &activityRaw, &terminalRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.NetworkWorkEntry{}, fmt.Errorf(
				"%w: network work: %w",
				store.ErrNetworkConversationNotFound,
				err,
			)
		}
		return store.NetworkWorkEntry{}, fmt.Errorf("store: scan network work: %w", err)
	}
	entry.ThreadID = threadID.String
	entry.DirectID = directID.String
	entry.TargetSessionID = targetID.String
	var err error
	entry.OpenedAt, err = parseNetworkWorkTimestamp("opened_at", openedRaw)
	if err != nil {
		return store.NetworkWorkEntry{}, err
	}
	entry.LastActivityAt, err = parseNetworkWorkTimestamp("last_activity_at", activityRaw)
	if err != nil {
		return store.NetworkWorkEntry{}, err
	}
	if terminalRaw.Valid {
		terminalAt, parseErr := parseNetworkWorkTimestamp("terminal_at", terminalRaw.String)
		if parseErr != nil {
			return store.NetworkWorkEntry{}, parseErr
		}
		entry.TerminalAt = &terminalAt
	}
	if err := entry.Validate(); err != nil {
		return store.NetworkWorkEntry{}, err
	}
	return entry, nil
}

func parseNetworkWorkTimestamp(field string, value string) (time.Time, error) {
	parsed, err := store.ParseTimestamp(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("store: parse network work %s: %w", field, err)
	}
	return parsed, nil
}
