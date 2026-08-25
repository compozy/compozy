package daemon

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerenv"
	skillspkg "github.com/compozy/compozy/internal/skills"
)

// SkillInjectionFilter decides whether one resolved skill remains in prompt catalogs.
type SkillInjectionFilter func(*skillspkg.Skill) bool

// HarnessContextResolverOption configures provider-native skill-root resolution.
type HarnessContextResolverOption func(*HarnessContextResolver)

// WithHarnessSkillInjectionHome configures the operator and managed provider homes.
func WithHarnessSkillInjectionHome(homePaths compozyconfig.HomePaths) HarnessContextResolverOption {
	return func(resolver *HarnessContextResolver) {
		resolver.skillInjection.homePaths = homePaths
	}
}

// WithHarnessSkillInjectionLogger configures suppression decision logging.
func WithHarnessSkillInjectionLogger(logger *slog.Logger) HarnessContextResolverOption {
	return func(resolver *HarnessContextResolver) {
		if logger != nil {
			resolver.skillInjection.logger = logger
		}
	}
}

// WithHarnessSkillInjectionEnvLookup configures the effective provider environment lookup.
func WithHarnessSkillInjectionEnvLookup(
	lookup func(string) (string, bool),
) HarnessContextResolverOption {
	return func(resolver *HarnessContextResolver) {
		if lookup != nil {
			resolver.skillInjection.lookupEnv = lookup
		}
	}
}

type skillInjectionPolicyResolver struct {
	mu           sync.Mutex
	profiles     map[string]*sessionSkillInjectionProfile
	profileOrder []string
	homePaths    compozyconfig.HomePaths
	lookupEnv    func(string) (string, bool)
	logger       *slog.Logger
}

type sessionSkillInjectionProfile struct {
	sessionID   string
	provider    string
	nativeRoots map[string]struct{}
	logger      *slog.Logger
	rootMu      sync.Mutex
	rootCache   map[string]string
}

const (
	maxSkillInjectionProfiles = 512
	maxSkillRootCacheEntries  = 256
	nativeProviderClaude      = "claude"
	nativeProviderHermes      = "hermes"
	nativeProviderOpenClaw    = "openclaw"
)

func newSkillInjectionPolicyResolver() skillInjectionPolicyResolver {
	return skillInjectionPolicyResolver{
		profiles:  make(map[string]*sessionSkillInjectionProfile),
		lookupEnv: os.LookupEnv,
		logger:    slog.Default(),
	}
}

func (r *skillInjectionPolicyResolver) resolve(sessionCtx HarnessSessionContext) SkillInjectionFilter {
	if r == nil || sessionCtx.Provider == "" {
		return nil
	}
	key := strings.TrimSpace(sessionCtx.SessionID)
	if key != "" {
		r.mu.Lock()
		if profile := r.profiles[key]; profile != nil {
			r.mu.Unlock()
			return profile.filter
		}
		profile := r.newProfile(sessionCtx)
		if len(r.profileOrder) >= maxSkillInjectionProfiles {
			oldest := r.profileOrder[0]
			r.profileOrder = r.profileOrder[1:]
			delete(r.profiles, oldest)
		}
		r.profiles[key] = profile
		r.profileOrder = append(r.profileOrder, key)
		r.mu.Unlock()
		return profile.filter
	}
	return r.newProfile(sessionCtx).filter
}

func (r *skillInjectionPolicyResolver) newProfile(
	sessionCtx HarnessSessionContext,
) *sessionSkillInjectionProfile {
	resolution := nativeSkillRootResolution{
		Provider:           sessionCtx.Provider,
		ProviderHomePolicy: sessionCtx.ProviderHomePolicy,
		Workspace:          sessionCtx.Workspace,
		HomePaths:          r.homePaths,
		LookupEnv:          r.lookupEnv,
	}
	roots := resolveSessionNativeSkillRoots(&resolution)
	set := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		set[root] = struct{}{}
	}
	return &sessionSkillInjectionProfile{
		sessionID:   strings.TrimSpace(sessionCtx.SessionID),
		provider:    sessionCtx.Provider,
		nativeRoots: set,
		logger:      r.logger,
		rootCache:   make(map[string]string),
	}
}

