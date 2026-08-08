package config

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	defaultGatewayPairingTTL      = 5 * time.Minute
	defaultGatewayPairingPending  = 8
	defaultGatewayStreamTicketTTL = 30 * time.Second
	defaultGatewayAuthWindow      = 60 * time.Second
	defaultGatewayAuthMaxFails    = 10
	defaultGatewayVerifyTimeout   = 10 * time.Second
	maximumGatewayListenerPort    = 65535
	positiveGatewayValueMessage   = "must be positive"
	gatewayConnectionSchemeSSH    = "ssh"
)

// GatewayConfig controls the local gateway ceiling and runtime tunables.
// Durable exposure intent lives in the global database, never in this config.
type GatewayConfig struct {
	Enabled          bool                      `toml:"enabled"`
	PrivatePort      int                       `toml:"private_port"`
	PublicPort       int                       `toml:"public_port"`
	ActiveConnection string                    `toml:"active_connection,omitempty"`
	Connections      []GatewayConnectionConfig `toml:"connections,omitempty"`
	Pairing          GatewayPairingConfig      `toml:"pairing"`
	StreamTicket     GatewayStreamTicketConfig `toml:"stream_ticket"`
	Auth             GatewayAuthConfig         `toml:"auth"`
	Verify           GatewayVerifyConfig       `toml:"verify"`

	CredentialsDir string `toml:"-" json:"-"`
}

// GatewayConnectionConfig is non-secret metadata for one client-side connection profile.
type GatewayConnectionConfig struct {
	Name             string `toml:"name"`
	Scheme           string `toml:"scheme"`
	Host             string `toml:"host"`
	Port             int    `toml:"port"`
	CredentialFile   string `toml:"credential_file,omitempty"`
	DefaultWorkspace string `toml:"default_workspace,omitempty"`
	RemoteHome       string `toml:"remote_home,omitempty"`
}

// GatewayPairingConfig bounds pending, short-lived pairing artifacts.
type GatewayPairingConfig struct {
	TTL        time.Duration `toml:"ttl"`
	MaxPending int           `toml:"max_pending"`
}

// GatewayStreamTicketConfig controls single-use stream ticket lifetime.
type GatewayStreamTicketConfig struct {
	TTL time.Duration `toml:"ttl"`
}

// GatewayAuthConfig groups device authentication controls.
type GatewayAuthConfig struct {
	RateLimit GatewayAuthRateLimitConfig `toml:"rate_limit"`
}

// GatewayAuthRateLimitConfig bounds failed authentication attempts by source.
type GatewayAuthRateLimitConfig struct {
	Window   time.Duration `toml:"window"`
	MaxFails int           `toml:"max_fails"`
}

// GatewayVerifyConfig bounds provider endpoint verification.
type GatewayVerifyConfig struct {
	Timeout time.Duration `toml:"timeout"`
}

func defaultGatewayConfig(homePaths HomePaths) GatewayConfig {
	return GatewayConfig{
		Enabled:        false,
		PrivatePort:    0,
		PublicPort:     0,
		CredentialsDir: homePaths.GatewayCredentialsDir,
		Pairing: GatewayPairingConfig{
			TTL:        defaultGatewayPairingTTL,
			MaxPending: defaultGatewayPairingPending,
		},
		StreamTicket: GatewayStreamTicketConfig{TTL: defaultGatewayStreamTicketTTL},
		Auth: GatewayAuthConfig{RateLimit: GatewayAuthRateLimitConfig{
			Window:   defaultGatewayAuthWindow,
			MaxFails: defaultGatewayAuthMaxFails,
		}},
		Verify: GatewayVerifyConfig{Timeout: defaultGatewayVerifyTimeout},
	}
}

