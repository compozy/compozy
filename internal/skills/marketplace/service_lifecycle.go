package marketplace

import (
	"context"
	"errors"
)

// Install installs one marketplace skill into the configured AGH skills root.
func (s *Service) Install(ctx context.Context, slug string, version string) (_ InstallResult, err error) {
	registry, err := s.loadRegistry()
	if err != nil {
		return InstallResult{}, err
	}
	defer func() {
		err = errors.Join(err, registry.Close())
	}()

	return InstallWithRegistry(ctx, s.homePaths.SkillsDir, registry, slug, version, "", s.now)
}

// Update checks or applies marketplace updates for one skill or every installed marketplace skill.
func (s *Service) Update(ctx context.Context, req UpdateRequest) (_ []UpdateResult, err error) {
	registry, err := s.loadRegistry()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, registry.Close())
	}()

	return UpdateWithRegistry(ctx, s.homePaths.SkillsDir, registry, req, s.now)
}

// Remove removes one installed marketplace skill.
func (s *Service) Remove(_ context.Context, name string) (RemoveResult, error) {
	return RemoveSkill(s.homePaths.SkillsDir, name)
}

// ListInstalled returns the global marketplace-backed skill installation projection.
func (s *Service) ListInstalled(_ context.Context) ([]InstalledSkill, error) {
	return ListInstalledSkills(s.homePaths.SkillsDir)
}
