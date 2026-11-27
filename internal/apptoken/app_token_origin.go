package apptoken

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

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

func NormalizeOrigin(raw string) (string, error) {
	parsed, err := parseAppOriginURL(raw)
	if err != nil {
		return "", err
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != schemeHTTP && scheme != schemeHTTPS {
		return "", ErrOriginInvalid
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || strings.ContainsAny(host, "%/") {
		return "", ErrOriginInvalid
	}
	if strings.Contains(host, ":") || strings.HasPrefix(parsed.Host, "[") {
		addr, err := netip.ParseAddr(host)
		if err != nil || !addr.Is6() || !strings.HasPrefix(parsed.Host, "[") {
			return "", ErrOriginInvalid
		}
	}
	if strings.HasSuffix(parsed.Host, ":") {
		return "", ErrOriginInvalid
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
		return nil, ErrOriginInvalid
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Opaque != "" || parsed.User != nil || parsed.Host == "" ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery ||
		parsed.Fragment != "" || strings.ContainsAny(raw, "*?#\\") {
		return nil, ErrOriginInvalid
	}
	return parsed, nil
}

func normalizeAppOriginPort(scheme, port string) (string, error) {
	if port == "" {
		return "", nil
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", ErrOriginInvalid
	}
	if scheme == schemeHTTP && number == 80 || scheme == schemeHTTPS && number == 443 {
		return "", nil
	}
	return strconv.Itoa(number), nil
}

func normalizeAppTokenOrigins(origins []string) ([]string, error) {
	normalized := make([]string, 0, len(origins))
	for _, origin := range origins {
		value, err := NormalizeOrigin(origin)
		if err != nil {
			return nil, fmt.Errorf("origin %q is invalid: %w", origin, err)
		}
		normalized = append(normalized, value)
	}
	slices.Sort(normalized)
	normalized = slices.Compact(normalized)
	if len(normalized) > OriginLimit {
		return nil, ErrOriginLimit
	}
	return normalized, nil
}

func normalizeAppTokenName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) == 0 ||
		utf8.RuneCountInString(name) > nameLimit {
		return "", ErrNameInvalid
	}
	return name, nil
}

func HasPrefix(secret string) bool {
	return strings.HasPrefix(secret, appTokenPrefix)
}
