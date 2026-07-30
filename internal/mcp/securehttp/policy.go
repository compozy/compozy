package securehttp

import (
	"net/netip"
	"net/url"
	"strings"
)

type policy struct {
	allowLoopback bool
}

func (p policy) validateURL(destination *url.URL) error {
	if destination == nil || destination.User != nil || destination.Fragment != "" ||
		strings.TrimSpace(destination.Hostname()) == "" {
		return ErrInvalidURL
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(destination.Hostname())), ".")
	if host == "" {
		return ErrInvalidURL
	}
	isExplicitLoopback := isExplicitLoopbackHost(host)
	if !strings.EqualFold(destination.Scheme, "https") {
		if !strings.EqualFold(destination.Scheme, "http") || !p.allowLoopback || !isExplicitLoopback {
			return ErrInsecureTransport
		}
	}
	if isExplicitLoopback && !p.allowLoopback {
		return ErrBlockedDestination
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if err := p.validateAddress(ip, isExplicitLoopback); err != nil {
			return err
		}
	}
	return nil
}

func (p policy) validateAddresses(addresses []netip.Addr, explicitLoopback bool) error {
	if len(addresses) == 0 {
		return ErrBlockedDestination
	}
	for _, address := range addresses {
		if err := p.validateAddress(address, explicitLoopback); err != nil {
			return err
		}
	}
	return nil
}

func (p policy) validateAddress(address netip.Addr, explicitLoopback bool) error {
	address = address.Unmap()
	if !address.IsValid() || address.Zone() != "" {
		return ErrBlockedDestination
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

func isExplicitLoopbackHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	address, err := netip.ParseAddr(host)
	return err == nil && address.IsLoopback()
}

func sameOrigin(left, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) && effectivePort(left) == effectivePort(right)
}

func effectivePort(destination *url.URL) string {
	if port := destination.Port(); port != "" {
		return port
	}
	switch strings.ToLower(destination.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
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
