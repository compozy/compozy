package computers

import (
	"bytes"
	"cmp"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json/v2"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"

	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
)

const (
	tunnelHTTPOrigin  = "http://" + remoteproto.TunnelHost
	tunnelProofOrigin = "https://" + remoteproto.TunnelHost
	healthInterval    = 2 * time.Second
	headerTimeout     = 6 * time.Second
	tunnelTimeout     = 2500 * time.Millisecond
	tunnelIOTimeout   = 3 * time.Second
	maxTunnelResponse = 64 << 10
)

type tunnel struct {
	mux       *yamux.Session
	socket    *websocket.Conn
	transport *http.Transport
	once      sync.Once
}

type connection struct {
	*tunnel
	route      store.ComputerRoute
	descriptor remoteproto.Descriptor
	key        crypto.Signer
	credential string
	expiresAt  time.Time
	scopes     []remoteproto.Scope
}

func noRedirects(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }

func routeClient(kind remoteproto.RouteKind) *http.Client {
	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: probeTimeout}).DialContext,
		TLSHandshakeTimeout:   probeTimeout,
		ResponseHeaderTimeout: probeTimeout,
		DisableKeepAlives:     true,
	}
	if kind == remoteproto.RouteCloud {
		transport.Proxy = http.ProxyFromEnvironment
	}
	return &http.Client{Transport: transport, CheckRedirect: noRedirects}
}

func fetchDescriptor(
	ctx context.Context, origin *url.URL, client *http.Client,
) (remoteproto.Descriptor, error) {
	target := origin.JoinPath(remoteproto.DescriptorPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return remoteproto.Descriptor{}, ErrOffline
	}
	// #nosec G704 -- unauthenticated preflight to the account's advertised address.
	resp, err := client.Do(req)
	if err != nil {
		return remoteproto.Descriptor{}, ErrOffline
	}
	defer resp.Body.Close()
	var descriptor remoteproto.Descriptor
	if resp.StatusCode != http.StatusOK ||
		json.UnmarshalRead(io.LimitReader(resp.Body, maxTunnelResponse), &descriptor) != nil {
		return remoteproto.Descriptor{}, ErrOffline
	}
	return descriptor, nil
}

func dialTunnel(
	ctx context.Context, origin *url.URL, client *http.Client, pin string, now time.Time,
) (*tunnel, error) {
	dial, cancel := context.WithTimeout(ctx, tunnelTimeout)
	defer cancel()
	target := origin.JoinPath(remoteproto.TunnelPath)
	socket, response, err := websocket.Dial(dial, target.String(), &websocket.DialOptions{
		HTTPClient: client, CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		return nil, ErrOffline
	}
	transport := websocket.NetConn(ctx, socket, websocket.MessageBinary)
	// #nosec G402 -- the replacement verifier requires the pinned private root and server name.
	inner := tls.Client(transport, &tls.Config{
		MinVersion: tls.VersionTLS13, InsecureSkipVerify: true,
		VerifyConnection: func(state tls.ConnectionState) error { return verifyTunnelRoot(state, pin, now) },
	})
	if err := inner.HandshakeContext(dial); err != nil {
		_ = socket.CloseNow()
		return nil, err
	}
	return multiplexTunnel(inner, socket)
}

func verifyTunnelRoot(state tls.ConnectionState, pin string, now time.Time) error {
	if len(state.PeerCertificates) != 2 {
		return ErrPinMismatch
	}
	root := state.PeerCertificates[1]
	hash := sha256.Sum256(root.Raw)
	if !root.IsCA || hex.EncodeToString(hash[:]) != pin {
		return ErrPinMismatch
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	_, err := state.PeerCertificates[0].Verify(x509.VerifyOptions{
		Roots: roots, DNSName: remoteproto.TunnelHost, CurrentTime: now,
	})
	if err != nil {
		return ErrPinMismatch
	}
	return nil
}

func multiplexTunnel(inner net.Conn, socket *websocket.Conn) (*tunnel, error) {
	options := yamux.DefaultConfig()
	options.LogOutput = io.Discard
	options.KeepAliveInterval = tunnelIOTimeout
	options.ConnectionWriteTimeout = tunnelIOTimeout
	options.StreamOpenTimeout = tunnelIOTimeout
	options.StreamCloseTimeout = tunnelIOTimeout
	mux, err := yamux.Client(inner, options)
	if err != nil {
		_ = socket.CloseNow()
		return nil, err
	}
	return &tunnel{mux: mux, socket: socket, transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return mux.Open()
		},
		DisableKeepAlives: true, DisableCompression: true, ResponseHeaderTimeout: headerTimeout,
		MaxConnsPerHost: 128, MaxResponseHeaderBytes: maxTunnelResponse,
	}}, nil
}

func (t *tunnel) close() {
	t.once.Do(func() {
		_ = t.socket.CloseNow()
		_ = t.mux.Close()
		t.transport.CloseIdleConnections()
	})
}

func (t *tunnel) mint(
	ctx context.Context, grant string, key crypto.Signer, now time.Time,
) (remoteproto.MintResponse, error) {
	proof, err := remoteproto.BuildDPoP(key, http.MethodPost, tunnelProofOrigin+remoteproto.SessionPath, "", now)
	if err != nil {
		return remoteproto.MintResponse{}, err
	}
	body, err := json.Marshal(remoteproto.MintRequest{Grant: grant, DPoP: proof})
	if err != nil {
		return remoteproto.MintResponse{}, err
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, tunnelHTTPOrigin+remoteproto.SessionPath, bytes.NewReader(body),
	)
	if err != nil {
		return remoteproto.MintResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := t.transport.RoundTrip(request)
	if err != nil {
		return remoteproto.MintResponse{}, ErrOffline
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return remoteproto.MintResponse{}, remoteFailure(response)
	}
	var session remoteproto.MintResponse
	if err := json.UnmarshalRead(io.LimitReader(response.Body, maxTunnelResponse), &session); err != nil ||
		session.Credential == "" || session.ExpiresAt <= now.Unix() {
		return remoteproto.MintResponse{}, ErrOffline
	}
	return session, nil
}

func remoteFailure(response *http.Response) error {
	var failure struct {
		Code    string            `json:"code"`
		Message string            `json:"error"`
		Details map[string]string `json:"details"`
	}
	if err := json.UnmarshalRead(io.LimitReader(response.Body, maxTunnelResponse), &failure); err != nil {
		return ErrOffline
	}
	return &Failure{
		Status: response.StatusCode, Code: cmp.Or(failure.Code, ErrOffline.Code),
		Message: cmp.Or(failure.Message, ErrOffline.Message), Details: failure.Details,
	}
}

func (c *connection) authenticate(req *http.Request, now time.Time) error {
	target := tunnelProofOrigin + req.URL.EscapedPath()
	proof, err := remoteproto.BuildDPoP(c.key, req.Method, target, c.credential, now)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "CompozyRemote "+c.credential)
	req.Header.Set("DPoP", proof)
	return nil
}

func (c *connection) check(ctx context.Context, now time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, healthInterval)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tunnelHTTPOrigin+remoteproto.SessionPath, nil)
	if err != nil {
		return err
	}
	if err := c.authenticate(req, now); err != nil {
		return err
	}
	response, err := c.transport.RoundTrip(req)
	if err != nil {
		return ErrOffline
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return remoteFailure(response)
	}
	return nil
}
