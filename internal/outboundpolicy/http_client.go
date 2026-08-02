package outboundpolicy

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"
)

const defaultMaxRedirects = 10

// NewHTTPClient clones a client and installs URL, redirect, and dial-time policy.
// A custom non-*http.Transport RoundTripper remains the explicit owner of its connection boundary.
func NewHTTPClient(base *http.Client, policy Policy, timeout time.Duration, maxRedirects int) *http.Client {
	if base == nil {
		base = &http.Client{}
	}
	client := *base
	if timeout > 0 && client.Timeout <= 0 {
		client.Timeout = timeout
	}
	client.Transport = secureRoundTripper(client.Transport, policy, timeout)
	client.CheckRedirect = secureRedirectPolicy(policy, client.CheckRedirect, maxRedirects)
	return &client
}

func secureRoundTripper(base http.RoundTripper, policy Policy, timeout time.Duration) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	transport, ok := base.(*http.Transport)
	if !ok {
		return policyRoundTripper{base: base, policy: policy}
	}

	clone := transport.Clone()
	clone.Proxy = nil
	underlying := NetworkDialer(&net.Dialer{Timeout: timeout})
	if clone.DialContext != nil {
		underlying = networkDialerFunc(clone.DialContext)
	}
	clone.DialContext = NewDialer(policy, net.DefaultResolver, underlying).DialContext
	clone.DialTLSContext = nil
	//nolint:staticcheck // Clearing the deprecated hook prevents an inherited client from bypassing DialContext.
	clone.DialTLS = nil
	if clone.TLSClientConfig == nil {
		clone.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		clone.TLSClientConfig = clone.TLSClientConfig.Clone()
		if clone.TLSClientConfig.MinVersion < tls.VersionTLS12 {
			clone.TLSClientConfig.MinVersion = tls.VersionTLS12
		}
	}
	return policyRoundTripper{base: clone, policy: policy}
}

func secureRedirectPolicy(
	policy Policy,
	prior func(*http.Request, []*http.Request) error,
	maxRedirects int,
) func(*http.Request, []*http.Request) error {
	if maxRedirects <= 0 {
		maxRedirects = defaultMaxRedirects
	}
	return func(next *http.Request, previous []*http.Request) error {
		if next == nil {
			return ErrInvalidURL
		}
		if len(previous) >= maxRedirects {
			return errors.New("outbound policy: too many redirects")
		}
		if prior != nil {
			if err := prior(next, previous); err != nil {
				return err
			}
		}
		if err := policy.ValidateURL(next.URL); err != nil {
			return err
		}
		if len(previous) != 0 && !SameOrigin(previous[len(previous)-1].URL, next.URL) {
			stripSensitiveHeaders(next.Header)
		}
		return nil
	}
}

func stripSensitiveHeaders(headers http.Header) {
	headers.Del("Authorization")
	headers.Del("Cookie")
	headers.Del("Proxy-Authorization")
}

type policyRoundTripper struct {
	base   http.RoundTripper
	policy Policy
}

func (t policyRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil {
		return nil, ErrInvalidURL
	}
	if err := t.policy.ValidateURL(request.URL); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(request)
}

func (t policyRoundTripper) CloseIdleConnections() {
	if closer, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

type networkDialerFunc func(context.Context, string, string) (net.Conn, error)

func (f networkDialerFunc) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	return f(ctx, network, address)
}
