package core

import (
	"context"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/observe"
)

// BridgeCatalogObserver is the required bounded bridge projection used by public catalog surfaces.
type BridgeCatalogObserver interface {
	CheckBridgeCatalogReady() error
	QueryBridgeEffectiveStatuses(
		ctx context.Context,
		records []bridgepkg.BridgeCatalogRecord,
	) (map[string]bridgepkg.BridgeStatus, error)
	QueryBridgeHealthFor(
		ctx context.Context,
		instances []bridgepkg.BridgeInstance,
	) ([]observe.BridgeInstanceHealth, error)
}

// BridgeService is the daemon-owned bridge runtime surface exposed by API transports.
type BridgeService interface {
	bridgepkg.Registry
	bridgepkg.TargetDirectory
	bridgepkg.BridgeTaskSubscriptionStore
	bridgepkg.TargetResolver
	bridgepkg.BridgeControlTransport
	bridgepkg.DeliveryTransport
	ListProviders(ctx context.Context) ([]bridgepkg.BridgeProvider, error)
	CountBridgeRoutes(ctx context.Context, bridgeInstanceIDs []string) (map[string]int, error)
	ListSecretBindings(ctx context.Context, bridgeInstanceID string) ([]bridgepkg.BridgeSecretBinding, error)
	ListSecretBindingsForInstances(
		ctx context.Context,
		bridgeInstanceIDs []string,
	) (map[string][]bridgepkg.BridgeSecretBinding, error)
	PutSecretBinding(ctx context.Context, binding bridgepkg.BridgeSecretBinding, secretValue *string) error
	DeleteSecretBinding(ctx context.Context, bridgeInstanceID string, bindingName string) error
	StartInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error)
	StopInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error)
	RestartInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error)
}
