package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

// StageWorkspaceDeletion atomically persists recovery identity and removes the workspace row.
func (g *WorkspaceRepo) StageWorkspaceDeletion(ctx context.Context, id string) error {
	if err := g.checkReady(ctx, "stage workspace deletion"); err != nil {
		return err
	}
	workspaceID := strings.TrimSpace(id)
	if workspaceID == "" {
		return errors.New("store: workspace id is required")
	}
	return store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		affected, err := queries.InsertWorkspaceDeletionIntent(ctx, sqlcgen.InsertWorkspaceDeletionIntentParams{
			RequestedAt: store.FormatTimestamp(g.now()), WorkspaceID: workspaceID,
		})
		if err != nil {
			return fmt.Errorf("store: stage workspace deletion intent %q: %w", workspaceID, err)
		}
		if affected == 0 {
			return fmt.Errorf("store: workspace %q: %w", workspaceID, compozyworkspace.ErrWorkspaceNotFound)
		}
		return deleteWorkspaceTx(ctx, queries, workspaceID)
	})
}

// GetWorkspaceDeletionIntent loads one durable post-delete finalization owner.
func (g *WorkspaceRepo) GetWorkspaceDeletionIntent(
	ctx context.Context,
	id string,
) (compozyworkspace.DeletionIntent, error) {
	if err := g.checkReady(ctx, "get workspace deletion intent"); err != nil {
		return compozyworkspace.DeletionIntent{}, err
	}
	workspaceID := strings.TrimSpace(id)
	if workspaceID == "" {
		return compozyworkspace.DeletionIntent{}, errors.New("store: workspace id is required")
	}
	row, err := g.queries.GetWorkspaceDeletionIntent(ctx, workspaceID)
	return workspaceDeletionIntentFromGenerated(row, err)
}

// ListWorkspaceDeletionIntents returns every pending finalization in stable order.
func (g *WorkspaceRepo) ListWorkspaceDeletionIntents(
	ctx context.Context,
) ([]compozyworkspace.DeletionIntent, error) {
	if err := g.checkReady(ctx, "list workspace deletion intents"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListWorkspaceDeletionIntents(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list workspace deletion intents: %w", err)
	}
	intents := make([]compozyworkspace.DeletionIntent, 0, len(rows))
	for _, row := range rows {
		intent, scanErr := workspaceDeletionIntentFromGenerated(row, nil)
		if scanErr != nil {
			return nil, scanErr
		}
		intents = append(intents, intent)
	}
	return intents, nil
}

// CompleteWorkspaceDeletion releases a durable intent only after external cleanup succeeds.
func (g *WorkspaceRepo) CompleteWorkspaceDeletion(ctx context.Context, id string) error {
	if err := g.checkReady(ctx, "complete workspace deletion"); err != nil {
		return err
	}
	workspaceID := strings.TrimSpace(id)
	if workspaceID == "" {
		return errors.New("store: workspace id is required")
	}
	affected, err := g.queries.DeleteWorkspaceDeletionIntent(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("store: complete workspace deletion intent %q: %w", workspaceID, err)
	}
	if affected == 0 {
		return fmt.Errorf(
			"store: workspace deletion intent %q: %w",
			workspaceID,
			compozyworkspace.ErrWorkspaceDeletionIntentNotFound,
		)
	}
	return nil
}

func deleteWorkspaceTx(ctx context.Context, queries *sqlcgen.Queries, workspaceID string) error {
	if err := ensureWorkspaceDeletionAllowed(ctx, queries, workspaceID); err != nil {
		return err
	}
	if err := deleteWorkspaceMCPState(ctx, queries, workspaceID); err != nil {
		return err
	}
	if err := deleteWorkspaceExtensionEnv(ctx, queries, workspaceID); err != nil {
		return err
	}
	if err := queries.DeleteSessionsByWorkspace(ctx, workspaceID); err != nil {
		return fmt.Errorf("store: delete stopped sessions for workspace %q: %w", workspaceID, err)
	}
	if _, err := queries.DeleteGatewayIngressBindingsByWorkspace(ctx, store.SQLNullString(workspaceID)); err != nil {
		return fmt.Errorf("store: delete gateway ingress bindings for workspace %q: %w", workspaceID, err)
	}
	affected, err := queries.DeleteWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("store: delete workspace %q: %w", workspaceID, mapWorkspaceDeleteConstraintError(err))
	}
	if affected == 0 {
		return fmt.Errorf("store: workspace %q: %w", workspaceID, compozyworkspace.ErrWorkspaceNotFound)
	}
	return nil
}

func workspaceDeletionIntentFromGenerated(
	row sqlcgen.WorkspaceDeletionIntent,
	queryErr error,
) (compozyworkspace.DeletionIntent, error) {
	if queryErr != nil {
		if errors.Is(queryErr, sql.ErrNoRows) {
			return compozyworkspace.DeletionIntent{}, compozyworkspace.ErrWorkspaceDeletionIntentNotFound
		}
		return compozyworkspace.DeletionIntent{}, fmt.Errorf("store: scan workspace deletion intent: %w", queryErr)
	}
	workspace, err := workspaceFromGenerated(sqlcgen.Workspace{
		ID: row.WorkspaceID, RootDir: row.RootDir, AddDirs: row.AddDirs, Name: row.Name,
		DefaultAgent: row.DefaultAgent, SandboxRef: row.SandboxRef,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil)
	if err != nil {
		return compozyworkspace.DeletionIntent{}, err
	}
	requestedAt, err := store.ParseTimestamp(row.RequestedAt)
	if err != nil {
		return compozyworkspace.DeletionIntent{}, fmt.Errorf("store: parse workspace deletion requested_at: %w", err)
	}
	return compozyworkspace.DeletionIntent{Workspace: workspace, RequestedAt: requestedAt}, nil
}
