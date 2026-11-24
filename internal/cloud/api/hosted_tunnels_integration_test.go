// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration

package api_test

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const hostedFixtureEmail = "hosted-owner@example.test"

func newHostedAuthFixture(t *testing.T) (authFixture, *hostedFakeCloudflare) {
	t.Helper()
	fake := newHostedFakeCloudflare(t)
	f := newAuthFixtureWithReplica(
		t,
		func(t *testing.T, cfg cloud.Config, register func(*cloud.App)) *remotetest.Replica {
			cfg.HostedTunnels = fake.client
			return remotetest.StartLocalReplica(t, cfg, register)
		},
	)
	t.Setenv("COMPOZY_CLOUD_URL", f.replica.Server.URL)
	return f, fake
}

func giveHostedPro(t *testing.T, f authFixture, account string) {
	t.Helper()
	f.db.Exec(t, `INSERT INTO %s.subscriptions (id, source, account_id, status, ends_at)
		VALUES ($1, 'complimentary', $1, 'active', $2)`, account, f.clock.Now().Add(time.Hour))
}

type hostedCloudTest struct {
	f       authFixture
	fake    *hostedFakeCloudflare
	owner   remoteTestDevice
	client  *cloudclient.Client
	service *control.Service
}

func newHostedCloudTest(t *testing.T) hostedCloudTest {
	t.Helper()
	f, fake := newHostedAuthFixture(t)
	owner := enrollRemoteDevice(t, f, hostedFixtureEmail, "hosted-lifecycle-owner")
	giveHostedPro(t, f, owner.account)
	client, err := cloudclient.New(cloudclient.WithClock(f.clock.Now))
	if err != nil {
		t.Fatal(err)
	}
	service := control.NewService(f.replica.App.Store, control.ServiceConfig{
		HostedTunnels: fake.client, Now: f.clock.Now,
	})
	return hostedCloudTest{f: f, fake: fake, owner: owner, client: client, service: service}
}

type hostedComputer struct {
	id  remoteproto.ComputerID
	key ed25519.PrivateKey
}

func (h hostedCloudTest) computer(t *testing.T, index int) hostedComputer {
	t.Helper()
	id := remoteproto.ComputerID(fmt.Sprintf("cmp_%04xtest", index))
	return hostedComputer{id: id, key: h.f.db.SeedControlComputer(t, h.owner.account, id, h.f.clock.Now())}
}

func (h hostedCloudTest) provision(t *testing.T, computer hostedComputer) remoteproto.HostedTunnel {
	t.Helper()
	tunnel, err := h.client.ProvisionHostedTunnel(t.Context(), computer.id, computer.key, "")
	if err != nil || tunnel.Hostname == "" || tunnel.ConnectorToken == "" {
		t.Fatalf("provision %s: hostname = %q, error = %v", computer.id, tunnel.Hostname, err)
	}
	return tunnel
}

func (h hostedCloudTest) state(t *testing.T, computer hostedComputer, want string) store.TunnelRef {
	t.Helper()
	ref := store.TunnelRef{ComputerID: computer.id}
	var state string
	err := h.f.conn.QueryRow(t.Context(), "SELECT generation, state FROM tunnels WHERE computer_id = $1", computer.id).
		Scan(&ref.Generation, &state)
	if err != nil || state != want {
		t.Fatalf("tunnel %s state = %q, want %q: %v", computer.id, state, want, err)
	}
	return ref
}

