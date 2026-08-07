package config

import "time"

const (
	defaultGatewayPairingTTL      = 5 * time.Minute
	defaultGatewayPairingPending  = 8
	defaultGatewayStreamTicketTTL = 30 * time.Second
	defaultGatewayAuthWindow      = 60 * time.Second
	defaultGatewayAuthMaxFails    = 10
	defaultGatewayVerifyTimeout   = 10 * time.Second
	maximumGatewayListenerPort    = 65535
	positiveGatewayValueMessage   = "must be positive"
)

// GatewayConfig controls the local gateway ceiling and runtime tunables.
// Durable exposure intent lives in the global database, never in this config.
type GatewayConfig struct {
	Enabled      bool                      `toml:"enabled"`
	PrivatePort  int                       `toml:"private_port"`
	PublicPort   int                       `toml:"public_port"`
	Pairing      GatewayPairingConfig      `toml:"pairing"`
	StreamTicket GatewayStreamTicketConfig `toml:"stream_ticket"`
	Auth         GatewayAuthConfig         `toml:"auth"`
	Verify       GatewayVerifyConfig       `toml:"verify"`

	CredentialsDir string `toml:"-" json:"-"`
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
		return ValidationError{Path: "gateway.pairing.max_pending", Message: positiveGatewayValueMessage}
	}
	if c.StreamTicket.TTL <= 0 {
		return ValidationError{Path: "gateway.stream_ticket.ttl", Message: positiveGatewayValueMessage}
	}
	if c.Auth.RateLimit.Window <= 0 {
		return ValidationError{Path: "gateway.auth.rate_limit.window", Message: positiveGatewayValueMessage}
	}
	if c.Auth.RateLimit.MaxFails <= 0 {
		return ValidationError{Path: "gateway.auth.rate_limit.max_fails", Message: positiveGatewayValueMessage}
	}
	if c.Verify.Timeout <= 0 {
		return ValidationError{Path: "gateway.verify.timeout", Message: positiveGatewayValueMessage}
	}
	return nil
}

func validateGatewayPort(path string, port int) error {
	if port < 0 || port > maximumGatewayListenerPort {
		return ValidationError{Path: path, Message: "must be between 0 and 65535"}
	}
	return nil
}
