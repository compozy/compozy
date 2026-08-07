package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"

	"github.com/compozy/compozy/internal/gateway"
)

type gatewayTierListener struct {
	server   Server
	port     int
	surfaces []gateway.Surface
}

type gatewayTierListeners struct {
	mu      sync.Mutex
	store   gateway.Store
	deps    *RuntimeDeps
	factory GatewayTierServerFactory
	active  map[gateway.Tier]gatewayTierListener
}

func newGatewayTierListeners(
	store gateway.Store,
	deps *RuntimeDeps,
	factory GatewayTierServerFactory,
) *gatewayTierListeners {
	return &gatewayTierListeners{
		store: store, deps: deps, factory: factory,
		active: make(map[gateway.Tier]gatewayTierListener),
	}
}

func (l *gatewayTierListeners) Bind(ctx context.Context, tier gateway.Tier) (netip.AddrPort, error) {
	if l == nil || l.store == nil || l.deps == nil || l.factory == nil {
		return netip.AddrPort{}, errors.New("daemon: gateway tier listener dependencies are required")
	}
	snapshot, err := l.store.Snapshot(ctx)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("daemon: load gateway listener surfaces: %w", err)
	}
	surfaces := desiredGatewaySurfaces(snapshot, tier)

	l.mu.Lock()
	defer l.mu.Unlock()
	if current, ok := l.active[tier]; ok && slices.Equal(current.surfaces, surfaces) {
		return gatewayLoopbackAddr(current.port)
	}
	server, err := l.factory(ctx, l.deps, tier, surfaces)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("daemon: construct %s gateway listener: %w", tier, err)
	}
	if server == nil {
		return netip.AddrPort{}, fmt.Errorf("daemon: construct %s gateway listener: server is required", tier)
	}
	current, replacing := l.active[tier]
	if replacing {
		if err := shutdownGatewayTierServer(ctx, current.server); err != nil {
			return netip.AddrPort{}, err
		}
	}
	if err := server.Start(ctx); err != nil {
		startErr := errors.Join(
			fmt.Errorf("daemon: start %s gateway listener: %w", tier, err),
			shutdownGatewayTierServer(ctx, server),
		)
		return netip.AddrPort{}, errors.Join(startErr, l.restoreGatewayTierListener(ctx, tier, current, replacing))
	}
	port := resolveDaemonPort(0, server)
	if port <= 0 || port > 65535 {
		shutdownErr := shutdownGatewayTierServer(ctx, server)
		portErr := errors.Join(
			fmt.Errorf("daemon: %s gateway listener did not report a resolved port", tier),
			shutdownErr,
		)
		return netip.AddrPort{}, errors.Join(portErr, l.restoreGatewayTierListener(ctx, tier, current, replacing))
	}
	l.active[tier] = gatewayTierListener{
		server: server, port: port, surfaces: append([]gateway.Surface(nil), surfaces...),
	}
	return gatewayLoopbackAddr(port)
}

func (l *gatewayTierListeners) restoreGatewayTierListener(
	ctx context.Context,
	tier gateway.Tier,
	current gatewayTierListener,
	replacing bool,
) error {
	if !replacing {
		delete(l.active, tier)
		return nil
	}
	if err := current.server.Start(context.WithoutCancel(ctx)); err != nil {
		delete(l.active, tier)
		return fmt.Errorf("daemon: restore previous %s gateway listener: %w", tier, err)
	}
	l.active[tier] = current
	return nil
}

func (l *gatewayTierListeners) Unbind(ctx context.Context, tier gateway.Tier) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	current, ok := l.active[tier]
	if !ok {
		return nil
	}
	if err := shutdownGatewayTierServer(ctx, current.server); err != nil {
		return err
	}
	delete(l.active, tier)
	return nil
}

func desiredGatewaySurfaces(snapshot gateway.Snapshot, tier gateway.Tier) []gateway.Surface {
	surfaces := make([]gateway.Surface, 0, len(snapshot.Surfaces))
	for _, exposure := range snapshot.Surfaces {
		if exposure.Tier == tier && exposure.Desired == gateway.DesiredEnabled {
			surfaces = append(surfaces, exposure.Surface)
		}
	}
	slices.Sort(surfaces)
	return surfaces
}

func gatewayLoopbackAddr(port int) (netip.AddrPort, error) {
	if port <= 0 || port > 65535 {
		return netip.AddrPort{}, fmt.Errorf("daemon: gateway listener port %d is out of range", port)
	}
	return netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(port)), nil
}

func shutdownGatewayTierServer(ctx context.Context, server Server) error {
	if server == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("daemon: shutdown gateway tier listener: %w", err)
	}
	return nil
}

type daemonGatewayEffects struct {
	listeners *gatewayTierListeners
	provider  gateway.EffectDriver
}

func (e *daemonGatewayEffects) Bind(ctx context.Context, tier gateway.Tier) (netip.AddrPort, error) {
	return e.listeners.Bind(ctx, tier)
}

func (e *daemonGatewayEffects) Unbind(ctx context.Context, tier gateway.Tier) error {
	return e.listeners.Unbind(ctx, tier)
}

func (e *daemonGatewayEffects) Establish(
	ctx context.Context,
	provider gateway.ProviderActivation,
	bound netip.AddrPort,
) (gateway.Reachability, error) {
	if e.provider == nil {
		return gateway.Reachability{}, fmt.Errorf(
			"%w: connectivity provider effects are unavailable",
			gateway.ErrExposureRefused,
		)
	}
	return e.provider.Establish(ctx, provider, bound)
}

func (e *daemonGatewayEffects) Teardown(ctx context.Context, provider gateway.ProviderActivation) error {
	if e.provider == nil {
		return nil
	}
	return e.provider.Teardown(ctx, provider)
}

func (e *daemonGatewayEffects) Verify(ctx context.Context, reachability gateway.Reachability) error {
	if e.provider == nil {
		return fmt.Errorf("%w: connectivity provider effects are unavailable", gateway.ErrExposureRefused)
	}
	return e.provider.Verify(ctx, reachability)
}

func (e *daemonGatewayEffects) Advertise(
	ctx context.Context,
	reachability gateway.Reachability,
	surfaces []gateway.Surface,
) error {
	if e.provider == nil {
		return fmt.Errorf("%w: connectivity provider effects are unavailable", gateway.ErrExposureRefused)
	}
	return e.provider.Advertise(ctx, reachability, surfaces)
}

func (e *daemonGatewayEffects) Withdraw(ctx context.Context, tier gateway.Tier) error {
	if e.provider == nil {
		return nil
	}
	return e.provider.Withdraw(ctx, tier)
}

var _ gateway.EffectDriver = (*daemonGatewayEffects)(nil)
