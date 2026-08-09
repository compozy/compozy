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
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
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
	mu             sync.RWMutex
	timeout        time.Duration
	resolver       outboundpolicy.Resolver
	publicResolver outboundpolicy.Resolver
	dialer         outboundpolicy.NetworkDialer
	rootCAs        *x509.CertPool
}

// Timeout returns the dedicated verification deadline.
func (v *EndpointVerifier) Timeout() time.Duration {
	if v == nil {
		return 0
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.timeout
}

var _ EndpointVerification = (*EndpointVerifier)(nil)

// NewEndpointVerifier constructs a verifier with a dedicated client timeout and public DNS path.
func NewEndpointVerifier(timeout time.Duration, publicDNSResolver string) (*EndpointVerifier, error) {
	if timeout <= 0 {
		return nil, errors.New("gateway: endpoint verification timeout must be positive")
	}
	publicResolver, err := newPublicDNSResolver(publicDNSResolver, timeout)
	if err != nil {
		return nil, err
	}
	return &EndpointVerifier{
		timeout: timeout, resolver: net.DefaultResolver, publicResolver: publicResolver,
		dialer: &net.Dialer{Timeout: timeout},
	}, nil
}

// Reconfigure applies live verification settings to future probes.
func (v *EndpointVerifier) Reconfigure(timeout time.Duration, publicDNSResolver string) error {
	if v == nil {
		return errors.New("gateway: endpoint verifier is required")
	}
	if timeout <= 0 {
		return errors.New("gateway: endpoint verification timeout must be positive")
	}
	resolver, err := newPublicDNSResolver(publicDNSResolver, timeout)
	if err != nil {
		return err
	}
	v.mu.Lock()
	v.timeout = timeout
	v.publicResolver = resolver
	if current, ok := v.dialer.(*net.Dialer); ok {
		updated := *current
		updated.Timeout = timeout
		v.dialer = &updated
	}
	v.mu.Unlock()
	return nil
}

// Verify fetches the tier-scoped challenge and requires an exact nonce echo.
func (v *EndpointVerifier) Verify(
	ctx context.Context,
	tier Tier,
	endpoint AdvertisedEndpoint,
	challengePath string,
	nonce string,
) error {
	timeout, resolver, publicResolver, dialer, rootCAs := v.configuration()
	if timeout <= 0 || resolver == nil || publicResolver == nil || dialer == nil {
		return errors.New("gateway: endpoint verifier is not configured")
	}
	if err := tier.Validate(); err != nil {
		return err
	}
	if err := endpoint.Validate(); err != nil {
		return fmt.Errorf("%w: invalid endpoint descriptor", ErrEndpointUnverified)
	}
	if tier == TierPublic && !strings.EqualFold(endpoint.Scheme, endpointSchemeHTTPS) {
		return fmt.Errorf("%w: HTTPS is required", ErrEndpointUnverified)
	}
	if !strings.HasPrefix(challengePath, ChallengePathPrefix) || strings.TrimSpace(nonce) == "" {
		return fmt.Errorf("%w: challenge is unavailable", ErrEndpointUnverified)
	}
	challengeURL, err := endpointChallengeURL(endpoint, challengePath)
	if err != nil {
		return fmt.Errorf("%w: invalid challenge URL", ErrEndpointUnverified)
	}
	if tier == TierPublic {
		resolver = publicResolver
	}
	client, err := endpointVerificationClient(tier, challengeURL, timeout, resolver, dialer, rootCAs)
	if err != nil {
		return fmt.Errorf("%w: endpoint policy refused", ErrEndpointUnverified)
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
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

func (v *EndpointVerifier) configuration() (
	time.Duration,
	outboundpolicy.Resolver,
	outboundpolicy.Resolver,
	outboundpolicy.NetworkDialer,
	*x509.CertPool,
) {
	if v == nil {
		return 0, nil, nil, nil, nil
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.timeout, v.resolver, v.publicResolver, v.dialer, v.rootCAs
}

func newPublicDNSResolver(address string, timeout time.Duration) (*net.Resolver, error) {
	return newPublicDNSResolverWithTransport(address, timeout, nil, nil)
}

type publicDNSDialFunc func(context.Context, string, string) (net.Conn, error)

func newPublicDNSResolverWithTransport(
	address string,
	timeout time.Duration,
	rootCAs *x509.CertPool,
	dial publicDNSDialFunc,
) (*net.Resolver, error) {
	trimmed := strings.TrimSpace(address)
	resolverAddress, err := netip.ParseAddrPort(trimmed)
	if err != nil || trimmed != address || resolverAddress.Port() == 0 {
		return nil, errors.New("gateway: public DNS resolver must be a public IP address with a non-zero port")
	}
	if err := outboundpolicy.New(false).ValidateResolvedAddresses(
		[]netip.Addr{resolverAddress.Addr()},
		resolverAddress.Addr().String(),
		strconv.Itoa(int(resolverAddress.Port())),
	); err != nil {
		return nil, errors.New("gateway: public DNS resolver must be a public IP address with a non-zero port")
	}
	if dial == nil {
		dialer := &net.Dialer{Timeout: timeout}
		dial = dialer.DialContext
	}
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
		ServerName: resolverAddress.Addr().String(),
	}
	return &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
		Dial: func(ctx context.Context, _ string, _ string) (net.Conn, error) {
			connection, err := dial(ctx, "tcp", resolverAddress.String())
			if err != nil {
				return nil, fmt.Errorf("gateway: connect public DNS-over-TLS resolver: %w", err)
			}
			secured := tls.Client(connection, tlsConfig.Clone())
			if err := secured.HandshakeContext(ctx); err != nil {
				return nil, errors.Join(
					fmt.Errorf("gateway: authenticate public DNS-over-TLS resolver: %w", err),
					connection.Close(),
				)
			}
			return secured, nil
		},
	}, nil
}

func endpointVerificationClient(
	tier Tier,
	endpointURL string,
	timeout time.Duration,
	resolver outboundpolicy.Resolver,
	dialer outboundpolicy.NetworkDialer,
	rootCAs *x509.CertPool,
) (*http.Client, error) {
	policy := outboundpolicy.New(false)
	if tier == TierPrivate {
		var err error
		policy, err = policy.WithTrustedOrigin(endpointURL)
		if err != nil {
			return nil, err
		}
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: rootCAs}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           outboundpolicy.NewDialer(policy, resolver, dialer).DialContext,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		IdleConnTimeout:       timeout,
		DisableKeepAlives:     true,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("gateway: endpoint verification redirects are forbidden")
		},
	}, nil
}

func endpointChallengeURL(endpoint AdvertisedEndpoint, challengePath string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint.URL))
	if err != nil || parsed == nil {
		return "", errors.New("gateway: parse endpoint URL")
	}
	if !strings.EqualFold(endpoint.Scheme, endpointSchemeHTTPS) {
		parsed.Scheme = endpointSchemeHTTPS
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
