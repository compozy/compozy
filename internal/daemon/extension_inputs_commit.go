package daemon

import (
	"context"
	"errors"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
)

func (b extensionInputBinder) Commit(ctx context.Context, plan *extensionInputPlan) (extensionpkg.InputState, error) {
	if plan == nil || plan.committed || plan.rolledBack {
		return extensionpkg.InputState{}, errors.New("daemon: input plan is not prepared")
	}
	for _, write := range plan.writes {
		mutation, err := b.service.applyExtensionSecret(
			ctx,
			plan.key,
			plan.instance.ProfileID,
			write,
			plan.previous[write.envName],
		)
		if err != nil {
			return extensionpkg.InputState{}, errors.Join(err, b.Rollback(ctx, plan))
		}
		plan.mutations = append(plan.mutations, mutation)
	}
	if len(plan.rows) > 0 {
		if err := b.service.inputs.Apply(ctx, plan.instance, plan.rows); err != nil {
			return extensionpkg.InputState{}, errors.Join(err, b.Rollback(ctx, plan))
		}
		plan.rowsCommitted = true
	}
	plan.committed = true
	return plan.state, nil
}

// Rollback remains available after Commit until the enclosing lifecycle operation completes.
func (b extensionInputBinder) Rollback(ctx context.Context, plan *extensionInputPlan) error {
	if plan == nil || plan.rolledBack {
		return nil
	}
	rollbackCtx, cancel := extensionSecretRollbackContext(ctx)
	defer cancel()
	var rollbackErr error
	if plan.rowsCommitted {
		inverse := make([]extensioninput.Mutation, len(plan.rows))
		for index, row := range plan.rows {
			inverse[index] = extensioninput.Mutation{InputID: row.InputID, Before: row.After, After: row.Before}
		}
		if err := b.service.inputs.Apply(rollbackCtx, plan.instance, inverse); err != nil {
			rollbackErr = err
		} else {
			plan.rowsCommitted = false
		}
	}
	if err := b.service.rollbackExtensionSecretMutations(
		rollbackCtx,
		plan.key,
		plan.instance.ProfileID,
		plan.mutations,
	); err != nil {
		rollbackErr = errors.Join(rollbackErr, err)
	} else {
		plan.mutations = nil
	}
	if rollbackErr == nil {
		plan.rolledBack = true
	}
	return rollbackErr
}