func TestHostedTunnelLifecycle(t *testing.T) {
	t.Run("Should fence an old deletion after allocating the same hostname again", func(t *testing.T) {
		// UT-074. Safety Invariant 9.
		h := newHostedCloudTest(t)
		computer := h.computer(t, 1)
		first := h.provision(t, computer)
		var prior store.TunnelRef
		for range 3 {
			prior = h.state(t, computer, "active")
			if err := h.client.DeleteHostedTunnel(t.Context(), computer.id, computer.key); err != nil {
				t.Fatal(err)
			}
			h.state(t, computer, "deleted")
			next := h.provision(t, computer)
			if next.Hostname != first.Hostname {
				t.Fatalf("reprovision changed %q to %q", first.Hostname, next.Hostname)
			}
		}
		if err := h.service.DeleteHostedGeneration(t.Context(), prior); err != nil {
			t.Fatal(err)
		}
		current := h.state(t, computer, "active")
		created, deleted := h.fake.counts()
		if prior.Generation != 3 || current.Generation != 4 || created != 4 || deleted != 3 ||
			h.fake.target(first.Hostname).Origin == "" {
			t.Fatalf("stale deletion changed the replacement: generations %d/%d, provider creates/deletes %d/%d",
				prior.Generation, current.Generation, created, deleted)
		}
	})

	t.Run("Should count held slots and honor the account's raised cap", func(t *testing.T) {
		// UT-075 and UT-143. Safety Invariant 7.
		h := newHostedCloudTest(t)
		states := []string{"reserved", "provisioning", "active"}
		for index := range 10 {
			computer := h.computer(t, index)
			h.f.db.Exec(t, `INSERT INTO %s.tunnels (computer_id, hostname, state, created_at)
				VALUES ($1, $2, $3, $4)`, computer.id,
				fmt.Sprintf("seed-%04d.compozy.run", index), states[index%len(states)], h.f.clock.Now())
		}
		failed := h.computer(t, 20)
		h.f.db.Exec(t, `INSERT INTO %s.tunnels (computer_id, hostname, state, created_at)
			VALUES ($1, 'failed-0020.compozy.run', 'failed', $2)`, failed.id, h.f.clock.Now())
		eleventh, twelfth, thirteenth := h.computer(t, 10), h.computer(t, 11), h.computer(t, 12)
		_, err := h.client.ProvisionHostedTunnel(t.Context(), eleventh.id, eleventh.key, "")
		expectHostedRefusal(t, err, http.StatusPaymentRequired, "cloud_routes_limit_reached")
		h.f.db.Exec(t, "UPDATE %s.accounts SET cloud_routes_max = 12 WHERE id = $1", h.owner.account)
		h.provision(t, eleventh)
		h.provision(t, twelfth)
		_, err = h.client.ProvisionHostedTunnel(t.Context(), thirteenth.id, thirteenth.key, "")
		expectHostedRefusal(t, err, http.StatusPaymentRequired, "cloud_routes_limit_reached")
		if created, _ := h.fake.counts(); created != 2 {
			t.Fatalf("Cloudflare creates = %d, want 2", created)
		}
	})

	t.Run("Should sweep expired or idle tunnels and immediately delete an unlinked tunnel", func(t *testing.T) {
		// IT-076. Safety Invariants 7 and 9.
		h := newHostedCloudTest(t)
		var active []hostedComputer
		for index := range 9 {
			computer := h.computer(t, index+1)
			h.provision(t, computer)
			active = append(active, computer)
		}
		h.competeForFinalSlot(t)
		expired, idle, online := active[0], active[1], active[2]
		now := h.f.clock.Now()
		h.f.db.Exec(
			t,
			"UPDATE %s.tunnels SET delete_after = $2 WHERE computer_id = $1",
			expired.id,
			now.Add(-time.Second),
		)
		h.f.db.Exec(t, "UPDATE %s.tunnels SET created_at = $2 WHERE computer_id IN ($1, $3)",
			idle.id, now.Add(-15*24*time.Hour), online.id)
		h.f.db.Exec(t, "UPDATE %s.computers SET last_control_at = $2 WHERE id = $1", online.id, now)
		if err := h.service.SweepHostedTunnels(t.Context()); err != nil {
			t.Fatal(err)
		}
		h.state(t, expired, "deleted")
		h.state(t, idle, "deleted")
		h.state(t, online, "active")
		remoteCloudRequest(t, h.f, h.owner, http.MethodDelete, "/v1/computers/"+string(online.id), nil).
			expect(t, http.StatusNoContent, "")
		h.state(t, online, "deleted")
		if created, deleted := h.fake.counts(); created != 10 || deleted != 3 {
			t.Fatalf("Cloudflare creates/deletes = %d/%d, want 10/3", created, deleted)
		}
	})

	t.Run("Should return retained credentials after Pro expires and decline suspended accounts", func(t *testing.T) {
		// Safety Invariant 34. The retained tunnel never consumes a new provider allocation.
		h := newHostedCloudTest(t)
		computer := h.computer(t, 1)
		first := h.provision(t, computer)
		now := h.f.clock.Now()
		h.f.db.Exec(t, "UPDATE %s.subscriptions SET ends_at = $2 WHERE account_id = $1", h.owner.account, now)
		h.f.db.Exec(
			t,
			"UPDATE %s.tunnels SET delete_after = $2 WHERE computer_id = $1",
			computer.id,
			now.Add(7*24*time.Hour),
		)
		retained := h.provision(t, computer)
		if retained.Hostname != first.Hostname || retained.Mode != remoteproto.HostedTunnelEntitlementOnly {
			t.Fatalf("retained route = %q in %q, want %q in entitlement-only mode",
				retained.Hostname, retained.Mode, first.Hostname)
		}
		other := h.computer(t, 2)
		_, err := h.client.ProvisionHostedTunnel(t.Context(), other.id, other.key, "")
		expectHostedRefusal(t, err, http.StatusPaymentRequired, "plan_required")
		h.f.db.Exec(t, "UPDATE %s.accounts SET state = 'suspended' WHERE id = $1", h.owner.account)
		_, err = h.client.ProvisionHostedTunnel(t.Context(), computer.id, computer.key, "")
		expectHostedRefusal(t, err, http.StatusForbidden, "account_suspended")
		if created, _ := h.fake.counts(); created != 1 {
			t.Fatalf("Cloudflare creates = %d, want 1", created)
		}
	})
}

func (h hostedCloudTest) competeForFinalSlot(t *testing.T) {
	t.Helper()
	start, results := make(chan struct{}), make(chan error, 2)
	for _, computer := range []hostedComputer{h.computer(t, 30), h.computer(t, 31)} {
		go func() {
			<-start
			_, err := h.client.ProvisionHostedTunnel(t.Context(), computer.id, computer.key, "")
			results <- err
		}()
	}
	close(start)
	first, second := <-results, <-results
	if first != nil {
		first, second = second, first
	}
	if first != nil {
		t.Fatalf("neither final slot contender was admitted: %v, %v", first, second)
	}
	expectHostedRefusal(t, second, http.StatusPaymentRequired, "cloud_routes_limit_reached")
	if created, _ := h.fake.counts(); created != 10 {
		t.Fatalf("final slot race produced %d total creates, want 10", created)
	}
}

func expectHostedRefusal(t *testing.T, err error, status int, code string) {
	t.Helper()
	failure, ok := errors.AsType[*cloudclient.Failure](err)
	if !ok || failure.Status != status || failure.Code != code {
		t.Fatalf("hosted request error = %v, want %d %s", err, status, code)
	}
}
