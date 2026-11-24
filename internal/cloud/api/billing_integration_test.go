//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	stripeWebhookPath = "/v1/billing/stripe-webhook"
	cloudAccountPath  = "/v1/account"
	stripeSubUpdated  = "customer.subscription.updated"
	stripeSubDeleted  = "customer.subscription.deleted"
	stripeSubCreated  = "customer.subscription.created"
)

func newBillingFixture(t *testing.T, configure func(*cloud.Config)) (authFixture, *remotetest.StripeFixture) {
	t.Helper()
	var stripe *remotetest.StripeFixture
	f := newAuthFixtureWithReplica(
		t,
		func(t *testing.T, cfg cloud.Config, register func(*cloud.App)) *remotetest.Replica {
			stripe = remotetest.StartStripe(t, cfg.Now, false)
			configureBilling(&cfg, stripe)
			if configure != nil {
				configure(&cfg)
			}
			if cfg.Issuer == remoteproto.CloudIssuer {
				return remotetest.StartReplica(t, cfg, register)
			}
			return remotetest.StartLocalReplica(t, cfg, register)
		},
	)
	configureBilling(&f.cfg, stripe)
	return f, stripe
}

func configureBilling(cfg *cloud.Config, stripe *remotetest.StripeFixture) {
	cfg.Stripe = stripe.Client
	cfg.MonthlyPrice, cfg.AnnualPrice = remotetest.StripeMonthly, remotetest.StripeAnnual
	cfg.Auth.MonthlyPriceID, cfg.Auth.AnnualPriceID = cfg.MonthlyPrice, cfg.AnnualPrice
}

func proStripeSub(now time.Time, id, account string) remotetest.StripeSubscription {
	return remotetest.StripeSubscription{ID: id, Account: account, Status: "active",
		Price: remotetest.StripeAnnual, Interval: "year", PeriodEnd: now.Add(365 * 24 * time.Hour)}
}

func newBillingDevice(t *testing.T, f authFixture, email string) remoteTestDevice {
	t.Helper()
	token := newEnrollment(t, f, auth.Google, providerProfile{email: email, subject: email, verified: true}, false)
	key, jwk := newDeviceKey(t)
	proof, err := remoteproto.BuildDPoP(key, http.MethodPost, f.replica.App.Issuer+enrollPath, "", f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"Authorization": "DPoP " + token.EnrollmentToken, "DPoP": proof}
	reply := callAuth(t, f.replica, http.MethodPost, enrollPath, enrollInput(jwk), nil, headers)
	reply.expect(t, http.StatusOK, "device_id")
	result := authJSON[remoteproto.DeviceEnrollResponse](t, reply)
	return remoteTestDevice{key, result.AccessToken, result.DeviceID, token.AccountID}
}

func stripeEventBody(event, kind, id string) []byte {
	switch kind {
	case "invoice.paid", "invoice.payment_failed":
		return fmt.Appendf(
			nil,
			`{"id":%q,"type":%q,"data":{"object":{"parent":{"subscription_details":{"subscription":%q}}}}}`,
			event,
			kind,
			id,
		)
	case "checkout.session.completed":
		return fmt.Appendf(nil, `{"id":%q,"type":%q,"data":{"object":{"subscription":%q}}}`, event, kind, id)
	default:
		return fmt.Appendf(nil, `{"id":%q,"type":%q,"data":{"object":{"id":%q,"status":"active"}}}`, event, kind, id)
	}
}

func sendStripeEvent(
	ctx context.Context, replica *remotetest.Replica, now time.Time, body []byte,
) (int, string, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		replica.Server.URL+stripeWebhookPath,
		bytes.NewReader(body),
	)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Stripe-Signature", remotetest.StripeSignature(body, now))
	response, err := replica.Server.Client().Do(req)
	if err != nil {
		return 0, "", err
	}
	result, err := io.ReadAll(response.Body)
	return response.StatusCode, string(result), errors.Join(err, response.Body.Close())
}

func deliverStripeEvent(t *testing.T, f authFixture, event, kind, id string) {
	t.Helper()
	status, body, err := sendStripeEvent(t.Context(), f.replica, f.clock.Now(), stripeEventBody(event, kind, id))
	if err != nil || status != http.StatusNoContent || body != "" {
		t.Fatalf("Stripe event %s: HTTP %d %s: %v", event, status, body, err)
	}
}

