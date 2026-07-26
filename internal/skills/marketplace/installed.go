package marketplace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/skills"
)

// RemoveSkill removes one installed marketplace skill.
func RemoveSkill(skillsDir string, name string) (RemoveResult, error) {
	normalizedName, err := NormalizeSkillName(name)
	if err != nil {
		return RemoveResult{}, err
	}

	installed, err := FindInstalledSkill(skillsDir, normalizedName)
	if err != nil {
		return RemoveResult{}, err
	}

	if err := os.RemoveAll(installed.Dir); err != nil {
		return RemoveResult{}, fmt.Errorf("remove marketplace skill %q: %w", normalizedName, err)
	}

	return RemoveResult{
		Name:   installed.Name,
		Slug:   installed.Provenance.Slug,
		Path:   installed.Dir,
		Status: "removed",
	}, nil
}

// ResolveInstalledSkillRegistry resolves the installed skill's source registry.
func ResolveInstalledSkillRegistry(
	registry Registry,
	installed InstalledSkill,
) (Registry, error) {
	registryName := strings.TrimSpace(installed.Provenance.Registry)
	if registryName == "" {
		return nil, classifiedf(
			ErrValidation,
			"marketplace skill %q is missing registry metadata",
			installed.Name,
		)
	}

	namedRegistry, ok := registry.(NamedRegistry)
	if !ok {
		return registry, nil
	}

	source := namedRegistry.SourceNamed(registryName)
	if source == nil {
		return nil, classifiedf(
			ErrNotConfigured,
			"marketplace registry source %q is not configured for %q",
			registryName,
			installed.Name,
		)
	}

	return SourceBackedRegistry{Source: source}, nil
}

// FindInstalledSkill reads one local marketplace-installed skill.
func FindInstalledSkill(skillsDir string, name string) (InstalledSkill, error) {
	skillDir, err := PathInsideRoot(skillsDir, filepath.Join(skillsDir, name))
	if err != nil {
		return InstalledSkill{}, fmt.Errorf("resolve skill path for %q: %w", name, err)
	}

	info, err := os.Stat(skillDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return InstalledSkill{}, classifiedf(ErrNotFound, "skill %q not found", name)
		}
		return InstalledSkill{}, fmt.Errorf("inspect skill directory %q: %w", skillDir, err)
	}
	if !info.IsDir() {
		return InstalledSkill{}, classifiedf(ErrValidation, "skill %q is not a directory", name)
	}

	hasSidecar, err := skills.HasSidecar(skillDir)
	if err != nil {
		return InstalledSkill{}, err
	}
	if !hasSidecar {
		return InstalledSkill{}, classifiedf(
			ErrNotMarketplace,
			"skill %q is not a marketplace-installed skill",
			name,
		)
	}

	return ReadInstalledSkill(skillDir)
}

// ListInstalledSkills returns every local marketplace-installed skill.
func ListInstalledSkills(skillsDir string) ([]InstalledSkill, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []InstalledSkill{}, nil
		}
		return nil, fmt.Errorf("read installed skills directory %q: %w", skillsDir, err)
	}

	items := make([]InstalledSkill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir, err := PathInsideRoot(skillsDir, filepath.Join(skillsDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf(
				"resolve installed skill path for %q: %w",
				entry.Name(),
				err,
			)
		}

		hasSidecar, err := skills.HasSidecar(skillDir)
		if err != nil {
			return nil, err
		}
		if !hasSidecar {
			continue
		}

		item, err := ReadInstalledSkill(skillDir)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items, nil
}

// ReadInstalledSkill parses one marketplace-installed skill directory.
func ReadInstalledSkill(skillDir string) (InstalledSkill, error) {
	provenance, err := skills.ReadSidecar(skillDir)
	if err != nil {
		return InstalledSkill{}, err
	}
	if provenance == nil {
		return InstalledSkill{}, classifiedf(ErrNotMarketplace, "missing provenance for %q", skillDir)
	}

	skillFile, err := PathInsideRoot(skillDir, filepath.Join(skillDir, SkillMarkdownFileName))
	if err != nil {
		return InstalledSkill{}, fmt.Errorf("resolve skill file in %q: %w", skillDir, err)
	}

	parsedSkill, err := skills.ParseSkillFile(skillFile)
	if err != nil {
		return InstalledSkill{}, err
	}

	return InstalledSkill{
		Name:       parsedSkill.Meta.Name,
		Dir:        parsedSkill.Dir,
		FilePath:   parsedSkill.FilePath,
		Provenance: *provenance,
	}, nil
}
