package daemon

import (
	"context"
	"errors"
	"fmt"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/resources"
)

func (r *bridgeRuntime) rollbackCreatedBridgeResource(
	ctx context.Context,
	createdRecord resources.Record[bridgepkg.BridgeInstanceSpec],
	action string,
	cause error,
) error {
	rollbackCtx := context.WithoutCancel(ctx)
	if err := r.resourceStore.Delete(
		rollbackCtx,
		r.resourceActorForRecordSource(createdRecord.Source),
		createdRecord.ID,
		createdRecord.Version,
	); err != nil {
		return fmt.Errorf(
			"daemon: %s bridge instance %q: follow-up failed and resource rollback also failed: %w",
			action,
			createdRecord.ID,
			errors.Join(cause, err),
		)
	}
	if err := r.applyBridgeResourcesFromStore(rollbackCtx); err != nil {
		return fmt.Errorf(
			"daemon: %s bridge instance %q: follow-up failed and resource rollback apply also failed: %w",
			action,
			createdRecord.ID,
			errors.Join(cause, err),
		)
	}
	return fmt.Errorf(
		"daemon: %s bridge instance %q: removed created resource after follow-up failure: %w",
		action,
		createdRecord.ID,
		cause,
	)
}

func (r *bridgeRuntime) rollbackBridgeResourceState(
	ctx context.Context,
	previousRecord resources.Record[bridgepkg.BridgeInstanceSpec],
	currentVersion int64,
	previousInstance *bridgepkg.BridgeInstance,
	action string,
	cause error,
) error {
	rollbackCtx := context.WithoutCancel(ctx)
	if _, err := r.resourceStore.Put(
		rollbackCtx,
		r.resourceActorForRecordSource(previousRecord.Source),
		resources.Draft[bridgepkg.BridgeInstanceSpec]{
			ID:              previousRecord.ID,
			Scope:           previousRecord.Scope,
			ExpectedVersion: currentVersion,
			Spec:            previousRecord.Spec,
		},
	); err != nil {
		return fmt.Errorf(
			"daemon: %s bridge instance %q: follow-up failed and resource rollback also failed: %w",
			action,
			previousRecord.ID,
			errors.Join(cause, err),
		)
	}
	if err := r.applyBridgeResourcesFromStore(rollbackCtx); err != nil {
		return fmt.Errorf(
			"daemon: %s bridge instance %q: follow-up failed and resource rollback apply also failed: %w",
			action,
			previousRecord.ID,
			errors.Join(cause, err),
		)
	}
	if previousInstance != nil {
		if err := r.persistCompensatingInstance(
			rollbackCtx,
			*previousInstance,
			"restore bridge instance after resource lifecycle failure",
		); err != nil {
			return fmt.Errorf(
				"daemon: %s bridge instance %q: follow-up failed and state rollback also failed: %w",
				action,
				previousRecord.ID,
				errors.Join(cause, err),
			)
		}
	}
	return fmt.Errorf(
		"daemon: %s bridge instance %q: restored persisted state after follow-up failure: %w",
		action,
		previousRecord.ID,
		cause,
	)
}

func (r *bridgeRuntime) transitionExtensionName(
	ctx context.Context,
	bridgeInstanceID string,
	reload bool,
	action string,
) (string, error) {
	if !reload {
		return "", nil
	}

	current, err := r.GetInstance(ctx, bridgeInstanceID)
	if err != nil {
		return "", fmt.Errorf(
			"daemon: %s bridge instance %q: load current extension: %w",
			action,
			bridgeInstanceID,
			err,
		)
	}
	if current == nil {
		return "", nil
	}
	return current.ExtensionName, nil
}

func (r *bridgeRuntime) transitionPreviousInstance(
	ctx context.Context,
	bridgeInstanceID string,
	reload bool,
	action string,
) (*bridgepkg.BridgeInstance, error) {
	if !reload {
		return nil, nil
	}

	current, err := r.GetInstance(ctx, bridgeInstanceID)
	if err != nil {
		return nil, fmt.Errorf(
			"daemon: %s bridge instance %q: load current state: %w",
			action,
			bridgeInstanceID,
			err,
		)
	}
	return current, nil
}

func (r *bridgeRuntime) updateTransitionState(
	ctx context.Context,
	bridgeInstanceID string,
	enabled bool,
	status bridgepkg.BridgeStatus,
	action string,
) (*bridgepkg.BridgeInstance, error) {
	updated, err := r.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:        bridgeInstanceID,
		Enabled:   enabled,
		Status:    status,
		UpdatedAt: r.now().UTC(),
	})
	if err != nil {
		return nil, fmt.Errorf("daemon: %s bridge instance %q: %w", action, bridgeInstanceID, err)
	}
	return updated, nil
}

func (r *bridgeRuntime) reloadTransitionState(
	ctx context.Context,
	bridgeInstanceID string,
	previous *bridgepkg.BridgeInstance,
	reload bool,
	action string,
) error {
	if !reload {
		return nil
	}
	if err := r.reloadExtensions(ctx, bridgeInstanceID); err != nil {
		return r.rollbackTransitionState(ctx, bridgeInstanceID, previous, action, err)
	}
	return nil
}

func (r *bridgeRuntime) rollbackTransitionState(
	ctx context.Context,
	bridgeInstanceID string,
	previous *bridgepkg.BridgeInstance,
	action string,
	reloadErr error,
) error {
	if previous == nil {
		return reloadErr
	}
	if rollbackErr := r.persistCompensatingInstance(
		ctx,
		*previous,
		"restore bridge instance after reload failure",
	); rollbackErr != nil {
		return fmt.Errorf(
			"daemon: %s bridge instance %q: reload failed and persisted-state rollback also failed: %w",
			action,
			bridgeInstanceID,
			errors.Join(reloadErr, rollbackErr),
		)
	}
	return fmt.Errorf(
		"daemon: %s bridge instance %q: restored persisted state after reload failure: %w",
		action,
		bridgeInstanceID,
		reloadErr,
	)
}
