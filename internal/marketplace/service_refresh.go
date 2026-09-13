package marketplace

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"

	"github.com/compozy/compozy/internal/diagnostics"
	storepkg "github.com/compozy/compozy/internal/store"
)

type refreshFlight struct {
	done       chan struct{}
	cancel     context.CancelFunc
	source     string
	generation int64
	outcome    RefreshOutcome
	err        error
}

func (s *CatalogService) Refresh(ctx context.Context, names ...string) (RefreshReport, error) {
	if err := s.checkReady(ctx); err != nil {
		return RefreshReport{}, err
	}
	s.sourceMu.RLock()
	selected := make(map[string]bool, len(names))
	for _, name := range names {
		if _, exists := s.byName[name]; !exists {
			s.sourceMu.RUnlock()
			return RefreshReport{}, fmt.Errorf("%w: %s", ErrSourceStateMissing, name)
		}
		selected[name] = true
	}
	flights := make([]*refreshFlight, 0, len(s.sources))
	for _, source := range s.sources {
		if !source.binding.Config.Enabled || (len(selected) > 0 && !selected[source.binding.Config.Name]) {
			continue
		}
		flight, err := s.startRefreshFlight(source, true)
		if err != nil {
			s.sourceMu.RUnlock()
			return RefreshReport{}, err
		}
		flights = append(flights, flight)
	}
	s.sourceMu.RUnlock()
	report := RefreshReport{Outcomes: make([]RefreshOutcome, 0, len(flights))}
	var resultErr error
	for _, flight := range flights {
		outcome, err := awaitRefreshFlight(ctx, flight)
		report.Outcomes = append(report.Outcomes, outcome)
		if err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("refresh %s: %w", flight.source, err))
		}
	}
	return report, resultErr
}

// startRefreshFlight is called under the source read lock; execution belongs to the service.
func (s *CatalogService) startRefreshFlight(source *registeredSource, force bool) (*refreshFlight, error) {
	s.flightMu.Lock()
	defer s.flightMu.Unlock()
	if s.closed {
		return nil, ErrServiceClosed
	}
	if source.flight != nil {
		return source.flight, nil
	}
	now := s.now().UTC()
	if !force && !source.lastAttempt.IsZero() && source.lastAttempt.Add(s.ttl).After(now) {
		return nil, nil
	}
	source.lastAttempt = now

	var ctx context.Context
	var cancel context.CancelFunc
	if source.binding.Config.Kind == SourceKindFeed {
		ctx, cancel = context.WithTimeout(s.lifecycleCtx, s.refreshTimeout)
	} else {
		// PluginSource owns the single budget spanning document, checkout and package work.
		ctx, cancel = context.WithCancel(s.lifecycleCtx)
	}
	flight := &refreshFlight{
		done:       make(chan struct{}),
		cancel:     cancel,
		source:     source.binding.Config.Name,
		generation: source.generation,
	}
	source.flight = flight
	s.flightWG.Go(func() {
		defer cancel()
		flight.outcome, flight.err = s.refreshSource(ctx, source)
		s.flightMu.Lock()
		if source.flight == flight {
			source.flight = nil
		}
		close(flight.done)
		s.flightMu.Unlock()
	})
	return flight, nil
}

func awaitRefreshFlight(ctx context.Context, flight *refreshFlight) (RefreshOutcome, error) {
	select {
	case <-flight.done:
		return flight.outcome, flight.err
	case <-ctx.Done():
		return failedRefreshOutcome(flight.source, flight.generation, errorClassCanceled), ctx.Err()
	}
}

func failedRefreshOutcome(source string, generation int64, class string) RefreshOutcome {
	return RefreshOutcome{Source: source, Generation: generation, Outcome: RefreshOutcomeFailed, ErrorClass: class}
}

func (s *CatalogService) currentSource(source *registeredSource) error {
	if s.byName[source.binding.Config.Name] != source || !source.binding.Config.Enabled {
		return storepkg.ErrMarketplaceCatalogGenerationStale
	}
	return s.lifecycleError()
}

