//go:build integration

package api_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/json/v2"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/email"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

func TestCloudDeviceSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should enroll with proof of possession and rotate and revoke only the bound session", func(t *testing.T) {
		t.Parallel()
		f := newAuthFixture(t)
		profile := providerProfile{email: "device@example.test", verified: true}
		token := newEnrollment(t, f, auth.GitHub, profile, false)
		key, jwk := newDeviceKey(t)
		other, _ := newDeviceKey(t)
		input := enrollInput(jwk)
		f.deviceRequest(t, http.MethodPost, enrollPath, token.EnrollmentToken, other, input).
			expect(t, http.StatusUnauthorized, "dpop_invalid")
		enrolled := f.enroll(t, token, key, input)
		if !enrolled.Admin || enrolled.DeviceID == "" {
			t.Fatal("first device was not enrolled as admin")
		}
		f.deviceRequest(t, http.MethodPost, enrollPath, token.EnrollmentToken, key, input).
			expect(t, http.StatusUnauthorized, "enrollment_token_invalid")
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE security_epoch = 2", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM device_keys WHERE state = 'active'", 1)
		assertAuthRow(t, f, `SELECT count(*) FROM security_events WHERE type = 'device_enrolled'
			AND details_json->>'location' = 'Lisbon, PT' AND details_json->>'device_name' = 'MacBook Pro'`, 1)

		headers := authProof(t, http.MethodGet, authProbePath, enrolled.AccessToken, key, f.clock.Now())
		callAuth(
			t,
			f.replica,
			http.MethodGet,
			authProbePath,
			nil,
			nil,
			headers,
		).expect(t, http.StatusOK, token.AccountID)
		peer := f.cfg
		peer.Postgres = f.db.Config(t, "auth2")
		second := remotetest.StartReplica(t, peer, registerAuthProbe)
		callAuth(t, second, http.MethodGet, authProbePath, nil, nil, headers).
			expect(t, http.StatusUnauthorized, "dpop_replayed")

		f.deviceRequest(t, http.MethodPost, refreshPath, enrolled.RefreshToken, other, nil).
			expect(t, http.StatusUnauthorized, "dpop_invalid")
		refresh := f.deviceRequest(t, http.MethodPost, refreshPath, enrolled.RefreshToken, key, nil)
		refresh.expect(t, http.StatusOK, "refresh_token")
		rotated := authJSON[remoteproto.DeviceSessionTokens](t, refresh)
		if rotated.RefreshToken == enrolled.RefreshToken || rotated.AccessToken == enrolled.AccessToken {
			t.Fatal("refresh did not issue distinct credentials")
		}
		f.deviceRequest(t, http.MethodPost, refreshPath, enrolled.RefreshToken, key, nil).
			expect(t, http.StatusUnauthorized, "session_invalid")
		f.deviceRequest(t, http.MethodPost, logoutPath, rotated.RefreshToken, key, nil).
			expect(t, http.StatusNoContent, "")
		f.deviceRequest(t, http.MethodGet, authProbePath, rotated.AccessToken, key, nil).
			expect(t, http.StatusUnauthorized, "session_invalid")
		f.deviceRequest(t, http.MethodPost, refreshPath, rotated.RefreshToken, key, nil).
			expect(t, http.StatusUnauthorized, "session_invalid")

		token = newEnrollment(t, f, auth.GitHub, profile, false)
		reused := f.enroll(t, token, key, input)
		if reused.DeviceID != enrolled.DeviceID || !reused.Admin {
			t.Fatal("new sign-in did not reuse the same device and key")
		}
		assertAuthRow(t, f, "SELECT count(*) FROM devices", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM security_events WHERE type = 'device_enrolled'", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE security_epoch = 2", 1)
	})

	t.Run("Should require fresh provider authentication to recover admin access", func(t *testing.T) {
		t.Parallel()
		f := newAuthFixture(t)
		profile := providerProfile{subject: "G1", email: "device@example.test", verified: true}
		key, jwk := newDeviceKey(t)
		token := newEnrollment(t, f, auth.Google, profile, false)
		first := f.enroll(t, token, key, enrollInput(jwk))
		if _, err := f.conn.Exec(t.Context(), "UPDATE devices SET revoked_at = $1", f.clock.Now()); err != nil {
			t.Fatal(err)
		}
		f.deviceRequest(t, http.MethodPost, refreshPath, first.RefreshToken, key, nil).
			expect(t, http.StatusUnauthorized, "device_revoked")
		token = newEnrollment(t, f, auth.Google, profile, false)
		f.deviceRequest(t, http.MethodPost, enrollPath, token.EnrollmentToken, key, enrollInput(jwk)).
			expect(t, http.StatusConflict, "device_key_revoked")
		other, otherJWK := newDeviceKey(t)
		if ordinary := f.enroll(t, token, other, enrollInput(otherJWK)); ordinary.Admin {
			t.Fatal("ordinary sign-in restored admin")
		}
		stale := f.clock.Now().Add(-time.Hour)
		profile.authTime = &stale
		reply, _ := f.signIn(t, auth.Google, profile, true)
		reply.expect(t, http.StatusForbidden, "fresh_sign_in_required")
		profile.authTime = new(f.clock.Now())
		token = newEnrollment(t, f, auth.Google, profile, true)
		recovered := f.enroll(t, token, other, enrollInput(otherJWK))
		if !recovered.Admin {
			t.Fatal("fresh sign-in could not restore admin")
		}
		assertAuthRow(t, f, "SELECT count(*) FROM security_events WHERE type = 'device_admin_changed'", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE security_epoch = 4", 1)
	})

	t.Run("Should give explicit reauthentication reasons for suspended and deleted accounts", func(t *testing.T) {
		t.Parallel()
		f := newAuthFixture(t)
		profile := providerProfile{email: "device@example.test", verified: true}
		key, jwk := newDeviceKey(t)
		token := newEnrollment(t, f, auth.GitHub, profile, false)
		d := f.enroll(t, token, key, enrollInput(jwk))
		if _, err := f.conn.Exec(t.Context(), "UPDATE accounts SET state = 'suspended'"); err != nil {
			t.Fatal(err)
		}
		f.deviceRequest(t, http.MethodPost, refreshPath, d.RefreshToken, key, nil).
			expect(t, http.StatusUnauthorized, `"reason":"account_suspended"`)
		reply, grant := f.signIn(t, auth.GitHub, profile, false)
		reply.expect(t, http.StatusForbidden, "account_suspended")
		f.poll(t, grant).expect(t, http.StatusForbidden, "account_suspended")

		if _, err := f.conn.Exec(t.Context(), "UPDATE accounts SET state = 'deleting'"); err != nil {
			t.Fatal(err)
		}
		f.deviceRequest(t, http.MethodPost, refreshPath, d.RefreshToken, key, nil).
			expect(t, http.StatusUnauthorized, `"reason":"account_deleted"`)
		if _, err := f.conn.Exec(t.Context(), `WITH deleted AS (DELETE FROM accounts RETURNING id)
			INSERT INTO deleted_accounts (account_id, deleted_at) SELECT id, now() FROM deleted`); err != nil {
			t.Fatal(err)
		}
		f.deviceRequest(t, http.MethodPost, refreshPath, d.RefreshToken, key, nil).
			expect(t, http.StatusUnauthorized, `"reason":"account_deleted"`)
	})

	t.Run("Should update news consent only through the authenticated patch or unsubscribe POST (UT-122)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariant 26. The database owns consent, with Resend synchronized after the explicit action.
		f, resend := newAccountEmailFixture(t, false)
		const address = "news@example.test"
		owner := newBillingDevice(t, f, address)
		patch := remoteproto.AccountEmailPatch{MarketingOptIn: new(true), Source: "settings"}
		f.request(t, http.MethodPatch, cloudAccountPath, patch).expect(t, http.StatusUnauthorized, "reauth_required")
		remoteCloudRequest(t, f, owner, http.MethodPatch, cloudAccountPath, patch).
			expect(t, http.StatusOK, `"marketing_opt_in":true`)
		assertAuthRow(t, f, `SELECT count(*) FROM accounts
			WHERE marketing_opt_in AND marketing_consent_at IS NOT NULL AND marketing_consent_source = 'settings'`, 1)
		if err := f.replica.App.Accounts.SweepAccountEmail(t.Context()); err != nil {
			t.Fatal(err)
		}
		link, err := url.Parse(resend.UnsubscribeURL(address))
		if err != nil || link.Path != email.UnsubscribePath || link.Query().Get("t") == "" {
			t.Fatal("Resend did not receive a per-account unsubscribe link")
		}
		path := link.RequestURI()
		f.request(t, http.MethodGet, path, nil).expect(t, http.StatusOK, "Unsubscribe from Compozy product news?")
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE marketing_opt_in", 1)
		form := url.Values{"List-Unsubscribe": {"One-Click"}}
		f.request(t, http.MethodPost, path+"tampered", form).expect(t, http.StatusBadRequest, "invalid")
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE marketing_opt_in", 1)
		f.request(t, http.MethodPost, path, form).expect(t, http.StatusOK, "product news and offers anymore.")
		assertAuthRow(t, f, `SELECT count(*) FROM accounts
			WHERE NOT marketing_opt_in AND marketing_consent_source = 'unsubscribe'`, 1)
		if err := f.replica.App.Accounts.SweepAccountEmail(t.Context()); err != nil {
			t.Fatal(err)
		}
		if resend.UnsubscribeURL(address) != "" {
			t.Fatal("unsubscribed contact stayed on the provider")
		}
		remoteCloudRequest(t, f, owner, http.MethodPatch, cloudAccountPath, patch).
			expect(t, http.StatusOK, `"marketing_opt_in":true`)
		// RFC 8058 allows multipart form data from the mail provider without any browser cookie.
		postRFC8058(t, f, path)
		assertAuthRow(t, f, `SELECT count(*) FROM accounts
			WHERE NOT marketing_opt_in AND marketing_consent_source = 'unsubscribe'`, 1)
	})

	t.Run("Should send one notice for a new key and require a fresh sign-in to revoke it (UT-132)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariants 22, 24 and 36. The notice is scoped, and its device revocation advances the epoch.
		f, resend := newAccountEmailFixture(t, false)
		p := providerProfile{subject: "notice-owner", email: "notice@example.test", verified: true}
		token := newEnrollment(t, f, auth.Google, p, false)
		key, jwk := newDeviceKey(t)
		first := f.enroll(t, token, key, enrollInput(jwk))
		owner := remoteTestDevice{key, first.AccessToken, first.DeviceID, token.AccountID}
		if err := f.replica.App.Accounts.SweepAccountEmail(t.Context()); err != nil {
			t.Fatal(err)
		}
		messages := resend.Messages()
		if len(messages) != 1 || messages[0].Subject != "New sign-in to Compozy on MacBook Pro (Lisbon, PT)" {
			t.Fatal("the first enrollment did not send the expected security notice")
		}
		token = newEnrollment(t, f, auth.Google, p, false)
		reuse := f.enroll(t, token, key, enrollInput(jwk))
		if reuse.DeviceID != first.DeviceID {
			t.Fatal("the same key did not reuse its device")
		}
		owner.access = reuse.AccessToken
		remoteCloudRequest(t, f, owner, http.MethodPost, cloudAccountPath+"/tokens", map[string]string{"name": "ci"}).
			expect(t, http.StatusCreated, "token")
		if err := f.replica.App.Accounts.SweepAccountEmail(t.Context()); err != nil || len(resend.Messages()) != 1 {
			t.Fatalf("a repeated enrollment or token creation generated another notice: %v", err)
		}
		for _, tc := range []struct {
			advance time.Duration
			want    int
		}{{6 * 24 * time.Hour, 1}, {2 * 24 * time.Hour, 0}} {
			f.clock.Advance(tc.advance)
			refresh := f.deviceRequest(t, http.MethodPost, refreshPath, reuse.RefreshToken, key, nil)
			refresh.expect(t, http.StatusOK, "refresh_token")
			reuse.DeviceSessionTokens = authJSON[remoteproto.DeviceSessionTokens](t, refresh)
			owner.access = reuse.AccessToken
			if account := billingAccount(t, f, owner); len(account.RecentSignIns) != tc.want ||
				tc.want == 1 && account.RecentSignIns[0].DeviceID != first.DeviceID {
				t.Fatalf("unexpected recent sign-ins: %+v", account.RecentSignIns)
			}
		}
		_, link, _ := strings.Cut(messages[0].Text, authOrigin)
		ordinary := purchaseWebSession(t, f, p)
		f.request(t, http.MethodPost, link, nil, ordinary).expect(t, http.StatusUnauthorized, "Sign in again")
		begin := f.request(t, http.MethodGet, link, nil, ordinary)
		begin.expect(t, http.StatusSeeOther, "")
		if redirectQuery(t, begin).Get("max_age") != "0" {
			t.Fatal("the ordinary web session bypassed fresh sign-in")
		}
		p.authTime = new(f.clock.Now())
		callback := f.callbackPath(t, auth.Google, begin, p)
		signedIn := f.request(t, http.MethodGet, callback, nil, begin.cookie(t, activationCookie))
		signedIn.expect(t, http.StatusSeeOther, link)
		fresh := signedIn.cookie(t, "__Host-compozy_notice")
		f.request(t, http.MethodGet, link, nil, fresh).expect(t, http.StatusOK, "Revoke this device")
		f.request(t, http.MethodPost, link, nil, fresh).expect(t, http.StatusOK, "was removed from your account")
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE security_epoch = 3", 1)
		f.deviceRequest(t, http.MethodPost, refreshPath, reuse.RefreshToken, key, nil).
			expect(t, http.StatusUnauthorized, `"reason":"device_revoked"`)
	})

	t.Run("Should revoke every credential and publish an empty snapshot during suspension (UT-135)", func(t *testing.T) {
		t.Parallel()
		// Safety Invariants 22, 31 and 34. Browser, device, token and computer credentials reach the same state.
		f := newAuthFixture(t)
		p := providerProfile{subject: "suspended-owner", email: "suspended@example.test", verified: true}
		enrollment := newEnrollment(t, f, auth.Google, p, false)
		key, jwk := newDeviceKey(t)
		first := f.enroll(t, enrollment, key, enrollInput(jwk))
		owner := remoteTestDevice{key, first.AccessToken, first.DeviceID, enrollment.AccountID}
		enrollment = newEnrollment(t, f, auth.Google, p, false)
		secondKey, secondJWK := newDeviceKey(t)
		f.enroll(t, enrollment, secondKey, enrollInput(secondJWK))
		pending := newEnrollment(t, f, auth.Google, p, false)
		webSession := purchaseWebSession(t, f, p)
		token := createLifecycleAccountToken(t, f, owner)
		computerKey := seedBrowserComputer(t, f, owner.account)
		f.db.Exec(t, "UPDATE %s.accounts SET security_epoch = 7 WHERE id = $1", owner.account)
		service := f.replica.App.Accounts
		identity, err := service.ComputerIdentity(t.Context(), "cmp_test")
		if err != nil {
			t.Fatal(err)
		}
		link, err := service.OpenLink(t.Context(), identity)
		if err != nil {
			t.Fatal(err)
		}
		if err := service.SuspendAccount(t.Context(), owner.account, "operator@example.test"); err != nil {
			t.Fatal(err)
		}
		assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE state = 'suspended' AND security_epoch = 8", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM device_sessions WHERE revoked_at IS NOT NULL", 2)
		assertAuthRow(t, f, "SELECT count(*) FROM cloud_web_sessions WHERE revoked_at IS NOT NULL", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM account_tokens WHERE revoked_at IS NOT NULL", 1)
		assertAuthRow(t, f, "SELECT count(*) FROM tunnels WHERE state = 'deleting'", 1)
		assertAuthRow(t, f, `SELECT count(*) FROM security_events
			WHERE type = 'account_suspended' AND actor = 'admin:operator@example.test'`, 1)
		if snapshot, err := service.LinkSnapshot(t.Context(), link); err != nil ||
			snapshot.SecurityEpoch != 8 || len(snapshot.Devices) != 0 || !snapshot.Linked {
			t.Fatalf("suspension snapshot = %+v: %v", snapshot, err)
		}
		thirdKey, thirdJWK := newDeviceKey(t)
		f.deviceRequest(t, http.MethodPost, enrollPath, pending.EnrollmentToken, thirdKey, enrollInput(thirdJWK)).
			expect(t, http.StatusForbidden, "account_suspended")
		for _, path := range []string{"/v1/computers/cmp_test/grants", "/v1/computers/reservations",
			"/v1/computers/link-tokens", "/v1/billing/checkout"} {
			remoteCloudRequest(t, f, owner, http.MethodPost, path, map[string]string{}).
				expect(t, http.StatusForbidden, "account_suspended")
		}
		f.request(t, http.MethodGet, "/billing/upgrade", nil, webSession).
			expect(t, http.StatusForbidden, "account_suspended")
		lifecycleComputerRequest(t, f, computerKey, "/v1/computers/cmp_test/tunnel", remoteproto.HostedTunnelRequest{}).
			expect(t, http.StatusForbidden, "account_suspended")
		const revalidation = "/v1/computers/cmp_test/revalidate"
		lifecycleComputerRequest(t, f, computerKey, revalidation, nil).
			expect(t, http.StatusUnauthorized, `"reason":"account_suspended"`)
		f.deviceRequest(t, http.MethodPost, refreshPath, first.RefreshToken, key, nil).
			expect(t, http.StatusUnauthorized, `"reason":"account_suspended"`)
		exchangeLifecycleAccountToken(t, f, token).expect(t, http.StatusUnauthorized, `"reason":"account_suspended"`)

		if err := service.UnsuspendAccount(t.Context(), owner.account, "operator@example.test"); err != nil {
			t.Fatal(err)
		}
		if snapshot, err := service.LinkSnapshot(t.Context(), link); err != nil ||
			snapshot.SecurityEpoch != 9 || len(snapshot.Devices) != 2 {
			t.Fatalf("unsuspension snapshot = %+v: %v", snapshot, err)
		}
		lifecycleComputerRequest(t, f, computerKey, revalidation, nil).expect(t, http.StatusNoContent, "")
		f.deviceRequest(t, http.MethodPost, refreshPath, first.RefreshToken, key, nil).
			expect(t, http.StatusUnauthorized, `"reason":"session_invalid"`)
		exchangeLifecycleAccountToken(t, f, token).expect(t, http.StatusUnauthorized, `"reason":"token_revoked"`)
		assertAuthRow(t, f, "SELECT count(*) FROM cloud_web_sessions WHERE revoked_at IS NOT NULL", 1)
		assertAuthRow(t, f, `SELECT count(*) FROM security_events
			WHERE type = 'account_unsuspended' AND actor = 'admin:operator@example.test'`, 1)
	})

	t.Run("Should count installations by source and activity window without counting a web session as an installation", func(t *testing.T) {
		t.Parallel()
		const metricsToken = "fixture-metrics"
		f := newAuthFixtureWithReplica(t, func(t *testing.T, cfg cloud.Config, register func(*cloud.App)) *remotetest.Replica {
			cfg.MetricsToken = metricsToken
			return remotetest.StartReplica(t, cfg, register)
		})
		owner := newBillingDevice(t, f, "account-metrics@example.test")
		browser := providerProfile{subject: "web-only", email: "web-only@example.test", verified: true}
		purchaseWebSession(t, f, browser)
		computer := remoteproto.ComputerID("cmp_metrics")
		f.db.SeedControlComputer(t, owner.account, computer, f.clock.Now())
		token := createLifecycleAccountToken(t, f, owner)
		exchangeLifecycleAccountToken(t, f, token).expect(t, http.StatusOK, owner.account)
		exchangeLifecycleAccountToken(t, f, token).expect(t, http.StatusOK, owner.account)
		f.request(t, http.MethodGet, "/metrics", nil).expect(t, http.StatusUnauthorized, "unauthorized")
		auth := map[string]string{"Authorization": "Bearer " + metricsToken}
		read := callAuth(t, f.replica, http.MethodGet, "/metrics", nil, nil, auth)
		read.expect(t, http.StatusOK, `cloud_accounts_total{provider="google"} 2`)
		read.expect(t, http.StatusOK, `cloud_accounts_active{window="1d"} 1`)
		for _, source := range []string{"session", "link", "token"} {
			read.expect(t, http.StatusOK, `cloud_installations_active{window="1d",source="`+source+`"} 1`)
		}
		f.clock.Advance(2 * 24 * time.Hour)
		read = callAuth(t, f.replica, http.MethodGet, "/metrics", nil, nil, auth)
		read.expect(t, http.StatusOK, `cloud_accounts_active{window="1d"} 0`)
		read.expect(t, http.StatusOK, `cloud_accounts_active{window="7d"} 1`)
		read.expect(t, http.StatusOK, `cloud_installations_active{window="30d",source="token"} 1`)
	})

}

