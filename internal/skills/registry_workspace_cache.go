package skills

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/filesnap"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type wsCache struct {
	skills        map[string]*Skill
	diagnostics   []SkillDiagnostic
	snapshots     map[string]filesnap.Snapshot
	lastAccess    time.Time
	globalVersion int64
}

type workspaceLoad struct {
	paths     []workspaceSkillPath
	snapshots map[string]filesnap.Snapshot
}

type workspaceSkillPath struct {
	filePath string
	source   SkillSource
}

type workspaceSkillRoot struct {
	dir    string
	source SkillSource
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

	paths, ok := workspaceCacheKeyPaths(resolved)
	if !ok {
		return "", nil
	}

	cacheKey := workspaceCacheKey(resolved, paths)
	if cacheKey == "" {
		return "", nil
	}

	cached := r.wsCache[cacheKey]
	if cached == nil {
		return cacheKey, nil
	}

	return cacheKey, cached.skills[name]
}

func (r *Registry) cachedWorkspaceSkillsFromResolved(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]*Skill, bool, error) {
	paths, ok := workspaceCacheKeyPaths(resolved)
	if !ok || len(paths) == 0 {
		return nil, false, nil
	}
	cacheKey := workspaceCacheKey(resolved, paths)
	if cacheKey == "" {
		return nil, false, nil
	}
	now := r.now()
	currentGlobalVersion := r.GlobalVersion()

	r.mu.Lock()
	cached := r.wsCache[cacheKey]
	if cached == nil || cached.globalVersion != currentGlobalVersion {
		r.mu.Unlock()
		return nil, false, nil
	}
	r.evictExpiredWorkspaceLocked(now)
	cached = r.wsCache[cacheKey]
	if cached == nil || cached.globalVersion != currentGlobalVersion {
		r.mu.Unlock()
		return nil, false, nil
	}
	r.mu.Unlock()

	snapshots, complete, err := workspaceSnapshotsForExplicitPaths(ctx, paths)
	if err != nil || !complete {
		return nil, false, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	cached = r.wsCache[cacheKey]
	if cached == nil ||
		cached.globalVersion != currentGlobalVersion ||
		!filesnap.Equal(cached.snapshots, snapshots) {
		return nil, false, nil
	}
	cached.lastAccess = now
	disabledSkills := mergeDisabledSkills(r.cfg.DisabledSkills, workspaceConfiguredDisabledSkills(resolved))
	disabledSkills = mergeDisabledSkills(disabledSkills, r.workspaceDisabled[cacheKey])
	return mergedSkillListWithDisabled(r.globalSkills, cached.skills, disabledSkills), true, nil
}

func workspaceSnapshotsForExplicitPaths(
	ctx context.Context,
	paths []workspaceSkillPath,
) (map[string]filesnap.Snapshot, bool, error) {
	snapshots := make(map[string]filesnap.Snapshot, len(paths))
	for _, path := range paths {
		if err := checkRegistryContext(ctx); err != nil {
			return nil, false, err
		}

		snapshot, err := filesnap.FromPath(path.filePath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return snapshots, false, nil
			}
			return nil, false, fmt.Errorf("skills: snapshot workspace skill %q: %w", path.filePath, err)
		}
		snapshots[path.filePath] = snapshot

		for _, sidecarPath := range []string{
			filepath.Join(filepath.Dir(path.filePath), sidecarFileName),
			filepath.Join(filepath.Dir(path.filePath), aghconfig.MCPJSONName),
		} {
			sidecarSnapshot, err := filesnap.FromPath(sidecarPath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return nil, false, fmt.Errorf("skills: snapshot workspace skill sidecar %q: %w", sidecarPath, err)
			}
			snapshots[sidecarPath] = sidecarSnapshot
		}
	}
	return snapshots, true, nil
}

func (r *Registry) workspaceLoadFromResolved(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) (workspaceLoad, error) {
	if resolved == nil {
		return workspaceLoad{}, nil
	}

	if load, ok, err := workspaceLoadFromRoots(ctx, resolved); ok || err != nil {
		return load, err
	}

	load := workspaceLoad{
		paths:     make([]workspaceSkillPath, 0, len(resolved.Skills)),
		snapshots: make(map[string]filesnap.Snapshot, len(resolved.Skills)),
	}

	for _, skillPath := range resolved.Skills {
		if err := checkRegistryContext(ctx); err != nil {
			return workspaceLoad{}, fmt.Errorf("skills: check registry context while loading workspace skills: %w", err)
		}

		path, include, err := workspaceSkillLoadPath(skillPath)
		if err != nil {
			return workspaceLoad{}, err
		}
		if !include {
			continue
		}

		snapshot, err := filesnap.FromPath(path.filePath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return workspaceLoad{}, fmt.Errorf("skills: snapshot workspace skill %q: %w", path.filePath, err)
		}

		load.snapshots[path.filePath] = snapshot
		for _, sidecarPath := range []string{
			filepath.Join(filepath.Dir(path.filePath), sidecarFileName),
			filepath.Join(filepath.Dir(path.filePath), aghconfig.MCPJSONName),
		} {
			sidecarSnapshot, err := filesnap.FromPath(sidecarPath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return workspaceLoad{}, fmt.Errorf("skills: snapshot workspace skill sidecar %q: %w", sidecarPath, err)
			}
			load.snapshots[sidecarPath] = sidecarSnapshot
		}
		load.paths = append(load.paths, path)
	}

	return load, nil
}

