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

		if ws.ArchivedAt != nil {
			reconciled = append(reconciled, ws)
			continue
		}

		missing, err := workspaceRootMissing(ws)
		if err != nil {
			return nil, err
		}
		if !missing {
			reconciled = append(reconciled, ws)
			continue
		}

		// An unavailable folder is not consent to delete its registration or history.
		// Explicit removal is the only destructive lifecycle operation.
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
