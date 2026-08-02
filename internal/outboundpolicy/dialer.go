package outboundpolicy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
)

// Resolver resolves all IP addresses for one outbound hostname.
type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

// NetworkDialer establishes one connection to an already-approved IP address.
type NetworkDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

// Dialer validates the complete DNS answer immediately before connecting to a pinned IP.
type Dialer struct {
	policy   Policy
	resolver Resolver
	dialer   NetworkDialer
}

// NewDialer constructs a policy-enforcing dialer.
func NewDialer(policy Policy, resolver Resolver, dialer NetworkDialer) Dialer {
	return Dialer{policy: policy, resolver: resolver, dialer: dialer}
}

// DialContext resolves, validates, and connects without performing a second hostname lookup.
func (d Dialer) DialContext(ctx context.Context, network string, endpoint string) (net.Conn, error) {
	if d.resolver == nil || d.dialer == nil {
		return nil, errors.New("outbound policy: dialer is not configured")
	}
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return nil, fmt.Errorf("outbound policy: invalid destination address: %w", err)
	}
	addresses, err := d.resolveAddresses(ctx, host, port)
	if err != nil {
		return nil, err
	}
	dialErrors := make([]error, 0, len(addresses))
	for _, address := range addresses {
		connection, dialErr := d.dialer.DialContext(ctx, network, net.JoinHostPort(address.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		dialErrors = append(dialErrors, dialErr)
	}
	return nil, fmt.Errorf("outbound policy: connect destination: %w", errors.Join(dialErrors...))
}

func (d Dialer) resolveAddresses(
	ctx context.Context,
	host string,
	port string,
) ([]netip.Addr, error) {
	if address, err := netip.ParseAddr(host); err == nil {
		if err := d.policy.ValidateResolvedAddresses([]netip.Addr{address}, host, port); err != nil {
			return nil, err
		}
		return []netip.Addr{address.Unmap()}, nil
	}
	addresses, err := d.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("outbound policy: resolve destination: %w", err)
	}
	if err := d.policy.ValidateResolvedAddresses(addresses, host, port); err != nil {
		return nil, err
	}
	return addresses, nil
}
