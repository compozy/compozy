package daemon

import (
	"context"
	"errors"
	"fmt"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/resources"
)

// CreateInstance persists one bridge instance and, when the instance is
// immediately enabled, reloads extensions so bridge-capable adapters can bind
// to the new runtime without requiring a manual restart.
func (r *bridgeRuntime) CreateInstance(
	ctx context.Context,
	req bridgepkg.CreateInstanceRequest,
) (*bridgepkg.BridgeInstance, error) {
	if r == nil {
		return nil, errors.New("daemon: bridge runtime is required")
	}
	if r.resourceDefinitionsEnabled() {
		return r.createInstanceResource(ctx, req)
	}

	ctx, unlockExtension := r.lockExtensionLifecycleContext(ctx, req.ExtensionName)
	defer unlockExtension()
	ctx, unlockInstance := r.lockInstanceLifecycleContext(ctx, req.ID)
	defer unlockInstance()

	created, err := r.Service.CreateInstance(ctx, req)
	if err != nil {
		return nil, err
	}
	if created == nil || !created.Enabled || created.Status.Normalize() == bridgepkg.BridgeStatusDisabled {
		return created, nil
	}
	if err := r.reloadExtensions(ctx, created.ID); err != nil {
		compensated := *created
		compensated.Enabled = false
		compensated.Status = bridgepkg.BridgeStatusDisabled
		if rollbackErr := r.persistCompensatingInstance(
			ctx,
			compensated,
			"disable newly created bridge instance after reload failure",
		); rollbackErr != nil {
			return nil, fmt.Errorf(
				"daemon: create bridge instance %q: reload failed and compensation also failed: %w",
				strings.TrimSpace(created.ID),
				errors.Join(err, rollbackErr),
			)
		}
		return nil, fmt.Errorf(
			"daemon: create bridge instance %q: persisted instance rolled back to disabled after reload failure: %w",
			strings.TrimSpace(created.ID),
			err,
		)
	}
	return created, nil
}

func (r *bridgeRuntime) createInstanceResource(
	ctx context.Context,
	req bridgepkg.CreateInstanceRequest,
) (*bridgepkg.BridgeInstance, error) {
	id, spec, err := bridgepkg.BridgeInstanceSpecFromCreateRequest(req, r.now)
	if err != nil {
		return nil, fmt.Errorf("daemon: create bridge instance resource: %w", err)
	}

	ctx, unlockExtension := r.lockExtensionLifecycleContext(ctx, spec.ExtensionName)
	defer unlockExtension()
	ctx, unlockInstance := r.lockInstanceLifecycleContext(ctx, id)
	defer unlockInstance()

	createdRecord, err := r.resourceStore.Put(
		ctx,
		r.resourceActorForSource(spec.Source),
		resources.Draft[bridgepkg.BridgeInstanceSpec]{
			ID:              id,
			Scope:           bridgepkg.ResourceScopeForBridge(spec.Scope, spec.WorkspaceID),
			ExpectedVersion: 0,
			Spec:            spec,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create bridge instance resource %q: %w", id, err)
	}
	if err := r.applyBridgeResourcesFromStore(ctx); err != nil {
		return nil, r.rollbackCreatedBridgeResource(ctx, createdRecord, "create", err)
	}
	if err := r.triggerBridgeResourceReconcile(ctx); err != nil {
		return nil, r.rollbackCreatedBridgeResource(ctx, createdRecord, "create", err)
	}

	created, err := r.GetInstance(ctx, id)
	if err != nil {
		return nil, r.rollbackCreatedBridgeResource(ctx, createdRecord, "create", err)
	}
	return created, nil
}

// UpdateInstance writes mutable bridge desired state through canonical resources when enabled.
func (r *bridgeRuntime) UpdateInstance(
	ctx context.Context,
	req bridgepkg.UpdateInstanceRequest,
) (*bridgepkg.BridgeInstance, error) {
	if r == nil {
		return nil, errors.New("daemon: bridge runtime is required")
	}
	if !r.resourceDefinitionsEnabled() {
		return r.Service.UpdateInstance(ctx, req)
	}
	return r.updateInstanceResource(ctx, req)
}

func (r *bridgeRuntime) updateInstanceResource(
	ctx context.Context,
	req bridgepkg.UpdateInstanceRequest,
) (*bridgepkg.BridgeInstance, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("daemon: update bridge instance resource %q: %w", strings.TrimSpace(req.ID), err)
	}

	current, err := r.loadMutableBridgeInstanceResource(ctx, strings.TrimSpace(req.ID))
	if err != nil {
		return nil, err
	}
	next := updatedBridgeInstanceSpec(current.Spec, req)

	ctx, unlockExtension := r.lockExtensionLifecycleContext(ctx, next.ExtensionName)
	defer unlockExtension()
	ctx, unlockInstance := r.lockInstanceLifecycleContext(ctx, current.ID)
	defer unlockInstance()

	previous, err := r.GetInstance(ctx, current.ID)
	if err != nil {
		return nil, fmt.Errorf("daemon: update bridge instance resource %q: load current state: %w", current.ID, err)
	}

	updatedRecord, err := r.putBridgeInstanceResource(ctx, current, next)
	if err != nil {
		return nil, err
	}
	if err := r.applyBridgeResourcesFromStore(ctx); err != nil {
		return nil, r.rollbackBridgeResourceState(ctx, current, updatedRecord.Version, previous, "update", err)
	}
	if err := r.applyBridgeUpdateOperationalState(ctx, current.ID, next.Enabled, req); err != nil {
		return nil, r.rollbackBridgeResourceState(ctx, current, updatedRecord.Version, previous, "update", err)
	}
	if err := r.triggerBridgeResourceReconcile(ctx); err != nil {
		return nil, r.rollbackBridgeResourceState(ctx, current, updatedRecord.Version, previous, "update", err)
	}

	updated, err := r.GetInstance(ctx, current.ID)
	if err != nil {
		return nil, r.rollbackBridgeResourceState(ctx, current, updatedRecord.Version, previous, "update", err)
	}
	return updated, nil
}

