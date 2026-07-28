package marketplace

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ListSkillsByInstallSlugs resolves exact curated skill acquisition identities in one read.
func (s *SQLiteStore) ListSkillsByInstallSlugs(
	ctx context.Context,
	installSlugs []string,
) ([]Entry, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}
	slugs := normalizeInstallSlugs(installSlugs)
	if len(slugs) == 0 {
		return nil, errors.New("marketplace catalog: skill install slugs are required")
	}
	rows, err := s.repository.ListMarketplaceSkillsByInstallSlugs(ctx, slugs)
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: list skill installs: %w", err)
	}
	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		entry, mapErr := marketplaceEntryFromRow(row)
		if mapErr != nil {
			return nil, mapErr
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func normalizeInstallSlugs(installSlugs []string) []string {
	unique := make(map[string]struct{}, len(installSlugs))
	for _, value := range installSlugs {
		if slug := strings.TrimSpace(value); slug != "" {
			unique[slug] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for slug := range unique {
		result = append(result, slug)
	}
	slices.Sort(result)
	return result
}
