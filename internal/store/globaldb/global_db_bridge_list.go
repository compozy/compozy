package globaldb

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/store"
)

const bridgeInstanceSelectWithOwner = `
	bi.id, bi.profile_id, p.name, p.color, COALESCE(p.icon, ''), COALESCE(p.emoji, ''), p.archived_at IS NOT NULL,
	bi.scope, bi.workspace_id, bi.platform, bi.extension_name, bi.display_name,
	bi.source, bi.enabled, bi.status, bi.dm_policy, bi.routing_policy, bi.provider_config,
	bi.delivery_defaults, bi.notification_suppress, bi.degradation_reason, bi.degradation_message,
	bi.created_at, bi.updated_at`

// ListBridgeInstances returns bridge instances matching readScope in stable
// display-name order.
func (g *BridgeRepo) ListBridgeInstances(
	ctx context.Context,
	readScope store.ReadScope,
) (instances []bridges.BridgeInstance, err error) {
	if err := g.checkReady(ctx, "list bridge instances"); err != nil {
		return nil, err
	}

	if err := readScope.Validate(); err != nil {
		return nil, fmt.Errorf("store: invalid bridge instance read scope: %w", err)
	}
	statement := `SELECT ` + bridgeInstanceSelectWithOwner + `
		FROM bridge_instances bi JOIN profiles p ON p.id = bi.profile_id`
	where, args := store.BuildClauses(store.ReadScopeClause("bi.profile_id", readScope))
	statement = store.AppendWhere(statement, where)
	statement += ` ORDER BY bi.display_name ASC, bi.created_at ASC, bi.id ASC`
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query bridge instances: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close bridge instance rows: %w", closeErr))
		}
	}()

	instances = make([]bridges.BridgeInstance, 0)
	for rows.Next() {
		instance, scanErr := scanBridgeInstance(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("store: map bridge instance list: %w", scanErr)
		}
		instances = append(instances, instance)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate bridge instance rows: %w", err)
	}

	return instances, nil
}

// ListBridgeInstancesByIDs returns one bounded bridge batch matching readScope
// in caller-supplied order.
func (g *BridgeRepo) ListBridgeInstancesByIDs(
	ctx context.Context,
	readScope store.ReadScope,
	bridgeInstanceIDs []string,
) (instances []bridges.BridgeInstance, err error) {
	if err := g.checkReady(ctx, "list bridge instances by ids"); err != nil {
		return nil, err
	}
	if err := readScope.Validate(); err != nil {
		return nil, fmt.Errorf("store: invalid bridge instance batch read scope: %w", err)
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
	statement := `SELECT ` + bridgeInstanceSelectWithOwner + `
		FROM bridge_instances bi JOIN profiles p ON p.id = bi.profile_id
		WHERE bi.id IN (` + placeholders(len(ids)) + `)`
	if !readScope.AllProfiles {
		statement += globalDBBridgeProfileClause
		args = append(args, readScope.ProfileID)
	}
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
