package config

import (
	"fmt"

	"net/url"

	"strings"
	"time"
)

// EffectiveHarness returns the configured provider harness or the command-backed default.
func (p ProviderConfig) EffectiveHarness() ProviderHarness {
	if p.Harness != "" {
		return p.Harness
	}
	return ProviderHarnessACP
}

// RequiresRuntimeModel reports whether AGH must provide a model to start this provider.
func (p ProviderConfig) RequiresRuntimeModel() bool {
	return p.EffectiveHarness() == ProviderHarnessPiACP
}

// EffectiveAuthMode returns the configured auth owner or the slot-derived default.
func (p ProviderConfig) EffectiveAuthMode() ProviderAuthMode {
	if p.AuthMode != "" {
		return p.AuthMode
	}
	if len(p.EffectiveCredentialSlots()) > 0 {
		return ProviderAuthModeBoundSecret
	}
	return ProviderAuthModeNativeCLI
}

// EffectiveEnvPolicy returns the configured provider environment inheritance policy.
func (p ProviderConfig) EffectiveEnvPolicy() ProviderEnvPolicy {
	if p.EnvPolicy != "" {
		return p.EnvPolicy
	}
	return ProviderEnvPolicyFiltered
}

// EffectiveHomePolicy returns the configured provider home inheritance policy.
func (p ProviderConfig) EffectiveHomePolicy() ProviderHomePolicy {
	if p.HomePolicy != "" {
		return p.HomePolicy
	}
	return ProviderHomePolicyOperator
}

// EffectiveNoneSecurity returns the auth_mode=none safety rationale.
func (p ProviderConfig) EffectiveNoneSecurity() ProviderNoneSecurity {
	if p.NoneSecurity != "" {
		return p.NoneSecurity
	}
	return ProviderNoneSecurityLocalTransport
}

// RuntimeProviderName returns the downstream runtime provider id for harnesses that need one.
func (p ProviderConfig) RuntimeProviderName(providerName string) string {
	if runtimeProvider := strings.TrimSpace(p.RuntimeProvider); runtimeProvider != "" {
		return runtimeProvider
	}
	return strings.TrimSpace(providerName)
}

// EffectiveCredentialSlots returns explicit launch credential slots.
func (p ProviderConfig) EffectiveCredentialSlots() []ProviderCredentialSlot {
	if len(p.CredentialSlots) > 0 {
		return cloneProviderCredentialSlots(p.CredentialSlots)
	}
	return nil
}

// SessionMCPEnabled reports whether AGH should pass per-session MCP servers to the provider.
func (p ProviderConfig) SessionMCPEnabled() bool {
	if p.SessionMCP == nil {
		return true
	}
	return *p.SessionMCP
}

// Validate reports whether the discovery source config is usable.
func (d ProviderModelsDiscoveryConfig) Validate(path string) error {
	command := strings.TrimSpace(d.Command)
	endpoint := strings.TrimSpace(d.Endpoint)
	if command != "" && unsafeDiscoveryCommand(command) {
		return fmt.Errorf("%s.command must be a single-line command", path)
	}
	if endpoint != "" {
		if err := validateAbsoluteHTTPURL(path+".endpoint", endpoint); err != nil {
			return err
		}
	}
	if command != "" && endpoint != "" {
		return fmt.Errorf("%s.command and %s.endpoint are mutually exclusive", path, path)
	}
	if strings.TrimSpace(d.Timeout) != "" {
		if err := validatePositiveDuration(path+".timeout", d.Timeout); err != nil {
			return err
		}
	}
	if d.Enabled != nil && *d.Enabled && command == "" && endpoint == "" {
		return fmt.Errorf("%s requires command or endpoint when enabled", path)
	}
	return nil
}

// DefaultModelCatalogConfig returns the default model catalog source config.
func DefaultModelCatalogConfig() ModelCatalogConfig {
	return ModelCatalogConfig{
		Sources: ModelCatalogSourcesConfig{
			ModelsDev: ModelsDevSourceConfig{
				Enabled:  new(true),
				Endpoint: defaultModelsDevEndpoint,
				TTL:      defaultModelsDevTTL,
				Timeout:  defaultModelsDevTimeout,
			},
		},
	}
}

// Validate reports whether model catalog config is usable.
func (c ModelCatalogConfig) Validate() error {
	return c.Sources.ModelsDev.Validate("model_catalog.sources.models_dev")
}

// EffectiveEnabled reports whether the models.dev source should run.
func (c ModelsDevSourceConfig) EffectiveEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// EffectiveEndpoint returns the configured endpoint or the default models.dev endpoint.
func (c ModelsDevSourceConfig) EffectiveEndpoint() string {
	if endpoint := strings.TrimSpace(c.Endpoint); endpoint != "" {
		return endpoint
	}
	return defaultModelsDevEndpoint
}

// EffectiveTTL returns the configured TTL or the default models.dev TTL.
func (c ModelsDevSourceConfig) EffectiveTTL() string {
	if ttl := strings.TrimSpace(c.TTL); ttl != "" {
		return ttl
	}
	return defaultModelsDevTTL
}

// EffectiveTimeout returns the configured timeout or the default models.dev timeout.
func (c ModelsDevSourceConfig) EffectiveTimeout() string {
	if timeout := strings.TrimSpace(c.Timeout); timeout != "" {
		return timeout
	}
	return defaultModelsDevTimeout
}

// Validate reports whether the models.dev source config is usable.
func (c ModelsDevSourceConfig) Validate(path string) error {
	if err := validateAbsoluteHTTPURL(path+".endpoint", c.EffectiveEndpoint()); err != nil {
		return err
	}
	if err := validatePositiveDuration(path+".ttl", c.EffectiveTTL()); err != nil {
		return err
	}
	return validatePositiveDuration(path+".timeout", c.EffectiveTimeout())
}

func validatePositiveDuration(path string, raw string) error {
	duration, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("%s must be a positive duration: %w", path, err)
	}
	if duration <= 0 {
		return fmt.Errorf("%s must be a positive duration", path)
	}
	return nil
}

func validateAbsoluteHTTPURL(path string, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be an absolute HTTP(S) URL", path)
	}
	switch parsed.Scheme {
	case string(MCPServerTransportHTTP), urlSchemeHTTPS:
		return nil
	default:
		return fmt.Errorf("%s must be an absolute HTTP(S) URL", path)
	}
}

func unsafeDiscoveryCommand(command string) bool {
	return strings.ContainsAny(command, "\x00\r\n")
}
