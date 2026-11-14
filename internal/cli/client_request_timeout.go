package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	// ClientRequestTimeoutEnv overrides the unary request budget without persisting it.
	ClientRequestTimeoutEnv = "COMPOZY_CLIENT_TIMEOUT"
)

// resolveClientRequestTimeout resolves the unary request budget of the daemon
// client. The environment override wins over the config key; both are clamped so
// a typo cannot remove the bound or hang a command forever. Streams keep their
// untimed client.
func resolveClientRequestTimeout(
	getenv func(string) string,
	loadConfig func() (compozyconfig.Config, error),
) (time.Duration, error) {
	// The environment outranks the config key, so a rejected config value only
	// applies when no override was supplied.
	override, overridden, err := clientRequestTimeoutFromEnvironment(getenv)
	if err != nil {
		return 0, err
	}
	if overridden {
		return clampClientRequestTimeout(override), nil
	}

	timeout := defaultUnixSocketClientTimeout
	if loadConfig != nil {
		cfg, err := loadConfig()
		switch {
		case err == nil:
			if cfg.Client.RequestTimeout > 0 {
				timeout = cfg.Client.RequestTimeout
			}
		case isClientRequestTimeoutConfigError(err):
			// An out-of-range setting is an operator mistake, not a reason to run
			// with a budget they never asked for.
			return 0, err
		}
	}
	return clampClientRequestTimeout(timeout), nil
}

func clientRequestTimeoutFromEnvironment(getenv func(string) string) (time.Duration, bool, error) {
	if getenv == nil {
		return 0, false, nil
	}
	raw := strings.TrimSpace(getenv(ClientRequestTimeoutEnv))
	if raw == "" {
		return 0, false, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, false, fmt.Errorf("cli: parse %s: %w", ClientRequestTimeoutEnv, err)
	}
	if parsed <= 0 {
		return 0, false, fmt.Errorf("cli: %s must be positive, got %q", ClientRequestTimeoutEnv, raw)
	}
	return parsed, true, nil
}

func clampClientRequestTimeout(timeout time.Duration) time.Duration {
	return min(max(timeout, compozyconfig.MinClientRequestTimeout), compozyconfig.MaxClientRequestTimeout)
}

func isClientRequestTimeoutConfigError(err error) bool {
	_, ok := errors.AsType[*compozyconfig.ClientRequestTimeoutError](err)
	return ok
}
