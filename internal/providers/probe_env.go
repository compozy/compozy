// Package providers owns provider authentication classification and probes.
package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/providerauth"
	"github.com/compozy/agh/internal/vault"
)

// VaultRefResolver resolves redacted provider credential metadata.
type VaultRefResolver interface {
	GetMetadata(ctx context.Context, ref string) (vault.Metadata, error)
}

// ProbeEnv supplies process, env, and vault access to provider auth probes.
type ProbeEnv struct {
	ProviderName string
	HomePaths    aghconfig.HomePaths
	LookPath     func(string) (string, error)
	LookupEnv    func(string) (string, bool)
	Vault        VaultRefResolver
	CommandEnv   []string
	RunCommand   ProviderAuthCommandRunner
}

// CredentialStatus reports one provider launch credential slot readiness.
type CredentialStatus struct {
	Name      string
	TargetEnv string
	SecretRef string
	Kind      string
	Required  bool
	Present   bool
	Source    string
}

// Normalize fills safe defaults without mutating the caller's env.
func (e *ProbeEnv) Normalize() ProbeEnv {
	if e == nil {
		return ProbeEnv{
			LookPath:  exec.LookPath,
			LookupEnv: os.LookupEnv,
		}
	}
	normalized := *e
	if normalized.LookPath == nil {
		normalized.LookPath = exec.LookPath
	}
	if normalized.LookupEnv == nil {
		normalized.LookupEnv = os.LookupEnv
	}
	if normalized.RunCommand == nil {
		normalized.RunCommand = DefaultProviderAuthCommandRunner
	}
	normalized.ProviderName = strings.TrimSpace(normalized.ProviderName)
	return normalized
}

// NativeCLIStatus resolves the CLI binary used by a native provider-auth probe.
func NativeCLIStatus(
	provider aghconfig.ProviderConfig,
	env *ProbeEnv,
) (*providerauth.NativeCLIStatus, error) {
	normalized := env.Normalize()
	return providerauth.NativeCLIStatusForProvider(provider, normalized.LookPath)
}

// LaunchCommandStatus resolves the first token of the launch command used by a session start.
func LaunchCommandStatus(
	provider aghconfig.ProviderConfig,
	env *ProbeEnv,
) (*providerauth.NativeCLIStatus, error) {
	normalized := env.Normalize()
	return providerauth.NativeCLIStatusForCommand(
		strings.TrimSpace(provider.Command),
		providerauth.NativeCLISourceCommand,
		normalized.LookPath,
	)
}

// CredentialStatuses resolves configured credential slots without reading plaintext secrets.
func CredentialStatuses(
	ctx context.Context,
	provider aghconfig.ProviderConfig,
	env *ProbeEnv,
) ([]CredentialStatus, error) {
	normalized := env.Normalize()
	slots := provider.EffectiveCredentialSlots()
	if len(slots) == 0 {
		return nil, nil
	}
	statuses := make([]CredentialStatus, 0, len(slots))
	for _, slot := range slots {
		status, err := credentialStatus(ctx, slot, normalized)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func credentialStatus(
	ctx context.Context,
	slot aghconfig.ProviderCredentialSlot,
	env ProbeEnv,
) (CredentialStatus, error) {
	secretRef := vault.NormalizeRef(slot.SecretRef)
	status := CredentialStatus{
		Name:      strings.TrimSpace(slot.Name),
		TargetEnv: strings.TrimSpace(slot.TargetEnv),
		SecretRef: secretRef,
		Kind:      strings.TrimSpace(slot.Kind),
		Required:  slot.Required,
	}
	switch {
	case vault.IsEnvRef(secretRef):
		status.Source = "env"
		envName, err := vault.EnvNameFromRef(secretRef)
		if err != nil {
			return CredentialStatus{}, err
		}
		value, ok := env.LookupEnv(envName)
		status.Present = ok && strings.TrimSpace(value) != ""
		return status, nil
	case vault.IsSecretRef(secretRef):
		status.Source = "vault"
		if env.Vault == nil {
			return status, nil
		}
		metadata, err := env.Vault.GetMetadata(ctx, secretRef)
		if err != nil {
			if errors.Is(err, vault.ErrSecretNotFound) || errors.Is(err, vault.ErrMissingSecret) {
				return status, nil
			}
			return CredentialStatus{}, fmt.Errorf("resolve provider credential metadata: %w", err)
		}
		status.Present = metadata.Present
		return status, nil
	default:
		status.Source = "unsupported"
		return status, nil
	}
}

func firstMissingRequiredCredential(statuses []CredentialStatus) (CredentialStatus, bool) {
	for _, status := range statuses {
		if status.Required && !status.Present {
			return status, true
		}
	}
	return CredentialStatus{}, false
}