func (r *bridgeRuntime) loadMutableBridgeInstanceResource(
	ctx context.Context,
	id string,
) (resources.Record[bridgepkg.BridgeInstanceSpec], error) {
	current, err := r.resourceStore.Get(ctx, r.resourceActor, id)
	if err != nil {
		return resources.Record[bridgepkg.BridgeInstanceSpec]{}, fmt.Errorf(
			"daemon: update bridge instance resource %q: %w",
			id,
			err,
		)
	}
	if current.Spec.Source == bridgepkg.BridgeInstanceSourcePackage {
		return resources.Record[bridgepkg.BridgeInstanceSpec]{}, fmt.Errorf(
			"daemon: update bridge instance resource %q: %w",
			current.ID,
			bridgepkg.ErrBridgeInstanceReadOnly,
		)
	}
	return current, nil
}

func updatedBridgeInstanceSpec(
	current bridgepkg.BridgeInstanceSpec,
	req bridgepkg.UpdateInstanceRequest,
) bridgepkg.BridgeInstanceSpec {
	next := current
	if req.DisplayName != nil {
		next.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.DMPolicy != nil {
		next.DMPolicy = req.DMPolicy.Normalize()
	}
	if req.RoutingPolicy != nil {
		next.RoutingPolicy = *req.RoutingPolicy
	}
	if req.ProviderConfig != nil {
		next.ProviderConfig = append([]byte(nil), (*req.ProviderConfig)...)
	}
	if req.DeliveryDefaults != nil {
		next.DeliveryDefaults = append([]byte(nil), (*req.DeliveryDefaults)...)
	}
	return next
}

func (r *bridgeRuntime) putBridgeInstanceResource(
	ctx context.Context,
	current resources.Record[bridgepkg.BridgeInstanceSpec],
	next bridgepkg.BridgeInstanceSpec,
) (resources.Record[bridgepkg.BridgeInstanceSpec], error) {
	record, err := r.resourceStore.Put(
		ctx,
		r.resourceActorForRecordSource(current.Source),
		resources.Draft[bridgepkg.BridgeInstanceSpec]{
			ID:              current.ID,
			Scope:           bridgepkg.ResourceScopeForBridge(next.Scope, next.WorkspaceID),
			ExpectedVersion: current.Version,
			Spec:            next,
		},
	)
	if err != nil {
		return resources.Record[bridgepkg.BridgeInstanceSpec]{}, fmt.Errorf(
			"daemon: update bridge instance resource %q: %w",
			current.ID,
			err,
		)
	}
	return record, nil
}

func (r *bridgeRuntime) applyBridgeUpdateOperationalState(
	ctx context.Context,
	id string,
	enabled bool,
	req bridgepkg.UpdateInstanceRequest,
) error {
	if !req.ClearDegradation && req.Degradation == nil {
		return nil
	}
	currentInstance, err := r.GetInstance(ctx, id)
	if err != nil {
		return err
	}
	_, err = r.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:               id,
		Enabled:          enabled,
		Status:           bridgeStatusForBridgeUpdate(currentInstance.Status, req),
		Degradation:      req.Degradation,
		ClearDegradation: req.ClearDegradation,
		UpdatedAt:        r.now().UTC(),
	})
	return err
}

func bridgeStatusForBridgeUpdate(
	current bridgepkg.BridgeStatus,
	req bridgepkg.UpdateInstanceRequest,
) bridgepkg.BridgeStatus {
	status := current.Normalize()
	if req.Degradation == nil || req.Degradation.IsZero() {
		return status
	}
	switch status {
	case bridgepkg.BridgeStatusDegraded,
		bridgepkg.BridgeStatusAuthRequired,
		bridgepkg.BridgeStatusError:
		return status
	default:
		return bridgepkg.BridgeStatusDegraded
	}
}

func (r *bridgeRuntime) resourceActorForSource(source bridgepkg.BridgeInstanceSource) resources.MutationActor {
	actor := r.resourceActor
	if actor.Kind == "" {
		actor = resourceReconcileActor()
	}
	normalized := source.Normalize()
	if normalized == "" {
		normalized = bridgepkg.BridgeInstanceSourceDynamic
	}
	actor.ID = "bridge." + strings.TrimSpace(string(normalized))
	actor.Source = resources.ResourceSource{
		Kind: resources.ResourceSourceKind("daemon"),
		ID:   actor.ID,
	}
	return actor
}

func (r *bridgeRuntime) resourceActorForRecordSource(source resources.ResourceSource) resources.MutationActor {
	actor := r.resourceActor
	if actor.Kind == "" {
		actor = resourceReconcileActor()
	}
	normalized := source.Normalize()
	if normalized.Kind == "" || strings.TrimSpace(normalized.ID) == "" {
		return actor
	}
	actor.ID = normalized.ID
	actor.Source = normalized
	return actor
}
