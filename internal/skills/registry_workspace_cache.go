package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/skillscan"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type wsCache struct {
	skills            map[string]*Skill
	commandCandidates []*Skill
	diagnostics       []SkillDiagnostic
	snapshots         map[string]filesnap.Snapshot
	rootPaths         []string
	lastAccess        time.Time
	globalVersion     int64
}

type workspaceLoad struct {
	paths     []workspaceSkillPath
	snapshots map[string]filesnap.Snapshot
	rootPaths []string
}

type workspaceSkillPath struct {
	filePath string
	root     compozyconfig.SkillRootSpec
}

func (r *Registry) workspaceDisabledSkillsSnapshot(cacheKey string, configured []string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	disabledSkills := mergeDisabledSkills(r.cfg.DisabledSkills, configured)
	if cacheKey == "" {
		return disabledSkills
	}
	return mergeDisabledSkills(disabledSkills, r.workspaceDisabled[cacheKey])
}

func workspaceConfiguredDisabledSkills(resolved *workspacepkg.ResolvedWorkspace) []string {
	if resolved == nil {
		return nil
	}
	return resolved.Config.Skills.DisabledSkills
}

func (r *Registry) workspaceSkillTargetLocked(name string, resolved *workspacepkg.ResolvedWorkspace) (string, *Skill) {
	if resolved == nil {
		return "", nil
	}

	cacheKey := workspaceCacheKey(resolved)
	if cacheKey == "" {
		return "", nil
	}

	cached := r.wsCache[cacheKey]
	if cached == nil {
		return cacheKey, nil
	}

	return cacheKey, cached.skills[name]
}

func (r *Registry) workspaceLoadFromResolved(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) (workspaceLoad, error) {
	if resolved == nil {
		return workspaceLoad{}, nil
	}
	return r.workspaceLoadFromRoots(ctx, resolved)
}

func (r *Registry) workspaceLoadFromRoots(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) (workspaceLoad, error) {
	cfg := r.registryConfigSnapshot(ctx)
	roots := rootsNotOwnedByHigherLayer(workspaceResolvedSkillRoots(resolved), cfg.GlobalSkillRoots)
	if len(roots) == 0 {
		return workspaceLoad{}, nil
	}
	load := workspaceLoad{
		paths:     make([]workspaceSkillPath, 0),
		snapshots: make(map[string]filesnap.Snapshot),
		rootPaths: make([]string, 0, len(roots)),
	}
	trustedRoots := make([]string, 0, len(roots)+len(cfg.GlobalSkillRoots))
	for _, root := range cfg.GlobalSkillRoots {
		trustedRoots = append(trustedRoots, root.Dir)
	}
	for _, root := range roots {
		load.rootPaths = append(load.rootPaths, canonicalRootIdentity(root.Dir))
		trustedRoots = append(trustedRoots, root.Dir)
	}
	scans := make([]configuredRootScan, 0, len(roots))
	for _, root := range roots {
		if err := checkRegistryContext(ctx); err != nil {
			return workspaceLoad{}, fmt.Errorf(
				"skills: check registry context while loading workspace skill roots: %w",
				err,
			)
		}
		result, err := skillscan.ScanDirectoryWithin(root.Dir, trustedRoots)
		if err != nil {
			return workspaceLoad{}, err
		}
		if err := r.emitSkillScanEvents(ctx, root, result.Stats); err != nil {
			return workspaceLoad{}, err
		}
		maps.Copy(load.snapshots, result.Snapshots)
		if result.Stats.Exists {
			rootSnapshot, snapshotErr := filesnap.FromPath(root.Dir)
			if snapshotErr != nil {
				return workspaceLoad{}, snapshotErr
			}
			load.snapshots[root.Dir] = rootSnapshot
		}
		for _, skillPath := range result.Paths {
			skillDir := filepath.Dir(skillPath)
			dirSnapshot, snapshotErr := filesnap.FromPath(skillDir)
			if snapshotErr != nil {
				return workspaceLoad{}, snapshotErr
			}
			load.snapshots[skillDir] = dirSnapshot
		}
		if err := recordSidecarSnapshots(result.Paths, load.snapshots); err != nil {
			return workspaceLoad{}, err
		}
		scans = append(scans, configuredRootScan{spec: root, result: result})
	}
	selected := selectedRootPaths(scans)
	for index, scan := range scans {
		for _, path := range selected[index] {
			load.paths = append(load.paths, workspaceSkillPath{
				filePath: path,
				root:     scan.spec,
			})
		}
	}
	return load, nil
}

