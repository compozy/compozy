package daemon

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	compozyconfig "github.com/compozy/compozy/internal/config"
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
	mu        sync.Mutex
	profiles  map[string]*sessionSkillInjectionProfile
	homePaths compozyconfig.HomePaths
	lookupEnv func(string) (string, bool)
	logger    *slog.Logger
}

type sessionSkillInjectionProfile struct {
	sessionID   string
	provider    string
	nativeRoots map[string]struct{}
	logger      *slog.Logger
}

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
		r.profiles[key] = profile
		r.mu.Unlock()
		return profile.filter
	}
	return r.newProfile(sessionCtx).filter
}

func (r *skillInjectionPolicyResolver) newProfile(
	sessionCtx HarnessSessionContext,
) *sessionSkillInjectionProfile {
	roots := resolveSessionNativeSkillRoots(nativeSkillRootResolution{
		Provider:           sessionCtx.Provider,
		ProviderHomePolicy: sessionCtx.ProviderHomePolicy,
		Workspace:          sessionCtx.Workspace,
		HomePaths:          r.homePaths,
		LookupEnv:          r.lookupEnv,
	})
	set := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		set[root] = struct{}{}
	}
	return &sessionSkillInjectionProfile{
		sessionID:   strings.TrimSpace(sessionCtx.SessionID),
		provider:    sessionCtx.Provider,
		nativeRoots: set,
		logger:      r.logger,
	}
}

func (p *sessionSkillInjectionProfile) filter(skill *skillspkg.Skill) bool {
	if p == nil || skill == nil || !nativeProviderReadsOrigin(p.provider, skill.Origin) {
		return true
	}
	root := canonicalNativeSkillRoot(skill.RootDir)
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

type nativeSkillRootResolution struct {
	Provider           string
	ProviderHomePolicy compozyconfig.ProviderHomePolicy
	Workspace          string
	HomePaths          compozyconfig.HomePaths
	LookupEnv          func(string) (string, bool)
}

func resolveSessionNativeSkillRoots(input nativeSkillRootResolution) []string {
	provider := canonicalNativeSkillProvider(input.Provider)
	if provider == "" {
		return nil
	}
	lookup := input.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	workspace := strings.TrimSpace(input.Workspace)
	home := nativeProviderHome(input.HomePaths, provider, input.ProviderHomePolicy)
	roots := make([]string, 0, 2)
	switch provider {
	case "claude":
		if workspace != "" {
			roots = append(roots, filepath.Join(workspace, ".claude", "skills"))
		}
		configDir := lookupTrimmed(lookup, "CLAUDE_CONFIG_DIR")
		if configDir == "" && home != "" {
			configDir = filepath.Join(home, ".claude")
			if input.ProviderHomePolicy == compozyconfig.ProviderHomePolicyIsolated {
				configDir = filepath.Join(home, "claude")
			}
		}
		if configDir != "" {
			roots = append(roots, filepath.Join(configDir, "skills"))
		}
	case "openclaw":
		if workspace != "" {
			roots = append(roots, filepath.Join(workspace, ".agents", "skills"))
		}
		stateDir := lookupTrimmed(lookup, "OPENCLAW_STATE_DIR")
		if input.ProviderHomePolicy == compozyconfig.ProviderHomePolicyIsolated && home != "" {
			stateDir = filepath.Join(home, "openclaw")
		}
		if stateDir != "" {
			roots = append(roots, filepath.Join(stateDir, "skills"))
		} else if home != "" {
			roots = append(roots, filepath.Join(home, ".agents", "skills"))
		}
	case "hermes":
		if workspace != "" {
			roots = append(roots, filepath.Join(workspace, ".agents", "skills"))
		}
		hermesHome := lookupTrimmed(lookup, "HERMES_HOME")
		if input.ProviderHomePolicy == compozyconfig.ProviderHomePolicyIsolated && home != "" {
			hermesHome = filepath.Join(home, "hermes")
		}
		if hermesHome == "" && home != "" {
			hermesHome = filepath.Join(home, ".hermes")
		}
		if hermesHome != "" {
			roots = append(roots, filepath.Join(hermesHome, "skills"))
		}
	}
	return canonicalNativeSkillRoots(roots)
}

func canonicalNativeSkillProvider(provider string) string {
	switch compozyconfig.CanonicalProviderName(provider) {
	case "claude":
		return "claude"
	case "openclaw":
		return "openclaw"
	case "hermes":
		return "hermes"
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
		return provider == "openclaw" || provider == "hermes"
	case compozyconfig.SkillSourceClaude:
		return provider == "claude"
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
