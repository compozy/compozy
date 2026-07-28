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

		missing, err := workspaceRootMissing(ws)
		if err != nil {
			return nil, err
		}
		if !missing {
			reconciled = append(reconciled, ws)
			continue
		}

		if err := r.Unregister(ctx, ws.ID); err != nil {
			if errors.Is(err, ErrWorkspaceNotFound) {
				continue
			}
			return nil, fmt.Errorf("workspace: prune missing workspace %q: %w", ws.ID, err)
		}
		r.logger.Info("workspace.prune",
			"workspace_id", ws.ID,
			"root_dir", ws.RootDir,
		)
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
