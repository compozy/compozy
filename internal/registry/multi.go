package registry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

// MultiRegistry aggregates multiple registry sources.
//
// Sources are ordered from lowest priority to highest priority. Higher-priority
// sources override lower-priority search results when the same slug appears
// more than once.
type MultiRegistry struct {
	sources []Source
	logger  *slog.Logger
}

type searchResult struct {
	listings []Listing
	err      error
	ran      bool
}

type detailResult struct {
	detail *Detail
	err    error
}

// NewMultiRegistry constructs a registry aggregator over one or more sources.
func NewMultiRegistry(logger *slog.Logger, sources ...Source) *MultiRegistry {
	cleaned := make([]Source, 0, len(sources))
	for _, source := range sources {
		if source != nil {
			cleaned = append(cleaned, source)
		}
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &MultiRegistry{
		sources: cleaned,
		logger:  logger,
	}
}

// Search queries every searchable registry source concurrently and merges the
// results using source priority.
func (m *MultiRegistry) Search(ctx context.Context, query string, opts SearchOpts) ([]Listing, error) {
	if err := checkMultiRegistryContext(ctx); err != nil {
		return nil, err
	}
	if len(m.sources) == 0 {
		return []Listing{}, nil
	}

	results := make([]searchResult, len(m.sources))
	var wg sync.WaitGroup
	searchableSources := 0

	for index, source := range m.sources {
		if !source.Capabilities().Search {
			m.logger.Debug(
				"registry: skipping non-searchable source",
				"source", sourceName(source, index),
				"query", strings.TrimSpace(query),
			)
			continue
		}

		results[index].ran = true
		searchableSources++
		wg.Add(1)

		go func(index int, source Source) {
			defer wg.Done()

			listings, err := source.Search(ctx, query, opts)
			if err != nil {
				results[index].err = wrapSourceError(source, index, "search", err)
				return
			}

			results[index].listings = normalizeListings(listings, sourceName(source, index))
		}(index, source)
	}

	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("registry: search canceled: %w", err)
	}

	if searchableSources == 0 {
		return []Listing{}, nil
	}

	merged := mergeListings(results)
	successes := 0
	errs := make([]error, 0, searchableSources)

	for index, result := range results {
		if !result.ran {
			continue
		}
		if result.err != nil {
			errs = append(errs, result.err)
			m.logger.Warn(
				"registry: source search failed",
				"source", sourceName(m.sources[index], index),
				"query", strings.TrimSpace(query),
				"error", result.err,
			)
			continue
		}
		successes++
	}

	if successes == 0 && len(errs) > 0 {
		return []Listing{}, errors.Join(errs...)
	}

	return merged, nil
}

// Info resolves package detail from the highest-priority source that has the
// requested slug.
func (m *MultiRegistry) Info(ctx context.Context, slug string) (*Detail, error) {
	source, detail, err := m.resolveSource(ctx, slug)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, NewPackageNotFoundError(slug)
	}

	detail.Source = firstNonEmpty(strings.TrimSpace(detail.Source), sourceName(source, sourceIndex(m.sources, source)))
	return detail, nil
}

// Download delegates the archive fetch to the highest-priority source that can
// resolve the requested slug.
func (m *MultiRegistry) Download(ctx context.Context, slug string, opts DownloadOpts) (*DownloadResult, error) {
	source, _, err := m.resolveSource(ctx, slug)
	if err != nil {
		return nil, err
	}

	result, err := source.Download(ctx, strings.TrimSpace(slug), opts)
	if err != nil {
		return nil, wrapSourceError(source, sourceIndex(m.sources, source), "download", err)
	}
	if result == nil {
		return nil, fmt.Errorf(
			"registry: source %q returned no download result for %q",
			sourceName(source, sourceIndex(m.sources, source)),
			strings.TrimSpace(slug),
		)
	}
	if result.Reader == nil {
		return nil, fmt.Errorf(
			"registry: source %q returned no download stream for %q",
			sourceName(source, sourceIndex(m.sources, source)),
			strings.TrimSpace(slug),
		)
	}
	if strings.TrimSpace(result.Slug) == "" {
		result.Slug = strings.TrimSpace(slug)
	}

	return result, nil
}

// CheckUpdate compares the local version against the latest version available
// from the resolved source.
func (m *MultiRegistry) CheckUpdate(ctx context.Context, slug string, currentVersion string) (*UpdateInfo, error) {
	detail, err := m.Info(ctx, slug)
	if err != nil {
		return nil, err
	}

	latestVersion := ""
	source := ""
	if detail != nil {
		latestVersion = strings.TrimSpace(detail.Version)
		source = strings.TrimSpace(detail.Source)
	}

	return &UpdateInfo{
		Slug:           strings.TrimSpace(slug),
		CurrentVersion: strings.TrimSpace(currentVersion),
		LatestVersion:  latestVersion,
		HasUpdate:      VersionIsNewer(currentVersion, latestVersion),
		Source:         source,
	}, nil
}

