package bridges

import (
	"context"
	"fmt"
	"strings"
)

// ListInstances returns all persisted bridge instances.
func (s *Service) ListInstances(ctx context.Context) ([]BridgeInstance, error) {
	if err := s.checkReady(ctx, "list bridge instances"); err != nil {
		return nil, err
	}

	instances, err := s.store.ListBridgeInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("bridges: list bridge instances: %w", err)
	}
	return cloneRegistryBridgeInstances(instances), nil
}

// ListCatalogRecords returns the lean persisted authority used before page hydration.
func (s *Service) ListCatalogRecords(
	ctx context.Context,
	query BridgeCatalogQuery,
) ([]BridgeCatalogRecord, error) {
	if err := s.checkReady(ctx, "list bridge catalog records"); err != nil {
		return nil, err
	}
	records, err := s.store.ListBridgeCatalogRecords(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("bridges: list bridge catalog records: %w", err)
	}
	return append([]BridgeCatalogRecord(nil), records...), nil
}

// ListInstancesByIDs returns one bounded instance batch in caller-supplied order.
func (s *Service) ListInstancesByIDs(
	ctx context.Context,
	bridgeInstanceIDs []string,
) ([]BridgeInstance, error) {
	if err := s.checkReady(ctx, "list bridge instances by ids"); err != nil {
		return nil, err
	}
	ids, err := normalizeBoundedBridgeInstanceIDs(bridgeInstanceIDs)
	if err != nil {
		return nil, fmt.Errorf("bridges: list bridge instances by ids: %w", err)
	}

	instances, err := s.store.ListBridgeInstancesByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("bridges: list bridge instances by ids: %w", err)
	}
	return cloneBridgeInstancesByIDOrder(instances, ids), nil
}

func cloneRegistryBridgeInstances(instances []BridgeInstance) []BridgeInstance {
	if len(instances) == 0 {
		return instances
	}
	cloned := make([]BridgeInstance, 0, len(instances))
	for _, instance := range instances {
		cloned = append(cloned, *cloneBridgeInstance(instance))
	}
	return cloned
}

func cloneBridgeInstancesByIDOrder(instances []BridgeInstance, ids []string) []BridgeInstance {
	byID := make(map[string]BridgeInstance, len(instances))
	for _, instance := range instances {
		id := strings.TrimSpace(instance.ID)
		if id != "" {
			byID[id] = instance
		}
	}
	ordered := make([]BridgeInstance, 0, len(ids))
	for _, id := range ids {
		if instance, ok := byID[id]; ok {
			ordered = append(ordered, *cloneBridgeInstance(instance))
		}
	}
	return ordered
}
