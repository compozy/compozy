package modelcatalog

import (
	"slices"
	"sort"
	"strings"
)

func markRowsStale(rows []ModelRow, lastError string) []ModelRow {
	staleRows := make([]ModelRow, 0, len(rows))
	for _, row := range rows {
		stale := row
		stale.Stale = true
		stale.LastError = RedactString(lastError)
		staleRows = append(staleRows, stale)
	}
	return staleRows
}

func cloneSourceStatuses(statuses []SourceStatus) []SourceStatus {
	return append([]SourceStatus(nil), statuses...)
}

func hasStaleFailureStatus(statuses []SourceStatus) bool {
	for _, status := range statuses {
		if status.Stale && status.RefreshState == RefreshStateFailed && status.RowCount > 0 {
			return true
		}
	}
	return false
}

func sourceProviders(source Source) []string {
	lister, ok := source.(sourceProviderLister)
	if !ok {
		return nil
	}
	return normalizedProviderIDs(lister.ProviderIDs())
}

func filterSourcesByProvider(
	sources []Source,
	providerID string,
	ownedSourceIDs map[string]struct{},
) []Source {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return sources
	}
	filtered := make([]Source, 0, len(sources))
	for _, source := range sources {
		providers := sourceProviders(source)
		_, owned := ownedSourceIDs[source.ID()]
		if len(providers) == 0 || slices.Contains(providers, providerID) || owned {
			filtered = append(filtered, source)
		}
	}
	return filtered
}

func filterStatusesByProviderOwnership(
	statuses []SourceStatus,
	sources map[string]Source,
	providerID string,
) []SourceStatus {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return statuses
	}
	filtered := make([]SourceStatus, 0, len(statuses))
	for _, status := range statuses {
		source, ok := sources[status.SourceID]
		if !ok {
			continue
		}
		providers := sourceProviders(source)
		if len(providers) == 0 || slices.Contains(providers, providerID) {
			filtered = append(filtered, status)
		}
	}
	return filtered
}

func filterModelsBySource(models []Model, sourceID string) []Model {
	trimmed := strings.TrimSpace(sourceID)
	if trimmed == "" {
		return models
	}
	filtered := make([]Model, 0, len(models))
	for _, model := range models {
		for _, source := range model.Sources {
			if source.SourceID == trimmed {
				filtered = append(filtered, model)
				break
			}
		}
	}
	return filtered
}

func normalizedProviderIDs(providerIDs []string) []string {
	providerSet := make(map[string]struct{}, len(providerIDs))
	for _, providerID := range providerIDs {
		trimmed := strings.TrimSpace(providerID)
		if trimmed != "" {
			providerSet[trimmed] = struct{}{}
		}
	}
	providers := make([]string, 0, len(providerSet))
	for providerID := range providerSet {
		providers = append(providers, providerID)
	}
	sort.Strings(providers)
	return providers
}
