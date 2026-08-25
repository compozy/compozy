package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/store"
)

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
		if profileID != candidate.ProfileID || candidate.Scope == callspkg.ScopeWorkspace && workspaceID != candidate.WorkspaceID {
			return fmt.Errorf("store: operator caller candidate owner does not match requested scope")
		}
		if state == "stopped" {
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
