package modelcatalog

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type sourceProviderLister interface {
	ProviderIDs() []string
}

type sourceTTLProvider interface {
	TTL() time.Duration
}

// CatalogService refreshes sources and projects stored model catalog rows.
type CatalogService struct {
	store          Store
	sources        []Source
	sourceByID     map[string]Source
	mergeMu        sync.RWMutex
	mergeOptions   MergeOptions
	lockMu         sync.Mutex
	refreshFlights map[string]*refreshFlight
	onFlightWait   func(providerID string)
}

type refreshFlight struct {
	scopeKey string
	done     chan struct{}
	statuses []SourceStatus
	err      error
}

var _ Service = (*CatalogService)(nil)

// NewService creates a model catalog service from a store and source list.
func NewService(store Store, sources []Source, mergeOptions MergeOptions) (*CatalogService, error) {
	if store == nil {
		return nil, fmt.Errorf("model catalog store is required")
	}
	normalizedSources := make([]Source, 0, len(sources))
	sourceByID := make(map[string]Source, len(sources))
	for _, source := range sources {
		if source == nil {
			return nil, fmt.Errorf("model catalog source is required")
		}
		if err := ValidateSourceIdentity(source.ID(), source.Kind()); err != nil {
			return nil, err
		}
		if source.Priority() <= 0 {
			return nil, fmt.Errorf("model catalog source %q priority must be positive", source.ID())
		}
		if _, exists := sourceByID[source.ID()]; exists {
			return nil, fmt.Errorf("model catalog source %q is registered more than once", source.ID())
		}
		normalizedSources = append(normalizedSources, source)
		sourceByID[source.ID()] = source
	}
	sort.SliceStable(normalizedSources, func(i, j int) bool {
		if normalizedSources[i].Priority() != normalizedSources[j].Priority() {
			return normalizedSources[i].Priority() > normalizedSources[j].Priority()
		}
		return normalizedSources[i].ID() < normalizedSources[j].ID()
	})
	return &CatalogService{
		store:          store,
		sources:        normalizedSources,
		sourceByID:     sourceByID,
		mergeOptions:   cloneMergeOptions(mergeOptions),
		refreshFlights: make(map[string]*refreshFlight),
	}, nil
}

// ListModels returns the merged catalog projection.
func (s *CatalogService) ListModels(ctx context.Context, opts ListOptions) ([]Model, error) {
	if ctx == nil {
		return nil, fmt.Errorf("model catalog list context is required")
	}
	now := defaultNow(opts.Now)
	listOpts := opts
	listOpts.Now = now
	projectionOpts := listOpts
	projectionOpts.SourceID = ""
	rows, err := s.store.ListRows(ctx, projectionOpts)
	if err != nil {
		return nil, fmt.Errorf("model catalog: list stored rows: %w", err)
	}
	needsRefresh := len(rows) == 0
	if strings.TrimSpace(listOpts.SourceID) != "" {
		sourceRows, sourceErr := s.store.ListRows(ctx, listOpts)
		if sourceErr != nil {
			return nil, fmt.Errorf("model catalog: list stored source rows: %w", sourceErr)
		}
		needsRefresh = len(sourceRows) == 0
	}

	var refreshStatuses []SourceStatus
	var refreshErr error
	if opts.Refresh || (needsRefresh && !opts.SkipRefreshIfEmpty && len(s.sources) > 0) {
		refreshStatuses, refreshErr = s.Refresh(ctx, RefreshOptions{
			ProviderID: opts.ProviderID,
			SourceID:   opts.SourceID,
			Force:      opts.Refresh,
			Now:        now,
		})
		rows, err = s.store.ListRows(ctx, projectionOpts)
		if err != nil {
			return nil, fmt.Errorf("model catalog: list stored rows after refresh: %w", err)
		}
	}
	s.mergeMu.RLock()
	mergeOptions := cloneMergeOptions(s.mergeOptions)
	s.mergeMu.RUnlock()
	models := MergeRows(rows, mergeOptions)
	models = filterAuthoritativeProviderModels(models)
	models, err = applyCatalogView(models, opts.View)
	if err != nil {
		return nil, err
	}
	models = filterModelsBySource(models, opts.SourceID)
	if len(models) == 0 && refreshErr != nil && !hasStaleFailureStatus(refreshStatuses) {
		return nil, refreshErr
	}
	return models, nil
}

