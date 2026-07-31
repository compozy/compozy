// Package securehttp provides outbound HTTP clients for remote MCP and OAuth services.
package securehttp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"
)

const (
	defaultTimeout          = 30 * time.Second
	defaultResponseBodySize = int64(1 << 20)
	defaultMaxRedirects     = 5
)

var (
	// ErrInvalidURL reports a URL that cannot be used as a remote HTTP destination.
	ErrInvalidURL = errors.New("securehttp: invalid destination URL")
	// ErrInsecureTransport reports HTTP used for a non-loopback destination.
	ErrInsecureTransport = errors.New("securehttp: HTTPS is required for non-loopback destinations")
	// ErrBlockedDestination reports an address outside the configured destination policy.
	ErrBlockedDestination = errors.New("securehttp: destination is blocked by network policy")
	// ErrTooManyRedirects reports a redirect chain beyond the client's configured limit.
	ErrTooManyRedirects = errors.New("securehttp: too many redirects")
	// ErrResponseBodyTooLarge reports a response body that exceeds the configured limit.
	ErrResponseBodyTooLarge = errors.New("securehttp: response body exceeds the configured limit")
)

// Client is an outbound HTTP client whose requests are checked against the MCP network policy.
type Client struct {
	httpClient *http.Client
	policy     policy
}

// Option configures a Client.
type Option func(*options)

type options struct {
	allowLoopback   bool
	timeout         time.Duration
	maxResponseSize int64
	maxRedirects    int
}

// WithAllowLoopback permits explicit loopback destinations, including HTTP.
// It is disabled by default so catalog destinations remain public-only.
func WithAllowLoopback(allow bool) Option {
	return func(options *options) {
		options.allowLoopback = allow
	}
}

// WithTimeout configures the end-to-end timeout. A non-positive value keeps the default.
func WithTimeout(timeout time.Duration) Option {
	return func(options *options) {
		if timeout > 0 {
			options.timeout = timeout
		}
	}
}

// WithMaxResponseBodyBytes configures the maximum readable response body. A non-positive value keeps the default.
func WithMaxResponseBodyBytes(size int64) Option {
	return func(options *options) {
		if size > 0 {
			options.maxResponseSize = size
		}
	}
}

// NewClient constructs a secure, proxy-free HTTP client.
func NewClient(clientOptions ...Option) *Client {
	configured := options{
		timeout:         defaultTimeout,
		maxResponseSize: defaultResponseBodySize,
		maxRedirects:    defaultMaxRedirects,
	}
	for _, option := range clientOptions {
		if option != nil {
			option(&configured)
		}
	}
	return newClient(configured, net.DefaultResolver, &net.Dialer{Timeout: configured.timeout})
}

// Do executes a request and returns errors without request URLs or credentials.
func (c *Client) Do(request *http.Request) (*http.Response, error) {
	if c == nil || c.httpClient == nil {
		return nil, errors.New("securehttp: client is not configured")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, RedactError(err)
	}
	return response, nil
}

// HTTPClient returns the underlying client for APIs that require *http.Client.
// Call Do directly or pass API errors through RedactError at the boundary.
func (c *Client) HTTPClient() *http.Client {
	if c == nil {
		return nil
	}
	return c.httpClient
}

// ValidateURL checks a URL against this client's static destination policy.
// DNS answers are revalidated immediately before every connection.
func (c *Client) ValidateURL(raw string) error {
	if c == nil {
		return errors.New("securehttp: client is not configured")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ErrInvalidURL
	}
	return c.policy.validateURL(parsed)
}

// ValidateURL checks a URL against the default public-only destination policy.
func ValidateURL(raw string) error {
	return NewClient().ValidateURL(raw)
}

func newClient(configured options, resolver ipResolver, dialer networkDialer) *Client {
	clientPolicy := policy{allowLoopback: configured.allowLoopback}
	safeDialer := secureDialer{policy: clientPolicy, resolver: resolver, dialer: dialer}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           safeDialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	securedTransport := secureTransport{
		base:            transport,
		policy:          clientPolicy,
		maxResponseSize: configured.maxResponseSize,
	}
	result := &Client{policy: clientPolicy}
	result.httpClient = &http.Client{
		Transport:     securedTransport,
		Timeout:       configured.timeout,
		CheckRedirect: result.checkRedirect(configured.maxRedirects),
	}
	return result
}

func (c *Client) checkRedirect(maxRedirects int) func(*http.Request, []*http.Request) error {
	return func(next *http.Request, previous []*http.Request) error {
		if err := c.policy.validateURL(next.URL); err != nil {
			return err
		}
		if len(previous) >= maxRedirects {
			return ErrTooManyRedirects
		}
		if len(previous) != 0 && !sameOrigin(previous[len(previous)-1].URL, next.URL) {
			next.Header.Del("Authorization")
			next.Header.Del("Cookie")
			next.Header.Del("Proxy-Authorization")
		}
		return nil
	}
}

// RedactError removes request URLs and credentials from errors returned by APIs
// that require the underlying HTTP client.
func RedactError(err error) error {
	if err == nil {
		return nil
	}
	redacted := err
	for {
		var requestError *url.Error
		if !errors.As(redacted, &requestError) {
			break
		}
		redacted = requestError.Err
		if redacted == nil {
			return errors.New("securehttp: request failed")
		}
	}
	return fmt.Errorf("securehttp: request failed: %w", redacted)
}

type ipResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type networkDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}
