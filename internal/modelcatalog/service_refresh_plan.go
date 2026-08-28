package modelcatalog

import (
	"context"
	"errors"
	"maps"
	"sort"
	"strings"
)

// RefreshPlan buffers source replacements until a whole catalog generation is ready.
type RefreshPlan struct {
	owner *CatalogService
	store *refreshPlanStore
}

// NewRefreshPlan creates an isolated, read-through refresh generation.
func (s *CatalogService) NewRefreshPlan() (*RefreshPlan, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("model catalog: refresh plan service is required")
	}
	store := &refreshPlanStore{
		base:         s.store,
		replacements: make(map[string]SourceRowsReplacement),
	}
	return &RefreshPlan{
		owner: s,
		store: store,
	}, nil
}

// Refresh stages one source refresh without publishing it to the durable store.
func (p *RefreshPlan) Refresh(
	ctx context.Context,
	source Source,
	opts RefreshOptions,
) ([]SourceStatus, error) {
	if p == nil || p.store == nil {
		return nil, errors.New("model catalog: refresh plan is required")
	}
	if ctx == nil {
		return nil, errors.New("model catalog: refresh plan context is required")
	}
	if source == nil {
		return nil, errors.New("model catalog: refresh plan source is required")
	}
	contexts, err := resolveSourceExecutionContexts([]Source{source}, opts.ExecutionContext)
	if err != nil {
		return nil, err
	}
	opts.SourceID = source.ID()
	opts.SourceContexts = contexts
	staging := &CatalogService{store: p.store}
	return staging.refreshSources(ctx, []Source{source}, opts, defaultNow(opts.Now))
}

// CommitRefreshPlan atomically publishes every staged source replacement.
func (s *CatalogService) CommitRefreshPlan(ctx context.Context, plan *RefreshPlan) error {
	if ctx == nil {
		return errors.New("model catalog: commit refresh plan context is required")
	}
	if plan == nil || plan.store == nil || plan.owner != s {
		return errors.New("model catalog: refresh plan does not belong to service")
	}
	return s.store.ReplaceSourceRowsBatch(ctx, plan.store.snapshot())
}

type refreshPlanStore struct {
	base         Store
	replacements map[string]SourceRowsReplacement
}

var _ Store = (*refreshPlanStore)(nil)