// UpdateMergeOptions replaces provider capability inputs for subsequent projections.
func (s *CatalogService) UpdateMergeOptions(options MergeOptions) {
	if s == nil {
		return
	}
	s.mergeMu.Lock()
	s.mergeOptions = cloneMergeOptions(options)
	s.mergeMu.Unlock()
}

// Refresh updates registered sources and returns their latest statuses.
func (s *CatalogService) Refresh(ctx context.Context, opts RefreshOptions) ([]SourceStatus, error) {
	if ctx == nil {
		return nil, fmt.Errorf("model catalog refresh context is required")
	}
	now := defaultNow(opts.Now)
	sources, err := s.selectSources(opts.SourceID)
	if err != nil {
		return nil, err
	}
	providerKey := strings.TrimSpace(opts.ProviderID)
	if providerKey == "" {
		return s.refreshAllProviders(ctx, sources, opts, now)
	}
	ownedSourceIDs, err := s.storedSourceIDsForProvider(ctx, providerKey, now)
	if err != nil {
		return nil, err
	}
	sources = filterSourcesByProvider(sources, providerKey, ownedSourceIDs)
	scopeKey := refreshFlightScopeKey(providerKey, opts)

	return s.withRefreshFlight(ctx, providerKey, scopeKey, func() ([]SourceStatus, error) {
		return s.refreshSources(ctx, sources, opts, now)
	})
}

// ListSourceStatus returns provider-scoped source health rows.
func (s *CatalogService) ListSourceStatus(ctx context.Context, providerID string) ([]SourceStatus, error) {
	if ctx == nil {
		return nil, fmt.Errorf("model catalog status context is required")
	}
	statuses, err := s.store.ListSourceStatus(ctx, strings.TrimSpace(providerID))
	if err != nil {
		return nil, fmt.Errorf("model catalog: list source status: %w", err)
	}
	statuses = filterStatusesByProviderOwnership(statuses, s.sourceByID, strings.TrimSpace(providerID))
	for index := range statuses {
		statuses[index].LastError = RedactString(statuses[index].LastError)
	}
	return statuses, nil
}

