package bridgesdk

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/subprocess"
)

// ManagedConfigResolver converts one daemon-managed instance into provider-specific config.
type ManagedConfigResolver[C any] func(*Session, subprocess.InitializeBridgeManagedInstance) C

// ManagedConfigPrepare applies provider-specific conflicts, listeners, and initial probes.
type ManagedConfigPrepare[C any] func(context.Context, *Session, []C) ([]C, error)

// ManagedConfigReconciler atomically resolves and swaps provider configs.
type ManagedConfigReconciler[C any] struct {
	Routes    *RouteTable[C]
	Resolve   ManagedConfigResolver[C]
	Prepare   ManagedConfigPrepare[C]
	Finalize  ManagedConfigPrepare[C]
	Identity  func(C) string
	Merge     RouteMergeFunc[C]
	OnRemoved func(C) error
	OnPublish func()
}

// Reconcile replaces the route table with one complete managed-instance snapshot.
func (r *ManagedConfigReconciler[C]) Reconcile(
	ctx context.Context,
	session *Session,
	managed []subprocess.InitializeBridgeManagedInstance,
) ([]C, error) {
	if r == nil || r.Routes == nil {
		return nil, errors.New("bridgesdk: managed config reconciler routes are required")
	}
	if r.Resolve == nil || r.Identity == nil {
		return nil, errors.New("bridgesdk: managed config reconciler callbacks are required")
	}
	var configs []C
	if len(managed) > 0 {
		configs = make([]C, 0, len(managed))
	}
	for _, instance := range managed {
		configs = append(configs, r.Resolve(session, instance))
	}
	if r.Prepare != nil {
		prepared, err := r.Prepare(ctx, session, configs)
		if err != nil {
			return nil, err
		}
		configs = prepared
	}
	if err := r.Publish(configs); err != nil {
		return nil, err
	}
	if r.OnPublish != nil {
		r.OnPublish()
	}
	if r.Finalize != nil {
		finalized, err := r.Finalize(ctx, session, configs)
		if err != nil {
			return nil, err
		}
		configs = finalized
		if err := r.Publish(configs); err != nil {
			return nil, err
		}
	}
	return configs, nil
}

// Publish atomically replaces the current route snapshot with already-resolved configs.
func (r *ManagedConfigReconciler[C]) Publish(configs []C) error {
	if r == nil || r.Routes == nil {
		return errors.New("bridgesdk: managed config reconciler routes are required")
	}
	if r.Identity == nil {
		return errors.New("bridgesdk: managed config reconciler identity callback is required")
	}
	next := make(map[string]C, len(configs))
	for _, config := range configs {
		instanceID := strings.TrimSpace(r.Identity(config))
		if instanceID == "" {
			return errors.New("bridgesdk: reconciled config instance id is required")
		}
		if _, duplicate := next[instanceID]; duplicate {
			return fmt.Errorf("bridgesdk: duplicate reconciled config %q", instanceID)
		}
		next[instanceID] = config
	}
	if err := r.Routes.ReplaceWithCleanup(next, r.Merge, r.OnRemoved); err != nil {
		return fmt.Errorf("bridgesdk: clean removed provider config: %w", err)
	}
	return nil
}
