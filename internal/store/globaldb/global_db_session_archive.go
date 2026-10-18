package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const sessionArchivedConstraintMessage = "session is archived"

// SessionArchivedAt returns workspace-scoped archive metadata for one session.
func (g *SessionRepo) SessionArchivedAt(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*time.Time, error) {
	if err := g.checkReady(ctx, "read session archive state"); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("store: session archive session id is required")
	}
	archivedAt, err := g.queries.GetSessionArchivedAt(ctx, sqlcgen.GetSessionArchivedAtParams{
		WorkspaceID: workspaceID, ID: sessionID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("store: read session archive state %q: %w", sessionID, err)
	}
	value, err := parseOptionalSessionInputTimestamp(archivedAt)
	if err != nil {
		return nil, fmt.Errorf("store: parse session archive state %q: %w", sessionID, err)
	}
	return value, nil
}

// SetSessionArchived archives or restores one workspace-owned session.
func (g *SessionRepo) SetSessionArchived(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	archived bool,
) (store.SessionInfo, error) {
	if err := g.checkReady(ctx, "set session archive state"); err != nil {
		return store.SessionInfo{}, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return store.SessionInfo{}, errors.New("store: session archive session id is required")
	}

	var result store.SessionInfo
	err := g.withImmediateTransaction(ctx, "archive session family", func(exec globalSQLExecutor) error {
		current, e := scanSessionInfo(
			exec.QueryRowContext(
				ctx,
				sessionInfoSelectQuery+" WHERE workspace_id = ? AND id = ?",
				workspaceID,
				sessionID,
			),
		)
		if errors.Is(e, sql.ErrNoRows) {
			return store.ErrSessionNotFound
		}
		if e != nil {
			return e
		}
		if current.Lineage != nil && current.Lineage.SpawnRole == store.SubagentSpawnRole {
			return store.ErrSubagentArchiveFollowsParent
		}
		q := sqlcgen.New(exec)
		family, e := q.ListSubagentArchiveFamily(
			ctx,
			sqlcgen.ListSubagentArchiveFamilyParams{WorkspaceID: workspaceID, SessionID: sessionID},
		)
		if e != nil {
			return e
		}
		ids := make([]string, 0, len(family))
		for _, member := range family {
			if archived && member.State != globalDBSessionStateStopped {
				return fmt.Errorf("%w: %s", store.ErrSessionArchiveRequiresStopped, member.ID)
			}
			ids = append(ids, member.ID)
		}
		encoded, e := subagentIDsJSON(ids)
		if e != nil {
			return e
		}
		now := store.FormatTimestamp(g.now())
		at := sql.NullString{}
		if archived {
			at = sql.NullString{String: now, Valid: true}
		}
		if e = q.SetSubagentFamilyArchived(
			ctx,
			sqlcgen.SetSubagentFamilyArchivedParams{WorkspaceID: workspaceID, Ids: encoded, ArchivedAt: at, Now: now},
		); e != nil {
			return mapSessionArchivedConstraint(sessionID, e)
		}
		result, e = scanSessionInfo(
			exec.QueryRowContext(
				ctx,
				sessionInfoSelectQuery+" WHERE workspace_id = ? AND id = ?",
				workspaceID,
				sessionID,
			),
		)
		return e
	})
	return result, err
}

func mapSessionArchivedConstraint(sessionID string, err error) error {
	if err != nil && strings.Contains(err.Error(), sessionArchivedConstraintMessage) {
		return fmt.Errorf("%w: %s", store.ErrSessionArchived, strings.TrimSpace(sessionID))
	}
	return err
}

var _ store.SessionArchiveStore = (*SessionRepo)(nil)
