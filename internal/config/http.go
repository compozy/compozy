package config

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// HTTPConfig controls the HTTP server bind address.
type HTTPConfig struct {
	Host              string   `toml:"host"`
	Port              int      `toml:"port"`
	AllowRemoteAccess bool     `toml:"allow_remote_access"`
	AllowedIPs        []string `toml:"allowed_ips,omitempty"`
}

// Validate ensures the HTTP bind settings are valid.
func (c HTTPConfig) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return errors.New("http.host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("http.port must be between 1 and 65535: %d", c.Port)
	}
	for index, value := range c.AllowedIPs {
		if err := validateHTTPAllowedIP(value); err != nil {
			return fmt.Errorf("http.allowed_ips[%d]: %w", index, err)
		}
	}
	return nil
}

func validateHTTPAllowedIP(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New("must not be empty")
	}
	if _, err := netip.ParsePrefix(trimmed); err == nil {
		return nil
	}
	if _, err := netip.ParseAddr(trimmed); err != nil {
		return fmt.Errorf("must be an IP address or CIDR prefix: %q", value)
	}
	return nil
}
