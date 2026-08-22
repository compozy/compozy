package session

import (
	"context"

	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/compozy/compozy/internal/providerenv"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/sandbox"
	"github.com/compozy/compozy/internal/subprocess"

	"github.com/compozy/compozy/internal/vault"
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
	resolved compozyconfig.ResolvedAgent,
	opts acp.StartOpts,
) (acp.StartOpts, error) {
	opts, secretBindings, err := m.prepareProviderStartPolicies(ctx, resolved, opts)
	if err != nil {
		return acp.StartOpts{}, err
	}
	if session != nil {
		session.addProviderSecretRedactions(secretBindings.redactionCleanups)
	}
	if resolved.Harness == compozyconfig.ProviderHarnessPiACP &&
		resolved.AuthMode == compozyconfig.ProviderAuthModeBoundSecret {
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
	resolved compozyconfig.ResolvedAgent,
	opts acp.StartOpts,
) (acp.StartOpts, providerSecretBindings, error) {
	profileSlots, err := authproviders.ProfileCredentialSlots(
		ctx,
		resolved.Provider,
		resolved.ProfileName,
		resolved.CredentialSlots,
		providerSecretMetadataResolver{resolver: m.providerSecrets},
	)
	if err != nil {
		return acp.StartOpts{}, providerSecretBindings{}, err
	}
	resolved.CredentialSlots = profileSlots
	opts.Env = setProviderStartEnv(opts.Env, resolved)

	if resolved.HomePolicy == compozyconfig.ProviderHomePolicyIsolated {
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
	if resolved.Harness == compozyconfig.ProviderHarnessPiACP &&
		resolved.AuthMode == compozyconfig.ProviderAuthModeNativeCLI {
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
	return opts, secretBindings, nil
}

func setProviderStartEnv(env []string, resolved compozyconfig.ResolvedAgent) []string {
	env = setSessionStartEnvValue(env, "COMPOZY_PROVIDER", strings.TrimSpace(resolved.Provider))
	env = setSessionStartEnvValue(env, "COMPOZY_PROVIDER_HARNESS", string(resolved.Harness))
	env = setSessionStartEnvValue(env, "COMPOZY_PROVIDER_AUTH_MODE", string(resolved.AuthMode))
	env = setSessionStartEnvValue(env, "COMPOZY_PROVIDER_ENV_POLICY", string(resolved.EnvPolicy))
	env = setSessionStartEnvValue(env, "COMPOZY_PROVIDER_HOME_POLICY", string(resolved.HomePolicy))
	env = setSessionStartEnvValue(env, "COMPOZY_MODEL", strings.TrimSpace(resolved.Model))
	return setProviderModelEnv(env, resolved)
}

func providerProbeEnvForStart(
	m *Manager,
	session *Session,
	resolved compozyconfig.ResolvedAgent,
	env []string,
	cwd string,
	launcher sandbox.Launcher,
) authproviders.ProbeEnv {
	probe := authproviders.ProbeEnv{
		ProviderName:  strings.TrimSpace(resolved.Provider),
		HomePaths:     m.homePaths,
		PreStartScope: providerPreStartScopeForSession(session, env),
		LookupEnv:     providerLookupEnv(env),
		Vault:         providerSecretMetadataResolver{resolver: m.providerSecrets},
		CommandEnv:    append([]string(nil), env...),
		CommandDir:    cwd,
	}
	configureProviderCommandRuntime(&probe, launcher)
	return probe
}

func (m *Manager) finalizeProviderProbeEnvForStart(
	session *Session,
	resolved compozyconfig.ResolvedAgent,
	opts acp.StartOpts,
) acp.StartOpts {
	if m == nil || opts.ProviderConfig == nil || strings.TrimSpace(opts.ProviderName) == "" {
		return opts
	}
	next := opts
	providerConfig := *opts.ProviderConfig
	providerConfig.Command = next.Command
	next.ProviderConfig = &providerConfig
	probeEnv := providerProbeEnvForStart(m, session, resolved, next.Env, next.Cwd, next.Launcher)
	next.ProviderAuthEnv = &probeEnv
	return next
}

func providerPreStartScopeForSession(
	session *Session,
	env []string,
) authproviders.PreStartScope {
	if session == nil {
		return authproviders.PreStartScope{}
	}
	info := session.Info()
	if info == nil || info.Sandbox == nil {
		return authproviders.PreStartScope{}
	}
	return authproviders.PreStartScope{
		WorkspaceID:       strings.TrimSpace(info.WorkspaceID),
		ProfileID:         strings.TrimSpace(info.ProfileID),
		HomeIdentity:      providerHomeIdentity(env),
		SandboxID:         strings.TrimSpace(info.Sandbox.SandboxID),
		SandboxBackend:    strings.TrimSpace(info.Sandbox.Backend),
		SandboxProfile:    strings.TrimSpace(info.Sandbox.Profile),
		SandboxInstanceID: strings.TrimSpace(info.Sandbox.InstanceID),
	}
}

func providerHomeIdentity(env []string) string {
	lookupEnv := providerLookupEnv(env)
	for _, key := range []string{"HOME", "USERPROFILE"} {
		if value, ok := lookupEnv(key); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func providerLookupEnv(env []string) func(string) (string, bool) {
	return subprocess.LookupEnvFunc(env)
}
