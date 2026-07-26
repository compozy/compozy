package globaldb

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/bridges"
)

// ListBridgeInstances returns all persisted bridge instances in stable display-name order.
func (g *BridgeRepo) ListBridgeInstances(
	ctx context.Context,
) (instances []bridges.BridgeInstance, err error) {
	if err := g.checkReady(ctx, "list bridge instances"); err != nil {
		return nil, err
	}

	rows, err := g.queries.ListBridgeInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: query bridge instances: %w", err)
	}

	instances = make([]bridges.BridgeInstance, 0, len(rows))
	for _, row := range rows {
		instance, mapErr := bridgeInstanceFromGenerated(row)
		if mapErr != nil {
			return nil, fmt.Errorf("store: map bridge instance list: %w", mapErr)
		}
		instances = append(instances, instance)
	}

	return instances, nil
}

// ListBridgeInstancesByIDs returns one bounded bridge batch in caller-supplied order.
func (g *BridgeRepo) ListBridgeInstancesByIDs(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (instances []bridges.BridgeInstance, err error) {
	if err := g.checkReady(ctx, "list bridge instances by ids"); err != nil {
		return nil, err
	}
	ids, args, err := normalizeBridgeCatalogBatchIDs(bridgeInstanceIDs)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, errors.New("store: at least one bridge instance id is required")
	}

	// dynamic-sql: sqlc cannot express a caller-sized IN list; only generated placeholders are interpolated.
	// #nosec G202 -- the interpolated fragment contains generated placeholders only.
	statement := `SELECT
			id, scope, workspace_id, platform, extension_name, display_name,
			source, enabled, status, dm_policy, routing_policy, provider_config,
			delivery_defaults, notification_suppress, degradation_reason, degradation_message,
			created_at, updated_at
		 FROM bridge_instances
		 WHERE id IN (` + placeholders(len(ids)) + `)`
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query bridge instances by ids: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close bridge instance batch rows: %w", closeErr))
		}
	}()

	byID := make(map[string]bridges.BridgeInstance, len(ids))
	for rows.Next() {
		instance, scanErr := scanBridgeInstance(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("store: scan bridge instance batch: %w", scanErr)
		}
		byID[instance.ID] = instance
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate bridge instance batch: %w", err)
	}

	instances = make([]bridges.BridgeInstance, 0, len(ids))
	for _, id := range ids {
		if instance, ok := byID[id]; ok {
			instances = append(instances, instance)
		}
	}
	return instances, nil
}
