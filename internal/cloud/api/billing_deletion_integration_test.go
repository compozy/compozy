// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration

package api_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil"
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

	t.Run("Should cancel every subscription before retrying a failed refund", func(t *testing.T) {
		// RC-06: A refund failure cannot leave Stripe renewing any of the deleting account's subscriptions.
		t.Parallel()
		f, stripe := newBillingFixture(t, nil)
		owner := newBillingDevice(t, f, "refund-retry@example.test")
		now := f.clock.Now()
		refundable := proStripeSub(now, "sub_a_refundable", owner.account)
		refundable.Invoices = []remotetest.StripeInvoice{
			{ID: "in_retry", Reason: "subscription_create", Price: refundable.Price, PaidAt: now.AddDate(0, 0, -5)},
		}
		other := proStripeSub(now, "sub_b_other", owner.account)
		for _, sub := range []remotetest.StripeSubscription{refundable, other} {
			stripe.Put(sub)
			deliverStripeEvent(t, f, "evt_"+sub.ID, stripeSubCreated, sub.ID)
		}
		f.db.Exec(t, "UPDATE %s.accounts SET state = 'deleting' WHERE id = $1", owner.account)
		stripe.FailRefund(true)
		if err := f.replica.App.Billing.CancelForDeletion(t.Context(), owner.account); err == nil {
			t.Fatal("deletion accepted a failed refund")
		}
		if cancellations, refunds := stripe.DeletionCalls(); len(cancellations) != 2 || len(refunds) != 0 {
			t.Fatalf("cancellations before refund retry = %v, refunds = %v", cancellations, refunds)
		}
		for _, sub := range []remotetest.StripeSubscription{refundable, other} {
			row, err := f.replica.App.Store.BillingSubscription(t.Context(), sub.ID)
			if err != nil || row.Status != "canceled" {
				t.Fatalf("subscription %s was not canceled: %+v, %v", sub.ID, row, err)
			}
		}
		deliverStripeEvent(t, f, "evt_refunded_sub_ended", stripeSubDeleted, refundable.ID)
		stripe.FailRefund(false)
		if err := f.replica.App.Billing.CancelForDeletion(t.Context(), owner.account); err != nil {
			t.Fatal(err)
		}
		if cancellations, refunds := stripe.DeletionCalls(); len(cancellations) != 2 || len(refunds) != 1 {
			t.Fatalf(
				"refund retry repeated or missed a Stripe operation: cancellations %v, refunds %v",
				cancellations,
				refunds,
			)
		}
	})

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

	t.Run(
		"Should disable all account credentials and open checkouts in the deletion transaction (UT-121)",
		func(t *testing.T) {
			t.Parallel()
			// Safety Invariants 22, 35 and 36. A retrying deletion is fenced and its tombstone is PII-free.
			resend := remotetest.StartResend(t)
			f, stripe := newBillingFixture(t, func(cfg *cloud.Config) {
				cfg.Issuer = authOrigin
				cfg.Email = resend.Client
			})
			const address = "delete-cloud@example.test"
			owner := newBillingDevice(t, f, address)
			other := newBillingDevice(t, f, address)
			request := remoteproto.AccountDeleteRequest{ConfirmEmail: address}
			remoteCloudRequest(t, f, other, http.MethodDelete, cloudAccountPath, request).
				expect(t, http.StatusForbidden, "account_admin_required")
			remoteCloudRequest(t, f, owner, http.MethodDelete, cloudAccountPath,
				remoteproto.AccountDeleteRequest{ConfirmEmail: "wrong@example.test"}).
				expect(t, http.StatusBadRequest, "confirm_email_mismatch")
			remoteCloudRequest(
				t,
				f,
				owner,
				http.MethodPost,
				"/v1/billing/checkout",
				remoteproto.CheckoutRequest{Interval: "year"},
			).
				expect(t, http.StatusOK, "checkout.stripe.com")
			sub := proStripeSub(f.clock.Now(), "sub_lifecycle", owner.account)
			stripe.Put(sub)
			deliverStripeEvent(t, f, "evt_delete_lifecycle", stripeSubCreated, sub.ID)
			stripe.FailDeletion(true)
			p := providerProfile{subject: address, email: address, verified: true}
			web := purchaseWebSession(t, f, p)
			pending := newEnrollment(t, f, auth.Google, p, false)
			token := createLifecycleAccountToken(t, f, owner)
			remoteCloudRequest(t, f, owner, http.MethodDelete, cloudAccountPath, request).
				expect(t, http.StatusAccepted, `"subscription_cancelled":true`)
			assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE state = 'deleting'", 1)
			assertAuthRow(t, f, "SELECT count(*) FROM device_sessions WHERE revoked_at IS NOT NULL", 2)
			assertAuthRow(t, f, "SELECT count(*) FROM cloud_web_sessions WHERE revoked_at IS NOT NULL", 1)
			assertAuthRow(t, f, "SELECT count(*) FROM account_tokens WHERE revoked_at IS NOT NULL", 1)
			assertAuthRow(t, f, "SELECT count(*) FROM billing_checkouts WHERE expired_at IS NOT NULL", 1)
			assertAuthRow(t, f, "SELECT count(*) FROM security_events WHERE type = 'account_deletion_started'", 1)
			exchangeLifecycleAccountToken(t, f, token).expect(t, http.StatusUnauthorized, `"reason":"account_deleted"`)
			for _, path := range []string{"/v1/computers/cmp_test/grants", "/v1/computers/reservations", "/v1/billing/checkout"} {
				remoteCloudRequest(t, f, owner, http.MethodPost, path, map[string]string{}).
					expect(t, http.StatusConflict, "account_deleting")
			}
			f.request(t, http.MethodGet, "/billing/upgrade", nil, web).
				expect(t, http.StatusConflict, "account_deleting")
			key, jwk := newDeviceKey(t)
			proof, err := remoteproto.BuildDPoP(
				key,
				http.MethodPost,
				f.replica.App.Issuer+enrollPath,
				"",
				f.clock.Now(),
			)
			if err != nil {
				t.Fatal(err)
			}
			callAuth(t, f.replica, http.MethodPost, enrollPath, enrollInput(jwk), nil, map[string]string{
				"Authorization": "DPoP " + pending.EnrollmentToken, "DPoP": proof,
			}).expect(t, http.StatusConflict, "account_deleting")
			reply, _ := f.signIn(t, auth.Google, p, false)
			reply.expect(t, http.StatusConflict, "account_deleting")
			awaitDeletionSweep(t, f, func() bool {
				cancels, _ := stripe.DeletionCalls()
				return len(cancels) > 0
			})
			if got := stripe.Checkouts(); len(got) != 1 || got[0].Status != "expired" {
				t.Fatalf("open Stripe checkout was left available: %v", got)
			}
			assertAuthRow(t, f, "SELECT count(*) FROM identities", 1)
			if resend.ContactDeletes() != 0 {
				t.Fatal("contact removal ran before cancellation")
			}
			stripe.FailDeletion(false)
			f.clock.Advance(2 * time.Minute)
			awaitDeletionSweep(t, f, func() bool {
				deleted, err := f.replica.App.Store.AuthAccountDeleted(t.Context(), owner.account)
				return err == nil && deleted
			})
			assertDeletedAccount(t, f)
		},
	)

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

