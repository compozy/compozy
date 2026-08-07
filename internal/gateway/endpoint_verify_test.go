package gateway

import (
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
)

func TestEndpointVerifierChallengeProtocol(t *testing.T) {
	t.Parallel()

	t.Run("Should verify the assigned tier challenge nonce [UT-063]", func(t *testing.T) {
		t.Parallel()
		registry := NewChallengeRegistry()
		path, nonce, cleanup, err := registry.Begin(TierPrivate)
		if err != nil {
			t.Fatalf("Begin() error = %v", err)
		}
		t.Cleanup(cleanup)
		verifier, endpoint := newChallengeVerifier(t, challengeHandler(registry, TierPrivate))
		if err := verifier.Verify(testutil.Context(t), TierPrivate, endpoint, path, nonce); err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
	})

	t.Run("Should reject another tier nonce or a missing challenge [UT-064]", func(t *testing.T) {
		t.Parallel()
		for _, serveTier := range []Tier{TierPublic, TierPrivate} {
			name := "missing nonce"
			if serveTier == TierPublic {
				name = "other tier nonce"
			}
			t.Run("Should reject "+name, func(t *testing.T) {
				t.Parallel()
				registry := NewChallengeRegistry()
				path, nonce, cleanup, err := registry.Begin(TierPrivate)
				if err != nil {
					t.Fatalf("Begin() error = %v", err)
				}
				t.Cleanup(cleanup)
				verifier, endpoint := newChallengeVerifier(t, challengeHandler(registry, serveTier))
				if serveTier == TierPrivate {
					cleanup()
				}
				err = verifier.Verify(testutil.Context(t), TierPrivate, endpoint, path, nonce)
				if !errors.Is(err, ErrEndpointUnverified) {
					t.Fatalf("Verify() error = %v, want ErrEndpointUnverified", err)
				}
			})
		}
	})
}

func TestEndpointVerifierSafetyPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should reject sensitive material hidden in an endpoint path", func(t *testing.T) {
		t.Parallel()
		verifier, err := NewEndpointVerifier(time.Second)
		if err != nil {
			t.Fatalf("NewEndpointVerifier() error = %v", err)
		}
		err = verifier.Verify(testutil.Context(t), TierPublic, AdvertisedEndpoint{
			URL:    "https://gateway.example.test/api_key=sk-gateway-endpoint-secret",
			Scheme: "https", Stability: EndpointStable,
		}, testChallengePath(), "nonce")
		if !errors.Is(err, ErrEndpointUnverified) {
			t.Fatalf("Verify() error = %v, want ErrEndpointUnverified", err)
		}
		if strings.Contains(err.Error(), "sk-gateway-endpoint-secret") {
			t.Fatalf("Verify() error leaked endpoint credential: %v", err)
		}
	})

	t.Run("Should reject a public HTTP endpoint [UT-065]", func(t *testing.T) {
		t.Parallel()
		verifier, err := NewEndpointVerifier(time.Second)
		if err != nil {
			t.Fatalf("NewEndpointVerifier() error = %v", err)
		}
		err = verifier.Verify(testutil.Context(t), TierPublic, AdvertisedEndpoint{
			URL: "http://gateway.example.test", Scheme: "http", Stability: EndpointStable,
		}, testChallengePath(), "nonce")
		if !errors.Is(err, ErrEndpointUnverified) {
			t.Fatalf("Verify() error = %v, want ErrEndpointUnverified", err)
		}
	})

	t.Run("Should reject an invalid TLS chain [UT-066]", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if _, err := w.Write([]byte("nonce")); err != nil {
				return
			}
		}))
		t.Cleanup(server.Close)
		verifier, err := NewEndpointVerifier(time.Second)
		if err != nil {
			t.Fatalf("NewEndpointVerifier() error = %v", err)
		}
		err = verifier.Verify(testutil.Context(t), TierPrivate, testEndpoint(server.URL), testChallengePath(), "nonce")
		if !errors.Is(err, ErrEndpointUnverified) {
			t.Fatalf("Verify() error = %v, want ErrEndpointUnverified", err)
		}
	})

	t.Run("Should reject redirects without following them [UT-067]", func(t *testing.T) {
		t.Parallel()
		var destinationCalled atomic.Bool
		verifier, endpoint := newChallengeVerifier(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/destination" {
				destinationCalled.Store(true)
				if _, err := w.Write([]byte("nonce")); err != nil {
					return
				}
				return
			}
			http.Redirect(w, r, "/destination", http.StatusFound)
		})
		err := verifier.Verify(testutil.Context(t), TierPrivate, endpoint, testChallengePath(), "nonce")
		if !errors.Is(err, ErrEndpointUnverified) {
			t.Fatalf("Verify() error = %v, want ErrEndpointUnverified", err)
		}
		if destinationCalled.Load() {
			t.Fatal("redirect destination was called")
		}
	})

	t.Run("Should reject a response larger than 64 KiB [UT-068]", func(t *testing.T) {
		t.Parallel()
		verifier, endpoint := newChallengeVerifier(t, func(w http.ResponseWriter, _ *http.Request) {
			if _, err := w.Write([]byte(strings.Repeat("x", maxChallengeResponseBytes+1))); err != nil {
				return
			}
		})
		err := verifier.Verify(testutil.Context(t), TierPrivate, endpoint, testChallengePath(), "nonce")
		if !errors.Is(err, ErrEndpointUnverified) {
			t.Fatalf("Verify() error = %v, want ErrEndpointUnverified", err)
		}
	})

	t.Run("Should honor its dedicated client timeout [UT-069]", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		t.Cleanup(server.Close)
		verifier, err := NewEndpointVerifier(30 * time.Millisecond)
		if err != nil {
			t.Fatalf("NewEndpointVerifier() error = %v", err)
		}
		roots := x509.NewCertPool()
		roots.AddCert(server.Certificate())
		verifier.rootCAs = roots
		started := time.Now()
		err = verifier.Verify(testutil.Context(t), TierPrivate, testEndpoint(server.URL), testChallengePath(), "nonce")
		if !errors.Is(err, ErrEndpointUnverified) {
			t.Fatalf("Verify() error = %v, want ErrEndpointUnverified", err)
		}
		if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
			t.Fatalf("Verify() elapsed = %s, want dedicated timeout", elapsed)
		}
	})

	t.Run("Should reject a public endpoint resolving inward [UT-070]", func(t *testing.T) {
		t.Parallel()
		verifier, err := NewEndpointVerifier(time.Second)
		if err != nil {
			t.Fatalf("NewEndpointVerifier() error = %v", err)
		}
		for _, host := range []string{
			"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.0.1", "169.254.1.1", "100.64.0.1",
			"[::1]", "[fc00::1]", "[fe80::1]",
		} {
			err = verifier.Verify(testutil.Context(t), TierPublic, AdvertisedEndpoint{
				URL: "https://" + host + ":443", Scheme: "https", Stability: EndpointStable,
			}, testChallengePath(), "nonce")
			if !errors.Is(err, ErrEndpointUnverified) {
				t.Fatalf("Verify(%s) error = %v, want ErrEndpointUnverified", host, err)
			}
		}
	})
}

func challengeHandler(registry ChallengeResolver, tier Tier) http.HandlerFunc {
	return func(w http.ResponseWriter, request *http.Request) {
		nonce, ok := registry.Resolve(tier, request.URL.Path)
		if !ok {
			http.NotFound(w, request)
			return
		}
		if _, err := w.Write([]byte(nonce)); err != nil {
			return
		}
	}
}

func newChallengeVerifier(t *testing.T, handler http.HandlerFunc) (*EndpointVerifier, AdvertisedEndpoint) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	verifier, err := NewEndpointVerifier(time.Second)
	if err != nil {
		t.Fatalf("NewEndpointVerifier() error = %v", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	verifier.rootCAs = roots
	return verifier, testEndpoint(server.URL)
}

func testEndpoint(rawURL string) AdvertisedEndpoint {
	return AdvertisedEndpoint{URL: rawURL, Scheme: "https", Stability: EndpointStable}
}

func testChallengePath() string {
	return ChallengePathPrefix + "0123456789abcdef"
}
