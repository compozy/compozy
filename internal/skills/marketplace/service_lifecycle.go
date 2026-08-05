package marketplace

import "context"

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
	return items, err
}

// Remove removes one installed marketplace skill.
func (s *Service) Remove(_ context.Context, name string) (RemoveResult, error) {
	return RemoveSkill(s.homePaths.SkillsDir, name)
}

// ListInstalled returns the global marketplace-backed skill installation projection.
func (s *Service) ListInstalled(_ context.Context) ([]InstalledSkill, error) {
	return ListInstalledSkills(s.homePaths.SkillsDir)
}
