package computers

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

const probeComputer remoteproto.ComputerID = "cmp_routetest"
const probeAccount = "acc_routetest"

func TestRouteSelection(t *testing.T) {
	t.Parallel()
	t.Run("Should prefer direct before faster SSH and hosted routes", func(t *testing.T) {
		t.Parallel()
		// UT-032. Safety Invariants 2 and 15. Completion order cannot select the route.
		clock := clockwork.NewFakeClock()
		pin, pair := tunnelCertificates(t, clock.Now())
		descriptor := remoteproto.Descriptor{
			ComputerID: probeComputer, TLSRootSHA256: pin, Protocol: remoteproto.CurrentProtocol,
		}
		delayed := func(delay time.Duration) func() bool {
			return func() bool { <-time.After(delay); return true }
		}
		direct := serveProbeTunnel(t, descriptor, pair, clock, false, delayed(300*time.Millisecond))
		ssh := serveProbeTunnel(t, descriptor, pair, clock, false, delayed(50*time.Millisecond))
		hosted := serveProbeTunnel(t, descriptor, pair, clock, true, delayed(20*time.Millisecond))
		sshAddress := probeURL(t, ssh.URL).Host
		cloud := &routingCloud{}
		routes := []store.ComputerRoute{
			{ID: "hosted", Kind: remoteproto.RouteCloud, Address: probeURL(t, hosted.URL).Host},
			{ID: "ssh", Kind: remoteproto.RouteSSH},
			{ID: "direct", Kind: remoteproto.RouteDirect, Address: probeURL(t, direct.URL).Host},
		}
		rows := []store.Computer{{
			ID: probeComputer, AccountID: probeAccount, Enabled: true, TLSRootSHA: pin, Routes: routes,
		}}
		registry := New(t.Context(), &routingStore{computers: rows}, newRoutingAccount(t), cloud, WithClock(clock),
			WithSSHForward(func(context.Context, store.ComputerRoute) (string, error) { return sshAddress, nil }),
			func(r *Registry) { r.routeHTTP = testRouteHTTP(hosted.Client()) })
		t.Cleanup(registry.Close)
		waitUnitRoute(t, registry, remoteproto.RouteDirect)
		if cloud.grants.Load() != 1 {
			t.Fatalf("selected route requested %d grants, want 1", cloud.grants.Load())
		}
	})
	t.Run("Should switch from hosted to direct on the better route tick", func(t *testing.T) {
		t.Parallel()
		// UT-033. Safety Invariants 1 and 2.
		clock := clockwork.NewFakeClock()
		pin, pair := tunnelCertificates(t, clock.Now())
		descriptor := remoteproto.Descriptor{
			ComputerID: probeComputer, TLSRootSHA256: pin, Protocol: remoteproto.CurrentProtocol,
		}
		var directUp atomic.Bool
		direct := serveProbeTunnel(t, descriptor, pair, clock, false, directUp.Load)
		hosted := serveProbeTunnel(t, descriptor, pair, clock, true, func() bool { return true })
		directURL := probeURL(t, direct.URL)
		hostedURL := probeURL(t, hosted.URL)
		rows := []store.Computer{{
			ID: probeComputer, AccountID: probeAccount, Enabled: true, TLSRootSHA: pin,
			Routes: []store.ComputerRoute{
				{ID: "direct", Kind: remoteproto.RouteDirect, Address: directURL.Host},
				{ID: "hosted", Kind: remoteproto.RouteCloud, Address: hostedURL.Host},
			},
		}}
		testAccount := newRoutingAccount(t)
		var log bytes.Buffer
		registry := New(t.Context(), &routingStore{computers: rows}, testAccount, &routingCloud{}, WithClock(clock),
			WithLogger(slog.New(slog.NewTextHandler(&log, nil))),
			func(r *Registry) { r.routeHTTP = testRouteHTTP(hosted.Client()) })
		t.Cleanup(registry.Close)
		waitUnitRoute(t, registry, remoteproto.RouteCloud)
		directUp.Store(true)
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		if err := clock.BlockUntilContext(ctx, 3); err != nil {
			t.Fatal(err)
		}
		clock.Advance(betterRouteInterval)
		waitUnitRoute(t, registry, remoteproto.RouteDirect)
		registry.Close()
		if !bytes.Contains(log.Bytes(), []byte("from=cloud to=direct reason=better_route")) {
			t.Fatalf("route switch event missing: %s", log.Bytes())
		}
	})
	t.Run("Should hold a route back after two drops within ten minutes", func(t *testing.T) {
		t.Parallel()
		// UT-034. A held route cannot enter descriptor preflight.
		clock := clockwork.NewFakeClock()
		route := store.ComputerRoute{ID: "direct", Kind: remoteproto.RouteDirect, Address: "127.0.0.1:1"}
		h := routeHealth{}.dropped(clock.Now())
		clock.Advance(dropWindow - time.Second)
		h = h.dropped(clock.Now())
		clock.Advance(holdBack - time.Second)
		s := supervisor{scope: &registryScope{registry: &Registry{clock: clock}}, ctx: t.Context(),
			health: map[string]routeHealth{route.ID: h}}
		probes := s.probe([]store.ComputerRoute{route})
		if !h.heldUntil.Equal(clock.Now().Add(time.Second)) || probes[0].problem.err != nil {
			t.Fatalf("hold-back did not skip preflight: %v", probes)
		}
	})
	t.Run("Should refuse a descriptor for a different computer before a grant", func(t *testing.T) {
		t.Parallel()
		// UT-035. Safety Invariant 2.
		clock := clockwork.NewFakeClock()
		pin, pair := tunnelCertificates(t, clock.Now())
		server := serveProbeTunnel(t, remoteproto.Descriptor{
			ComputerID: "cmp_other", TLSRootSHA256: pin, Protocol: remoteproto.CurrentProtocol,
		}, pair, clock, false, func() bool { return true })
		route := store.ComputerRoute{ID: "wrong", Kind: remoteproto.RouteDirect, Address: probeURL(t, server.URL).Host}
		cloud := &routingCloud{}
		s := supervisor{
			scope:    &registryScope{registry: &Registry{clock: clock, cloud: cloud, routeHTTP: routeClient}},
			ctx:      t.Context(),
			computer: store.Computer{ID: probeComputer, TLSRootSHA: pin},
			errors:   make(map[string]string),
		}
		link, problem := s.connect(s.probe([]store.ComputerRoute{route}))
		if link != nil || !errors.Is(problem.err, ErrIdentityMismatch) || cloud.grants.Load() != 0 {
			t.Fatalf("wrong descriptor admitted: error = %v, grants = %d", problem.err, cloud.grants.Load())
		}
	})
	t.Run("Should report a connected SSH host with lingering off (UT-047)", func(t *testing.T) {
		t.Parallel()
		clock := clockwork.NewFakeClock()
		pin, pair := tunnelCertificates(t, clock.Now())
		host := serveProbeTunnel(t, remoteproto.Descriptor{
			ComputerID: probeComputer, TLSRootSHA256: pin, Protocol: remoteproto.CurrentProtocol,
		}, pair, clock, false, func() bool { return true })
		route := store.ComputerRoute{ID: "ssh", Kind: remoteproto.RouteSSH, SSH: &store.ComputerSSHTarget{
			Alias: "devbox", User: "pedro", ServiceWarning: "linger_off",
		}}
		rows := []store.Computer{{
			ID: probeComputer, AccountID: probeAccount, Enabled: true, TLSRootSHA: pin,
			Routes: []store.ComputerRoute{route},
		}}
		address := probeURL(t, host.URL).Host
		registry := New(
			t.Context(),
			&routingStore{computers: rows},
			newRoutingAccount(t),
			&routingCloud{},
			WithClock(clock),
			WithSSHForward(func(context.Context, store.ComputerRoute) (string, error) { return address, nil }),
		)
		t.Cleanup(registry.Close)
		testutil.Eventually(t, "SSH connected with a lingering diagnosis", 5*time.Second, func() bool {
			view, err := registry.Get(t.Context(), probeComputer)
			return err == nil && view.RouteInUse != nil && view.Status == NeedsAttention && view.Diagnosis != nil &&
				view.Diagnosis.Code == "linger_off" && strings.Contains(view.Diagnosis.Message, "sudo loginctl enable-linger pedro")
		})
	})
}