// Close closes every underlying source and joins any close errors.
func (m *MultiRegistry) Close() error {
	errs := make([]error, 0, len(m.sources))
	for index, source := range m.sources {
		if source == nil {
			continue
		}
		if err := source.Close(); err != nil {
			errs = append(errs, wrapSourceError(source, index, "close", err))
		}
	}
	return errors.Join(errs...)
}

// SourceNamed returns the highest-priority source with the requested name.
func (m *MultiRegistry) SourceNamed(name string) Source {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}

	for _, source := range slices.Backward(m.sources) {
		if source == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(source.Name()), trimmed) {
			return source
		}
	}

	return nil
}

func (m *MultiRegistry) resolveSource(ctx context.Context, slug string) (Source, *Detail, error) {
	if err := checkMultiRegistryContext(ctx); err != nil {
		return nil, nil, err
	}

	trimmedSlug := strings.TrimSpace(slug)
	if trimmedSlug == "" {
		return nil, nil, errors.New("registry: slug is required")
	}
	if len(m.sources) == 0 {
		return nil, nil, NewPackageNotFoundError(trimmedSlug)
	}

	results := make([]detailResult, len(m.sources))
	var wg sync.WaitGroup

	for index, source := range m.sources {
		wg.Add(1)
		go func(index int, source Source) {
			defer wg.Done()

			detail, err := source.Info(ctx, trimmedSlug)
			if err != nil {
				if errors.Is(err, ErrPackageNotFound) {
					return
				}
				results[index].err = wrapSourceError(source, index, "info", err)
				return
			}

			if detail != nil && strings.TrimSpace(detail.Source) == "" {
				detail.Source = sourceName(source, index)
			}
			results[index].detail = detail
		}(index, source)
	}

	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, nil, fmt.Errorf("registry: info canceled for %q: %w", trimmedSlug, err)
	}

	for index, v := range slices.Backward(m.sources) {
		if results[index].detail != nil {
			return v, results[index].detail, nil
		}
	}

	errs := make([]error, 0, len(m.sources))
	for _, result := range results {
		if result.err != nil {
			errs = append(errs, result.err)
		}
	}
	if len(errs) > 0 {
		return nil, nil, errors.Join(errs...)
	}

	return nil, nil, NewPackageNotFoundError(trimmedSlug)
}

func mergeListings(results []searchResult) []Listing {
	if len(results) == 0 {
		return []Listing{}
	}

	totalListings := 0
	for _, result := range results {
		totalListings += len(result.listings)
	}

	order := make([]string, 0, totalListings)
	merged := make(map[string]Listing, totalListings)

	for _, result := range results {
		for _, listing := range result.listings {
			slug := strings.TrimSpace(listing.Slug)
			if slug == "" {
				continue
			}

			listing.Slug = slug
			if _, ok := merged[slug]; !ok {
				order = append(order, slug)
			}
			merged[slug] = listing
		}
	}

	items := make([]Listing, 0, len(order))
	for _, slug := range order {
		items = append(items, merged[slug])
	}
	if len(items) == 0 {
		return []Listing{}
	}
	return items
}

func normalizeListings(listings []Listing, source string) []Listing {
	if len(listings) == 0 {
		return nil
	}

	next := 0
	for _, listing := range listings {
		listing.Slug = strings.TrimSpace(listing.Slug)
		if listing.Slug == "" {
			continue
		}
		if strings.TrimSpace(listing.Source) == "" {
			listing.Source = source
		}
		listings[next] = listing
		next++
	}
	if next == 0 {
		return nil
	}
	return listings[:next]
}

func checkMultiRegistryContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("registry: context is required")
	}
	return ctx.Err()
}

func wrapSourceError(source Source, index int, operation string, err error) error {
	return fmt.Errorf("registry: %s via %s: %w", operation, sourceName(source, index), err)
}

func sourceName(source Source, index int) string {
	if source == nil {
		return fmt.Sprintf("source-%d", index)
	}
	if name := strings.TrimSpace(source.Name()); name != "" {
		return name
	}
	return fmt.Sprintf("source-%d", index)
}

func sourceIndex(sources []Source, target Source) int {
	for index, source := range sources {
		if source == target {
			return index
		}
	}
	return -1
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
