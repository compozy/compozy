//go:build integration && (darwin || linux)

package api_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/sse"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestDaemonBilling(t *testing.T) {
	root := t.TempDir()

	t.Run(
		"Should receive checkout completion over the account stream with hosting disabled (IT-064)",
		func(t *testing.T) {
			f, stripe := newBillingFixture(t, nil)
			d, _ := startDaemonAccountFixture(t, f, filepath.Join(root, "ctrl"))
			const email = "stream-checkout@example.test"
			progress := d.stream(t, contract.AccountSignInEventsPath, "")
			signInDaemonAccount(t, f, d, progress, email)
			progress.cancel()
			if status := readRemoteDaemonStatus(t, d); status.Enabled || status.Entitlement != nil {
				t.Fatalf("checkout controller unexpectedly hosts remote access: %+v", status)
			}
			d.uds(t, http.MethodPost, contract.AccountCheckoutPath, contract.AccountCheckoutRequest{Interval: "year"}).
				expect(t, http.StatusOK, "checkout.stripe.com")
			events := d.stream(t, accountStreamPath, "")
			defer events.cancel()
			checkouts := stripe.Checkouts()
			if len(checkouts) != 1 {
				t.Fatalf("checkout count = %d", len(checkouts))
			}
			completed := stripe.Complete(checkouts[0].ID)
			deliverStripeEvent(t, f, "evt_checkout_stream", "checkout.session.completed", completed.Sub)
			events.until(t, func(frame sse.Event) bool {
				if frame.Event != "account.changed" {
					return false
				}
				change := decodeDaemonAccountEvent[account.Event](t, frame).Data
				return change.Plan == "pro" && change.PaymentIssue != nil && !*change.PaymentIssue
			})
			d.request(t, http.MethodGet, contract.AccountPath, nil).expect(t, http.StatusOK, `"plan":"pro"`)
			d.uds(t, http.MethodPost, contract.AccountPath+"/billing-portal", nil).
				expect(t, http.StatusOK, "billing.stripe.com")
		},
	)

	t.Run(
		"Should fence hosted access and reconnect the same computers on renewal (IT-062, IT-063, IT-082)",
		func(t *testing.T) {
			// Safety Invariants 4, 6, 8 and 15. Existing leases, a retained hostname and the controller observe the change.
			f, stripe, fake, owner, sub := hostedBillingFixture(t)
			hostHome := filepath.Join(root, "hosted")
			host, _, a := linkBillingHost(t, f, hostHome, true)
			_, _, b := linkBillingHost(t, f, filepath.Join(root, "winner"), false)
			_, _, c := linkBillingHost(t, f, filepath.Join(root, "limited"), false)
			hostname := enableBillingHostedRoute(t, host, a)
			edge := hostedEdgeClient(t, hostHome)
			direct := &http.Client{Timeout: 3 * time.Second}
			t.Cleanup(direct.CloseIdleConnections)
			controller, _ := startComputerController(t, f, filepath.Join(root, "list"), hostedFixtureEmail, nil,
				daemon.WithComputersOptions(computers.WithRouteHTTP(func(kind remoteproto.RouteKind) *http.Client {
					if kind == remoteproto.RouteCloud {
						return edge
					}
					return direct
				})))
			for _, id := range []remoteproto.ComputerID{a, b, c} {
				addAccountComputer(t, controller, id)
			}
			waitComputerRoute(t, controller, a, remoteproto.RouteCloud)
			waitComputerRoute(t, controller, b, remoteproto.RouteDirect)
			waitComputerRoute(t, controller, c, remoteproto.RouteDirect)
			f.db.Exec(
				t,
				`UPDATE %s.computers SET last_session_at = $2 WHERE id = $1`,
				string(b),
				f.clock.Now().Add(time.Second),
			)
			lapseStripe(t, f, stripe, sub)
			assertPlanLimited(t, f, map[remoteproto.ComputerID]bool{a: true, b: false, c: true})
			testutil.Eventually(t, "two previously connected computers are limited", 9*time.Second, func() bool {
				f.clock.Advance(100 * time.Millisecond)
				return readControllerComputer(t, controller, a).Status == string(computers.NeedsPro) &&
					readControllerComputer(t, controller, c).Status == string(computers.NeedsPro)
			})
			waitComputerRoute(t, controller, b, remoteproto.RouteDirect)
			base := "https://" + hostname
			hostedBillingRequest(t, edge, http.MethodPost, base+"/remote/v1/session").
				expect(t, http.StatusPaymentRequired, "plan_required")
			assertRetainedTunnel(t, f, a)

			remoteCloudRequest(
				t,
				f,
				owner,
				http.MethodPost,
				"/v1/billing/checkout",
				remoteproto.CheckoutRequest{Interval: "year"},
			).
				expect(t, http.StatusOK, "checkout.stripe.com")
			checkout := stripe.Complete(stripe.Checkouts()[0].ID)
			deliverStripeEvent(t, f, "evt_resubscribe", "checkout.session.completed", checkout.Sub)
			assertPlanLimited(t, f, map[remoteproto.ComputerID]bool{a: false, b: false, c: false})
			testutil.Eventually(t, "the same computers reconnect on the restored plan", 9*time.Second, func() bool {
				f.clock.Advance(time.Second)
				return readControllerComputer(t, controller, a).Status == string(computers.Connected) &&
					readControllerComputer(t, controller, c).Status == string(computers.Connected)
			})
			waitComputerRoute(t, controller, a, remoteproto.RouteCloud)
			deliverStripeEvent(t, f, "evt_stale_cancellation", stripeSubUpdated, sub.ID)
			lease := readRemoteDaemonStatus(t, host).Entitlement
			if account := billingAccount(t, f, owner); account.Plan != "pro" || lease == nil || lease.Plan != "pro" {
				t.Fatalf(
					"ended subscription removed the replacement's entitlement: %+v, lease %+v",
					account.AccountPlan,
					lease,
				)
			}
			hostedBillingRequest(t, edge, http.MethodGet, base+remoteproto.BrowserStatusPath).
				expect(t, http.StatusOK, `"state":"ready"`)
			if created, deleted := fake.counts(); created != 1 || deleted != 0 {
				t.Fatalf("renewal replaced the retained tunnel: created %d, deleted %d", created, deleted)
			}
		},
	)

	t.Run(
		"Should retain the plan-required hosted shell across a restart until the seven-day deletion (IT-074)",
		func(t *testing.T) {
			// Safety Invariants 4, 9 and 15. The retained generation serves only status and the SPA until it expires.
			f, stripe, fake, _, sub := hostedBillingFixture(t)
			home := filepath.Join(root, "lapse")
			host, ring, id := linkBillingHost(t, f, home, true)
			hostname := enableBillingHostedRoute(t, host, id)
			edge := hostedEdgeClient(t, home)
			base := "https://" + hostname
			prior := waitHostedConnector(t, home, 0)
			lapseStripe(t, f, stripe, sub)
			expectHostedLapse(t, edge, base)
			cutoff := assertRetainedTunnel(t, f, id)
			host.stop()

			f.clock.Advance(24 * time.Hour)
			boot := daemonAccountBoot{keyring: ring, options: []daemon.Option{
				daemon.WithCloudflaredInstaller(hostedFixtureInstaller),
			}}
			host, _ = startConfiguredDaemonAccount(t, f, home, boot)
			restarted := waitHostedConnector(t, home, prior.PID)
			if restarted.Hostname != hostname {
				t.Fatalf("restart received a different hostname: %q", restarted.Hostname)
			}
			expectHostedLapse(t, edge, base)

			f.clock.Advance(cutoff.Sub(f.clock.Now()) + time.Second)
			sweeper := control.NewService(
				f.replica.App.Store,
				control.ServiceConfig{HostedTunnels: fake.client, Now: f.clock.Now},
			)
			if err := sweeper.SweepHostedTunnels(t.Context()); err != nil {
				t.Fatal(err)
			}
			testutil.Eventually(t, "the running daemon stops the expired connector", 6*time.Second, func() bool {
				return readRemoteDaemonStatus(t, host).CloudRoute.Hostname == nil
			})
			host.stop()
			host, _ = startConfiguredDaemonAccount(t, f, home, boot)
			testutil.Eventually(t, "retention expiry refuses a new connector", 6*time.Second, func() bool {
				status := readRemoteDaemonStatus(t, host)
				return status.CloudRoute.State == remoteproto.HostedTunnelEntitlementOnly &&
					status.CloudRoute.Hostname == nil
			})
			assertDeletedTunnelRefused(t, f, ring, id)
			if last, err := readHostedConnector(home); err != nil || last.PID != restarted.PID {
				t.Fatalf("a new connector started after deletion: %+v: %v", last, err)
			}
			var state string
			if err := f.conn.QueryRow(t.Context(), "SELECT state FROM tunnels WHERE computer_id = $1", id).
				Scan(&state); err != nil || state != "deleted" {
				t.Fatalf("tunnel after expiry: %s: %v", state, err)
			}
		},
	)
}

