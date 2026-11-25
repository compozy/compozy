// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration

package api_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const pollPath = deviceFlowPath + "/token"

func TestCloudDeviceGrant(t *testing.T) {
	t.Parallel()

	t.Run("Should bind activation and redeem a grant once (UT-117)", func(t *testing.T) {
		t.Parallel()
		f := newAuthFixture(t)
		g := f.startGrant(t, auth.GitHub)
		if len(strings.ReplaceAll(g.UserCode, "-", "")) != 8 || g.Interval != 5 || g.ExpiresIn != 900 ||
			!strings.Contains(g.VerificationURIComplete, "provider=github") {
			t.Fatal("invalid device-grant activation response")
		}
		f.request(t, http.MethodPost, deviceFlowPath, map[string]any{"marketing_opt_in": true}).
			expect(t, http.StatusBadRequest, "invalid_request")
		first := f.request(t, http.MethodGet, activationURL(g.UserCode, auth.GitHub, false), nil)
		first.expect(t, http.StatusOK, g.UserCode)
		first.expect(t, http.StatusOK, "Requested from Lisbon, PT")
		first.expect(t, http.StatusOK, "Only continue if you started this sign-in on your own computer.")
		// UT-132, Safety Invariant 30. The edge's coarse location is persisted without the requesting IP.
		assertAuthRow(t, f, `SELECT count(*) FROM device_grants WHERE request_location = 'Lisbon, PT'`, 1)
		cookie := first.cookie(t, activationCookie)
		redirect := f.continueGrant(t, g, auth.GitHub, cookie, false)
		authorize := redirectQuery(t, redirect)
		if authorize.Get("nonce") != "" || authorize.Get("code_challenge_method") != "S256" {
			t.Fatal("invalid GitHub authorization parameters")
		}
		f.poll(t, g).expect(t, http.StatusBadRequest, "slow_down")
		f.clock.Advance(10 * time.Second)
		f.poll(t, g).expect(t, http.StatusBadRequest, "authorization_pending")
		complete := f.request(t, http.MethodGet, f.callbackPath(t, auth.GitHub, redirect, providerProfile{
			email: "dev@example.test", verified: true,
		}), nil, cookie)
		complete.expect(t, http.StatusOK, "Return to CompozyOS.")
		for _, c := range complete.Cookies() {
			if c.Name == webSessionCookie {
				t.Fatal("device activation created a cloud web session")
			}
		}
		f.clock.Advance(10 * time.Second)
		reply := f.poll(t, g)
		reply.expect(t, http.StatusOK, "enrollment_token")
		token := authJSON[remoteproto.DeviceFlowTokenResponse](t, reply)
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE marketing_opt_in", 0)
		if token.AccountID == "" || token.Email != "dev@example.test" || !token.NewAccount {
			t.Fatal("invalid enrollment response")
		}
		f.poll(t, g).expect(t, http.StatusBadRequest, "expired_token")
	})

	t.Run("Should stop expired and denied grants (UT-117)", func(t *testing.T) {
		t.Parallel()
		f := newAuthFixture(t)
		expired := f.startGrant(t, "")
		f.clock.Advance(15 * time.Minute)
		f.poll(t, expired).expect(t, http.StatusBadRequest, "expired_token")
		g := f.startGrant(t, auth.Google)
		cookie := f.openActivation(t, g, auth.Google)
		redirect := f.continueGrant(t, g, auth.Google, cookie, false)
		authorize := redirectQuery(t, redirect)
		if authorize.Get("nonce") == "" {
			t.Fatal("Google authorization lacked a nonce")
		}
		path := "/v1/auth/callback/google?" + url.Values{
			"state": {authorize.Get("state")}, "error": {"access_denied"},
		}.Encode()
		f.request(t, http.MethodGet, path, nil, cookie).expect(t, http.StatusBadRequest, "access_denied")
		f.poll(t, g).expect(t, http.StatusBadRequest, "access_denied")
		assertAuthRow(t, f, "SELECT count(*) FROM enrollment_tokens", 0)
	})

	t.Run("Should allow a fresh browser when old code-entry windows fill the cache", func(t *testing.T) {
		// RC-16: A full bounded cache must preserve activation for browsers with no failures.
		t.Parallel()
		f := newAuthFixture(t)
		find := f.replica.App.Auth.FindActivation
		for i := range 16_384 {
			if _, err := find(t.Context(), strconv.Itoa(i), ""); err == nil || err.Error() != "user_code_invalid" {
				t.Fatalf("recording a browser code failure: %v", err)
			}
		}
		g := f.startGrant(t, auth.GitHub)
		if grant, err := find(t.Context(), "fresh-browser", g.UserCode); err != nil || grant.UserCode != g.UserCode {
			t.Fatalf("a fresh browser could not resolve its code: %v", err)
		}
		if _, err := find(t.Context(), "fresh-browser", ""); err == nil || err.Error() != "user_code_invalid" {
			t.Fatalf("a fresh browser could not record its first mistake: %v", err)
		}
	})

	t.Run("Should authorize one racing browser and refuse invalid callback bindings (UT-123)", func(t *testing.T) {
		t.Parallel()
		f := newAuthFixture(t)
		g := f.startGrant(t, auth.GitHub)
		c1, c2 := f.openActivation(t, g, auth.GitHub), f.openActivation(t, g, auth.GitHub)
		profile := providerProfile{email: "race@example.test", verified: true}
		callback1 := f.callbackPath(t, auth.GitHub, f.continueGrant(t, g, auth.GitHub, c1, false), profile)
		callback2 := f.callbackPath(t, auth.GitHub, f.continueGrant(t, g, auth.GitHub, c2, false), profile)
		f.request(t, http.MethodGet, callback1, nil, c2).expect(t, http.StatusBadRequest, "sign_in_state_invalid")
		wrongProvider := strings.Replace(callback1, "/callback/github", "/callback/google", 1)
		f.request(t, http.MethodGet, wrongProvider, nil, c1).expect(t, http.StatusBadRequest, "sign_in_state_invalid")
		var replies [2]authReply
		t.Run("Should complete both callbacks against the same grant", func(t *testing.T) {
			for i, callback := range []string{callback1, callback2} {
				t.Run([]string{"First browser", "Second browser"}[i], func(t *testing.T) {
					t.Parallel()
					replies[i] = f.request(t, http.MethodGet, callback, nil, []*http.Cookie{c1, c2}[i])
				})
			}
		})
		winner, loser := replies[0], replies[1]
		if winner.StatusCode == http.StatusConflict {
			winner, loser = loser, winner
		}
		winner.expect(t, http.StatusOK, "Return to CompozyOS.")
		loser.expect(t, http.StatusConflict, "This code was already used.")
		f.request(t, http.MethodGet, callback1, nil, c1).expect(t, http.StatusBadRequest, "sign_in_state_invalid")
		assertAuthRow(t, f, "SELECT count(*) FROM security_events WHERE type = 'sign_in'", 1)
	})

	t.Run("Should preserve cancellation and limit five wrong entries per browser (UT-123)", func(t *testing.T) {
		t.Parallel()
		f := newAuthFixture(t)
		g := f.startGrant(t, auth.GitHub)
		cookie := f.openActivation(t, g, auth.GitHub)
		redirect := f.continueGrant(t, g, auth.GitHub, cookie, false)
		f.request(t, http.MethodDelete, deviceFlowPath+"/"+g.DeviceCode, nil).expect(t, http.StatusNoContent, "")
		path := f.callbackPath(t, auth.GitHub, redirect, providerProfile{email: "dev@example.test", verified: true})
		f.request(t, http.MethodGet, path, nil, cookie).expect(t, http.StatusBadRequest, "access_denied")
		f.poll(t, g).expect(t, http.StatusBadRequest, "access_denied")
		for range 5 {
			f.request(t, http.MethodGet, activatePath+"?user_code=wrong", nil, cookie).
				expect(t, http.StatusBadRequest, "user_code_invalid")
		}
		limited := f.request(t, http.MethodGet, activatePath+"?user_code=wrong", nil, cookie)
		limited.expect(t, http.StatusTooManyRequests, "rate_limited")
		if limited.Header.Get("Retry-After") != "900" {
			t.Fatal("code-entry rate limit lacked a retry window")
		}
		f.clock.Advance(15 * time.Minute)
		f.request(t, http.MethodGet, activatePath+"?user_code=wrong", nil, cookie).
			expect(t, http.StatusBadRequest, "user_code_invalid")
	})
}

