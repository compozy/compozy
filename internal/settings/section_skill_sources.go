package settings

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	skillspkg "github.com/compozy/compozy/internal/skills"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (s *service) skillProjectionWorkspace(
	ctx context.Context,
	cfg *compozyconfig.Config,
	resolved *workspacepkg.ResolvedWorkspace,
	scope ScopeKind,
	profileName string,
) (*workspacepkg.ResolvedWorkspace, error) {
	if scope == ScopeUser && resolved == nil {
		return nil, nil
	}
	projection := workspacepkg.ResolvedWorkspace{}
	if resolved != nil {
		projection = *resolved
	}
	if cfg != nil {
		projection.Config = compozyconfig.CloneConfig(cfg)
	}
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return &projection, nil
	}
	profileID, err := s.profileResolver.AvailableProfileID(ctx, profileName)
	if err != nil {
		return nil, fmt.Errorf("settings: resolve skill source profile %q: %w", profileName, err)
	}
	projection.ProfileName = profileName
	projection.ProfileID = strings.TrimSpace(profileID)
	projection.ProfileRoot = filepath.Join(s.homePaths.ProfilesDir, profileName)
	return &projection, nil
}

func (s *service) buildSkillSourceReadModel(
	cfg compozyconfig.SkillsConfig,
	resolved *workspacepkg.ResolvedWorkspace,
	scope ScopeKind,
	statuses []skillspkg.SkillSourceRootStatus,
	runtimeAvailable bool,
) ([]SkillSourceItem, *SkillSourceInheritance, error) {
	items := presetSkillSourceItems(cfg)
	indexBySlug := make(map[string]int, len(items)+len(statuses))
	for index := range items {
		indexBySlug[items[index].Slug] = index
	}
	for _, status := range statuses {
		slug := strings.TrimSpace(status.Spec.SourceSlug)
		index, ok := indexBySlug[slug]
		if !ok {
			item := customSkillSourceItem(slug, status.Spec.Dir)
			items = append(items, item)
			index = len(items) - 1
			indexBySlug[slug] = index
		}
		items[index].Roots = append(items[index].Roots, skillSourceRootItem(status, runtimeAvailable))
	}
	appendUnmeasuredCustomSources(&items, indexBySlug, cfg.CustomSources)

	inherits, err := s.skillSourceInheritance(resolved, scope)
	if err != nil {
		return nil, nil, err
	}
	return items, inherits, nil
}

func presetSkillSourceItems(cfg compozyconfig.SkillsConfig) []SkillSourceItem {
	presets := compozyconfig.SkillSourcePresets()
	items := make([]SkillSourceItem, 0, len(presets))
	for _, preset := range presets {
		kind := string(compozyconfig.RootKindPreset)
		if preset.AlwaysOn {
			kind = string(compozyconfig.RootKindBuiltin)
		}
		items = append(items, SkillSourceItem{
			Slug: preset.Slug, Label: preset.Label, Kind: kind,
			Enabled:  preset.AlwaysOn || slices.Contains(cfg.Sources, preset.Slug),
			AlwaysOn: preset.AlwaysOn, DefaultOn: preset.DefaultOn,
			WorkspacePath: preset.WorkspaceRel, GlobalPath: preset.GlobalPath,
			Roots: []SkillSourceRootItem{},
		})
	}
	return items
}

func customSkillSourceItem(slug string, path string) SkillSourceItem {
	return SkillSourceItem{
		Slug: slug, Label: slug, Kind: string(compozyconfig.RootKindCustom),
		Enabled: true, Path: path, Roots: []SkillSourceRootItem{},
	}
}

func appendUnmeasuredCustomSources(
	items *[]SkillSourceItem,
	indexBySlug map[string]int,
	paths []string,
) {
	for _, path := range paths {
		slug := compozyconfig.CustomSourceSlug(path, paths)
		if _, exists := indexBySlug[slug]; exists {
			continue
		}
		*items = append(*items, customSkillSourceItem(slug, path))
		indexBySlug[slug] = len(*items) - 1
	}
}

func skillSourceRootItem(
	status skillspkg.SkillSourceRootStatus,
	runtimeAvailable bool,
) SkillSourceRootItem {
	item := SkillSourceRootItem{
		RootID: status.Spec.RootID(), Path: status.Spec.Dir,
		Exists: status.Exists, Readable: status.Readable, Truncated: status.Truncated,
		SkippedLinks:  append([]skillspkg.SkillSourceSkippedLink(nil), status.SkippedLinks...),
		Collisions:    append([]skillspkg.SkillSourceCollision(nil), status.Collisions...),
		Verification:  status.Verification,
		NativeReaders: append([]string(nil), status.NativeReaders...),
	}
	if runtimeAvailable && (!status.Exists || status.Readable) {
		scanned := status.ScannedCount
		skills := status.SkillCount
		item.ScannedCount = &scanned
		item.SkillCount = &skills
	}
	return item
}

func (s *service) skillSourceInheritance(
	resolved *workspacepkg.ResolvedWorkspace,
	scope ScopeKind,
) (*SkillSourceInheritance, error) {
	if scope != ScopeWorkspace || resolved == nil {
		return nil, nil
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(
		s.homePaths,
		resolved.RootDir,
		compozyconfig.WriteScopeWorkspace,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("settings: resolve workspace skill source inheritance: %w", err)
	}
	presence, err := compozyconfig.ReadSkillSourceOverridePresence(target.Path())
	if err != nil {
		return nil, fmt.Errorf("settings: read workspace skill source inheritance: %w", err)
	}
	return &SkillSourceInheritance{
		Sources: !presence.Sources, CustomSources: !presence.CustomSources,
	}, nil
}
