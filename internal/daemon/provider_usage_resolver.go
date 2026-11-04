package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"

	config "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/providerenv"
	"github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/providerusage"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/vault"
)

// Resolve on each domain reconciliation so replaced bound secrets are fenced by
// the collector's credential identity, including changes without a config edit.
type providerUsageResolver struct {
	mu         sync.RWMutex
	cfg        config.Config
	home       config.HomePaths
	baseEnv    []string
	profiles   *profile.Manager
	secrets    modelcatalog.ProviderSecretResolver
	metadata   providers.VaultRefResolver
	collectors providerusage.Registry
	logger     *slog.Logger
	// selectorWarned holds the selector key last logged as missing per provider, so a gated alias
	// logs once per change instead of on every resolution.
	selectorWarned sync.Map
}

// Targets skips archived and reserved profiles so they cannot block reconciliation.
func (r *providerUsageResolver) Targets(ctx context.Context, profileID string) ([]providerusage.Target, error) {
	r.mu.RLock()
	cfg := config.CloneConfig(&r.cfg)
	r.mu.RUnlock()
	profileName := "default"
	if r.profiles != nil {
		name, err := r.profiles.ProfileName(ctx, profileID)
		// Archived or lifecycle-reserved profiles cannot own new collection work;
		// they publish no targets instead of failing reconciliation for every profile.
		if errors.Is(err, profile.ErrArchived) || errors.Is(err, profile.ErrUnavailable) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		profileName = name
	} else if profileID != store.DefaultProfileID {
		return nil, providerusage.NewUsageError(providerusage.ErrorCodeProfileNotFound, profileID)
	}
	if err := config.ApplyProviderUsageProfile(r.home, profileName, &cfg); err != nil {
		return nil, fmt.Errorf("provider usage: load profile layer: %w", err)
	}
	ids := config.BuiltinProviders()
	maps.Copy(ids, cfg.Providers)
	targets := make([]providerusage.Target, 0, len(ids))
	for _, id := range slices.Sorted(maps.Keys(ids)) {
		provider, err := cfg.ResolveProvider(id)
		if err != nil {
			return nil, fmt.Errorf("provider usage: resolve provider: %w", err)
		}
		target := providerusage.Target{ProfileID: profileID, ProviderID: id, ProviderName: provider.DisplayName}
		if target.ProviderName == "" {
			target.ProviderName = id
		}
		target.CollectorID = r.collectorFamily(id, provider)
		switch {
		case !cfg.ProviderUsage.ProviderEnabled(id):
			target.Error = providerusage.NewUsageError(providerusage.ErrorCodeDisabled, id)
		case r.collectors[target.CollectorID] == nil:
			target.Error = providerusage.NewUsageError(providerusage.ErrorCodeNoCollector, id)
		case !providerUsageAuthSupported(target.CollectorID, provider.EffectiveAuthMode()):
			target.Error = providerusage.NewUsageError(providerusage.ErrorCodeNoCollector, id)
			target.Error.Message = "Usage collection does not support this provider authentication mode"
		default:
			route, fingerprint, routeErr := r.route(
				ctx,
				profileName,
				id,
				target.CollectorID,
				provider,
				cfg.ProviderUsage.ClaudeKeychainAccess(),
			)
			target.Route, target.RouteFingerprint = route, fingerprint
			if routeErr != nil {
				target.Error = providerusage.AsUsageError(routeErr, id)
			}
		}
		if target.Error != nil {
			target.Status = target.Error.Status()
		}
		targets = append(targets, target)
	}
	return targets, nil
}

// collectorFamily picks the collector that owns a provider's usage. A provider's own ID wins, so
// Pi-harness built-ins whose runtime_provider names a model vendor keep their collector; a configured
// alias (for example a second Claude account) collects through its runtime_provider's collector
// with its own launch environment and secrets. It returns the own ID when neither has a collector.
func (r *providerUsageResolver) collectorFamily(id string, provider config.ProviderConfig) string {
	if r.collectors[id] != nil {
		return id
	}
	runtime := strings.TrimSpace(provider.RuntimeProvider)
	if runtime == "" || r.collectors[runtime] == nil ||
		!providerUsageAuthSupported(runtime, provider.EffectiveAuthMode()) {
		return id
	}
	// Key-based collectors read the bound `api_key` slot; an alias without it cannot collect.
	if provider.EffectiveAuthMode() == config.ProviderAuthModeBoundSecret &&
		!slices.ContainsFunc(provider.EffectiveCredentialSlots(), func(slot config.ProviderCredentialSlot) bool {
			return slot.Name == providerUsageAPIKeySlot
		}) {
		return id
	}
	return runtime
}

// providerUsageAPIKeySlot is the bound credential slot key-based usage collectors read.
const providerUsageAPIKeySlot = "api_key"