func (s *CatalogService) refreshSource(ctx context.Context, source *registeredSource) (RefreshOutcome, error) {
	document, err := source.binding.Fetcher.Fetch(ctx)
	if err != nil {
		return s.recordFailure(source, classifyFetchError(err), errors.Join(ErrSourceUnavailable, err))
	}
	if document == nil {
		return s.recordFailure(
			source,
			"validation",
			errors.Join(ErrSourceUnavailable, errors.New("marketplace catalog: source returned nil document")),
		)
	}
	name := source.binding.Config.Name
	fetched := *document
	fetched.Entries = slices.Clone(document.Entries)
	fetched.FetchedAt = s.now().UTC()
	if fetched.SourceRef != "" && fetched.SourceRef != source.binding.Config.Ref {
		return s.recordFailure(
			source,
			"validation",
			errors.New("marketplace catalog: fetched document changed its origin"),
		)
	}
	fetched.SourceRef, fetched.SourceKind = source.binding.Config.Ref, source.binding.Config.Kind
	for i := range fetched.Entries {
		fetched.Entries[i].FetchedAt = fetched.FetchedAt
	}
	s.sourceMu.RLock()
	if err := s.currentSource(source); err != nil {
		s.sourceMu.RUnlock()
		return failedRefreshOutcome(name, source.generation, "generation_stale"), err
	}
	err = s.store.ReplaceSource(ctx, name, source.generation, &fetched)
	s.sourceMu.RUnlock()
	if err != nil {
		return s.recordFailure(source, "store", err)
	}
	outcome := RefreshOutcome{
		Source:     name,
		Generation: source.generation,
		Outcome:    RefreshOutcomeSucceeded,
		EntryCount: len(fetched.Entries),
	}
	if err := s.notify(outcome); err != nil {
		return outcome, fmt.Errorf("marketplace catalog: persist %q refresh event: %w", name, err)
	}
	return outcome, nil
}

func (s *CatalogService) recordFailure(
	source *registeredSource,
	errorClass string,
	cause error,
) (RefreshOutcome, error) {
	ctx, cancel := s.boundedLifecycleContext()
	defer cancel()
	name := source.binding.Config.Name
	outcome := failedRefreshOutcome(name, source.generation, errorClass)
	s.sourceMu.RLock()
	if err := s.currentSource(source); err != nil {
		s.sourceMu.RUnlock()
		outcome.ErrorClass = "generation_stale"
		return outcome, errors.Join(cause, err)
	}
	redacted := diagnostics.RedactAndBound(cause.Error(), maxStoredErrorBytes)
	markErr := s.store.MarkSourceStale(ctx, name, source.generation, errorClass, redacted)
	if errors.Is(markErr, storepkg.ErrMarketplaceCatalogGenerationStale) {
		s.sourceMu.RUnlock()
		outcome.ErrorClass = "generation_stale"
		return outcome, errors.Join(cause, markErr)
	}
	state, stateErr := s.store.SourceState(ctx, name)
	s.sourceMu.RUnlock()
	outcome.Stale = true
	if stateErr == nil {
		outcome.EntryCount = state.EntryCount
	}
	return outcome, errors.Join(cause, markErr, stateErr, s.notify(outcome))
}

func (s *CatalogService) notify(outcome RefreshOutcome) error {
	if s.notifier == nil {
		return nil
	}
	notifyCtx, cancel := s.boundedLifecycleContext()
	defer cancel()
	notifyErr := s.notifier.NotifyCatalogRefresh(notifyCtx, outcome)
	if lifecycleErr := s.lifecycleError(); lifecycleErr != nil {
		return errors.Join(lifecycleErr, notifyErr)
	}
	return notifyErr
}

func (s *CatalogService) boundedLifecycleContext() (context.Context, context.CancelFunc) {
	s.sourceMu.RLock()
	timeout := s.refreshTimeout
	s.sourceMu.RUnlock()
	return context.WithTimeout(s.lifecycleCtx, timeout)
}

func classifyFetchError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return errorClassCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return errorClassTimeout
	case errors.Is(err, ErrResponseTooLarge):
		return "payload_too_large"
	}
	if matched, ok := errors.AsType[*UnsupportedManifestVersionError](err); ok && matched != nil {
		return "manifest_version"
	}
	if matched, ok := errors.AsType[*httpStatusError](err); ok && matched != nil {
		return "http_status"
	}
	if errors.Is(err, ErrCatalogDecode) {
		return "decode"
	}
	if errors.Is(err, ErrCatalogValidation) {
		return "validation"
	}
	if matched, ok := errors.AsType[*url.Error](err); ok && matched != nil {
		return errorClassNetwork
	}
	return errorClassNetwork
}