func (s *CatalogService) persistSourceRows(
	ctx context.Context,
	source Source,
	providerID string,
	rows []ModelRow,
	now time.Time,
	stale bool,
	lastError string,
) ([]SourceStatus, error) {
	grouped := groupRowsByProvider(source, rows)
	providers := providerKeys(grouped)
	if strings.TrimSpace(providerID) != "" && len(providers) == 0 {
		providers = []string{strings.TrimSpace(providerID)}
	}
	if len(providers) == 0 {
		var err error
		providers, err = s.providersForSource(ctx, source, providerID, now)
		if err != nil {
			return nil, err
		}
	}
	statuses := make([]SourceStatus, 0, len(providers))
	state := RefreshStateSucceeded
	if stale {
		state = RefreshStateFailed
	}
	for _, provider := range providers {
		providerRows := grouped[provider]
		for index := range providerRows {
			providerRows[index] = normalizeSourceRow(source, providerRows[index], now, stale, lastError)
		}
		status := sourceStatus(source, provider, now, len(providerRows), stale, lastError, state)
		if stale {
			s.preserveLastSuccess(ctx, provider, &status)
		}
		if err := s.store.ReplaceSourceRows(ctx, source.ID(), provider, providerRows, status); err != nil {
			return nil, fmt.Errorf("model catalog: persist source rows for %q/%q: %w", source.ID(), provider, err)
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (s *CatalogService) persistDisabledSource(
	ctx context.Context,
	source Source,
	providerID string,
	now time.Time,
) ([]SourceStatus, error) {
	providers, err := s.providersForSource(ctx, source, providerID, now)
	if err != nil {
		return nil, err
	}
	statuses := make([]SourceStatus, 0, len(providers))
	for _, provider := range providers {
		status := sourceStatus(source, provider, now, 0, false, "", RefreshStateDisabled)
		if err := s.store.ReplaceSourceRows(ctx, source.ID(), provider, nil, status); err != nil {
			return nil, fmt.Errorf("model catalog: persist disabled source status: %w", err)
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (s *CatalogService) providersForSource(
	ctx context.Context,
	source Source,
	providerID string,
	now time.Time,
) ([]string, error) {
	if trimmed := strings.TrimSpace(providerID); trimmed != "" {
		return []string{trimmed}, nil
	}
	if lister, ok := source.(sourceProviderLister); ok {
		currentProviders := normalizedProviderIDs(lister.ProviderIDs())
		if len(currentProviders) > 0 {
			storedProviders, err := s.storedProvidersForSource(ctx, source, now)
			if err != nil {
				return nil, err
			}
			return normalizedProviderIDs(append(currentProviders, storedProviders...)), nil
		}
	}
	providers, err := s.storedProvidersForSource(ctx, source, now)
	if err != nil {
		return nil, err
	}
	return providers, nil
}

func (s *CatalogService) storedProvidersForSource(ctx context.Context, source Source, now time.Time) ([]string, error) {
	providerSet := make(map[string]struct{})
	statuses, err := s.store.ListSourceStatus(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("model catalog: list stored source providers for %q: %w", source.ID(), err)
	}
	for _, status := range statuses {
		if status.SourceID != source.ID() {
			continue
		}
		if providerID := strings.TrimSpace(status.ProviderID); providerID != "" {
			providerSet[providerID] = struct{}{}
		}
	}
	rows, err := s.store.ListRows(ctx, ListOptions{
		SourceID:     source.ID(),
		IncludeAll:   true,
		IncludeStale: true,
		Now:          now,
	})
	if err != nil {
		return nil, fmt.Errorf("model catalog: list stored source rows for %q: %w", source.ID(), err)
	}
	for _, row := range rows {
		if providerID := strings.TrimSpace(row.ProviderID); providerID != "" {
			providerSet[providerID] = struct{}{}
		}
	}
	providers := make([]string, 0, len(providerSet))
	for providerID := range providerSet {
		providers = append(providers, providerID)
	}
	sort.Strings(providers)
	return providers, nil
}

func (s *CatalogService) selectSources(sourceID string) ([]Source, error) {
	trimmed := strings.TrimSpace(sourceID)
	if trimmed == "" {
		return s.sources, nil
	}
	source, ok := s.sourceByID[trimmed]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrSourceNotRegistered, trimmed)
	}
	return []Source{source}, nil
}

func normalizeSourceRow(source Source, row ModelRow, now time.Time, stale bool, lastError string) ModelRow {
	normalized := row
	if strings.TrimSpace(normalized.SourceID) == "" {
		normalized.SourceID = source.ID()
	}
	if normalized.SourceKind == "" {
		normalized.SourceKind = source.Kind()
	}
	if normalized.Priority == 0 {
		normalized.Priority = source.Priority()
	}
	if normalized.RefreshedAt.IsZero() {
		normalized.RefreshedAt = now
	}
	normalized.Stale = stale || normalized.Stale
	if lastError != "" {
		normalized.LastError = RedactString(lastError)
	} else {
		normalized.LastError = RedactString(normalized.LastError)
	}
	return normalized
}

func sourceStatus(
	source Source,
	providerID string,
	now time.Time,
	rowCount int,
	stale bool,
	lastError string,
	state RefreshState,
) SourceStatus {
	status := SourceStatus{
		SourceID:     source.ID(),
		SourceKind:   source.Kind(),
		ProviderID:   providerID,
		Priority:     source.Priority(),
		LastRefresh:  now,
		RefreshState: state,
		RowCount:     rowCount,
		Stale:        stale,
		LastError:    RedactString(lastError),
	}
	if state == RefreshStateSucceeded {
		status.LastSuccess = now
	}
	if ttlProvider, ok := source.(sourceTTLProvider); ok && ttlProvider.TTL() > 0 {
		status.NextRefresh = now.Add(ttlProvider.TTL())
	}
	return status
}

func (s *CatalogService) preserveLastSuccess(ctx context.Context, providerID string, status *SourceStatus) {
	statuses, err := s.store.ListSourceStatus(ctx, providerID)
	if err != nil {
		return
	}
	for _, previous := range statuses {
		if previous.SourceID == status.SourceID {
			status.LastSuccess = previous.LastSuccess
			return
		}
	}
}

func groupRowsByProvider(source Source, rows []ModelRow) map[string][]ModelRow {
	grouped := make(map[string][]ModelRow)
	for _, row := range rows {
		providerID := strings.TrimSpace(row.ProviderID)
		if providerID == "" {
			continue
		}
		normalized := row
		normalized.SourceID = source.ID()
		normalized.SourceKind = source.Kind()
		normalized.Priority = source.Priority()
		grouped[providerID] = append(grouped[providerID], normalized)
	}
	return grouped
}

func providerKeys(grouped map[string][]ModelRow) []string {
	providers := make([]string, 0, len(grouped))
	for providerID := range grouped {
		providers = append(providers, providerID)
	}
	sort.Strings(providers)
	return providers
}
