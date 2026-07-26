package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func (r *bridgeRuntime) rollbackManagedInstanceStates(
	ctx context.Context,
	instances []bridgepkg.BridgeInstance,
	action string,
) error {
	if len(instances) == 0 {
		return nil
	}

	var rollbackErr error
	for _, instance := range instances {
		if err := r.persistCompensatingInstance(ctx, instance, action); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	return rollbackErr
}

func bridgeInstanceIDs(instances []bridgepkg.BridgeInstance) []string {
	if len(instances) == 0 {
		return nil
	}

	ids := make([]string, 0, len(instances))
	for _, instance := range instances {
		ids = append(ids, strings.TrimSpace(instance.ID))
	}
	return ids
}

func (r *bridgeRuntime) persistCompensatingInstance(
	ctx context.Context,
	instance bridgepkg.BridgeInstance,
	action string,
) error {
	if r == nil {
		return errors.New("daemon: bridge runtime is required")
	}
	instance.UpdatedAt = r.now().UTC()
	if err := instance.Validate(); err != nil {
		return fmt.Errorf("daemon: %s %q: validate compensated state: %w", action, strings.TrimSpace(instance.ID), err)
	}

	rollbackCtx := context.WithoutCancel(ctx)
	if err := r.store.UpdateBridgeInstance(rollbackCtx, instance); err != nil {
		return fmt.Errorf("daemon: %s %q: persist compensated state: %w", action, strings.TrimSpace(instance.ID), err)
	}
	return nil
}
