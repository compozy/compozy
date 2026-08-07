package gateway

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/outboundpolicy"
)

const maxChallengeResponseBytes = 64 << 10

// EndpointVerification is the core-owned proof boundary used by provider supervision.
type EndpointVerification interface {
	Verify(context.Context, Tier, AdvertisedEndpoint, string, string) error
	Timeout() time.Duration
}

// EndpointVerifier proves that one provider endpoint reaches the assigned tier listener.
type EndpointVerifier struct {
	timeout  time.Duration
	resolver outboundpolicy.Resolver
	dialer   outboundpolicy.NetworkDialer
	rootCAs  *x509.CertPool
}

// Timeout returns the dedicated verification deadline.
func (v *EndpointVerifier) Timeout() time.Duration {
	if v == nil {
		return 0
	}
	return v.timeout
}

var _ EndpointVerification = (*EndpointVerifier)(nil)

// NewEndpointVerifier constructs a verifier with a dedicated client timeout.
func NewEndpointVerifier(timeout time.Duration) (*EndpointVerifier, error) {
	if timeout <= 0 {
		return nil, errors.New("gateway: endpoint verification timeout must be positive")
	}
	return &EndpointVerifier{
		timeout: timeout, resolver: net.DefaultResolver, dialer: &net.Dialer{Timeout: timeout},
	}, nil
}

// Verify fetches the tier-scoped challenge and requires an exact nonce echo.
func (v *EndpointVerifier) Verify(
	ctx context.Context,
	tier Tier,
	endpoint AdvertisedEndpoint,
	challengePath string,
	nonce string,
) error {
	if v == nil || v.resolver == nil || v.dialer == nil || v.timeout <= 0 {
		return errors.New("gateway: endpoint verifier is not configured")
	}
	if err := tier.Validate(); err != nil {
		return err
	}
	if err := endpoint.Validate(); err != nil {
		return fmt.Errorf("%w: invalid endpoint descriptor", ErrEndpointUnverified)
	}
	if !strings.EqualFold(endpoint.Scheme, endpointSchemeHTTPS) {
		return fmt.Errorf("%w: HTTPS is required", ErrEndpointUnverified)
	}
	if !strings.HasPrefix(challengePath, ChallengePathPrefix) || strings.TrimSpace(nonce) == "" {
		return fmt.Errorf("%w: challenge is unavailable", ErrEndpointUnverified)
	}
	challengeURL, err := endpointChallengeURL(endpoint.URL, challengePath)
	if err != nil {
		return fmt.Errorf("%w: invalid challenge URL", ErrEndpointUnverified)
	}
	client, err := v.clientFor(tier, endpoint.URL)
	if err != nil {
		return fmt.Errorf("%w: endpoint policy refused", ErrEndpointUnverified)
	}
	requestCtx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, challengeURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("%w: construct challenge request", ErrEndpointUnverified)
	}
	request.Header.Set("Cache-Control", "no-store")
	request.Close = true
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: endpoint probe failed", ErrEndpointUnverified)
	}
	body, readErr := readBoundedChallengeBody(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		return fmt.Errorf("%w: challenge response is invalid", ErrEndpointUnverified)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: challenge returned a non-success status", ErrEndpointUnverified)
	}
	if string(body) != nonce {
		return fmt.Errorf("%w: challenge nonce did not match", ErrEndpointUnverified)
	}
	return nil
}

func (v *EndpointVerifier) clientFor(tier Tier, endpointURL string) (*http.Client, error) {
	policy := outboundpolicy.New(false)
	if tier == TierPrivate {
		var err error
		policy, err = policy.WithTrustedOrigin(endpointURL)
		if err != nil {
			return nil, err
		}
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: v.rootCAs}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           outboundpolicy.NewDialer(policy, v.resolver, v.dialer).DialContext,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   v.timeout,
		ResponseHeaderTimeout: v.timeout,
		IdleConnTimeout:       v.timeout,
		DisableKeepAlives:     true,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   v.timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("gateway: endpoint verification redirects are forbidden")
		},
	}, nil
}

func endpointChallengeURL(rawEndpoint string, challengePath string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawEndpoint))
	if err != nil || parsed == nil {
		return "", errors.New("gateway: parse endpoint URL")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + challengePath
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func readBoundedChallengeBody(body io.Reader) ([]byte, error) {
	if body == nil {
		return nil, errors.New("gateway: challenge response body is required")
	}
	result, err := io.ReadAll(io.LimitReader(body, maxChallengeResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("gateway: read challenge response: %w", err)
	}
	if len(result) > maxChallengeResponseBytes {
		return nil, errors.New("gateway: challenge response exceeds 64 KiB")
	}
	return result, nil
}