func newEnrollment(t *testing.T, f authFixture, provider string, profile providerProfile, fresh bool,
) remoteproto.DeviceFlowTokenResponse {
	t.Helper()
	reply, grant := f.signIn(t, provider, profile, fresh)
	reply.expect(t, http.StatusOK, "Return to CompozyOS.")
	return f.redeem(t, grant)
}

func enrollInput(jwk remoteproto.JWK) remoteproto.DeviceEnrollRequest {
	return remoteproto.DeviceEnrollRequest{PublicJWK: jwk, Name: "MacBook Pro", Kind: "desktop",
		InstallationID: testInstallation}
}

func (f authFixture) enroll(t *testing.T, token remoteproto.DeviceFlowTokenResponse, key *ecdsa.PrivateKey,
	input remoteproto.DeviceEnrollRequest,
) remoteproto.DeviceEnrollResponse {
	t.Helper()
	reply := f.deviceRequest(t, http.MethodPost, enrollPath, token.EnrollmentToken, key, input)
	reply.expect(t, http.StatusOK, "device_id")
	return authJSON[remoteproto.DeviceEnrollResponse](t, reply)
}

func (f authFixture) deviceRequest(t *testing.T, method, path, token string, key *ecdsa.PrivateKey, input any,
) authReply {
	t.Helper()
	return callAuth(t, f.replica, method, path, input, nil, authProof(t, method, path, token, key, f.clock.Now()))
}

