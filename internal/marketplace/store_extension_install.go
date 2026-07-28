package marketplace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// GetExtensionByInstallSlug resolves one exact feed-authoritative extension acquisition row.
func (s *SQLiteStore) GetExtensionByInstallSlug(
	ctx context.Context,
	installSlug string,
	version string,
) (*Entry, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}
	trimmedSlug := strings.TrimSpace(installSlug)
	if trimmedSlug == "" {
		return nil, errors.New("marketplace catalog: extension install slug is required")
	}
	trimmedVersion := strings.TrimSpace(version)
	row, err := s.repository.GetMarketplaceExtensionByInstallSlug(ctx, trimmedSlug, trimmedVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: extension install %s@%s", ErrEntryNotFound, trimmedSlug, trimmedVersion)
	}
	if err != nil {
		return nil, fmt.Errorf(
			"marketplace catalog: read extension install %q version %q: %w",
			trimmedSlug,
			trimmedVersion,
			err,
		)
	}
	entry, err := marketplaceEntryFromRow(row)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}