func (r *providerUsageResolver) route(
	ctx context.Context, profileName, id, collectorID string, provider config.ProviderConfig, keychain bool,
) (providerusage.ResolvedRoute, string, error) {
	env := procutil.FilteredDaemonEnv(r.baseEnv)
	if provider.EffectiveEnvPolicy() == config.ProviderEnvPolicyIsolated {
		env = procutil.IsolatedDaemonEnv(r.baseEnv)
	}
	parsed, err := config.ParseLaunchCommand(provider.Command)
	if err != nil {
		return providerusage.ResolvedRoute{}, "", providerusage.NewUsageError(
			providerusage.ErrorCodeProviderUnavailable,
			id,
		)
	}
	env, err = providerenv.ResolveHomeEnv(r.home, id, provider.EffectiveHomePolicy(), env)
	if err != nil {
		return providerusage.ResolvedRoute{}, "", providerusage.NewUsageError(
			providerusage.ErrorCodeProviderUnavailable,
			id,
		)
	}
	// Session startup applies the home policy first and the launch command's environment last.
	env, owned, err := applyLaunchEnvironment(env, parsed)
	if err != nil {
		usageErr := providerusage.NewUsageError(providerusage.ErrorCodeProviderUnavailable, id)
		usageErr.Message = err.Error()
		return providerusage.ResolvedRoute{}, "", usageErr
	}
	if usageErr := r.aliasSelectorError(id, collectorID, provider, env, owned); usageErr != nil {
		return providerusage.ResolvedRoute{}, "", usageErr
	}
	// Declared credentials never remain in the general child environment.
	for _, slot := range provider.EffectiveCredentialSlots() {
		env = slices.DeleteFunc(env, func(entry string) bool { return strings.HasPrefix(entry, slot.TargetEnv+"=") })
	}
	route := providerusage.ResolvedRoute{
		Env:            env,
		BoundSecrets:   map[string]string{},
		KeychainAccess: collectorID == nativeProviderClaude && keychain,
	}
	if err := r.bindSecrets(ctx, profileName, id, provider, &route); err != nil {
		return route, "", err
	}

	if provider.EffectiveAuthMode() == config.ProviderAuthModeNativeCLI {
		native, err := providers.NativeCLIStatus(
			ctx,
			provider,
			&providers.ProbeEnv{ProviderName: id, HomePaths: r.home, CommandEnv: env},
		)
		if err == nil && native != nil && native.Present {
			route.NativeCommand = native.Path
		}
	}
	// Reuse the catalog's execution fingerprint, plus the resolved environment and
	// native binary identity. Secret values are fenced separately by collectors.
	source, err := modelcatalog.NewLiveProviderSource(
		id,
		provider,
		&modelcatalog.LiveProviderSourcesConfig{HomePaths: r.home, BaseEnv: env, SecretResolver: r.secrets},
	)
	fingerprint := config.CommandFingerprint(provider.Command)
	if err == nil {
		fingerprint, err = source.CatalogExecutionFingerprint()
		if err != nil {
			return route, "", providerusage.NewUsageError(providerusage.ErrorCodeProviderUnavailable, id)
		}
	}
	return route, modelcatalog.CatalogExecutionFingerprint(
		fingerprint,
		strings.Join(env, "\x00"),
		route.NativeCommand,
		fmt.Sprint(route.KeychainAccess),
	), nil
}

// aliasSelectorError fails closed for a native-CLI alias whose effective account variable was not
// assigned a non-empty value by its own launch command: inherited daemon values would read the
// default account and publish its quota under the alias. The family-ID provider and bound-secret
// aliases are exempt.
func (r *providerUsageResolver) aliasSelectorError(
	id, collectorID string, provider config.ProviderConfig, env []string, owned map[string]bool,
) *providerusage.UsageError {
	if id == collectorID || provider.EffectiveAuthMode() != config.ProviderAuthModeNativeCLI {
		return nil
	}
	selector, ok := r.collectors[collectorID].(providerusage.AccountSelector)
	if !ok {
		return nil
	}
	key := selector.SelectorKey(env)
	if key == "" || owned[key] {
		r.selectorWarned.Delete(id)
		return nil
	}
	if previous, logged := r.selectorWarned.Swap(id, key); (!logged || previous != key) && r.logger != nil {
		r.logger.Warn("provider_usage.alias_selector_missing",
			"provider", id, "collector", collectorID, "selector_key", key)
	}
	usageErr := providerusage.NewUsageError(providerusage.ErrorCodeAuthMissing, id)
	usageErr.Message = "Set " + key + " in this provider's command so Compozy reads its own sign-in"
	return usageErr
}

func (r *providerUsageResolver) bindSecrets(
	ctx context.Context,
	profileName, id string,
	provider config.ProviderConfig,
	route *providerusage.ResolvedRoute,
) error {
	if provider.EffectiveAuthMode() == config.ProviderAuthModeBoundSecret {
		slots, err := providers.ProfileCredentialSlots(
			ctx,
			id,
			profileName,
			provider.EffectiveCredentialSlots(),
			r.metadata,
		)
		if err != nil {
			return providerusage.NewUsageError(providerusage.ErrorCodeAuthMissing, id)
		}
		for _, slot := range slots {
			if slot.SecretRef == "" || r.secrets == nil {
				if slot.Required {
					return providerusage.NewUsageError(providerusage.ErrorCodeAuthMissing, id)
				}
				continue
			}
			value, err := r.secrets.ResolveRef(ctx, vault.NormalizeRef(slot.SecretRef))
			if err != nil || value == "" {
				if slot.Required {
					return providerusage.NewUsageError(providerusage.ErrorCodeAuthMissing, id)
				}
				continue
			}
			route.BoundSecrets[slot.Name] = value
		}
	}
	return nil
}

func providerUsageAuthSupported(id string, mode config.ProviderAuthMode) bool {
	switch id {
	case "codex", nativeProviderClaude, "cursor", "gemini", "kimi-cli":
		return mode == config.ProviderAuthModeNativeCLI
	case "openrouter", "vercel-ai-gateway", "zai", "moonshot":
		return mode == config.ProviderAuthModeBoundSecret
	default:
		return false
	}
}
