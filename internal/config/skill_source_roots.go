package config

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/resources"
)

// RootKind identifies the physical convention that produced one skill root.
type RootKind string

const (
	RootKindBuiltin RootKind = "builtin"
	RootKindPreset  RootKind = "preset"
	RootKindCustom  RootKind = "custom"
)

// SkillRootSpec is the typed, profile-aware identity of one skill root.
type SkillRootSpec struct {
	Dir           string
	SourceSlug    string
	Kind          RootKind
	ResourceScope resources.ResourceScope
	ProfileID     string
	WorkspaceID   string
}

// RootID returns an opaque identity derived only from stable ownership and location fields.
func (s SkillRootSpec) RootID() string {
	parts := []string{
		string(s.ResourceScope.Kind.Normalize()),
		strings.TrimSpace(s.ProfileID),
		strings.TrimSpace(s.WorkspaceID),
		string(s.Kind),
		canonicalSkillSourcePath(s.Dir),
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "root_" + hex.EncodeToString(digest[:16])
}

// SkillsDirs resolves configured roots owned by this discovery layer.
func (r WorkspaceDiscoveryRoot) SkillsDirs(cfg *SkillsConfig) []SkillRootSpec {
	if r.Source == WorkspaceDiscoverySourceGlobal {
		return ResolveGlobalSkillRoots(cfg, HomePaths{
			SkillsDir: filepath.Join(r.Dir, SkillsDirName), OperatorHomeDir: r.OperatorHomeDir,
		})
	}
	settings := effectiveSkillsConfig(cfg)
	scope := r.skillResourceScope()
	roots := make([]SkillRootSpec, 0, len(settings.Sources)+len(settings.CustomSources)+1)

	if r.Source != WorkspaceDiscoverySourceProfile && r.Source != WorkspaceDiscoverySourceWorkspaceProfile {
		for _, slug := range settings.Sources {
			preset, ok := skillSourcePreset(slug)
			if !ok || preset.AlwaysOn || strings.TrimSpace(preset.WorkspaceRel) == "" {
				continue
			}
			roots = append(roots, r.skillRootSpec(filepath.Join(r.Dir, filepath.FromSlash(preset.WorkspaceRel)), slug, RootKindPreset, scope))
		}
	}

	resolvedCustom := make([]string, 0, len(settings.CustomSources))
	for _, path := range settings.CustomSources {
		dir := resolveCustomSkillSourcePath(path, r.Dir, scope.Kind)
		if dir == "" {
			continue
		}
		resolvedCustom = append(resolvedCustom, dir)
	}
	slugs := CustomSourceSlugs(resolvedCustom)
	for _, dir := range resolvedCustom {
		roots = append(roots, r.skillRootSpec(dir, slugs[canonicalSkillSourcePath(dir)], RootKindCustom, scope))
	}

	compozyDir := filepath.Join(r.Dir, SkillsDirName)
	if !r.usesHomeResourceLayout() {
		compozyDir = filepath.Join(r.Dir, DirName, SkillsDirName)
	}
	return append(roots, r.skillRootSpec(compozyDir, SkillSourceCompozy, RootKindBuiltin, scope))
}

// ResolveGlobalSkillRoots resolves the user-owned global roots in effective config order.
func ResolveGlobalSkillRoots(cfg *SkillsConfig, home HomePaths) []SkillRootSpec {
	settings := effectiveSkillsConfig(cfg)
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	roots := make([]SkillRootSpec, 0, len(settings.Sources)+len(settings.CustomSources)+1)

	for _, slug := range settings.Sources {
		preset, ok := skillSourcePreset(slug)
		if !ok || preset.AlwaysOn || strings.TrimSpace(preset.GlobalPath) == "" {
			continue
		}
		roots = append(roots, SkillRootSpec{
			Dir: resolveGlobalSkillSourcePath(preset.GlobalPath, home), SourceSlug: slug,
			Kind: RootKindPreset, ResourceScope: scope,
		})
	}

	resolvedCustom := make([]string, 0, len(settings.CustomSources))
	for _, path := range settings.CustomSources {
		resolvedCustom = append(resolvedCustom, resolveGlobalSkillSourcePath(path, home))
	}
	slugs := CustomSourceSlugs(resolvedCustom)
	for _, path := range resolvedCustom {
		canonical := canonicalSkillSourcePath(path)
		if canonical == "" || !filepath.IsAbs(canonical) {
			continue
		}
		roots = append(roots, SkillRootSpec{
			Dir: canonical, SourceSlug: slugs[canonical], Kind: RootKindCustom, ResourceScope: scope,
		})
	}

	return append(roots, SkillRootSpec{
		Dir: canonicalSkillSourcePath(home.SkillsDir), SourceSlug: SkillSourceCompozy,
		Kind: RootKindBuiltin, ResourceScope: scope,
	})
}

func resolveGlobalSkillSourcePath(path string, home HomePaths) string {
	clean := strings.TrimSpace(path)
	if clean == "~" {
		return canonicalSkillSourcePath(home.OperatorHomeDir)
	}
	if strings.HasPrefix(clean, "~/") && strings.TrimSpace(home.OperatorHomeDir) != "" {
		return canonicalSkillSourcePath(filepath.Join(home.OperatorHomeDir, clean[2:]))
	}
	return canonicalSkillSourcePath(clean)
}

func effectiveSkillsConfig(cfg *SkillsConfig) SkillsConfig {
	if cfg != nil {
		return *cfg
	}
	return SkillsConfig{Sources: []string{SkillSourceAgents}}
}

func (r WorkspaceDiscoveryRoot) skillResourceScope() resources.ResourceScope {
	switch r.Source {
	case WorkspaceDiscoverySourceGlobal:
		return resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	case WorkspaceDiscoverySourceProfile:
		return resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: strings.TrimSpace(r.ProfileID)}
	case WorkspaceDiscoverySourceWorkspaceProfile:
		return resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspaceProfile, ID: strings.TrimSpace(r.ResourceScopeID)}
	default:
		return resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: strings.TrimSpace(r.WorkspaceID)}
	}
}

func (r WorkspaceDiscoveryRoot) skillRootSpec(
	dir string,
	slug string,
	kind RootKind,
	scope resources.ResourceScope,
) SkillRootSpec {
	return SkillRootSpec{
		Dir: canonicalSkillSourcePath(dir), SourceSlug: slug, Kind: kind, ResourceScope: scope,
		ProfileID: strings.TrimSpace(r.ProfileID), WorkspaceID: strings.TrimSpace(r.WorkspaceID),
	}
}

func resolveCustomSkillSourcePath(path string, workspaceRoot string, scope resources.ResourceScopeKind) string {
	expanded := canonicalSkillSourcePath(path)
	if expanded == "" {
		return ""
	}
	if filepath.IsAbs(expanded) || scope == resources.ResourceScopeKindUser || scope == resources.ResourceScopeKindProfile {
		return expanded
	}
	return canonicalSkillSourcePath(filepath.Join(workspaceRoot, path))
}

func canonicalSkillSourcePath(path string) string {
	expanded, err := expandUserPath(path)
	if err != nil {
		expanded = strings.TrimSpace(path)
	}
	if expanded == "" {
		return ""
	}
	absolute, err := filepath.Abs(expanded)
	if err != nil {
		return filepath.Clean(expanded)
	}
	canonical, err := resolveCanonicalDirIfExists(absolute)
	if err != nil {
		return filepath.Clean(absolute)
	}
	return filepath.Clean(canonical)
}