func billingAccount(t *testing.T, f authFixture, device remoteTestDevice) remoteproto.AccountSummary {
	t.Helper()
	status := remoteCloudRequest(t, f, device, http.MethodGet, cloudAccountPath, nil)
	status.expect(t, http.StatusOK, "plan")
	return authJSON[remoteproto.AccountSummary](t, status)
}

func TestCloudSubscriptionRefresh(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should derive Pro from the current signed webhook trigger and deduplicate it (UT-060, UT-062)",
		func(t *testing.T) {
			// Safety Invariant 8. The same event is fetched and committed once, only after signature verification.
			t.Parallel()
			f, stripe := newBillingFixture(t, nil)
			owner := newBillingDevice(t, f, "billing@example.test")
			sub := proStripeSub(f.clock.Now(), "sub_first", owner.account)
			stripe.Put(sub)
			body := stripeEventBody("evt_create", stripeSubCreated, sub.ID)
			status, errorBody, err := sendStripeEvent(t.Context(), f.replica, f.clock.Now().Add(-6*time.Minute), body)
			if err != nil || status != http.StatusBadRequest ||
				!strings.Contains(errorBody, "stripe_signature_invalid") {
				t.Fatalf("stale signature: HTTP %d %s: %v", status, errorBody, err)
			}
			for range 2 {
				deliverStripeEvent(t, f, "evt_create", stripeSubCreated, sub.ID)
			}
			account := billingAccount(t, f, owner)
			if account.Plan != "pro" || account.Subscription == nil ||
				account.Subscription.CurrentPeriodEnd == nil || !account.Subscription.CurrentPeriodEnd.Equal(sub.PeriodEnd) ||
				stripe.Reads(sub.ID) != 1 {
				t.Fatalf(
					"billing after two deliveries: %+v, subscription reads %d",
					account.AccountPlan,
					stripe.Reads(sub.ID),
				)
			}
			assertAuthRow(t, f, "SELECT count(*) FROM stripe_events WHERE event_id = 'evt_create'", 1)
		},
	)

	t.Run(
		"Should retain Pro during dunning and clear it after final cancellation (UT-061, IT-065)",
		func(t *testing.T) {
			// Safety Invariant 8. A payment webhook supplies only the subscription reference.
			t.Parallel()
			f, stripe := newBillingFixture(t, nil)
			owner := newBillingDevice(t, f, "dunning@example.test")
			sub := proStripeSub(f.clock.Now(), "sub_dunning", owner.account)
			for _, step := range []struct {
				status, kind, plan string
				paymentIssue       bool
			}{
				{"past_due", "invoice.payment_failed", "pro", true},
				{"active", "invoice.paid", "pro", false},
				{"past_due", "invoice.payment_failed", "pro", true},
				{"canceled", stripeSubDeleted, "free", false},
			} {
				sub.Status = step.status
				stripe.Put(sub)
				deliverStripeEvent(t, f, fmt.Sprintf("evt_%d", stripe.Reads(sub.ID)), step.kind, sub.ID)
				account := billingAccount(t, f, owner)
				if account.Plan != step.plan || account.PaymentIssue != step.paymentIssue {
					t.Fatalf("%s => %+v", step.status, account.AccountPlan)
				}
			}
		},
	)

	t.Run(
		"Should use the shorter current period and preserve entitlement from a second row (UT-063, UT-113)",
		func(t *testing.T) {
			// Safety Invariant 8. An old active event cannot revive an ended subscription or cancel its replacement.
			t.Parallel()
			f, stripe := newBillingFixture(t, nil)
			owner := newBillingDevice(t, f, "renew@example.test")
			first := proStripeSub(f.clock.Now(), "sub_old", owner.account)
			stripe.Put(first)
			deliverStripeEvent(t, f, "evt_initial", stripeSubCreated, first.ID)
			first.PeriodEnd = first.PeriodEnd.Add(-6 * time.Hour)
			stripe.Put(first)
			deliverStripeEvent(t, f, "evt_refund", stripeSubUpdated, first.ID)
			account := billingAccount(t, f, owner)
			if account.Subscription == nil || account.Subscription.CurrentPeriodEnd == nil ||
				!account.Subscription.CurrentPeriodEnd.Equal(first.PeriodEnd) {
				t.Fatalf("shortened subscription was not applied: %+v", account.Subscription)
			}
			first.Status = "canceled"
			stripe.Put(first)
			deliverStripeEvent(t, f, "evt_end", stripeSubDeleted, first.ID)
			deliverStripeEvent(t, f, "evt_old", stripeSubUpdated, first.ID)
			if account := billingAccount(t, f, owner); account.Plan != "free" || account.Subscription != nil {
				t.Fatalf("an old event restored the canceled subscription: %+v", account.AccountPlan)
			}
			second := proStripeSub(f.clock.Now(), "sub_replacement", owner.account)
			stripe.Put(second)
			deliverStripeEvent(t, f, "evt_replacement", stripeSubCreated, second.ID)
			deliverStripeEvent(t, f, "evt_older", stripeSubUpdated, first.ID)
			if account := billingAccount(t, f, owner); account.Plan != "pro" {
				t.Fatalf("replacement did not preserve Pro: %+v", account.AccountPlan)
			}
			second.Status = "canceled"
			stripe.Put(second)
			deliverStripeEvent(t, f, "evt_end_replacement", stripeSubDeleted, second.ID)
			if account := billingAccount(t, f, owner); account.Plan != "free" {
				t.Fatalf("both subscriptions ended: %+v", account.AccountPlan)
			}
		},
	)
}

