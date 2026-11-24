//go:build integration

package api_test

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/v2"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonboulle/clockwork"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/turnstile"
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
	googleClient         = "google-client"
	googleIssuer         = "https://accounts.google.com"
	testInstallation     = "test-installation"
)

type authFixture struct {
	db       *remotetest.Database
	conn     *pgx.Conn
	replica  *remotetest.Replica
	cfg      cloud.Config
	clock    *clockwork.FakeClock
	provider *providerFixture
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
	provider, endpoints := newProviderFixture(t, now)
	oauth, err := auth.NewOAuth(
		auth.OAuthClient{ID: googleClient, Secret: "google-secret"},
		auth.OAuthClient{ID: "github-client", Secret: "github-secret"}, auth.WithOAuthEndpoints(endpoints),
	)
	if err != nil {
		t.Fatal(err)
	}
	check, err := turnstile.New(
		"test-turnstile-secret",
		"cloud.test",
		turnstile.WithEndpoint(provider.origin+"/verify"),
	)
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

type providerExchange struct {
	profile   providerProfile
	challenge string
	nonce     string
}

type providerFixture struct {
	mu        sync.Mutex
	exchanges map[string]providerExchange
	key       *rsa.PrivateKey
	now       func() time.Time
	origin    string
}

func newProviderFixture(t *testing.T, now func() time.Time) (*providerFixture, auth.OAuthEndpoints) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &providerFixture{key: key, now: now, exchanges: make(map[string]providerExchange)}
	server := httptest.NewServer(http.HandlerFunc(p.serve))
	t.Cleanup(server.Close)
	p.origin = server.URL
	return p, auth.OAuthEndpoints{
		GoogleAuthorize: p.origin + "/google/authorize", GoogleToken: p.origin + "/google/token",
		GoogleJWKS:      p.origin + "/google/keys",
		GitHubAuthorize: p.origin + "/github/authorize", GitHubToken: p.origin + "/github/token",
		GitHubUser: p.origin + "/github/user", GitHubEmails: p.origin + "/github/emails",
	}
}

func (p *providerFixture) serve(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/google/keys":
		api.WriteJSON(w, http.StatusOK, map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "test",
			"n": base64.RawURLEncoding.EncodeToString(p.key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(p.key.E)).Bytes()),
		}}})
	case "/google/token", "/github/token":
		p.exchange(w, r)
	case "/github/user", "/github/emails":
		p.github(w, r)
	case "/verify":
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		pass := strings.HasPrefix(r.Form.Get("response"), "pass-")
		api.WriteJSON(w, http.StatusOK, map[string]any{
			"success": pass, "hostname": "cloud.test", "action": strings.TrimPrefix(r.Form.Get("response"), "pass-"),
		})
	default:
		http.NotFound(w, r)
	}
}

func (p *providerFixture) exchange(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	code := r.Form.Get("code")
	exchange := p.profile(code)
	actual := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
	if exchange.challenge == "" || base64.RawURLEncoding.EncodeToString(actual[:]) != exchange.challenge {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if r.URL.Path == "/github/token" {
		api.WriteJSON(w, http.StatusOK, map[string]string{"access_token": code})
		return
	}
	token, err := p.googleJWT(exchange)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	api.WriteJSON(w, http.StatusOK, map[string]string{"id_token": token})
}

func (p *providerFixture) github(w http.ResponseWriter, r *http.Request) {
	profile := p.profile(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")).profile
	if profile.githubID == 0 {
		profile.githubID = 42
	}
	if r.URL.Path == "/github/user" {
		api.WriteJSON(w, http.StatusOK, map[string]any{"id": profile.githubID, "name": "Test User"})
		return
	}
	api.WriteJSON(w, http.StatusOK, []map[string]any{
		{"email": "other@example.test", "primary": false, "verified": true},
		{"email": profile.email, "primary": true, "verified": profile.verified},
	})
}

func (p *providerFixture) profile(code string) providerExchange {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exchanges[code]
}

func (p *providerFixture) googleJWT(exchange providerExchange) (string, error) {
	profile := exchange.profile
	now := p.now()
	if profile.issuer == "" {
		profile.issuer = googleIssuer
	}
	if profile.audience == "" {
		profile.audience = googleClient
	}
	if profile.nonce == "" {
		profile.nonce = exchange.nonce
	}
	claims := map[string]any{
		"iss": profile.issuer, "aud": profile.audience, "sub": profile.subject, "nonce": profile.nonce,
		"email": profile.email, "email_verified": profile.verified, "name": "Test User",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}
	if profile.authTime != nil {
		claims["auth_time"] = profile.authTime.Unix()
	}
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	b64 := base64.RawURLEncoding
	input := b64.EncodeToString([]byte(`{"alg":"RS256","kid":"test"}`)) + "." + b64.EncodeToString(body)
	hash := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(nil, p.key, crypto.SHA256, hash[:])
	return input + "." + b64.EncodeToString(sig), err
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