func (f authFixture) startGrant(t *testing.T, provider string) remoteproto.DeviceFlowResponse {
	t.Helper()
	reply := callAuth(t, f.replica, http.MethodPost, deviceFlowPath, remoteproto.DeviceFlowRequest{
		Provider: provider, InstallationID: testInstallation, ClientName: "MacBook Pro", ClientPlatform: "macOS",
	}, nil, map[string]string{"X-Compozy-Location": "Lisbon, Lisboa, PT", "X-Compozy-Client-IP": "192.0.2.9"})
	reply.expect(t, http.StatusOK, "device_code")
	return authJSON[remoteproto.DeviceFlowResponse](t, reply)
}

func activationURL(code, provider string, fresh bool) string {
	return activatePath + "?" + auth.ActivationQuery(code, provider, fresh)
}

func (f authFixture) openActivation(t *testing.T, g remoteproto.DeviceFlowResponse, provider string) *http.Cookie {
	t.Helper()
	reply := f.request(t, http.MethodGet, activationURL(g.UserCode, provider, false), nil)
	reply.expect(t, http.StatusOK, g.UserCode)
	return reply.cookie(t, activationCookie)
}

func (f authFixture) continueGrant(t *testing.T, g remoteproto.DeviceFlowResponse,
	provider string, cookie *http.Cookie, fresh bool,
) authReply {
	t.Helper()
	form := url.Values{"user_code": {g.UserCode}, "provider": {provider}, "cf-turnstile-response": {"pass-activate"}}
	if fresh {
		form.Set("max_age", "0")
	}
	reply := f.request(t, http.MethodPost, activatePath, form, cookie)
	reply.expect(t, http.StatusSeeOther, "")
	location, err := reply.Location()
	if err != nil || !strings.HasPrefix(location.String(), f.provider.Origin+"/") {
		t.Fatal("activation did not redirect to the configured provider")
	}
	return reply
}

