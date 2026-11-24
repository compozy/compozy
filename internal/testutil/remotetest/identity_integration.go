//go:build integration

package remotetest

import (
	"cmp"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/v2"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/turnstile"
)

const (
	GoogleClient = "google-client"
	GoogleIssuer = "https://accounts.google.com"
)

type ProviderProfile struct {
	Subject, Name, Email, Issuer, Audience, Nonce string
	GitHubID                                      int64
	Verified                                      bool
	AuthTime                                      *time.Time
}

type providerExchange struct {
	profile   ProviderProfile
	challenge string
	nonce     string
}

type IdentityProvider struct {
	mu             sync.Mutex
	exchanges      map[string]providerExchange
	key            *rsa.PrivateKey
	now            func() time.Time
	Origin         string
	browserProfile *ProviderProfile
	turnstileHost  string
}

func StartIdentityProvider(
	t Fixture,
	now func() time.Time,
	browserProfile *ProviderProfile,
	turnstileHost string,
) (*IdentityProvider, auth.OAuthEndpoints) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &IdentityProvider{key: key, now: now, browserProfile: browserProfile, turnstileHost: turnstileHost,
		exchanges: make(map[string]providerExchange)}
	server := httptest.NewServer(http.HandlerFunc(p.serve))
	t.Cleanup(server.Close)
	p.Origin = server.URL
	return p, auth.OAuthEndpoints{
		GoogleAuthorize: p.Origin + "/google/authorize", GoogleToken: p.Origin + "/google/token",
		GoogleJWKS:      p.Origin + "/google/keys",
		GitHubAuthorize: p.Origin + "/github/authorize", GitHubToken: p.Origin + "/github/token",
		GitHubUser: p.Origin + "/github/user", GitHubEmails: p.Origin + "/github/emails",
	}
}

func (p *IdentityProvider) serve(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/google/authorize", "/github/authorize":
		p.authorize(w, r)
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
			"success": pass, "hostname": p.turnstileHost, "action": strings.TrimPrefix(r.Form.Get("response"), "pass-"),
		})
	default:
		http.NotFound(w, r)
	}
}

func (p *IdentityProvider) exchange(w http.ResponseWriter, r *http.Request) {
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

func (p *IdentityProvider) github(w http.ResponseWriter, r *http.Request) {
	profile := p.profile(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")).profile
	if profile.GitHubID == 0 {
		profile.GitHubID = 42
	}
	if r.URL.Path == "/github/user" {
		api.WriteJSON(
			w,
			http.StatusOK,
			map[string]any{"id": profile.GitHubID, "name": cmp.Or(profile.Name, "Test User")},
		)
		return
	}
	api.WriteJSON(w, http.StatusOK, []map[string]any{
		{"email": "other@example.test", "primary": false, "verified": true},
		{"email": profile.Email, "primary": true, "verified": profile.Verified},
	})
}

func (p *IdentityProvider) profile(code string) providerExchange {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exchanges[code]
}

func (p *IdentityProvider) googleJWT(exchange providerExchange) (string, error) {
	profile := exchange.profile
	now := p.now()
	if profile.Issuer == "" {
		profile.Issuer = GoogleIssuer
	}
	if profile.Audience == "" {
		profile.Audience = GoogleClient
	}
	if profile.Nonce == "" {
		profile.Nonce = exchange.nonce
	}
	claims := map[string]any{
		"iss": profile.Issuer, "aud": profile.Audience, "sub": profile.Subject, "nonce": profile.Nonce,
		"email": profile.Email, "email_verified": profile.Verified, "name": cmp.Or(profile.Name, "Test User"),
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}
	if profile.AuthTime != nil {
		claims["auth_time"] = profile.AuthTime.Unix()
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

// IssueCode registers an exchange for an observed authorization request.
func (p *IdentityProvider) IssueCode(query url.Values, profile ProviderProfile) string {
	code := auth.Secret()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.exchanges[code] = providerExchange{
		profile: profile, challenge: query.Get("code_challenge"), nonce: query.Get("nonce"),
	}
	return code
}

func (p *IdentityProvider) authorize(w http.ResponseWriter, r *http.Request) {
	if p.browserProfile == nil {
		http.NotFound(w, r)
		return
	}
	query := r.URL.Query()
	target, err := url.Parse(query.Get("redirect_uri"))
	// The fake accepts callbacks only on its local cloud listener or Docker's private bridge.
	if err != nil || target.Scheme != "http" || target.Hostname() != p.turnstileHost ||
		target.Path != "/v1/auth/callback/"+strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"), "/authorize") ||
		query.Get("state") == "" || query.Get("code_challenge") == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	callback := target.Query()
	profile := *p.browserProfile
	if query.Get("max_age") == "0" {
		profile.AuthTime = new(p.now())
	}
	callback.Set("code", p.IssueCode(query, profile))
	callback.Set("state", query.Get("state"))
	target.RawQuery = callback.Encode()
	// #nosec G710 -- limited to the fixture cloud's callback.
	http.Redirect(w, r, target.String(), http.StatusSeeOther)
}

func (p *IdentityProvider) Turnstile() (*turnstile.Client, error) {
	return turnstile.New("test-turnstile-secret", p.turnstileHost, turnstile.WithEndpoint(p.Origin+"/verify"))
}
