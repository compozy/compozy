package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	registrypkg "github.com/compozy/agh/internal/registry"
)

// LoadMarketplaceSources resolves marketplace sources and closes rejected sources.
func LoadMarketplaceSources(
	ctx context.Context,
	loader MarketplaceSourceLoader,
	sourceFilter string,
) ([]registrypkg.Source, error) {
	if ctx == nil {
		return nil, errors.New("extension: context is required")
	}
	if loader == nil {
		return nil, errors.New("extension: marketplace source loader is required")
	}

	sources, err := loader(ctx)
	if err != nil {
		return nil, joinMarketplaceSourceError(
			fmt.Errorf("%w: %w", ErrMarketplaceSourceUnavailable, err),
			closeRegistrySources(sources),
		)
	}
	if len(sources) == 0 {
		err := fmt.Errorf("%w: no marketplace registry sources are configured", ErrMarketplaceSourceUnavailable)
		return nil, joinMarketplaceSourceError(err, closeRegistrySources(sources))
	}

	filtered := filterExtensionRegistrySources(sources, sourceFilter)
	if len(filtered) == 0 {
		err := fmt.Errorf(
			"%w: marketplace registry source %q is not configured",
			ErrMarketplaceSourceUnavailable,
			sourceFilter,
		)
		return nil, joinMarketplaceSourceError(err, closeRegistrySources(sources))
	}
	if err := closeUnselectedExtensionRegistrySources(sources, sourceFilter); err != nil {
		return nil, joinMarketplaceSourceError(err, closeRegistrySources(filtered))
	}
	return filtered, nil
}

func filterExtensionRegistrySources(sources []registrypkg.Source, sourceFilter string) []registrypkg.Source {
	filter := strings.ToLower(strings.TrimSpace(sourceFilter))
	if filter == "" {
		return sources
	}
	filtered := make([]registrypkg.Source, 0, len(sources))
	for _, source := range sources {
		if source != nil && strings.EqualFold(strings.TrimSpace(source.Name()), filter) {
			filtered = append(filtered, source)
		}
	}
	return filtered
}

func closeUnselectedExtensionRegistrySources(sources []registrypkg.Source, sourceFilter string) error {
	filter := strings.TrimSpace(sourceFilter)
	if filter == "" {
		return nil
	}
	dropped := make([]registrypkg.Source, 0, len(sources))
	for _, source := range sources {
		if source == nil || strings.EqualFold(strings.TrimSpace(source.Name()), filter) {
			continue
		}
		dropped = append(dropped, source)
	}
	return closeRegistrySources(dropped)
}

func closeRegistrySources(sources []registrypkg.Source) error {
	errs := make([]error, 0, len(sources))
	for _, source := range sources {
		if source == nil {
			continue
		}
		if err := source.Close(); err != nil {
			errs = append(errs, fmt.Errorf("extension: close registry source %q: %w", source.Name(), err))
		}
	}
	return errors.Join(errs...)
}

func joinMarketplaceSourceError(base error, extra error) error {
	if extra == nil {
		return base
	}
	if base == nil {
		return extra
	}
	return errors.Join(base, extra)
}