func hostedBillingFixture(t *testing.T) (
	authFixture, *remotetest.StripeFixture, *hostedFakeCloudflare, remoteTestDevice, remotetest.StripeSubscription,
) {
	t.Helper()
	fake := newHostedFakeCloudflare(t)
	f, stripe := newBillingFixture(t, func(cfg *cloud.Config) { cfg.HostedTunnels = fake.client })
	owner := newBillingDevice(t, f, hostedFixtureEmail)
	sub := proStripeSub(f.clock.Now(), "sub_hosted", owner.account)
	stripe.Put(sub)
	deliverStripeEvent(t, f, "evt_hosted_active", stripeSubCreated, sub.ID)
	return f, stripe, fake, owner, sub
}

func assertDeletedTunnelRefused(t *testing.T, f authFixture, ring *daemonAccountKeyring, id remoteproto.ComputerID) {
	t.Helper()
	var raw string
	ring.entries.Range(func(key, value any) bool {
		if strings.HasSuffix(key.([2]string)[1], ":remote-host:computer-key") {
			raw = value.(string)
			return false
		}
		return true
	})
	seed, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil || len(seed) != ed25519.SeedSize {
		t.Fatal("missing fixture computer signing key")
	}
	client, err := cloudclient.New(cloudclient.WithClock(f.clock.Now))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ProvisionHostedTunnel(t.Context(), id, ed25519.NewKeyFromSeed(seed), "")
	expectHostedRefusal(t, err, http.StatusPaymentRequired, "plan_required")
}

