package daemon

import (
	"log/slog"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensionpkg "github.com/compozy/agh/internal/extension"
)

func newBridgeRuntime(
	store bridgeRuntimeStore,
	logger *slog.Logger,
	now func() time.Time,
	secretResolver BridgeSecretResolver,
	brokerOpts ...bridgepkg.DeliveryBrokerOption,
) *bridgeRuntime {
	if store == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	var registry *extensionpkg.Registry
	if dbSource, ok := store.(extensionDBSource); ok && dbSource.DB() != nil {
		registry = extensionpkg.NewRegistry(dbSource.DB())
	}

	brokerOpts = append(
		brokerOpts,
		bridgepkg.WithDeliveryLedgerStore(store),
		bridgepkg.WithDeliveryBrokerNow(now),
	)
	return &bridgeRuntime{
		Service:        bridgepkg.NewRegistry(store, bridgepkg.WithNow(now)),
		store:          store,
		registry:       registry,
		secretResolver: secretResolver,
		broker:         bridgepkg.NewBroker(nil, brokerOpts...),
		logger:         logger,
		now:            now,
	}
}
