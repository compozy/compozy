//go:build integration

package api_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/v2"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/remoteproto"
)

func TestCloudBrowserSignIn(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should fail closed before persisting OAuth or pending keys when Turnstile fails (UT-130)",
		func(t *testing.T) {
			t.Parallel()
			f := newAuthFixture(t)
			g := f.startGrant(t, auth.GitHub)
			cookie := f.openActivation(t, g, auth.GitHub)
			form := url.Values{"provider": {auth.GitHub}, "user_code": {g.UserCode}}
			for _, token := range []string{"", "fail", "pass-browser_authorize"} {
				form.Set("cf-turnstile-response", token)
				assertTurnstileFailure(t, f.request(t, http.MethodPost, activatePath, form, cookie))
				assertAuthRow(t, f, `SELECT count(*) FROM device_grants WHERE oauth_state_sha256 IS NOT NULL
				OR pkce_verifier_enc IS NOT NULL OR activation_nonce_sha256 IS NOT NULL`, 0)
			}
			request := browserForm(t)
			page := f.request(t, http.MethodGet, browserAuthorizePath+"?"+request.Encode(), nil)
			assertTurnstileFailure(t, page)
			request.Set("cf-turnstile-response", "fail")
			assertTurnstileFailure(
				t,
				f.request(t, http.MethodPost, browserAuthorizePath, request, page.cookie(t, activationCookie)),
			)
			assertAuthRow(t, f, "SELECT count(*) FROM cloud_oauth_attempts", 0)
			assertAuthRow(t, f, "SELECT count(*) FROM device_keys", 0)
			f.continueGrant(t, g, auth.GitHub, cookie, false)
			assertAuthRow(t, f, `SELECT count(*) FROM device_grants WHERE oauth_state_sha256 IS NOT NULL
			AND pkce_verifier_enc IS NOT NULL AND activation_nonce_sha256 IS NOT NULL`, 1)
		},
	)

	t.Run("Should reuse a web session and keep browser keys pending until redemption", func(t *testing.T) {
		t.Parallel()
		f := newAuthFixture(t)
		profile := providerProfile{subject: "G1", email: "browser@example.test", verified: true}
		reply, grant := f.signIn(t, auth.Google, profile, false)
		reply.expect(t, http.StatusOK, "Return to CompozyOS.")
		account := f.redeem(t, grant)
		seedBrowserComputer(t, f, account.AccountID)

		request := browserForm(t)
		first := f.request(t, http.MethodGet, browserAuthorizePath+"?"+request.Encode(), nil)
		assertTurnstileFailure(t, first)
		cookie := first.cookie(t, activationCookie)
		request.Set("cf-turnstile-response", "pass-browser_authorize")
		oauth := f.request(t, http.MethodPost, browserAuthorizePath, request, cookie)
		oauth.expect(t, http.StatusSeeOther, "")
		reply = f.request(t, http.MethodGet, f.callbackPath(t, auth.Google, oauth, profile), nil, cookie)
		assertBrowserRedirect(t, reply, request.Get("state"), "code")
		webCookie := reply.cookie(t, webSessionCookie)
		binding := reply.cookie(t, browserCookie)
		assertAuthRow(t, f, "SELECT count(*) FROM device_keys WHERE state = 'pending'", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM device_keys WHERE state = 'active'", 0)
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE security_epoch = 1", 1)

		request.Del("cf-turnstile-response")
		next := browserAuthorizePath + "?" + request.Encode()
		reply = f.request(t, http.MethodGet, next, nil, webCookie)
		assertBrowserRedirect(t, reply, request.Get("state"), "error")
		if got := redirectQuery(t, reply).Get("error"); got != "browser_key_rebind_required" {
			t.Fatalf("browser key moved across devices: %q", got)
		}
		for range 4 {
			f.clock.Advance(20 * 24 * time.Hour)
			reply = f.request(t, http.MethodGet, next, nil, webCookie, binding)
			assertBrowserRedirect(t, reply, request.Get("state"), "code")
		}
		assertAuthRow(t, f, "SELECT count(*) FROM cloud_web_sessions", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM devices", 1)
		f.clock.Advance(10 * 24 * time.Hour)
		assertTurnstileFailure(t, f.request(t, http.MethodGet, next, nil, webCookie, binding))
	})

	t.Run("Should activate only a proof-bound key and sign the resulting epoch (UT-114)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariants 22 and 24. The cloud transaction owns both the key and the signed grant epoch.
		f := newAuthFixture(t)
		_, deviceGrant := f.signIn(t, auth.Google, providerProfile{
			subject: "G1", email: "browser@example.test", verified: true,
		}, false)
		account := f.redeem(t, deviceGrant)
		computerKey := seedBrowserComputer(t, f, account.AccountID)
		form, verifier, jkt := browserAuthorization(t, "testbox-a1b2.compozy.run")
		callback := browserOAuth(t, f, form, "browser@example.test")
		code := redirectQuery(t, callback).Get("code")
		sessionReq := remoteproto.BrowserSessionRequest{Code: code, CodeVerifier: verifier}
		input := remoteproto.BrowserTokenRequest{
			BrowserSessionRequest: sessionReq,
			JKT:                   jkt,
		}
		assertAuthRow(t, f, "SELECT count(*) FROM device_keys WHERE state = 'pending'", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE security_epoch = 1", 1)

		wrongKey := input
		wrongKey.JKT = auth.Secret()
		redeemBrowser(t, f, computerKey, wrongKey).expect(t, http.StatusForbidden, "browser_key_mismatch")
		wrongVerifier := input
		wrongVerifier.CodeVerifier = auth.Secret()
		redeemBrowser(t, f, computerKey, wrongVerifier).expect(t, http.StatusForbidden, "browser_code_invalid")
		assertAuthRow(t, f, "SELECT count(*) FROM device_keys WHERE state = 'pending'", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE security_epoch = 1", 1)

		reply := redeemBrowser(t, f, computerKey, input)
		reply.expect(t, http.StatusOK, "grant")
		verifierJWS, err := remoteproto.NewJWSVerifier(authOrigin, f.cfg.Keys.JWKS(), nil)
		if err != nil {
			t.Fatal(err)
		}
		signed := authJSON[remoteproto.GrantResponse](t, reply)
		grant, err := verifierJWS.VerifyGrant(t.Context(), signed.Grant, f.clock.Now())
		if err != nil || grant.SecurityEpoch != 2 || !grant.Browser || grant.Route != remoteproto.RouteCloud ||
			grant.Cnf.JKT != jkt || grant.Audience != "cmp_test" {
			t.Fatalf("browser grant claims = %+v, error = %v", grant, err)
		}
		assertAuthRow(t, f, "SELECT count(*) FROM device_keys WHERE state = 'active' AND activated_at IS NOT NULL", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE security_epoch = 2", 1)
		f.clock.Advance(6 * time.Minute)
		redeemBrowser(t, f, computerKey, input).expect(t, http.StatusForbidden, "browser_code_used")

		response := f.request(
			t,
			http.MethodGet,
			browserAuthorizePath+"?"+form.Encode(),
			nil,
			callback.cookie(t, webSessionCookie),
		)
		assertBrowserRedirect(t, response, form.Get("state"), "error")
		if got := redirectQuery(t, response).Get("error"); got != "browser_key_rebind_required" {
			t.Fatalf("an active key must stay assigned to its original device, got %q", got)
		}
		assertAuthRow(t, f, "SELECT count(*) FROM devices", 1)
	})
}

func browserOAuth(t *testing.T, f authFixture, form url.Values, email string) authReply {
	t.Helper()
	page := f.request(t, http.MethodGet, browserAuthorizePath+"?"+form.Encode(), nil)
	assertTurnstileFailure(t, page)
	cookie := page.cookie(t, activationCookie)
	request := form.Clone()
	request.Set("cf-turnstile-response", "pass-browser_authorize")
	oauth := f.request(t, http.MethodPost, browserAuthorizePath, request, cookie)
	oauth.expect(t, http.StatusSeeOther, "")
	callback := f.callbackPath(t, auth.Google, oauth, providerProfile{subject: "G1", email: email, verified: true})
	return f.request(t, http.MethodGet, callback, nil, cookie)
}

func browserAuthorization(t *testing.T, hostname string) (url.Values, string, string) {
	t.Helper()
	_, key := newDeviceKey(t)
	data, err := json.Marshal(key)
	if err != nil {
		t.Fatal(err)
	}
	jkt, err := key.Thumbprint()
	if err != nil {
		t.Fatal(err)
	}
	verifier := auth.Secret()
	challenge := sha256.Sum256([]byte(verifier))
	return url.Values{
		"hostname": {hostname}, "public_jwk": {base64.RawURLEncoding.EncodeToString(data)},
		"state": {auth.Secret()}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"provider": {auth.Google},
	}, verifier, jkt
}

func redeemBrowser(
	t *testing.T,
	f authFixture,
	key ed25519.PrivateKey,
	input remoteproto.BrowserTokenRequest,
) authReply {
	t.Helper()
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := remoteproto.SignComputerProof(
		key,
		http.MethodPost,
		authOrigin+remoteproto.BrowserRedemptionPath,
		body,
		f.clock.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return callAuth(t, f.replica, http.MethodPost, remoteproto.BrowserRedemptionPath, input, nil,
		map[string]string{remoteproto.ComputerProofHeader: proof})
}

func browserForm(t *testing.T) url.Values {
	t.Helper()
	_, jwk := newDeviceKey(t)
	encoded, err := json.Marshal(jwk)
	if err != nil {
		t.Fatal(err)
	}
	return url.Values{
		"hostname": {"testbox-a1b2.compozy.run"}, "public_jwk": {base64.RawURLEncoding.EncodeToString(encoded)},
		"state": {auth.Secret()}, "code_challenge": {auth.Secret()}, "provider": {auth.Google},
	}
}

func assertTurnstileFailure(t *testing.T, response authReply) {
	t.Helper()
	response.expect(t, http.StatusBadRequest, "turnstile_required")
	if response.Header.Get("Location") != "" || bytes.Contains(response.body, []byte("test-turnstile-secret")) {
		t.Fatal("challenge failure redirected or returned private configuration")
	}
}

func assertBrowserRedirect(t *testing.T, reply authReply, state, result string) {
	t.Helper()
	reply.expect(t, http.StatusSeeOther, "")
	target, err := reply.Location()
	if err != nil || target.Scheme != "https" || target.Host != "testbox-a1b2.compozy.run" ||
		target.Path != "/remote/v1/browser/callback" || target.Query().Get("state") != state ||
		target.Query().Get(result) == "" {
		t.Fatal("invalid browser authorization redirect")
	}
}

func seedBrowserComputer(t *testing.T, f authFixture, accountID string) ed25519.PrivateKey {
	t.Helper()
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwk, err := remoteproto.NewPublicJWK(public)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(jwk)
	if err != nil {
		t.Fatal(err)
	}
	now := f.clock.Now()
	if _, err := f.conn.Exec(t.Context(), `INSERT INTO computers
		(id, account_id, name, platform, version, computer_jwk, tls_root_sha256, remote_enabled)
		VALUES ('cmp_test', $1, 'Test box', 'linux', '1', $2, 'root', true)`, accountID, data); err != nil {
		t.Fatal(err)
	}
	if _, err := f.conn.Exec(t.Context(), `INSERT INTO tunnels (computer_id, hostname, state)
		VALUES ('cmp_test', 'testbox-a1b2.compozy.run', 'active')`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.conn.Exec(t.Context(), `INSERT INTO subscriptions (id, source, account_id, status, ends_at)
		VALUES ('csub_test', 'complimentary', $1, 'active', $2)`, accountID, now.AddDate(1, 0, 0)); err != nil {
		t.Fatal(err)
	}
	return key
}
