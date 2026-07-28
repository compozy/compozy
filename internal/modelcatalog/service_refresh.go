package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

func (s *CatalogService) storedSourceIDsForProvider(
	ctx context.Context,
	providerID string,
	now time.Time,
) (map[string]struct{}, error) {
	owned := make(map[string]struct{})
	statuses, err := s.store.ListSourceStatus(ctx, providerID)
	if err != nil {
		return nil, fmt.Errorf("model catalog: list stored source ownership for %q: %w", providerID, err)
	}
	for _, status := range statuses {
		if sourceID := strings.TrimSpace(status.SourceID); sourceID != "" {
			owned[sourceID] = struct{}{}
		}
	}
	rows, err := s.store.ListRows(ctx, ListOptions{
		ProviderID:   providerID,
		IncludeAll:   true,
		IncludeStale: true,
		Now:          now,
	})
	if err != nil {
		return nil, fmt.Errorf("model catalog: list stored rows for provider %q: %w", providerID, err)
	}
	for _, row := range rows {
		if sourceID := strings.TrimSpace(row.SourceID); sourceID != "" {
			owned[sourceID] = struct{}{}
		}
	}
	return owned, nil
}

func (s *CatalogService) refreshSources(
	ctx context.Context,
	sources []Source,
	opts RefreshOptions,
	now time.Time,
) ([]SourceStatus, error) {
	statuses := make([]SourceStatus, 0, len(sources))
	var degradedErrs []error
	var failedErrs []error
	successes := 0
	failures := 0
	staleFallbacks := 0
	for _, source := range sources {
		sourceStatuses, outcome, err := s.refreshSource(ctx, source, opts, now)
		statuses = append(statuses, sourceStatuses...)
		if err != nil {
			if outcome == refreshOutcomeStale {
				degradedErrs = append(degradedErrs, err)
			} else if !errors.Is(err, ErrSourceDisabled) {
				failures++
				failedErrs = append(
					failedErrs,
					fmt.Errorf("model catalog source %q failed: %s", source.ID(), RedactString(err.Error())),
				)
			}
		}
		switch outcome {
		case refreshOutcomeSuccess:
			successes++
		case refreshOutcomeStale:
			staleFallbacks++
		}
	}
	if len(degradedErrs) > 0 {
		return statuses, errors.Join(degradedErrs...)
	}
	if successes == 0 && staleFallbacks == 0 && failures > 0 {
		return statuses, errors.Join(
			fmt.Errorf("%w (%d failed)", ErrAllSourcesFailed, failures),
			errors.Join(failedErrs...),
		)
	}
	return statuses, nil
}

func (s *CatalogService) refreshAllProviders(
	ctx context.Context,
	sources []Source,
	opts RefreshOptions,
	now time.Time,
) ([]SourceStatus, error) {
	statuses := make([]SourceStatus, 0)
	var firstErr error
	var degradedErrs []error
	successes := 0

	for _, source := range sources {
		providers := sourceProviders(source)
		if len(providers) > 0 {
			storedProviders, err := s.storedProvidersForSource(ctx, source, now)
			if err != nil {
				return statuses, err
			}
			providers = normalizedProviderIDs(append(providers, storedProviders...))
		}
		if len(providers) == 0 {
			sourceStatuses, err := s.refreshSources(ctx, []Source{source}, opts, now)
			statuses = append(statuses, sourceStatuses...)
			if err != nil {
				if hasStaleFailureStatus(sourceStatuses) {
					degradedErrs = append(degradedErrs, err)
				} else if firstErr == nil {
					firstErr = err
				}
			} else {
				successes++
			}
			continue
		}

		snapshot := &sourceRefreshSnapshot{Source: source}
		for _, providerID := range providers {
			providerOpts := opts
			providerOpts.ProviderID = providerID
			providerOpts.SourceID = source.ID()
			scopeKey := refreshFlightScopeKey(providerID, providerOpts)
			sourceStatuses, err := s.withRefreshFlight(ctx, providerID, scopeKey, func() ([]SourceStatus, error) {
				return s.refreshSources(ctx, []Source{snapshot}, providerOpts, now)
			})
			statuses = append(statuses, sourceStatuses...)
			if err != nil {
				if hasStaleFailureStatus(sourceStatuses) {
					degradedErrs = append(degradedErrs, err)
				} else if firstErr == nil {
					firstErr = err
				}
				continue
			}
			successes++
		}
	}

	if len(degradedErrs) > 0 {
		return statuses, errors.Join(degradedErrs...)
	}
	if successes == 0 && firstErr != nil {
		return statuses, firstErr
	}
	return statuses, nil
}

type sourceRefreshSnapshot struct {
	Source
	once sync.Once
	rows []ModelRow
	err  error
}

var _ Source = (*sourceRefreshSnapshot)(nil)

func (s *sourceRefreshSnapshot) ProviderIDs() []string {
	return sourceProviders(s.Source)
}

func (s *sourceRefreshSnapshot) TTL() time.Duration {
	provider, ok := s.Source.(sourceTTLProvider)
	if !ok {
		return 0
	}
	return provider.TTL()
}

func (s *sourceRefreshSnapshot) ListModels(ctx context.Context, opts ListOptions) ([]ModelRow, error) {
	s.once.Do(func() {
		globalOpts := opts
		globalOpts.ProviderID = ""
		s.rows, s.err = s.Source.ListModels(ctx, globalOpts)
	})
	return filterRowsByProvider(cloneModelRows(s.rows), opts.ProviderID), s.err
}

type refreshOutcome int

const (
	refreshOutcomeEmpty refreshOutcome = iota
	refreshOutcomeSuccess
	refreshOutcomeStale
)

