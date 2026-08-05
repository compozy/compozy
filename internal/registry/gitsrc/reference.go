package gitsrc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/outboundpolicy"
)

type repositoryRef struct {
	raw  string
	host string
	port string
}

// ValidateRepositoryRef enforces the public git-source repository grammar and literal-address policy.
func ValidateRepositoryRef(value string) error {
	_, err := parseRepositoryRef(value)
	return err
}

func parseRepositoryRef(value string) (_ repositoryRef, err error) {
	defer func() {
		if err != nil && !errors.Is(err, ErrInvalidRepositoryRef) {
			err = fmt.Errorf("%w: %w", ErrInvalidRepositoryRef, err)
		}
	}()
	repository := strings.TrimSpace(value)
	parsed, err := parseSecureRepositoryURL(repository)
	if err != nil {
		return repositoryRef{}, err
	}
	host, err := normalizedRepositoryHost(parsed)
	if err != nil {
		return repositoryRef{}, err
	}
	port, err := validatedRepositoryPort(parsed.Port())
	if err != nil {
		return repositoryRef{}, err
	}
	parsed.Host = canonicalRepositoryAuthority(host, parsed.Port())
	if err := outboundpolicy.New(false).ValidateURL(parsed); err != nil {
		return repositoryRef{}, fmt.Errorf("gitsrc: repository destination: %w", err)
	}
	return repositoryRef{raw: parsed.String(), host: host, port: port}, nil
}

func parseSecureRepositoryURL(repository string) (*url.URL, error) {
	if repository == "" {
		return nil, errors.New("gitsrc: repository URL is required")
	}
	parsed, err := url.Parse(repository)
	if err != nil {
		return nil, fmt.Errorf("gitsrc: parse repository URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return nil, errors.New("gitsrc: repository URL must use HTTPS")
	}
	if parsed.User != nil {
		return nil, errors.New("gitsrc: repository URL must not include credentials")
	}
	if parsed.Fragment != "" || strings.Contains(repository, "#") ||
		parsed.RawQuery != "" || parsed.ForceQuery || strings.Contains(repository, "?") {
		return nil, errors.New("gitsrc: repository URL must not include a query or fragment")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		return nil, errors.New("gitsrc: repository URL path is required")
	}
	return parsed, nil
}

func normalizedRepositoryHost(parsed *url.URL) (string, error) {
	rawHost := strings.TrimSpace(parsed.Hostname())
	if rawHost == "" {
		return "", errors.New("gitsrc: repository URL hostname is required")
	}
	if strings.HasSuffix(rawHost, ".") || !isASCII(rawHost) {
		return "", errors.New("gitsrc: repository URL hostname must use ASCII without a trailing dot")
	}
	return strings.ToLower(rawHost), nil
}

func validatedRepositoryPort(explicitPort string) (string, error) {
	if explicitPort == "" {
		return "443", nil
	}
	number, err := strconv.ParseUint(explicitPort, 10, 16)
	if err != nil || number == 0 {
		return "", errors.New("gitsrc: repository URL port is invalid")
	}
	return explicitPort, nil
}

func canonicalRepositoryAuthority(host string, explicitPort string) string {
	if explicitPort != "" {
		return net.JoinHostPort(host, explicitPort)
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

func isASCII(value string) bool {
	for index := range len(value) {
		if value[index] > 0x7f {
			return false
		}
	}
	return true
}

func resolveRepositoryAddresses(
	ctx context.Context,
	resolver outboundpolicy.Resolver,
	repository repositoryRef,
) ([]netip.Addr, error) {
	if resolver == nil {
		return nil, errors.New("gitsrc: repository resolver is required")
	}
	var addresses []netip.Addr
	if address, err := netip.ParseAddr(repository.host); err == nil {
		addresses = []netip.Addr{address}
	} else {
		resolved, err := resolver.LookupNetIP(ctx, "ip", repository.host)
		if err != nil {
			return nil, fmt.Errorf("gitsrc: resolve repository destination: %w", err)
		}
		addresses = resolved
	}
	if err := outboundpolicy.New(false).ValidateResolvedAddresses(
		addresses,
		repository.host,
		repository.port,
	); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRepositoryDestinationBlocked, err)
	}
	result := make([]netip.Addr, len(addresses))
	for index, address := range addresses {
		result[index] = address.Unmap()
	}
	return result, nil
}

func curlResolveValue(repository repositoryRef, addresses []netip.Addr) string {
	formatted := make([]string, 0, len(addresses))
	for _, address := range addresses {
		address = address.Unmap()
		value := address.String()
		if address.Is6() {
			value = "[" + value + "]"
		}
		formatted = append(formatted, value)
	}
	return "+" + repository.host + ":" + repository.port + ":" + strings.Join(formatted, ",")
}

func repositoryName(repository string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(repository), "/")
	if colon := strings.LastIndex(trimmed, ":"); colon >= 0 && !strings.Contains(trimmed[colon+1:], "/") {
		trimmed = trimmed[colon+1:]
	}
	name := filepath.Base(trimmed)
	return strings.TrimSuffix(name, ".git")
}