func testRouteHTTP(hosted *http.Client) func(remoteproto.RouteKind) *http.Client {
	return func(kind remoteproto.RouteKind) *http.Client {
		if kind == remoteproto.RouteCloud {
			return hosted
		}
		return routeClient(kind)
	}
}

func TestComputerDiagnosis(t *testing.T) {
	t.Parallel()
	// UT-036. Safety Invariant 4.
	for _, test := range []struct {
		name string
		err  error
		want Status
	}{
		{"Should diagnose an expired entitlement as needing attention", &Failure{Code: CodeLeaseExpired}, NeedsAttention},
		{"Should show needs Pro when the entitlement explicitly requires a plan", ErrPlanRequired, NeedsPro},
		{"Should show incompatible when protocol ranges do not overlap (UT-051)", ErrIncompatible, Incompatible},
		{"Should show disabled when local use is switched off", ErrDisabled, Disabled},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			status, diagnostic := diagnose(activeFailure(test.err), time.Unix(1, 0))
			if status != test.want || diagnostic.Code != test.err.Error() {
				t.Fatalf("status and diagnosis = %s %v, want %s %s", status, diagnostic, test.want, test.err)
			}
		})
	}
	t.Run("Should find the shared wire version for overlapping peers (UT-051)", func(t *testing.T) {
		t.Parallel()
		controller := remoteproto.ProtocolRange{Min: 1, Max: 1}
		host := remoteproto.ProtocolRange{Min: 2, Max: 2}
		if _, ok := controller.Overlap(host); ok {
			t.Fatal("disjoint ranges admitted a shared protocol")
		}
		controller.Max, host.Max = 2, 3
		if shared, ok := controller.Overlap(host); !ok || shared != (remoteproto.ProtocolRange{Min: 2, Max: 2}) {
			t.Fatalf("negotiated range = %+v, compatible = %t", shared, ok)
		}
	})
}

