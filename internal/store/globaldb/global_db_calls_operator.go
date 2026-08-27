package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/store"
)

func (g *CallRepo) ReopenOperatorCaller(
	ctx context.Context,
	sessionID string,
	at time.Time,
) error {
	if err := g.checkReady(ctx, "reopen operator caller"); err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("store: operator caller session is required")
	}
	return g.tasks.withTaskImmediateTransaction(ctx, "reopen operator caller", func(exec taskSQLExecutor) error {
		var state string
		var drainingAt sql.NullString
		var openCalls int
		err := exec.QueryRowContext(ctx, `SELECT session.state, session.draining_at,
			(SELECT COUNT(1) FROM calls WHERE governed_root_id = session.id
				AND state IN ('queued', 'running'))
			FROM sessions session
			JOIN operator_caller_sessions operator ON operator.session_id = session.id
			WHERE session.id = ?`, sessionID).Scan(&state, &drainingAt, &openCalls)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("store: session %q is not an operator caller", sessionID)
		}
		if err != nil {
			return fmt.Errorf("store: inspect operator caller %q before reopen: %w", sessionID, err)
		}
		if !drainingAt.Valid {
			return nil
		}
		if state != globalDBSessionStateStopped {
			return fmt.Errorf("store: operator caller %q must be stopped before reopen", sessionID)
		}
		if openCalls != 0 {
			return fmt.Errorf("store: operator caller %q still has %d open calls", sessionID, openCalls)
		}
		if _, err := exec.ExecContext(ctx, `UPDATE sessions
			SET draining_at = NULL, updated_at = ? WHERE id = ?`, store.FormatTimestamp(at), sessionID); err != nil {
			return fmt.Errorf("store: reopen operator caller %q: %w", sessionID, err)
		}
		return nil
	})
}

func (g *CallRepo) ResolveOperatorCaller(
	ctx context.Context,
	candidate callspkg.OperatorCallerBinding,
) (winner callspkg.OperatorCallerBinding, err error) {
	if err := g.checkReady(ctx, "resolve operator caller"); err != nil {
		return callspkg.OperatorCallerBinding{}, err
	}
	candidate.ProfileID = strings.TrimSpace(candidate.ProfileID)
	candidate.WorkspaceID = strings.TrimSpace(candidate.WorkspaceID)
	candidate.SessionID = strings.TrimSpace(candidate.SessionID)
	if candidate.ProfileID == "" || candidate.SessionID == "" {
		return callspkg.OperatorCallerBinding{}, fmt.Errorf("store: operator caller profile and session are required")
	}
	if candidate.Scope == callspkg.ScopeGlobal && candidate.WorkspaceID != "" ||
		candidate.Scope == callspkg.ScopeWorkspace && candidate.WorkspaceID == "" {
		return callspkg.OperatorCallerBinding{}, fmt.Errorf("store: invalid operator caller scope binding")
	}
	err = g.tasks.withTaskImmediateTransaction(ctx, "resolve operator caller", func(exec taskSQLExecutor) error {
		var profileID, workspaceID, state string
		if err := exec.QueryRowContext(ctx, `SELECT profile_id, workspace_id, state FROM sessions WHERE id = ?`, candidate.SessionID).
			Scan(&profileID, &workspaceID, &state); err != nil {
			return fmt.Errorf("store: inspect operator caller candidate %q: %w", candidate.SessionID, err)
		}
		if profileID != candidate.ProfileID ||
			candidate.Scope == callspkg.ScopeWorkspace && workspaceID != candidate.WorkspaceID {
			return fmt.Errorf("store: operator caller candidate owner does not match requested scope")
		}
		if state == globalDBSessionStateStopped {
			return fmt.Errorf("store: operator caller candidate must be live")
		}
		result, err := exec.ExecContext(ctx, `INSERT INTO operator_caller_sessions
			(profile_id, scope, workspace_id, session_id, created_at) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(profile_id, scope, workspace_id) DO NOTHING`, candidate.ProfileID,
			string(candidate.Scope), candidate.WorkspaceID, candidate.SessionID, store.FormatTimestamp(g.now()))
		if err != nil {
			return fmt.Errorf("store: insert operator caller binding: %w", err)
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("store: inspect operator caller insert: %w", err)
		}
		winner = candidate
		winner.Created = inserted == 1
		return exec.QueryRowContext(ctx, `SELECT session_id FROM operator_caller_sessions
			WHERE profile_id = ? AND scope = ? AND workspace_id = ?`, candidate.ProfileID,
			string(candidate.Scope), candidate.WorkspaceID).Scan(&winner.SessionID)
	})
	return winner, err
}

func (g *CallRepo) IsOperatorCallerSession(ctx context.Context, sessionID string) (bool, error) {
	if err := g.checkReady(ctx, "inspect operator caller"); err != nil {
		return false, err
	}
	var marker int
	err := g.db.QueryRowContext(ctx, `SELECT 1 FROM operator_caller_sessions WHERE session_id = ?`,
		strings.TrimSpace(sessionID)).Scan(&marker)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("store: inspect operator caller session: %w", err)
	}
	return true, nil
}