func authProof(t *testing.T, method, path, token string, key *ecdsa.PrivateKey, now time.Time) map[string]string {
	t.Helper()
	var access string
	if path == authProbePath {
		access = token
	}
	proof, err := remoteproto.BuildDPoP(key, method, authOrigin+path, access, now)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{"Authorization": "DPoP " + token, "DPoP": proof}
}

func postRFC8058(t *testing.T, f authFixture, path string) {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("List-Unsubscribe", "One-Click"); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, f.replica.Server.URL+path, &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	res, err := f.replica.Server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("RFC 8058 confirmation returned HTTP %d", res.StatusCode)
	}
}

func purchaseWebSession(t *testing.T, f authFixture, profile providerProfile) *http.Cookie {
	t.Helper()
	page := f.request(t, http.MethodGet, "/billing/upgrade", nil)
	page.expect(t, http.StatusOK, "Continue with Google")
	cookie := page.cookie(t, activationCookie)
	form := url.Values{
		"provider": {auth.Google}, "interval": {"year"}, "csrf": {cookie.Value},
		"cf-turnstile-response": {"pass-browser_authorize"},
	}
	begin := f.request(t, http.MethodPost, "/billing/upgrade/sign-in", form, cookie)
	begin.expect(t, http.StatusSeeOther, "")
	callback := f.callbackPath(t, auth.Google, begin, profile)
	result := f.request(t, http.MethodGet, callback, nil, cookie)
	result.expect(t, http.StatusSeeOther, "")
	return result.cookie(t, webSessionCookie)
}

func createLifecycleAccountToken(t *testing.T, f authFixture, device remoteTestDevice) string {
	t.Helper()
	result := remoteCloudRequest(t, f, device, http.MethodPost, cloudAccountPath+"/tokens", map[string]string{"name": "ci"})
	result.expect(t, http.StatusCreated, "token")
	return authJSON[struct {
		Token string `json:"token"`
	}](t, result).Token
}

func exchangeLifecycleAccountToken(t *testing.T, f authFixture, token string) authReply {
	t.Helper()
	return callAuth(t, f.replica, http.MethodPost, "/v1/account-tokens/exchange", map[string]string{
		"installation_id": "ins_ci_exchange",
	}, nil, map[string]string{"Authorization": "Bearer " + token})
}

func lifecycleComputerRequest(t *testing.T, f authFixture, key ed25519.PrivateKey, path string, in any) authReply {
	t.Helper()
	var body []byte
	if in != nil {
		var err error
		body, err = json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
	}
	proof, err := remoteproto.SignComputerProof(key, http.MethodPost, f.replica.App.Issuer+path, body, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	return callAuth(t, f.replica, http.MethodPost, path, in, nil, map[string]string{remoteproto.ComputerProofHeader: proof})
}
