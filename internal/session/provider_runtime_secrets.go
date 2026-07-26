package session

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"

	authproviders "github.com/compozy/agh/internal/providers"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/vault"
)

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

func setProviderModelEnv(env []string, resolved aghconfig.ResolvedAgent) []string {
	model := strings.TrimSpace(resolved.Model)
	if model == "" || resolved.Harness != aghconfig.ProviderHarnessACP {
		return env
	}

	runtimeProvider := strings.TrimSpace(resolved.RuntimeProvider)
	if runtimeProvider == "" {
		runtimeProvider = strings.TrimSpace(resolved.Provider)
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
	resolved aghconfig.ResolvedAgent,
	env []string,
) (providerSecretBindings, error) {
	bindings := providerSecretBindings{
		env:                env,
		injectedTargetEnvs: make(map[string]struct{}),
	}
	if len(resolved.CredentialSlots) == 0 {
		return bindings, nil
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
	return bindings, nil
}

func (m *Manager) injectProviderSecret(
	ctx context.Context,
	resolved aghconfig.ResolvedAgent,
	slot aghconfig.ProviderCredentialSlot,
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

func isMissingRequiredProviderSecret(slot aghconfig.ProviderCredentialSlot, err error) bool {
	return slot.Required &&
		(errors.Is(err, vault.ErrMissingSecret) || errors.Is(err, vault.ErrSecretNotFound))
}

func providerCredentialFailure(
	resolved aghconfig.ResolvedAgent,
	slot aghconfig.ProviderCredentialSlot,
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
