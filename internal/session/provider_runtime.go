package session

import (
	"context"

	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/providerenv"
	authproviders "github.com/compozy/agh/internal/providers"

	"github.com/compozy/agh/internal/vault"
)

const (
	providerRuntimeAPIKeyKey = "api_key"
)

type envProviderSecretResolver struct {
	lookupEnv func(string) (string, bool)
}

const (
	runtimeProviderAnthropic = "anthropic"
	runtimeProviderClaude    = "claude"
	runtimeProviderCodex     = "codex"
)

func (r envProviderSecretResolver) ResolveRef(ctx context.Context, ref string) (string, error) {
	if ctx == nil {
		return "", errors.New("session: provider secret context is required")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	normalized := vault.NormalizeRef(ref)
	if !vault.IsEnvRef(normalized) {
		return "", fmt.Errorf("%w: %s", vault.ErrUnsupportedSecretRef, normalized)
	}
	if r.lookupEnv == nil {
		return "", errors.New("session: provider env lookup is not configured")
	}
	envName := strings.TrimSpace(strings.TrimPrefix(normalized, "env:"))
	value, ok := r.lookupEnv(envName)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%w: env:%s", vault.ErrMissingSecret, envName)
	}
	return value, nil
}

func (m *Manager) prepareProviderForStart(
	ctx context.Context,
	session *Session,
	resolved aghconfig.ResolvedAgent,
	opts acp.StartOpts,
) (acp.StartOpts, error) {
	opts, secretBindings, err := m.prepareProviderStartPolicies(ctx, resolved, opts)
	if err != nil {
		return acp.StartOpts{}, err
	}
	if session != nil {
		session.addProviderSecretRedactions(secretBindings.redactionCleanups)
	}
	if resolved.Harness == aghconfig.ProviderHarnessPiACP &&
		resolved.AuthMode == aghconfig.ProviderAuthModeBoundSecret {
		runtimeDir, err := m.materializePiRuntime(session, resolved, secretBindings.injectedTargetEnvs)
		if err != nil {
			return acp.StartOpts{}, err
		}
		opts.Env = setSessionStartEnvValue(opts.Env, "PI_CODING_AGENT_DIR", runtimeDir)
	}
	return opts, nil
}

func (m *Manager) prepareProviderStartPolicies(
	ctx context.Context,
	resolved aghconfig.ResolvedAgent,
	opts acp.StartOpts,
) (acp.StartOpts, providerSecretBindings, error) {
	opts.Env = setProviderStartEnv(opts.Env, resolved)

	var err error
	if resolved.HomePolicy == aghconfig.ProviderHomePolicyIsolated {
		opts.Env, err = providerenv.ApplyHomePolicy(
			m.homePaths,
			strings.TrimSpace(resolved.Provider),
			resolved.HomePolicy,
			opts.Env,
		)
		if err != nil {
			return acp.StartOpts{}, providerSecretBindings{}, fmt.Errorf("session: apply provider home policy: %w", err)
		}
	}
	if resolved.Harness == aghconfig.ProviderHarnessPiACP &&
		resolved.AuthMode == aghconfig.ProviderAuthModeNativeCLI {
		opts.Env, err = providerenv.ApplyPiAgentDirPolicy(
			m.homePaths,
			strings.TrimSpace(resolved.Provider),
			resolved.HomePolicy,
			opts.Env,
		)
		if err != nil {
			return acp.StartOpts{}, providerSecretBindings{}, fmt.Errorf(
				"session: apply pi auth directory policy: %w",
				err,
			)
		}
	}
	secretBindings, err := m.injectProviderSecrets(ctx, resolved, opts.Env)
	if err != nil {
		return acp.StartOpts{}, providerSecretBindings{}, err
	}
	opts.Env = secretBindings.env
	opts.ProviderName = strings.TrimSpace(resolved.Provider)
	providerConfig := providerConfigFromResolvedAgent(resolved)
	opts.ProviderConfig = &providerConfig
	probeEnv := providerProbeEnvForStart(m, resolved, opts.Env)
	opts.ProviderAuthEnv = &probeEnv
	return opts, secretBindings, nil
}

func setProviderStartEnv(env []string, resolved aghconfig.ResolvedAgent) []string {
	env = setSessionStartEnvValue(env, "AGH_PROVIDER", strings.TrimSpace(resolved.Provider))
	env = setSessionStartEnvValue(env, "AGH_PROVIDER_HARNESS", string(resolved.Harness))
	env = setSessionStartEnvValue(env, "AGH_PROVIDER_AUTH_MODE", string(resolved.AuthMode))
	env = setSessionStartEnvValue(env, "AGH_PROVIDER_ENV_POLICY", string(resolved.EnvPolicy))
	env = setSessionStartEnvValue(env, "AGH_PROVIDER_HOME_POLICY", string(resolved.HomePolicy))
	env = setSessionStartEnvValue(env, "AGH_MODEL", strings.TrimSpace(resolved.Model))
	return setProviderModelEnv(env, resolved)
}

func providerProbeEnvForStart(
	m *Manager,
	resolved aghconfig.ResolvedAgent,
	env []string,
) authproviders.ProbeEnv {
	return authproviders.ProbeEnv{
		ProviderName: strings.TrimSpace(resolved.Provider),
		HomePaths:    m.homePaths,
		LookupEnv:    providerLookupEnv(env),
		Vault:        providerSecretMetadataResolver{resolver: m.providerSecrets},
		CommandEnv:   append([]string(nil), env...),
	}
}

func providerLookupEnv(env []string) func(string) (string, bool) {
	values := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return func(key string) (string, bool) {
		if value, ok := values[key]; ok {
			return value, true
		}
		return os.LookupEnv(key)
	}
}
