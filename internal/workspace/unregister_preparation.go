package workspace

import (
	"context"
	"errors"
)

// UnregisterPreparation owns reversible external state around deletion of a
// workspace row and its database-owned children.
type UnregisterPreparation interface {
	BeforeDelete(ctx context.Context) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// UnregisterPreparer stages external state for one workspace deletion.
type UnregisterPreparer func(ctx context.Context, workspace Workspace) (UnregisterPreparation, error)

// SetUnregisterPreparer installs the late-bound runtime owner for workspace
// state that lives outside the workspace store, such as session directories.
func (r *Resolver) SetUnregisterPreparer(preparer UnregisterPreparer) {
	if r == nil || r.unregister == nil {
		return
	}
	r.unregister.preparerMu.Lock()
	r.unregister.preparer = preparer
	r.unregister.preparerMu.Unlock()
}

func (r *Resolver) prepareUnregister(
	ctx context.Context,
	workspace Workspace,
) (UnregisterPreparation, error) {
	if r == nil || r.unregister == nil {
		return nil, errors.New("workspace: unregister coordinator is required")
	}
	r.unregister.preparerMu.RLock()
	preparer := r.unregister.preparer
	r.unregister.preparerMu.RUnlock()
	if preparer == nil {
		return nil, nil
	}
	return preparer(ctx, workspace)
}
