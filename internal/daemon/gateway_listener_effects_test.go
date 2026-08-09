package daemon

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/gateway"
)

type gatewayListenerStoreStub struct {
	gateway.Store
	mu       sync.Mutex
	snapshot gateway.Snapshot
}

func (s *gatewayListenerStoreStub) Snapshot(context.Context) (gateway.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return gateway.Snapshot{
		Providers: append([]gateway.ProviderActivation(nil), s.snapshot.Providers...),
		Surfaces:  append([]gateway.SurfaceExposure(nil), s.snapshot.Surfaces...),
		Devices:   append([]gateway.DeviceSession(nil), s.snapshot.Devices...),
	}, nil
}

type gatewayListenerServerStub struct {
	mu                  sync.Mutex
	port                int
	startErr            error
	shutdownErr         error
	rejectCanceledStart bool
	startCalls          int
	shutdownCalls       int
}

func (s *gatewayListenerServerStub) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startCalls++
	if s.rejectCanceledStart && ctx.Err() != nil {
		return ctx.Err()
	}
	return s.startErr
}

func (s *gatewayListenerServerStub) Shutdown(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shutdownCalls++
	return s.shutdownErr
}

func (s *gatewayListenerServerStub) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

func (s *gatewayListenerServerStub) counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startCalls, s.shutdownCalls
}

