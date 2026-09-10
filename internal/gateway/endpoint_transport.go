package gateway

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"github.com/compozy/compozy/internal/outboundpolicy"
)

// The relay carries opaque TLS bytes; the original URL still owns SNI, certificate and nonce checks.
type verificationRelayResolver struct{ address netip.Addr }

func (r verificationRelayResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{r.address}, nil
}

type verificationRelayDialer struct {
	address string
	dialer  outboundpolicy.NetworkDialer
}

func (d verificationRelayDialer) DialContext(ctx context.Context, network, _ string) (net.Conn, error) {
	return outboundpolicy.NewDialer(outboundpolicy.New(true), net.DefaultResolver, d.dialer).
		DialContext(ctx, network, d.address)
}

var errEndpointRedirect = errors.New("gateway: endpoint verification redirects are forbidden")

func endpointProbeError(err error) error {
	reason := "endpoint transport failed; inspect provider connectivity"
	switch {
	case errors.Is(err, errEndpointRedirect):
		reason = "endpoint redirect refused; forward the exact challenge path"
	case errors.Is(err, outboundpolicy.ErrBlockedDestination):
		reason = "endpoint address policy refused the destination"
	case errors.Is(err, context.Canceled):
		reason = "endpoint probe canceled"
	case errors.Is(err, context.DeadlineExceeded):
		reason = "endpoint probe timed out; check provider routing and listener readiness"
	case errors.Is(err, syscall.ECONNREFUSED):
		reason = "endpoint connection refused; check provider listener readiness"
	case errors.Is(err, syscall.ENETUNREACH), errors.Is(err, syscall.EHOSTUNREACH):
		reason = "endpoint network unreachable; check the provider network route"
	default:
		if dnsErr, ok := errors.AsType[*net.DNSError](err); ok && dnsErr != nil {
			reason = "endpoint DNS resolution failed; check the provider resolver"
		} else if certErr, ok := errors.AsType[*tls.CertificateVerificationError](err); ok && certErr != nil {
			reason = "endpoint TLS certificate verification failed; check hostname, trust and certificate validity"
		} else if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
			reason = "endpoint probe timed out; check provider routing and listener readiness"
		}
	}
	return fmt.Errorf("%w: %s", ErrEndpointUnverified, reason)
}

func validateVerificationAddress(value string) error {
	if value == "" {
		return nil
	}
	address, err := netip.ParseAddrPort(value)
	if err != nil || !address.Addr().IsLoopback() || address.Addr().Zone() != "" || address.Port() == 0 {
		return errors.New("gateway: verification address must be a loopback IP with a non-zero port")
	}
	return nil
}
