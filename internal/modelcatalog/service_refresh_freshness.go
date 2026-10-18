package modelcatalog

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

// maxFailureRefreshBackoff caps how far consecutive source failures defer the next refresh.
const maxFailureRefreshBackoff = time.Hour

// freshSelectedSource answers a non-forced single-source refresh from stored status
// before any row ownership lookup when that source's TTL has not elapsed.
func (s *CatalogService) freshSelectedSource(
	ctx context.Context,
	sources []Source,
	opts RefreshOptions,
	now time.Time,
) ([]SourceStatus, bool) {
	if opts.Force || strings.TrimSpace(opts.SourceID) == "" || len(sources) != 1 {
		return nil, false
	}
	executionContext, err := sourceExecutionContext(opts.SourceContexts, sources[0].ID())
	if err != nil {
		return nil, false
	}
	return freshSourceStatuses(ctx, s.store, sources[0], opts.ProviderID, executionContext, now)
}

// freshSourceStatuses returns the stored statuses of a TTL source when every one of
// them defers its next refresh past now. A provider-scoped check requires that
// provider's status; a providerless check requires every stored provider status plus
// one for each provider the source currently declares.
func freshSourceStatuses(
	ctx context.Context,
	store Store,
	source Source,
	providerID string,
	executionContext CatalogExecutionContext,
	now time.Time,
) ([]SourceStatus, bool) {
	if ttlProvider, ok := source.(sourceTTLProvider); !ok || ttlProvider.TTL() <= 0 {
		return nil, false
	}
	providerID = strings.TrimSpace(providerID)
	statuses, err := store.ListSourceStatus(ctx, StatusOptions{
		ProviderID:       providerID,
		ExecutionContext: executionContext,
		SourceContexts:   singleSourceExecutionContext(source.ID(), executionContext),
	})
	if err != nil {
		return nil, false
	}
	owned := filterStatusesBySource(statuses, source.ID())
	if len(owned) == 0 {
		return nil, false
	}
	for _, status := range owned {
		if !status.FreshAt(now) {
			return nil, false
		}
	}
	required := sourceProviders(source)
	if providerID != "" {
		required = []string{providerID}
	}
	for _, required := range required {
		if !slices.ContainsFunc(owned, func(status SourceStatus) bool { return status.ProviderID == required }) {
			return nil, false
		}
	}
	return owned, true
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

// carryPriorFailureState keeps the prior last-success time on a failed status and
// doubles the refresh deferral after consecutive failures, capped at
// maxFailureRefreshBackoff, so failing probes stop retrying every TTL.
func (s *CatalogService) carryPriorFailureState(
	ctx context.Context,
	providerID string,
	executionContext CatalogExecutionContext,
	status *SourceStatus,
) error {
	statuses, err := s.store.ListSourceStatus(ctx, StatusOptions{
		ProviderID:       providerID,
		ExecutionContext: executionContext,
		SourceContexts: map[string]CatalogExecutionContext{
			status.SourceID: executionContext,
		},
	})
	if err != nil {
		return fmt.Errorf("model catalog: list prior source status for %q/%q: %w", status.SourceID, providerID, err)
	}
	for _, previous := range statuses {
		if previous.SourceID == status.SourceID {
			status.LastSuccess = previous.LastSuccess
			status.NextRefresh = failureBackoffNextRefresh(previous, *status)
			return nil
		}
	}
	return nil
}

func failureBackoffNextRefresh(previous SourceStatus, current SourceStatus) time.Time {
	if current.NextRefresh.IsZero() || current.LastRefresh.IsZero() ||
		previous.RefreshState != RefreshStateFailed ||
		previous.NextRefresh.IsZero() || previous.LastRefresh.IsZero() {
		return current.NextRefresh
	}
	ttl := current.NextRefresh.Sub(current.LastRefresh)
	interval := min(2*previous.NextRefresh.Sub(previous.LastRefresh), max(ttl, maxFailureRefreshBackoff))
	if interval <= ttl {
		return current.NextRefresh
	}
	return current.LastRefresh.Add(interval)
}
