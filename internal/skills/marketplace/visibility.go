package marketplace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/skills"
)

// VerifyInstallVisible verifies that a completed marketplace install is visible
// through AGH skill discovery with the marketplace provenance that agents rely on.
func VerifyInstallVisible(resolver SkillResolver, result InstallResult) error {
	if resolver == nil {
		return classifiedf(
			ErrUnavailable,
			"installed marketplace skill %q cannot be verified because skill discovery is not configured",
			result.Name,
		)
	}
	skill, ok := resolver.Get(result.Name)
	if !ok {
		return classifiedf(
			ErrUnavailable,
			"installed marketplace skill %q is not visible after skill discovery; inspect %s and retry the install",
			result.Name,
			result.Path,
		)
	}
	if err := verifyInstallDiscovery(skill, result); err != nil {
		return err
	}
	return verifyInstallProvenance(skill, result)
}

func verifyInstallDiscovery(skill *skills.Skill, result InstallResult) error {
	if skill.Source != skills.SourceMarketplace {
		return classifiedf(
			ErrUnavailable,
			"installed marketplace skill %q resolved as %s%s after skill discovery; "+
				"remove or disable that declaration and retry the install",
			result.Name,
			skills.SkillSourceName(skill.Source),
			installVisibilityLocation(skill),
		)
	}
	discoveredDir := strings.TrimSpace(skill.Dir)
	if discoveredDir == "" && strings.TrimSpace(skill.FilePath) != "" {
		discoveredDir = filepath.Dir(skill.FilePath)
	}
	canonicalDiscovered, err := canonicalInstallVisibilityPath(discoveredDir)
	if err != nil {
		return classifiedf(
			ErrUnavailable,
			"installed marketplace skill %q discovery path cannot be resolved: %v",
			result.Name,
			err,
		)
	}
	canonicalInstalled, err := canonicalInstallVisibilityPath(result.Path)
	if err != nil {
		return classifiedf(
			ErrUnavailable,
			"installed marketplace skill %q result path cannot be resolved: %v",
			result.Name,
			err,
		)
	}
	if canonicalDiscovered != canonicalInstalled {
		return classifiedf(
			ErrUnavailable,
			"installed marketplace skill %q resolved from %s after skill discovery, want %s",
			result.Name,
			canonicalDiscovered,
			canonicalInstalled,
		)
	}
	return nil
}

func verifyInstallProvenance(skill *skills.Skill, result InstallResult) error {
	if skill.Provenance == nil {
		return classifiedf(
			ErrUnavailable,
			"installed marketplace skill %q is missing provenance after skill discovery; inspect %s and retry the install",
			result.Name,
			result.Path,
		)
	}
	if strings.TrimSpace(skill.Provenance.Slug) != strings.TrimSpace(result.Slug) {
		return classifiedf(
			ErrUnavailable,
			"installed marketplace skill %q resolved slug %q after skill discovery, want %q; "+
				"remove the conflicting skill and retry the install",
			result.Name,
			skill.Provenance.Slug,
			result.Slug,
		)
	}
	if strings.TrimSpace(skill.Provenance.Registry) != strings.TrimSpace(result.Registry) {
		return classifiedf(
			ErrUnavailable,
			"installed marketplace skill %q resolved registry %q after skill discovery, want %q",
			result.Name,
			skill.Provenance.Registry,
			result.Registry,
		)
	}
	if strings.TrimSpace(skill.Provenance.Version) != strings.TrimSpace(result.Version) {
		return classifiedf(
			ErrUnavailable,
			"installed marketplace skill %q resolved version %q after skill discovery, want %q",
			result.Name,
			skill.Provenance.Version,
			result.Version,
		)
	}
	if strings.TrimSpace(skill.Provenance.Hash) != strings.TrimSpace(result.Hash) {
		return classifiedf(
			ErrUnavailable,
			"installed marketplace skill %q resolved hash %q after skill discovery, want %q",
			result.Name,
			skill.Provenance.Hash,
			result.Hash,
		)
	}
	if !skill.Enabled {
		return classifiedf(
			ErrUnavailable,
			"installed marketplace skill %q is visible but disabled after skill discovery; "+
				"enable the skill and retry discovery",
			result.Name,
		)
	}
	return nil
}

func canonicalInstallVisibilityPath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("path is empty")
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path %q: %w", trimmed, err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return filepath.Clean(canonical), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return filepath.Clean(absolute), nil
	}
	return "", fmt.Errorf("resolve symlinks for %q: %w", absolute, err)
}

func installVisibilityLocation(skill *skills.Skill) string {
	if skill == nil {
		return ""
	}
	path := firstNonEmpty(skill.FilePath, skill.Dir)
	if path == "" {
		return ""
	}
	return fmt.Sprintf(" from %s", path)
}
