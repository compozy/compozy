package session

import (
	"context"

	"errors"
	"fmt"
	"log/slog"
	"runtime"

	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"

	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/vault"
	shellquote "github.com/kballard/go-shellquote"
)

func sessionEnvironmentCommand(command string, declarations compozyconfig.SessionEnv) (string, error) {
	if len(declarations) == 0 {
		return command, nil
	}
	parsed, err := compozyconfig.ParseLaunchCommand(command)
	if err != nil {
		return "", fmt.Errorf("session: parse declared environment command: %w", err)
	}
	kept := make([]string, 0, len(parsed.Environment)+1+len(parsed.Args))
	for _, assignment := range parsed.Environment {
		name, _, _ := strings.Cut(assignment, "=")
		declared := false
		for target := range declarations {
			if name == target || (runtime.GOOS == "windows" && strings.EqualFold(name, target)) {
				declared = true
				break
			}
		}
		if !declared {
			kept = append(kept, assignment)
		}
	}
	kept = append(kept, parsed.Executable)
	kept = append(kept, parsed.Args...)
	return shellquote.Join(kept...), nil
}

type providerSecretMetadataGetter interface {
	GetMetadata(context.Context, string) (vault.Metadata, error)
}

type providerSecretMetadataResolver struct {
	resolver ProviderSecretResolver
}

func (r providerSecretMetadataResolver) GetMetadata(
	ctx context.Context,
	ref string,
) (vault.Metadata, error) {
	if r.resolver == nil {
		return vault.Metadata{}, vault.ErrSecretNotFound
	}
	if getter, ok := r.resolver.(providerSecretMetadataGetter); ok {
		return getter.GetMetadata(ctx, ref)
	}
	value, err := r.resolver.ResolveRef(ctx, ref)
	if err != nil {
		return vault.Metadata{}, err
	}
	return vault.Metadata{
		Ref:     vault.NormalizeRef(ref),
		Present: strings.TrimSpace(value) != "",
	}, nil
}

// setProviderModelEnv keeps native Claude model advertisement aligned with the negotiated transport ID.
func setProviderModelEnv(env []string, resolved compozyconfig.ResolvedAgent, preferredModel string) []string {
	model := strings.TrimSpace(resolved.Model)
	if model == "" || resolved.Harness != compozyconfig.ProviderHarnessACP {
		return env
	}

	runtimeProvider := strings.TrimSpace(resolved.RuntimeProvider)
	if runtimeProvider == "" {
		runtimeProvider = strings.TrimSpace(resolved.Provider)
	}
	// Claude's launch override changes its advertised model IDs. Seed the same
	// validated transport ID that ACP will select, while COMPOZY_MODEL stays logical.
	if runtimeProvider == runtimeProviderClaude && strings.TrimSpace(preferredModel) != "" {
		model = strings.TrimSpace(preferredModel)
	}

	switch runtimeProvider {
	case runtimeProviderAnthropic, runtimeProviderClaude:
		return setSessionStartEnvValue(env, "ANTHROPIC_MODEL", model)
	default:
		return env
	}
}

type providerSecretBindings struct {
	env                []string
	injectedTargetEnvs map[string]struct{}
	redactionCleanups  []func()
}

func (m *Manager) injectProviderSecrets(
	ctx context.Context,
	resolved compozyconfig.ResolvedAgent,
	env []string,
) (providerSecretBindings, error) {
	bindings := providerSecretBindings{
		env:                env,
		injectedTargetEnvs: make(map[string]struct{}),
	}
	for _, slot := range resolved.CredentialSlots {
		updated, targetEnv, cleanup, err := m.injectProviderSecret(
			ctx,
			resolved,
			slot,
			bindings.env,
		)
		if err != nil {
			runProviderSecretRedactions(bindings.redactionCleanups)
			return providerSecretBindings{}, err
		}
		bindings.env = updated
		if targetEnv == "" {
			continue
		}
		bindings.injectedTargetEnvs[targetEnv] = struct{}{}
		if cleanup != nil {
			bindings.redactionCleanups = append(bindings.redactionCleanups, cleanup)
		}
	}
	return m.injectSessionEnvironment(ctx, resolved, bindings)
}

