package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/bridges"
)

// CountBridgeRoutes returns route totals for a bounded bridge catalog page in one query.
func (g *BridgeRepo) CountBridgeRoutes(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (counts map[string]int, err error) {
	if err := g.checkReady(ctx, "count bridge routes"); err != nil {
		return nil, err
	}
	ids, args, err := normalizeBridgeCatalogBatchIDs(bridgeInstanceIDs)
	if err != nil {
		return nil, err
	}
	counts = make(map[string]int, len(ids))
	for _, id := range ids {
		counts[id] = 0
	}
	if len(ids) == 0 {
		return counts, nil
	}

	// dynamic-sql: sqlc cannot express a caller-sized IN list; only generated placeholders are interpolated.
	// #nosec G202 -- the interpolated fragment contains generated placeholders only.
	statement := `SELECT bridge_instance_id, COUNT(*)
		FROM bridge_routes
		WHERE bridge_instance_id IN (` + placeholders(len(ids)) + `)
		GROUP BY bridge_instance_id`
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: count bridge routes for catalog page: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close bridge route count rows: %w", closeErr))
		}
	}()
	for rows.Next() {
		var id string
		var count int
		if scanErr := rows.Scan(&id, &count); scanErr != nil {
			return nil, fmt.Errorf("store: scan bridge route count: %w", scanErr)
		}
		counts[id] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate bridge route counts: %w", err)
	}
	return counts, nil
}

// ListBridgeSecretBindingsForInstances loads bindings for a bounded bridge catalog page in one query.
func (g *BridgeRepo) ListBridgeSecretBindingsForInstances(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (bindings map[string][]bridges.BridgeSecretBinding, err error) {
	if err := g.checkReady(ctx, "list bridge secret bindings for instances"); err != nil {
		return nil, err
	}
	ids, args, err := normalizeBridgeCatalogBatchIDs(bridgeInstanceIDs)
	if err != nil {
		return nil, err
	}
	bindings = make(map[string][]bridges.BridgeSecretBinding, len(ids))
	for _, id := range ids {
		bindings[id] = []bridges.BridgeSecretBinding{}
	}
	if len(ids) == 0 {
		return bindings, nil
	}

	// dynamic-sql: sqlc cannot express a caller-sized IN list; only generated placeholders are interpolated.
	// #nosec G202 -- the interpolated fragment contains generated placeholders only.
	statement := `SELECT bridge_instance_id, binding_name, secret_ref, kind, created_at, updated_at
		FROM bridge_secret_bindings
		WHERE bridge_instance_id IN (` + placeholders(len(ids)) + `)
		ORDER BY bridge_instance_id ASC, binding_name ASC`
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query bridge secret bindings for catalog page: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close bridge catalog binding rows: %w", closeErr))
		}
	}()
	for rows.Next() {
		binding, scanErr := scanBridgeSecretBinding(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		bindings[binding.BridgeInstanceID] = append(bindings[binding.BridgeInstanceID], binding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate bridge catalog bindings: %w", err)
	}
	return bindings, nil
}

func normalizeBridgeCatalogBatchIDs(input []string) ([]string, []any, error) {
	if len(input) > bridges.MaxBridgeCatalogLimit {
		return nil, nil, fmt.Errorf(
			"store: bridge catalog batch exceeds maximum %d",
			bridges.MaxBridgeCatalogLimit,
		)
	}
	ids := make([]string, 0, len(input))
	args := make([]any, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, raw := range input {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, nil, errors.New("store: bridge instance id is required")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
		args = append(args, id)
	}
	return ids, args, nil
}
