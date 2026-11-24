//go:build integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const hostedBrowser = "hosted-browser"

const hostedTurnstileStub = `const form = document.querySelector("#provider-form");
const challenge = document.querySelector("#turnstile");
if (form && challenge) {
  const token = form.elements.namedItem("cf-turnstile-response");
  if (token) token.value = "pass-" + (challenge.dataset.action || "browser_authorize");
  const button = form.querySelector("button[name=provider]");
  if (button) form.requestSubmit(button);
}
`

func stripHostPort(hostport string) string {
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return strings.ToLower(host)
	}
	return strings.ToLower(hostport)
}

func startHostedBrowserEdge(
	f *fixture,
	cloudURL string,
	idp *remotetest.IdentityProvider,
	fake *hostedFakeCloudflare,
	profile remotetest.ProviderProfile,
) int {
	f.Helper()
	const providerPath = "/fixture/google/authorize"
	cloudTarget, err := url.Parse(cloudURL)
	if err != nil || cloudTarget.Host == "" {
		f.Fatalf("invalid cloud URL %q: %v", cloudURL, err)
	}
	cloudProxy := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(cloudTarget)
			request.Out.Host = stripHostPort(request.In.Host)
			request.Out.Header.Set("X-Forwarded-Proto", "https")
		},
		ModifyResponse: func(response *http.Response) error {
			if query, matched := strings.CutPrefix(
				response.Header.Get("Location"),
				idp.Origin+"/google/authorize?",
			); matched {
				response.Header.Set("Location", providerPath+"?"+query)
			}
			return nil
		},
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := stripHostPort(r.Host)
		switch {
		case host == "cloud.compozy.com" && r.URL.Path == "/auth-assets/turnstile.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = io.WriteString(w, hostedTurnstileStub)
		case host == "cloud.compozy.com" && r.URL.Path == providerPath:
			query := r.URL.Query()
			code := idp.IssueCode(query, profile)
			callback := url.Values{"state": {query.Get("state")}, "code": {code}}
			http.Redirect(w, r, "/v1/auth/callback/google?"+callback.Encode(), http.StatusSeeOther)
		case host == "cloud.compozy.com":
			cloudProxy.ServeHTTP(w, r)
		case strings.HasSuffix(host, ".compozy.run"):
			edge := fake.target(host)
			if edge.Origin == "" {
				http.NotFound(w, r)
				return
			}
			target, parseErr := url.Parse(edge.Origin)
			if parseErr != nil || target.Host == "" {
				http.Error(w, "invalid hosted tunnel target", http.StatusBadGateway)
				return
			}
			proxy := &httputil.ReverseProxy{
				FlushInterval: -1,
				Rewrite: func(request *httputil.ProxyRequest) {
					request.SetURL(target)
					request.Out.Host = host
					request.Out.Header.Set("X-Forwarded-Proto", "https")
				},
			}
			proxy.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	f.Cleanup(server.Close)
	return server.Listener.Addr().(*net.TCPAddr).Port
}

