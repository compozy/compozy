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

// ClientConfig controls CLI-side transport behavior. It is machine-scoped: a
// profile or workspace layer cannot pin it, because the CLI resolves it before a
// profile is selected for the command.
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

// ClientRequestTimeoutError reports an out-of-range client.request_timeout, so a
// caller can tell an invalid setting from an unrelated configuration failure.
type ClientRequestTimeoutError struct {
	Value time.Duration
}

func (e *ClientRequestTimeoutError) Error() string {
	return fmt.Sprintf(
		"client.request_timeout must be between %s and %s, got %s",
		MinClientRequestTimeout,
		MaxClientRequestTimeout,
		e.Value,
	)
}

// Validate reports whether the client settings are usable.
func (c ClientConfig) Validate() error {
	if c.RequestTimeout < MinClientRequestTimeout || c.RequestTimeout > MaxClientRequestTimeout {
		return &ClientRequestTimeoutError{Value: c.RequestTimeout}
	}
	return nil
}

func (o clientOverlay) Apply(dst *ClientConfig) {
	if o.RequestTimeout != nil {
		dst.RequestTimeout = *o.RequestTimeout
	}
}
