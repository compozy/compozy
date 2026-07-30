package securehttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
)

type secureDialer struct {
	policy   policy
	resolver ipResolver
	dialer   networkDialer
}

func (d secureDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if d.resolver == nil || d.dialer == nil {
		return nil, errors.New("securehttp: dialer is not configured")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("securehttp: invalid destination address: %w", err)
	}
	explicitLoopback := isExplicitLoopbackHost(host)
	addresses, err := d.resolveAddresses(ctx, host, explicitLoopback)
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
	return nil, fmt.Errorf("securehttp: connect destination: %w", errors.Join(dialErrors...))
}

func (d secureDialer) resolveAddresses(
	ctx context.Context,
	host string,
	explicitLoopback bool,
) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		if err := d.policy.validateAddresses([]netip.Addr{ip}, explicitLoopback); err != nil {
			return nil, err
		}
		return []netip.Addr{ip.Unmap()}, nil
	}
	addresses, err := d.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("securehttp: resolve destination: %w", err)
	}
	if err := d.policy.validateAddresses(addresses, explicitLoopback); err != nil {
		return nil, err
	}
	return addresses, nil
}

type secureTransport struct {
	base            http.RoundTripper
	policy          policy
	maxResponseSize int64
}

var _ http.RoundTripper = secureTransport{}

func (t secureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil {
		return nil, ErrInvalidURL
	}
	if err := t.policy.validateURL(request.URL); err != nil {
		return nil, err
	}
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	if response.Body != nil {
		response.Body = &limitedReadCloser{ReadCloser: response.Body, remaining: t.maxResponseSize}
	}
	return response, nil
}
