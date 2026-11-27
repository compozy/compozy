//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	hostedBrowser        = "hosted-browser"
	hostedEdgeAddressEnv = "COMPOZY_TEST_HOSTED_EDGE_ADDRESS"
	hostedEdgeCertEnv    = "COMPOZY_TEST_HOSTED_EDGE_CERT"
)

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
) (int, []string) {
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

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				ModifyResponse: exposeBrowserFixtureCloud(cloudURL),
			}
			proxy.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	return startFixtureHostedEdge(f, handler, "*.compozy.run", "cloud.compozy.com")
}

// The daemon uses a local Cloud endpoint. Browsers reach the fixture through the HTTPS edge.
func exposeBrowserFixtureCloud(cloudURL string) func(*http.Response) error {
	return func(response *http.Response) error {
		if response.Body == http.NoBody || response.StatusCode != http.StatusOK ||
			!strings.HasPrefix(response.Header.Get("Content-Type"), "text/html") {
			return nil
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		metadata := []byte(`name="compozy-cloud-origin" content="` + cloudURL + `"`)
		if !bytes.Contains(body, metadata) {
			return errors.New("hosted shell lacks the fixture Cloud origin")
		}
		body = bytes.Replace(
			body, metadata, []byte(`name="compozy-cloud-origin" content="https://cloud.compozy.com"`), 1,
		)
		response.Body = io.NopCloser(bytes.NewReader(body))
		response.ContentLength = int64(len(body))
		response.Header.Set("Content-Length", strconv.Itoa(len(body)))
		return nil
	}
}

func startFixtureHostedEdge(f *fixture, handler http.Handler, hostnames ...string) (int, []string) {
	f.Helper()
	server := httptest.NewUnstartedServer(handler)
	certificate := remotetest.HostedEdgeCertificate(f, hostnames...)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	f.Cleanup(server.Close)
	edgeCert := filepath.Join(f.TempDir(), "edge.der")
	if err := os.WriteFile(edgeCert, server.Certificate().Raw, 0o600); err != nil {
		f.Fatal(err)
	}
	address := server.Listener.Addr().(*net.TCPAddr)
	return address.Port, []string{
		fixtureConnectorEnv + "=1",
		hostedEdgeAddressEnv + "=" + address.String(),
		hostedEdgeCertEnv + "=" + edgeCert,
	}
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

	edgePort, edgeEnv := startHostedBrowserEdge(f, cloudURL, idp, fake, profile)

	controller := prepareDaemon(f, true)
	configureHostedListener(f, controller)
	installFixtureConnector(f, controller.Home)
	configureLocalAgent(f, controller, mock)
	startDaemon(f, controller, cloudURL, append(edgeEnv, "COMPOZY_WEB_DIST_DIR="+distDir)...)

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
