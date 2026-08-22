package providers

import (
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

type preStartCacheKey struct {
	ProviderName      string
	WorkspaceID       string
	ProfileID         string
	HomeIdentity      string
	SandboxID         string
	SandboxBackend    string
	SandboxProfile    string
	SandboxInstanceID string
	AuthMode          compozyconfig.ProviderAuthMode
	EnvPolicy         compozyconfig.ProviderEnvPolicy
	HomePolicy        compozyconfig.ProviderHomePolicy
	Harness           compozyconfig.ProviderHarness
	RuntimeProvider   string
	Fingerprint       preStartCacheFingerprint
}

func (s *PreStarter) newPreStartCacheKey(
	provider compozyconfig.ProviderConfig,
	env *ProbeEnv,
) (preStartCacheKey, bool) {
	if s == nil || !s.cacheReady {
		return preStartCacheKey{}, false
	}
	scope := normalizePreStartScope(env.PreStartScope)
	providerName := strings.TrimSpace(env.ProviderName)
	if providerName == "" || !scope.cacheable() {
		return preStartCacheKey{}, false
	}
	fingerprint, fingerprinted := preStartSemanticFingerprint(s.cacheKey, provider, env)
	if !fingerprinted {
		return preStartCacheKey{}, false
	}
	return preStartCacheKey{
		ProviderName:      providerName,
		WorkspaceID:       scope.WorkspaceID,
		ProfileID:         scope.ProfileID,
		HomeIdentity:      scope.HomeIdentity,
		SandboxID:         scope.SandboxID,
		SandboxBackend:    scope.SandboxBackend,
		SandboxProfile:    scope.SandboxProfile,
		SandboxInstanceID: scope.SandboxInstanceID,
		AuthMode:          provider.EffectiveAuthMode(),
		EnvPolicy:         provider.EffectiveEnvPolicy(),
		HomePolicy:        provider.EffectiveHomePolicy(),
		Harness:           provider.EffectiveHarness(),
		RuntimeProvider:   strings.TrimSpace(provider.RuntimeProviderName(providerName)),
		Fingerprint:       fingerprint,
	}, true
}

func normalizePreStartScope(scope PreStartScope) PreStartScope {
	return PreStartScope{
		WorkspaceID:       strings.TrimSpace(scope.WorkspaceID),
		ProfileID:         strings.TrimSpace(scope.ProfileID),
		HomeIdentity:      strings.TrimSpace(scope.HomeIdentity),
		SandboxID:         strings.TrimSpace(scope.SandboxID),
		SandboxBackend:    strings.TrimSpace(scope.SandboxBackend),
		SandboxProfile:    strings.TrimSpace(scope.SandboxProfile),
		SandboxInstanceID: strings.TrimSpace(scope.SandboxInstanceID),
	}
}

func (s PreStartScope) cacheable() bool {
	return s.WorkspaceID != "" &&
		s.ProfileID != "" &&
		s.HomeIdentity != "" &&
		s.SandboxID != "" &&
		s.SandboxBackend != "" &&
		s.SandboxProfile != ""
}
