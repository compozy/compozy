package skills

import (
	"slices"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
)

// Unchanged publication inputs reuse parsed definitions across callback retries.
type discoveryCacheEntry struct {
	generation  int64
	roots       []compozyconfig.SkillRootSpec
	paths       []workspaceSkillPath
	disabled    []string
	snapshots   map[string]filesnap.Snapshot
	candidates  []*Skill
	diagnostics []SkillDiagnostic
}

func (e *discoveryCacheEntry) matches(key discoveryCacheEntry) bool {
	return e != nil &&
		e.generation == key.generation &&
		slices.Equal(e.roots, key.roots) &&
		slices.Equal(e.paths, key.paths) &&
		slices.Equal(e.disabled, key.disabled) &&
		filesnap.Equal(e.snapshots, key.snapshots)
}

func (r *Registry) cachedDiscovery(cacheKey string, key discoveryCacheEntry) ([]*Skill, []SkillDiagnostic, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.discoveryCache[cacheKey]
	if !ok || !entry.matches(key) {
		return nil, nil, false
	}
	return cloneCommandSkillSlice(entry.candidates), cloneDiagnostics(entry.diagnostics), true
}

func (r *Registry) storeDiscovery(
	cacheKey string,
	key discoveryCacheEntry,
	candidates []*Skill,
	diagnostics []SkillDiagnostic,
) {
	entry := &discoveryCacheEntry{
		generation:  key.generation,
		roots:       slices.Clone(key.roots),
		paths:       slices.Clone(key.paths),
		disabled:    slices.Clone(key.disabled),
		snapshots:   filesnap.Clone(key.snapshots),
		candidates:  cloneCommandSkillSlice(candidates),
		diagnostics: cloneDiagnostics(diagnostics),
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.discoveryCache[cacheKey] = entry
}
