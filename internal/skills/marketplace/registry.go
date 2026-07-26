package marketplace

import (
	"context"
	"errors"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	registrypkg "github.com/compozy/agh/internal/registry"
	registryclawhub "github.com/compozy/agh/internal/registry/clawhub"
)

// DefaultSourceLoader resolves the configured marketplace registry source.
func DefaultSourceLoader(registryCfg aghconfig.MarketplaceConfig) ([]registrypkg.Source, error) {
	registryName := strings.ToLower(strings.TrimSpace(registryCfg.Registry))
	if registryName == "" {
		registryName = DefaultRegistry
	}

	switch registryName {
	case DefaultRegistry:
		return []registrypkg.Source{
			registryclawhub.NewClient(registryCfg.BaseURL),
		}, nil
	default:
		return nil, classifiedf(
			ErrNotConfigured,
			"unsupported marketplace registry %q",
			registryCfg.Registry,
		)
	}
}

// Search queries configured marketplace sources for skill packages.
func (s *Service) Search(
	ctx context.Context,
	query string,
	offset int,
	limit int,
) (_ []registrypkg.Listing, err error) {
	if offset < 0 {
		return nil, classifiedf(ErrValidation, "search offset must be non-negative: %d", offset)
	}
	if limit <= 0 {
		return nil, classifiedf(ErrValidation, "search limit must be positive: %d", limit)
	}

	registry, err := s.loadRegistry()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, registry.Close())
	}()

	return registry.Search(ctx, query, registrypkg.SearchOpts{
		Limit:  limit,
		Offset: offset,
		Type:   registrypkg.PackageTypeSkill,
	})
}

// Info resolves remote metadata for one marketplace skill.
func (s *Service) Info(ctx context.Context, slug string) (_ *registrypkg.Detail, err error) {
	normalizedSlug, err := NormalizeSkillSlug(slug)
	if err != nil {
		return nil, err
	}

	registry, err := s.loadRegistry()
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, registry.Close())
	}()

	detail, err := registry.Info(ctx, normalizedSlug)
	if err != nil {
		return nil, classifyRegistryLookup(err)
	}
	if detail == nil {
		return nil, classifiedf(
			ErrNotFound,
			"marketplace info returned no detail for %q",
			normalizedSlug,
		)
	}
	return detail, nil
}

// Download implements Registry.
func (r SourceBackedRegistry) Download(
	ctx context.Context,
	slug string,
	opts registrypkg.DownloadOpts,
) (*registrypkg.DownloadResult, error) {
	if r.Source == nil {
		return nil, classifiedf(ErrNotConfigured, "registry source is required")
	}
	return r.Source.Download(ctx, slug, opts)
}

// Info implements Registry.
func (r SourceBackedRegistry) Info(
	ctx context.Context,
	slug string,
) (*registrypkg.Detail, error) {
	if r.Source == nil {
		return nil, classifiedf(ErrNotConfigured, "registry source is required")
	}

	detail, err := r.Source.Info(ctx, slug)
	if err != nil {
		return nil, err
	}
	if detail != nil && strings.TrimSpace(detail.Source) == "" {
		detail.Source = strings.TrimSpace(r.Source.Name())
	}
	return detail, nil
}

// CheckUpdate implements Registry.
func (r SourceBackedRegistry) CheckUpdate(
	ctx context.Context,
	slug string,
	currentVersion string,
) (*registrypkg.UpdateInfo, error) {
	detail, err := r.Info(ctx, slug)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, classifiedf(
			ErrNotFound,
			"marketplace info returned no detail for %q",
			slug,
		)
	}

	latestVersion := strings.TrimSpace(detail.Version)
	return &registrypkg.UpdateInfo{
		Slug:           strings.TrimSpace(slug),
		CurrentVersion: strings.TrimSpace(currentVersion),
		LatestVersion:  latestVersion,
		HasUpdate:      registrypkg.VersionIsNewer(currentVersion, latestVersion),
		Source:         strings.TrimSpace(detail.Source),
	}, nil
}

func (s *Service) loadRegistry() (*registrypkg.MultiRegistry, error) {
	sources, err := s.sourceLoader(s.skillsConfig.Marketplace)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, classifiedf(ErrNotConfigured, "no skill registry sources are configured")
	}
	return registrypkg.NewMultiRegistry(s.logger, sources...), nil
}
