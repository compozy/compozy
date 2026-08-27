package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

type sourceListBootstrapper interface {
	BootstrapOnList() bool
}

const bootstrapRetryDelay = 50 * time.Millisecond

func (s *CatalogService) bootstrapSourcesOnList(
	ctx context.Context,
	opts ListOptions,
	now time.Time,
) ([]SourceStatus, bool, error) {
	if opts.Refresh || opts.SkipRefreshIfEmpty {
		return nil, false, nil
	}
	sources, err := s.selectSources(opts.SourceID)
	if err != nil {
		return nil, false, err
	}
	requestedProvider := strings.TrimSpace(opts.ProviderID)
	statuses := make([]SourceStatus, 0)
	handled := false
	var refreshErrs []error
	for _, source := range sources {
		bootstrapper, ok := source.(sourceListBootstrapper)
		if !ok || !bootstrapper.BootstrapOnList() {
			continue
		}
		providers := sourceProviders(source)
		if requestedProvider != "" {
			if len(providers) > 0 && !slices.Contains(providers, requestedProvider) {
				continue
			}
			providers = []string{requestedProvider}
		}
		for _, providerID := range providers {
			executionContext, contextErr := sourceExecutionContext(opts.SourceContexts, source.ID())
			if contextErr != nil {
				return statuses, handled, contextErr
			}
			handled = true
			refreshed, refreshErr := s.withBootstrapFlight(
				ctx,
				source.ID(),
				providerID,
				executionContext,
				func(retry bool) ([]SourceStatus, error) {
					if !retry {
						attempted, statusErr := s.sourceRefreshAttempted(
							ctx,
							source.ID(),
							providerID,
							executionContext,
						)
						if statusErr != nil {
							return nil, statusErr
						}
						if attempted {
							return nil, nil
						}
					}
					return s.Refresh(ctx, RefreshOptions{
						ProviderID:       providerID,
						SourceID:         source.ID(),
						ExecutionContext: opts.ExecutionContext,
						Force:            retry,
						Now:              now,
					})
				},
			)
			statuses = append(statuses, refreshed...)
			if refreshErr != nil {
				refreshErrs = append(refreshErrs, refreshErr)
			}
		}
	}
	return statuses, handled, errors.Join(refreshErrs...)
}

func (s *CatalogService) withBootstrapFlight(
	ctx context.Context,
	sourceID string,
	providerID string,
	executionContext CatalogExecutionContext,
	fn func(retry bool) ([]SourceStatus, error),
) ([]SourceStatus, error) {
	contextID, err := executionContext.ID()
	if err != nil {
		return nil, err
	}
	key := sourceID + "\x00" + providerID + "\x00" + contextID
	retry := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		s.bootstrapMu.Lock()
		flight := s.bootstrapFlights[key]
		if flight == nil {
			flight = &bootstrapFlight{done: make(chan struct{})}
			s.bootstrapFlights[key] = flight
			s.bootstrapMu.Unlock()

			flight.statuses, flight.err = fn(retry)
			flight.retryable = ctx.Err() != nil &&
				(errors.Is(flight.err, context.Canceled) || errors.Is(flight.err, context.DeadlineExceeded))
			s.bootstrapMu.Lock()
			close(flight.done)
			delete(s.bootstrapFlights, key)
			s.bootstrapMu.Unlock()
			return cloneSourceStatuses(flight.statuses), flight.err
		}
		s.bootstrapMu.Unlock()

		if hook := s.onBootstrapWait; hook != nil {
			hook(sourceID, providerID)
		}
		select {
		case <-flight.done:
			if flight.retryable {
				retry = true
				if err := waitForBootstrapRetry(ctx); err != nil {
					return nil, err
				}
				continue
			}
			return cloneSourceStatuses(flight.statuses), flight.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func waitForBootstrapRetry(ctx context.Context) error {
	timer := time.NewTimer(bootstrapRetryDelay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *CatalogService) sourceRefreshAttempted(
	ctx context.Context,
	sourceID string,
	providerID string,
	executionContext CatalogExecutionContext,
) (bool, error) {
	statuses, err := s.store.ListSourceStatus(ctx, StatusOptions{
		ProviderID:       providerID,
		ExecutionContext: executionContext,
		SourceContexts: map[string]CatalogExecutionContext{
			sourceID: executionContext,
		},
	})
	if err != nil {
		return false, fmt.Errorf(
			"model catalog: list bootstrap status for %q/%q: %w",
			sourceID,
			providerID,
			err,
		)
	}
	for _, status := range statuses {
		if status.SourceID == sourceID {
			return true, nil
		}
	}
	return false, nil
}
