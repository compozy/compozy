package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *CatalogService) recordEmptySourceFailure(
	ctx context.Context,
	source Source,
	providerID string,
	executionContext CatalogExecutionContext,
	previousExecutionContext CatalogExecutionContext,
	now time.Time,
	redacted string,
	sourceErr error,
) ([]SourceStatus, refreshOutcome, error) {
	historyExecutionContext, err := failureHistoryExecutionContext(
		executionContext,
		previousExecutionContext,
		sourceRetainsContextPartitions(source),
	)
	if err != nil {
		return nil, refreshOutcomeEmpty, err
	}
	providers, err := s.providersForSource(ctx, source, providerID, historyExecutionContext, now)
	if err != nil {
		return nil, refreshOutcomeEmpty, err
	}
	statuses := make([]SourceStatus, 0, len(providers))
	staleCount := 0
	permanent := IsPermanentSourceFailure(sourceErr)
	sameContext := executionContextIsEmpty(previousExecutionContext)
	for _, provider := range providers {
		previous, listErr := s.store.ListRows(ctx, ListOptions{
			ProviderID:       provider,
			SourceID:         source.ID(),
			ExecutionContext: historyExecutionContext,
			SourceContexts: map[string]CatalogExecutionContext{
				source.ID(): historyExecutionContext,
			},
			IncludeAll:   true,
			IncludeStale: true,
			Now:          now,
		})
		if listErr != nil {
			return nil, refreshOutcomeEmpty, fmt.Errorf(
				"model catalog: load stale rows for %q: %w",
				source.ID(),
				listErr,
			)
		}
		staleRows := markRowsStale(previous, redacted)
		status := sourceStatus(source, provider, now, len(staleRows), true, redacted, RefreshStateFailed)
		previousStatus, err := s.carryPriorFailureState(
			ctx,
			provider,
			historyExecutionContext,
			&status,
			permanent,
		)
		if err != nil {
			return nil, refreshOutcomeEmpty, errors.Join(sourceErr, err)
		}
		// Repeated identical permanent failures preserve the parked row without a write.
		if permanent && sameContext && unchangedPermanentFailure(previousStatus, status) {
			previousStatus.RetainContextPartitions = status.RetainContextPartitions
			staleCount += previousStatus.RowCount
			statuses = append(statuses, previousStatus)
			continue
		}
		if err := s.store.ReplaceSourceRows(
			ctx,
			executionContext,
			source.ID(),
			provider,
			staleRows,
			status,
		); err != nil {
			return nil, refreshOutcomeEmpty, fmt.Errorf(
				"model catalog: persist failed source status: %w",
				err,
			)
		}
		staleCount += len(staleRows)
		statuses = append(statuses, status)
	}
	if staleCount > 0 {
		return statuses, refreshOutcomeStale, sourceErr
	}
	return statuses, refreshOutcomeEmpty, sourceErr
}

func failureHistoryExecutionContext(
	current CatalogExecutionContext,
	previous CatalogExecutionContext,
	denyCrossInvocationTransfer bool,
) (CatalogExecutionContext, error) {
	current, err := NormalizePersistedExecutionContext(current)
	if err != nil {
		return CatalogExecutionContext{}, err
	}
	if executionContextIsEmpty(previous) {
		return current, nil
	}
	previous, err = NormalizePersistedExecutionContext(previous)
	if err != nil {
		return CatalogExecutionContext{}, err
	}
	if previous.Scope != current.Scope ||
		previous.ProfileID != current.ProfileID ||
		previous.WorkspaceID != current.WorkspaceID {
		return CatalogExecutionContext{}, fmt.Errorf(
			"model catalog: previous execution context must match the current ownership scope",
		)
	}
	// Cross-selector stale rows stay stored, but never prove availability for the new invocation.
	if denyCrossInvocationTransfer &&
		strings.TrimSpace(previous.CommandFingerprint) != strings.TrimSpace(current.CommandFingerprint) {
		return current, nil
	}
	return previous, nil
}
