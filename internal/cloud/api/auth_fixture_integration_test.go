//go:build integration

package api_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonboulle/clockwork"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	authOrigin           = "https://cloud.test"
	deviceFlowPath       = "/v1/device-flow"
	activatePath         = "/activate"
	browserAuthorizePath = "/v1/browser/authorize"
	enrollPath           = "/v1/devices/enroll"
	refreshPath          = "/v1/session/refresh"
	logoutPath           = "/v1/session/logout"
	activationCookie     = "__Host-compozy_act"
	webSessionCookie     = "__Host-compozy_ws"
	browserCookie        = "__Host-compozy_bd"
	googleClient         = remotetest.GoogleClient
	testInstallation     = "test-installation"
)

type authFixture struct {
	db       *remotetest.Database
	conn     *pgx.Conn
	replica  *remotetest.Replica
	cfg      cloud.Config
	clock    *clockwork.FakeClock
	provider *remotetest.IdentityProvider
}

func newAuthFixture(t *testing.T) authFixture {
	t.Helper()
	return newAuthFixtureWithReplica(t, remotetest.StartReplica)
}

func newAuthFixtureWithReplica(
	t *testing.T,
	start func(*testing.T, cloud.Config, func(*cloud.App)) *remotetest.Replica,
) authFixture {
	t.Helper()
	clock := clockwork.NewFakeClockAt(time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC))
	fixture := newAuthFixtureWithTime(t, start, clock.Now)
	fixture.clock = clock
	return fixture
}

func newAuthFixtureWithTime(
	t *testing.T,
	start func(*testing.T, cloud.Config, func(*cloud.App)) *remotetest.Replica,
	now func() time.Time,
) authFixture {
	t.Helper()
	db := remotetest.NewDatabase(t)
	db.Migrate(t)
	provider, endpoints := remotetest.StartIdentityProvider(t, now, nil, "cloud.test")
	oauth, err := auth.NewOAuth(
		auth.OAuthClient{ID: googleClient, Secret: "google-secret"},
		auth.OAuthClient{ID: "github-client", Secret: "github-secret"}, auth.WithOAuthEndpoints(endpoints),
	)
	if err != nil {
		t.Fatal(err)
	}
	check, err := provider.Turnstile()
	if err != nil {
		t.Fatal(err)
	}
	keys, err := api.NewSigningKeys(remotetest.NewSigner(t, "cloud-auth"), nil)
	if err != nil {
		t.Fatal(err)
	}
	config := cloud.Config{
		Postgres: db.Config(t, "auth1"), Keys: keys, Now: now, Issuer: authOrigin,
		Auth: &auth.Config{
			OAuth: oauth, EncryptionKey: bytes.Repeat([]byte{7}, 32), Turnstile: check,
			TurnstileSiteKey: "test-site-key", HostedDomain: "compozy.run", TrustEdge: true,
		},
	}
	conn, err := pgx.Connect(t.Context(), db.Config(t, "assert").DirectDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(t.Context()) })
	replica := start(t, config, registerAuthProbe)
	return authFixture{db: db, conn: conn, replica: replica, cfg: config, provider: provider}
}

const authProbePath = "/v1/test-device-principal"

func registerAuthProbe(app *cloud.App) {
	app.Routes().Handle("GET "+authProbePath, api.RequireDevice(
		app.Auth, app.AccountLimit, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, _ := api.DevicePrincipalFromContext(r.Context())
			api.WriteJSON(w, http.StatusOK, subject)
		}),
	))
}

type providerProfile struct {
	subject, email, issuer, audience, nonce string
	githubID                                int64
	verified                                bool
	authTime                                *time.Time
}

type authReply struct {
	*http.Response
	body []byte
}

func callAuth(t *testing.T, replica *remotetest.Replica, method, path string, input any,
	cookies []*http.Cookie, headers map[string]string,
) authReply {
	t.Helper()
	var body io.Reader
	contentType := "application/json"
	switch form, ok := input.(url.Values); {
	case ok:
		body, contentType = strings.NewReader(form.Encode()), "application/x-www-form-urlencoded"
	case input != nil:
		if data, err := json.Marshal(input); err != nil {
			t.Fatal(err)
		} else {
			body = bytes.NewReader(data)
		}
	}
	req, err := http.NewRequestWithContext(t.Context(), method, replica.Server.URL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	client := &http.Client{Transport: replica.Server.Client().Transport, Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return authReply{Response: response, body: data}
}

func (f authFixture) request(t *testing.T, method, path string, input any, cookies ...*http.Cookie) authReply {
	t.Helper()
	return callAuth(t, f.replica, method, path, input, cookies, nil)
}

func (r authReply) expect(t *testing.T, status int, fragment string) {
	t.Helper()
	if r.StatusCode != status || !bytes.Contains(r.body, []byte(fragment)) {
		t.Fatalf("HTTP status %d, wanted %d with %q; response length %d", r.StatusCode, status, fragment, len(r.body))
	}
}

func (r authReply) cookie(t *testing.T, name string) *http.Cookie {
	t.Helper()
	for _, c := range r.Cookies() {
		if c.Name == name {
			if !c.Secure || !c.HttpOnly || c.Path != "/" || c.Domain != "" || c.SameSite != http.SameSiteLaxMode {
				t.Fatalf("%s lacks host-only Secure/HttpOnly/Lax attributes", name)
			}
			return c
		}
	}
	t.Fatalf("response lacked %s cookie", name)
	return nil
}

func authJSON[A any](t *testing.T, reply authReply) A {
	t.Helper()
	var output A
	if err := json.Unmarshal(reply.body, &output); err != nil {
		t.Fatal("could not decode auth response")
	}
	return output
}

func newDeviceKey(t *testing.T) (*ecdsa.PrivateKey, remoteproto.JWK) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwk, err := remoteproto.NewPublicJWK(key.Public())
	if err != nil {
		t.Fatal(err)
	}
	return key, jwk
}
