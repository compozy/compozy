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
	if getenv != nil {
		raw := strings.TrimSpace(getenv(ClientRequestTimeoutEnv))
		if raw != "" {
			parsed, err := time.ParseDuration(raw)
			if err != nil {
				return 0, fmt.Errorf("cli: parse %s: %w", ClientRequestTimeoutEnv, err)
			}
			if parsed <= 0 {
				return 0, fmt.Errorf("cli: %s must be positive, got %q", ClientRequestTimeoutEnv, raw)
			}
			timeout = parsed
		}
	}
	return min(max(timeout, compozyconfig.MinClientRequestTimeout), compozyconfig.MaxClientRequestTimeout), nil
}

func isClientRequestTimeoutConfigError(err error) bool {
	_, ok := errors.AsType[*compozyconfig.ClientRequestTimeoutError](err)
	return ok
}
