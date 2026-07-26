package marketplace

import (
	"context"
	"errors"
)

// ResolveSkillInstalls returns the curated identities for exact remote skill slugs.
func (s *CatalogService) ResolveSkillInstalls(
	ctx context.Context,
	installSlugs []string,
) ([]Entry, error) {
	if err := s.checkReady(ctx, KindSkill); err != nil {
		return nil, err
	}
	refreshErr := s.ensureFresh(ctx, KindSkill)
	entries, listErr := s.store.ListSkillsByInstallSlugs(ctx, installSlugs)
	if listErr != nil {
		return nil, errors.Join(refreshErr, listErr)
	}
	if refreshErr != nil && len(entries) == 0 {
		return nil, refreshErr
	}
	return entries, nil
}