func (r *Registry) cachedWorkspaceSkillsIfFresh(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	cacheKey string,
	disabled []string,
) ([]*Skill, bool) {
	if cacheKey == "" {
		return nil, false
	}
	cfg := r.registryConfigSnapshot(ctx)
	roots := rootsNotOwnedByHigherLayer(workspaceResolvedSkillRoots(resolved), cfg.GlobalSkillRoots)
	rootPaths := make([]string, 0, len(roots))
	for _, root := range roots {
		rootPaths = append(rootPaths, canonicalRootIdentity(root.Dir))
	}
	now := r.now()

	r.mu.RLock()
	cached := r.wsCache[cacheKey]
	if cached == nil || cached.lastAccess.Before(now.Add(-workspaceCacheTTL)) ||
		cached.globalVersion != r.globalVersion.Load() || !slices.Equal(cached.rootPaths, rootPaths) {
		r.mu.RUnlock()
		return nil, false
	}
	snapshots := filesnap.Clone(cached.snapshots)
	r.mu.RUnlock()

	current := make(map[string]filesnap.Snapshot, len(snapshots))
	for path := range snapshots {
		snapshot, err := filesnap.FromPath(path)
		if err != nil {
			return nil, false
		}
		current[path] = snapshot
	}
	if !filesnap.Equal(snapshots, current) {
		return nil, false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	cached = r.wsCache[cacheKey]
	if cached == nil || cached.globalVersion != r.globalVersion.Load() || !filesnap.Equal(cached.snapshots, snapshots) {
		return nil, false
	}
	cached.lastAccess = now
	return mergedSkillListWithDisabled(r.globalSkills, cached.skills, disabled), true
}

func rootsNotOwnedByHigherLayer(
	roots []compozyconfig.SkillRootSpec,
	higher []compozyconfig.SkillRootSpec,
) []compozyconfig.SkillRootSpec {
	owned := make(map[string]struct{}, len(higher))
	for _, root := range higher {
		owned[canonicalRootIdentity(root.Dir)] = struct{}{}
	}
	filtered := make([]compozyconfig.SkillRootSpec, 0, len(roots))
	for _, root := range roots {
		identity := canonicalRootIdentity(root.Dir)
		if _, exists := owned[identity]; exists {
			continue
		}
		owned[identity] = struct{}{}
		filtered = append(filtered, root)
	}
	return filtered
}

func canonicalRootIdentity(path string) string {
	canonical, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err == nil {
		return canonical
	}
	return filepath.Clean(path)
}

func workspaceResolvedSkillRoots(resolved *workspacepkg.ResolvedWorkspace) []compozyconfig.SkillRootSpec {
	if resolved == nil {
		return nil
	}
	discoveryRoots := compozyconfig.WorkspaceDiscoveryRoots(
		resolved.RootDir,
		resolved.AdditionalDirs,
		compozyconfig.HomePaths{ProfilesDir: filepath.Dir(resolved.ProfileRoot)},
		resolved.ProfileName,
	)
	roots := make([]compozyconfig.SkillRootSpec, 0, len(discoveryRoots)*2)
	for _, discoveryRoot := range slices.Backward(discoveryRoots) {
		if discoveryRoot.Source == compozyconfig.WorkspaceDiscoverySourceGlobal {
			continue
		}
		discoveryRoot.ProfileID = strings.TrimSpace(resolved.ProfileID)
		discoveryRoot.WorkspaceID = resourceWorkspaceKey(resolved)
		discoveryRoot.ResourceScopeID = discoveryRoot.WorkspaceID + "@pf:" + strings.TrimSpace(resolved.ProfileName)
		roots = append(roots, discoveryRoot.SkillsDirs(&resolved.Config.Skills)...)
	}
	return roots
}

func (r *Registry) evictExpiredWorkspaceLocked(now time.Time) {
	cutoff := now.Add(-workspaceCacheTTL)
	for workspace, entry := range r.wsCache {
		if entry.lastAccess.Before(cutoff) {
			delete(r.wsCache, workspace)
		}
	}
}

func workspaceCacheKey(resolved *workspacepkg.ResolvedWorkspace) string {
	if resolved == nil {
		return ""
	}
	profileSuffix := ""
	if profileID := strings.TrimSpace(resolved.ProfileID); profileID != "" {
		profileSuffix = "@pf:" + profileID
	}
	sourceGeneration := workspaceSourceConfigGeneration(resolved)
	if workspaceID := strings.TrimSpace(resolved.WorkspaceID); workspaceID != "" {
		return "id:" + workspaceID + profileSuffix + sourceGeneration
	}
	if id := strings.TrimSpace(resolved.ID); id != "" {
		return "id:" + id + profileSuffix + sourceGeneration
	}
	if profileID := strings.TrimSpace(resolved.ProfileID); profileID != "" {
		return "profile:" + profileID + sourceGeneration
	}
	if root := strings.TrimSpace(resolved.RootDir); root != "" {
		return "root:" + root + profileSuffix + sourceGeneration
	}
	return ""
}

func workspaceSourceConfigGeneration(resolved *workspacepkg.ResolvedWorkspace) string {
	if resolved == nil {
		return ""
	}
	parts := make([]string, 0, len(resolved.Config.Skills.Sources)+len(resolved.Config.Skills.CustomSources)+1)
	parts = append(parts, resolved.Config.Skills.Sources...)
	parts = append(parts, "\x01")
	parts = append(parts, resolved.Config.Skills.CustomSources...)
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "@src:" + hex.EncodeToString(digest[:8])
}
