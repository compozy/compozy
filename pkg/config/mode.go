package config

import (
	"context"
	"strings"
)

// App operation modes
const (
	ModeStandalone  = "standalone"
	ModeDistributed = "distributed"
)

// Context key for current app mode
const ModeCtxKey ContextKey = "mode"

// NormalizeMode trims spaces and lowercases the provided mode string
func NormalizeMode(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// WithMode stores the normalized app mode in context
func WithMode(ctx context.Context, mode string) context.Context {
	return context.WithValue(ctx, ModeCtxKey, NormalizeMode(mode))
}

// ModeFrom attempts to read mode from context; if not set, falls back to Config in context
func ModeFrom(ctx context.Context) string {
	if ctx != nil {
		if v, ok := ctx.Value(ModeCtxKey).(string); ok && v != "" {
			return NormalizeMode(v)
		}
	}
	cfg := FromContext(ctx)
	if cfg != nil && cfg.Mode != "" {
		return NormalizeMode(cfg.Mode)
	}
	return ""
}

// IsStandalone returns true if current mode is standalone
func IsStandalone(ctx context.Context) bool {
	return ModeFrom(ctx) == ModeStandalone
}

// IsDistributed returns true if current mode is distributed
func IsDistributed(ctx context.Context) bool {
	return ModeFrom(ctx) == ModeDistributed
}
