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

const sessionLaterConstraintMessage = "session is in later"

// SessionLaterState reads the Later state and back-from-Later mark of one session.
func (g *SessionRepo) SessionLaterState(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*store.SessionLater, *time.Time, error) {
	if err := g.checkReady(ctx, "read session later state"); err != nil {
		return nil, nil, err
	}
	sessionID = strings.TrimSpace(sessionID)
	row, err := g.queries.GetSessionLaterState(ctx, sqlcgen.GetSessionLaterStateParams{
		WorkspaceID: strings.TrimSpace(workspaceID),
		ID:          sessionID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("store: read session later state %q: %w", sessionID, err)
	}
	scanned := store.SessionInfo{}
	if err := populateSessionLaterScanParts(&scanned, &sessionInfoRow{
		parkedAt:             row.ParkedAt,
		parkReason:           row.ParkReason,
		parkUntil:            row.ParkUntil,
		laterReturnedAt:      row.LaterReturnedAt,
		laterPendingMessages: int(row.PendingMessages),
	}); err != nil {
		return nil, nil, err
	}
	return scanned.LaterState(), scanned.LaterReturnedAtValue(), nil
}

// ParkSession marks one stopped, unarchived workspace session as parked in Later.
func (g *SessionRepo) ParkSession(ctx context.Context, park store.SessionLaterPark) (store.SessionInfo, error) {
	if err := g.checkReady(ctx, "park session"); err != nil {
		return store.SessionInfo{}, err
	}
	workspaceID := strings.TrimSpace(park.WorkspaceID)
	sessionID := strings.TrimSpace(park.SessionID)
	if sessionID == "" {
		return store.SessionInfo{}, errors.New("store: park session id is required")
	}
	var result store.SessionInfo
	err := g.withImmediateTransaction(ctx, "park session", func(exec globalSQLExecutor) error {
		current, e := readWorkspaceSessionInfo(ctx, exec, workspaceID, sessionID)
		if e != nil {
			return e
		}
		if current.ArchivedAt != nil {
			return fmt.Errorf("%w: %s", store.ErrSessionArchived, sessionID)
		}
		if current.State != globalDBSessionStateStopped {
			return fmt.Errorf("%w: %s", store.ErrSessionLaterRequiresStopped, sessionID)
		}
		until := sql.NullString{}
		if park.ReturnAt != nil {
			until = sql.NullString{String: store.FormatTimestamp(*park.ReturnAt), Valid: true}
		}
		now := sql.NullString{String: store.FormatTimestamp(g.now()), Valid: true}
		if _, e = sqlcgen.New(exec).ParkSession(ctx, sqlcgen.ParkSessionParams{
			ParkedAt:    now,
			ParkReason:  strings.TrimSpace(park.Reason),
			ParkUntil:   until,
			WorkspaceID: workspaceID,
			ID:          sessionID,
		}); e != nil {
			return mapSessionLaterConstraint(sessionID, e)
		}
		result, e = readWorkspaceSessionInfo(ctx, exec, workspaceID, sessionID)
		return e
	})
	return result, err
}

// UnparkSession clears Later state for one workspace session.
func (g *SessionRepo) UnparkSession(ctx context.Context, workspaceID string, sessionID string) (bool, error) {
	if err := g.checkReady(ctx, "unpark session"); err != nil {
		return false, err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false, errors.New("store: unpark session id is required")
	}
	affected, err := g.queries.UnparkSession(ctx, sqlcgen.UnparkSessionParams{
		UpdatedAt:   store.FormatTimestamp(g.now()),
		WorkspaceID: strings.TrimSpace(workspaceID),
		ID:          sessionID,
	})
	if err != nil {
		return false, fmt.Errorf("store: unpark session %q: %w", sessionID, err)
	}
	return affected > 0, nil
}

// UnparkSessions clears Later state for a family in one transaction, so a
// failure leaves every member as it was.
func (g *SessionRepo) UnparkSessions(ctx context.Context, workspaceID string, sessionIDs []string) error {
	if err := g.checkReady(ctx, "unpark sessions"); err != nil {
		return err
	}
	at := store.FormatTimestamp(g.now())
	return g.withImmediateTransaction(ctx, "unpark sessions", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		for _, id := range sessionIDs {
			if _, err := q.UnparkSession(ctx, sqlcgen.UnparkSessionParams{
				UpdatedAt: at, WorkspaceID: strings.TrimSpace(workspaceID), ID: strings.TrimSpace(id),
			}); err != nil {
				return fmt.Errorf("store: unpark session %q: %w", id, err)
			}
		}
		return nil
	})
}

