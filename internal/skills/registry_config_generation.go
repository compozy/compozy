package skills

import (
	"context"
	"errors"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/store"
)

var ErrConfigGenerationSuperseded = errors.New("skills: config generation superseded")

type configGenerationContextKey struct{}

// WithConfigGeneration binds registry publication work to one settings generation.
func WithConfigGeneration(ctx context.Context, generation int64) context.Context {
	if ctx == nil || generation <= 0 {
		return ctx
	}
	return context.WithValue(ctx, configGenerationContextKey{}, generation)
}

// ConfigGenerationFromContext returns the generation bound to registry publication work.
func ConfigGenerationFromContext(ctx context.Context) int64 {
	if ctx == nil {
		return 0
	}
	generation, _ := ctx.Value(configGenerationContextKey{}).(int64)
	return generation
}

// ConfigGeneration returns the last source-policy generation committed with the catalog.
func (r *Registry) ConfigGeneration() int64 {
	if r == nil {
		return 0
	}
	return r.configGeneration.Load()
}

// ApplyConfigGeneration stages a new source policy and commits it directly when resources are not authoritative.
// A true result requires the caller to republish skill resources using the same generation-bound context.
func (r *Registry) ApplyConfigGeneration(
	ctx context.Context,
	generation int64,
	cfg RegistryConfig,
) (bool, error) {
	if r == nil {
		return false, errors.New("skills: registry is required")
	}
	if err := checkRegistryContext(ctx); err != nil {
		return false, err
	}
	if generation <= 0 {
		return false, errors.New("skills: config generation must be positive")
	}
	candidate := cloneRegistryConfig(cfg)
	r.mu.Lock()
	activeGeneration := r.configGeneration.Load()
	if generation <= activeGeneration || (r.pendingGeneration > 0 && generation <= r.pendingGeneration) {
		winningGeneration := max(activeGeneration, r.pendingGeneration)
		r.mu.Unlock()
		return false, r.supersededGenerationError(ctx, generation, winningGeneration)
	}
	if r.pendingGeneration > 0 {
		if err := r.writeSkillSourcesSuperseded(ctx, r.pendingGeneration, generation); err != nil {
			r.mu.Unlock()
			return false, err
		}
	}
	r.pendingConfig = &candidate
	r.pendingGeneration = generation
	r.pendingProjection = nil
	r.pendingRevision = 0
	resourceAuthority := r.resourceAuthority
	r.mu.Unlock()
	if resourceAuthority {
		return true, nil
	}

	loaded, snapshots, diagnostics, commandCandidates, err := r.loadGlobalSkills(
		WithConfigGeneration(ctx, generation),
		append([]string(nil), candidate.DisabledSkills...),
		candidate,
	)
	if err != nil {
		r.AbortConfigGeneration(generation)
		return false, r.SourceApplyFailureError(ctx, generation, err)
	}
	if err := checkRegistryContext(ctx); err != nil {
		r.AbortConfigGeneration(generation)
		return false, err
	}
	r.mu.Lock()
	if r.pendingGeneration != generation || r.pendingConfig == nil {
		winningGeneration := max(r.configGeneration.Load(), r.pendingGeneration)
		r.mu.Unlock()
		return false, r.supersededGenerationError(ctx, generation, winningGeneration)
	}
	if err := r.writeSkillSourcesApplied(ctx, generation, *r.pendingConfig); err != nil {
		r.pendingConfig = nil
		r.pendingGeneration = 0
		r.pendingProjection = nil
		r.pendingRevision = 0
		r.mu.Unlock()
		return false, err
	}
	r.cfg = cloneRegistryConfig(*r.pendingConfig)
	r.pendingConfig = nil
	r.pendingGeneration = 0
	r.pendingProjection = nil
	r.pendingRevision = 0
	r.globalSnapshots = filesnap.Clone(snapshots)
	r.globalDiagnostics = cloneDiagnostics(diagnostics)
	r.globalSkills = loaded
	r.globalCommandCandidates = cloneCommandSkillSlice(commandCandidates)
	r.globalLoaded = true
	r.wsCache = make(map[string]*wsCache)
	r.configGeneration.Store(generation)
	r.globalVersion.Add(1)
	shadowEvents := r.buildSkillShadowSummariesFromResolved(
		mergedSkillList(loaded, nil), registryGlobalKey, store.DefaultProfileID, "", "",
	)
	r.mu.Unlock()
	r.emitEventSummaries(ctx, shadowEvents)
	return false, nil
}

// CommitConfigGeneration atomically publishes a staged resource projection and its source policy.
func (r *Registry) CommitConfigGeneration(ctx context.Context, generation int64) error {
	if r == nil {
		return errors.New("skills: registry is required")
	}
	if err := checkRegistryContext(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	if r.pendingGeneration != generation || r.pendingConfig == nil || r.pendingProjection == nil {
		winningGeneration := max(r.configGeneration.Load(), r.pendingGeneration)
		r.mu.Unlock()
		return r.supersededGenerationError(ctx, generation, winningGeneration)
	}
	projection := *r.pendingProjection
	if err := r.writeSkillSourcesApplied(ctx, generation, *r.pendingConfig); err != nil {
		r.pendingConfig = nil
		r.pendingGeneration = 0
		r.pendingProjection = nil
		r.pendingRevision = 0
		r.mu.Unlock()
		return err
	}
	r.cfg = cloneRegistryConfig(*r.pendingConfig)
	r.applyResourceProjectionLocked(r.pendingRevision, projection)
	r.pendingConfig = nil
	r.pendingGeneration = 0
	r.pendingProjection = nil
	r.pendingRevision = 0
	r.configGeneration.Store(generation)
	r.mu.Unlock()

	r.emitResourceProjectionSummaries(ctx, projection)
	return nil
}

// AbortConfigGeneration discards an uncommitted candidate without touching the active catalog.
func (r *Registry) AbortConfigGeneration(generation int64) {
	if r == nil || generation <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pendingGeneration == generation {
		r.pendingConfig = nil
		r.pendingGeneration = 0
		r.pendingProjection = nil
		r.pendingRevision = 0
	}
}

func (r *Registry) registryConfigSnapshot(ctx context.Context) RegistryConfig {
	if r == nil {
		return RegistryConfig{}
	}
	generation := ConfigGenerationFromContext(ctx)
	r.mu.RLock()
	defer r.mu.RUnlock()
	if generation > 0 && generation == r.pendingGeneration && r.pendingConfig != nil {
		return cloneRegistryConfig(*r.pendingConfig)
	}
	return cloneRegistryConfig(r.cfg)
}

func cloneRegistryConfig(cfg RegistryConfig) RegistryConfig {
	cloned := cfg
	cloned.GlobalSkillRoots = append([]compozyconfig.SkillRootSpec(nil), cfg.GlobalSkillRoots...)
	cloned.DisabledSkills = append([]string(nil), cfg.DisabledSkills...)
	return cloned
}