func prepareHostedBrowser(
	f *fixture,
	repo string,
	db *remotetest.Database,
	mock string,
) scenarioManifest {
	f.Helper()
	distDir := filepath.Join(repo, "web", "dist")
	if _, err := os.Stat(filepath.Join(distDir, "index.html")); err != nil {
		f.Fatalf("hosted-browser scenario requires built web/dist bundle: %v", err)
	}

	fake := newHostedFakeCloudflare(f)
	profile := remotetest.ProviderProfile{
		Subject:  "remote-e2e",
		Name:     "Pedro",
		Email:    fixtureEmail,
		Verified: true,
	}
	idp, endpoints := remotetest.StartIdentityProvider(f, time.Now, &profile, "127.0.0.1")
	oauth, err := auth.NewOAuth(
		auth.OAuthClient{ID: remotetest.GoogleClient, Secret: "google-secret"},
		auth.OAuthClient{ID: "github-client", Secret: "github-secret"},
		auth.WithOAuthEndpoints(endpoints),
	)
	if err != nil {
		f.Fatal(err)
	}
	check, err := idp.Turnstile()
	if err != nil {
		f.Fatal(err)
	}
	cloudURL := remotetest.StartLocalReplicaOn(f, "", cloud.Config{
		Postgres:      db.Config(f, "web"),
		HostedTunnels: fake.client,
		Auth: &auth.Config{
			OAuth:            oauth,
			EncryptionKey:    bytes.Repeat([]byte{7}, 32),
			Turnstile:        check,
			TurnstileSiteKey: "test-site-key",
			HostedDomain:     "compozy.run",
		},
	}, nil).Server.URL

	edgePort := startHostedBrowserEdge(f, cloudURL, idp, fake, profile)

	controller := prepareDaemon(f, true)
	cfgPath := filepath.Join(controller.Home, "config.toml")
	existing, err := os.ReadFile(cfgPath)
	if err != nil {
		f.Fatal(err)
	}
	writeFile(f, cfgPath, strings.Replace(
		string(existing),
		"[remote]\nlistener = '127.0.0.1:0'\n",
		"[remote]\nlistener = '127.0.0.1:0'\nhosted_listener = '127.0.0.1:0'\n",
		1,
	))
	installFixtureConnector(f, controller.Home)
	configureLocalAgent(f, controller, mock)
	startDaemon(
		f,
		controller,
		cloudURL,
		fixtureConnectorEnv+"=1",
		"COMPOZY_WEB_DIST_DIR="+distDir,
	)

	controller.DeviceID = signIn(f, controller)
	cliJSON[contract.OnboardingStatusResponse](f, controller, cloudURL, "onboarding", "complete")

	// Grant Pro subscription so Hosted route can be turned on from Settings -> Computers.
	var accountID string
	if err := db.Pool.QueryRow(
		f.Context(),
		fmt.Sprintf("SELECT id FROM %s.accounts WHERE email = $1", db.Schema),
		fixtureEmail,
	).Scan(&accountID); err != nil {
		f.Fatalf("lookup fixture account for Pro subscription: %v", err)
	}
	db.Exec(
		f,
		`INSERT INTO %s.subscriptions (id, source, account_id, status, ends_at)
		 VALUES ($1, 'complimentary', $1, 'active', $2)`,
		accountID,
		time.Now().Add(time.Hour),
	)

	linked := cliJSON[contract.RemoteLinkResponse](f, controller, cloudURL, "remote", "link")
	statusClient := httpClientFor(controller)
	defer statusClient.CloseIdleConnections()
	remotetest.Await(f, "linked host Pro entitlement", 20*time.Second, func(ctx context.Context) bool {
		var status contract.RemoteStatusResponse
		return requestJSON(ctx, statusClient, http.MethodGet, "/api/remote", nil, &status) == nil &&
			status.Entitlement != nil && status.Entitlement.Plan == "pro"
	})

	workspace := filepath.Join(controller.OperatorHome, "project")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		f.Fatal(err)
	}
	session := cliJSON[contract.SessionPayload](
		f,
		controller,
		cloudURL,
		"session",
		"new",
		"--cwd",
		workspace,
		"--agent",
		remotetest.SSHAgent,
		"--name",
		searchTitle,
	)
	if session.ID == "" {
		f.Fatal("hosted-browser session fixture returned no ID")
	}

	return scenarioManifest{
		Scenario:     hostedBrowser,
		CloudURL:     cloudURL,
		Controller:   controller,
		Computers:    []computerTarget{{ID: linked.ComputerID, Name: linked.Name, Socket: controller.Socket}},
		Email:        fixtureEmail,
		SessionID:    session.ID,
		SessionTitle: searchTitle,
		WorkspaceID:  session.WorkspaceID,
		EdgePort:     edgePort,
	}
}
