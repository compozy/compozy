package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

type webhookProviderConfig struct {
	Webhook struct {
		PublicURL  string `json:"public_url"`
		ListenAddr string `json:"listen_addr"`
		Path       string `json:"path"`
	} `json:"webhook"`
}

func decodeWebhookProviderConfig(instance BridgeInstance, missingMessage string) (webhookProviderConfig, error) {
	raw := bytes.TrimSpace(instance.ProviderConfig)
	if len(raw) == 0 {
		return webhookProviderConfig{}, errors.New(missingMessage)
	}
	var config webhookProviderConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return webhookProviderConfig{}, fmt.Errorf("bridges: decode provider config: %w", err)
	}
	return config, nil
}

var nonPublicWebhookPrefixes = []netip.Prefix{
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

// WebhookLocalTarget resolves the provider-owned loopback listener used by the gateway proxy.
func WebhookLocalTarget(instance BridgeInstance) (*url.URL, error) {
	config, err := decodeWebhookProviderConfig(instance, "bridges: webhook local target is required")
	if err != nil {
		return nil, err
	}
	listenAddr := strings.TrimSpace(config.Webhook.ListenAddr)
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return nil, fmt.Errorf("bridges: parse webhook listen_addr: %w", err)
	}
	address, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil || !address.Unmap().IsLoopback() {
		return nil, errors.New("bridges: webhook listen_addr must use a loopback IP")
	}
	if strings.TrimSpace(port) == "" || port == "0" {
		return nil, errors.New("bridges: gateway ingress requires a fixed webhook listen port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 || strconv.Itoa(portNumber) != port {
		return nil, errors.New("bridges: valid webhook listen port between 1 and 65535 is required")
	}
	webhookPath := strings.TrimSpace(config.Webhook.Path)
	if webhookPath == "" {
		return nil, errors.New("bridges: webhook path is required")
	}
	if !strings.HasPrefix(webhookPath, "/") {
		webhookPath = "/" + webhookPath
	}
	return &url.URL{
		Scheme: "http", Host: net.JoinHostPort(address.String(), strconv.Itoa(portNumber)), Path: webhookPath,
	}, nil
}

// WebhookPublicURL returns the configured external callback URL.
func WebhookPublicURL(instance BridgeInstance) (string, error) {
	config, err := decodeWebhookProviderConfig(instance, "bridges: webhook public_url is required")
	if err != nil {
		return "", err
	}
	publicURL := strings.TrimSpace(config.Webhook.PublicURL)
	if publicURL == "" {
		return "", errors.New("bridges: webhook public_url is required")
	}
	if err := ValidateWebhookPublicURL(publicURL); err != nil {
		return "", err
	}
	parsed, err := url.Parse(publicURL)
	if err != nil {
		return "", fmt.Errorf("bridges: parse webhook public_url: %w", err)
	}
	return parsed.String(), nil
}

// ValidateWebhookPublicURL enforces the public callback URL boundary.
func ValidateWebhookPublicURL(publicURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(publicURL))
	if err != nil {
		return fmt.Errorf("bridges: parse webhook public_url: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return errors.New("bridges: webhook public_url must use HTTPS")
	}
	if strings.TrimSpace(parsed.Host) == "" || strings.TrimSpace(parsed.Hostname()) == "" {
		return errors.New("bridges: webhook public_url must include a host")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return errors.New("bridges: webhook public_url must not contain userinfo or fragment")
	}
	host := strings.TrimSpace(strings.ToLower(parsed.Hostname()))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("bridges: webhook public_url host must be publicly routable")
	}
	if ip, parseErr := netip.ParseAddr(host); parseErr == nil {
		if err := ValidateWebhookDestinationIP(ip); err != nil {
			return err
		}
	}
	return nil
}

// ValidateWebhookDestinationIP rejects non-public callback destinations.
func ValidateWebhookDestinationIP(ip netip.Addr) error {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.Zone() != "" || !ip.IsGlobalUnicast() || ip.IsUnspecified() ||
		ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || isSpecialUseWebhookAddress(ip) {
		return errors.New("bridges: webhook destination IP must be publicly routable")
	}
	return nil
}

func isSpecialUseWebhookAddress(ip netip.Addr) bool {
	for _, prefix := range nonPublicWebhookPrefixes {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}
