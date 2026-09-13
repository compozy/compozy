package marketplace

import (
	"context"
	"errors"
	"log/slog"

	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

func WithPackageCache(cache *pluginsource.PackageCache) ServiceOption {
	return func(service *CatalogService) { service.packageCache = cache }
}

func WithLogger(logger *slog.Logger) ServiceOption {
	return func(service *CatalogService) { service.logger = logger }
}

func (s *CatalogService) refreshWithPackageCache(
	ctx context.Context,
	source *registeredSource,
) (outcome RefreshOutcome, resultErr error) {
	defer func() {
		if s.logger != nil {
			s.logger.InfoContext(ctx, "marketplace.source.refresh", "source", outcome.Source,
				"generation", outcome.Generation, "outcome", outcome.Outcome, "error_class", outcome.ErrorClass)
			if outcome.ErrorClass == budgetExhausted {
				s.logger.WarnContext(ctx, "marketplace.source.budget_exhausted", "source", outcome.Source)
			}
		}
	}()
	if s.packageCache == nil {
		return s.refreshSource(ctx, source)
	}
	release := s.packageCache.Hold()
	outcome, err := s.refreshSource(ctx, source)
	release()
	maintenanceCtx, cancel := s.boundedLifecycleContext()
	defer cancel()
	report, sweepErr := s.packageCache.SweepCurrent(maintenanceCtx, s.packagePins)
	if s.logger != nil {
		s.logger.InfoContext(maintenanceCtx, "marketplace.package_cache.sweep",
			"evicted_count", report.EvictedCount, "evicted_bytes", report.EvictedBytes, "error", sweepErr)
	}
	return outcome, errors.Join(err, sweepErr)
}

func (s *CatalogService) packagePins(ctx context.Context) (map[string]struct{}, error) {
	s.sourceMu.RLock()
	defer s.sourceMu.RUnlock()
	names := make([]string, 0, len(s.sources))
	for _, source := range s.sources {
		names = append(names, source.binding.Config.Name)
	}
	pins, err := s.store.PackageDigests(ctx, names)
	if err != nil {
		return nil, err
	}
	if s.installedPackages != nil {
		installed, err := s.installedPackages(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range installed {
			if item.DigestSHA256 != "" {
				pins[item.DigestSHA256] = struct{}{}
			}
		}
	}
	return pins, nil
}

func (s *CatalogService) projectPackageAvailability(ctx context.Context, entry *Entry) error {
	if s.packageCache == nil || entry == nil || entry.SourceName == CompozyCatalogSource || entry.InstallBlocker != "" {
		return nil
	}
	reader, err := s.packageCache.Open(ctx, entry.DigestSHA256)
	if errors.Is(err, pluginsource.ErrPackageUnavailable) {
		entry.Installable = false
		entry.InstallBlocker = "package_unavailable"
		return nil
	}
	if err != nil {
		return err
	}
	return reader.Close()
}
