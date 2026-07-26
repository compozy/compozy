package memory

import (
	"context"
	"errors"
	"fmt"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	storepkg "github.com/compozy/agh/internal/store"
)

// ListMemoryEventSummaries returns canonical memory events from every visible
// memory authority, keeping workspace DB rows visible to observe adapters.
func (s *Store) ListMemoryEventSummaries(
	ctx context.Context,
	workspaces []string,
	query storepkg.EventSummaryQuery,
) ([]storepkg.EventSummary, error) {
	if ctx == nil {
		return nil, errors.New("memory: event summary context is required")
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}

	sources, err := s.observabilitySources(ctx, workspaces)
	if err != nil {
		return nil, err
	}

	summaries := make([]storepkg.EventSummary, 0)
	var listErr error
	for _, source := range sources {
		sourceSummaries, err := source.catalog.listEventSummaries(ctx, source.id, source.filters, query)
		if err != nil {
			listErr = fmt.Errorf("memory: list %s memory events: %w", source.id, err)
			break
		}
		summaries = append(summaries, sourceSummaries...)
	}
	if err := errors.Join(listErr, closeObservabilitySources(ctx, sources)); err != nil {
		return nil, err
	}

	sortEventSummaries(summaries)
	return clampEventSummaries(summaries, query.Limit), nil
}

// HealthStats returns derived-catalog stats for the visible scopes.
func (s *Store) HealthStats(ctx context.Context, workspaces []string) (memcontract.HealthStats, error) {
	if ctx == nil {
		return memcontract.HealthStats{}, errors.New("memory: health stats context is required")
	}
	if s.catalog == nil {
		return memcontract.HealthStats{}, nil
	}

	sources, err := s.healthSources(ctx, workspaces)
	if err != nil {
		return memcontract.HealthStats{}, err
	}

	accumulator := newHealthAccumulator()
	var statsErr error
	for _, source := range sources {
		if err := accumulator.addSource(ctx, source); err != nil {
			statsErr = err
			break
		}
	}
	if err := errors.Join(statsErr, closeObservabilitySources(ctx, sources)); err != nil {
		return memcontract.HealthStats{}, err
	}
	return accumulator.stats(), nil
}
