package workspace

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/skillscan"
)

type workspaceSkillScan struct {
	discovery skillscan.DirectoryResult
	names     map[string]string
}

func (r *Resolver) cachedSkillSources(workspaceID, profileName string) map[string]workspaceSkillScan {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cached := r.cache[workspaceProfileCacheKey(workspaceID, profileName)]
	if cached != nil && !cached.lastAccess.Before(r.now().Add(-r.cacheTTL)) {
		return cached.skillSources
	}
	return nil
}

func (scan *workspaceScan) scanSkillSource(
	ctx context.Context,
	root workspaceSkillRoot,
	rootOrder int,
	trustedRoots []string,
	previous workspaceSkillScan,
) error {
	skillsDir := root.spec.Dir
	if err := addSnapshotIfExists(skillsDir, scan.snapshots); err != nil {
		return fmt.Errorf("workspace: snapshot skills directory %q: %w", skillsDir, err)
	}
	current := previous
	if !previous.discovery.Unchanged(ctx, skillsDir, trustedRoots) {
		fresh, err := scanWorkspaceSkillSource(skillsDir, trustedRoots)
		if err != nil {
			return err
		}
		current = fresh
	}
	scan.skillSources[skillsDir] = current
	maps.Copy(scan.snapshots, current.discovery.Snapshots)
	for _, skillFile := range current.discovery.Paths {
		if err := checkContext(ctx); err != nil {
			return err
		}
		name, valid := current.names[skillFile]
		if !valid {
			continue
		}
		skillDir := filepath.Dir(skillFile)
		if err := addSnapshotIfExists(skillDir, scan.snapshots); err != nil {
			return fmt.Errorf("workspace: snapshot skill directory %q: %w", skillDir, err)
		}
		if err := addSnapshotIfExists(filepath.Join(skillDir, compozyconfig.MCPJSONName), scan.snapshots); err != nil {
			return fmt.Errorf("workspace: snapshot skill MCP sidecar %q: %w", skillDir, err)
		}
		scan.skills = append(scan.skills, skillCandidate{
			name: name, dir: skillDir, source: root.source, rootOrder: rootOrder,
		})
	}
	return nil
}

func scanWorkspaceSkillSource(skillsDir string, trustedRoots []string) (workspaceSkillScan, error) {
	result, err := skillscan.ScanDirectoryWithin(skillsDir, trustedRoots)
	if err != nil {
		return workspaceSkillScan{}, fmt.Errorf("workspace: scan skills directory %q: %w", skillsDir, err)
	}
	names := make(map[string]string, len(result.Paths))
	for _, skillFile := range result.Paths {
		name, err := loadWorkspaceSkillName(skillFile)
		if err != nil {
			if errors.Is(err, errInvalidWorkspaceSkillDefinition) {
				continue
			}
			return workspaceSkillScan{}, fmt.Errorf("workspace: load skill identity %q: %w", skillFile, err)
		}
		names[skillFile] = name
	}
	return workspaceSkillScan{discovery: result, names: names}, nil
}
