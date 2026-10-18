package config

import "errors"

const (
	// DefaultSessionDeriveMaxReplayBytes bounds the carried context of a continued or forked session.
	DefaultSessionDeriveMaxReplayBytes = 131072
	// DefaultSessionDeriveMaxMessageBytes caps one carried message body.
	DefaultSessionDeriveMaxMessageBytes = 16384

	minSessionDeriveMaxReplayBytes  = 4096
	minSessionDeriveMaxMessageBytes = 1024
)

var (
	errSessionDeriveMaxReplayBytes = errors.New(
		"session.derive.max_replay_bytes must be at least 4096",
	)
	errSessionDeriveMaxMessageBytes = errors.New(
		"session.derive.max_message_bytes must be between 1024 and session.derive.max_replay_bytes",
	)
)

// SessionDeriveConfig bounds the context carried into a continued or forked session.
type SessionDeriveConfig struct {
	// MaxReplayBytes is the hard upper bound of the serialized carried transcript.
	MaxReplayBytes int `toml:"max_replay_bytes"`
	// MaxMessageBytes caps each carried message body.
	MaxMessageBytes int `toml:"max_message_bytes"`
}

type sessionDeriveOverlay struct {
	MaxReplayBytes  *int `toml:"max_replay_bytes"`
	MaxMessageBytes *int `toml:"max_message_bytes"`
}

// DefaultSessionDeriveConfig returns the default carried-context bounds.
func DefaultSessionDeriveConfig() SessionDeriveConfig {
	return SessionDeriveConfig{
		MaxReplayBytes:  DefaultSessionDeriveMaxReplayBytes,
		MaxMessageBytes: DefaultSessionDeriveMaxMessageBytes,
	}
}

// Validate ensures the carried-context bounds are usable.
func (c SessionDeriveConfig) Validate() error {
	if c.MaxReplayBytes < minSessionDeriveMaxReplayBytes {
		return errSessionDeriveMaxReplayBytes
	}
	if c.MaxMessageBytes < minSessionDeriveMaxMessageBytes || c.MaxMessageBytes > c.MaxReplayBytes {
		return errSessionDeriveMaxMessageBytes
	}
	return nil
}

func (o sessionDeriveOverlay) Apply(dst *SessionDeriveConfig) {
	if o.MaxReplayBytes != nil {
		dst.MaxReplayBytes = *o.MaxReplayBytes
	}
	if o.MaxMessageBytes != nil {
		dst.MaxMessageBytes = *o.MaxMessageBytes
	}
}
