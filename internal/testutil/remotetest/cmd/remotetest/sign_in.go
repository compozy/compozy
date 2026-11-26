//go:build integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	signInFirstRun   = "sign-in-first-run"
	signInSignOut    = "sign-in-sign-out"
	accountDeletePro = "account-delete-pro"
)

func prepareSignInFirstRun(f *fixture, db *remotetest.Database) scenarioManifest {
	f.Helper()
	cloudURL := startCloud(f, db, "", nil).Server.URL
	controller := prepareDaemon(f, false)
	startDaemon(f, controller, cloudURL, signInRequiredEnv+"=1")
	return scenarioManifest{
		Scenario:   signInFirstRun,
		CloudURL:   cloudURL,
		Controller: controller,
		Email:      fixtureEmail,
		Computers:  []computerTarget{},
	}
}

func prepareSignInSignOut(f *fixture, db *remotetest.Database) scenarioManifest {
	f.Helper()
	cloudURL := startCloud(f, db, "", nil).Server.URL
	controller := prepareDaemon(f, false)
	startDaemon(f, controller, cloudURL, signInRequiredEnv+"=1")
	controller.DeviceID = signIn(f, controller)
	cliJSON[contract.OnboardingStatusResponse](f, controller, cloudURL, "onboarding", "complete")
	return scenarioManifest{
		Scenario:   signInSignOut,
		CloudURL:   cloudURL,
		Controller: controller,
		Email:      fixtureEmail,
		Computers:  []computerTarget{},
	}
}

func prepareAccountDeletePro(f *fixture, db *remotetest.Database) scenarioManifest {
	f.Helper()
	stripe := remotetest.StartStripe(f, time.Now, false)
	replica := startCloud(f, db, "", func(cfg *cloud.Config, _ *remotetest.IdentityProvider) {
		cfg.Stripe = stripe.Client
		cfg.MonthlyPrice, cfg.AnnualPrice = remotetest.StripeMonthly, remotetest.StripeAnnual
		cfg.Auth.MonthlyPriceID, cfg.Auth.AnnualPriceID = cfg.MonthlyPrice, cfg.AnnualPrice
	})
	controller := prepareDaemon(f, false)
	startDaemon(f, controller, replica.Server.URL, signInRequiredEnv+"=1")
	controller.DeviceID = signIn(f, controller)
	cliJSON[contract.OnboardingStatusResponse](f, controller, replica.Server.URL, "onboarding", "complete")

	conn, err := replica.App.Store.ConnectDirect(f.Context())
	if err != nil {
		f.Fatalf("connect cloud db for account-delete-pro: %v", err)
	}
	var accountID string
	queryErr := conn.QueryRow(f.Context(), "SELECT id FROM accounts WHERE email = $1", fixtureEmail).Scan(&accountID)
	closeErr := conn.Close(f.Context())
	if queryErr != nil {
		f.Fatalf("load fixture account id for account-delete-pro: %v", queryErr)
	}
	if closeErr != nil {
		f.Fatalf("close cloud db connection for account-delete-pro: %v", closeErr)
	}

	now := time.Now()
	const subID = "sub_e2e_delete_pro"
	stripe.Put(remotetest.StripeSubscription{
		ID:        subID,
		Account:   accountID,
		Status:    "active",
		Price:     remotetest.StripeAnnual,
		Interval:  "year",
		PeriodEnd: now.Add(363 * 24 * time.Hour),
		Invoices: []remotetest.StripeInvoice{{
			ID:     "in_e2e_delete_pro",
			Reason: "subscription_create",
			Price:  remotetest.StripeAnnual,
			PaidAt: now.Add(-48 * time.Hour),
		}},
	})
	deliverFixtureStripePaid(f, replica.Server.URL, now, subID)

	client := httpClientFor(controller)
	defer client.CloseIdleConnections()
	remotetest.Await(f, "Pro subscription visible on controller", 15*time.Second, func(ctx context.Context) bool {
		var status contract.AccountStatusResponse
		return requestJSON(ctx, client, http.MethodGet, contract.AccountPath+"?refresh=true", nil, &status) == nil &&
			status.Plan == "pro" && status.Subscription != nil && status.Subscription.RefundEligibleUntil != nil
	})

	replica.App.Routes().HandleFunc("GET /fixture/deletion", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if sweepErr := replica.App.Accounts.SweepAccountDeletions(ctx); sweepErr != nil {
			slog.DebugContext(ctx, "Account deletion sweep will retry", "error", sweepErr)
		}
		deletes, refunds := stripe.DeletionCalls()
		api.WriteJSON(w, http.StatusOK, map[string]any{
			"deletes": deletes,
			"refunds": len(refunds),
		})
	})

	return scenarioManifest{
		Scenario:   accountDeletePro,
		CloudURL:   replica.Server.URL,
		Controller: controller,
		Email:      fixtureEmail,
		StripeURL:  stripe.Origin,
		Computers:  []computerTarget{},
	}
}

func deliverFixtureStripePaid(f *fixture, cloudURL string, now time.Time, subID string) {
	f.Helper()
	raw := []byte(fmt.Sprintf(
		`{"id":"evt_e2e_delete_pro_paid","type":"invoice.paid","data":{"object":{"parent":{"subscription_details":{"subscription":%q}}}}}`,
		subID,
	))
	ctx, cancel := context.WithTimeout(f.Context(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, cloudURL+"/v1/billing/stripe-webhook", bytes.NewReader(raw),
	)
	if err != nil {
		f.Fatalf("build fixture stripe webhook: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", remotetest.StripeSignature(raw, now))
	webhookClient := &http.Client{Timeout: 10 * time.Second}
	defer webhookClient.CloseIdleConnections()
	resp, err := webhookClient.Do(req)
	if err != nil {
		f.Fatalf("deliver fixture stripe webhook: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			f.Error(closeErr)
		}
	}()
	if resp.StatusCode != http.StatusNoContent {
		f.Fatalf("fixture stripe webhook returned HTTP %d", resp.StatusCode)
	}
}
