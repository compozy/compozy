package workspace

import (
	"context"
	"errors"
	"fmt"
)

func (r *Resolver) reconcileWorkspaceList(ctx context.Context, workspaces []Workspace) ([]Workspace, error) {
	reconciled := make([]Workspace, 0, len(workspaces))
	for _, ws := range workspaces {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}

		ws = r.withRootAvailability(ctx, ws)
		// An unavailable folder is not consent to delete its registration or history.
		reconciled = append(reconciled, ws)
	}

	return reconciled, nil
}

func workspaceRootMissing(ws Workspace) (bool, error) {
	if _, err := canonicalRoot(ws.RootDir); err != nil {
		if errors.Is(err, ErrWorkspaceRootMissing) {
			return true, nil
		}
		return false, fmt.Errorf("workspace: inspect registered workspace %q root: %w", ws.ID, err)
	}

	return false, nil
}

func (r *Resolver) withRootAvailability(ctx context.Context, ws Workspace) Workspace {
	missing, err := workspaceRootMissing(ws)
	if err != nil {
		r.logger.WarnContext(ctx, "workspace.root_inspection_failed", "workspace_id", ws.ID, "error", err)
		return ws
	}
	ws.RootMissing = missing
	return ws
}