func TestCloudBillingCheckout(t *testing.T) {
	t.Parallel()

	t.Run("Should pass server-owned prices and leave an expired checkout Free (UT-137, IT-061)", func(t *testing.T) {
		// Safety Invariant 34. A suspended account cannot issue a new Stripe checkout.
		t.Parallel()
		f, stripe := newBillingFixture(t, nil)
		owner := newBillingDevice(t, f, "checkout@example.test")
		path := "/v1/billing/checkout"
		for _, interval := range []string{"month", "year"} {
			response := remoteCloudRequest(
				t,
				f,
				owner,
				http.MethodPost,
				path,
				remoteproto.CheckoutRequest{Interval: interval},
			)
			response.expect(t, http.StatusOK, "checkout.stripe.com")
		}
		checkouts := stripe.Checkouts()
		if len(checkouts) != 2 {
			t.Fatalf("checkouts = %d", len(checkouts))
		}
		for i, price := range []string{remotetest.StripeMonthly, remotetest.StripeAnnual} {
			form := checkouts[i].Form
			if form.Get("line_items[0][price]") != price || form.Get("client_reference_id") != owner.account ||
				form.Get("managed_payments[enabled]") != "true" || form.Get("allow_promotion_codes") != "true" {
				t.Fatalf("invalid account or price on Stripe checkout %d", i)
			}
			stripe.Expire(checkouts[i].ID)
		}
		remoteCloudRequest(
			t,
			f,
			owner,
			http.MethodPost,
			path,
			map[string]string{"interval": "year", "price_id": "wrong"},
		).
			expect(t, http.StatusBadRequest, "invalid_request")
		status := billingAccount(t, f, owner)
		if status.Plan != "free" || status.Subscription != nil {
			t.Fatalf("expiry produced an entitlement: %+v", status.AccountPlan)
		}
		f.db.Exec(t, "UPDATE %s.accounts SET state = 'suspended' WHERE id = $1", owner.account)
		remoteCloudRequest(t, f, owner, http.MethodPost, path, remoteproto.CheckoutRequest{Interval: "year"}).
			expect(t, http.StatusForbidden, "account_suspended")
		if len(stripe.Checkouts()) != 2 {
			t.Fatal("invalid or suspended account created a checkout")
		}
	})
}

