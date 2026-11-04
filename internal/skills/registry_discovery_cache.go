package skills

import (
	"maps"
	"slices"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
)

// workspaceSourceKeyMarker separates a workspace cache key's scope from its source-config hash.
const workspaceSourceKeyMarker = "@src:"

// Unchanged publication inputs reuse parsed definitions across callback retries.
type discoveryCacheEntry struct {
	generation  int64
	roots       []compozyconfig.SkillRootSpec
	paths       []workspaceSkillPath
	disabled    []string
	snapshots   map[string]filesnap.Snapshot
	candidates  []*Skill
	diagnostics []SkillDiagnostic
	lastAccess  time.Time
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
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.discoveryCache[cacheKey]
	if !ok || !entry.matches(key) {
		return nil, nil, false
	}
	entry.lastAccess = now
	return cloneCommandSkillSlice(entry.candidates), cloneDiagnostics(entry.diagnostics), true
}

func (r *Registry) storeDiscovery(
	cacheKey string,
	key discoveryCacheEntry,
	candidates []*Skill,
	diagnostics []SkillDiagnostic,
) {
	now := r.now()
	entry := &discoveryCacheEntry{
		generation:  key.generation,
		roots:       slices.Clone(key.roots),
		paths:       slices.Clone(key.paths),
		disabled:    slices.Clone(key.disabled),
		snapshots:   filesnap.Clone(key.snapshots),
		candidates:  cloneCommandSkillSlice(candidates),
		diagnostics: cloneDiagnostics(diagnostics),
		lastAccess:  now,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evictExpiredWorkspaceLocked(now)
	if scope, ok := workspaceDiscoveryScope(cacheKey); ok {
		// A new source configuration replaces the scope's previous variants.
		maps.DeleteFunc(r.discoveryCache, func(existing string, _ *discoveryCacheEntry) bool {
			existingScope, variant := workspaceDiscoveryScope(existing)
			return variant && existing != cacheKey && existingScope == scope
		})
	}
	r.discoveryCache[cacheKey] = entry
}

// evictExpiredDiscoveryLocked keeps the global snapshot warm and drops idle workspace variants.
func (r *Registry) evictExpiredDiscoveryLocked(cutoff time.Time) {
	maps.DeleteFunc(r.discoveryCache, func(key string, entry *discoveryCacheEntry) bool {
		return key != registryGlobalKey && entry.lastAccess.Before(cutoff)
	})
}

// workspaceDiscoveryScope strips the source-config hash that workspaceCacheKey appends.
func workspaceDiscoveryScope(cacheKey string) (string, bool) {
	scope, _, found := strings.CutLast(cacheKey, workspaceSourceKeyMarker)
	if !found {
		return "", false
	}
	return scope, true
}
