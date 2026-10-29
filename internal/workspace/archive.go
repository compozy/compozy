package workspace

import (
	"context"
)

type ArchiveBlocker struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type BusyError struct {
	Blockers []ArchiveBlocker `json:"blockers"`
}

func (e *BusyError) Error() string {
	return "workspace has active sessions, live Loops, or enabled automations"
}

func (r *Resolver) SetArchived(ctx context.Context, ref string, archived bool) (Workspace, error) {
	ws, err := r.Get(ctx, ref)
	if err != nil {
		return Workspace{}, err
	}
	updated, err := r.store.SetWorkspaceArchived(ctx, ws.ID, archived)
	if err != nil {
		return Workspace{}, err
	}
	r.Invalidate(ws.ID)
	return updated, nil
}
