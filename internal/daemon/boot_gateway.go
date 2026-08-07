package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/gateway"
)

func (d *Daemon) bootGateway(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil {
		return errors.New("daemon: boot gateway state is required")
	}
	store, ok := state.registry.(gateway.Store)
	if !ok {
		return errors.New("daemon: registry does not implement the gateway store")
	}
	deviceStore, deviceStoreAvailable := state.registry.(gateway.DeviceStore)
	if !deviceStoreAvailable {
		policy, err := gateway.NewPolicy(store, nil, state.cfg.Gateway.Enabled)
		if err != nil {
			return fmt.Errorf("daemon: create fail-closed gateway policy: %w", err)
		}
		cleanup.add(policy.Close)
		state.gateway = policy
		if _, err := policy.Reconcile(ctx); err != nil && !errors.Is(err, gateway.ErrExposureRefused) {
			return fmt.Errorf("daemon: reconcile fail-closed gateway: %w", err)
		}
		state.logger.Warn("daemon: gateway device store unavailable; continuing local-only")
		return nil
	}
	devices, err := gateway.NewDeviceService(
		deviceStore,
		gateway.WithDeviceClock(d.now),
		gateway.WithPairingLimits(
			state.cfg.Gateway.Pairing.MaxPending,
			state.cfg.Gateway.Pairing.TTL,
		),
		gateway.WithStreamTicketLimits(256, state.cfg.Gateway.StreamTicket.TTL),
	)
	if err != nil {
		return fmt.Errorf("daemon: create gateway device service: %w", err)
	}
	listeners := newGatewayTierListeners(store, &state.deps, d.gatewayTierFactory)
	effects := &daemonGatewayEffects{listeners: listeners, provider: d.gatewayProviderEffects}
	policy, err := gateway.NewPolicy(
		store,
		effects,
		state.cfg.Gateway.Enabled,
		gateway.WithAuthGate(devices),
	)
	if err != nil {
		return fmt.Errorf("daemon: create gateway policy: %w", err)
	}
	service, err := gateway.NewService(policy, devices)
	if err != nil {
		return fmt.Errorf("daemon: create gateway service: %w", err)
	}
	cleanup.add(policy.Close)
	state.gateway = policy
	state.deps.Gateway = service
	if _, err := policy.Reconcile(ctx); err != nil {
		if errors.Is(err, gateway.ErrExposureRefused) {
			state.logger.Warn("daemon: gateway exposure refused; continuing local-only", "error", err)
			return nil
		}
		return fmt.Errorf("daemon: reconcile gateway before server advertisement: %w", err)
	}
	return nil
}
