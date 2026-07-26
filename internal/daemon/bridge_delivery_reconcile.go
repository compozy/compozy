package daemon

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

type bridgeDeliveryScope struct {
	scope       bridgepkg.Scope
	workspaceID string
}

func (d *Daemon) bootBridgeDeliveryReconcile(ctx context.Context, state *bootState) error {
	if state == nil || state.bridges == nil {
		return nil
	}
	if err := state.bridges.reconcileBridgeDeliveries(ctx); err != nil {
		return fmt.Errorf("daemon: reconcile bridge deliveries: %w", err)
	}
	return nil
}

func (r *bridgeRuntime) reconcileBridgeDeliveries(ctx context.Context) error {
	if r == nil || r.broker == nil || r.store == nil {
		return errors.New("daemon: durable bridge runtime is required")
	}
	if ctx == nil {
		return errors.New("daemon: bridge delivery reconciliation context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	instances, err := r.ListInstances(ctx)
	if err != nil {
		return fmt.Errorf("daemon: list bridge instances for delivery reconciliation: %w", err)
	}
	instanceByID := make(map[string]bridgepkg.BridgeInstance, len(instances))
	scopeSet := make(map[bridgeDeliveryScope]struct{})
	for _, instance := range instances {
		normalized := instance.Normalized()
		instanceByID[normalized.ID] = normalized
		scopeSet[bridgeDeliveryScope{
			scope:       normalized.Scope,
			workspaceID: normalized.WorkspaceID,
		}] = struct{}{}
	}
	scopes := make([]bridgeDeliveryScope, 0, len(scopeSet))
	for scope := range scopeSet {
		scopes = append(scopes, scope)
	}
	sort.Slice(scopes, func(i int, j int) bool {
		if scopes[i].scope != scopes[j].scope {
			return scopes[i].scope < scopes[j].scope
		}
		return scopes[i].workspaceID < scopes[j].workspaceID
	})

	for _, scope := range scopes {
		if err := r.reconcileBridgeDeliveryScope(ctx, scope, instanceByID); err != nil {
			return err
		}
	}

	r.broker.CompleteDeliveryReconciliation()
	return nil
}

func (r *bridgeRuntime) reconcileBridgeDeliveryScope(
	ctx context.Context,
	scope bridgeDeliveryScope,
	instanceByID map[string]bridgepkg.BridgeInstance,
) error {
	query := bridgepkg.DeliveryLedgerQuery{
		Scope:       scope.scope,
		WorkspaceID: scope.workspaceID,
	}
	if err := r.broker.LoadDeliveryMetrics(ctx, query); err != nil {
		return err
	}
	query.State = bridgepkg.DeliveryLedgerStateActive
	records, err := r.store.ListBridgeDeliveries(ctx, query)
	if err != nil {
		return fmt.Errorf("daemon: list unfinished bridge deliveries: %w", err)
	}
	sort.Slice(records, func(i int, j int) bool {
		if !records[i].UpdatedAt.Equal(records[j].UpdatedAt) {
			return records[i].UpdatedAt.Before(records[j].UpdatedAt)
		}
		return records[i].DeliveryID < records[j].DeliveryID
	})
	for _, record := range records {
		if err := validateBridgeDeliveryScopeRecord(scope, record); err != nil {
			return err
		}
		instance, ok := instanceByID[strings.TrimSpace(record.BridgeInstanceID)]
		if !ok {
			return fmt.Errorf(
				"daemon: reconcile delivery %q: bridge instance %q not found",
				record.DeliveryID,
				record.BridgeInstanceID,
			)
		}
		if instance.Scope != record.Scope || instance.WorkspaceID != record.WorkspaceID {
			return fmt.Errorf(
				"daemon: reconcile delivery %q: instance scope does not match ledger",
				record.DeliveryID,
			)
		}
		if err := r.broker.ReconcileDelivery(ctx, record, instance.ExtensionName); err != nil {
			return fmt.Errorf("daemon: reconcile delivery %q: %w", record.DeliveryID, err)
		}
	}
	return nil
}

func validateBridgeDeliveryScopeRecord(
	scope bridgeDeliveryScope,
	record bridgepkg.DeliveryLedgerRecord,
) error {
	if record.Scope == scope.scope && record.WorkspaceID == scope.workspaceID {
		return nil
	}
	return fmt.Errorf(
		"daemon: reconcile delivery %q: ledger row escaped requested scope",
		record.DeliveryID,
	)
}