func TestStripeReplicaOrdering(t *testing.T) {
	t.Parallel()
	for _, second := range []string{"webhook", "reconciliation"} {
		t.Run("Should serialize a later "+second+" across replicas (IT-066)", func(t *testing.T) {
			// Safety Invariant 8. Assert Postgres sees the second replica blocked on the advisory lock.
			t.Parallel()
			f, stripe := newBillingFixture(t, nil)
			owner := newBillingDevice(t, f, "replica@example.test")
			sub := proStripeSub(f.clock.Now(), "sub_ordered", owner.account)
			stripe.Put(sub)
			deliverStripeEvent(t, f, "evt_seed", stripeSubCreated, sub.ID)
			secondConfig := f.cfg
			secondConfig.Postgres = f.db.Config(t, "billing2")
			replica := remotetest.StartLocalReplica(t, secondConfig, nil)
			fetched, release := make(chan struct{}), make(chan struct{})
			unblock := sync.OnceFunc(func() { close(release) })
			defer unblock()
			stripe.SetReadBarrier(func(r *http.Request, snapshot remotetest.StripeSubscription) {
				if snapshot.Status == "active" {
					close(fetched)
					select {
					case <-release:
					case <-r.Context().Done():
					}
				}
			})
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			start := func(target *remotetest.Replica, event string) <-chan error {
				outcome := make(chan error, 1)
				go func() {
					status, body, err := sendStripeEvent(
						ctx,
						target,
						f.clock.Now(),
						stripeEventBody(event, stripeSubUpdated, sub.ID),
					)
					if err == nil && status != http.StatusNoContent {
						err = fmt.Errorf("HTTP %d %s", status, body)
					}
					outcome <- err
				}()
				return outcome
			}
			a := start(f.replica, "evt_a")
			select {
			case <-fetched:
			case <-ctx.Done():
				t.Fatal("first Stripe fetch did not start")
			}
			sub.Status = "canceled"
			stripe.Put(sub)
			var b <-chan error
			if second == "webhook" {
				b = start(replica, "evt_b")
			} else {
				reconciled := make(chan error, 1)
				go func() { reconciled <- replica.App.Billing.Reconcile(ctx) }()
				b = reconciled
			}
			testutil.Eventually(t, "second replica waits for the subscription lock", 3*time.Second, func() bool {
				var waiting bool
				err := f.conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity
					WHERE application_name = $1 AND wait_event = 'advisory')`,
					f.db.ApplicationName("billing2", "pooled")).Scan(&waiting)
				return err == nil && waiting
			})
			if reads := stripe.Reads(sub.ID); reads != 2 {
				t.Fatalf("second replica fetched before the lock was released (%d reads)", reads)
			}
			unblock()
			for _, jobs := range []<-chan error{a, b} {
				select {
				case err := <-jobs:
					if err != nil {
						t.Fatal(err)
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			if account := billingAccount(t, f, owner); account.Plan != "free" {
				t.Fatalf("final checkout plan = %s", account.Plan)
			}
		})
	}
}

func TestStripeLeaseUpdate(t *testing.T) {
	t.Parallel()
	t.Run("Should push a cloud lease as soon as the checkout changes plan (IT-060)", func(t *testing.T) {
		// Safety Invariants 4, 6 and 8. A signed lease changes only after an entitled row is committed.
		t.Parallel()
		f, stripe := newBillingFixture(t, func(cfg *cloud.Config) { cfg.Issuer = remoteproto.CloudIssuer })
		owner := newBillingDevice(t, f, "lease@example.test")
		computer := remoteproto.ComputerID("cmp_billing_lease")
		key := f.db.SeedControlComputer(t, owner.account, computer, f.clock.Now())
		link := remotetest.AuthenticateControl(t, f.replica, computer, key)
		snapshot := remotetest.ReadControlMessage(t, link, 2*time.Second)
		if snapshot.Type != remoteproto.ControlSnapshot {
			t.Fatalf("first frame = %s, want snapshot", snapshot.Type)
		}
		remotetest.SendControl(
			t,
			link,
			remoteproto.ControlAck,
			remoteproto.SecurityAck{SecurityEpoch: snapshot.SecurityEpoch},
		)
		first := remotetest.ReadControlMessage(t, link, 2*time.Second)
		if first.Type != remoteproto.ControlLease {
			t.Fatalf("first lease frame = %s", first.Type)
		}
		sub := proStripeSub(f.clock.Now(), "sub_lease", owner.account)
		stripe.Put(sub)
		deliverStripeEvent(t, f, "evt_lease", stripeSubCreated, sub.ID)
		deadline := time.Now().Add(time.Second)
		for {
			message := remotetest.ReadControlMessage(t, link, time.Until(deadline))
			if message.Type != remoteproto.ControlLease {
				continue
			}
			var token string
			if err := json.Unmarshal(message.Data, &token); err != nil {
				t.Fatal(err)
			}
			verifier, err := remoteproto.NewJWSVerifier(f.replica.App.Issuer, f.replica.App.Keys.JWKS(), nil)
			if err != nil {
				t.Fatal(err)
			}
			lease, err := verifier.VerifyLease(t.Context(), token, f.clock.Now())
			if err != nil || lease.Plan != "pro" || !slices.Contains(lease.AllowedRoutes, remoteproto.RouteCloud) {
				t.Fatalf("new lease = %+v: %v", lease, err)
			}
			break
		}
		if account := billingAccount(t, f, owner); account.Plan != "pro" {
			t.Fatalf("post-webhook account = %s", account.Plan)
		}
	})
}

func TestStripePlanLapse(t *testing.T) {
	t.Parallel()
	for _, tie := range []bool{false, true} {
		label := "Should rank the last successful session before any heartbeat (UT-064, UT-065)"
		if tie {
			label = "Should break activity ties by earliest link and then ID (UT-064)"
		}
		t.Run(label, func(t *testing.T) {
			// Safety Invariants 4 and 7. Entitlement, the selected Free slot and retained tunnels share the account lock.
			t.Parallel()
			f, stripe := newBillingFixture(t, nil)
			owner := newBillingDevice(t, f, "lapse@example.test")
			now := f.clock.Now()
			a, b, c := remoteproto.ComputerID("cmp_a"), remoteproto.ComputerID("cmp_b"), remoteproto.ComputerID("cmp_c")
			sub := proStripeSub(now, "sub_rank", owner.account)
			stripe.Put(sub)
			deliverStripeEvent(t, f, "evt_rank_start", stripeSubCreated, sub.ID)
			remoteCloudRequest(t, f, owner, http.MethodPost, "/v1/computers/reservations", nil).
				expect(t, http.StatusCreated, "reservation_id")
			for _, id := range []remoteproto.ComputerID{b, a, c} {
				f.db.SeedControlComputer(t, owner.account, id, now)
				f.db.Exec(t, `INSERT INTO %s.tunnels (computer_id, hostname, state, created_at)
					VALUES ($1, $1 || '.compozy.run', 'active', $2)`, string(id), now)
			}
			f.db.Exec(t, "UPDATE %s.computers SET linked_at = $2, last_session_at = $3 WHERE id = $1",
				string(b), now.Add(-time.Hour), now)
			aSession := now.Add(-time.Minute)
			if tie {
				aSession = now
			}
			f.db.Exec(t, "UPDATE %s.computers SET last_session_at = $2, last_control_at = $3 WHERE id = $1",
				string(a), aSession, now.Add(time.Second))
			sub.Status = "canceled"
			stripe.Put(sub)
			deliverStripeEvent(t, f, "evt_rank_end", stripeSubDeleted, sub.ID)
			assertAuthRow(t, f, "SELECT count(*) FROM slot_reservations WHERE released_at IS NULL", 0)
			assertPlanLimited(t, f, map[remoteproto.ComputerID]bool{a: true, b: false, c: true})
			for _, id := range []remoteproto.ComputerID{a, b, c} {
				var deadline time.Time
				if err := f.conn.QueryRow(t.Context(), "SELECT delete_after FROM tunnels WHERE computer_id = $1", id).
					Scan(&deadline); err != nil || !deadline.Equal(now.Add(7*24*time.Hour)) {
					t.Fatalf("tunnel %s deadline = %v: %v", id, deadline, err)
				}
			}
			remoteCloudRequest(t, f, owner, http.MethodDelete, "/v1/computers/"+string(b), nil).
				expect(t, http.StatusNoContent, "")
			assertPlanLimited(t, f, map[remoteproto.ComputerID]bool{a: false, c: true})
			remoteCloudRequest(t, f, owner, http.MethodPost, "/v1/computers/reservations", nil).
				expect(t, http.StatusPaymentRequired, "plan_limit_reached")
		})
	}
}

func assertPlanLimited(t *testing.T, f authFixture, want map[remoteproto.ComputerID]bool) {
	t.Helper()
	for id, limited := range want {
		computer, err := f.replica.App.Store.ControlComputer(t.Context(), id)
		if err != nil || computer.PlanLimited != limited {
			t.Fatalf("%s plan-limited = %v, want %v: %v", id, computer.PlanLimited, limited, err)
		}
	}
}
