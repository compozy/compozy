package tailscale

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozysdk "github.com/compozy/compozy/sdk/go"
)

const (
	providerSchemeHTTPS = "https"
	privateListenerPort = "8443"
	publicListenerPort  = "443"
	challengePathPrefix = "/.well-known/compozy/gateway-challenge/"
)

func validateEstablishRequest(
	req compozysdk.ConnectivityEstablishRequest,
) (string, netip.AddrPort, time.Time, error) {
	tier, err := validateTier(req.Tier)
	if err != nil {
		return "", netip.AddrPort{}, time.Time{}, err
	}
	target, err := netip.ParseAddrPort(strings.TrimSpace(req.ForwardTarget))
	if err != nil || !target.Addr().IsLoopback() || target.Port() == 0 {
		return "", netip.AddrPort{}, time.Time{}, errors.New(
			"tailscale: loopback forward target is required",
		)
	}
	path := strings.TrimSpace(req.ChallengePath)
	if !strings.HasPrefix(path, challengePathPrefix) || len(path) == len(challengePathPrefix) {
		return "", netip.AddrPort{}, time.Time{}, errors.New(
			"tailscale: valid challenge path is required",
		)
	}
	if req.Deadline.IsZero() || !req.Deadline.After(time.Now()) {
		return "", netip.AddrPort{}, time.Time{}, errors.New(
			"tailscale: future establish deadline is required",
		)
	}
	return tier, target, req.Deadline, nil
}

func validateTier(value string) (string, error) {
	tier := strings.TrimSpace(value)
	if tier != tierPrivate && tier != tierPublic {
		return "", fmt.Errorf("tailscale: unsupported gateway tier %q", value)
	}
	return tier, nil
}

func selectCertificateDomain(domains []string) (string, error) {
	clean := make([]string, 0, len(domains))
	for _, domain := range domains {
		domain = strings.TrimSuffix(strings.TrimSpace(domain), ".")
		if domain == "" || strings.ContainsAny(domain, "/:@") {
			continue
		}
		clean = append(clean, domain)
	}
	if len(clean) == 0 {
		return "", errors.New("tailscale: tailnet HTTPS certificate domain is unavailable")
	}
	return clean[0], nil
}

func sameCertificateDomain(domains []string, expected string) bool {
	selected, err := selectCertificateDomain(domains)
	return err == nil && selected == expected
}

func listenForTier(
	ctx context.Context,
	node tailscaleNode,
	tier string,
	domain string,
) (net.Listener, compozysdk.ConnectivityAdvertisedEndpoint, error) {
	var (
		listener net.Listener
		rawURL   string
		err      error
	)
	switch tier {
	case tierPrivate:
		listener, err = node.ListenPrivate(ctx)
		rawURL = (&url.URL{Scheme: providerSchemeHTTPS, Host: net.JoinHostPort(domain, privateListenerPort)}).String()
	case tierPublic:
		if err := node.ProvisionCertificate(ctx, domain); err != nil {
			return nil, compozysdk.ConnectivityAdvertisedEndpoint{}, err
		}
		listener, err = node.ListenPublic(ctx)
		rawURL = (&url.URL{Scheme: providerSchemeHTTPS, Host: domain}).String()
	default:
		return nil, compozysdk.ConnectivityAdvertisedEndpoint{}, errors.New(
			"tailscale: valid tier is required",
		)
	}
	if err != nil {
		return nil, compozysdk.ConnectivityAdvertisedEndpoint{}, err
	}
	return listener, compozysdk.ConnectivityAdvertisedEndpoint{
		URL: rawURL, Scheme: providerSchemeHTTPS, Stability: providerEndpointStable,
	}, nil
}

func closeListener(listener net.Listener) error {
	if listener == nil {
		return nil
	}
	if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("tailscale: close listener: %w", err)
	}
	return nil
}

func defaultStateDirectory() (string, error) {
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return "", fmt.Errorf("tailscale: resolve CompozyOS home: %w", err)
	}
	return filepath.Join(homePaths.GatewayDir, "tailscale"), nil
}
