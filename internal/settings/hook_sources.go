package settings

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
)

type hookSourceEntry struct {
	declaration hookspkg.HookDecl
	source      SourceRef
	target      WriteTargetKind
	writable    bool
}

func (s *service) buildHookItems(
	scope ScopeKind,
	workspaceID string,
	profileName string,
	workspaceRoot string,
) ([]HookItem, error) {
	sources, err := s.loadHookSources(scope, workspaceID, profileName, workspaceRoot)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]HookItem, 0, len(names))
	for _, name := range names {
		entries := sources[name]
		if len(entries) == 0 {
			continue
		}
		winner := entries[len(entries)-1]
		metadata := SourceMetadata{EffectiveSource: winner.source}
		for index := range entries[:len(entries)-1] {
			metadata.ShadowedSources = append(metadata.ShadowedSources, entries[index].source)
		}
		metadata.AvailableTargets = writableHookTargets(entries)
		items = append(items, cloneHookItem(&HookItem{
			Name: name, Declaration: winner.declaration, SourceMetadata: metadata,
		}))
	}
	return items, nil
}

func (s *service) loadHookSources(
	scope ScopeKind,
	workspaceID string,
	profileName string,
	workspaceRoot string,
) (map[string][]hookSourceEntry, error) {
	sources := make(map[string][]hookSourceEntry)
	appendLayer := func(path string, target WriteTargetKind, writable bool) error {
		declarations, err := compozyconfig.LoadHookDeclarationsFile(path)
		if err != nil {
			return err
		}
		for _, declaration := range declarations {
			name := strings.TrimSpace(declaration.Name)
			if name == "" {
				continue
			}
			sources[name] = append(sources[name], hookSourceEntry{
				declaration: declaration,
				source:      sourceRefForWriteTarget(target, workspaceID, profileName),
				target:      target,
				writable:    writable,
			})
		}
		return nil
	}

	if err := appendLayer(s.homePaths.ConfigFile, WriteTargetGlobalConfig, true); err != nil {
		return nil, fmt.Errorf("settings: load user hook declarations: %w", err)
	}
	if scope == ScopeProfile {
		profilePath := filepath.Join(s.homePaths.ProfilesDir, profileName, compozyconfig.ConfigName)
		if err := appendLayer(profilePath, WriteTargetProfileConfig, true); err != nil {
			return nil, fmt.Errorf("settings: load profile hook declarations: %w", err)
		}
	}
	if scope == ScopeWorkspace || (scope == ScopeProfile && strings.TrimSpace(workspaceRoot) != "") {
		if err := appendLayer(workspaceConfigPath(workspaceRoot), WriteTargetWorkspaceConfig, true); err != nil {
			return nil, fmt.Errorf("settings: load workspace hook declarations: %w", err)
		}
		if scope == ScopeProfile {
			workspaceProfilePath := filepath.Join(
				workspaceRoot,
				compozyconfig.DirName,
				compozyconfig.ProfilesDirName,
				profileName,
				compozyconfig.ConfigName,
			)
			if err := appendLayer(workspaceProfilePath, WriteTargetWorkspaceProfileConfig, false); err != nil {
				return nil, fmt.Errorf("settings: load workspace-profile hook declarations: %w", err)
			}
		}
	}
	return sources, nil
}

func writableHookTargets(entries []hookSourceEntry) []WriteTargetKind {
	targets := make([]WriteTargetKind, 0, len(entries))
	seen := make(map[WriteTargetKind]struct{}, len(entries))
	for index := range entries {
		entry := &entries[index]
		if !entry.writable {
			continue
		}
		if _, ok := seen[entry.target]; ok {
			continue
		}
		seen[entry.target] = struct{}{}
		targets = append(targets, entry.target)
	}
	return targets
}
