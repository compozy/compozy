package marketplace

import (
	"context"
	"errors"
	"fmt"
)

// Install installs one marketplace skill into the configured Compozy skills root.
func (s *Service) Install(ctx context.Context, slug string, version string) (item InstallResult, err error) {
	registry, err := s.loadRegistry()
	if err != nil {
		return InstallResult{}, err
	}
	defer func() {
		appendRegistrySourceCleanupToInstall(&item, &err, registry.Close())
	}()

	item, err = InstallWithRegistry(ctx, s.homePaths.SkillsDir, registry, slug, version, "", s.now)
	return item, err
}

// Update checks or applies marketplace updates for one skill or every installed marketplace skill.
func (s *Service) Update(ctx context.Context, req UpdateRequest) (items []UpdateResult, err error) {
	registry, err := s.loadRegistry()
	if err != nil {
		return nil, err
	}
	defer func() {
		appendRegistrySourceCleanupToUpdates(items, &err, registry.Close())
	}()

	items, err = UpdateWithRegistry(ctx, s.homePaths.SkillsDir, registry, req, s.now)
	if s.exposures != nil && !req.CheckOnly {
		verificationErrors := make([]error, 0)
		for _, item := range items {
			if item.Status != UpdateStatusUpdated {
				continue
			}
			if verifyErr := s.exposures.VerifyCanonicalDir(ctx, item.Path); verifyErr != nil {
				verificationErrors = append(
					verificationErrors,
					fmt.Errorf("verify exposures after marketplace update %q: %w", item.Name, verifyErr),
				)
			}
		}
		if len(verificationErrors) > 0 {
			err = errors.Join(append([]error{err}, verificationErrors...)...)
		}
	}
	return items, err
}

// Remove removes one installed marketplace skill.
func (s *Service) Remove(ctx context.Context, name string) (RemoveResult, error) {
	normalizedName, err := NormalizeSkillName(name)
	if err != nil {
		return RemoveResult{}, err
	}
	installed, err := FindInstalledSkill(s.homePaths.SkillsDir, normalizedName)
	if err != nil {
		return RemoveResult{}, err
	}
	if s.exposures != nil {
		if err := s.exposures.CleanupCanonicalDir(ctx, installed.Dir); err != nil {
			return RemoveResult{}, err
		}
	}
	return removeInstalledSkill(s.homePaths.SkillsDir, installed)
}

// ListInstalled returns the global marketplace-backed skill installation projection.
func (s *Service) ListInstalled(_ context.Context) ([]InstalledSkill, error) {
	return ListInstalledSkills(s.homePaths.SkillsDir)
}
