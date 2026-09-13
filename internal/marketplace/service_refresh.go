package marketplace

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/compozy/compozy/internal/diagnostics"
	storepkg "github.com/compozy/compozy/internal/store"
)

type refreshFlight struct {
	done    chan struct{}
	outcome RefreshOutcome
	err     error
}

type refreshOnAccessSource interface {
	refreshOnAccess() bool
}

func (s *CatalogService) ensureFresh(ctx context.Context) error {
	if source, ok := s.source.(refreshOnAccessSource); ok && source.refreshOnAccess() {
		_, err := s.withRefreshFlight(ctx)
		return err
	}
	state, err := s.store.SourceState(ctx, CompozyCatalogSource)
	switch {
	case errors.Is(err, ErrSourceStateMissing):
		_, err = s.withRefreshFlight(ctx)
		return err
	case err != nil:
		return err
	case state.Stale || state.FetchedAt.IsZero() || !state.FetchedAt.Add(s.ttl).After(s.now().UTC()):
		_, err = s.withRefreshFlight(ctx)
		return err
	default:
		return nil
	}
}

func (s *CatalogService) withRefreshFlight(ctx context.Context) (RefreshOutcome, error) {
	if err := ctx.Err(); err != nil {
		return canceledRefreshOutcome(), err
	}
	s.flightMu.Lock()
	if s.closed {
		s.flightMu.Unlock()
		return canceledRefreshOutcome(), ErrServiceClosed
	}
	if existing := s.flight; existing != nil {
		s.flightMu.Unlock()
		return awaitRefreshFlight(ctx, existing)
	}
	flight := &refreshFlight{done: make(chan struct{})}
	s.flight = flight
	s.flightWG.Add(1)
	refreshCtx, cancel := context.WithTimeout(s.lifecycleCtx, s.refreshTimeout)
	s.flightMu.Unlock()

	// The service owns this flight until completion. Its service-level
	// deadline bounds the lifetime independently of any individual caller.
	go s.runRefreshFlight(refreshCtx, cancel, flight)
	return awaitRefreshFlight(ctx, flight)
}

func (s *CatalogService) runRefreshFlight(
	ctx context.Context,
	cancel context.CancelFunc,
	flight *refreshFlight,
) {
	defer s.flightWG.Done()
	defer cancel()
	flight.outcome, flight.err = s.refreshSource(ctx)
	s.flightMu.Lock()
	s.flight = nil
	close(flight.done)
	s.flightMu.Unlock()
}

func awaitRefreshFlight(ctx context.Context, flight *refreshFlight) (RefreshOutcome, error) {
	select {
	case <-flight.done:
		return flight.outcome, flight.err
	case <-ctx.Done():
		return canceledRefreshOutcome(), ctx.Err()
	}
}

func canceledRefreshOutcome() RefreshOutcome {
	return RefreshOutcome{
		Source:     CompozyCatalogSource,
		Outcome:    RefreshOutcomeFailed,
		ErrorClass: errorClassCanceled,
	}
}

func (s *CatalogService) refreshSource(ctx context.Context) (RefreshOutcome, error) {
	source := s.source
	now := s.now().UTC()
	generation := int64(0)

	state, err := s.store.SourceState(ctx, CompozyCatalogSource)
	if err != nil && !errors.Is(err, ErrSourceStateMissing) {
		return canceledRefreshOutcome(), err
	}
	if state != nil {
		generation = state.Generation
	}

	document, err := source.Fetch(ctx)
	if err != nil {
		if lifecycleErr := s.lifecycleError(); lifecycleErr != nil {
			return canceledRefreshOutcome(), errors.Join(lifecycleErr, err)
		}
		return s.recordFailure(
			generation,
			classifyFetchError(err),
			errors.Join(ErrSourceUnavailable, err),
		)
	}
	if document == nil {
		return s.recordFailure(
			generation,
			"validation",
			errors.Join(ErrSourceUnavailable, errors.New("marketplace catalog: source returned nil document")),
		)
	}
	document.FetchedAt = now
	for index := range document.Entries {
		document.Entries[index].FetchedAt = now
	}
	if lifecycleErr := s.lifecycleError(); lifecycleErr != nil {
		return canceledRefreshOutcome(), lifecycleErr
	}

	err = s.store.ReplaceSource(ctx, CompozyCatalogSource, generation, document)

	if err != nil {
		return s.recordFailure(generation, "store", err)
	}
	outcome := RefreshOutcome{
		Source: CompozyCatalogSource, Generation: generation,
		Outcome:    RefreshOutcomeSucceeded,
		EntryCount: len(document.Entries),
	}
	if err := s.notify(outcome); err != nil {
		return outcome, fmt.Errorf("marketplace catalog: persist %q refresh event: %w", CompozyCatalogSource, err)
	}
	return outcome, nil
}

func (s *CatalogService) recordFailure(
	generation int64,
	errorClass string,
	cause error,
) (RefreshOutcome, error) {
	// Fetch/store failures may arrive because the refresh deadline expired. Use
	// a fresh service-owned deadline so failure state and its event outlive that
	// dead operation context while Close can still cancel and join the work.
	failureCtx, cancel := s.boundedLifecycleContext()
	defer cancel()
	redacted := diagnostics.RedactAndBound(cause.Error(), maxStoredErrorBytes)
	markErr := s.store.MarkSourceStale(failureCtx, CompozyCatalogSource, generation, errorClass, redacted)

	if errors.Is(markErr, storepkg.ErrMarketplaceCatalogGenerationStale) {
		return RefreshOutcome{Source: CompozyCatalogSource, Generation: generation,
			Outcome: RefreshOutcomeFailed, ErrorClass: "generation_stale"}, errors.Join(cause, markErr)
	}
	state, stateErr := s.store.SourceState(failureCtx, CompozyCatalogSource)
	outcome := RefreshOutcome{
		Outcome: RefreshOutcomeFailed,
		Stale:   true,
		Source:  CompozyCatalogSource, Generation: generation,
		ErrorClass: errorClass,
	}
	if stateErr == nil {
		outcome.EntryCount = state.EntryCount
	}
	notifyErr := s.notify(outcome)
	return outcome, errors.Join(cause, markErr, stateErr, notifyErr)
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
	return context.WithTimeout(s.lifecycleCtx, s.refreshTimeout)
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
