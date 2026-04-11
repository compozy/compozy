package extensions

import (
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// DeclaredProvider captures one provider declaration extracted from a manifest.
type DeclaredProvider struct {
	Extension    Ref
	ManifestPath string
	ExtensionDir string
	Manifest     *Manifest
	ProviderEntry
}

// DeclaredProviders groups provider declarations by manifest category.
type DeclaredProviders struct {
	IDE    []DeclaredProvider
	Review []DeclaredProvider
	Model  []DeclaredProvider
}

// DeclaredSkillPack captures one resolved skill-pack path from a manifest.
type DeclaredSkillPack struct {
	Extension    Ref
	ManifestPath string
	Pattern      string
	ResolvedPath string
	SourceFS     fs.FS
	SourceDir    string
}

// DeclaredSkillPacks contains the resolved skill-pack inventory.
type DeclaredSkillPacks struct {
	Packs []DeclaredSkillPack
}

// ExtractDeclaredProviders converts discovered entries into a grouped provider
// inventory for downstream overlay assembly.
func ExtractDeclaredProviders(entries []DiscoveredExtension) DeclaredProviders {
	var inventory DeclaredProviders

	for i := range entries {
		entry := &entries[i]
		inventory.IDE = appendDeclaredProviders(inventory.IDE, entry, entry.Manifest.Providers.IDE)
		inventory.Review = appendDeclaredProviders(inventory.Review, entry, entry.Manifest.Providers.Review)
		inventory.Model = appendDeclaredProviders(inventory.Model, entry, entry.Manifest.Providers.Model)
	}

	slices.SortFunc(inventory.IDE, compareDeclaredProvider)
	slices.SortFunc(inventory.Review, compareDeclaredProvider)
	slices.SortFunc(inventory.Model, compareDeclaredProvider)

	return inventory
}

// ExtractDeclaredSkillPacks resolves skill-pack patterns into deterministic
// path entries for downstream installation flows.
func ExtractDeclaredSkillPacks(entries []DiscoveredExtension) DeclaredSkillPacks {
	if len(entries) == 0 {
		return DeclaredSkillPacks{}
	}

	seen := make(map[string]struct{})
	packs := make([]DeclaredSkillPack, 0)

	for i := range entries {
		entry := &entries[i]
		for _, pattern := range entry.Manifest.Resources.Skills {
			for _, resolvedPath := range entry.resolveSkillPattern(pattern) {
				key := entry.ManifestPath + "\x00" + pattern + "\x00" + resolvedPath
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}

				packs = append(packs, DeclaredSkillPack{
					Extension:    entry.Ref,
					ManifestPath: entry.ManifestPath,
					Pattern:      pattern,
					ResolvedPath: resolvedPath,
					SourceFS:     entry.skillSourceFS(),
					SourceDir:    entry.skillSourceDir(resolvedPath),
				})
			}
		}
	}

	slices.SortFunc(packs, compareDeclaredSkillPack)
	return DeclaredSkillPacks{Packs: packs}
}

func appendDeclaredProviders(
	dst []DeclaredProvider,
	entry *DiscoveredExtension,
	providers []ProviderEntry,
) []DeclaredProvider {
	for i := range providers {
		dst = append(dst, DeclaredProvider{
			Extension:     entry.Ref,
			ManifestPath:  entry.ManifestPath,
			ExtensionDir:  entry.ExtensionDir,
			Manifest:      entry.Manifest,
			ProviderEntry: cloneProviderEntry(providers[i]),
		})
	}
	return dst
}

func compareDeclaredProvider(left, right DeclaredProvider) int {
	if diff := strings.Compare(left.Name, right.Name); diff != 0 {
		return diff
	}
	if diff := sourceRank(left.Extension.Source) - sourceRank(right.Extension.Source); diff != 0 {
		return diff
	}
	return strings.Compare(left.ManifestPath, right.ManifestPath)
}

func compareDeclaredSkillPack(left, right DeclaredSkillPack) int {
	if diff := strings.Compare(left.Extension.Name, right.Extension.Name); diff != 0 {
		return diff
	}
	if diff := sourceRank(left.Extension.Source) - sourceRank(right.Extension.Source); diff != 0 {
		return diff
	}
	return strings.Compare(left.ResolvedPath, right.ResolvedPath)
}

func (e DiscoveredExtension) resolveSkillPattern(pattern string) []string {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return nil
	}

	if e.diskRoot != "" {
		matches, err := filepath.Glob(filepath.Join(e.diskRoot, filepath.FromSlash(trimmed)))
		if err != nil {
			return nil
		}
		slices.Sort(matches)
		return matches
	}

	if e.rootFS == nil || e.fsBase == "" {
		return nil
	}

	matches, err := fs.Glob(e.rootFS, path.Join(e.fsBase, trimmed))
	if err != nil {
		return nil
	}
	slices.Sort(matches)

	resolved := make([]string, 0, len(matches))
	prefix := e.fsBase + "/"
	for _, match := range matches {
		relative := strings.TrimPrefix(match, prefix)
		resolved = append(resolved, path.Join(e.ExtensionDir, relative))
	}
	return resolved
}

func (e DiscoveredExtension) skillSourceFS() fs.FS {
	if e.diskRoot != "" {
		return nil
	}
	return e.rootFS
}

func (e DiscoveredExtension) skillSourceDir(resolvedPath string) string {
	if e.diskRoot != "" {
		return filepath.Base(filepath.Clean(resolvedPath))
	}
	if e.fsBase == "" {
		return ""
	}
	virtualDir := strings.TrimPrefix(strings.TrimPrefix(resolvedPath, e.ExtensionDir), "/")
	return path.Join(e.fsBase, virtualDir)
}
