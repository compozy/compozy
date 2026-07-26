package daemon

import (
	"context"
	"errors"
	"fmt"

	"slices"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	extensionpkg "github.com/compozy/agh/internal/extension"

	"github.com/compozy/agh/internal/subprocess"
)

func (r *bridgeRuntime) reloadExtensions(ctx context.Context, bridgeInstanceID string) error {
	if r == nil {
		return errors.New("daemon: bridge runtime is required")
	}
	if ctx == nil {
		return errors.New("daemon: bridge runtime reload context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.RLock()
	extensions := r.extensions
	r.mu.RUnlock()
	if extensions == nil {
		return nil
	}

	if err := extensions.Reload(ctx); err != nil {
		return fmt.Errorf("daemon: reload extensions for bridge instance %q: %w", bridgeInstanceID, err)
	}
	return nil
}

func (r *bridgeRuntime) managedInstancesForExtension(
	ctx context.Context,
	extensionName string,
) ([]bridgepkg.BridgeInstance, error) {
	trimmed := strings.TrimSpace(extensionName)
	if trimmed == "" {
		return nil, errors.New("daemon: bridge runtime extension name is required")
	}

	instances, err := r.ListInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: list bridge instances for extension %q: %w", trimmed, err)
	}

	matches := make([]bridgepkg.BridgeInstance, 0, 1)
	for _, instance := range instances {
		if strings.TrimSpace(instance.ExtensionName) != trimmed {
			continue
		}
		if !instance.Enabled || instance.Status.Normalize() == bridgepkg.BridgeStatusDisabled {
			continue
		}
		matches = append(matches, instance)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf(
			"%w: no enabled bridge instance configured for extension %q",
			extensionpkg.ErrBridgeRuntimeDeferred,
			trimmed,
		)
	}

	slices.SortFunc(matches, func(left bridgepkg.BridgeInstance, right bridgepkg.BridgeInstance) int {
		return strings.Compare(left.ID, right.ID)
	})
	return matches, nil
}

func (r *bridgeRuntime) prepareManagedBridgeRuntime(
	ctx context.Context,
	instances []bridgepkg.BridgeInstance,
) ([]subprocess.InitializeBridgeManagedInstance, error) {
	if len(instances) == 0 {
		return nil, errors.New("daemon: bridge runtime requires at least one managed instance")
	}

	ctx, unlock := r.lockManagedInstanceLifecycleSet(ctx, bridgeInstanceIDs(instances))
	defer unlock()

	resolvedSecrets := make(map[string][]subprocess.InitializeBridgeBoundSecret, len(instances))
	for _, instance := range instances {
		boundSecrets, err := r.resolveBoundSecrets(ctx, instance.ID)
		if err != nil {
			return nil, fmt.Errorf("daemon: resolve bound secrets for bridge instance %q: %w", instance.ID, err)
		}
		resolvedSecrets[instance.ID] = boundSecrets
	}

	updated := make([]subprocess.InitializeBridgeManagedInstance, 0, len(instances))
	previous := make([]bridgepkg.BridgeInstance, 0, len(instances))
	for _, instance := range instances {
		launching := instance
		if instance.Status.Normalize() != bridgepkg.BridgeStatusStarting {
			transitioned, err := r.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
				ID:        instance.ID,
				Enabled:   instance.Enabled,
				Status:    bridgepkg.BridgeStatusStarting,
				UpdatedAt: r.now().UTC(),
			})
			if err != nil {
				if rollbackErr := r.rollbackManagedInstanceStates(
					ctx,
					previous,
					"restore bridge instances after launch failure",
				); rollbackErr != nil {
					return nil, fmt.Errorf(
						"daemon: launch bridge runtime for extension %q: transition failed and rollback also failed: %w",
						strings.TrimSpace(instance.ExtensionName),
						errors.Join(err, rollbackErr),
					)
				}
				return nil, fmt.Errorf(
					"daemon: launch bridge runtime for extension %q: restored persisted state after launch failure: %w",
					strings.TrimSpace(instance.ExtensionName),
					err,
				)
			}
			previous = append(previous, instance)
			launching = *transitioned
		}

		updated = append(updated, subprocess.InitializeBridgeManagedInstance{
			Instance:     bridgepkg.BridgeInstanceToContract(launching),
			BoundSecrets: resolvedSecrets[instance.ID],
		})
	}

	return updated, nil
}
