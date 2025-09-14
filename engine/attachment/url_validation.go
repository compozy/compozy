package attachment

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
)

// validateHTTPURL parses, normalizes, and validates that s is an absolute HTTP(S) URL
// and performs SSRF safeguards against loopback, link-local, and private ranges.
// During unit tests, local addresses are allowed unless COMPOZY_SSRF_STRICT=1.
func validateHTTPURL(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if !u.IsAbs() {
		return "", fmt.Errorf("invalid URL: must be absolute")
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return "", fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid URL: missing host")
	}
	if ssrfStrict() || !allowLocalForTests() {
		host := u.Hostname()
		if host == "" {
			return "", fmt.Errorf("invalid URL: missing host")
		}
		// Evaluate host IPs; block loopback, private, link-local, multicast, unspecified
		if ip := net.ParseIP(host); ip != nil {
			if isBlockedIP(ip) {
				return "", fmt.Errorf("blocked destination IP: %s", ip.String())
			}
		} else {
			if addrs, err := net.DefaultResolver.LookupIPAddr(context.Background(), host); err == nil {
				for _, a := range addrs {
					if isBlockedIP(a.IP) {
						return "", fmt.Errorf("blocked destination IP: %s", a.IP.String())
					}
				}
			}
		}
	}
	return u.String(), nil
}

func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 10 {
			return true
		}
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
	}
	return false
}

// allowLocalForTests returns true when running `go test` to avoid blocking
// httptest servers bound to loopback. Can be overridden by COMPOZY_SSRF_STRICT.
func allowLocalForTests() bool {
	if ssrfStrict() {
		return false
	}
	if flag.Lookup("test.v") != nil {
		return true
	}
	return false
}

func ssrfStrict() bool { return os.Getenv("COMPOZY_SSRF_STRICT") == "1" }
