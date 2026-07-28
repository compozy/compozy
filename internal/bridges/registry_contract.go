package bridges

import "context"

// RegistryStore is the persistence surface consumed by the daemon-owned registry.
type RegistryStore interface {
	InsertBridgeInstance(ctx context.Context, instance BridgeInstance) error
	UpdateBridgeInstance(ctx context.Context, instance BridgeInstance) error
	GetBridgeInstance(ctx context.Context, id string) (BridgeInstance, error)
	ListBridgeInstances(ctx context.Context) ([]BridgeInstance, error)
	ListBridgeCatalogRecords(ctx context.Context, query BridgeCatalogQuery) ([]BridgeCatalogRecord, error)
	ListBridgeInstancesByIDs(ctx context.Context, bridgeInstanceIDs []string) ([]BridgeInstance, error)
	PutBridgeRoute(ctx context.Context, route BridgeRoute) error
	ResolveBridgeRoute(ctx context.Context, key RoutingKey) (BridgeRoute, error)
	ListBridgeRoutes(ctx context.Context, bridgeInstanceID string) ([]BridgeRoute, error)
}

// Registry owns bridge lifecycle validation and canonical routing-key construction.
type Registry interface {
	CreateInstance(ctx context.Context, req CreateInstanceRequest) (*BridgeInstance, error)
	GetInstance(ctx context.Context, id string) (*BridgeInstance, error)
	ListInstances(ctx context.Context) ([]BridgeInstance, error)
	ListCatalogRecords(ctx context.Context, query BridgeCatalogQuery) ([]BridgeCatalogRecord, error)
	ListInstancesByIDs(ctx context.Context, bridgeInstanceIDs []string) ([]BridgeInstance, error)
	UpdateInstance(ctx context.Context, req UpdateInstanceRequest) (*BridgeInstance, error)
	UpdateInstanceState(ctx context.Context, req UpdateInstanceStateRequest) (*BridgeInstance, error)
	BuildRoutingKey(ctx context.Context, key RoutingKey) (RoutingKey, error)
	ResolveRoute(ctx context.Context, key RoutingKey) (*BridgeRoute, error)
	ResolveOrCreateRoute(ctx context.Context, route BridgeRoute) (*BridgeRoute, bool, error)
	UpsertRoute(ctx context.Context, route BridgeRoute) (*BridgeRoute, error)
	ListRoutes(ctx context.Context, bridgeInstanceID string) ([]BridgeRoute, error)
}
