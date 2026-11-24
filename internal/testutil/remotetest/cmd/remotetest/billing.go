//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/billing"
	"github.com/compozy/compozy/internal/cloud/cloudflare"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	fixtureConnectorEnv = "COMPOZY_TEST_HOSTED_CONNECTOR"
	fixtureHostname     = "purchase-fixture.compozy.run"
	fixtureTunnelID     = "10000000-0000-4000-8000-000000000001"
	foreignEmail        = "other-account@example.test"
)

func preparePurchase(f *fixture, db *remotetest.Database, scenario string) scenarioManifest {
	f.Helper()
	stripe := remotetest.StartStripe(f, time.Now, true)
	replica := startCloud(f, db, "", func(cfg *cloud.Config, idp *remotetest.IdentityProvider) {
		cfg.Stripe = stripe.Client
		cfg.MonthlyPrice, cfg.AnnualPrice = remotetest.StripeMonthly, remotetest.StripeAnnual
		cfg.Auth.MonthlyPriceID, cfg.Auth.AnnualPriceID = cfg.MonthlyPrice, cfg.AnnualPrice
		idp.SetBrowserGitHubProfile(remotetest.ProviderProfile{Email: foreignEmail, Verified: true, GitHubID: 107})
		if scenario == hostedPurchase {
			cfg.HostedTunnels = startPurchaseCloudflare(f)
		}
	})
	controller := prepareDaemon(f, false)
	result := scenarioManifest{
		Scenario:   scenario,
		CloudURL:   replica.Server.URL,
		Controller: controller,
		Email:      fixtureEmail,
		StripeURL:  stripe.Origin,
		Computers:  []computerTarget{},
		HostedURL:  "https://" + fixtureHostname,
	}
	startDaemon(f, controller, result.CloudURL)
	replica.App.Routes().HandleFunc("GET /fixture/billing", func(w http.ResponseWriter, r *http.Request) {
		writePurchaseEvidence(w, r, replica, stripe)
	})
	if scenario == hostedPurchase {
		prepareRetainedHost(f, db, &result)
	} else {
		db.Exec(
			f,
			"INSERT INTO %s.accounts (id, email, name, state, first_sign_in_provider) VALUES ($1, $2, $3, 'suspended', 'github')",
			"acc_foreign",
			foreignEmail,
			"Other account",
		)
		const otherComputer = "cmp_foreign_purchase"
		db.SeedControlComputer(f, "acc_foreign", otherComputer, time.Now())
		seedRetainedTunnel(f, db, otherComputer, "")
	}
	return result
}

func writePurchaseEvidence(
	w http.ResponseWriter, r *http.Request, replica *remotetest.Replica, stripe *remotetest.StripeFixture,
) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	conn, err := replica.App.Store.ConnectDirect(ctx)
	if err != nil {
		http.Error(w, "missing fixture database", http.StatusServiceUnavailable)
		return
	}
	defer func() {
		if err := conn.Close(ctx); err != nil {
			slog.DebugContext(ctx, "Could not close purchase fixture connection", "error", err)
		}
	}()
	var account string
	var devices, installations int
	err = conn.QueryRow(ctx, `SELECT a.id,
		(SELECT count(*) FROM devices d WHERE d.account_id = a.id),
		(SELECT count(DISTINCT installation_id) FROM devices d WHERE d.account_id = a.id)
		FROM accounts a WHERE a.email = $1`, fixtureEmail).Scan(&account, &devices, &installations)
	if err != nil {
		http.Error(w, "missing purchase account", http.StatusNotFound)
		return
	}
	view, err := replica.App.Billing.Overview(ctx, account, billing.Target{Interval: "year"}, nil)
	if err != nil {
		http.Error(w, "missing billing state", http.StatusServiceUnavailable)
		return
	}
	checkouts := stripe.Checkouts()
	price, promo, owner := "", "", ""
	if len(checkouts) == 1 {
		form := checkouts[0].Form
		price, promo, owner = form.Get(
			"line_items[0][price]",
		), form.Get(
			"allow_promotion_codes",
		), form.Get(
			"client_reference_id",
		)
	}
	api.WriteJSON(w, http.StatusOK, map[string]any{
		"plan": view.Billing.Plan, "devices": devices, "installations": installations, "checkouts": len(checkouts),
		"price": price, "promotion_codes": promo == "true", "owned_checkout": owner == account,
	})
}

