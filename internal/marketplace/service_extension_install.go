package marketplace

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ResolveExtensionInstall resolves curated acquisition refs without consulting plugin source names.
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
	entry, err := s.store.GetExtensionByInstallSlug(ctx, installSlug, version)
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: resolve extension install: %w", err)
	}
	return entry, nil
}