func (m *Manager) injectSessionEnvironment(
	ctx context.Context,
	resolved compozyconfig.ResolvedAgent,
	bindings providerSecretBindings,
) (providerSecretBindings, error) {
	values, err := authproviders.ResolveSessionEnv(ctx, resolved.SessionEnv, resolved.ProfileName, m.providerSecrets)
	if err != nil {
		runProviderSecretRedactions(bindings.redactionCleanups)
		return providerSecretBindings{}, err
	}
	for name := range resolved.SessionEnv {
		bindings.env = unsetSessionStartEnvKeys(bindings.env, name)
		value, present := values[name]
		if !present {
			logger := m.logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.WarnContext(ctx, "session_env.optional_missing", "name", name, "provider", resolved.Provider)
			delete(bindings.injectedTargetEnvs, name)
			continue
		}
		bindings.redactionCleanups = append(bindings.redactionCleanups,
			authproviders.RegisterSessionEnvRedaction(name, resolved.SessionEnv[name], value))
		bindings.env = setSessionStartEnvValue(bindings.env, name, value)
		bindings.injectedTargetEnvs[name] = struct{}{}
	}
	return bindings, nil
}

func (m *Manager) injectProviderSecret(
	ctx context.Context,
	resolved compozyconfig.ResolvedAgent,
	slot compozyconfig.ProviderCredentialSlot,
	env []string,
) ([]string, string, func(), error) {
	secretRef := vault.NormalizeRef(slot.SecretRef)
	targetEnv := strings.TrimSpace(slot.TargetEnv)
	if secretRef == "" || targetEnv == "" {
		return env, "", nil, nil
	}
	if vault.IsSecretRef(secretRef) {
		env = unsetSessionStartEnvKeys(env, targetEnv)
	}
	value, err := m.providerSecrets.ResolveRef(ctx, secretRef)
	if err != nil {
		if shouldSkipMissingProviderSecret(resolved, secretRef, slot, err) {
			return env, "", nil, nil
		}
		resolveErr := fmt.Errorf(
			"session: resolve provider credential %q for %q: %w",
			slot.Name,
			resolved.Provider,
			err,
		)
		if isMissingRequiredProviderSecret(slot, err) {
			return nil, "", nil, providerCredentialFailure(resolved, slot, resolveErr)
		}
		return nil, "", nil, resolveErr
	}
	return setSessionStartEnvValue(
			env,
			targetEnv,
			value,
		), targetEnv, diagnostics.RegisterDynamicSecret(
			value,
		), nil
}

func isMissingRequiredProviderSecret(slot compozyconfig.ProviderCredentialSlot, err error) bool {
	return slot.Required &&
		(errors.Is(err, vault.ErrMissingSecret) || errors.Is(err, vault.ErrSecretNotFound))
}

func providerCredentialFailure(
	resolved compozyconfig.ResolvedAgent,
	slot compozyconfig.ProviderCredentialSlot,
	err error,
) error {
	status := authproviders.CredentialStatus{
		Name:      strings.TrimSpace(slot.Name),
		TargetEnv: strings.TrimSpace(slot.TargetEnv),
		SecretRef: vault.NormalizeRef(slot.SecretRef),
		Kind:      strings.TrimSpace(slot.Kind),
		Required:  slot.Required,
	}
	item := authproviders.DiagnosticItem(
		strings.TrimSpace(resolved.Provider),
		authproviders.MissingCredentialClassification(status),
	)
	return acp.WrapFailure(
		store.FailureProviderAuth,
		"provider auth pre-start probe failed",
		diagnostics.NewStructuredError(item, err),
	)
}
