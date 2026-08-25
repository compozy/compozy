package workspace

import (
	"context"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (r *Resolver) configForSkillScan(
	ws Workspace,
	profileName string,
	cacheKey string,
) (compozyconfig.Config, bool, error) {
	r.mu.Lock()
	cached := r.cache[cacheKey]
	var cfg compozyconfig.Config
	if cached != nil {
		cfg = compozyconfig.CloneConfig(&cached.resolved.Config)
	}
	r.mu.Unlock()
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
	cached compozyconfig.Config,
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
	if sameSkillSourceConfig(cached.Skills, loaded.Skills) {
		return loaded, scan, nil
	}
	refreshed, err := r.scanWorkspace(ctx, ws, profileName, &loaded.Skills)
	if err != nil {
		return compozyconfig.Config{}, workspaceScan{}, err
	}
	return loaded, refreshed, nil
}

func (r *Resolver) loadWorkspaceConfig(workspaceRoot string, profileName string) (compozyconfig.Config, error) {
	if strings.TrimSpace(profileName) != "" && r.loadProfileConfig != nil {
		return r.loadProfileConfig(workspaceRoot, profileName)
	}
	return r.loadConfig(workspaceRoot)
}

func sameSkillSourceConfig(left compozyconfig.SkillsConfig, right compozyconfig.SkillsConfig) bool {
	return slices.Equal(left.Sources, right.Sources) && slices.Equal(left.CustomSources, right.CustomSources)
}
