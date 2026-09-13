package marketplace

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ResolveExtensionInstall reads the selected source's approved projection without remote refresh.
func (s *CatalogService) ResolveExtensionInstall(ctx context.Context, installSlug, version string) (*Entry, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}
	installSlug, version = strings.TrimSpace(installSlug), strings.TrimSpace(version)
	if installSlug == "" {
		return nil, errors.New("marketplace catalog: extension install slug is required")
	}
	s.sourceMu.RLock()
	defer s.sourceMu.RUnlock()
	name, entryID, ok := strings.Cut(installSlug, "/")
	if !ok || entryID == "" || strings.Contains(entryID, "/") {
		return nil, ErrEntryNotFound
	}
	if source, exists := s.byName[name]; exists && source.binding.Config.Kind != SourceKindFeed {
		if !source.binding.Config.Enabled {
			return nil, ErrEntryNotFound
		}
		entry, err := s.store.GetEntry(ctx, name, entryID)
		if err != nil {
			return nil, err
		}
		if version != "" && entry.Version != version {
			return nil, ErrEntryNotFound
		}
		return entry, nil
	}
	entry, err := s.store.GetExtensionByInstallSlug(ctx, installSlug, version)
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: resolve extension install: %w", err)
	}
	return entry, nil
}
