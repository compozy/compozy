package gateway

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
)

const testPublicDNSResolver = "1.1.1.1:853"

func TestEndpointVerifierChallengeProtocol(t *testing.T) {
	t.Parallel()

	t.Run("Should apply a live timeout to future endpoint probes", func(t *testing.T) {
		t.Parallel()
		verifier, err := NewEndpointVerifier(time.Second, testPublicDNSResolver)
		if err != nil {
			t.Fatalf("NewEndpointVerifier() error = %v", err)
		}
		if err := verifier.Reconfigure(250*time.Millisecond, "8.8.8.8:853"); err != nil {
			t.Fatalf("Reconfigure() error = %v", err)
		}
		if got := verifier.Timeout(); got != 250*time.Millisecond {
			t.Fatalf("Timeout() = %s, want 250ms", got)
		}
		_, _, _, dialer, _ := verifier.configuration()
		networkDialer, ok := dialer.(*net.Dialer)
		if !ok || networkDialer.Timeout != 250*time.Millisecond {
			t.Fatalf("configured dialer = %#v, want 250ms timeout", dialer)
		}
	})

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

	t.Run("Should use the configured public DNS resolver for public proof", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if _, err := w.Write([]byte("nonce")); err != nil {
				return
			}
		}))
		t.Cleanup(server.Close)
		_, port, err := net.SplitHostPort(server.Listener.Addr().String())
		if err != nil {
			t.Fatalf("SplitHostPort() error = %v", err)
		}
		verifier, err := NewEndpointVerifier(time.Second, testPublicDNSResolver)
		if err != nil {
			t.Fatalf("NewEndpointVerifier() error = %v", err)
		}
		roots := x509.NewCertPool()
		roots.AddCert(server.Certificate())
		privateResolver := &recordingEndpointResolver{addresses: []netip.Addr{
			netip.MustParseAddr("100.64.0.1"),
		}}
		publicResolver := &recordingEndpointResolver{addresses: []netip.Addr{
			netip.MustParseAddr("93.184.216.34"),
		}}
		verifier.resolver = privateResolver
		verifier.publicResolver = publicResolver
		verifier.dialer = &fixedEndpointDialer{address: server.Listener.Addr().String()}
		verifier.rootCAs = roots

		err = verifier.Verify(
			testutil.Context(t),
			TierPublic,
			testEndpoint(fmt.Sprintf("https://example.com:%s", port)),
			testChallengePath(),
			"nonce",
		)
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if got := publicResolver.callCount(); got != 1 {
			t.Fatalf("public resolver calls = %d, want 1", got)
		}
		if got := privateResolver.callCount(); got != 0 {
			t.Fatalf("system resolver calls = %d, want 0", got)
		}
	})

	t.Run("Should authenticate the public DNS-over-TLS resolver", func(t *testing.T) {
		t.Parallel()
		serverAddress, roots, served := startDNSOverTLSTestServer(t)
		resolver, err := newPublicDNSResolverWithTransport(
			testPublicDNSResolver,
			time.Second,
			roots,
			redirectDNSDialer(serverAddress),
		)
		if err != nil {
			t.Fatalf("newPublicDNSResolverWithTransport() error = %v", err)
		}
		addresses, err := resolver.LookupNetIP(testutil.Context(t), "ip4", "gateway.example.test")
		if err != nil {
			t.Fatalf("LookupNetIP() error = %v", err)
		}
		want := netip.MustParseAddr("93.184.216.34")
		if len(addresses) != 1 || addresses[0] != want {
			t.Fatalf("LookupNetIP() = %v, want [%s]", addresses, want)
		}
		if err := <-served; err != nil {
			t.Fatalf("DNS-over-TLS server error = %v", err)
		}
	})

	t.Run("Should reject an unauthenticated public DNS-over-TLS resolver", func(t *testing.T) {
		t.Parallel()
		serverAddress, _, served := startDNSOverTLSTestServer(t)
		resolver, err := newPublicDNSResolverWithTransport(
			testPublicDNSResolver,
			time.Second,
			x509.NewCertPool(),
			redirectDNSDialer(serverAddress),
		)
		if err != nil {
			t.Fatalf("newPublicDNSResolverWithTransport() error = %v", err)
		}
		if _, err := resolver.LookupNetIP(
			testutil.Context(t), "ip4", "gateway.example.test",
		); err == nil {
			t.Fatal("LookupNetIP() error = nil, want TLS authentication failure")
		}
		if err := <-served; err == nil {
			t.Fatal("DNS-over-TLS server error = nil, want rejected handshake")
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
		verifier, err := NewEndpointVerifier(time.Second, testPublicDNSResolver)
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
		verifier, err := NewEndpointVerifier(time.Second, testPublicDNSResolver)
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

	t.Run("Should allow a declared tailnet scheme only on the private tier", func(t *testing.T) {
		t.Parallel()
		verifier, endpoint := newChallengeVerifier(t, func(w http.ResponseWriter, _ *http.Request) {
			if _, err := w.Write([]byte("nonce")); err != nil {
				return
			}
		})
		endpoint.URL = strings.Replace(endpoint.URL, "https://", "tsnet://", 1)
		endpoint.Scheme = "tsnet"
		endpoint.SchemePolicy = EndpointSchemePolicyTailnetInternal

		if err := verifier.Verify(
			testutil.Context(t), TierPrivate, endpoint, testChallengePath(), "nonce",
		); err != nil {
			t.Fatalf("Verify(private tailnet endpoint) error = %v", err)
		}
		if err := verifier.Verify(
			testutil.Context(t), TierPublic, endpoint, testChallengePath(), "nonce",
		); !errors.Is(err, ErrEndpointUnverified) {
			t.Fatalf("Verify(public tailnet endpoint) error = %v, want ErrEndpointUnverified", err)
		}

		undeclared := endpoint
		undeclared.SchemePolicy = ""
		if err := verifier.Verify(
			testutil.Context(t), TierPrivate, undeclared, testChallengePath(), "nonce",
		); !errors.Is(err, ErrEndpointUnverified) {
			t.Fatalf("Verify(undeclared private scheme) error = %v, want ErrEndpointUnverified", err)
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
		verifier, err := NewEndpointVerifier(time.Second, testPublicDNSResolver)
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
		verifier, err := NewEndpointVerifier(30*time.Millisecond, testPublicDNSResolver)
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
		verifier, err := NewEndpointVerifier(time.Second, testPublicDNSResolver)
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
	verifier, err := NewEndpointVerifier(time.Second, testPublicDNSResolver)
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

type recordingEndpointResolver struct {
	mu        sync.Mutex
	addresses []netip.Addr
	calls     int
}

func (r *recordingEndpointResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return append([]netip.Addr(nil), r.addresses...), nil
}

func (r *recordingEndpointResolver) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

type fixedEndpointDialer struct {
	address string
	dialer  net.Dialer
}

func startDNSOverTLSTestServer(t *testing.T) (string, *x509.CertPool, <-chan error) {
	t.Helper()
	certificate, root := newDNSOverTLSTestCertificate(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
	})
	if err != nil {
		t.Fatalf("tls.Listen() error = %v", err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("Close(DNS-over-TLS listener) error = %v", err)
		}
	})
	served := make(chan error, 1)
	go func() {
		served <- serveDNSOverTLSQuery(listener)
		close(served)
	}()
	roots := x509.NewCertPool()
	roots.AddCert(root)
	return listener.Addr().String(), roots, served
}

func newDNSOverTLSTestCertificate(t *testing.T) (tls.Certificate, *x509.Certificate) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() error = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "public-dns-test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("1.1.1.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate() error = %v", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("x509.ParseCertificate() error = %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: privateKey}, parsed
}

func redirectDNSDialer(address string) publicDNSDialFunc {
	return func(ctx context.Context, network string, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
}

func serveDNSOverTLSQuery(listener net.Listener) (resultErr error) {
	connection, err := listener.Accept()
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, connection.Close())
	}()
	var sizeBytes [2]byte
	if _, err := io.ReadFull(connection, sizeBytes[:]); err != nil {
		return err
	}
	query := make([]byte, binary.BigEndian.Uint16(sizeBytes[:]))
	if _, err := io.ReadFull(connection, query); err != nil {
		return err
	}
	response, err := dnsAResponse(query, [4]byte{93, 184, 216, 34})
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint16(sizeBytes[:], uint16(len(response)))
	if _, err := connection.Write(append(sizeBytes[:], response...)); err != nil {
		return err
	}
	return nil
}

func dnsAResponse(query []byte, address [4]byte) ([]byte, error) {
	if len(query) < 17 {
		return nil, errors.New("DNS query is too short")
	}
	questionEnd := 12
	for questionEnd < len(query) && query[questionEnd] != 0 {
		questionEnd += int(query[questionEnd]) + 1
	}
	questionEnd += 5
	if questionEnd > len(query) {
		return nil, errors.New("DNS question is invalid")
	}
	response := append([]byte(nil), query[:2]...)
	response = append(response,
		0x81, 0x80,
		0x00, 0x01,
		0x00, 0x01,
		0x00, 0x00,
		0x00, 0x00,
	)
	response = append(response, query[12:questionEnd]...)
	response = append(response,
		0xc0, 0x0c,
		0x00, 0x01,
		0x00, 0x01,
		0x00, 0x00, 0x00, 0x3c,
		0x00, 0x04,
	)
	return append(response, address[:]...), nil
}

func (d *fixedEndpointDialer) DialContext(
	ctx context.Context,
	network string,
	_ string,
) (net.Conn, error) {
	return d.dialer.DialContext(ctx, network, d.address)
}
