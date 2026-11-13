package config

import (
	"fmt"
	"time"
)

const (
	// DefaultClientRequestTimeout bounds one unary CLI request to the daemon.
	DefaultClientRequestTimeout = 30 * time.Second
	// MinClientRequestTimeout keeps the bound meaningful.
	MinClientRequestTimeout = 5 * time.Second
	// MaxClientRequestTimeout bounds how long one command may wait for admission.
	MaxClientRequestTimeout = 30 * time.Minute
)

// ClientConfig controls CLI-side transport behavior. The daemon reads it too so a
// profile can pin the same budget for every client on the machine.
type ClientConfig struct {
	RequestTimeout time.Duration `toml:"request_timeout"`
}

type clientOverlay struct {
	RequestTimeout *time.Duration `toml:"request_timeout"`
}

// DefaultClientConfig returns the CLI transport defaults.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{RequestTimeout: DefaultClientRequestTimeout}
}

// Validate reports whether the client settings are usable.
func (c ClientConfig) Validate() error {
	if c.RequestTimeout < MinClientRequestTimeout || c.RequestTimeout > MaxClientRequestTimeout {
		return fmt.Errorf(
			"client.request_timeout must be between %s and %s, got %s",
			MinClientRequestTimeout,
			MaxClientRequestTimeout,
			c.RequestTimeout,
		)
	}
	return nil
}

func (o clientOverlay) Apply(dst *ClientConfig) {
	if o.RequestTimeout != nil {
		dst.RequestTimeout = *o.RequestTimeout
	}
}