func redirectQuery(t *testing.T, response authReply) url.Values {
	t.Helper()
	target, err := response.Location()
	if err != nil {
		t.Fatal("no redirect target")
	}
	return target.Query()
}

func (f authFixture) callbackPath(t *testing.T, provider string, redirect authReply, profile providerProfile) string {
	t.Helper()
	query := redirectQuery(t, redirect)
	if query.Get("state") == "" || query.Get("code_challenge") == "" {
		t.Fatal("provider request lacked OAuth state or PKCE")
	}
	code := f.provider.IssueCode(query, remotetest.ProviderProfile{
		Subject: profile.subject, Email: profile.email, Issuer: profile.issuer, Audience: profile.audience,
		Nonce: profile.nonce, GitHubID: profile.githubID, Verified: profile.verified, AuthTime: profile.authTime,
	})
	return "/v1/auth/callback/" + provider + "?" + url.Values{"state": {query.Get("state")}, "code": {code}}.Encode()
}

func (f authFixture) poll(t *testing.T, g remoteproto.DeviceFlowResponse) authReply {
	t.Helper()
	return f.request(t, http.MethodPost, pollPath, remoteproto.DeviceFlowTokenRequest{DeviceCode: g.DeviceCode})
}

func (f authFixture) signIn(
	t *testing.T,
	provider string,
	profile providerProfile,
	fresh bool,
) (authReply, remoteproto.DeviceFlowResponse) {
	t.Helper()
	g := f.startGrant(t, provider)
	cookie := f.openActivation(t, g, provider)
	redirect := f.continueGrant(t, g, provider, cookie, fresh)
	if fresh && redirectQuery(t, redirect).Get("max_age") != "0" {
		t.Fatal("fresh Google authorization lacked max_age=0")
	}
	reply := f.request(t, http.MethodGet, f.callbackPath(t, provider, redirect, profile), nil, cookie)
	return reply, g
}

func (f authFixture) redeem(t *testing.T, g remoteproto.DeviceFlowResponse) remoteproto.DeviceFlowTokenResponse {
	t.Helper()
	f.clock.Advance(5 * time.Second)
	reply := f.poll(t, g)
	reply.expect(t, http.StatusOK, "enrollment_token")
	return authJSON[remoteproto.DeviceFlowTokenResponse](t, reply)
}

func assertAuthRow(t *testing.T, f authFixture, statement string, expected int) {
	t.Helper()
	var actual int
	if err := f.conn.QueryRow(t.Context(), statement).Scan(&actual); err != nil || actual != expected {
		t.Fatalf("auth state = %d, expected %d (%v) for %s", actual, expected, err, statement)
	}
}