func awaitDeletionSweep(t *testing.T, f authFixture, ok func() bool) {
	t.Helper()
	testutil.Eventually(t, "account deletion step", 5*time.Second, func() bool {
		if err := f.replica.App.Accounts.SweepAccountDeletions(t.Context()); err != nil {
			t.Logf("account deletion is retrying: %v", err)
		}
		return ok()
	})
}

func assertDeletedAccount(t *testing.T, f authFixture) {
	t.Helper()
	for _, table := range []string{"accounts", "identities", "cloud_web_sessions", "computers", "subscriptions",
		"account_deletion_jobs", "account_deletion_identity_holds", "account_contact_addresses", "device_sessions",
		"devices", "billing_checkouts"} {
		assertAuthRow(t, f, "SELECT count(*) FROM "+table, 0)
	}
	assertAuthRow(t, f, "SELECT count(*) FROM security_events WHERE account_id IS NOT NULL", 0)
	assertAuthRow(t, f, `SELECT count(*) FROM deleted_accounts
		WHERE (security_event_counts->>'account_deletion_started')::int = 1`, 1)
}

func TestTransactionalOperationsMail(t *testing.T) {
	t.Parallel()
	t.Run("Should deliver lifecycle notices on schedule without marketing consent (UT-140)", func(t *testing.T) {
		t.Parallel()
		resend := remotetest.StartResend(t)
		f, stripe := newBillingFixture(t, func(cfg *cloud.Config) { cfg.Email = resend.Client })
		d0 := f.clock.Now()
		lapsed := newBillingDevice(t, f, "pro-ended@example.test")
		renewed := newBillingDevice(t, f, "pro-renewed@example.test")
		offline := newBillingDevice(t, f, "offline@example.test")
		ended := proStripeSub(d0, "sub_ended_mail", lapsed.account)
		resubscribed := proStripeSub(d0, "sub_renewed_mail", renewed.account)
		offlineSub := proStripeSub(d0, "sub_offline_mail", offline.account)
		for _, sub := range []remotetest.StripeSubscription{ended, resubscribed, offlineSub} {
			stripe.Put(sub)
			deliverStripeEvent(t, f, "evt_"+sub.ID+"_start", stripeSubCreated, sub.ID)
		}
		comp := remoteproto.ComputerID("cmp_offline_mail")
		f.db.SeedControlComputer(t, offline.account, comp, d0)
		f.db.Exec(t, `INSERT INTO %s.tunnels (computer_id, hostname, state, created_at)
            VALUES ($1, 'offline-mail.compozy.run', 'active', $2)`, comp, d0)
		for _, sub := range []remotetest.StripeSubscription{ended, resubscribed} {
			sub.Status = "canceled"
			stripe.Put(sub)
			deliverStripeEvent(t, f, "evt_"+sub.ID+"_end", stripeSubDeleted, sub.ID)
		}
		runMail := func() {
			t.Helper()
			if err := f.replica.App.RunOperationalMail(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		runMail()
		runMail()
		deadline := d0.AddDate(0, 0, 7).Format("January 2, 2006")
		assertOperationalMessage(t, resend, "pro-ended@example.test", "Pro ended", deadline, 1)
		assertOperationalMessage(t, resend, "pro-renewed@example.test", "Pro ended", deadline, 1)

		f.clock.Advance(3 * 24 * time.Hour)
		resubscribed.Status = "active"
		stripe.Put(resubscribed)
		deliverStripeEvent(t, f, "evt_renew_notice", stripeSubUpdated, resubscribed.ID)
		f.clock.Advance(2 * 24 * time.Hour)
		runMail()
		const reminder = "Your Compozy hosted routes will be deleted in 2 days"
		assertOperationalMessage(t, resend, "pro-ended@example.test", reminder, deadline, 1)
		assertOperationalMessage(t, resend, "pro-renewed@example.test", reminder, "", 0)

		f.clock.Advance(4 * 24 * time.Hour)
		runMail()
		const offlineSubject = "A Compozy computer has been offline for 10 days"
		assertOperationalMessage(t, resend, "offline@example.test", offlineSubject, "", 0)
		f.clock.Advance(24 * time.Hour)
		runMail()
		runMail()
		assertOperationalMessage(t, resend, "offline@example.test", offlineSubject,
			d0.AddDate(0, 0, 14).Format("January 2, 2006"), 1)
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE marketing_opt_in", 0)
		assertLifecycleNotices(t, f, resend)
	})
}

func assertOperationalMessage(
	t *testing.T, resend *remotetest.ResendFixture, to, subject, text string, want int,
) {
	t.Helper()
	count := 0
	for _, message := range resend.Messages() {
		if message.Subject != subject || !slices.Contains(message.Recipients, to) {
			continue
		}
		if message.From != "no-reply@mail.compozy.com" || !strings.Contains(message.Text, text) {
			t.Fatalf("invalid %q message sender or content: %+v", subject, message)
		}
		count++
	}
	if count != want {
		t.Fatalf("%q sent %d times, want %d", subject, count, want)
	}
}

func assertLifecycleNotices(t *testing.T, f authFixture, resend *remotetest.ResendFixture) {
	t.Helper()
	const suspendedAddr, deletedAddr = "suspended-mail@example.test", "deleted-mail@example.test"
	suspended := newBillingDevice(t, f, suspendedAddr)
	deleted := newBillingDevice(t, f, deletedAddr)
	if err := f.replica.App.Accounts.SuspendAccount(
		t.Context(),
		suspended.account,
		"operator@example.test",
	); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := f.replica.App.Accounts.SweepAccountEmail(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	assertOperationalMessage(t, resend, suspendedAddr, "Your Compozy account was suspended", "support@compozy.com", 1)

	resend.SetOnSend(func(message remotetest.ResendMessage) {
		if message.Subject != "Your Compozy account was deleted" {
			return
		}
		exists, err := f.replica.App.Store.AuthAccountDeleted(t.Context(), deleted.account)
		if err != nil || exists {
			t.Errorf("deletion email must be accepted before the tombstone exists: %v", err)
		}
	})
	request := remoteproto.AccountDeleteRequest{ConfirmEmail: deletedAddr}
	remoteCloudRequest(t, f, deleted, http.MethodDelete, cloudAccountPath, request).
		expect(t, http.StatusAccepted, "deleting")
	awaitDeletionSweep(t, f, func() bool {
		exists, err := f.replica.App.Store.AuthAccountDeleted(t.Context(), deleted.account)
		return err == nil && exists
	})
	assertOperationalMessage(t, resend, deletedAddr, "Your Compozy account was deleted", "local", 1)
}