func prepareRetainedHost(f *fixture, db *remotetest.Database, m *scenarioManifest) {
	f.Helper()
	host := prepareDaemon(f, true)
	origin := configureHostedListener(f, host)
	installFixtureConnector(f, host.Home)
	startDaemon(f, host, m.CloudURL, fixtureConnectorEnv+"=1")
	signIn(f, host)
	result := cliJSON[contract.RemoteLinkResponse](f, host, m.CloudURL, "remote", "link")
	seedRetainedTunnel(f, db, string(result.ComputerID), "http://"+origin)
	cliJSON[contract.RemoteStatusResponse](f, host, m.CloudURL, "remote", "cloud-route", "enable")
	client := httpClientFor(host)
	defer client.CloseIdleConnections()
	remotetest.Await(f, "retained hosted listener", 15*time.Second, func(ctx context.Context) bool {
		var status contract.RemoteStatusResponse
		return requestJSON(ctx, client, http.MethodGet, contract.RemotePath, nil, &status) == nil &&
			status.CloudRoute.State == remoteproto.HostedTunnelEntitlementOnly &&
			status.CloudRoute.Hostname != nil && *status.CloudRoute.Hostname == fixtureHostname
	})
	target := &url.URL{Scheme: "http", Host: origin}
	proxy := httputil.NewSingleHostReverseProxy(target)
	edge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = fixtureHostname
		proxy.ServeHTTP(w, r)
	}))
	f.Cleanup(edge.Close)
	m.HostedProxy = edge.URL
	m.Computers = append(m.Computers, computerTarget{ID: result.ComputerID, Name: result.Name, Socket: host.Socket})
}

func configureHostedListener(f *fixture, host daemonInstance) string {
	f.Helper()
	addr, err := new(net.ListenConfig).Listen(f.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		f.Fatal(err)
	}
	origin := addr.Addr().String()
	if err := addr.Close(); err != nil {
		f.Fatal(err)
	}
	config := filepath.Join(host.Home, "config.toml")
	content, err := os.ReadFile(config)
	if err != nil {
		f.Fatal(err)
	}
	writeFile(
		f,
		config,
		strings.Replace(string(content), "[remote]\n", "[remote]\nhosted_listener = '"+origin+"'\n", 1),
	)
	return origin
}

func seedRetainedTunnel(f *fixture, db *remotetest.Database, id, origin string) {
	f.Helper()
	db.Exec(f, `INSERT INTO %s.tunnels (computer_id, hostname, state, cf_tunnel_id, hosted_origin, delete_after)
		VALUES ($1, $2, 'active', $3, nullif($4, ''), now() + interval '7 days')`,
		id, fixtureHostname, fixtureTunnelID, origin)
}

func startPurchaseCloudflare(f *fixture) *cloudflare.Client {
	f.Helper()
	const path = "/accounts/fixture/cfd_tunnel/" + fixtureTunnelID
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+path, func(w http.ResponseWriter, _ *http.Request) {
		api.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "result": map[string]string{
			"id": fixtureTunnelID, "name": "compozy-prod-purchase-fixture-1",
		}})
	})
	mux.HandleFunc("GET "+path+"/token", func(w http.ResponseWriter, _ *http.Request) {
		api.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "result": "fixture-connector"})
	})
	mux.HandleFunc("PUT "+path+"/configurations", func(w http.ResponseWriter, _ *http.Request) {
		api.WriteJSON(w, http.StatusOK, map[string]any{"success": true})
	})
	server := httptest.NewServer(mux)
	f.Cleanup(server.Close)
	client, err := cloudflare.New(cloudflare.Config{AccountID: "fixture", ZoneID: "fixture",
		Token: "test-key", Domain: cloudflare.ProductionDomain, BaseURL: server.URL})
	if err != nil {
		f.Fatal(err)
	}
	return client
}

func fixtureConnectorPath(_ context.Context, home string) (string, error) {
	return filepath.Join(home, "tools", "cloudflared", "fixture", "cloudflared"), nil
}

func installFixtureConnector(f *fixture, home string) {
	f.Helper()
	binary, err := os.Executable()
	if err != nil {
		f.Fatal(err)
	}
	link, err := fixtureConnectorPath(f.Context(), home)
	if err != nil {
		f.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		f.Fatal(err)
	}
	if err := os.Link(binary, link); err != nil {
		f.Fatal(err)
	}
}