func linkBillingHost(
	t *testing.T, f authFixture, home string, hosted bool,
) (daemonAccountFixture, *daemonAccountKeyring, remoteproto.ComputerID) {
	t.Helper()
	boot := daemonAccountBoot{direct: !hosted}
	if hosted {
		installHostedConnectorFixture(t, home)
		configureHostedDaemon(t, home, "")
		boot.options = []daemon.Option{daemon.WithCloudflaredInstaller(hostedFixtureInstaller)}
	}
	host, ring := startConfiguredDaemonAccount(t, f, home, boot)
	progress := host.stream(t, contract.AccountSignInEventsPath, "")
	signInAccountDevice(t, f, host, progress, hostedFixtureEmail)
	progress.cancel()
	reply := host.uds(t, http.MethodPost, contract.RemoteLinkPath, contract.RemoteLinkRequest{})
	reply.expect(t, http.StatusOK, "computer_id")
	waitForHostLease(t, host)
	return host, ring, authJSON[contract.RemoteLinkResponse](t, reply).ComputerID
}

func enableBillingHostedRoute(t *testing.T, host daemonAccountFixture, id remoteproto.ComputerID) string {
	t.Helper()
	host.uds(t, http.MethodPatch, contract.RemotePath, contract.RemotePatchRequest{CloudRoute: new(true)}).
		expect(t, http.StatusOK, string(id))
	return *waitHostedRouteReady(t, host).CloudRoute.Hostname
}

func lapseStripe(t *testing.T, f authFixture, stripe *remotetest.StripeFixture, sub remotetest.StripeSubscription) {
	t.Helper()
	sub.Status = "canceled"
	stripe.Put(sub)
	deliverStripeEvent(t, f, "evt_hosted_lapsed", stripeSubDeleted, sub.ID)
}

func assertRetainedTunnel(t *testing.T, f authFixture, id remoteproto.ComputerID) time.Time {
	t.Helper()
	var state string
	var cutoff time.Time
	if err := f.conn.QueryRow(t.Context(), "SELECT state, delete_after FROM tunnels WHERE computer_id = $1", id).
		Scan(&state, &cutoff); err != nil || state != "active" {
		t.Fatalf("downgrade did not retain the active tunnel: %s: %v", state, err)
	}
	return cutoff
}

func expectHostedLapse(t *testing.T, client *http.Client, base string) {
	t.Helper()
	testutil.Eventually(t, "hosted listener enters entitlement-only mode", 8*time.Second, func() bool {
		reply, err := readHostedBilling(t.Context(), client, http.MethodGet, base+remoteproto.BrowserStatusPath)
		return err == nil && reply.StatusCode == http.StatusOK && bytes.Contains(reply.body, []byte("plan_required"))
	})
	hostedBillingRequest(t, client, http.MethodGet, base+"/").expect(t, http.StatusOK, "<html")
	hostedBillingRequest(t, client, http.MethodGet, base+"/api/sessions").
		expect(t, http.StatusPaymentRequired, "plan_required")
	hostedBillingRequest(t, client, http.MethodPost, base+"/api/webhooks/global/test--webhook").
		expect(t, http.StatusPaymentRequired, "plan_required")
}

func hostedBillingRequest(t *testing.T, client *http.Client, method, target string) authReply {
	t.Helper()
	reply, err := readHostedBilling(t.Context(), client, method, target)
	if err != nil {
		t.Fatal(err)
	}
	return reply
}

func readHostedBilling(ctx context.Context, client *http.Client, method, target string) (authReply, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, http.NoBody)
	if err != nil {
		return authReply{}, err
	}
	result, err := client.Do(req)
	if err != nil {
		return authReply{}, err
	}
	raw, err := io.ReadAll(result.Body)
	return authReply{Response: result, body: raw}, errors.Join(err, result.Body.Close())
}
