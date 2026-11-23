//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/compozy/compozy/internal/cloud/auth"
)

func TestCloudIdentity(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should keep the provider subject authoritative across linking and email changes (UT-118)",
		func(t *testing.T) {
			t.Parallel()
			f := newAuthFixture(t)
			google := providerProfile{subject: "G1", email: " Pedro@Example.com ", verified: true}
			github := providerProfile{email: "pedro@example.com", verified: true}
			reply, grant := f.signIn(t, auth.Google, google, false)
			reply.expect(t, http.StatusOK, "Return to CompozyOS.")
			first := f.redeem(t, grant)
			if first.Email != github.email || !first.NewAccount {
				t.Fatal("first sign-in did not canonicalize the verified email")
			}
			reply, grant = f.signIn(t, auth.GitHub, github, false)
			reply.expect(t, http.StatusOK, "Return to CompozyOS.")
			second := f.redeem(t, grant)
			if second.AccountID != first.AccountID || second.NewAccount {
				t.Fatal("matching verified email did not link the account")
			}
			assertAuthRow(t, f, "SELECT count(*) FROM accounts", 1)
			assertAuthRow(t, f, "SELECT count(*) FROM identities", 2)

			google.email = "updated@example.com"
			reply, grant = f.signIn(t, auth.Google, google, false)
			reply.expect(t, http.StatusOK, "Return to CompozyOS.")
			updated := f.redeem(t, grant)
			if updated.AccountID != first.AccountID || updated.Email != google.email {
				t.Fatal("linked identity did not update its own email")
			}
			unrelated := providerProfile{subject: "G2", email: "other@example.com", verified: true}
			reply, _ = f.signIn(t, auth.Google, unrelated, false)
			reply.expect(t, http.StatusOK, "Return to CompozyOS.")
			github.email = unrelated.email
			reply, grant = f.signIn(t, auth.GitHub, github, false)
			reply.expect(t, http.StatusConflict, "sign_in_identity_conflict")
			f.poll(t, grant).expect(t, http.StatusConflict, "sign_in_identity_conflict")
			assertAuthRow(t, f, "SELECT count(*) FROM accounts", 2)
			assertAuthRow(t, f, "SELECT count(*) FROM identities", 3)
			var id, email string
			err := f.conn.QueryRow(
				t.Context(),
				"SELECT account_id, email FROM identities WHERE provider = 'github' AND subject = '42'",
			).Scan(&id, &email)
			if err != nil || id != first.AccountID || email != "pedro@example.com" {
				t.Fatal("identity conflict modified the existing GitHub identity")
			}
			assertAuthRow(t, f, "SELECT count(*) FROM accounts WHERE email = 'updated@example.com'", 1)
		},
	)

	t.Run("Should reject unverified email and mismatched Google claims (UT-118)", func(t *testing.T) {
		t.Parallel()
		f := newAuthFixture(t)
		cases := []struct {
			name, provider, code string
			profile              providerProfile
			status               int
		}{
			{"Google email", auth.Google, "sign_in_email_unverified",
				providerProfile{subject: "G1", email: "google@example.test"}, http.StatusForbidden},
			{"GitHub primary email", auth.GitHub, "sign_in_email_unverified",
				providerProfile{email: "github@example.test"}, http.StatusForbidden},
			{"Google audience", auth.Google, "sign_in_provider_failed",
				providerProfile{subject: "G1", email: "google@example.test", verified: true, audience: "wrong-client"},
				http.StatusBadGateway},
			{
				"Google issuer",
				auth.Google,
				"sign_in_provider_failed",
				providerProfile{
					subject:  "G1",
					email:    "google@example.test",
					verified: true,
					issuer:   "https://issuer.test",
				},
				http.StatusBadGateway,
			},
			{"Google nonce", auth.Google, "sign_in_provider_failed",
				providerProfile{subject: "G1", email: "google@example.test", verified: true, nonce: "wrong-nonce"},
				http.StatusBadGateway},
		}
		for _, tc := range cases {
			t.Run("Should refuse "+tc.name, func(t *testing.T) {
				reply, _ := f.signIn(t, tc.provider, tc.profile, false)
				reply.expect(t, tc.status, tc.code)
			})
		}
		assertAuthRow(t, f, "SELECT count(*) FROM accounts", 0)
		assertAuthRow(t, f, "SELECT count(*) FROM identities", 0)
	})
}
