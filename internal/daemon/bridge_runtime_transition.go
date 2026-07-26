package daemon

import (
	"context"
	"errors"
	"fmt"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/subprocess"
)

func (r *bridgeRuntime) StartInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	return r.transitionInstance(ctx, id, true, bridgepkg.BridgeStatusStarting, true, "start")
}

func (r *bridgeRuntime) StopInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	return r.transitionInstance(ctx, id, false, bridgepkg.BridgeStatusDisabled, true, "stop")
}

func (r *bridgeRuntime) RestartInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	return r.transitionInstance(ctx, id, true, bridgepkg.BridgeStatusStarting, true, "restart")
}

func (r *bridgeRuntime) ResolveBridgeRuntime(
	ctx context.Context,
	extensionName string,
) (*subprocess.InitializeBridgeRuntime, error) {
	if r == nil {
		return nil, errors.New("daemon: bridge runtime is required")
	}
	if ctx == nil {
		return nil, errors.New("daemon: bridge runtime context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ctx, unlock := r.lockExtensionLifecycleContext(ctx, extensionName)
	defer unlock()

	managedInstances, err := r.managedInstancesForExtension(ctx, extensionName)
	if err != nil {
		return nil, err
	}

	launching, err := r.prepareManagedBridgeRuntime(ctx, managedInstances)
	if err != nil {
		return nil, err
	}

	runtime := subprocess.InitializeBridgeRuntime{
		RuntimeVersion:   subprocess.InitializeBridgeRuntimeVersion2,
		Purpose:          subprocess.BridgeRuntimePurposeService,
		Provider:         strings.TrimSpace(extensionName),
		Platform:         strings.TrimSpace(launching[0].Instance.Platform),
		ManagedInstances: launching,
	}
	if err := runtime.Validate(); err != nil {
		return nil, fmt.Errorf(
			"daemon: build bridge runtime for extension %q: %w",
			strings.TrimSpace(extensionName),
			err,
		)
	}
	return &runtime, nil
}

func (r *bridgeRuntime) transitionInstance(
	ctx context.Context,
	id string,
	enabled bool,
	status bridgepkg.BridgeStatus,
	reload bool,
	action string,
) (*bridgepkg.BridgeInstance, error) {
	if r == nil {
		return nil, errors.New("daemon: bridge runtime is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("daemon: %s bridge instance context is required", action)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.resourceDefinitionsEnabled() {
		return r.transitionResourceInstance(ctx, id, enabled, status, reload, action)
	}

	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, fmt.Errorf("daemon: %s bridge instance id is required", action)
	}

	extensionName, err := r.transitionExtensionName(ctx, trimmedID, reload, action)
	if err != nil {
		return nil, err
	}
	ctx, unlockExtension := r.lockExtensionLifecycleContext(ctx, extensionName)
	defer unlockExtension()
	ctx, unlockInstance := r.lockInstanceLifecycleContext(ctx, trimmedID)
	defer unlockInstance()

	previous, err := r.transitionPreviousInstance(ctx, trimmedID, reload, action)
	if err != nil {
		return nil, err
	}

	if _, err := r.updateTransitionState(ctx, trimmedID, enabled, status, action); err != nil {
		return nil, err
	}

	if err := r.reloadTransitionState(ctx, trimmedID, previous, reload, action); err != nil {
		return nil, err
	}

	return r.GetInstance(ctx, trimmedID)
}

func (r *bridgeRuntime) transitionResourceInstance(
	ctx context.Context,
	id string,
	enabled bool,
	status bridgepkg.BridgeStatus,
	reload bool,
	action string,
) (*bridgepkg.BridgeInstance, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil, fmt.Errorf("daemon: %s bridge instance id is required", action)
	}

	currentRecord, err := r.resourceStore.Get(ctx, r.resourceActor, trimmedID)
	if err != nil {
		return nil, fmt.Errorf("daemon: %s bridge instance %q: load resource: %w", action, trimmedID, err)
	}

	ctx, unlockExtension := r.lockExtensionLifecycleContext(ctx, currentRecord.Spec.ExtensionName)
	defer unlockExtension()
	ctx, unlockInstance := r.lockInstanceLifecycleContext(ctx, trimmedID)
	defer unlockInstance()

	previous, err := r.GetInstance(ctx, trimmedID)
	if err != nil {
		return nil, fmt.Errorf("daemon: %s bridge instance %q: load current state: %w", action, trimmedID, err)
	}

	nextSpec := currentRecord.Spec
	nextSpec.Enabled = enabled
	updatedRecord, err := r.resourceStore.Put(
		ctx,
		r.resourceActorForRecordSource(currentRecord.Source),
		resources.Draft[bridgepkg.BridgeInstanceSpec]{
			ID:              currentRecord.ID,
			Scope:           bridgepkg.ResourceScopeForBridge(nextSpec.Scope, nextSpec.WorkspaceID),
			ExpectedVersion: currentRecord.Version,
			Spec:            nextSpec,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: %s bridge instance %q: write resource: %w", action, trimmedID, err)
	}
	if err := r.applyBridgeResourcesFromStoreWithoutReload(ctx); err != nil {
		return nil, err
	}
	return r.finalizeTransitionResourceInstance(
		ctx,
		currentRecord,
		updatedRecord,
		previous,
		trimmedID,
		enabled,
		status,
		reload,
		action,
	)
}

func (r *bridgeRuntime) finalizeTransitionResourceInstance(
	ctx context.Context,
	currentRecord resources.Record[bridgepkg.BridgeInstanceSpec],
	updatedRecord resources.Record[bridgepkg.BridgeInstanceSpec],
	previous *bridgepkg.BridgeInstance,
	trimmedID string,
	enabled bool,
	status bridgepkg.BridgeStatus,
	reload bool,
	action string,
) (*bridgepkg.BridgeInstance, error) {
	_, err := r.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:        trimmedID,
		Enabled:   enabled,
		Status:    status,
		UpdatedAt: r.now().UTC(),
	})
	if err != nil {
		return nil, r.rollbackBridgeResourceState(
			ctx,
			currentRecord,
			updatedRecord.Version,
			previous,
			action,
			fmt.Errorf("daemon: %s bridge instance %q: set runtime state: %w", action, trimmedID, err),
		)
	}

	if reload {
		if err := r.reloadExtensions(ctx, trimmedID); err != nil {
			return nil, r.rollbackBridgeResourceState(
				ctx,
				currentRecord,
				updatedRecord.Version,
				previous,
				action,
				err,
			)
		}
	}
	if err := r.triggerBridgeResourceReconcile(ctx); err != nil {
		return nil, r.rollbackBridgeResourceState(
			ctx,
			currentRecord,
			updatedRecord.Version,
			previous,
			action,
			err,
		)
	}
	return r.GetInstance(ctx, trimmedID)
}
