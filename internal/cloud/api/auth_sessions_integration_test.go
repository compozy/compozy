//go:build integration

package api_test

import (
	"crypto/ecdsa"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cloud/auth"
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
