package skills

import (
	"context"
	"errors"

	"log/slog"

	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/agh/internal/filesnap"

	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const (
	registryAdditionalKey   = "additional"
	registryAgentLocalValue = "agent-local"
	registryBundledKey      = "bundled"
	registryGlobalKey       = "global"
	registryUserKey         = "user"
)

const (
	workspaceCacheTTL          = 10 * time.Minute
	skillSourceMarketplaceName = "marketplace"
	skillSourceWorkspaceName   = "workspace"
)

// Option customizes a Registry instance.
type Option func(*Registry)

// Registry manages global skills loaded at boot and lazily cached workspace skills.
type Registry struct {
	mu                 sync.RWMutex
	globalSkills       map[string]*Skill
	resourceAuthority  bool
	resourceRevision   int64
	resourceWorkspaces map[string]map[string]*Skill
	globalLoaded       bool
	globalSnapshots    map[string]filesnap.Snapshot
	globalDiagnostics  []SkillDiagnostic
	workspaceDisabled  map[string][]string
	wsCache            map[string]*wsCache

	globalVersion atomic.Int64

	cfg               RegistryConfig
	logger            *slog.Logger
	now               func() time.Time
	events            store.EventSummaryStore
	activationContext ActivationContextProvider
}

type skillToggleScope int

const (
	skillToggleScopeGlobal skillToggleScope = iota
	skillToggleScopeWorkspace
)

// WithLogger injects the logger used for registry diagnostics.
func WithLogger(logger *slog.Logger) Option {
	return func(registry *Registry) {
		registry.logger = logger
	}
}

// WithNow injects the clock used for cache timestamps and eviction.
func WithNow(now func() time.Time) Option {
	return func(registry *Registry) {
		registry.now = now
	}
}

// WithEventSummaryStore injects the optional global observe-event writer used
// for public skill.shadowed and skills.load_failed events.
func WithEventSummaryStore(events store.EventSummaryStore) Option {
	return func(registry *Registry) {
		registry.events = events
	}
}

// NewRegistry constructs a Registry with the provided configuration.
func NewRegistry(cfg RegistryConfig, opts ...Option) *Registry {
	registry := &Registry{
		globalSkills:       make(map[string]*Skill),
		resourceWorkspaces: make(map[string]map[string]*Skill),
		globalSnapshots:    make(map[string]filesnap.Snapshot),
		workspaceDisabled:  make(map[string][]string),
		wsCache:            make(map[string]*wsCache),
		cfg:                cfg,
		logger:             slog.Default(),
		now:                time.Now,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(registry)
		}
	}

	if registry.logger == nil {
		registry.logger = slog.Default()
	}
	if registry.now == nil {
		registry.now = time.Now
	}

	return registry
}

// SetEventSummaryStore updates the optional global observe-event writer after
// registry construction.
func (r *Registry) SetEventSummaryStore(events store.EventSummaryStore) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = events
}

// LoadAll loads bundled and user-level skills into the global registry snapshot.
func (r *Registry) LoadAll(ctx context.Context) error {
	return r.reloadGlobal(ctx)
}

// RefreshGlobal re-scans the global skill sources and swaps them in atomically.
func (r *Registry) RefreshGlobal(ctx context.Context) error {
	return r.reloadGlobal(ctx)
}

// GlobalVersion returns the current global snapshot version.
func (r *Registry) GlobalVersion() int64 {
	return r.globalVersion.Load()
}

// Get returns a cloned global skill by name when present.
func (r *Registry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.lookupSkillLocked(name)
	if !ok {
		return nil, false
	}

	return cloneSkill(skill), true
}

// List returns the current global skills sorted by skill name.
func (r *Registry) List() []*Skill {
	return r.rawList()
}

func (r *Registry) rawList() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return mergedSkillList(r.globalSkills, nil)
}

// LoadContent loads the full markdown body for one resolved skill.
func (r *Registry) LoadContent(ctx context.Context, skill *Skill) (string, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return "", err
	}
	if skill == nil {
		return "", errors.New("skills: skill is required")
	}

	switch skill.Source {
	case SourceBundled:
		if r.cfg.BundledFS == nil {
			return "", errors.New("skills: bundled skills filesystem is required")
		}
		return readBundledSkillContent(r.cfg.BundledFS, skill.FilePath)
	default:
		if err := r.verifyMarketplaceSkill(skill); err != nil {
			return "", err
		}
		return ReadSkillContent(skill.FilePath)
	}
}

// LoadResource loads a resource file relative to one resolved skill.
func (r *Registry) LoadResource(ctx context.Context, skill *Skill, relativePath string) (string, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return "", err
	}
	if skill == nil {
		return "", errors.New("skills: skill is required")
	}

	switch skill.Source {
	case SourceBundled:
		if r.cfg.BundledFS == nil {
			return "", errors.New("skills: bundled skills filesystem is required")
		}
		return readBundledSkillResource(r.cfg.BundledFS, skill.Dir, relativePath)
	default:
		if err := r.verifyMarketplaceSkill(skill); err != nil {
			return "", err
		}
		return ReadSkillResourceContent(skill.Dir, relativePath)
	}
}

func (r *Registry) refreshWorkspaceCacheLocked(
	resolved *workspacepkg.ResolvedWorkspace,
	load workspaceLoad,
	cacheKey string,
	workspaceSkills map[string]*Skill,
	workspaceDiagnostics []SkillDiagnostic,
	workspaceDisabled []string,
	now time.Time,
) ([]*Skill, []store.EventSummary) {
	r.evictExpiredWorkspaceLocked(now)
	globalSkills := r.globalSkills
	currentGlobalVersion := r.globalVersion.Load()
	workspaceKey := resourceWorkspaceKey(resolved)
	r.logWorkspaceSkillOverrides(globalSkills, workspaceSkills, workspaceKey)
	shadowEvents := r.buildSkillShadowSummaries(
		globalSkills,
		workspaceSkills,
		skillSourceWorkspaceName,
		workspaceKey,
		"",
	)
	r.wsCache[cacheKey] = &wsCache{
		skills:        workspaceSkills,
		diagnostics:   workspaceDiagnostics,
		snapshots:     load.snapshots,
		lastAccess:    now,
		globalVersion: currentGlobalVersion,
	}
	return mergedSkillListWithDisabled(globalSkills, workspaceSkills, workspaceDisabled), shadowEvents
}
