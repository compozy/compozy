package remoteproto

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json/v2"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

const fixtureIssuedAt = 1795392000

func TestVerifyGrant(t *testing.T) {
	t.Parallel()

	t.Run("Should verify the shared signed fixture and its device binding (UT-005)", func(t *testing.T) {
		t.Parallel()
		jwks := readFixture[JWKS](t, "grant_v1.jwks.json")
		device := readFixture[JWK](t, "grant_v1.device.jwk.json")
		verifier := grantVerifier(t, jwks, nil)
		raw, err := os.ReadFile("testdata/grant_v1.jws")
		if err != nil {
			t.Fatal(err)
		}
		claims := verifyGrant(t, verifier, strings.TrimSpace(string(raw)), time.Unix(fixtureIssuedAt+10, 0))
		jkt, err := device.Thumbprint()
		if err != nil {
			t.Fatal(err)
		}
		if claims.Cnf.JKT != jkt || claims.Audience != "cmp_abcdefgh" || claims.Subject != "dev_abcdef" ||
			claims.Route != RouteSSH || claims.SecurityEpoch != 7 {
			t.Fatalf("unexpected claims: %+v", claims)
		}
	})

	t.Run("Should honor 30 seconds of grant leeway (UT-006)", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []struct {
			name      string
			expiredBy time.Duration
			want      error
		}{
			{name: "Should accept a grant 29 seconds after expiry", expiredBy: 29 * time.Second},
			{name: "Should reject a grant 31 seconds after expiry", expiredBy: 31 * time.Second, want: ErrGrantExpired},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				now := time.Unix(fixtureIssuedAt, 0)
				signer := newTestSigner(t, "leeway")
				verifier := grantVerifier(t, JWKS{Keys: []JWK{signer.PublicJWK()}}, nil)
				claims := newTestGrant(now.Add(-tt.expiredBy).Add(-GrantTTL))
				token := signTestGrant(t, signer, claims)
				if _, err := verifier.VerifyGrant(t.Context(), token, now); !errors.Is(err, tt.want) {
					t.Fatalf("VerifyGrant = %v, want %v", err, tt.want)
				}
			})
		}
	})

	t.Run("Should refresh once for an unknown kid and verify rotated keys (UT-007)", func(t *testing.T) {
		t.Parallel()
		now := time.Unix(fixtureIssuedAt, 0)
		old := newTestSigner(t, "old")
		rotated := newTestSigner(t, "rotated")
		unknown := newTestSigner(t, "unknown")
		refreshes := 0
		verifier := grantVerifier(t, JWKS{Keys: []JWK{old.PublicJWK()}}, func(context.Context) (JWKS, error) {
			refreshes++
			return JWKS{Keys: []JWK{old.PublicJWK(), rotated.PublicJWK()}}, nil
		})
		claims := newTestGrant(now)
		token := signTestGrant(t, unknown, claims)
		if _, err := verifier.VerifyGrant(t.Context(), token, now); !errors.Is(err, ErrGrantSignatureInvalid) {
			t.Fatalf("unknown kid error = %v", err)
		}
		if refreshes != 1 {
			t.Fatalf("refreshes = %d, want 1", refreshes)
		}
		verifyGrant(t, verifier, signTestGrant(t, rotated, claims), now)
		if refreshes != 1 {
			t.Fatalf("cached key caused another refresh, total %d", refreshes)
		}
	})

	t.Run("Should reject absent cnf and legacy top-level jkt (UT-007)", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []struct {
			name   string
			claims map[string]any
		}{
			{name: "Should reject missing cnf", claims: map[string]any{}},
			{name: "Should reject a top-level jkt", claims: map[string]any{"jkt": hashBase64([]byte("key"))}},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				signer := newTestSigner(t, "binding")
				verifier := grantVerifier(t, JWKS{Keys: []JWK{signer.PublicJWK()}}, nil)
				token, err := signer.sign(grantType, tt.claims)
				if err != nil {
					t.Fatal(err)
				}
				_, err = verifier.VerifyGrant(t.Context(), token, time.Unix(fixtureIssuedAt, 0))
				if !errors.Is(err, ErrGrantBindingMissing) {
					t.Fatalf("VerifyGrant = %v", err)
				}
			})
		}
	})
}

func newTestSigner(t *testing.T, kid string) *Signer {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewSigner(kid, key)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func grantVerifier(t *testing.T, jwks JWKS, refresh JWKSFetcher) *JWSVerifier {
	t.Helper()
	verifier, err := NewJWSVerifier(CloudIssuer, jwks, refresh)
	if err != nil {
		t.Fatal(err)
	}
	return verifier
}

func newTestGrant(now time.Time) GrantClaims {
	return GrantClaims{
		Issuer: CloudIssuer, Audience: "cmp_abcdefgh", Subject: "dev_abcdef", AccountID: "acc_fixture",
		Route: RouteSSH, Cnf: Confirmation{JKT: hashBase64([]byte("key"))}, Scopes: []Scope{ScopeSessions},
		JTI: "grant-1", IssuedAt: now.Unix(), ExpiresAt: now.Add(GrantTTL).Unix(), SecurityEpoch: 1,
	}
}

func signTestGrant(t *testing.T, signer *Signer, claims GrantClaims) string {
	t.Helper()
	token, err := signer.SignGrant(claims)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func verifyGrant(t *testing.T, verifier Verifier, token string, now time.Time) GrantClaims {
	t.Helper()
	claims, err := verifier.VerifyGrant(t.Context(), token, now)
	if err != nil {
		t.Fatal(err)
	}
	return claims
}

func readFixture[T any](t *testing.T, name string) T {
	t.Helper()
	var result T
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
