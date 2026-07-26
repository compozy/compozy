package daytona

import (
	"context"

	"errors"
	"fmt"

	"log/slog"
	"net"
	"net/http"
	"net/url"

	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	launcherSidecarPort               = 40241
	launcherSidecarVersion            = "agh-daytona-launcher-sidecar-v1"
	launcherSidecarPath               = "/tmp/agh-daytona-launcher-sidecar-v1"
	launcherSidecarLogPath            = "/tmp/agh-daytona-launcher-sidecar-v1.log"
	sidecarHealthPath                 = "healthz"
	sidecarLaunchPath                 = "v1/launch"
	sidecarSessionStreamBasePath      = "v1/sessions"
	sidecarHealthTimeout              = 30 * time.Second
	sidecarHealthPollInterval         = 200 * time.Millisecond
	sidecarRequestTimeout             = 30 * time.Second
	sidecarCloseTimeout               = 5 * time.Second
	sidecarFrameClientStdin      byte = 0x01
	sidecarFrameClientCloseStdin      = 0x02
	sidecarFrameClientStop            = 0x03
	sidecarFrameServerStdout     byte = 0x01
	sidecarFrameServerStderr          = 0x02
	sidecarFrameServerExit            = 0x03
	sidecarFrameServerError           = 0x04
)

type sidecarTransport struct {
	logger             *slog.Logger
	newClient          sandboxClientFactory
	bootstrap          transport
	clientDialer       sshClientDialer
	httpClient         *http.Client
	healthTimeout      time.Duration
	healthPollInterval time.Duration
	closeTimeout       time.Duration
	binaryMu           sync.Mutex
	binaries           map[string][]byte
}

type sidecarEndpoint struct {
	base       *url.URL
	httpClient *http.Client
	wsDialer   *websocket.Dialer
	closeFn    func() error
}

type sidecarHealthResponse struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
}

type sidecarLaunchRequest struct {
	Command string `json:"command"`
}

type sidecarLaunchResponse struct {
	ID string `json:"id"`
}

type sidecarExitPayload struct {
	ExitCode int    `json:"exitCode"`
	Stderr   string `json:"stderr"`
}

type deadlineConn struct {
	net.Conn
}

func (c deadlineConn) SetDeadline(time.Time) error {
	return nil
}

func (c deadlineConn) SetReadDeadline(time.Time) error {
	return nil
}

func (c deadlineConn) SetWriteDeadline(time.Time) error {
	return nil
}

func newSidecarTransport(
	logger *slog.Logger,
	newClient sandboxClientFactory,
	bootstrap transport,
) *sidecarTransport {
	if logger == nil {
		logger = slog.Default()
	}
	var clientDialer sshClientDialer
	if dialer, ok := bootstrap.(sshClientDialer); ok {
		clientDialer = dialer
	}
	return &sidecarTransport{
		logger:       logger,
		newClient:    newClient,
		bootstrap:    bootstrap,
		clientDialer: clientDialer,
		httpClient: &http.Client{
			Timeout: sidecarRequestTimeout,
		},
		healthTimeout:      sidecarHealthTimeout,
		healthPollInterval: sidecarHealthPollInterval,
		closeTimeout:       sidecarCloseTimeout,
		binaries:           make(map[string][]byte),
	}
}

func (t *sidecarTransport) Dial(
	ctx context.Context,
	sandbox sandboxInfo,
	command string,
) (_ transportSession, err error) {
	if t == nil {
		return nil, errors.New("sandbox/daytona: launcher sidecar transport is required")
	}
	endpoint, err := t.ensureSidecar(ctx, sandbox)
	if err != nil {
		return nil, err
	}
	return t.dialEndpoint(ctx, endpoint, command)
}

func (t *sidecarTransport) dialEndpoint(
	ctx context.Context,
	endpoint sidecarEndpoint,
	command string,
) (_ transportSession, err error) {
	endpointOwned := false
	defer func() {
		if !endpointOwned {
			err = errors.Join(err, endpoint.Close())
		}
	}()
	sessionID, err := t.launch(ctx, endpoint, command)
	if err != nil {
		return nil, err
	}
	session, err := t.connect(ctx, endpoint, sessionID)
	if err != nil {
		return nil, err
	}
	endpointOwned = true
	return session, nil
}

func (t *sidecarTransport) ensureSidecar(
	ctx context.Context,
	info sandboxInfo,
) (_ sidecarEndpoint, err error) {
	sandbox, err := t.loadSandbox(ctx, info)
	if err != nil {
		return sidecarEndpoint{}, err
	}
	binary, err := t.sidecarBinary(ctx, info)
	if err != nil {
		return sidecarEndpoint{}, err
	}
	if err := sandbox.WriteFile(ctx, launcherSidecarPath, binary); err != nil {
		return sidecarEndpoint{}, fmt.Errorf("sandbox/daytona: upload launcher sidecar: %w", err)
	}
	endpoint, err := t.openTunnel(ctx, info)
	if err != nil {
		return sidecarEndpoint{}, err
	}
	endpointOwned := false
	defer func() {
		if !endpointOwned {
			err = errors.Join(err, endpoint.Close())
		}
	}()
	healthy, err := t.health(ctx, endpoint)
	if err == nil && healthy {
		endpointOwned = true
		return endpoint, nil
	}
	if err := t.startSidecar(ctx, info); err != nil {
		return sidecarEndpoint{}, err
	}
	if err := t.waitForHealth(ctx, endpoint); err != nil {
		return sidecarEndpoint{}, err
	}
	endpointOwned = true
	return endpoint, nil
}

func (t *sidecarTransport) openTunnel(ctx context.Context, sandbox sandboxInfo) (sidecarEndpoint, error) {
	if t.clientDialer == nil {
		return sidecarEndpoint{}, errors.New("sandbox/daytona: launcher sidecar SSH tunnel is not configured")
	}
	client, err := t.clientDialer.DialClient(ctx, sandbox)
	if err != nil {
		return sidecarEndpoint{}, fmt.Errorf("sandbox/daytona: open launcher sidecar SSH tunnel: %w", err)
	}
	baseURL, err := url.Parse("http://sidecar")
	if err != nil {
		return sidecarEndpoint{}, fmt.Errorf(
			"sandbox/daytona: parse launcher sidecar tunnel base URL: %w",
			errors.Join(err, client.Close()),
		)
	}
	targetAddr := fmt.Sprintf("127.0.0.1:%d", launcherSidecarPort)
	httpTransport := &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			conn, err := client.Dial("tcp", targetAddr)
			if err != nil {
				return nil, err
			}
			return deadlineConn{Conn: conn}, nil
		},
	}
	httpClient := &http.Client{
		Transport: httpTransport,
		Timeout:   sidecarRequestTimeout,
	}
	wsDialer := &websocket.Dialer{
		NetDialContext: func(context.Context, string, string) (net.Conn, error) {
			conn, err := client.Dial("tcp", targetAddr)
			if err != nil {
				return nil, err
			}
			return deadlineConn{Conn: conn}, nil
		},
	}
	return sidecarEndpoint{
		base:       baseURL,
		httpClient: httpClient,
		wsDialer:   wsDialer,
		closeFn: func() error {
			httpTransport.CloseIdleConnections()
			return client.Close()
		},
	}, nil
}
