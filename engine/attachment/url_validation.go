package attachment

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/url"

	appconfig "github.com/compozy/compozy/pkg/config"
)

// validateHTTPURL parses, normalizes, and validates that s is an absolute HTTP(S) URL
// and performs SSRF safeguards against loopback, link-local, and private ranges.
// During unit tests, local addresses are allowed unless ATTACHMENTS_SSRF_STRICT=1.
func validateHTTPURL(ctx context.Context, s string) (string, error) {
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
	if ssrfStrict(ctx) || !allowLocalForTests() {
		host := u.Hostname()
		if host == "" {
			return "", fmt.Errorf("invalid URL: missing host")
		}
		if ip := net.ParseIP(host); ip != nil {
			if isBlockedIP(ip) {
				return "", fmt.Errorf("blocked destination IP: %s", ip.String())
			}
		} else if addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host); err == nil {
			for _, a := range addrs {
				if isBlockedIP(a.IP) {
					return "", fmt.Errorf("blocked destination IP: %s", a.IP.String())
				}
			}
		} else if ssrfStrict(ctx) {
			return "", fmt.Errorf("DNS resolution failed: %w", err)
		}
	}
	return u.String(), nil
}

func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
		return true
	}
	return false
}

// allowLocalForTests returns true when running `go test` to avoid blocking
// httptest servers bound to loopback. Can be overridden by ATTACHMENTS_SSRF_STRICT.
//
// Examples:
// - Allow loopback during tests (default): `go test ./...`
// - Enforce strict blocking even in tests: `ATTACHMENTS_SSRF_STRICT=1 go test ./...`
// - At runtime (non-tests), strictness is controlled by config/env: ATTACHMENTS_SSRF_STRICT.
func allowLocalForTests() bool { return flag.Lookup("test.v") != nil }

func ssrfStrict(ctx ...context.Context) bool {
	if len(ctx) > 0 && ctx[0] != nil {
		if cfg := appconfig.FromContext(ctx[0]); cfg != nil {
			return cfg.Attachments.SSRFStrict
		}
	}
	return false
}
