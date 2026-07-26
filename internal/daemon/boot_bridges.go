package daemon

import (
	"context"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func (d *Daemon) composeBridgeRuntime(state *bootState, cleanup *bootCleanup) *bridgeRuntime {
	if state == nil || state.registry == nil {
		return nil
	}

	store, ok := state.registry.(bridgeRuntimeStore)
	if !ok {
		if state.logger != nil {
			state.logger.Debug(
				"daemon: skipping bridge runtime because registry does not expose bridge persistence",
			)
		}
		return nil
	}

	resolver := d.bridgeSecretResolver
	if !d.bridgeSecretResolverExplicit && state.providerVault != nil {
		resolver = vaultBridgeSecretResolver{service: state.providerVault}
	}
	runtime := newBridgeRuntime(
		store,
		state.logger,
		d.now,
		resolver,
		bridgepkg.WithDeliveryBrokerDescriptorLookup(bridgeToolMetadataLookup{state: state}),
		bridgepkg.WithDeliveryBrokerRegistrationGate(),
	)
	if runtime == nil {
		return nil
	}
	runtime.deadEntities = state.deadEntities
	if cleanup != nil {
		cleanup.add(func(context.Context) error {
			runtime.Close()
			return nil
		})
	}
	return runtime
}
