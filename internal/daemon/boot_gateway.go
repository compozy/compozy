package daemon

import (
	"context"
	"errors"
	"fmt"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
)

func (d *Daemon) bootGateway(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil {
		return errors.New("daemon: boot gateway state is required")
	}
	gatewayStore := state.registry
	deviceStore, deviceStoreAvailable := state.registry.(gateway.DeviceStore)
	if !deviceStoreAvailable {
		return d.bootFailClosedGateway(ctx, state, cleanup, gatewayStore)
	}
	return d.bootDeviceGateway(ctx, state, cleanup, gatewayStore, deviceStore)
}

func (d *Daemon) bootFailClosedGateway(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
	gatewayStore gateway.Store,
) error {
	policy, err := gateway.NewPolicy(gatewayStore, nil, state.cfg.Gateway.Enabled)
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

func (d *Daemon) bootDeviceGateway(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
	gatewayStore gateway.Store,
	deviceStore gateway.DeviceStore,
) error {
	devices, err := gateway.NewDeviceService(
		deviceStore,
		gateway.WithDeviceClock(d.now),
		gateway.WithPairingLimits(
			state.cfg.Gateway.Pairing.MaxPending,
			state.cfg.Gateway.Pairing.TTL,
		),
		gateway.WithStreamTicketLimits(gateway.DefaultStreamTicketLimit, state.cfg.Gateway.StreamTicket.TTL),
	)
	if err != nil {
		return fmt.Errorf("daemon: create gateway device service: %w", err)
	}
	state.deps.GatewayAuthLimiter = gateway.NewAuthFailureLimiter(
		state.cfg.Gateway.Auth.RateLimit.MaxFails,
		state.cfg.Gateway.Auth.RateLimit.Window,
		d.now,
	)
	challenges := gateway.NewChallengeRegistry()
	state.deps.GatewayChallenges = challenges
	providerEffects, err := d.resolveGatewayProviderEffects(state, gatewayStore, challenges)
	if err != nil {
		return err
	}
	listeners := newGatewayTierListeners(gatewayStore, &state.deps, d.gatewayTierFactory)
	effects := &daemonGatewayEffects{listeners: listeners, provider: providerEffects}
	policy, err := gateway.NewPolicy(
		gatewayStore,
		effects,
		state.cfg.Gateway.Enabled,
		gateway.WithAuthGate(devices),
	)
	if err != nil {
		return fmt.Errorf("daemon: create gateway policy: %w", err)
	}
	ingressStore, ok := state.registry.(gateway.IngressStore)
	if !ok {
		return errors.New("daemon: registry does not implement the gateway ingress store")
	}
	eventWriter, ok := state.registry.(store.EventSummaryStore)
	if !ok {
		return errors.New("daemon: registry does not implement the gateway ingress event store")
	}
	service, err := gateway.NewService(
		policy,
		devices,
		gateway.WithIngress(
			ingressStore,
			state.accessPolicy,
			gatewayIngressAuditSink{writer: eventWriter, now: d.now},
			d.now,
		),
	)
	if err != nil {
		return fmt.Errorf("daemon: create gateway service: %w", err)
	}
	cleanup.add(policy.Close)
	state.gateway = policy
	state.deps.Gateway = service
	return reconcileGatewayPolicy(ctx, state, policy)
}

func reconcileGatewayPolicy(ctx context.Context, state *bootState, policy gateway.Policy) error {
	if _, err := policy.Reconcile(ctx); err != nil {
		if errors.Is(err, gateway.ErrExposureRefused) {
			state.logger.Warn(
				"daemon: gateway exposure refused; continuing local-only",
				"error",
				err,
			)
			return nil
		}
		if errors.Is(err, gateway.ErrProviderDegraded) {
			state.logger.Warn(
				"daemon: gateway provider degraded; continuing local-only",
				"error",
				err,
			)
			return nil
		}
		return fmt.Errorf("daemon: reconcile gateway before server advertisement: %w", err)
	}
	return nil
}

func (d *Daemon) resolveGatewayProviderEffects(
	state *bootState,
	gatewayStore gateway.Store,
	challenges *gateway.ChallengeRegistry,
) (gateway.EffectDriver, error) {
	if d.gatewayProviderEffects != nil {
		return d.gatewayProviderEffects, nil
	}
	verifier, err := gateway.NewEndpointVerifier(
		state.cfg.Gateway.Verify.Timeout,
		state.cfg.Gateway.Verify.PublicDNSResolver,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create gateway endpoint verifier: %w", err)
	}
	state.gatewayVerifier = verifier
	providerEffects, err := d.newExtensionGatewayProvider(state, gatewayStore, challenges, verifier)
	if err != nil {
		state.logger.Warn(
			"daemon: connectivity provider unavailable; continuing local-only",
			"error",
			err,
		)
		return nil, nil
	}
	return providerEffects, nil
}

func (d *Daemon) newExtensionGatewayProvider(
	state *bootState,
	gatewayStore gateway.Store,
	challenges *gateway.ChallengeRegistry,
	verifier gateway.EndpointVerification,
) (gateway.EffectDriver, error) {
	dbSource, ok := state.registry.(extensionDBSource)
	if !ok || dbSource.DB() == nil {
		return nil, errors.New("daemon: extension registry database is unavailable")
	}
	runtime, ok := state.currentExtensionRuntime().(gateway.ConnectivitySourceResolver)
	if !ok || runtime == nil {
		return nil, errors.New("daemon: extension runtime does not provide connectivity sources")
	}
	identities, ok := gatewayStore.(gateway.ProviderIdentityStore)
	if !ok {
		return nil, errors.New("daemon: gateway store does not provide provider identities")
	}
	trust, err := extensionpkg.NewConnectivityProviderTrustResolver(
		extensionpkg.NewRegistry(dbSource.DB()),
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create connectivity provider trust resolver: %w", err)
	}
	eventWriter, ok := state.registry.(store.EventSummaryStore)
	if !ok {
		return nil, errors.New("daemon: gateway event summary store is unavailable")
	}
	supervisor, err := gateway.NewProviderSupervisor(
		runtime,
		trust,
		identities,
		challenges,
		verifier,
		gateway.WithProviderSupervisorClock(d.now),
		gateway.WithProviderSupervisorLogger(state.logger),
		gateway.WithProviderAuditSink(gatewayProviderAuditSink{writer: eventWriter, now: d.now}),
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create connectivity provider supervisor: %w", err)
	}
	return supervisor, nil
}