func (s *CatalogService) refreshSource(
	ctx context.Context,
	source Source,
	opts RefreshOptions,
	now time.Time,
) ([]SourceStatus, refreshOutcome, error) {
	if !opts.Force &&
		strings.TrimSpace(opts.ProviderID) != "" &&
		sourceHasFreshStatus(ctx, s.store, source, opts.ProviderID, now) {
		statuses, err := s.store.ListSourceStatus(ctx, opts.ProviderID)
		if err != nil {
			return nil, refreshOutcomeEmpty, fmt.Errorf("model catalog: load fresh source status: %w", err)
		}
		return filterStatusesBySource(statuses, source.ID()), refreshOutcomeSuccess, nil
	}

	rows, err := source.ListModels(ctx, ListOptions{
		ProviderID:   opts.ProviderID,
		SourceID:     source.ID(),
		Refresh:      true,
		IncludeAll:   true,
		IncludeStale: true,
		Now:          now,
	})
	if err != nil {
		return s.recordSourceFailure(ctx, source, opts.ProviderID, rows, now, err)
	}
	statuses, err := s.persistSourceRows(ctx, source, opts.ProviderID, rows, now, false, "")
	if err != nil {
		return nil, refreshOutcomeEmpty, err
	}
	if len(rows) > 0 {
		return statuses, refreshOutcomeSuccess, nil
	}
	return statuses, refreshOutcomeEmpty, nil
}

func (s *CatalogService) recordSourceFailure(
	ctx context.Context,
	source Source,
	providerID string,
	rows []ModelRow,
	now time.Time,
	sourceErr error,
) ([]SourceStatus, refreshOutcome, error) {
	if errors.Is(sourceErr, ErrSourceDisabled) {
		statuses, err := s.persistDisabledSource(ctx, source, providerID, now)
		return statuses, refreshOutcomeEmpty, err
	}
	redacted := sourceErrorText(sourceErr)
	if len(rows) > 0 {
		staleRows := markRowsStale(rows, redacted)
		statuses, err := s.persistSourceRows(ctx, source, providerID, staleRows, now, true, redacted)
		if err != nil {
			return nil, refreshOutcomeEmpty, err
		}
		return statuses, refreshOutcomeStale, sourceErr
	}

	providers, err := s.providersForSource(ctx, source, providerID, now)
	if err != nil {
		return nil, refreshOutcomeEmpty, err
	}
	statuses := make([]SourceStatus, 0, len(providers))
	staleCount := 0
	for _, provider := range providers {
		previous, err := s.store.ListRows(ctx, ListOptions{
			ProviderID:   provider,
			SourceID:     source.ID(),
			IncludeAll:   true,
			IncludeStale: true,
			Now:          now,
		})
		if err != nil {
			return nil, refreshOutcomeEmpty, fmt.Errorf("model catalog: load stale rows for %q: %w", source.ID(), err)
		}
		staleRows := markRowsStale(previous, redacted)
		status := sourceStatus(source, provider, now, len(staleRows), true, redacted, RefreshStateFailed)
		s.preserveLastSuccess(ctx, provider, &status)
		if err := s.store.ReplaceSourceRows(ctx, source.ID(), provider, staleRows, status); err != nil {
			return nil, refreshOutcomeEmpty, fmt.Errorf("model catalog: persist failed source status: %w", err)
		}
		if len(staleRows) > 0 {
			staleCount += len(staleRows)
		}
		statuses = append(statuses, status)
	}
	if staleCount > 0 {
		return statuses, refreshOutcomeStale, sourceErr
	}
	return statuses, refreshOutcomeEmpty, sourceErr
}

func (s *CatalogService) withRefreshFlight(
	ctx context.Context,
	providerID string,
	scopeKey string,
	fn func() ([]SourceStatus, error),
) ([]SourceStatus, error) {
	for {
		s.lockMu.Lock()
		flight := s.refreshFlights[providerID]
		if flight == nil {
			flight = &refreshFlight{
				scopeKey: scopeKey,
				done:     make(chan struct{}),
			}
			s.refreshFlights[providerID] = flight
			s.lockMu.Unlock()

			flight.statuses, flight.err = fn()
			s.lockMu.Lock()
			close(flight.done)
			delete(s.refreshFlights, providerID)
			s.lockMu.Unlock()
			return cloneSourceStatuses(flight.statuses), flight.err
		}
		s.lockMu.Unlock()
		if hook := s.onFlightWait; hook != nil {
			hook(providerID)
		}
		select {
		case <-flight.done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if flight.scopeKey == scopeKey {
			return cloneSourceStatuses(flight.statuses), flight.err
		}
	}
}

func refreshFlightScopeKey(providerKey string, opts RefreshOptions) string {
	return fmt.Sprintf("%s\x00%s\x00%t", providerKey, strings.TrimSpace(opts.SourceID), opts.Force)
}

func sourceHasFreshStatus(ctx context.Context, store Store, source Source, providerID string, now time.Time) bool {
	if ttlProvider, ok := source.(sourceTTLProvider); !ok || ttlProvider.TTL() <= 0 {
		return false
	}
	statuses, err := store.ListSourceStatus(ctx, providerID)
	if err != nil {
		return false
	}
	for _, status := range statuses {
		if status.SourceID != source.ID() {
			continue
		}
		return status.RefreshState == RefreshStateSucceeded &&
			!status.NextRefresh.IsZero() &&
			status.NextRefresh.After(now)
	}
	return false
}

func filterStatusesBySource(statuses []SourceStatus, sourceID string) []SourceStatus {
	filtered := make([]SourceStatus, 0, len(statuses))
	for _, status := range statuses {
		if status.SourceID == sourceID {
			filtered = append(filtered, status)
		}
	}
	return filtered
}