func (s *refreshPlanStore) ReplaceSourceRows(
	ctx context.Context,
	executionContext CatalogExecutionContext,
	sourceID string,
	providerID string,
	rows []ModelRow,
	status SourceStatus,
) error {
	if ctx == nil {
		return errors.New("model catalog: stage source rows context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := refreshPlanSourceProviderKey(executionContext, sourceID, providerID)
	if err != nil {
		return err
	}
	s.replacements[key] = SourceRowsReplacement{
		ExecutionContext: executionContext,
		SourceID:         sourceID,
		ProviderID:       providerID,
		Rows:             cloneModelRows(rows),
		Status:           status,
	}
	return nil
}

func (s *refreshPlanStore) ReplaceSourceRowsBatch(
	ctx context.Context,
	replacements []SourceRowsReplacement,
) error {
	if ctx == nil {
		return errors.New("model catalog: stage source rows batch context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	next := make(map[string]SourceRowsReplacement, len(s.replacements)+len(replacements))
	maps.Copy(next, s.replacements)
	for _, replacement := range replacements {
		key, err := refreshPlanSourceProviderKey(
			replacement.ExecutionContext,
			replacement.SourceID,
			replacement.ProviderID,
		)
		if err != nil {
			return err
		}
		replacement.Rows = cloneModelRows(replacement.Rows)
		next[key] = replacement
	}
	s.replacements = next
	return nil
}

func (s *refreshPlanStore) ListRows(ctx context.Context, opts ListOptions) ([]ModelRow, error) {
	rows, err := s.base.ListRows(ctx, opts)
	if err != nil {
		return nil, err
	}
	result := make([]ModelRow, 0, len(rows))
	for _, row := range rows {
		if replacementMatchesRow(s.replacements, row, opts) {
			continue
		}
		result = append(result, row)
	}
	for _, replacement := range s.replacements {
		if !replacementMatchesList(replacement, opts) {
			continue
		}
		for _, row := range replacement.Rows {
			if row.Stale && !opts.IncludeAll && !opts.IncludeStale {
				continue
			}
			result = append(result, row)
		}
	}
	return cloneModelRows(result), nil
}

func (s *refreshPlanStore) ListSourceStatus(
	ctx context.Context,
	opts StatusOptions,
) ([]SourceStatus, error) {
	statuses, err := s.base.ListSourceStatus(ctx, opts)
	if err != nil {
		return nil, err
	}
	result := make([]SourceStatus, 0, len(statuses)+len(s.replacements))
	for _, status := range statuses {
		if replacementMatchesStatus(s.replacements, status, opts) {
			continue
		}
		result = append(result, status)
	}
	trimmedProviderID := strings.TrimSpace(opts.ProviderID)
	for _, replacement := range s.replacements {
		if (trimmedProviderID == "" || replacement.ProviderID == trimmedProviderID) &&
			replacementContextSelected(replacement, opts.SourceContexts) {
			result = append(result, replacement.Status)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ProviderID != result[j].ProviderID {
			return result[i].ProviderID < result[j].ProviderID
		}
		return result[i].SourceID < result[j].SourceID
	})
	return cloneSourceStatuses(result), nil
}

func (s *refreshPlanStore) snapshot() []SourceRowsReplacement {
	keys := make([]string, 0, len(s.replacements))
	for key := range s.replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	replacements := make([]SourceRowsReplacement, 0, len(keys))
	for _, key := range keys {
		replacement := s.replacements[key]
		replacement.Rows = cloneModelRows(replacement.Rows)
		replacements = append(replacements, replacement)
	}
	return replacements
}

func replacementMatchesList(replacement SourceRowsReplacement, opts ListOptions) bool {
	if !replacementContextSelected(replacement, opts.SourceContexts) {
		return false
	}
	providerID := strings.TrimSpace(opts.ProviderID)
	if providerID != "" && replacement.ProviderID != providerID {
		return false
	}
	sourceID := strings.TrimSpace(opts.SourceID)
	return sourceID == "" || replacement.SourceID == sourceID
}

func refreshPlanSourceProviderKey(
	executionContext CatalogExecutionContext,
	sourceID string,
	providerID string,
) (string, error) {
	contextID, err := executionContext.ID()
	if err != nil {
		return "", err
	}
	return contextID + "\x00" + strings.TrimSpace(sourceID) + "\x00" + strings.TrimSpace(providerID), nil
}

func replacementMatchesRow(
	replacements map[string]SourceRowsReplacement,
	row ModelRow,
	opts ListOptions,
) bool {
	return replacementMatchesSourceProvider(replacements, opts.SourceContexts, row.SourceID, row.ProviderID)
}

func replacementMatchesStatus(
	replacements map[string]SourceRowsReplacement,
	status SourceStatus,
	opts StatusOptions,
) bool {
	return replacementMatchesSourceProvider(replacements, opts.SourceContexts, status.SourceID, status.ProviderID)
}

func replacementMatchesSourceProvider(
	replacements map[string]SourceRowsReplacement,
	contexts map[string]CatalogExecutionContext,
	sourceID string,
	providerID string,
) bool {
	executionContext, ok := contexts[sourceID]
	if !ok {
		return false
	}
	key, err := refreshPlanSourceProviderKey(executionContext, sourceID, providerID)
	if err != nil {
		return false
	}
	_, replaced := replacements[key]
	return replaced
}

func replacementContextSelected(
	replacement SourceRowsReplacement,
	contexts map[string]CatalogExecutionContext,
) bool {
	selected, ok := contexts[replacement.SourceID]
	if !ok {
		return false
	}
	selectedID, err := selected.ID()
	if err != nil {
		return false
	}
	replacementID, err := replacement.ExecutionContext.ID()
	return err == nil && selectedID == replacementID
}
