package modelcatalog

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

// maxFailureRefreshBackoff caps exponential retries for transient failures; permanent failures stay parked.
const maxFailureRefreshBackoff = 2 * time.Hour

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

// carryPriorFailureState preserves last success and either parks the source or backs off a transient failure.
func (s *CatalogService) carryPriorFailureState(
	ctx context.Context,
	providerID string,
	executionContext CatalogExecutionContext,
	status *SourceStatus,
	permanent bool,
) (SourceStatus, error) {
	statuses, err := s.store.ListSourceStatus(ctx, StatusOptions{
		ProviderID:       providerID,
		ExecutionContext: executionContext,
		SourceContexts: map[string]CatalogExecutionContext{
			status.SourceID: executionContext,
		},
	})
	if err != nil {
		return SourceStatus{}, fmt.Errorf(
			"model catalog: list prior source status for %q/%q: %w",
			status.SourceID,
			providerID,
			err,
		)
	}
	var previous SourceStatus
	for _, candidate := range statuses {
		if candidate.SourceID == status.SourceID {
			previous = candidate
			status.LastSuccess = candidate.LastSuccess
			break
		}
	}
	status.NextRefresh = failureNextRefresh(previous, *status, permanent)
	return previous, nil
}

// unchangedPermanentFailure avoids writing the same parked status again.
func unchangedPermanentFailure(previous SourceStatus, current SourceStatus) bool {
	return previous.PermanentFailureHeld() && current.PermanentFailureHeld() &&
		previous.SourceKind == current.SourceKind &&
		previous.Priority == current.Priority &&
		previous.LastError == current.LastError &&
		previous.RowCount == current.RowCount &&
		previous.Stale == current.Stale
}

func failureNextRefresh(previous SourceStatus, current SourceStatus, permanent bool) time.Time {
	if current.NextRefresh.IsZero() || current.LastRefresh.IsZero() {
		return current.NextRefresh
	}
	if permanent {
		return time.Time{}
	}
	ttl := current.NextRefresh.Sub(current.LastRefresh)
	// A transient failure after a permanent hold restarts the doubling from the TTL.
	if previous.RefreshState != RefreshStateFailed || previous.PermanentFailureHeld() ||
		previous.NextRefresh.IsZero() || previous.LastRefresh.IsZero() {
		return current.NextRefresh
	}
	interval := min(2*previous.NextRefresh.Sub(previous.LastRefresh), max(ttl, maxFailureRefreshBackoff))
	if interval <= ttl {
		return current.NextRefresh
	}
	return current.LastRefresh.Add(interval)
}
