package workspace

import (
	"errors"
	"io/fs"
	"path/filepath"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
)

func (r *Resolver) freshSnapshot(key string, ws Workspace, workspaceID, profileName, profileID string) (ResolvedWorkspace, bool) {
	r.mu.RLock()
	now := r.now()
	cached := r.cache[key]
	fresh := cached != nil && r.snapshotTTL > 0 && !now.Before(cached.validatedAt) &&
		now.Sub(cached.validatedAt) < r.snapshotTTL && now.Sub(cached.lastAccess) < r.cacheTTL &&
		sameWorkspaceRuntimeInputs(cached.workspace, ws)
	r.mu.RUnlock()
	if !fresh || !r.snapshotConfigUnchanged(ws, profileName, cached.snapshots) {
		return ResolvedWorkspace{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cache[key] != cached {
		return ResolvedWorkspace{}, false
	}
	cached.lastAccess = now
	resolved := cloneResolvedWorkspace(&cached.resolved)
	resolved.Workspace = cloneWorkspace(ws)
	resolved.WorkspaceID, resolved.ProfileID = workspaceID, profileID
	return resolved, true
}

func (r *Resolver) snapshotConfigUnchanged(ws Workspace, profileName string, snapshots map[string]filesnap.Snapshot) bool {
	paths := []string{r.homePaths.ConfigFile, filepath.Join(r.homePaths.HomeDir, compozyconfig.MCPJSONName)}
	var roots []string
	if ws.RootDir != "" {
		roots = append(roots, filepath.Join(ws.RootDir, compozyconfig.DirName))
		paths = append(paths, compozyconfig.WorkspaceDotEnvFile(ws.RootDir))
	}
	if profileName != "" {
		roots = append(roots, filepath.Join(r.homePaths.ProfilesDir, profileName))
		if ws.RootDir != "" {
			roots = append(roots, filepath.Join(ws.RootDir, compozyconfig.DirName, compozyconfig.ProfilesDirName, profileName))
		}
	}
	for _, root := range roots {
		paths = append(paths, filepath.Join(root, compozyconfig.ConfigName), filepath.Join(root, compozyconfig.MCPJSONName))
	}
	for _, path := range paths {
		previous, found := snapshots[path]
		current, err := filesnap.FromPath(path)
		if errors.Is(err, fs.ErrNotExist) {
			if found && previous.Size != -1 {
				return false
			}
			continue
		}
		if err != nil || !found || !previous.Equal(current) {
			return false
		}
	}
	return true
}