// Validate ensures every gateway tunable is safe and bounded.
func (c GatewayConfig) Validate() error {
	if err := validateGatewayPort("gateway.private_port", c.PrivatePort); err != nil {
		return err
	}
	if err := validateGatewayPort("gateway.public_port", c.PublicPort); err != nil {
		return err
	}
	if c.Pairing.TTL <= 0 {
		return ValidationError{Path: "gateway.pairing.ttl", Message: positiveGatewayValueMessage}
	}
	if c.Pairing.MaxPending <= 0 {
		return ValidationError{
			Path:    "gateway.pairing.max_pending",
			Message: positiveGatewayValueMessage,
		}
	}
	if c.StreamTicket.TTL <= 0 {
		return ValidationError{
			Path:    "gateway.stream_ticket.ttl",
			Message: positiveGatewayValueMessage,
		}
	}
	if c.Auth.RateLimit.Window <= 0 {
		return ValidationError{
			Path:    "gateway.auth.rate_limit.window",
			Message: positiveGatewayValueMessage,
		}
	}
	if c.Auth.RateLimit.MaxFails <= 0 {
		return ValidationError{
			Path:    "gateway.auth.rate_limit.max_fails",
			Message: positiveGatewayValueMessage,
		}
	}
	if c.Verify.Timeout <= 0 {
		return ValidationError{Path: "gateway.verify.timeout", Message: positiveGatewayValueMessage}
	}
	connections := make(map[string]struct{}, len(c.Connections))
	for index := range c.Connections {
		connection := c.Connections[index]
		if err := connection.Validate(); err != nil {
			return fmt.Errorf("gateway.connections[%d]: %w", index, err)
		}
		if _, exists := connections[connection.Name]; exists {
			return ValidationError{
				Path:    "gateway.connections",
				Message: fmt.Sprintf("duplicate name %q", connection.Name),
			}
		}
		connections[connection.Name] = struct{}{}
	}
	if active := strings.TrimSpace(c.ActiveConnection); active != "" {
		if _, exists := connections[active]; !exists {
			return ValidationError{
				Path:    "gateway.active_connection",
				Message: fmt.Sprintf("unknown connection %q", active),
			}
		}
	}
	return nil
}

// Validate ensures a connection profile is safe to resolve from operator-global config.
func (c GatewayConnectionConfig) Validate() error {
	name := strings.TrimSpace(c.Name)
	if name != c.Name || !ValidGatewayConnectionName(name) {
		return errors.New("name must contain only letters, digits, hyphens, or underscores")
	}
	scheme := strings.TrimSpace(c.Scheme)
	if scheme != c.Scheme || scheme != urlSchemeHTTPS && scheme != gatewayConnectionSchemeSSH {
		return errors.New("scheme must be https or ssh")
	}
	host := strings.TrimSpace(c.Host)
	normalizedHost := strings.Trim(host, "[]")
	if normalizedHost == "" || strings.ContainsAny(host, "/?#@\r\n\t") {
		return errors.New("host must be a hostname or IP address")
	}
	if parsed := net.ParseIP(normalizedHost); parsed == nil &&
		strings.Contains(host, ":") {
		return errors.New("IPv6 hosts must be valid addresses")
	}
	if c.Port <= 0 || c.Port > maximumGatewayListenerPort {
		return errors.New("port must be between 1 and 65535")
	}
	if err := validateGatewayConnectionSchemeFields(c, name, scheme); err != nil {
		return err
	}
	if strings.ContainsAny(c.RemoteHome, "\x00\r\n") {
		return errors.New("remote_home contains control characters")
	}
	return nil
}

func validateGatewayConnectionSchemeFields(
	connection GatewayConnectionConfig,
	name string,
	scheme string,
) error {
	credentialFile := strings.TrimSpace(connection.CredentialFile)
	if scheme == urlSchemeHTTPS && credentialFile != name+".cred" {
		return errors.New("HTTPS profile credential_file must match <name>.cred")
	}
	if scheme == urlSchemeHTTPS && strings.TrimSpace(connection.RemoteHome) != "" {
		return errors.New("HTTPS profiles cannot set remote_home")
	}
	if scheme == gatewayConnectionSchemeSSH && credentialFile != "" {
		return errors.New("SSH profiles cannot reference a gateway credential")
	}
	if scheme == gatewayConnectionSchemeSSH && strings.TrimSpace(connection.DefaultWorkspace) != "" {
		return errors.New("SSH profiles cannot set default_workspace")
	}
	return nil
}

// ValidGatewayConnectionName reports whether name is safe for config, paths, and keyring identities.
func ValidGatewayConnectionName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, character := range name {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validateGatewayPort(path string, port int) error {
	if port < 0 || port > maximumGatewayListenerPort {
		return ValidationError{Path: path, Message: "must be between 0 and 65535"}
	}
	return nil
}
