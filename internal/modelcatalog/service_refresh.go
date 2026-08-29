package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

func (s *CatalogService) storedSourceIDsForProvider(
	ctx context.Context,
	providerID string,
	opts RefreshOptions,
	now time.Time,
) (map[string]struct{}, error) {
	owned := make(map[string]struct{})
	statuses, err := s.store.ListSourceStatus(ctx, StatusOptions{
		ProviderID:       providerID,
		ExecutionContext: opts.ExecutionContext,
		SourceContexts:   cloneSourceExecutionContexts(opts.SourceContexts),
	})
	if err != nil {
		return nil, fmt.Errorf("model catalog: list stored source ownership for %q: %w", providerID, err)
	}
	for _, status := range statuses {
		if sourceID := strings.TrimSpace(status.SourceID); sourceID != "" {
			owned[sourceID] = struct{}{}
		}
	}
	rows, err := s.store.ListRows(ctx, ListOptions{
		ProviderID:       providerID,
		ExecutionContext: opts.ExecutionContext,
		SourceContexts:   cloneSourceExecutionContexts(opts.SourceContexts),
		IncludeAll:       true,
		IncludeStale:     true,
		Now:              now,
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
		if err := ctx.Err(); err != nil {
			return statuses, err
		}
		sourceStatuses, outcome, err := s.refreshSource(ctx, source, opts, now)
		statuses = append(statuses, sourceStatuses...)
		if err != nil {
			if outcome == refreshOutcomeStale {
				degradedErrs = append(degradedErrs, err)
			} else if !errors.Is(err, ErrSourceDisabled) {
				failures++
				failedErrs = append(failedErrs, &sourceRefreshError{sourceID: source.ID(), err: err})
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
		if err := ctx.Err(); err != nil {
			return statuses, err
		}
		providers := sourceProviders(source)
		if len(providers) > 0 {
			executionContext, contextErr := sourceExecutionContext(opts.SourceContexts, source.ID())
			if contextErr != nil {
				return statuses, contextErr
			}
			storedProviders, err := s.storedProvidersForSource(ctx, source, executionContext, now)
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
			if err := ctx.Err(); err != nil {
				return statuses, err
			}
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
	executionContext, err := sourceExecutionContext(opts.SourceContexts, source.ID())
	if err != nil {
		return nil, refreshOutcomeEmpty, err
	}
	if !opts.Force &&
		strings.TrimSpace(opts.ProviderID) != "" &&
		sourceHasFreshStatus(ctx, s.store, source, opts.ProviderID, executionContext, now) {
		statuses, err := s.store.ListSourceStatus(ctx, StatusOptions{
			ProviderID:       opts.ProviderID,
			ExecutionContext: executionContext,
			SourceContexts: map[string]CatalogExecutionContext{
				source.ID(): executionContext,
			},
		})
		if err != nil {
			return nil, refreshOutcomeEmpty, fmt.Errorf("model catalog: load fresh source status: %w", err)
		}
		return filterStatusesBySource(statuses, source.ID()), refreshOutcomeSuccess, nil
	}

	rows, err := source.ListModels(ctx, ListOptions{
		ProviderID:       opts.ProviderID,
		SourceID:         source.ID(),
		ExecutionContext: executionContext,
		SourceContexts: map[string]CatalogExecutionContext{
			source.ID(): executionContext,
		},
		Refresh:      true,
		IncludeAll:   true,
		IncludeStale: true,
		Now:          now,
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, refreshOutcomeEmpty, ctxErr
		}
		return s.recordSourceFailure(
			ctx,
			source,
			opts.ProviderID,
			executionContext,
			opts.PreviousExecutionContext,
			rows,
			now,
			err,
		)
	}
	statuses, err := s.persistSourceRows(ctx, source, opts.ProviderID, rows, executionContext, now, false, "")
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
	executionContext CatalogExecutionContext,
	previousExecutionContext CatalogExecutionContext,
	rows []ModelRow,
	now time.Time,
	sourceErr error,
) ([]SourceStatus, refreshOutcome, error) {
	if errors.Is(sourceErr, ErrSourceDisabled) {
		statuses, err := s.persistDisabledSource(ctx, source, providerID, executionContext, now)
		return statuses, refreshOutcomeEmpty, err
	}
	redacted := sourceErrorText(sourceErr)
	if len(rows) > 0 {
		staleRows := markRowsStale(rows, redacted)
		statuses, err := s.persistSourceRows(
			ctx,
			source,
			providerID,
			staleRows,
			executionContext,
			now,
			true,
			redacted,
		)
		if err != nil {
			return nil, refreshOutcomeEmpty, errors.Join(sourceErr, err)
		}
		return statuses, refreshOutcomeStale, sourceErr
	}

	return s.recordEmptySourceFailure(
		ctx,
		source,
		providerID,
		executionContext,
		previousExecutionContext,
		now,
		redacted,
		sourceErr,
	)
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

func (s *CatalogService) joinExistingRefreshFlight(
	ctx context.Context,
	providerID string,
	scopeKey string,
) ([]SourceStatus, bool, error) {
	s.lockMu.Lock()
	flight := s.refreshFlights[providerID]
	if flight == nil || flight.scopeKey != scopeKey {
		s.lockMu.Unlock()
		return nil, false, nil
	}
	s.lockMu.Unlock()
	if hook := s.onFlightWait; hook != nil {
		hook(providerID)
	}
	select {
	case <-flight.done:
		return cloneSourceStatuses(flight.statuses), true, flight.err
	case <-ctx.Done():
		return nil, true, ctx.Err()
	}
}

func refreshFlightScopeKey(providerKey string, opts RefreshOptions) string {
	contextParts := make([]string, 0, len(opts.SourceContexts))
	for sourceID, executionContext := range opts.SourceContexts {
		contextParts = append(contextParts, sourceID+"="+executionContext.CommandFingerprint)
	}
	sort.Strings(contextParts)
	return fmt.Sprintf(
		"%s\x00%s\x00%s\x00%s\x00%t\x00%s",
		providerKey,
		strings.TrimSpace(opts.SourceID),
		executionContextScopeKey(opts.ExecutionContext),
		strings.TrimSpace(opts.ExecutionContext.CommandFingerprint),
		opts.Force,
		strings.Join(contextParts, "\x1e"),
	)
}

func sourceHasFreshStatus(
	ctx context.Context,
	store Store,
	source Source,
	providerID string,
	executionContext CatalogExecutionContext,
	now time.Time,
) bool {
	if ttlProvider, ok := source.(sourceTTLProvider); !ok || ttlProvider.TTL() <= 0 {
		return false
	}
	statuses, err := store.ListSourceStatus(ctx, StatusOptions{
		ProviderID:       providerID,
		ExecutionContext: executionContext,
		SourceContexts: map[string]CatalogExecutionContext{
			source.ID(): executionContext,
		},
	})
	if err != nil {
		return false
	}
	for _, status := range statuses {
		if status.SourceID != source.ID() {
			continue
		}
		if status.NextRefresh.IsZero() || !status.NextRefresh.After(now) {
			return false
		}
		switch status.RefreshState {
		case RefreshStateSucceeded, RefreshStateFailed, RefreshStateDisabled:
			return true
		default:
			return false
		}
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
