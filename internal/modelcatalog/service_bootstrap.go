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
			attempted, statusErr := s.sourceRefreshAttempted(ctx, source.ID(), providerID)
			if statusErr != nil {
				return statuses, handled, statusErr
			}
			handled = true
			if attempted {
				continue
			}
			refreshed, refreshErr := s.Refresh(ctx, RefreshOptions{
				ProviderID: providerID,
				SourceID:   source.ID(),
				Now:        now,
			})
			statuses = append(statuses, refreshed...)
			if refreshErr != nil {
				refreshErrs = append(refreshErrs, refreshErr)
			}
		}
	}
	return statuses, handled, errors.Join(refreshErrs...)
}

func (s *CatalogService) sourceRefreshAttempted(
	ctx context.Context,
	sourceID string,
	providerID string,
) (bool, error) {
	statuses, err := s.store.ListSourceStatus(ctx, providerID)
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
