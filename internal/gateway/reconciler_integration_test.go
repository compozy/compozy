//go:build integration

package gateway_test

import (
	"context"
	"errors"
	"net/netip"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGatewayFaultCompensationWithSQLite(t *testing.T) {
	t.Parallel()

	cases := []struct {
		failAt    string
		wantCalls []string
	}{
		{failAt: "persist", wantCalls: []string{}},
		{failAt: "bind", wantCalls: []string{"bind", "unbind"}},
		{failAt: "establish", wantCalls: []string{"bind", "establish", "teardown", "unbind"}},
		{failAt: "verify", wantCalls: []string{"bind", "establish", "verify", "teardown", "unbind"}},
		{
			failAt:    "advertise",
			wantCalls: []string{"bind", "establish", "verify", "advertise", "withdraw", "teardown", "unbind"},
		},
	}
	for _, tt := range cases {
		t.Run("Should unwind a partial "+tt.failAt+" effect in reverse [IT-004]", func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			db, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), store.GlobalDatabaseName))
			if err != nil {
				t.Fatalf("OpenGlobalDB() error = %v", err)
			}
			t.Cleanup(func() {
				if err := db.Close(ctx); err != nil {
					t.Errorf("Close() error = %v", err)
				}
			})
			provider := gateway.ProviderIdentity{Name: "connectivity-test", InstallSource: "bundled"}
			if changed, err := db.Transition(ctx, gateway.TransitionRequest{
				Target: gateway.TargetProvider, Tier: gateway.TierPrivate,
				Desired: gateway.DesiredEnabled, Provider: provider,
			}, time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)); err != nil || !changed {
				t.Fatalf("seed provider transition = (%t, %v), want changed", changed, err)
			}

			var store gateway.Store = db.GatewayRepo
			if tt.failAt == "persist" {
				store = failingGatewayTransitionStore{Store: store}
			}
			effects := newFaultGatewayEffects(tt.failAt)
			policy, err := gateway.NewPolicy(
				store,
				effects,
				true,
				gateway.WithAuthGate(gateway.AuthGateFunc(func(context.Context) (bool, error) { return true, nil })),
			)
			if err != nil {
				t.Fatalf("NewPolicy() error = %v", err)
			}
			_, err = policy.Transition(ctx, gateway.TransitionRequest{
				Target: gateway.TargetSurface, Tier: gateway.TierPrivate,
				Surface: gateway.SurfaceOperatorUI, Desired: gateway.DesiredEnabled,
			})
			if err == nil {
				t.Fatal("Transition() error = nil, want injected failure")
			}
			if effects.reachable() {
				t.Fatalf("partial %s effect remained reachable: %#v", tt.failAt, effects.state())
			}
			if got := effects.calls(); len(got) < len(tt.wantCalls) || !slices.Equal(got[:len(tt.wantCalls)], tt.wantCalls) {
				t.Fatalf("effect call prefix = %#v, want %#v", got, tt.wantCalls)
			}
		})
	}
}

type failingGatewayTransitionStore struct{ gateway.Store }

func (failingGatewayTransitionStore) Transition(
	context.Context,
	gateway.TransitionRequest,
	time.Time,
) (bool, error) {
	return false, errors.New("injected persist failure")
}

type faultGatewayEffects struct {
	mu          sync.Mutex
	failAt      string
	callLog     []string
	bound       bool
	established bool
	advertised  bool
}

func newFaultGatewayEffects(failAt string) *faultGatewayEffects {
	return &faultGatewayEffects{failAt: failAt, callLog: []string{}}
}

func (e *faultGatewayEffects) record(name string) error {
	e.callLog = append(e.callLog, name)
	if e.failAt == name {
		return errors.New("injected " + name + " failure")
	}
	return nil
}

func (e *faultGatewayEffects) Bind(context.Context, gateway.Tier) (netip.AddrPort, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bound = true
	return netip.MustParseAddrPort("127.0.0.1:43210"), e.record("bind")
}

func (e *faultGatewayEffects) Unbind(context.Context, gateway.Tier) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bound = false
	return e.record("unbind")
}

func (e *faultGatewayEffects) Establish(
	context.Context,
	gateway.ProviderActivation,
	netip.AddrPort,
) (gateway.Reachability, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.established = true
	return gateway.Reachability{
		Tier: gateway.TierPrivate, Addresses: []string{"https://gateway.example.test"}, Health: gateway.HealthHealthy,
	}, e.record("establish")
}

func (e *faultGatewayEffects) Teardown(context.Context, gateway.ProviderActivation) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.established = false
	return e.record("teardown")
}

func (e *faultGatewayEffects) Verify(context.Context, gateway.Reachability) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.record("verify")
}

func (e *faultGatewayEffects) Advertise(context.Context, gateway.Reachability, []gateway.Surface) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.advertised = true
	return e.record("advertise")
}

func (e *faultGatewayEffects) Withdraw(context.Context, gateway.Tier) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.advertised = false
	return e.record("withdraw")
}

func (e *faultGatewayEffects) reachable() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.bound || e.established || e.advertised
}

func (e *faultGatewayEffects) state() map[string]bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return map[string]bool{"bound": e.bound, "established": e.established, "advertised": e.advertised}
}

func (e *faultGatewayEffects) calls() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.callLog)
}