func TestGatewayTierListeners(t *testing.T) {
	t.Parallel()

	t.Run("Should publish the resolved loopback port and reuse an unchanged listener", func(t *testing.T) {
		t.Parallel()

		store := &gatewayListenerStoreStub{snapshot: gateway.Snapshot{Surfaces: []gateway.SurfaceExposure{
			{Surface: gateway.SurfaceOperatorUI, Tier: gateway.TierPrivate, Desired: gateway.DesiredEnabled},
		}}}
		server := &gatewayListenerServerStub{port: 43127}
		factoryCalls := 0
		listeners := newGatewayTierListeners(store, &RuntimeDeps{}, func(
			_ context.Context,
			_ *RuntimeDeps,
			tier gateway.Tier,
			surfaces []gateway.Surface,
		) (Server, error) {
			factoryCalls++
			if tier != gateway.TierPrivate || !slices.Equal(surfaces, []gateway.Surface{gateway.SurfaceOperatorUI}) {
				t.Fatalf("factory plan = (%q, %v)", tier, surfaces)
			}
			return server, nil
		})

		first, err := listeners.Bind(context.Background(), gateway.TierPrivate)
		if err != nil {
			t.Fatalf("Bind(first) error = %v", err)
		}
		second, err := listeners.Bind(context.Background(), gateway.TierPrivate)
		if err != nil {
			t.Fatalf("Bind(second) error = %v", err)
		}
		want := netip.MustParseAddrPort("127.0.0.1:43127")
		if first != want || second != want {
			t.Fatalf("Bind() addresses = (%s, %s), want %s", first, second, want)
		}
		if factoryCalls != 1 {
			t.Fatalf("factory calls = %d, want 1", factoryCalls)
		}
		startCalls, shutdownCalls := server.counts()
		if startCalls != 1 || shutdownCalls != 0 {
			t.Fatalf("server calls before unbind = (start=%d, shutdown=%d)", startCalls, shutdownCalls)
		}
		if err := listeners.Unbind(context.Background(), gateway.TierPrivate); err != nil {
			t.Fatalf("Unbind(first) error = %v", err)
		}
		if err := listeners.Unbind(context.Background(), gateway.TierPrivate); err != nil {
			t.Fatalf("Unbind(second) error = %v", err)
		}
		_, shutdownCalls = server.counts()
		if shutdownCalls != 1 {
			t.Fatalf("Shutdown() calls = %d, want 1", shutdownCalls)
		}
	})

	t.Run("Should shut down a listener whose start fails", func(t *testing.T) {
		t.Parallel()

		startErr := errors.New("partial start failure")
		server := &gatewayListenerServerStub{port: 43128, startErr: startErr}
		store := &gatewayListenerStoreStub{snapshot: gateway.Snapshot{Surfaces: []gateway.SurfaceExposure{
			{Surface: gateway.SurfaceWebhookIngress, Tier: gateway.TierPublic, Desired: gateway.DesiredEnabled},
		}}}
		listeners := newGatewayTierListeners(store, &RuntimeDeps{}, func(
			context.Context,
			*RuntimeDeps,
			gateway.Tier,
			[]gateway.Surface,
		) (Server, error) {
			return server, nil
		})

		_, err := listeners.Bind(context.Background(), gateway.TierPublic)
		if !errors.Is(err, startErr) {
			t.Fatalf("Bind() error = %v, want start failure", err)
		}
		startCalls, shutdownCalls := server.counts()
		if startCalls != 1 || shutdownCalls != 1 {
			t.Fatalf("server calls = (start=%d, shutdown=%d), want (1, 1)", startCalls, shutdownCalls)
		}
	})

	t.Run("Should shut down a listener that cannot report a resolved port", func(t *testing.T) {
		t.Parallel()

		server := &gatewayListenerServerStub{}
		store := &gatewayListenerStoreStub{snapshot: gateway.Snapshot{Surfaces: []gateway.SurfaceExposure{
			{Surface: gateway.SurfaceOperatorUI, Tier: gateway.TierPrivate, Desired: gateway.DesiredEnabled},
		}}}
		listeners := newGatewayTierListeners(store, &RuntimeDeps{}, func(
			context.Context,
			*RuntimeDeps,
			gateway.Tier,
			[]gateway.Surface,
		) (Server, error) {
			return server, nil
		})

		if _, err := listeners.Bind(context.Background(), gateway.TierPrivate); err == nil {
			t.Fatal("Bind() error = nil, want resolved-port failure")
		}
		_, shutdownCalls := server.counts()
		if shutdownCalls != 1 {
			t.Fatalf("Shutdown() calls = %d, want 1", shutdownCalls)
		}
	})

	t.Run("Should replace only the public listener when its consented inventory changes", func(t *testing.T) {
		t.Parallel()

		store := &gatewayListenerStoreStub{snapshot: gateway.Snapshot{Surfaces: []gateway.SurfaceExposure{
			{Surface: gateway.SurfaceWebhookIngress, Tier: gateway.TierPublic, Desired: gateway.DesiredEnabled},
		}}}
		servers := []*gatewayListenerServerStub{{port: 43130}, {port: 43131}, {port: 43132}}
		var plans [][]gateway.Surface
		listeners := newGatewayTierListeners(store, &RuntimeDeps{}, func(
			_ context.Context,
			_ *RuntimeDeps,
			tier gateway.Tier,
			surfaces []gateway.Surface,
		) (Server, error) {
			if tier != gateway.TierPublic {
				t.Fatalf("factory tier = %q, want public", tier)
			}
			plans = append(plans, slices.Clone(surfaces))
			if len(plans) > len(servers) {
				t.Fatalf("listener factory calls = %d, want at most %d", len(plans), len(servers))
			}
			return servers[len(plans)-1], nil
		})

		first, err := listeners.Bind(t.Context(), gateway.TierPublic)
		if err != nil {
			t.Fatalf("Bind(ingress) error = %v", err)
		}
		store.mu.Lock()
		store.snapshot.Surfaces = append(store.snapshot.Surfaces, gateway.SurfaceExposure{
			Surface: gateway.SurfaceOperatorUI, Tier: gateway.TierPublic, Desired: gateway.DesiredEnabled,
		})
		store.mu.Unlock()
		second, err := listeners.Bind(t.Context(), gateway.TierPublic)
		if err != nil {
			t.Fatalf("Bind(combined) error = %v", err)
		}
		store.mu.Lock()
		store.snapshot.Surfaces = store.snapshot.Surfaces[:1]
		store.mu.Unlock()
		third, err := listeners.Bind(t.Context(), gateway.TierPublic)
		if err != nil {
			t.Fatalf("Bind(withdrawn operator) error = %v", err)
		}
		if err := listeners.Unbind(t.Context(), gateway.TierPublic); err != nil {
			t.Fatalf("Unbind(public) error = %v", err)
		}

		wantAddresses := []netip.AddrPort{
			netip.MustParseAddrPort("127.0.0.1:43130"),
			netip.MustParseAddrPort("127.0.0.1:43131"),
			netip.MustParseAddrPort("127.0.0.1:43132"),
		}
		if got := []netip.AddrPort{first, second, third}; !slices.Equal(got, wantAddresses) {
			t.Fatalf("listener addresses = %v, want %v", got, wantAddresses)
		}
		wantPlans := [][]gateway.Surface{
			{gateway.SurfaceWebhookIngress},
			{gateway.SurfaceOperatorUI, gateway.SurfaceWebhookIngress},
			{gateway.SurfaceWebhookIngress},
		}
		if !reflect.DeepEqual(plans, wantPlans) {
			t.Fatalf("listener plans = %#v, want %#v", plans, wantPlans)
		}
		for index, server := range servers {
			startCalls, shutdownCalls := server.counts()
			if startCalls != 1 || shutdownCalls != 1 {
				t.Fatalf("server %d calls = (start=%d, shutdown=%d), want (1, 1)", index, startCalls, shutdownCalls)
			}
		}
	})

	t.Run("Should restore the previous listener when a replacement cannot start", func(t *testing.T) {
		t.Parallel()

		startErr := errors.New("replacement start failure")
		store := &gatewayListenerStoreStub{snapshot: gateway.Snapshot{Surfaces: []gateway.SurfaceExposure{{
			Surface: gateway.SurfaceWebhookIngress, Tier: gateway.TierPublic, Desired: gateway.DesiredEnabled,
		}}}}
		previous := &gatewayListenerServerStub{port: 43133}
		replacement := &gatewayListenerServerStub{port: 43134, startErr: startErr}
		factoryCalls := 0
		listeners := newGatewayTierListeners(store, &RuntimeDeps{}, func(
			context.Context,
			*RuntimeDeps,
			gateway.Tier,
			[]gateway.Surface,
		) (Server, error) {
			factoryCalls++
			if factoryCalls == 1 {
				return previous, nil
			}
			return replacement, nil
		})

		if _, err := listeners.Bind(t.Context(), gateway.TierPublic); err != nil {
			t.Fatalf("Bind(initial) error = %v", err)
		}
		store.mu.Lock()
		store.snapshot.Surfaces = append(store.snapshot.Surfaces, gateway.SurfaceExposure{
			Surface: gateway.SurfaceOperatorUI, Tier: gateway.TierPublic, Desired: gateway.DesiredEnabled,
		})
		store.mu.Unlock()
		if _, err := listeners.Bind(t.Context(), gateway.TierPublic); !errors.Is(err, startErr) {
			t.Fatalf("Bind(replacement) error = %v, want replacement start failure", err)
		}
		active, ok := listeners.active[gateway.TierPublic]
		if !ok || active.server != previous || active.port != 43133 {
			t.Fatalf("active listener = %#v, want restored previous listener", active)
		}
		previousStarts, previousShutdowns := previous.counts()
		if previousStarts != 2 || previousShutdowns != 1 {
			t.Fatalf("previous calls = (start=%d, shutdown=%d), want (2, 1)", previousStarts, previousShutdowns)
		}
		replacementStarts, replacementShutdowns := replacement.counts()
		if replacementStarts != 1 || replacementShutdowns != 1 {
			t.Fatalf(
				"replacement calls = (start=%d, shutdown=%d), want (1, 1)",
				replacementStarts,
				replacementShutdowns,
			)
		}
	})

	t.Run("Should replace a stale listener when re-enabled after shutdown fails", func(t *testing.T) {
		t.Parallel()

		shutdownErr := errors.New("listener still running")
		stale := &gatewayListenerServerStub{port: 43135, shutdownErr: shutdownErr}
		replacement := &gatewayListenerServerStub{port: 43138}
		store := &gatewayListenerStoreStub{snapshot: gateway.Snapshot{Surfaces: []gateway.SurfaceExposure{{
			Surface: gateway.SurfaceOperatorUI, Tier: gateway.TierPrivate, Desired: gateway.DesiredEnabled,
		}}}}
		factoryCalls := 0
		listeners := newGatewayTierListeners(store, &RuntimeDeps{}, func(
			context.Context,
			*RuntimeDeps,
			gateway.Tier,
			[]gateway.Surface,
		) (Server, error) {
			factoryCalls++
			if factoryCalls == 1 {
				return stale, nil
			}
			return replacement, nil
		})

		if _, err := listeners.Bind(t.Context(), gateway.TierPrivate); err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		if err := listeners.Unbind(t.Context(), gateway.TierPrivate); !errors.Is(err, shutdownErr) {
			t.Fatalf("Unbind(first) error = %v, want shutdown failure", err)
		}
		if active, ok := listeners.active[gateway.TierPrivate]; !ok || active.server != stale || !active.stale {
			t.Fatalf("active listener after failed shutdown = %#v, want retained stale server", active)
		}
		stale.mu.Lock()
		stale.shutdownErr = nil
		stale.mu.Unlock()
		bound, err := listeners.Bind(t.Context(), gateway.TierPrivate)
		if err != nil {
			t.Fatalf("Bind(recovery) error = %v", err)
		}
		if want := netip.MustParseAddrPort("127.0.0.1:43138"); bound != want {
			t.Fatalf("Bind(recovery) = %s, want %s", bound, want)
		}
		if active := listeners.active[gateway.TierPrivate]; active.server != replacement || active.stale {
			t.Fatalf("active listener after recovery = %#v, want fresh replacement", active)
		}
		_, shutdownCalls := stale.counts()
		if shutdownCalls != 2 {
			t.Fatalf("stale Shutdown() calls = %d, want 2", shutdownCalls)
		}
	})

	t.Run("Should restore the previous listener after the caller context is canceled", func(t *testing.T) {
		t.Parallel()

		startErr := errors.New("replacement start failure")
		store := &gatewayListenerStoreStub{snapshot: gateway.Snapshot{Surfaces: []gateway.SurfaceExposure{{
			Surface: gateway.SurfaceWebhookIngress, Tier: gateway.TierPublic, Desired: gateway.DesiredEnabled,
		}}}}
		previous := &gatewayListenerServerStub{port: 43136, rejectCanceledStart: true}
		replacement := &gatewayListenerServerStub{port: 43137, startErr: startErr}
		factoryCalls := 0
		listeners := newGatewayTierListeners(store, &RuntimeDeps{}, func(
			context.Context,
			*RuntimeDeps,
			gateway.Tier,
			[]gateway.Surface,
		) (Server, error) {
			factoryCalls++
			if factoryCalls == 1 {
				return previous, nil
			}
			return replacement, nil
		})

		if _, err := listeners.Bind(t.Context(), gateway.TierPublic); err != nil {
			t.Fatalf("Bind(initial) error = %v", err)
		}
		store.mu.Lock()
		store.snapshot.Surfaces = append(store.snapshot.Surfaces, gateway.SurfaceExposure{
			Surface: gateway.SurfaceOperatorUI, Tier: gateway.TierPublic, Desired: gateway.DesiredEnabled,
		})
		store.mu.Unlock()
		canceledCtx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := listeners.Bind(canceledCtx, gateway.TierPublic); !errors.Is(err, startErr) {
			t.Fatalf("Bind(replacement) error = %v, want replacement failure", err)
		}
		active, ok := listeners.active[gateway.TierPublic]
		if !ok || active.server != previous || active.port != 43136 {
			t.Fatalf("active listener = %#v, want previous listener restored", active)
		}
	})
}

func TestGatewaySurfaceSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		tier      gateway.Tier
		surfaces  []gateway.Surface
		want      string
		wantErrIs error
	}{
		{
			name: "Should select private operator routes", tier: gateway.TierPrivate,
			surfaces: []gateway.Surface{gateway.SurfaceOperatorUI}, want: "private",
		},
		{
			name: "Should select public ingress routes", tier: gateway.TierPublic,
			surfaces: []gateway.Surface{gateway.SurfaceWebhookIngress}, want: "public_ingress",
		},
		{
			name: "Should select public operator routes", tier: gateway.TierPublic,
			surfaces: []gateway.Surface{gateway.SurfaceOperatorUI}, want: "public_operator",
		},
		{
			name: "Should select combined public routes", tier: gateway.TierPublic,
			surfaces: []gateway.Surface{
				gateway.SurfaceWebhookIngress,
				gateway.SurfaceOperatorUI,
			},
			want: "public_combined",
		},
		{
			name: "Should refuse an empty public surface set", tier: gateway.TierPublic,
			wantErrIs: gateway.ErrExposureRefused,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := gatewaySurfaceSet(test.tier, test.surfaces)
			if test.wantErrIs != nil {
				if !errors.Is(err, test.wantErrIs) {
					t.Fatalf("gatewaySurfaceSet() error = %v, want %v", err, test.wantErrIs)
				}
				return
			}
			if err != nil || string(got) != test.want {
				t.Fatalf("gatewaySurfaceSet() = (%q, %v), want %q", got, err, test.want)
			}
		})
	}
}