// UnparkSessionParkedAt clears Later state only when the session still carries
// the park made at parkedAt, so undoing a failed move never clears a newer park.
func (g *SessionRepo) UnparkSessionParkedAt(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	parkedAt time.Time,
) (bool, error) {
	if err := g.checkReady(ctx, "unpark owned session"); err != nil {
		return false, err
	}
	sessionID = strings.TrimSpace(sessionID)
	affected, err := g.queries.UnparkSessionParkedAt(ctx, sqlcgen.UnparkSessionParkedAtParams{
		UpdatedAt:   store.FormatTimestamp(g.now()),
		WorkspaceID: strings.TrimSpace(workspaceID),
		ID:          sessionID,
		ParkedAt:    sql.NullString{String: store.FormatTimestamp(parkedAt), Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf("store: unpark owned session %q: %w", sessionID, err)
	}
	return affected > 0, nil
}

// ReturnDueLaterSessions brings back parked sessions whose return time passed.
// Each return is a conditional update, so concurrent sweeps never return a
// session twice.
func (g *SessionRepo) ReturnDueLaterSessions(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]store.SessionLaterReturn, error) {
	if err := g.checkReady(ctx, "return due later sessions"); err != nil {
		return nil, err
	}
	if limit <= 0 {
		return nil, errors.New("store: later return limit must be positive")
	}
	at := store.FormatTimestamp(now)
	var returned []store.SessionLaterReturn
	err := g.withImmediateTransaction(ctx, "return due later sessions", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		due, e := q.ListDueLaterSessions(ctx, sqlcgen.ListDueLaterSessionsParams{
			Now:      sql.NullString{String: at, Valid: true},
			RowLimit: int64(limit),
		})
		if e != nil {
			return fmt.Errorf("store: list due later sessions: %w", e)
		}
		returned = make([]store.SessionLaterReturn, 0, len(due))
		for _, row := range due {
			affected, e := q.ReturnLaterSession(ctx, sqlcgen.ReturnLaterSessionParams{
				Now:         sql.NullString{String: at, Valid: true},
				WorkspaceID: row.WorkspaceID,
				ID:          row.ID,
			})
			if e != nil {
				return fmt.Errorf("store: return later session %q: %w", row.ID, e)
			}
			if affected == 0 {
				continue
			}
			returned = append(returned, store.SessionLaterReturn{
				WorkspaceID: row.WorkspaceID,
				SessionID:   row.ID,
				ProfileID:   row.ProfileID,
				Name:        row.Name,
				Reason:      row.ParkReason,
				ReturnedAt:  now,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return returned, nil
}

// ClearLaterReturn clears the back-from-Later mark of one session.
func (g *SessionRepo) ClearLaterReturn(ctx context.Context, sessionID string) (bool, error) {
	if err := g.checkReady(ctx, "clear later return"); err != nil {
		return false, err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false, errors.New("store: clear later return session id is required")
	}
	affected, err := g.queries.ClearLaterReturn(ctx, sqlcgen.ClearLaterReturnParams{
		Now: sql.NullString{String: store.FormatTimestamp(g.now()), Valid: true},
		ID:  sessionID,
	})
	if err != nil {
		return false, fmt.Errorf("store: clear later return %q: %w", sessionID, err)
	}
	return affected > 0, nil
}

func readWorkspaceSessionInfo(
	ctx context.Context,
	exec globalSQLExecutor,
	workspaceID string,
	sessionID string,
) (store.SessionInfo, error) {
	info, err := scanSessionInfo(exec.QueryRowContext(
		ctx,
		sessionInfoSelectQuery+" WHERE workspace_id = ? AND id = ?",
		workspaceID,
		sessionID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionInfo{}, fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
	}
	return info, err
}

func mapSessionLaterConstraint(sessionID string, err error) error {
	if err != nil && strings.Contains(err.Error(), sessionLaterConstraintMessage) {
		return fmt.Errorf("%w: %s", store.ErrSessionInLater, strings.TrimSpace(sessionID))
	}
	return err
}

var _ store.SessionLaterStore = (*SessionRepo)(nil)
