// Package client provides a typed, standard-library-only CompozyOS app client.
package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	identityTimeout = 5 * time.Second
	requestTimeout  = 30 * time.Second
)

// Client connects an app to a CompozyOS daemon.
type Client struct {
	httpClient    *http.Client
	baseURL       string
	token         string
	clientID      string
	logger        *slog.Logger
	daemonVersion string
}

type options struct {
	baseURL    string
	socket     string
	home       string
	token      string
	clientID   string
	httpClient *http.Client
	logger     *slog.Logger
}

// Option configures daemon discovery or requests.
type Option func(*options)

// WithBaseURL selects an HTTP origin ahead of environment and local discovery.
func WithBaseURL(value string) Option { return func(o *options) { o.baseURL = value } }

// WithSocket selects the unix socket tried before the home directory's TCP record.
func WithSocket(value string) Option { return func(o *options) { o.socket = value } }

// WithHome selects the daemon home ahead of COMPOZY_HOME and ~/.compozy.
func WithHome(value string) Option { return func(o *options) { o.home = value } }

// WithToken sets the bearer token sent on every request.
func WithToken(value string) Option { return func(o *options) { o.token = value } }

// WithHTTPClient uses a caller-configured HTTP client without mutating it.
func WithHTTPClient(value *http.Client) Option { return func(o *options) { o.httpClient = value } }

// WithLogger selects the logger for daemon version warnings.
func WithLogger(value *slog.Logger) Option { return func(o *options) { o.logger = value } }

// WithClientID sets the X-Compozy-Client-ID header (default "sdk-go").
func WithClientID(value string) Option { return func(o *options) { o.clientID = value } }

// New discovers a daemon and verifies its identity before returning a client.
func New(ctx context.Context, opts ...Option) (*Client, error) {
	o := options{token: os.Getenv("COMPOZY_APP_TOKEN"), clientID: "sdk-go", logger: slog.Default()}
	for _, opt := range opts {
		if opt == nil {
			return nil, errors.New("compozy client: nil option")
		}
		opt(&o)
	}
	if o.logger == nil {
		o.logger = slog.Default()
	}
	if strings.TrimSpace(o.clientID) == "" {
		return nil, errors.New("compozy client: client ID must not be empty")
	}
	c, err := discover(ctx, o)
	if err != nil {
		return nil, err
	}
	warnVersionMismatch(c.logger, moduleVersion(), c.daemonVersion)
	return c, nil
}

// DaemonVersion returns the version reported when the client connected.
func (c *Client) DaemonVersion() string { return c.daemonVersion }

func connect(ctx context.Context, o options, baseURL, socket string) (*Client, error) {
	ctx, cancel := context.WithTimeout(ctx, identityTimeout)
	defer cancel()
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("compozy client: parse base URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("compozy client: base URL must be an HTTP(S) URL without credentials, query, or fragment")
	}
	httpClient, err := transportClient(o.httpClient, socket)
	if err != nil {
		return nil, err
	}
	c := &Client{
		httpClient: httpClient, baseURL: strings.TrimRight(baseURL, "/"),
		token: o.token, clientID: o.clientID, logger: o.logger,
	}
	var identity struct {
		Daemon struct {
			Version string `json:"version"`
		} `json:"daemon"`
	}
	if err := c.Do(ctx, http.MethodGet, "/api/status/identity", nil, &identity); err != nil {
		httpClient.CloseIdleConnections()
		return nil, err
	}
	if strings.TrimSpace(identity.Daemon.Version) == "" {
		httpClient.CloseIdleConnections()
		return nil, errors.New("compozy client: identity response has no daemon version")
	}
	c.daemonVersion = identity.Daemon.Version
	return c, nil
}

func transportClient(custom *http.Client, socket string) (*http.Client, error) {
	var result http.Client
	if custom != nil {
		result = *custom
	}
	if socket == "" && result.Transport != nil {
		return &result, nil
	}
	transport := &http.Transport{
		Proxy:             http.ProxyFromEnvironment,
		DialContext:       (&net.Dialer{Timeout: identityTimeout, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2: true, MaxIdleConns: 100, IdleConnTimeout: 90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: requestTimeout,
		ExpectContinueTimeout: time.Second,
	}
	if result.Transport != nil {
		existing, ok := result.Transport.(*http.Transport)
		if !ok {
			return nil, errors.New("compozy client: unix sockets require an *http.Transport")
		}
		transport = existing.Clone()
	}
	if socket != "" {
		transport.Proxy = nil
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: identityTimeout}).DialContext(ctx, "unix", socket)
		}
	}
	result.Transport = transport
	return &result, nil
}