func TestRemoveSSHComputer(t *testing.T) {
	t.Parallel()
	t.Run("Should remove the computer when SSH uninstall fails (UT-050)", func(t *testing.T) {
		t.Parallel()
		route := store.ComputerRoute{ID: "ssh", Kind: remoteproto.RouteSSH, Address: "pedro@devbox:22"}
		rows := []store.Computer{{ID: probeComputer, AccountID: probeAccount, Routes: []store.ComputerRoute{route}}}
		storage := new(removalStore)
		storage.computers = rows
		var tried string
		registry := New(t.Context(), storage, newRoutingAccount(t), &routingCloud{},
			WithSSHUninstall(func(_ context.Context, route store.ComputerRoute) error {
				tried = route.Address
				return errors.New("host unreachable")
			}))
		t.Cleanup(registry.Close)
		uninstall, err := registry.Remove(t.Context(), probeComputer, RemoveOptions{Uninstall: true})
		if err != nil || uninstall != UninstallFailed || tried != route.Address || !storage.removed {
			t.Fatalf(
				"remove = %v, uninstall = %q, attempted = %q, removed = %t",
				err,
				uninstall,
				tried,
				storage.removed,
			)
		}
		if _, err := registry.Get(t.Context(), probeComputer); !errors.Is(err, ErrNotFound) {
			t.Fatalf("removed computer lookup = %v", err)
		}
	})
}

type removalStore struct {
	routingStore
	removed bool
}

func (s *removalStore) RemoveComputer(_ context.Context, owner string, id remoteproto.ComputerID) error {
	s.removed = owner == probeAccount && id == probeComputer
	return nil
}

func waitUnitRoute(t *testing.T, registry *Registry, kind remoteproto.RouteKind) {
	t.Helper()
	testutil.Eventually(t, "controller route "+string(kind), 5*time.Second, func() bool {
		view, err := registry.Get(t.Context(), probeComputer)
		return err == nil && view.Status == Connected && view.RouteInUse != nil && view.RouteInUse.Kind == kind
	})
}

type routingStore struct {
	store.ComputerStore
	computers []store.Computer
}

func (s *routingStore) ControllerComputers(context.Context, string) ([]store.Computer, error) {
	return s.computers, nil
}
func (*routingStore) RecordComputerConnection(context.Context, string, remoteproto.Descriptor, time.Time) error {
	return nil
}

type routingCloud struct {
	Cloud
	grants atomic.Int32
}

func (*routingCloud) Computers(context.Context, cloudclient.Credentials) ([]remoteproto.AccountComputer, error) {
	return nil, nil
}

func (c *routingCloud) ComputerGrant(
	context.Context, cloudclient.Credentials, remoteproto.ComputerID, remoteproto.RouteKind,
) (string, error) {
	c.grants.Add(1)
	return "fixture-grant", nil
}

type routingAccount struct {
	lifetime context.Context
	key      *ecdsa.PrivateKey
}

func newRoutingAccount(t *testing.T) routingAccount {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return routingAccount{lifetime: t.Context(), key: key}
}

func (a routingAccount) IdentityLifetime() (context.Context, account.Installation) {
	return a.lifetime, account.Installation{State: account.SignedIn, AccountID: probeAccount}
}

func (a routingAccount) UseDevice(ctx context.Context, run func(context.Context, cloudclient.Credentials) error) error {
	return run(ctx, cloudclient.Credentials{Key: a.key})
}

func probeURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
