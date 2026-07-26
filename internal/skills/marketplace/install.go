package marketplace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	registrypkg "github.com/compozy/agh/internal/registry"
	"github.com/compozy/agh/internal/skills"
)

// InstallWithRegistry installs one marketplace skill using the supplied registry.
func InstallWithRegistry(
	ctx context.Context,
	skillsDir string,
	registry Registry,
	slug string,
	version string,
	targetDirOverride string,
	now func() time.Time,
) (item InstallResult, err error) {
	normalizedSlug, err := NormalizeSkillSlug(slug)
	if err != nil {
		return InstallResult{}, err
	}
	if err := ensureMarketplaceSkillsDir(skillsDir); err != nil {
		return InstallResult{}, err
	}
	detail, err := loadMarketplaceSkillDetail(ctx, registry, normalizedSlug)
	if err != nil {
		return InstallResult{}, err
	}

	tempRoot, err := os.MkdirTemp(skillsDir, ".agh-skill-stage-*")
	if err != nil {
		return InstallResult{}, fmt.Errorf("create temporary install directory: %w", err)
	}
	defer joinRemoveAll(&err, tempRoot, "remove temporary install directory")

	installer := registrypkg.NewInstaller(registry)
	result, err := installer.Install(ctx, normalizedSlug, registrypkg.DownloadOpts{
		Version: strings.TrimSpace(version),
	}, tempRoot)
	if err != nil {
		return InstallResult{}, err
	}
	if result == nil {
		return InstallResult{}, classifiedf(
			ErrNotFound,
			"marketplace install returned no result for %q",
			normalizedSlug,
		)
	}
	result.InstallPath, err = PathInsideRoot(tempRoot, result.InstallPath)
	if err != nil {
		return InstallResult{}, fmt.Errorf("validate staged install path for %q: %w", normalizedSlug, err)
	}

	return finalizeMarketplaceInstall(
		skillsDir,
		normalizedSlug,
		detail,
		result,
		targetDirOverride,
		now,
	)
}

func finalizeMarketplaceInstall(
	skillsDir string,
	normalizedSlug string,
	detail *registrypkg.Detail,
	result *registrypkg.InstallResult,
	targetDirOverride string,
	now func() time.Time,
) (InstallResult, error) {
	if result == nil {
		return InstallResult{}, classifiedf(
			ErrNotFound,
			"marketplace install returned no result for %q",
			normalizedSlug,
		)
	}
	localName, err := NormalizeSkillName(result.Name)
	if err != nil {
		return InstallResult{}, err
	}
	resolvedVersion := firstNonEmpty(result.Version, detail.Version)
	registryName := firstNonEmpty(detail.Source, DefaultRegistry)
	targetDir, err := ResolveMarketplaceInstallTarget(
		skillsDir,
		localName,
		targetDirOverride,
	)
	if err != nil {
		return InstallResult{}, fmt.Errorf("resolve install path for %q: %w", normalizedSlug, err)
	}
	if err := ensureMarketplaceInstallTargetReplaceable(targetDir, registryName, normalizedSlug); err != nil {
		return InstallResult{}, err
	}

	hash, err := skills.ComputeDirectoryHash(result.InstallPath)
	if err != nil {
		return InstallResult{}, fmt.Errorf(
			"compute extracted skill hash for %q: %w",
			normalizedSlug,
			err,
		)
	}

	installedAt := normalizeMarketplaceInstallTime(now)
	if err := WriteInstalledSkillProvenance(
		result.InstallPath,
		hash,
		registryName,
		normalizedSlug,
		resolvedVersion,
		installedAt,
	); err != nil {
		return InstallResult{}, err
	}

	if err := registrypkg.MoveInstalledDir(result.InstallPath, targetDir, true); err != nil {
		return InstallResult{}, err
	}

	return InstallResult{
		Name:     localName,
		Slug:     normalizedSlug,
		Version:  resolvedVersion,
		Registry: registryName,
		Path:     targetDir,
		Hash:     hash,
		Status:   "installed",
	}, nil
}

func ensureMarketplaceInstallTargetReplaceable(targetDir string, registryName string, slug string) error {
	info, err := os.Stat(targetDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect install target %q: %w", targetDir, err)
	}
	if !info.IsDir() {
		return classifiedf(ErrValidation, "install target %q is not a directory", targetDir)
	}

	hasSidecar, err := skills.HasSidecar(targetDir)
	if err != nil {
		return err
	}
	if !hasSidecar {
		return classifiedf(ErrNotMarketplace, "install target %q is not a marketplace-installed skill", targetDir)
	}

	provenance, err := skills.ReadSidecar(targetDir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(provenance.Registry) != strings.TrimSpace(registryName) ||
		strings.TrimSpace(provenance.Slug) != strings.TrimSpace(slug) {
		return classifiedf(
			ErrValidation,
			"install target %q belongs to marketplace skill %q from %q",
			targetDir,
			strings.TrimSpace(provenance.Slug),
			strings.TrimSpace(provenance.Registry),
		)
	}
	return nil
}

// WriteInstalledSkillProvenance writes marketplace provenance for one installed skill.
func WriteInstalledSkillProvenance(
	installPath string,
	hash string,
	registryName string,
	slug string,
	resolvedVersion string,
	installedAt time.Time,
) error {
	if err := skills.WriteSidecar(installPath, skills.Provenance{
		Hash:        hash,
		Registry:    registryName,
		Slug:        slug,
		Version:     resolvedVersion,
		InstalledAt: installedAt,
	}); err != nil {
		return fmt.Errorf("write provenance for %q: %w", slug, err)
	}
	return nil
}
