package providers

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/vault"
)

// SessionEnvResolver resolves declared references at the process launch boundary.
type SessionEnvResolver interface {
	ResolveRef(context.Context, string) (string, error)
}

// RegisterSessionEnvRedaction keeps credential provenance separate from operational socket addresses.
func RegisterSessionEnvRedaction(name string, binding config.SessionEnvBinding, value string) func() {
	source := strings.TrimPrefix(binding.Ref, "env:")
	sensitive := func(name string) bool {
		return !sessionEnvSocketName(name) && procutil.SensitiveEnvName(name)
	}
	if vault.IsSecretRef(binding.Ref) || sensitive(name) || sensitive(source) {
		return diagnostics.RegisterRequiredSecret(value)
	}
	if sessionEnvSocketName(name) || sessionEnvSocketName(source) {
		return func() {}
	}
	return diagnostics.RegisterDynamicSecret(value)
}

func sessionEnvSocketName(name string) bool {
	return strings.EqualFold(name, "SSH_AUTH_SOCK") || strings.EqualFold(name, "DBUS_SESSION_BUS_ADDRESS")
}

// SessionEnvStatuses reports metadata only; it never resolves Vault plaintext.
func SessionEnvStatuses(
	ctx context.Context,
	declarations config.SessionEnv,
	env *ProbeEnv,
) ([]CredentialStatus, error) {
	if err := declarations.Validate("session_env"); err != nil {
		return nil, err
	}
	normalized := env.Normalize()
	statuses := make([]CredentialStatus, 0, len(declarations))
	for _, name := range slices.Sorted(maps.Keys(declarations)) {
		binding := declarations[name]
		status, err := credentialStatus(ctx, config.ProviderCredentialSlot{
			Name: name, TargetEnv: name, SecretRef: binding.Ref, Required: binding.Required,
		}, &normalized)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// ResolveSessionEnv resolves explicit declarations without inheriting undeclared values.
// Missing optional declarations are absent from the result. Callers must remove their
// inherited targets and register returned values for redaction for the process lifetime.
func ResolveSessionEnv(
	ctx context.Context,
	declarations config.SessionEnv,
	profileName string,
	resolver SessionEnvResolver,
) (map[string]string, error) {
	if err := declarations.Validate("session_env"); err != nil {
		return nil, err
	}
	values := make(map[string]string, len(declarations))
	for _, name := range slices.Sorted(maps.Keys(declarations)) {
		binding := declarations[name]
		if strings.HasPrefix(binding.Ref, vault.ProfileSecretRefPrefix) {
			if err := vault.ValidateProfileSecretRefAccess(binding.Ref, profileName); err != nil {
				return nil, fmt.Errorf("session_env.%s: %w", name, err)
			}
		}
		if resolver == nil {
			return nil, errors.New("session_env: reference resolver is unavailable")
		}
		value, err := resolver.ResolveRef(ctx, binding.Ref)
		if err == nil && strings.TrimSpace(value) == "" {
			err = vault.ErrMissingSecret
		}
		if err != nil {
			missing := errors.Is(err, vault.ErrMissingSecret) || errors.Is(err, vault.ErrSecretNotFound)
			if missing && !binding.Required {
				continue
			}
			return nil, fmt.Errorf("session_env.%s: resolve declared reference: %w", name, err)
		}
		values[name] = value
	}
	return values, nil
}
