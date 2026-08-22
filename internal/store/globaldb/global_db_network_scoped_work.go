package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
			return store.NetworkWorkEntry{}, fmt.Errorf("%w: network work: %w", store.ErrNetworkConversationNotFound, err)
		}
		return store.NetworkWorkEntry{}, fmt.Errorf("store: scan network work: %w", err)
	}
	entry.ThreadID = threadID.String
	entry.DirectID = directID.String
	entry.TargetSessionID = targetID.String
	var err error
	entry.OpenedAt, err = store.ParseTimestamp(openedRaw)
	if err != nil {
		return store.NetworkWorkEntry{}, fmt.Errorf("store: parse network work opened_at: %w", err)
	}
	entry.LastActivityAt, err = store.ParseTimestamp(activityRaw)
	if err != nil {
		return store.NetworkWorkEntry{}, fmt.Errorf("store: parse network work last_activity_at: %w", err)
	}
	if terminalRaw.Valid {
		terminalAt, parseErr := store.ParseTimestamp(terminalRaw.String)
		if parseErr != nil {
			return store.NetworkWorkEntry{}, fmt.Errorf("store: parse network work terminal_at: %w", parseErr)
		}
		entry.TerminalAt = &terminalAt
	}
	if err := entry.Validate(); err != nil {
		return store.NetworkWorkEntry{}, err
	}
	return entry, nil
}
