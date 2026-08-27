package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const sessionArchivedConstraintMessage = "session is archived"

// SessionArchivedAt returns workspace-scoped archive metadata for one session.
func (g *SessionRepo) SessionArchivedAt(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*time.Time, error) {
	info, err := g.sessionForArchive(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	return cloneArchiveTime(info.ArchivedAtValue()), nil
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
	if workspaceID == "" || sessionID == "" {
		return store.SessionInfo{}, errors.New("store: session archive workspace and session ids are required")
	}

	current, err := g.sessionForArchive(ctx, workspaceID, sessionID)
	if err != nil {
		return store.SessionInfo{}, err
	}
	if archived && strings.TrimSpace(current.State) != globalDBSessionStateStopped {
		return store.SessionInfo{}, fmt.Errorf("%w: %s", store.ErrSessionArchiveRequiresStopped, sessionID)
	}
	if archived == (current.ArchivedAtValue() != nil) {
		return current, nil
	}

	now := g.now()
	archivedAt := any(nil)
	query := `UPDATE sessions SET archived_at = ?, updated_at = ?
		WHERE workspace_id = ? AND id = ?`
	if archived {
		archivedAt = store.FormatTimestamp(now)
		query += " AND state = 'stopped' AND archived_at IS NULL"
	} else {
		query += " AND archived_at IS NOT NULL"
	}
	result, err := g.db.ExecContext(
		ctx,
		query,
		archivedAt,
		store.FormatTimestamp(now),
		workspaceID,
		sessionID,
	)
	if err != nil {
		return store.SessionInfo{}, fmt.Errorf("store: update session archive state %q: %w", sessionID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return store.SessionInfo{}, fmt.Errorf("store: read session archive rows affected %q: %w", sessionID, err)
	}
	if affected == 0 {
		latest, readErr := g.sessionForArchive(ctx, workspaceID, sessionID)
		if readErr != nil {
			return store.SessionInfo{}, readErr
		}
		if archived && strings.TrimSpace(latest.State) != globalDBSessionStateStopped {
			return store.SessionInfo{}, fmt.Errorf("%w: %s", store.ErrSessionArchiveRequiresStopped, sessionID)
		}
		return latest, nil
	}
	return g.sessionForArchive(ctx, workspaceID, sessionID)
}

func mapSessionArchivedConstraint(sessionID string, err error) error {
	if err != nil && strings.Contains(err.Error(), sessionArchivedConstraintMessage) {
		return fmt.Errorf("%w: %s", store.ErrSessionArchived, strings.TrimSpace(sessionID))
	}
	return err
}

func (g *SessionRepo) sessionForArchive(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (store.SessionInfo, error) {
	if err := g.checkReady(ctx, "read session archive state"); err != nil {
		return store.SessionInfo{}, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	if workspaceID == "" || sessionID == "" {
		return store.SessionInfo{}, errors.New("store: session archive workspace and session ids are required")
	}
	info, err := scanSessionInfo(g.db.QueryRowContext(
		ctx,
		sessionInfoSelectQuery+" WHERE workspace_id = ? AND id = ?",
		workspaceID,
		sessionID,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.SessionInfo{}, fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
		}
		return store.SessionInfo{}, fmt.Errorf("store: read session archive state %q: %w", sessionID, err)
	}
	return info, nil
}

func cloneArchiveTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

var _ store.SessionArchiveStore = (*SessionRepo)(nil)
