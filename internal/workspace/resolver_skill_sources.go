package workspace

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
)

// cachedSkillsConfig keeps one loaded skills section with the configuration files it was loaded from.
type cachedSkillsConfig struct {
	rootDir    string
	skills     compozyconfig.SkillsConfig
	snapshots  map[string]filesnap.Snapshot
	lastAccess time.Time
}

// SkillsConfig returns the effective skills settings for one registered workspace and profile.
// It reloads configuration only when a file the loader reads changed since the cached load, so
// periodic callers such as the skills watcher stat those files instead of parsing them every tick.
func (r *Resolver) SkillsConfig(ws Workspace, profileName string) (compozyconfig.SkillsConfig, error) {
	profileName = strings.TrimSpace(profileName)
	// Snapshot before loading: an edit racing the load leaves older snapshots, forcing a reload next call.
	snapshots := make(map[string]filesnap.Snapshot)
	if err := r.snapshotConfigDependencies(ws, profileName, snapshots); err != nil {
		return compozyconfig.SkillsConfig{}, err
	}
	cacheKey := workspaceProfileCacheKey(ws.ID, profileName)
	now := r.now()
	r.mu.Lock()
	r.evictExpiredLocked(now)
	if cached := r.skillsConfigCache[cacheKey]; cached != nil && cached.rootDir == ws.RootDir &&
		filesnap.Equal(cached.snapshots, snapshots) {
		cached.lastAccess = now
		skills := cloneSkillsConfig(cached.skills)
		r.mu.Unlock()
		return skills, nil
	}
	// Invalidate and InvalidateAll bump the generation; a load they overtake must not be published.
	generation := r.agentConfigGeneration
	r.mu.Unlock()
	loaded, err := r.loadWorkspaceConfig(ws.RootDir, profileName)
	if err != nil {
		return compozyconfig.SkillsConfig{}, fmt.Errorf("workspace: load config for %q: %w", ws.RootDir, err)
	}
	r.mu.Lock()
	if generation == r.agentConfigGeneration {
		r.skillsConfigCache[cacheKey] = &cachedSkillsConfig{
			rootDir: ws.RootDir, skills: cloneSkillsConfig(loaded.Skills), snapshots: snapshots, lastAccess: now,
		}
	}
	r.mu.Unlock()
	return loaded.Skills, nil
}

// deleteSkillsConfigLocked drops every cached skills section of one workspace while the resolver mutex is held.
func (r *Resolver) deleteSkillsConfigLocked(workspaceID string) {
	for key := range r.skillsConfigCache {
		if key == workspaceID || strings.HasPrefix(key, workspaceID+"\x00") {
			delete(r.skillsConfigCache, key)
		}
	}
}

// evictSkillsConfigCacheLocked removes skills sections not read since cutoff while the resolver mutex is held.
func (r *Resolver) evictSkillsConfigCacheLocked(cutoff time.Time) {
	for key, cached := range r.skillsConfigCache {
		if cached.lastAccess.Before(cutoff) {
			delete(r.skillsConfigCache, key)
		}
	}
}

func cloneSkillsConfig(src compozyconfig.SkillsConfig) compozyconfig.SkillsConfig {
	cloned := src
	cloned.Sources = slices.Clone(src.Sources)
	cloned.CustomSources = slices.Clone(src.CustomSources)
	cloned.DisabledSkills = slices.Clone(src.DisabledSkills)
	cloned.AllowedMarketplaceHooks = slices.Clone(src.AllowedMarketplaceHooks)
	return cloned
}

func (r *Resolver) configForSkillScan(
	ws Workspace,
	profileName string,
	cacheKey string,
) (compozyconfig.Config, bool, error) {
	r.mu.RLock()
	cached := r.cache[cacheKey]
	var cfg compozyconfig.Config
	if cached != nil {
		cfg = compozyconfig.CloneConfig(&cached.resolved.Config)
	}
	r.mu.RUnlock()
	if cached != nil {
		return cfg, true, nil
	}
	loaded, err := r.loadWorkspaceConfig(ws.RootDir, profileName)
	if err != nil {
		return compozyconfig.Config{}, false, fmt.Errorf("workspace: load config for %q: %w", ws.RootDir, err)
	}
	return loaded, false, nil
}

func (r *Resolver) refreshSkillScanConfig(
	ctx context.Context,
	ws Workspace,
	profileName string,
	cached *compozyconfig.Config,
	scan workspaceScan,
) (compozyconfig.Config, workspaceScan, error) {
	loaded, err := r.loadWorkspaceConfig(ws.RootDir, profileName)
	if err != nil {
		return compozyconfig.Config{}, workspaceScan{}, fmt.Errorf(
			"workspace: load config for %q: %w",
			ws.RootDir,
			err,
		)
	}
	if cached != nil && sameSkillSourceConfig(cached.Skills, loaded.Skills) {
		return loaded, scan, nil
	}
	refreshed, err := r.scanWorkspace(ctx, ws, profileName, &loaded.Skills)
	if err != nil {
		return compozyconfig.Config{}, workspaceScan{}, err
	}
	return loaded, refreshed, nil
}

func (r *Resolver) loadWorkspaceConfig(workspaceRoot string, profileName string) (compozyconfig.Config, error) {
	if err := ValidatePersonalProfileRoot(
		workspaceRoot,
		r.homePaths.HomeDir,
		r.homePaths.ProfilesDir,
		profileName,
	); err != nil {
		return compozyconfig.Config{}, err
	}
	if strings.TrimSpace(profileName) != "" && r.loadProfileConfig != nil {
		return r.loadProfileConfig(workspaceRoot, profileName)
	}
	return r.loadConfig(workspaceRoot)
}

func sameSkillSourceConfig(left compozyconfig.SkillsConfig, right compozyconfig.SkillsConfig) bool {
	return slices.Equal(left.Sources, right.Sources) && slices.Equal(left.CustomSources, right.CustomSources)
}
