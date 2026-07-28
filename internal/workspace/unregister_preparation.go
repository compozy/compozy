package workspace

import "context"

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
	if r == nil {
		return
	}
	r.unregisterMu.Lock()
	r.unregisterPreparer = preparer
	r.unregisterMu.Unlock()
}

func (r *Resolver) prepareUnregister(
	ctx context.Context,
	workspace Workspace,
) (UnregisterPreparation, error) {
	r.unregisterMu.RLock()
	preparer := r.unregisterPreparer
	r.unregisterMu.RUnlock()
	if preparer == nil {
		return nil, nil
	}
	return preparer(ctx, workspace)
}
