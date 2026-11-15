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
// client. The environment override wins over the config key; both are bounded so
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
		if override < compozyconfig.MinClientRequestTimeout || override > compozyconfig.MaxClientRequestTimeout {
			// Both sources reject an out-of-range value: silently clamping would change
			// what the operator asked for.
			return 0, fmt.Errorf(
				"cli: %s must be between %s and %s, got %s",
				ClientRequestTimeoutEnv,
				compozyconfig.MinClientRequestTimeout,
				compozyconfig.MaxClientRequestTimeout,
				override,
			)
		}
		return override, nil
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
	return timeout, nil
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

func isClientRequestTimeoutConfigError(err error) bool {
	_, ok := errors.AsType[*compozyconfig.ClientRequestTimeoutError](err)
	return ok
}
