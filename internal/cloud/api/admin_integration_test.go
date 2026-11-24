// SPDX-License-Identifier: FSL-1.1-MIT

//go:build integration

package api_test

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/compozy/compozy/internal/cloud"
	"github.com/compozy/compozy/internal/cloud/admin"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/testutil/remotetest"
)

const (
	adminOperator = "operator@example.test"
	adminTeam     = "fixture.cloudflareaccess.com"
	adminAudience = "admin-test"
	accessHeader  = "Cf-Access-Jwt-Assertion"
)

type adminFixture struct {
	authFixture
	key *rsa.PrivateKey
}

type adminKeyTransport struct {
	client *http.Client
	origin *url.URL
}

func (tr adminKeyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != adminTeam || r.URL.Path != "/cdn-cgi/access/certs" || r.URL.Scheme != "https" {
		return nil, fmt.Errorf("unexpected Access key request: %s %s", r.Method, r.URL)
	}
	request := r.Clone(r.Context())
	request.URL.Scheme, request.URL.Host = tr.origin.Scheme, tr.origin.Host
	request.Host = tr.origin.Host
	return tr.client.Do(request)
}

func adminAccessFixture(t *testing.T) (*admin.AccessConfig, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encoder := base64.RawURLEncoding
	jwks := struct {
		Keys []map[string]string `json:"keys"`
	}{[]map[string]string{{
		"kid": "known", "kty": "RSA", "alg": "RS256", "use": "sig",
		"n": encoder.EncodeToString(key.N.Bytes()),
		"e": encoder.EncodeToString(new(big.Int).SetInt64(int64(key.E)).Bytes()),
	}}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.MarshalWrite(w, jwks); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	base, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &admin.AccessConfig{
		TeamDomain: adminTeam, Audience: adminAudience, Allowlist: []string{adminOperator},
		Transport: adminKeyTransport{client: server.Client(), origin: base},
	}, key
}

func newAdminFixture(t *testing.T, change func(*cloud.Config)) adminFixture {
	t.Helper()
	access, key := adminAccessFixture(t)
	f := newAuthFixtureWithReplica(
		t,
		func(t *testing.T, cfg cloud.Config, register func(*cloud.App)) *remotetest.Replica {
			cfg.Admin = access
			if change != nil {
				change(&cfg)
			}
			return remotetest.StartLocalReplica(t, cfg, register)
		},
	)
	return adminFixture{authFixture: f, key: key}
}

func (f adminFixture) assertion(t *testing.T, email, issuer, audience, kid string, exp time.Time) string {
	t.Helper()
	encoder := base64.RawURLEncoding
	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": kid})
	if err != nil {
		t.Fatal(err)
	}
	now := f.clock.Now()
	payload, err := json.Marshal(map[string]any{
		"iss": issuer, "aud": []string{audience}, "email": email, "sub": "operator-access-subject",
		"type": "app", "iat": now.Add(-2 * time.Minute).Unix(), "nbf": now.Add(-time.Minute).Unix(), "exp": exp.Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	input := encoder.EncodeToString(header) + "." + encoder.EncodeToString(payload)
	digest := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + encoder.EncodeToString(sig)
}

func (f adminFixture) requestAdmin(t *testing.T, method, path string, input any) authReply {
	t.Helper()
	assertion := f.assertion(
		t,
		adminOperator,
		"https://"+adminTeam,
		adminAudience,
		"known",
		f.clock.Now().Add(time.Hour),
	)
	return callAuth(t, f.replica, method, path, input, nil, map[string]string{accessHeader: assertion})
}

func (f adminFixture) action(t *testing.T, form url.Values, action, kind string) authReply {
	t.Helper()
	form.Set("confirm", "yes")
	reply := f.requestAdmin(t, http.MethodPost, "/admin/actions/"+action, form)
	if reply.StatusCode != http.StatusSeeOther {
		reply.expect(t, http.StatusSeeOther, kind)
	}
	location := reply.Header.Get("Location")
	receipt := f.requestAdmin(t, http.MethodGet, location, nil)
	receipt.expect(t, http.StatusOK, kind)
	receipt.expect(t, http.StatusOK, "admin:"+adminOperator)
	return reply
}

func TestCloudAdmin(t *testing.T) {
	t.Run("Should refuse and log missing or unverifiable operator access (IT-108)", func(t *testing.T) {
		// Safety Invariants 31 and 36. Verification is against the test team's JWKS, never the account's application auth.
		f := newAdminFixture(t, nil)
		expires := f.clock.Now().Add(time.Hour)
		unknown := f
		var err error
		unknown.key, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		for index, tc := range []struct{ name, assertion string }{
			{"Should refuse a missing assertion", ""},
			{"Should refuse an unknown signing key", unknown.assertion(t, adminOperator, "https://"+adminTeam, adminAudience, "unknown", expires)},
			{"Should refuse a bad signature with the known key ID", unknown.assertion(t, adminOperator, "https://"+adminTeam, adminAudience, "known", expires)},
			{"Should refuse the wrong audience", f.assertion(t, adminOperator, "https://"+adminTeam, "other", "known", expires)},
			{"Should refuse an unlisted email", f.assertion(t, "other@example.test", "https://"+adminTeam, adminAudience, "known", expires)},
			{"Should refuse the wrong team", f.assertion(t, adminOperator, "https://other.cloudflareaccess.com", adminAudience, "known", expires)},
			{"Should refuse an expired assertion", f.assertion(t, adminOperator, "https://"+adminTeam, adminAudience, "known", f.clock.Now())},
		} {
			t.Run(tc.name, func(t *testing.T) {
				headers := make(map[string]string)
				if tc.assertion != "" {
					headers[accessHeader] = tc.assertion
				}
				callAuth(t, f.replica, http.MethodGet, "/admin", nil, nil, headers).
					expect(t, http.StatusForbidden, "Admin access denied.")
				assertAuthRow(
					t,
					f.authFixture,
					"SELECT count(*) FROM security_events WHERE type = 'admin_access_denied'",
					index+1,
				)
			})
		}
		f.requestAdmin(t, http.MethodGet, "/admin", nil).expect(t, http.StatusOK, "Gate policy")
		f.request(t, http.MethodPost, "/admin/contacts.csv", nil).
			expect(t, http.StatusForbidden, "Admin access denied.")
		assertion := f.assertion(t, adminOperator, "https://"+adminTeam, adminAudience, "known", expires)
		callAuth(t, f.replica, http.MethodPost, "/admin/contacts.csv", nil, nil,
			map[string]string{accessHeader: assertion, "Sec-Fetch-Site": "cross-site"}).
			expect(t, http.StatusForbidden, "Admin access denied.")
		assertAuthRow(t, f.authFixture, "SELECT count(*) FROM security_events WHERE type = 'contacts_exported'", 0)
	})

	t.Run("Should change entitlement and take down only the confirmed tunnel generation (IT-108)", func(t *testing.T) {
		// Safety Invariants 7, 9 and 31. Exercise browser actions through the real API, control link and provider client.
		fake := newHostedFakeCloudflare(t)
		f := newAdminFixture(t, func(cfg *cloud.Config) { cfg.HostedTunnels = fake.client })
		owner := newBillingDevice(t, f.authFixture, "admin-hosted@example.test")
		t.Setenv("COMPOZY_CLOUD_URL", f.replica.Server.URL)
		client, err := cloudclient.New(cloudclient.WithClock(f.clock.Now))
		if err != nil {
			t.Fatal(err)
		}
		h := hostedCloudTest{f: f.authFixture, owner: owner, fake: fake, client: client}
		computer := h.computer(t, 1)
		link := remotetest.AuthenticateControl(t, f.replica, computer.id, computer.key)
		snapshot := remotetest.ReadControlMessage(t, link, 2*time.Second)
		if snapshot.Type != remoteproto.ControlSnapshot {
			t.Fatalf("first control frame = %s", snapshot.Type)
		}
		remotetest.SendControl(
			t,
			link,
			remoteproto.ControlAck,
			remoteproto.SecurityAck{SecurityEpoch: snapshot.SecurityEpoch},
		)
		first := remotetest.ReadControlMessage(t, link, 2*time.Second)
		if first.Type != remoteproto.ControlLease {
			t.Fatalf("first lease frame = %s", first.Type)
		}
		expires := f.clock.Now().Add(time.Hour)
		f.action(t, url.Values{"account_id": {owner.account}, "ends_at": {expires.Format(time.RFC3339)}},
			"complimentary-pro", "complimentary_pro_granted")
		if account := billingAccount(t, f.authFixture, owner); account.Plan != "pro" || account.Subscription == nil ||
			account.Subscription.Source != "complimentary" {
			t.Fatalf("account summary after grant = %+v", account.AccountPlan)
		}
		assertAdminProLease(t, f, link)

		firstTunnel := h.provision(t, computer)
		for index := 2; index <= 10; index++ {
			h.provision(t, h.computer(t, index))
		}
		eleventh := h.computer(t, 11)
		_, err = client.ProvisionHostedTunnel(t.Context(), eleventh.id, eleventh.key, "")
		expectHostedRefusal(t, err, http.StatusPaymentRequired, "cloud_routes_limit_reached")
		f.action(
			t,
			url.Values{"account_id": {owner.account}, "cap": {"12"}},
			"hosted-route-cap",
			"hosted_route_cap_changed",
		)
		h.provision(t, eleventh)

		f.requestAdmin(t, http.MethodGet, "/admin/search?q="+url.QueryEscape(firstTunnel.Hostname), nil).
			expect(t, http.StatusOK, "Delete tunnel")
		f.requestAdmin(t, http.MethodGet, "/admin/confirm/delete-tunnel?target="+url.QueryEscape(firstTunnel.Hostname), nil).
			expect(t, http.StatusOK, "admin-hosted@example.test")
		form := url.Values{
			"hostname": {firstTunnel.Hostname}, "generation": {"1"}, "account_id": {owner.account}, "confirm": {"yes"},
		}
		if err := client.DeleteHostedTunnel(t.Context(), computer.id, computer.key); err != nil {
			t.Fatal(err)
		}
		h.provision(t, computer)
		f.requestAdmin(t, http.MethodPost, "/admin/actions/delete-tunnel", form).
			expect(t, http.StatusBadRequest, "Invalid request")
		h.state(t, computer, "active")
		form.Set("generation", "2")
		fake.failNextDelete()
		f.action(t, form, "delete-tunnel", "hosted_tunnel_takedown")
		if err := f.replica.App.Accounts.SweepHostedTunnels(t.Context()); err != nil {
			t.Fatal(err)
		}
		h.state(t, computer, "deleted")
		if fake.target(firstTunnel.Hostname).Origin != "" {
			t.Fatal("the taken-down hostname still points to a Cloudflare tunnel")
		}
		_, err = client.ProvisionHostedTunnel(t.Context(), computer.id, computer.key, "")
		expectHostedRefusal(t, err, http.StatusForbidden, "hosted_route_taken_down")
	})

	t.Run("Should scope exports and audit the operator's privacy and gate actions (IT-108)", func(t *testing.T) {
		var stripe *remotetest.StripeFixture
		f := newAdminFixture(t, func(cfg *cloud.Config) {
			stripe = remotetest.StartStripe(t, cfg.Now, false)
			configureBilling(cfg, stripe)
		})
		const address = "admin-export@example.test"
		owner := newBillingDevice(t, f.authFixture, address)
		other := newBillingDevice(t, f.authFixture, "admin-other@example.test")
		remoteCloudRequest(t, f.authFixture, owner, http.MethodPatch, cloudAccountPath,
			remoteproto.AccountEmailPatch{MarketingOptIn: new(true), Source: "settings"}).
			expect(t, http.StatusOK, "marketing_opt_in")
		f.requestAdmin(t, http.MethodGet, "/admin/search?q="+url.QueryEscape(owner.account), nil).
			expect(t, http.StatusOK, address)
		f.requestAdmin(t, http.MethodGet, "/admin/accounts/"+owner.account, nil).
			expect(t, http.StatusOK, "Device sessions")
		subscription := proStripeSub(f.clock.Now(), "sub_admin_export", owner.account)
		stripe.Put(subscription)
		deliverStripeEvent(t, f.authFixture, "evt_admin_export", stripeSubCreated, subscription.ID)
		f.requestAdmin(t, http.MethodGet, "/admin/search?q="+url.QueryEscape("cus_"+owner.account), nil).
			expect(t, http.StatusOK, address)
		f.requestAdmin(t, http.MethodGet, "/admin", nil).expect(t, http.StatusOK, "New accounts · google")

		assertAdminExports(t, f, owner, other)
		form := url.Values{
			"account_id": {owner.account},
			"target":     {address},
			"provider":   {"google"},
			"confirm":    {"yes"},
		}
		f.requestAdmin(t, http.MethodPost, "/admin/actions/detach-identity", form).
			expect(t, http.StatusBadRequest, "Invalid request")
		assertAuthRow(t, f.authFixture, "SELECT count(*) FROM identities", 2)
		form.Set("written_request", "Please detach my Google identity.")
		action := f.action(t, form, "detach-identity", "identity_detached")
		f.requestAdmin(t, http.MethodGet, action.Header.Get("Location"), nil).
			expect(t, http.StatusOK, "Please detach my Google identity.")
		assertAuthRow(t, f.authFixture, "SELECT count(*) FROM identities", 1)
		assertAdminRevocations(t, f, owner, other)
		assertAdminGatePolicy(t, f)

		start := url.Values{"account_id": {owner.account}}
		stripe.FailDeletion(true)
		f.action(t, start, "start-deletion", "account_deletion_started")
		awaitDeletionSweep(t, f.authFixture, func() bool {
			cancels, _ := stripe.DeletionCalls()
			return len(cancels) > 0
		})
		assertAuthRow(t, f.authFixture, "SELECT count(*) FROM accounts WHERE state = 'deleting'", 1)
		stripe.FailDeletion(false)
		f.clock.Advance(2 * time.Minute)
		awaitDeletionSweep(t, f.authFixture, func() bool {
			deleted, err := f.replica.App.Store.AuthAccountDeleted(t.Context(), owner.account)
			return err == nil && deleted
		})
		f.requestAdmin(t, http.MethodGet, "/admin/accounts/"+owner.account, nil).
			expect(t, http.StatusNotFound, "not found")
		f.requestAdmin(t, http.MethodGet, "/admin/accounts/"+other.account, nil).
			expect(t, http.StatusOK, "Device sessions")
	})
}

func assertAdminProLease(t *testing.T, f adminFixture, link *websocket.Conn) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		msg := remotetest.ReadControlMessage(t, link, time.Until(deadline))
		if msg.Type != remoteproto.ControlLease {
			continue
		}
		var token string
		if err := json.Unmarshal(msg.Data, &token); err != nil {
			t.Fatal(err)
		}
		lease, err := adminVerifier(t, f).VerifyLease(t.Context(), token, f.clock.Now())
		if err != nil || lease.Plan != "pro" || !slices.Contains(lease.AllowedRoutes, remoteproto.RouteCloud) {
			t.Fatalf("complimentary grant lease = %+v: %v", lease, err)
		}
		return
	}
}

func adminVerifier(t *testing.T, f adminFixture) *remoteproto.JWSVerifier {
	t.Helper()
	verify, err := remoteproto.NewJWSVerifier(f.replica.App.Issuer, f.replica.App.Keys.JWKS(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return verify
}

func assertAdminExports(t *testing.T, f adminFixture, owner, other remoteTestDevice) {
	t.Helper()
	reply := f.requestAdmin(t, http.MethodPost, "/admin/accounts/"+owner.account+"/export.json", nil)
	reply.expect(t, http.StatusOK, "account_exported")
	exports := authJSON[map[string]jsontext.Value](t, reply)
	for _, section := range []string{
		"account", "identities", "installations", "devices", "device_keys", "device_sessions", "web_sessions",
		"computers", "tunnels", "subscriptions", "account_tokens", "consent", "security_events",
	} {
		if value, ok := exports[section]; !ok || len(value) == 0 {
			t.Fatalf("account export omitted %s", section)
		}
	}
	for _, secret := range []string{
		other.account, "admin-other@example.test", owner.access, "_sha256", "pkce_verifier_enc", "public_jwk",
		"computer_jwk", "code_challenge", "decision_jws",
	} {
		if bytes.Contains(reply.body, []byte(secret)) {
			t.Fatalf("the account export contains a secret or another account's data")
		}
	}
	if id := reply.Header.Get("X-Compozy-Security-Event"); id == "" {
		t.Fatal("account export did not return an audit receipt")
	}
	csvReply := f.requestAdmin(t, http.MethodPost, "/admin/contacts.csv", nil)
	csvReply.expect(t, http.StatusOK, "consent_at")
	rows, err := csv.NewReader(bytes.NewReader(csvReply.body)).ReadAll()
	if err != nil || len(rows) != 2 || rows[1][0] != "admin-export@example.test" || rows[1][2] == "" ||
		rows[1][3] != "settings" {
		t.Fatalf("contacts CSV did not export just the opted-in account: %d rows: %v", len(rows), err)
	}
	f.requestAdmin(t, http.MethodGet, "/admin/events/"+csvReply.Header.Get("X-Compozy-Security-Event"), nil).
		expect(t, http.StatusOK, "contacts_exported")
}

func assertAdminRevocations(t *testing.T, f adminFixture, owner, other remoteTestDevice) {
	t.Helper()
	var sessionID string
	err := f.conn.QueryRow(t.Context(), "SELECT id FROM device_sessions WHERE account_id = $1", owner.account).
		Scan(&sessionID)
	if err != nil {
		t.Fatal(err)
	}
	f.action(
		t,
		url.Values{"account_id": {owner.account}, "target": {sessionID}},
		"revoke-session",
		"device_session_revoked",
	)
	remoteCloudRequest(t, f.authFixture, owner, http.MethodGet, cloudAccountPath, nil).
		expect(t, http.StatusUnauthorized, "reauth_required")
	f.action(
		t,
		url.Values{"account_id": {owner.account}, "target": {string(owner.id)}},
		"revoke-device",
		"device_revoked",
	)
	remoteCloudRequest(t, f.authFixture, other, http.MethodGet, cloudAccountPath, nil).expect(t, http.StatusOK, "plan")
}

func assertAdminGatePolicy(t *testing.T, f adminFixture) {
	t.Helper()
	until := f.clock.Now().Add(time.Hour)
	f.action(t, url.Values{"mode": {"grace"}, "until": {until.Format(time.RFC3339)}, "message": {"Sign-in outage"}},
		"gate-policy", "gate_policy_changed")
	signed, _ := getCloud(t, f.replica.Server.Client(), f.replica.Server.URL+"/v1/gate-policy")
	policy, err := adminVerifier(t, f).VerifyGatePolicy(t.Context(), signed, f.clock.Now())
	if err != nil || !policy.GraceAt(f.clock.Now()) || !policy.Until.Equal(until) ||
		policy.Message != "Sign-in outage" {
		t.Fatalf("updated gate policy = %+v: %v", policy, err)
	}
}
