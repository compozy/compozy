//go:build integration

package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestCloudBillingDeletion(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should refund only the eligible last payment and finish every cancellation (UT-136, UT-137)",
		func(t *testing.T) {
			// Safety Invariant 35. An external failure leaves the row entitled and an idempotent retry finishes it.
			t.Parallel()
			for _, tc := range []struct {
				name, interval, reason string
				daysAgo                int
				wantRefund             bool
			}{
				{"Should refund an annual renewal paid five days ago", "year", "subscription_cycle", 5, true},
				{"Should leave an annual renewal paid twenty days ago without refund", "year", "subscription_cycle", 20, false},
				{"Should leave a monthly renewal without refund", "month", "subscription_cycle", 3, false},
				{"Should refund a first monthly payment", "month", "subscription_create", 3, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					f, stripe := newBillingFixture(t, nil)
					owner := newBillingDevice(t, f, "deletion@example.test")
					now := f.clock.Now()
					paid := now.AddDate(0, 0, -tc.daysAgo)
					sub := proStripeSub(now, "sub_delete", owner.account)
					sub.Interval = tc.interval
					if tc.interval == "month" {
						sub.Price = remotetest.StripeMonthly
					}
					sub.Invoices = []remotetest.StripeInvoice{
						{ID: "in_last", Reason: tc.reason, Price: sub.Price, PaidAt: paid},
					}
					if tc.reason == "subscription_cycle" {
						sub.Invoices = append(sub.Invoices, remotetest.StripeInvoice{
							ID:     "in_first",
							Reason: "subscription_create",
							Price:  sub.Price,
							PaidAt: paid.AddDate(0, -1, 0),
						})
					}
					stripe.Put(sub)
					deliverStripeEvent(t, f, "evt_paid", "invoice.paid", sub.ID)
					assertRefundWindow(t, f, owner, paid, tc.reason == "subscription_create" || tc.interval == "year")
					f.db.Exec(t, `INSERT INTO %s.subscriptions (id, source, account_id, status, ends_at)
					VALUES ('csub_delete', 'complimentary', $1, 'active', $2)`, owner.account, now.Add(time.Hour))
					f.db.Exec(t, "UPDATE %s.accounts SET state = 'deleting' WHERE id = $1", owner.account)
					stripe.FailDeletion(true)
					if err := f.replica.App.Billing.CancelForDeletion(t.Context(), owner.account); err == nil {
						t.Fatal("deletion accepted an unconfirmed Stripe cancellation")
					}
					row, err := f.replica.App.Store.BillingSubscription(t.Context(), sub.ID)
					if err != nil || row.Status != "active" {
						t.Fatalf("failed deletion updated the entitled row: %+v: %v", row, err)
					}
					stripe.FailDeletion(false)
					if err := f.replica.App.Billing.CancelForDeletion(t.Context(), owner.account); err != nil {
						t.Fatal(err)
					}
					deletes, refunds := stripe.DeletionCalls()
					if len(deletes) != 2 || deletes[0] != sub.ID || deletes[1] != sub.ID ||
						(len(refunds) == 1) != tc.wantRefund {
						t.Fatalf("deletion calls = %v, refunds = %v, want refund %v", deletes, refunds, tc.wantRefund)
					}
					for key, refund := range refunds {
						if key != "delete_refund_sub_delete_inpay_in_last" ||
							refund.Get("payment_intent") != "pi_in_last" || refund.Get("amount") != "7999" {
							t.Fatalf("refund did not target the full last payment: %s %v", key, refund)
						}
					}
					row, err = f.replica.App.Store.BillingSubscription(t.Context(), sub.ID)
					if err != nil || row.Status != "canceled" || row.CancelAtPeriodEnd {
						t.Fatalf("cancellation was not immediate: %+v: %v", row, err)
					}
					comp, err := f.replica.App.Store.BillingSubscription(t.Context(), "csub_delete")
					if err != nil || !comp.EndsAt.Equal(now) {
						t.Fatalf("complimentary entitlement remaining: %+v: %v", comp, err)
					}
				})
			}
		},
	)

	t.Run("Should leave complimentary grants intact during Stripe reconciliation (UT-137)", func(t *testing.T) {
		t.Parallel()
		f, stripe := newBillingFixture(t, nil)
		owner := newBillingDevice(t, f, "complimentary@example.test")
		ends := f.clock.Now().Add(30 * 24 * time.Hour)
		f.db.Exec(t, `INSERT INTO %s.subscriptions (id, source, account_id, status, ends_at)
			VALUES ('csub_gift', 'complimentary', $1, 'active', $2)`, owner.account, ends)
		first := billingAccount(t, f, owner)
		if first.Plan != "pro" || first.Subscription == nil || first.Subscription.Source != "complimentary" {
			t.Fatalf("complimentary entitlement = %+v", first.AccountPlan)
		}
		sub := proStripeSub(f.clock.Now(), "sub_gift", owner.account)
		stripe.Put(sub)
		deliverStripeEvent(t, f, "evt_stripe_gift", stripeSubCreated, sub.ID)
		if err := f.replica.App.Billing.Reconcile(t.Context()); err != nil {
			t.Fatal(err)
		}
		gift, err := f.replica.App.Store.BillingSubscription(t.Context(), "csub_gift")
		if err != nil || gift.Source != "complimentary" || !gift.EndsAt.Equal(ends) {
			t.Fatalf("Stripe changed a complimentary grant: %+v: %v", gift, err)
		}
		f.clock.Advance(30 * 24 * time.Hour)
		if err := f.replica.App.Billing.ReconcileComplimentary(t.Context()); err != nil {
			t.Fatal(err)
		}
		// Refresh the device session after the clock jump.
		owner = newBillingDevice(t, f, "complimentary@example.test")
		if account := billingAccount(t, f, owner); account.Plan != "pro" || account.Subscription == nil ||
			account.Subscription.Source != "stripe" {
			t.Fatalf("gift expiry removed active Stripe entitlement: %+v", account.AccountPlan)
		}
		sub.Status = "canceled"
		stripe.Put(sub)
		deliverStripeEvent(t, f, "evt_both_ended", stripeSubDeleted, sub.ID)
		if account := billingAccount(t, f, owner); account.Plan != "free" || account.Subscription != nil {
			t.Fatalf("account retained an ended subscription: %+v", account.AccountPlan)
		}
	})
}

func assertRefundWindow(t *testing.T, f authFixture, owner remoteTestDevice, paid time.Time, eligible bool) {
	t.Helper()
	account := billingAccount(t, f, owner)
	if account.Subscription == nil {
		t.Fatal("account is missing its subscription")
	}
	until := account.Subscription.RefundEligibleUntil
	if (!eligible && until != nil) || (eligible && (until == nil || !until.Equal(paid.Add(14*24*time.Hour)))) {
		t.Fatalf("refund window = %v, paid %v, eligible %v", until, paid, eligible)
	}
	reply := remoteCloudRequest(t, f, owner, http.MethodPost, "/v1/billing/portal", nil)
	reply.expect(t, http.StatusOK, "billing.stripe.com")
}
