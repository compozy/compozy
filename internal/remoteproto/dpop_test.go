package remoteproto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"net/http"
	"testing"
	"time"
)

const testProofURL = "https://x/remote/v1/session"

func TestDPoP(t *testing.T) {
	t.Parallel()
	now := time.Unix(fixtureIssuedAt, 0)

	t.Run("Should verify the method and target of a mint proof (UT-008)", func(t *testing.T) {
		t.Parallel()
		private, _, binding := deviceProof(t, "")
		token, err := BuildDPoP(private, binding.Method, testProofURL+"?ignore=value", "", now)
		if err != nil {
			t.Fatal(err)
		}
		v := NewDPoPVerifier()
		for _, tt := range []struct {
			name   string
			method string
			url    string
		}{
			{name: "Should reject a mismatched method", method: http.MethodGet, url: testProofURL},
			{name: "Should reject a mismatched URL", method: http.MethodPost, url: testProofURL + "/wrong"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				bad := DPoPBinding{Method: tt.method, URL: tt.url, JKT: binding.JKT}
				if _, err := v.VerifyDPoP(token, bad, now); !errors.Is(err, ErrDPoPInvalid) {
					t.Fatalf("VerifyDPoP = %v, want dpop_invalid", err)
				}
			})
		}
		proof, err := v.VerifyDPoP(token, binding, now)
		if err != nil || proof.URL != testProofURL || proof.JTI == "" || proof.IssuedAt != now.Unix() {
			t.Fatalf("VerifyDPoP = %+v, %v", proof, err)
		}
	})

	t.Run("Should reject stale, replayed and misbound proofs (UT-012)", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []struct {
			name      string
			iat       time.Duration
			advance   time.Duration
			wrongHash bool
			want      error
		}{
			{name: "Should reject iat 61 seconds old", iat: -61 * time.Second, want: ErrDPoPInvalid},
			{name: "Should reject iat 61 seconds ahead", iat: 61 * time.Second, want: ErrDPoPInvalid},
			{name: "Should reject duplicate jti within 120 seconds", iat: time.Minute, advance: 119 * time.Second, want: ErrDPoPReplayed},
			{name: "Should reject an ath of another credential", wrongHash: true, want: ErrDPoPInvalid},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				private, jwk, binding := deviceProof(t, "session-credential")
				ath := hashBase64([]byte(binding.Credential))
				if tt.wrongHash {
					ath = hashBase64([]byte("other-credential"))
				}
				proof := DPoPProof{
					Method: binding.Method, URL: binding.URL, JTI: "one-proof-id",
					IssuedAt: now.Add(tt.iat).Unix(), TokenHash: &ath,
				}
				token, err := signDeviceJWS(private, jwk, dpopType, proof)
				if err != nil {
					t.Fatal(err)
				}
				v := NewDPoPVerifier()
				if tt.advance > 0 {
					if _, err := v.VerifyDPoP(token, binding, now); err != nil {
						t.Fatalf("initial proof = %v", err)
					}
				}
				if _, err := v.VerifyDPoP(token, binding, now.Add(tt.advance)); !errors.Is(err, tt.want) {
					t.Fatalf("VerifyDPoP = %v, want %v", err, tt.want)
				}
			})
		}
	})

	t.Run("Should bind a proof to the browser or tunnel credential mode", func(t *testing.T) {
		t.Parallel()
		private, _, binding := deviceProof(t, "session-credential")
		token, err := BuildDPoP(private, binding.Method, binding.URL, binding.Credential, now)
		if err != nil {
			t.Fatal(err)
		}
		v := NewDPoPVerifier()
		browser := DPoPBinding{Method: binding.Method, URL: binding.URL, JKT: binding.JKT}
		if _, err := v.VerifyDPoP(token, browser, now); !errors.Is(err, ErrDPoPInvalid) {
			t.Fatalf("browser accepted tunnel proof: %v", err)
		}
		if _, err := v.VerifyDPoP(token, binding, now); err != nil {
			t.Fatalf("tunnel refused correctly bound proof: %v", err)
		}
	})
}

func deviceProof(t *testing.T, credential string) (*ecdsa.PrivateKey, JWK, DPoPBinding) {
	t.Helper()
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwk, err := NewPublicJWK(private.Public())
	if err != nil {
		t.Fatal(err)
	}
	jkt, err := jwk.Thumbprint()
	if err != nil {
		t.Fatal(err)
	}
	return private, jwk, DPoPBinding{Method: http.MethodPost, URL: testProofURL, JKT: jkt, Credential: credential}
}
