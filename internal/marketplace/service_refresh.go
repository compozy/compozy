package marketplace

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/compozy/compozy/internal/diagnostics"
)

type refreshFlight struct {
	done    chan struct{}
	outcome RefreshOutcome
	err     error
}

type refreshOnAccessSource interface {
	refreshOnAccess() bool
}

func (s *CatalogService) ensureFresh(ctx context.Context, kind Kind) error {
	if source, ok := s.sources[kind].(refreshOnAccessSource); ok && source.refreshOnAccess() {
		_, err := s.withRefreshFlight(ctx, kind)
		return err
	}
	state, err := s.store.KindState(ctx, kind)
	switch {
	case errors.Is(err, ErrKindStateMissing):
		_, err = s.withRefreshFlight(ctx, kind)
		return err
	case err != nil:
		return err
	case state.Stale || state.FetchedAt.IsZero() || !state.FetchedAt.Add(s.ttl).After(s.now().UTC()):
		_, err = s.withRefreshFlight(ctx, kind)
		return err
	default:
		return nil
	}
}

func (s *CatalogService) withRefreshFlight(ctx context.Context, kind Kind) (RefreshOutcome, error) {
	if err := ctx.Err(); err != nil {
		return canceledRefreshOutcome(kind), err
	}
	s.flightMu.Lock()
	if s.closed {
		s.flightMu.Unlock()
		return canceledRefreshOutcome(kind), ErrServiceClosed
	}
	if existing := s.flights[kind]; existing != nil {
		s.flightMu.Unlock()
		return awaitRefreshFlight(ctx, kind, existing)
	}
	flight := &refreshFlight{done: make(chan struct{})}
	s.flights[kind] = flight
	s.flightWG.Add(1)
	refreshCtx, cancel := context.WithTimeout(s.lifecycleCtx, s.refreshTimeout)
	s.flightMu.Unlock()

	// The flight map owns this goroutine until completion. Its service-level
	// deadline bounds the lifetime independently of any individual caller.
	go s.runRefreshFlight(refreshCtx, cancel, kind, flight)
	return awaitRefreshFlight(ctx, kind, flight)
}

func (s *CatalogService) runRefreshFlight(
	ctx context.Context,
	cancel context.CancelFunc,
	kind Kind,
	flight *refreshFlight,
) {
	defer s.flightWG.Done()
	defer cancel()
	flight.outcome, flight.err = s.refreshKind(ctx, kind)
	s.flightMu.Lock()
	delete(s.flights, kind)
	close(flight.done)
	s.flightMu.Unlock()
}

func awaitRefreshFlight(ctx context.Context, kind Kind, flight *refreshFlight) (RefreshOutcome, error) {
	select {
	case <-flight.done:
		return flight.outcome, flight.err
	case <-ctx.Done():
		return canceledRefreshOutcome(kind), ctx.Err()
	}
}

func canceledRefreshOutcome(kind Kind) RefreshOutcome {
	return RefreshOutcome{
		Kind:       kind,
		Outcome:    RefreshOutcomeFailed,
		ErrorClass: errorClassCanceled,
	}
}

func (s *CatalogService) refreshKind(ctx context.Context, kind Kind) (RefreshOutcome, error) {
	source := s.sources[kind]
	now := s.now().UTC()
	document, err := source.Fetch(ctx)
	if err != nil {
		if lifecycleErr := s.lifecycleError(); lifecycleErr != nil {
			return canceledRefreshOutcome(kind), errors.Join(lifecycleErr, err)
		}
		return s.recordFailure(
			kind,
			classifyFetchError(err),
			errors.Join(ErrSourceUnavailable, err),
		)
	}
	if document == nil {
		return s.recordFailure(
			kind,
			"validation",
			errors.Join(ErrSourceUnavailable, errors.New("marketplace catalog: source returned nil document")),
		)
	}
	document.FetchedAt = now
	for index := range document.Entries {
		document.Entries[index].FetchedAt = now
	}
	if lifecycleErr := s.lifecycleError(); lifecycleErr != nil {
		return canceledRefreshOutcome(kind), lifecycleErr
	}
	if err := s.store.ReplaceKind(ctx, kind, document); err != nil {
		return s.recordFailure(kind, "store", err)
	}
	outcome := RefreshOutcome{
		Kind:       kind,
		Outcome:    RefreshOutcomeSucceeded,
		EntryCount: len(document.Entries),
	}
	if err := s.notify(outcome); err != nil {
		return outcome, fmt.Errorf("marketplace catalog: persist %q refresh event: %w", kind, err)
	}
	return outcome, nil
}

func (s *CatalogService) recordFailure(
	kind Kind,
	errorClass string,
	cause error,
) (RefreshOutcome, error) {
	// Fetch/store failures may arrive because the refresh deadline expired. Use
	// a fresh service-owned deadline so failure state and its event outlive that
	// dead operation context while Close can still cancel and join the work.
	failureCtx, cancel := s.boundedLifecycleContext()
	defer cancel()
	redacted := diagnostics.RedactAndBound(cause.Error(), maxStoredErrorBytes)
	markErr := s.store.MarkKindStale(failureCtx, kind, errorClass, redacted)
	state, stateErr := s.store.KindState(failureCtx, kind)
	outcome := RefreshOutcome{
		Kind:       kind,
		Outcome:    RefreshOutcomeFailed,
		Stale:      true,
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
