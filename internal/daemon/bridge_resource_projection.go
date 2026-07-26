package daemon

import (
	"context"
	"errors"
	"fmt"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/resources"
)

type bridgeResourceReloadMode uint8

const (
	bridgeResourceReloadRequired bridgeResourceReloadMode = iota
	bridgeResourceReloadDeferred
)

func (r *bridgeRuntime) BuildBridgeResourceState(
	ctx context.Context,
	records []resources.Record[bridgepkg.BridgeInstanceSpec],
) (resources.ProjectionPlan, error) {
	if r == nil {
		return nil, errors.New("daemon: bridge runtime is required")
	}
	return bridgepkg.BuildResourceState(ctx, r.store, records, r.now)
}

func (r *bridgeRuntime) ApplyBridgeResourceState(ctx context.Context, plan resources.ProjectionPlan) error {
	return r.applyBridgeResourceState(ctx, plan, bridgeResourceReloadRequired)
}

func (r *bridgeRuntime) applyBridgeResourceState(
	ctx context.Context,
	plan resources.ProjectionPlan,
	reloadMode bridgeResourceReloadMode,
) error {
	if r == nil {
		return errors.New("daemon: bridge runtime is required")
	}
	typed, ok := plan.(*bridgepkg.ResourceProjectionPlan)
	if !ok {
		return fmt.Errorf("daemon: bridge resource plan has type %T", plan)
	}
	if typed == nil {
		return errors.New("daemon: bridge resource plan is required")
	}
	if err := bridgepkg.ApplyResourceState(ctx, r.store, typed); err != nil {
		return err
	}
	if reloadMode == bridgeResourceReloadDeferred ||
		typed.OperationCount() == 0 || len(typed.ChangedExtensions()) == 0 {
		return nil
	}
	if err := r.reloadExtensions(ctx, "bridge.resource"); err != nil {
		rollbackCtx := context.WithoutCancel(ctx)
		rollbackPlan := typed.RollbackPlan()
		if rollbackErr := bridgepkg.ApplyResourceState(rollbackCtx, r.store, rollbackPlan); rollbackErr != nil {
			return fmt.Errorf(
				"daemon: apply bridge resource state: reload failed and rollback also failed: %w",
				errors.Join(err, rollbackErr),
			)
		}
		return fmt.Errorf("daemon: apply bridge resource state: rolled back after reload failure: %w", err)
	}
	return nil
}

func (r *bridgeRuntime) applyBridgeResourcesFromStore(ctx context.Context) error {
	return r.applyBridgeResourcesFromStoreWithReload(ctx, bridgeResourceReloadRequired)
}

// applyBridgeResourcesFromStoreWithoutReload projects desired state before a
// lifecycle transition performs its single, post-state extension reload.
func (r *bridgeRuntime) applyBridgeResourcesFromStoreWithoutReload(ctx context.Context) error {
	return r.applyBridgeResourcesFromStoreWithReload(ctx, bridgeResourceReloadDeferred)
}

func (r *bridgeRuntime) applyBridgeResourcesFromStoreWithReload(
	ctx context.Context,
	reloadMode bridgeResourceReloadMode,
) error {
	records, err := r.resourceStore.List(ctx, r.resourceActor, resources.ResourceFilter{
		Kind: bridgepkg.BridgeInstanceResourceKind,
	})
	if err != nil {
		return fmt.Errorf("daemon: list bridge instance resources: %w", err)
	}
	plan, err := r.BuildBridgeResourceState(ctx, records)
	if err != nil {
		return err
	}
	return r.applyBridgeResourceState(ctx, plan, reloadMode)
}

func (r *bridgeRuntime) triggerBridgeResourceReconcile(ctx context.Context) error {
	if r == nil || r.resourceTrigger == nil {
		return nil
	}
	return r.resourceTrigger(ctx, bridgepkg.BridgeInstanceResourceKind, resources.ReconcileReasonWrite)
}