func (p *sessionSkillInjectionProfile) filter(skill *skillspkg.Skill) bool {
	if p == nil || skill == nil || !nativeProviderReadsOrigin(p.provider, skill.Origin) {
		return true
	}
	root := p.canonicalSkillRoot(skill.RootDir)
	if root == "" {
		return true
	}
	if _, native := p.nativeRoots[root]; !native {
		return true
	}
	logger := p.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info(
		"skills.injection.suppressed",
		"session_id", p.sessionID,
		"skill", strings.TrimSpace(skill.Meta.Name),
		"source", strings.TrimSpace(skill.Origin),
		"provider", p.provider,
	)
	return false
}

func (p *sessionSkillInjectionProfile) canonicalSkillRoot(path string) string {
	trimmed := strings.TrimSpace(path)
	p.rootMu.Lock()
	defer p.rootMu.Unlock()
	if cached, ok := p.rootCache[trimmed]; ok {
		return cached
	}
	if len(p.rootCache) >= maxSkillRootCacheEntries {
		clear(p.rootCache)
	}
	canonical := canonicalNativeSkillRoot(trimmed)
	p.rootCache[trimmed] = canonical
	return canonical
}

type nativeSkillRootResolution struct {
	Provider           string
	ProviderHomePolicy compozyconfig.ProviderHomePolicy
	Workspace          string
	HomePaths          compozyconfig.HomePaths
	LookupEnv          func(string) (string, bool)
}

func resolveSessionNativeSkillRoots(input *nativeSkillRootResolution) []string {
	if input == nil {
		return nil
	}
	provider := canonicalNativeSkillProvider(input.Provider)
	if provider == "" {
		return nil
	}
	spec, ok := providerenv.NativeProviderHomeSpec(provider)
	if !ok || spec.WorkspaceSkillDir == "" {
		return nil
	}
	lookup := input.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	workspace := strings.TrimSpace(input.Workspace)
	home := nativeProviderHome(input.HomePaths, provider, input.ProviderHomePolicy)
	roots := make([]string, 0, 2)
	if workspace != "" {
		roots = append(roots, filepath.Join(workspace, spec.WorkspaceSkillDir, "skills"))
	}
	providerDir := ""
	if len(spec.EnvKeys) > 0 {
		providerDir = lookupTrimmed(lookup, spec.EnvKeys[0])
	}
	if input.ProviderHomePolicy == compozyconfig.ProviderHomePolicyIsolated && home != "" {
		providerDir = filepath.Join(home, spec.IsolatedChild)
	} else if providerDir == "" && home != "" {
		providerDir = filepath.Join(home, spec.OperatorFallback)
	}
	if providerDir != "" {
		roots = append(roots, filepath.Join(providerDir, "skills"))
	}
	return canonicalNativeSkillRoots(roots)
}

func canonicalNativeSkillProvider(provider string) string {
	switch compozyconfig.CanonicalProviderName(provider) {
	case nativeProviderClaude:
		return nativeProviderClaude
	case nativeProviderOpenClaw:
		return nativeProviderOpenClaw
	case nativeProviderHermes:
		return nativeProviderHermes
	default:
		return ""
	}
}

func nativeProviderHome(
	homePaths compozyconfig.HomePaths,
	provider string,
	policy compozyconfig.ProviderHomePolicy,
) string {
	if policy == compozyconfig.ProviderHomePolicyIsolated {
		if strings.TrimSpace(homePaths.HomeDir) == "" {
			return ""
		}
		return filepath.Join(homePaths.HomeDir, "providers", provider)
	}
	return strings.TrimSpace(homePaths.OperatorHomeDir)
}

func nativeProviderReadsOrigin(provider string, origin string) bool {
	switch strings.TrimSpace(origin) {
	case compozyconfig.SkillSourceAgents:
		return provider == nativeProviderOpenClaw || provider == nativeProviderHermes
	case compozyconfig.SkillSourceClaude:
		return provider == nativeProviderClaude
	default:
		return false
	}
}

func canonicalNativeSkillRoots(roots []string) []string {
	seen := make(map[string]struct{}, len(roots))
	result := make([]string, 0, len(roots))
	for _, root := range roots {
		canonical := canonicalNativeSkillRoot(root)
		if canonical == "" {
			continue
		}
		if _, duplicate := seen[canonical]; duplicate {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}
	return result
}

func canonicalNativeSkillRoot(root string) string {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return ""
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return filepath.Clean(trimmed)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(absolute)
}

func lookupTrimmed(lookup func(string) (string, bool), key string) string {
	value, ok := lookup(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
