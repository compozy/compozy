// Package outboundpolicy defines the shared destination boundary for outbound network clients.
package outboundpolicy

import (
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

var (
	// ErrInvalidURL reports a URL that cannot be used as an outbound destination.
	ErrInvalidURL = errors.New("outbound policy: invalid destination URL")
	// ErrInsecureTransport reports HTTP used outside an explicitly trusted origin or loopback allowance.
	ErrInsecureTransport = errors.New("outbound policy: HTTPS is required for untrusted destinations")
	// ErrBlockedDestination reports an address outside the configured destination policy.
	ErrBlockedDestination = errors.New("outbound policy: destination is blocked")
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// Policy validates URL and resolved-address destinations. Values are immutable after construction.
type Policy struct {
	allowLoopback     bool
	privateOrigins    map[origin]struct{}
	credentialOrigins map[origin]struct{}
}

type origin struct {
	scheme string
	host   string
	port   string
}

// New returns a public-network policy, optionally allowing explicit loopback hosts.
func New(allowLoopback bool) Policy {
	return Policy{allowLoopback: allowLoopback}
}

// WithTrustedOrigin returns a copy that permits one exact operator-owned HTTP(S) origin,
// including its resolved private addresses and credential forwarding. The URL must not contain
// credentials or a fragment.
func (p Policy) WithTrustedOrigin(raw string) (Policy, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Policy{}, ErrInvalidURL
	}
	trusted, err := originFromURL(parsed)
	if err != nil {
		return Policy{}, err
	}
	p.privateOrigins = cloneOriginsWith(p.privateOrigins, trusted)
	p.credentialOrigins = cloneOriginsWith(p.credentialOrigins, trusted)
	return p, nil
}

// WithCredentialOrigin returns a copy that permits credential forwarding to one exact public
// HTTPS origin without granting access to private resolved addresses.
func (p Policy) WithCredentialOrigin(raw string) (Policy, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return Policy{}, ErrInvalidURL
	}
	credentialOrigin, err := originFromURL(parsed)
	if err != nil {
		return Policy{}, err
	}
	if credentialOrigin.scheme != schemeHTTPS {
		return Policy{}, ErrInsecureTransport
	}
	p.credentialOrigins = cloneOriginsWith(p.credentialOrigins, credentialOrigin)
	return p, nil
}

// ValidateURL applies scheme, origin, and literal-address policy before a request is sent.
func (p Policy) ValidateURL(destination *url.URL) error {
	parsedOrigin, err := originFromURL(destination)
	if err != nil {
		return err
	}
	_, allowPrivate := p.privateOrigins[parsedOrigin]
	explicitLoopback := IsExplicitLoopbackHost(parsedOrigin.host)
	if parsedOrigin.scheme != schemeHTTPS {
		if parsedOrigin.scheme != schemeHTTP || (!allowPrivate && (!p.allowLoopback || !explicitLoopback)) {
			return ErrInsecureTransport
		}
	}
	if explicitLoopback && !allowPrivate && !p.allowLoopback {
		return ErrBlockedDestination
	}
	if address, parseErr := netip.ParseAddr(parsedOrigin.host); parseErr == nil {
		return p.validateAddresses([]netip.Addr{address}, explicitLoopback, allowPrivate)
	}
	return nil
}

// ValidateResolvedAddresses rejects a complete DNS answer when any address violates the policy
// for the exact destination host and port.
func (p Policy) ValidateResolvedAddresses(addresses []netip.Addr, host string, port string) error {
	explicitLoopback := IsExplicitLoopbackHost(host)
	allowPrivate := p.allowsPrivateEndpoint(host, port)
	return p.validateAddresses(addresses, explicitLoopback, allowPrivate)
}

func (p Policy) validateAddresses(addresses []netip.Addr, explicitLoopback bool, allowPrivate bool) error {
	if len(addresses) == 0 {
		return ErrBlockedDestination
	}
	for _, address := range addresses {
		if err := p.validateAddress(address, explicitLoopback, allowPrivate); err != nil {
			return err
		}
	}
	return nil
}

func (p Policy) allowsPrivateEndpoint(host string, port string) bool {
	host = normalizeHost(host)
	for configured := range p.privateOrigins {
		if configured.host == host && configured.port == port {
			return true
		}
	}
	return false
}

// IsTrustedOrigin reports whether a URL matches one exact operator-owned origin.
func (p Policy) IsTrustedOrigin(destination *url.URL) bool {
	parsed, err := originFromURL(destination)
	if err != nil {
		return false
	}
	_, trusted := p.credentialOrigins[parsed]
	return trusted
}

// SameOrigin compares normalized scheme, hostname, and effective port.
func SameOrigin(left, right *url.URL) bool {
	leftOrigin, leftErr := originFromURL(left)
	rightOrigin, rightErr := originFromURL(right)
	return leftErr == nil && rightErr == nil && leftOrigin == rightOrigin
}

// IsExplicitLoopbackHost recognizes literal and reserved-name loopback destinations.
func IsExplicitLoopbackHost(host string) bool {
	host = normalizeHost(host)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	address, err := netip.ParseAddr(host)
	return err == nil && address.IsLoopback()
}

func (p Policy) validateAddress(address netip.Addr, explicitLoopback bool, allowPrivate bool) error {
	address = address.Unmap()
	if !address.IsValid() || address.Zone() != "" {
		return ErrBlockedDestination
	}
	if allowPrivate {
		return nil
	}
	if explicitLoopback {
		if p.allowLoopback && address.IsLoopback() {
			return nil
		}
		return ErrBlockedDestination
	}
	if !address.IsGlobalUnicast() || address.IsUnspecified() || address.IsLoopback() ||
		address.IsPrivate() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() ||
		address.IsMulticast() || isSpecialUseAddress(address) {
		return ErrBlockedDestination
	}
	return nil
}

func cloneOriginsWith(existing map[origin]struct{}, added origin) map[origin]struct{} {
	cloned := make(map[origin]struct{}, len(existing)+1)
	for configured := range existing {
		cloned[configured] = struct{}{}
	}
	cloned[added] = struct{}{}
	return cloned
}

func originFromURL(destination *url.URL) (origin, error) {
	if destination == nil || destination.User != nil || destination.Fragment != "" {
		return origin{}, ErrInvalidURL
	}
	scheme := strings.ToLower(strings.TrimSpace(destination.Scheme))
	if scheme != schemeHTTP && scheme != schemeHTTPS {
		return origin{}, ErrInvalidURL
	}
	host := normalizeHost(destination.Hostname())
	if host == "" {
		return origin{}, ErrInvalidURL
	}
	port := destination.Port()
	if port == "" {
		port = effectivePort(scheme)
	} else {
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil || number == 0 {
			return origin{}, ErrInvalidURL
		}
	}
	if _, _, err := net.SplitHostPort(net.JoinHostPort(host, port)); err != nil {
		return origin{}, ErrInvalidURL
	}
	return origin{scheme: scheme, host: host, port: port}, nil
}

func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func effectivePort(scheme string) string {
	if scheme == schemeHTTP {
		return "80"
	}
	return "443"
}

func isSpecialUseAddress(address netip.Addr) bool {
	for _, prefix := range specialUsePrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

var specialUsePrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:2::/48"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
}
