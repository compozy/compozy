package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/diagnosticcontract"
)

// ErrProviderDisabled reports that the operator disabled the provider with providers.<id>.enabled = false.
var ErrProviderDisabled = errors.New("provider disabled")

// ErrProviderDefaultDisableRejected reports an attempt to disable the effective default provider.
var ErrProviderDefaultDisableRejected = errors.New("cannot disable the default provider")

// ProviderDisabledError names the disabled provider and how to re-enable it.
type ProviderDisabledError struct {
	Provider string
}

func (e *ProviderDisabledError) Error() string {
	return fmt.Sprintf(
		"provider %q is disabled; enable it in Settings → Providers or run `compozy config set providers.%s.enabled true`",
		e.Provider,
		e.Provider,
	)
}

// Is matches ErrProviderDisabled.
func (e *ProviderDisabledError) Is(target error) bool { return target == ErrProviderDisabled }

// DiagnosticCode returns the stable provider_disabled code.
func (e *ProviderDisabledError) DiagnosticCode() string {
	return diagnosticcontract.CodeProviderDisabled
}

// ProviderDefaultDisableError rejects disabling the effective default provider (BR-3).
type ProviderDefaultDisableError struct {
	Provider string
	// PendingDefault marks the projected-config case: the active default still names Provider
	// while a defaults.provider change waits for a daemon restart.
	PendingDefault bool
}

func (e *ProviderDefaultDisableError) Error() string {
	if e.PendingDefault {
		return fmt.Sprintf(
			"cannot disable the active default provider %q yet; the defaults.provider change is pending a daemon restart",
			e.Provider,
		)
	}
	return fmt.Sprintf(
		"providers.%s.enabled: cannot disable the default provider %q; set defaults.provider to another provider first",
		e.Provider,
		e.Provider,
	)
}

// Is matches ErrProviderDefaultDisableRejected.
func (e *ProviderDefaultDisableError) Is(target error) bool {
	return target == ErrProviderDefaultDisableRejected
}

// DiagnosticCode returns the stable provider_default_disable_rejected code.
func (e *ProviderDefaultDisableError) DiagnosticCode() string {
	return diagnosticcontract.CodeProviderDefaultDisableRejected
}

// ProviderEnabled resolves name and reports whether it is enabled. Unknown names report false, nil error.
// Enabled state is per provider ID and never inherited through runtime_provider (BR-7).
func (c *Config) ProviderEnabled(name string) (bool, error) {
	providerName := CanonicalProviderName(name)
	if providerName == "" {
		return false, errors.New("provider name is required")
	}
	if _, builtin := builtinProviders[providerName]; !builtin {
		if c == nil {
			return false, nil
		}
		if _, configured := c.Providers[providerName]; !configured {
			return false, nil
		}
	}
	resolved, err := c.ResolveProvider(providerName)
	if err != nil {
		return false, err
	}
	return resolved.EffectiveEnabled(), nil
}

// RequireEnabledProvider returns a *ProviderDisabledError (matching ErrProviderDisabled) when the
// provider is disabled. Unknown or unresolvable providers pass through so callers keep their
// existing resolution errors.
func (c *Config) RequireEnabledProvider(name string) error {
	providerName := CanonicalProviderName(name)
	if providerName == "" || !c.providerExplicitlyDisabled(providerName) {
		return nil
	}
	return &ProviderDisabledError{Provider: providerName}
}

// providerExplicitlyDisabled reads the authored enabled flag without resolving the provider, so
// disabled checks never turn into resolution errors.
func (c *Config) providerExplicitlyDisabled(providerName string) bool {
	if c == nil {
		return false
	}
	override, ok := c.Providers[providerName]
	return ok && !override.EffectiveEnabled()
}

// IsProviderDisabled reports whether name is a provider the operator disabled.
func (c *Config) IsProviderDisabled(name string) bool {
	return c.providerExplicitlyDisabled(CanonicalProviderName(name))
}

func (c *Config) validateDefaultProviderEnabled() error {
	provider := CanonicalProviderName(strings.TrimSpace(c.Defaults.Provider))
	if provider == "" || !c.providerExplicitlyDisabled(provider) {
		return nil
	}
	return &ProviderDefaultDisableError{Provider: provider}
}
