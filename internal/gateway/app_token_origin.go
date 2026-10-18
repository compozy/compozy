package gateway

import (
	"fmt"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func NormalizeAppOrigin(raw string) (string, error) {
	parsed, err := parseAppOriginURL(raw)
	if err != nil {
		return "", err
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != endpointSchemeHTTP && scheme != endpointSchemeHTTPS {
		return "", ErrAppTokenOriginInvalid
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || strings.ContainsAny(host, "%/") {
		return "", ErrAppTokenOriginInvalid
	}
	if strings.Contains(host, ":") || strings.HasPrefix(parsed.Host, "[") {
		addr, err := netip.ParseAddr(host)
		if err != nil || !addr.Is6() || !strings.HasPrefix(parsed.Host, "[") {
			return "", ErrAppTokenOriginInvalid
		}
	}
	if strings.HasSuffix(parsed.Host, ":") {
		return "", ErrAppTokenOriginInvalid
	}
	port, err := normalizeAppOriginPort(scheme, parsed.Port())
	if err != nil {
		return "", err
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		host += ":" + port
	}
	return scheme + "://" + host, nil
}

func parseAppOriginURL(raw string) (*url.URL, error) {
	if len(raw) < 8 || len(raw) > 2048 || strings.ContainsFunc(raw, unicode.IsSpace) {
		return nil, ErrAppTokenOriginInvalid
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Opaque != "" || parsed.User != nil || parsed.Host == "" ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery ||
		parsed.Fragment != "" || strings.ContainsAny(raw, "*?#\\") {
		return nil, ErrAppTokenOriginInvalid
	}
	return parsed, nil
}

func normalizeAppOriginPort(scheme, port string) (string, error) {
	if port == "" {
		return "", nil
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", ErrAppTokenOriginInvalid
	}
	if scheme == endpointSchemeHTTP && number == 80 || scheme == endpointSchemeHTTPS && number == 443 {
		return "", nil
	}
	return strconv.Itoa(number), nil
}

func normalizeAppTokenOrigins(origins []string) ([]string, error) {
	normalized := make([]string, 0, len(origins))
	for _, origin := range origins {
		value, err := NormalizeAppOrigin(origin)
		if err != nil {
			return nil, fmt.Errorf("origin %q is invalid: %w", origin, err)
		}
		normalized = append(normalized, value)
	}
	slices.Sort(normalized)
	normalized = slices.Compact(normalized)
	if len(normalized) > AppTokenOriginLimit {
		return nil, ErrAppTokenOriginLimit
	}
	return normalized, nil
}

func normalizeAppTokenName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) == 0 ||
		utf8.RuneCountInString(name) > AppTokenNameLimit {
		return "", ErrAppTokenNameInvalid
	}
	return name, nil
}

func HasAppTokenPrefix(secret string) bool {
	return strings.HasPrefix(secret, appTokenPrefix)
}