func runFixtureConnector(ctx context.Context, args []string) error {
	metrics := slices.Index(args, "--metrics")
	if metrics < 0 || metrics+1 >= len(args) || os.Getenv("TUNNEL_TOKEN") != "fixture-connector" {
		return errors.New("invalid fixture connector invocation")
	}
	listener, err := new(net.ListenConfig).Listen(ctx, "tcp4", args[metrics+1])
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	server := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}
	defer context.AfterFunc(ctx, func() {
		if err := server.Close(); err != nil {
			slog.DebugContext(ctx, "Could not close hosted fixture connector", "error", err)
		}
	})()
	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func prepareFreeUpgrade(f *fixture, db *remotetest.Database, assets remotetest.SSHAssets) scenarioManifest {
	f.Helper()
	stripe := remotetest.StartStripe(f, time.Now, true)
	replica := startCloud(f, db, assets.Gateway, func(cfg *cloud.Config, _ *remotetest.IdentityProvider) {
		cfg.Stripe = stripe.Client
		cfg.MonthlyPrice, cfg.AnnualPrice = remotetest.StripeMonthly, remotetest.StripeAnnual
		cfg.Auth.MonthlyPriceID, cfg.Auth.AnnualPriceID = cfg.MonthlyPrice, cfg.AnnualPrice
	})
	webhookClient := &http.Client{Timeout: 10 * time.Second}
	f.Cleanup(webhookClient.CloseIdleConnections)
	stripe.SetOnComplete(func(checkout remotetest.StripeCheckout) {
		if checkout.Sub == "" {
			return
		}
		raw := []byte(fmt.Sprintf(
			`{"id":"evt_checkout_%s","type":"checkout.session.completed","data":{"object":{"id":%q,"subscription":%q}}}`,
			checkout.ID, checkout.ID, checkout.Sub,
		))
		ctx, cancel := context.WithTimeout(f.Context(), 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(
			ctx, http.MethodPost, replica.Server.URL+"/v1/billing/stripe-webhook", bytes.NewReader(raw),
		)
		if err != nil {
			slog.WarnContext(ctx, "Could not build fixture stripe webhook", "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Stripe-Signature", remotetest.StripeSignature(raw, time.Now()))
		resp, err := webhookClient.Do(req)
		if err != nil {
			slog.WarnContext(ctx, "Could not deliver fixture stripe webhook", "error", err)
			return
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.DebugContext(ctx, "Could not close fixture stripe webhook response", "error", closeErr)
		}
	})

	controller := prepareDaemon(f, false)
	mock := filepath.Join(f.TempDir(), "echo.json")
	writeFile(f, mock, echoFixture)
	host2, releaseEnv := prepareSSH(f, controller, assets, replica.Server.URL, mock, testHost2Name)
	startDaemon(f, controller, replica.Server.URL, releaseEnv...)
	controller.DeviceID = signIn(f, controller)
	cliJSON[contract.OnboardingStatusResponse](f, controller, replica.Server.URL, "onboarding", "complete")

	conn, err := replica.App.Store.ConnectDirect(f.Context())
	if err != nil {
		f.Fatalf("connect cloud db for free-upgrade: %v", err)
	}
	var accountID string
	queryErr := conn.QueryRow(f.Context(), "SELECT id FROM accounts WHERE email = $1", fixtureEmail).Scan(&accountID)
	closeErr := conn.Close(f.Context())
	if queryErr != nil {
		f.Fatalf("load fixture account id for free-upgrade: %v", queryErr)
	}
	if closeErr != nil {
		f.Fatalf("close cloud db connection for free-upgrade: %v", closeErr)
	}
	db.SeedControlComputer(f, accountID, "cmp_free_slot_1", time.Now())
	cliJSON[contract.Computer](f, controller, replica.Server.URL, "computer", "add", "account", "cmp_free_slot_1")

	return scenarioManifest{
		Scenario:   freeUpgrade,
		CloudURL:   replica.Server.URL,
		Controller: controller,
		Email:      fixtureEmail,
		StripeURL:  stripe.Origin,
		Computers:  []computerTarget{host2},
	}
}
