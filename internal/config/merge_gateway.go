package config

import "time"

type gatewayOverlay struct {
	Enabled      *bool                      `toml:"enabled"`
	PrivatePort  *int                       `toml:"private_port"`
	PublicPort   *int                       `toml:"public_port"`
	Pairing      gatewayPairingOverlay      `toml:"pairing"`
	StreamTicket gatewayStreamTicketOverlay `toml:"stream_ticket"`
	Auth         gatewayAuthOverlay         `toml:"auth"`
	Verify       gatewayVerifyOverlay       `toml:"verify"`
}

type gatewayPairingOverlay struct {
	TTL        *time.Duration `toml:"ttl"`
	MaxPending *int           `toml:"max_pending"`
}

type gatewayStreamTicketOverlay struct {
	TTL *time.Duration `toml:"ttl"`
}

type gatewayAuthOverlay struct {
	RateLimit gatewayAuthRateLimitOverlay `toml:"rate_limit"`
}

type gatewayAuthRateLimitOverlay struct {
	Window   *time.Duration `toml:"window"`
	MaxFails *int           `toml:"max_fails"`
}

type gatewayVerifyOverlay struct {
	Timeout *time.Duration `toml:"timeout"`
}

func (o gatewayOverlay) Apply(dst *GatewayConfig) {
	applyOptional(o.Enabled, &dst.Enabled)
	applyOptional(o.PrivatePort, &dst.PrivatePort)
	applyOptional(o.PublicPort, &dst.PublicPort)
	applyOptional(o.Pairing.TTL, &dst.Pairing.TTL)
	applyOptional(o.Pairing.MaxPending, &dst.Pairing.MaxPending)
	applyOptional(o.StreamTicket.TTL, &dst.StreamTicket.TTL)
	applyOptional(o.Auth.RateLimit.Window, &dst.Auth.RateLimit.Window)
	applyOptional(o.Auth.RateLimit.MaxFails, &dst.Auth.RateLimit.MaxFails)
	applyOptional(o.Verify.Timeout, &dst.Verify.Timeout)
}
