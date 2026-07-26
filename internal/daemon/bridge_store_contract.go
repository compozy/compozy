package daemon

import (
	"context"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

type bridgeDedupStore interface {
	PutBridgeIngestDedup(ctx context.Context, record bridgepkg.IngestDedupRecord) error
	GetBridgeIngestDedup(
		ctx context.Context,
		idempotencyKey string,
		lookupAt time.Time,
	) (bridgepkg.IngestDedupRecord, error)
	DeleteExpiredBridgeIngestDedup(ctx context.Context, now time.Time) (int64, error)
}

type bridgeRuntimeStore interface {
	bridgepkg.RegistryStore
	bridgepkg.DeliveryLedgerStore
	bridgepkg.TargetDirectoryStore
	bridgepkg.ResourceProjectionStore
	bridgepkg.BridgeTaskSubscriptionStore
	bridgeCatalogBatchStore
	bridgeDedupStore
	PutBridgeSecretBinding(ctx context.Context, binding bridgepkg.BridgeSecretBinding) error
	ListBridgeSecretBindings(ctx context.Context, bridgeInstanceID string) ([]bridgepkg.BridgeSecretBinding, error)
	DeleteBridgeSecretBinding(ctx context.Context, bridgeInstanceID string, bindingName string) error
}
