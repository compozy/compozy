//go:build integration

package remotetest

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	gorilla "github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"

	"github.com/compozy/compozy/internal/remoteproto"
)

const tunnelHTTPOrigin = "http://compozy-remote"
const tunnelProofOrigin = "https://compozy-remote"

type Tunnel struct {
	mux *yamux.Session
}

func DialTunnel(t *testing.T, address string, descriptor remoteproto.Descriptor, now func() time.Time) *Tunnel {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	// #nosec G107 -- address comes from the test's local daemon listener.
	ws, _, err := websocket.Dial(ctx, "ws://"+address+"/remote/v1/tunnel", nil)
	if err != nil {
		t.Fatalf("dial tunnel websocket: %v", err)
	}
	t.Cleanup(func() { _ = ws.CloseNow() })
	transport := websocket.NetConn(t.Context(), ws, websocket.MessageBinary)
	// #nosec G402 -- verification uses the expected private trust root and server identity.
	inner := tls.Client(transport, &tls.Config{
		MinVersion: tls.VersionTLS13, InsecureSkipVerify: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			return verifyTunnelRoot(state, descriptor.TLSRootSHA256, now())
		},
	})
	if err := inner.HandshakeContext(ctx); err != nil {
		t.Fatalf("handshake inside the tunnel: %v", err)
	}
	options := yamux.DefaultConfig()
	options.LogOutput = io.Discard
	mux, err := yamux.Client(inner, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mux.Close(); err != nil {
			t.Errorf("close tunnel: %v", err)
		}
	})
	return &Tunnel{mux: mux}
}

func verifyTunnelRoot(state tls.ConnectionState, pin string, now time.Time) error {
	if len(state.PeerCertificates) != 2 {
		return errors.New("tunnel must carry the leaf and private root")
	}
	root := state.PeerCertificates[1]
	hash := sha256.Sum256(root.Raw)
	if hex.EncodeToString(hash[:]) != pin {
		return errors.New("tunnel root pin mismatch")
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	_, err := state.PeerCertificates[0].Verify(x509.VerifyOptions{
		Roots: roots, DNSName: "compozy-remote", CurrentTime: now,
	})
	return err
}

func TunnelDescriptor(t *testing.T, address string) remoteproto.Descriptor {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	target := "http://" + address + "/remote/v1/descriptor"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("read remote descriptor: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	var descriptor remoteproto.Descriptor
	if response.StatusCode != http.StatusOK || json.UnmarshalRead(response.Body, &descriptor) != nil ||
		descriptor.ComputerID == "" || descriptor.TLSRootSHA256 == "" {
		t.Fatalf("invalid remote descriptor: HTTP %d, %+v", response.StatusCode, descriptor)
	}
	return descriptor
}

func (tunnel *Tunnel) Mint(t *testing.T, key crypto.Signer, grant string, now time.Time) (int, []byte) {
	t.Helper()
	const path = "/remote/v1/session"
	proof, err := remoteproto.BuildDPoP(key, http.MethodPost, tunnelProofOrigin+path, "", now)
	if err != nil {
		t.Fatal(err)
	}
	request := tunnelRequest(t, http.MethodPost, path, remoteproto.MintRequest{Grant: grant, DPoP: proof})
	return tunnel.Request(t, request)
}

func TunnelAuthorizedRequest(
	t *testing.T, method, path string, input any, key crypto.Signer, credential string, now time.Time,
) *http.Request {
	t.Helper()
	request := tunnelRequest(t, method, path, input)
	proof, err := remoteproto.BuildDPoP(key, method, tunnelProofOrigin+path, credential, now)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "CompozyRemote "+credential)
	request.Header.Set("DPoP", proof)
	return request
}

func tunnelRequest(t *testing.T, method, path string, input any) *http.Request {
	t.Helper()
	var payload []byte
	if input != nil {
		var err error
		payload, err = json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequestWithContext(t.Context(), method, tunnelHTTPOrigin+path, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

func (tunnel *Tunnel) Request(t *testing.T, request *http.Request) (int, []byte) {
	t.Helper()
	status, body, err := tunnel.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	return status, body
}

func (tunnel *Tunnel) RoundTrip(request *http.Request) (int, []byte, error) {
	transport := &http.Transport{
		DialContext:       func(context.Context, string, string) (net.Conn, error) { return tunnel.mux.Open() },
		DisableKeepAlives: true,
	}
	defer transport.CloseIdleConnections()
	response, err := (&http.Client{Transport: transport, Timeout: 8 * time.Second}).Do(request)
	if err != nil {
		return 0, nil, err
	}
	body, err := io.ReadAll(response.Body)
	return response.StatusCode, body, errors.Join(err, response.Body.Close())
}

func (tunnel *Tunnel) OpenStream(t *testing.T, request *http.Request) *gorilla.Conn {
	t.Helper()
	dialer := gorilla.Dialer{HandshakeTimeout: 3 * time.Second,
		NetDialContext: func(context.Context, string, string) (net.Conn, error) { return tunnel.mux.Open() }}
	target := "ws://compozy-remote" + request.URL.RequestURI()
	socket, response, err := dialer.DialContext(request.Context(), target, request.Header)
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		t.Fatalf("open websocket inside tunnel: %v", err)
	}
	t.Cleanup(func() { _ = socket.Close() })
	return socket
}
