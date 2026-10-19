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
		denyCrossInvocationTransfer(source),
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
		if err := s.carryPriorFailureState(ctx, provider, historyExecutionContext, &status); err != nil {
			return nil, refreshOutcomeEmpty, errors.Join(sourceErr, err)
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
	// A changed OpenCode invocation selector partitions rows: prior rows stay
	// stored but never prove availability for the new invocation. Explicit
	// discovery commands on other providers retain their established stale
	// behavior.
	if denyCrossInvocationTransfer &&
		strings.TrimSpace(previous.CommandFingerprint) != strings.TrimSpace(current.CommandFingerprint) {
		return current, nil
	}
	return previous, nil
}

// denyCrossInvocationTransfer reports whether stale rows must never cross an
// invocation-selector change. Only the automatic OpenCode partition embeds
// the resolved selector; other providers keep their established stale
// transfer behavior.
func denyCrossInvocationTransfer(source Source) bool {
	if source == nil {
		return false
	}
	return strings.TrimSpace(source.ID()) == SourceKindProviderLiveID(liveSourcesOpencodeKey)
}
