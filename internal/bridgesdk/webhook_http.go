package bridgesdk

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

type webhookIPResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type webhookNetworkDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type safeWebhookDialer struct {
	resolver webhookIPResolver
	dialer   webhookNetworkDialer
}

func newWebhookHTTPClient(resolver webhookIPResolver, dialer webhookNetworkDialer) *http.Client {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if dialer == nil {
		dialer = &net.Dialer{Timeout: webhookReachabilityTimeout}
	}
	safeDialer := &safeWebhookDialer{resolver: resolver, dialer: dialer}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           safeDialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   webhookReachabilityTimeout,
		ResponseHeaderTimeout: webhookReachabilityTimeout,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   webhookReachabilityTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (d *safeWebhookDialer) DialContext(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	if d == nil || d.resolver == nil || d.dialer == nil {
		return nil, errors.New("bridgesdk: safe webhook dialer is not configured")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("bridgesdk: parse webhook destination: %w", err)
	}

	addresses, err := d.resolvePublicAddresses(ctx, host)
	if err != nil {
		return nil, err
	}
	dialErrors := make([]error, 0, len(addresses))
	for _, ip := range addresses {
		connection, dialErr := d.dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		dialErrors = append(dialErrors, dialErr)
	}
	return nil, fmt.Errorf("bridgesdk: dial public webhook destination: %w", errors.Join(dialErrors...))
}

func (d *safeWebhookDialer) resolvePublicAddresses(
	ctx context.Context,
	host string,
) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		if err := bridgepkg.ValidateWebhookDestinationIP(ip); err != nil {
			return nil, err
		}
		return []netip.Addr{ip.Unmap()}, nil
	}

	addresses, err := d.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("bridgesdk: resolve webhook destination: %w", err)
	}
	if len(addresses) == 0 {
		return nil, errors.New("bridgesdk: webhook destination resolved to no addresses")
	}
	validated := make([]netip.Addr, 0, len(addresses))
	for _, ip := range addresses {
		if err := bridgepkg.ValidateWebhookDestinationIP(ip); err != nil {
			return nil, err
		}
		validated = append(validated, ip.Unmap())
	}
	return validated, nil
}
