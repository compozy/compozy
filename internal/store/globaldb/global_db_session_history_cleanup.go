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

// sessionHistoryCleanupSelect reads every archived session in scope, every
// unarchived session hanging from an archived parent, and every unarchived
// parent of an archived session, so the planner sees the live sessions an
// archived one hangs from and never mistakes them for gone. Daemon-internal
// auto-title sessions are never tree members, so they are left out.
const sessionHistoryCleanupSelect = `SELECT id, workspace_id, COALESCE(name, ''),
	COALESCE(parent_session_id, ''), COALESCE(spawn_role, ''), lineage_kind, archived_at,
	EXISTS(SELECT 1 FROM agent_instances a WHERE a.main_session_id = sessions.id AND a.archived_at IS NULL)
FROM sessions`

const sessionHistoryCleanupMembership = `(archived_at IS NOT NULL OR EXISTS(
	SELECT 1 FROM sessions p
	WHERE p.id = sessions.parent_session_id AND p.workspace_id = sessions.workspace_id
		AND p.archived_at IS NOT NULL) OR EXISTS(
	SELECT 1 FROM sessions c
	WHERE c.parent_session_id = sessions.id AND c.workspace_id = sessions.workspace_id
		AND c.archived_at IS NOT NULL))`

const sessionHistoryCleanupNotInternal = `LOWER(TRIM(COALESCE(spawn_role, ''))) <> 'auto-title'`

// ListSessionHistoryCleanupRows reads the archived history one cleanup plans from.
func (g *SessionRepo) ListSessionHistoryCleanupRows(
	ctx context.Context,
	query store.SessionHistoryCleanupQuery,
) (rows []store.SessionHistoryCleanupRow, err error) {
	if err := g.checkReady(ctx, "list session history cleanup rows"); err != nil {
		return nil, err
	}
	profileID := strings.TrimSpace(query.ProfileID)
	if profileID == "" {
		return nil, errors.New("store: session history cleanup requires a profile id")
	}
	workspaceID := strings.TrimSpace(query.WorkspaceID)
	if (workspaceID != "") == query.AllWorkspaces {
		return nil, errors.New("store: session history cleanup requires one workspace or all workspaces")
	}
	where, args := store.BuildClauses(
		store.StringClause("profile_id", profileID),
		store.SessionWorkspaceClause(workspaceID),
	)
	where = append(where, sessionHistoryCleanupMembership, sessionHistoryCleanupNotInternal, sessionSideVisible)
	sqlQuery := store.AppendWhere(sessionHistoryCleanupSelect, where)

	// dynamic-sql: the workspace scope is either one workspace, the user selector, or none.
	result, err := g.readDB.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query session history cleanup rows: %w", err)
	}
	defer func() {
		if closeErr := result.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close session history cleanup rows: %w", closeErr))
		}
	}()
	rows = make([]store.SessionHistoryCleanupRow, 0)
	for result.Next() {
		row, scanErr := scanSessionHistoryCleanupRow(result)
		if scanErr != nil {
			return nil, scanErr
		}
		rows = append(rows, row)
	}
	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate session history cleanup rows: %w", err)
	}
	return rows, nil
}

func scanSessionHistoryCleanupRow(scanner interface{ Scan(...any) error }) (store.SessionHistoryCleanupRow, error) {
	var (
		row         store.SessionHistoryCleanupRow
		lineageKind string
		archivedAt  sql.NullString
	)
	if err := scanner.Scan(
		&row.ID,
		&row.WorkspaceID,
		&row.Name,
		&row.ParentSessionID,
		&row.SpawnRole,
		&lineageKind,
		&archivedAt,
		&row.InstanceMain,
	); err != nil {
		return store.SessionHistoryCleanupRow{}, fmt.Errorf("store: scan session history cleanup row: %w", err)
	}
	row.LineageKind = store.LineageKind(lineageKind)
	if archivedAt.Valid {
		parsed, err := store.ParseTimestamp(archivedAt.String)
		if err != nil {
			return store.SessionHistoryCleanupRow{}, fmt.Errorf(
				"store: parse archived_at of session %q: %w", row.ID, err,
			)
		}
		row.ArchivedAt = &parsed
	}
	return row, nil
}

// DeleteArchivedSession deletes one session only while it is still archived and
// still owned by the expected profile and workspace, in the same transaction
// that tombstones its derive receipts.
func (g *SessionRepo) DeleteArchivedSession(ctx context.Context, target store.ArchivedSessionDelete) error {
	if err := g.checkReady(ctx, "delete archived session"); err != nil {
		return err
	}
	sessionID := strings.TrimSpace(target.SessionID)
	profileID := strings.TrimSpace(target.ProfileID)
	if sessionID == "" || profileID == "" {
		return errors.New("store: archived session delete requires a session id and profile id")
	}
	deletedAt := g.now()
	return g.withImmediateTransaction(ctx, "delete archived session", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		main, err := queries.IsInstanceMainSession(ctx, sql.NullString{String: sessionID, Valid: true})
		if err != nil {
			return err
		}
		if main {
			return store.ErrSessionIsInstanceMain
		}
		// The delete clears child_session_id through ON DELETE SET NULL.
		if err := queries.StopDeletedSubagentChildActivity(ctx, sqlcgen.StopDeletedSubagentChildActivityParams{
			ChildID: sql.NullString{String: sessionID, Valid: true}, Now: store.FormatTimestamp(deletedAt),
		}); err != nil {
			return fmt.Errorf("store: stop deleted subagent child activity %q: %w", sessionID, err)
		}
		affected, err := queries.DeleteArchivedSession(ctx, sqlcgen.DeleteArchivedSessionParams{
			ID:          sessionID,
			WorkspaceID: strings.TrimSpace(target.WorkspaceID),
			ProfileID:   profileID,
		})
		if err != nil {
			return fmt.Errorf("store: delete archived session %q: %w", sessionID, err)
		}
		if affected == 0 {
			return archivedSessionDeleteRefusal(ctx, queries, target)
		}
		return markDerivationChildDeleted(ctx, queries, sessionID, deletedAt)
	})
}

// archivedSessionDeleteRefusal tells a session that is gone from one that is no
// longer archived or no longer in the expected scope; a session moved to another
// workspace still exists, so it is refused rather than reported gone.
func archivedSessionDeleteRefusal(
	ctx context.Context,
	queries *sqlcgen.Queries,
	target store.ArchivedSessionDelete,
) error {
	_, err := queries.GetSessionWorkspaceID(ctx, strings.TrimSpace(target.SessionID))
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("store: delete archived session %q: %w", target.SessionID, store.ErrSessionNotFound)
	}
	if err != nil {
		return fmt.Errorf("store: read archived session %q: %w", target.SessionID, err)
	}
	return fmt.Errorf("store: delete archived session %q: %w", target.SessionID, store.ErrSessionNotArchived)
}

var _ store.SessionHistoryCleanupStore = (*SessionRepo)(nil)
