package computers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"
	"github.com/jonboulle/clockwork"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/remoteproto"
)

func TestTunnelTransport(t *testing.T) {
	t.Parallel()
	t.Run("Should reject a different private root before an application request", func(t *testing.T) {
		t.Parallel()
		// UT-037. Safety Invariants 1 and 2.
		clock := clockwork.NewFakeClock()
		pin, pair := tunnelCertificates(t, clock.Now())
		otherPin, _ := tunnelCertificates(t, clock.Now())
		host := serveProbeTunnel(t, remoteproto.Descriptor{
			ComputerID: probeComputer, TLSRootSHA256: pin, Protocol: remoteproto.CurrentProtocol,
		}, pair, clock, false, func() bool { return true })
		link, err := dialTunnel(
			t.Context(), probeURL(t, host.URL), routeClient(remoteproto.RouteDirect), otherPin, clock.Now(),
		)
		if link != nil || !errors.Is(err, ErrPinMismatch) {
			t.Fatalf("wrong root handshake = %v, want %v", err, ErrPinMismatch)
		}
	})
	t.Run("Should remove local credentials and sign a fresh bound request", func(t *testing.T) {
		t.Parallel()
		// UT-038. Safety Invariants 11, 13 and 18.
		clock := clockwork.NewFakeClock()
		account := newRoutingAccount(t)
		c := &connection{key: account.key, credential: "fixture-credential", expiresAt: clock.Now().Add(time.Hour)}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/sessions", nil)
		headers := []string{
			"Cookie", "Authorization", "DPoP", claimHeader,
			agentidentity.HeaderSessionID, agentidentity.HeaderAgent,
			"X-Compozy-Client-Token", "X-Compozy-Client-Workspace",
			"X-Compozy-Gateway-Session", "X-Compozy-Gateway-Device",
		}
		for _, header := range headers {
			req.Header.Set(header, "local-value")
		}
		r := &Registry{clock: clock}
		withTrailer := req.Clone(t.Context())
		withTrailer.Header.Del(claimHeader)
		withTrailer.Trailer = http.Header{claimHeader: nil}
		for _, input := range []*http.Request{req, withTrailer} {
			response, err := r.RoundTrip(t.Context(), probeComputer, input)
			if response != nil {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Error(closeErr)
				}
			}
			if response != nil || !errors.Is(err, ErrClaimToken) {
				t.Fatalf("claim token request returned response %v and error %v", response != nil, err)
			}
		}
		req.Header.Del(claimHeader)
		out, err := c.bridgeRequest(t.Context(), req, r)
		if err != nil {
			t.Fatal(err)
		}
		if out.Host != remoteproto.TunnelHost || out.Header.Get("Authorization") != "CompozyRemote fixture-credential" {
			t.Fatalf("incorrect tunnel target or authorization header")
		}
		for _, header := range headers {
			if out.Header.Get(header) == "local-value" {
				t.Fatalf("local header survived the bridge: %s", header)
			}
		}
		jwk, err := remoteproto.NewPublicJWK(account.key.Public())
		if err != nil {
			t.Fatal(err)
		}
		jkt, err := jwk.Thumbprint()
		if err != nil {
			t.Fatal(err)
		}
		binding := remoteproto.DPoPBinding{
			Method: http.MethodGet, URL: tunnelProofOrigin + "/api/sessions", JKT: jkt, Credential: c.credential,
		}
		if _, err := remoteproto.NewDPoPVerifier().
			VerifyDPoP(out.Header.Get("DPoP"), binding, clock.Now()); err != nil {
			t.Fatalf("invalid bridge DPoP: %v", err)
		}
	})
}

func tunnelCertificates(t *testing.T, now time.Time) (string, tls.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := &x509.Certificate{
		SerialNumber: big.NewInt(1), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, root, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2), DNSNames: []string{remoteproto.TunnelHost},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		NotBefore: root.NotBefore, NotAfter: root.NotAfter,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(rootDER)
	return hex.EncodeToString(hash[:]), tls.Certificate{Certificate: [][]byte{leafDER, rootDER}, PrivateKey: key}
}

func serveProbeTunnel(
	t *testing.T, descriptor remoteproto.Descriptor, certificate tls.Certificate,
	clock clockwork.Clock, outerTLS bool, available func() bool,
) *httptest.Server {
	t.Helper()
	router := http.NewServeMux()
	router.HandleFunc(remoteproto.DescriptorPath, func(w http.ResponseWriter, _ *http.Request) {
		if !available() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if err := json.MarshalWrite(w, descriptor); err != nil {
			t.Errorf("write descriptor: %v", err)
		}
	})
	router.HandleFunc(remoteproto.TunnelPath, func(w http.ResponseWriter, req *http.Request) {
		socket, err := websocket.Accept(w, req, nil)
		if err != nil {
			return
		}
		defer socket.CloseNow()
		secure := tls.Server(websocket.NetConn(req.Context(), socket, websocket.MessageBinary), &tls.Config{
			MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		})
		options := yamux.DefaultConfig()
		options.LogOutput = io.Discard
		mux, err := yamux.Server(secure, options)
		if err != nil {
			return
		}
		defer mux.Close()
		server := &http.Server{
			ReadHeaderTimeout: time.Second,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodGet {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusCreated)
				if err := json.MarshalWrite(w, remoteproto.MintResponse{
					SessionID:  "rs_fixture",
					Credential: "fixture-credential",
					ExpiresAt:  clock.Now().Add(time.Hour).Unix(),
				}); err != nil {
					t.Errorf("write fake mint: %v", err)
				}
			}),
		}
		server.SetKeepAlivesEnabled(false)
		defer server.Close()
		_ = server.Serve(mux)
	})
	server := httptest.NewUnstartedServer(router)
	if outerTLS {
		server.StartTLS()
	} else {
		server.Start()
	}
	t.Cleanup(server.Close)
	return server
}
