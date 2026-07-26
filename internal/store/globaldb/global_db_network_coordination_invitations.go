package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

var _ workspacepkg.CoordinationInvitations = (*WorkspaceRepo)(nil)

// GetInvitation returns one persisted dismissal. Absent rows are explicit not-dismissed.
func (g *WorkspaceRepo) GetInvitation(
	ctx context.Context,
	workspaceID string,
	scopeKind string,
	scopeID string,
) (workspacepkg.CoordinationInvitation, error) {
	if err := g.checkReady(ctx, "get network coordination invitation"); err != nil {
		return workspacepkg.CoordinationInvitation{}, err
	}
	workspace := strings.TrimSpace(workspaceID)
	kind := strings.TrimSpace(scopeKind)
	id := strings.TrimSpace(scopeID)
	if workspace == "" || kind == "" || id == "" {
		return workspacepkg.CoordinationInvitation{}, fmt.Errorf(
			"store: invitation workspace_id, scope_kind, and scope_id are required",
		)
	}
	if err := validateCoordinationInvitationScope(ctx, g.db, workspace, kind, id); err != nil {
		return workspacepkg.CoordinationInvitation{}, err
	}
	row, err := g.queries.GetNetworkCoordinationInvitation(ctx, sqlcgen.GetNetworkCoordinationInvitationParams{
		WorkspaceID: workspace,
		ScopeKind:   kind,
		ScopeID:     id,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return workspacepkg.CoordinationInvitation{
			WorkspaceID: workspace,
			ScopeKind:   kind,
			ScopeID:     id,
		}, nil
	}
	if err != nil {
		return workspacepkg.CoordinationInvitation{}, fmt.Errorf(
			"store: get network coordination invitation %s/%s: %w",
			kind,
			id,
			err,
		)
	}
	return invitationFromGenerated(row)
}

// DismissInvitation upserts a dismissal row for the scope.
func (g *WorkspaceRepo) DismissInvitation(
	ctx context.Context,
	workspaceID string,
	scopeKind string,
	scopeID string,
	actor string,
) (invitation workspacepkg.CoordinationInvitation, err error) {
	if err := g.checkReady(ctx, "dismiss network coordination invitation"); err != nil {
		return workspacepkg.CoordinationInvitation{}, err
	}
	workspace := strings.TrimSpace(workspaceID)
	kind := strings.TrimSpace(scopeKind)
	id := strings.TrimSpace(scopeID)
	dismissedBy := strings.TrimSpace(actor)
	if workspace == "" || kind == "" || id == "" {
		return workspacepkg.CoordinationInvitation{}, fmt.Errorf(
			"store: invitation workspace_id, scope_kind, and scope_id are required",
		)
	}
	if dismissedBy == "" {
		return workspacepkg.CoordinationInvitation{}, fmt.Errorf(
			"store: invitation dismissed_by is required",
		)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	err = store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		if validateErr := validateCoordinationInvitationScope(ctx, tx, workspace, kind, id); validateErr != nil {
			return validateErr
		}
		queries := sqlcgen.New(tx)
		row, upsertErr := queries.UpsertNetworkCoordinationInvitation(
			ctx,
			sqlcgen.UpsertNetworkCoordinationInvitationParams{
				WorkspaceID: workspace,
				ScopeKind:   kind,
				ScopeID:     id,
				DismissedAt: now,
				DismissedBy: dismissedBy,
			},
		)
		if upsertErr != nil {
			return fmt.Errorf(
				"store: upsert network coordination invitation %s/%s: %w",
				kind,
				id,
				upsertErr,
			)
		}
		parsed, parseErr := invitationFromGenerated(row)
		if parseErr != nil {
			return parseErr
		}
		invitation = parsed
		return nil
	})
	if err != nil {
		return workspacepkg.CoordinationInvitation{}, err
	}
	return invitation, nil
}

// ResetInvitation deletes the dismissal row so the invitation may appear again.
func (g *WorkspaceRepo) ResetInvitation(
	ctx context.Context,
	workspaceID string,
	scopeKind string,
	scopeID string,
) error {
	if err := g.checkReady(ctx, "reset network coordination invitation"); err != nil {
		return err
	}
	workspace := strings.TrimSpace(workspaceID)
	kind := strings.TrimSpace(scopeKind)
	id := strings.TrimSpace(scopeID)
	if workspace == "" || kind == "" || id == "" {
		return fmt.Errorf("store: invitation workspace_id, scope_kind, and scope_id are required")
	}
	return store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		if validateErr := validateCoordinationInvitationScope(ctx, tx, workspace, kind, id); validateErr != nil {
			return validateErr
		}
		queries := sqlcgen.New(tx)
		if delErr := queries.DeleteNetworkCoordinationInvitation(
			ctx,
			sqlcgen.DeleteNetworkCoordinationInvitationParams{
				WorkspaceID: workspace,
				ScopeKind:   kind,
				ScopeID:     id,
			},
		); delErr != nil {
			return fmt.Errorf(
				"store: delete network coordination invitation %s/%s: %w",
				kind,
				id,
				delErr,
			)
		}
		return nil
	})
}

func validateCoordinationInvitationScope(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	scopeKind string,
	scopeID string,
) error {
	var exists int
	var err error
	switch scopeKind {
	case workspacepkg.InvitationScopeWorkspace:
		if scopeID != workspaceID {
			return fmt.Errorf("%w: workspace invitation scope does not match workspace", taskpkg.ErrValidation)
		}
		err = exec.QueryRowContext(ctx, `SELECT 1 FROM workspaces WHERE id = ?`, workspaceID).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return workspacepkg.ErrWorkspaceNotFound
		}
	case workspacepkg.InvitationScopeTask:
		err = exec.QueryRowContext(
			ctx,
			`SELECT 1 FROM tasks WHERE id = ? AND workspace_id = ?`,
			scopeID,
			workspaceID,
		).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.ErrTaskNotFound
		}
	default:
		return fmt.Errorf("%w: unsupported invitation scope_kind %q", taskpkg.ErrValidation, scopeKind)
	}
	if err != nil {
		return fmt.Errorf("store: validate network coordination invitation scope: %w", err)
	}
	return nil
}

func invitationFromGenerated(row sqlcgen.NetworkCoordinationInvitation) (workspacepkg.CoordinationInvitation, error) {
	dismissedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(row.DismissedAt))
	if err != nil {
		dismissedAt, err = time.Parse(time.RFC3339, strings.TrimSpace(row.DismissedAt))
		if err != nil {
			return workspacepkg.CoordinationInvitation{}, fmt.Errorf(
				"store: parse invitation dismissed_at: %w",
				err,
			)
		}
	}
	return workspacepkg.CoordinationInvitation{
		WorkspaceID: strings.TrimSpace(row.WorkspaceID),
		ScopeKind:   strings.TrimSpace(row.ScopeKind),
		ScopeID:     strings.TrimSpace(row.ScopeID),
		Dismissed:   true,
		DismissedAt: dismissedAt.UTC(),
		DismissedBy: strings.TrimSpace(row.DismissedBy),
	}, nil
}