func workspaceLoadFromRoots(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) (workspaceLoad, bool, error) {
	roots := workspaceSkillRoots(resolved)
	if len(roots) == 0 {
		return workspaceLoad{}, false, nil
	}
	if !workspaceSkillPathsMatchRoots(resolved.Skills, roots) {
		return workspaceLoad{}, false, nil
	}

	load := workspaceLoad{
		paths:     make([]workspaceSkillPath, 0),
		snapshots: make(map[string]filesnap.Snapshot),
	}

	// Load lower-precedence roots first so later overlays preserve the
	// documented workspace > additional ordering and emit shadow audits.
	for _, root := range slices.Backward(roots) {
		if err := checkRegistryContext(ctx); err != nil {
			return workspaceLoad{}, true, fmt.Errorf(
				"skills: check registry context while loading workspace skill roots: %w",
				err,
			)
		}

		paths, dirSnapshots, err := scanDirectoryWithSnapshots(root.dir)
		if err != nil {
			return workspaceLoad{}, true, err
		}
		maps.Copy(load.snapshots, dirSnapshots)
		if err := recordSidecarSnapshots(paths, load.snapshots); err != nil {
			return workspaceLoad{}, true, err
		}
		for _, path := range paths {
			load.paths = append(load.paths, workspaceSkillPath{
				filePath: path,
				source:   root.source,
			})
		}
	}

	return load, true, nil
}

func workspaceSkillRoots(resolved *workspacepkg.ResolvedWorkspace) []workspaceSkillRoot {
	if resolved == nil {
		return nil
	}

	roots := make([]workspaceSkillRoot, 0, len(resolved.AdditionalDirs)+1)
	if root := strings.TrimSpace(resolved.RootDir); root != "" {
		roots = append(roots, workspaceSkillRoot{
			dir:    filepath.Join(root, aghconfig.DirName, aghconfig.SkillsDirName),
			source: SourceWorkspace,
		})
	}
	for _, additionalDir := range resolved.AdditionalDirs {
		if root := strings.TrimSpace(additionalDir); root != "" {
			roots = append(roots, workspaceSkillRoot{
				dir:    filepath.Join(root, aghconfig.DirName, aghconfig.SkillsDirName),
				source: SourceAdditional,
			})
		}
	}

	return roots
}

func workspaceSkillPathsMatchRoots(
	paths []workspacepkg.SkillPath,
	roots []workspaceSkillRoot,
) bool {
	for _, path := range paths {
		loadPath, include, err := workspaceSkillLoadPath(path)
		if err != nil {
			return false
		}
		if !include {
			continue
		}
		if !workspaceSkillPathMatchesAnyRoot(loadPath.filePath, roots) {
			return false
		}
	}
	return true
}

func workspaceSkillPathMatchesAnyRoot(path string, roots []workspaceSkillRoot) bool {
	for _, root := range roots {
		if workspaceSkillPathMatchesRoot(path, root.dir) {
			return true
		}
	}
	return false
}

func workspaceSkillPathMatchesRoot(path string, root string) bool {
	trimmedPath := strings.TrimSpace(path)
	trimmedRoot := strings.TrimSpace(root)
	if trimmedPath == "" || trimmedRoot == "" {
		return false
	}

	rel, err := filepath.Rel(trimmedRoot, trimmedPath)
	if err != nil {
		return false
	}
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func workspaceCacheKeyPaths(resolved *workspacepkg.ResolvedWorkspace) ([]workspaceSkillPath, bool) {
	if resolved == nil {
		return nil, true
	}
	paths := make([]workspaceSkillPath, 0, len(resolved.Skills))
	for _, skillPath := range resolved.Skills {
		path, include, err := workspaceSkillLoadPath(skillPath)
		if err != nil {
			return nil, false
		}
		if include {
			paths = append(paths, path)
		}
	}
	return paths, true
}

func workspaceSkillLoadPath(skillPath workspacepkg.SkillPath) (workspaceSkillPath, bool, error) {
	source, include, err := skillSourceFromWorkspacePath(skillPath.Source)
	if err != nil {
		return workspaceSkillPath{}, false, fmt.Errorf(
			"skills: resolve workspace skill source %q: %w",
			skillPath.Source,
			err,
		)
	}
	if !include {
		return workspaceSkillPath{}, false, nil
	}

	skillDir := strings.TrimSpace(skillPath.Dir)
	if skillDir == "" {
		return workspaceSkillPath{}, false, nil
	}

	return workspaceSkillPath{
		filePath: filepath.Join(skillDir, skillFileName),
		source:   source,
	}, true, nil
}

func (r *Registry) evictExpiredWorkspaceLocked(now time.Time) {
	cutoff := now.Add(-workspaceCacheTTL)
	for workspace, entry := range r.wsCache {
		if entry.lastAccess.Before(cutoff) {
			delete(r.wsCache, workspace)
		}
	}
}

func workspaceCacheKey(resolved *workspacepkg.ResolvedWorkspace, paths []workspaceSkillPath) string {
	if resolved == nil {
		return ""
	}
	if id := strings.TrimSpace(resolved.ID); id != "" {
		return "id:" + id
	}
	if root := strings.TrimSpace(resolved.RootDir); root != "" {
		return "root:" + root
	}
	if len(paths) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, path := range paths {
		if builder.Len() > 0 {
			builder.WriteByte('|')
		}
		builder.WriteString(skillSourceName(path.source))
		builder.WriteByte(':')
		builder.WriteString(path.filePath)
	}

	return builder.String()
}
