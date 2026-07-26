package modelcatalog

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

const defaultReasoningEffortField = "default_reasoning_effort"

// ProviderConfigSource projects one replaceable provider-config snapshot.
type ProviderConfigSource struct {
	id        string
	kind      SourceKind
	priority  int
	mu        sync.RWMutex
	providers map[string]aghconfig.ProviderConfig
}

var _ Source = (*ProviderConfigSource)(nil)

// NewBuiltinSource creates the offline bootstrap source from AGH built-ins.
func NewBuiltinSource() Source {
	return newProviderConfigSource(
		SourceIDBuiltin,
		SourceKindBuiltin,
		PriorityBuiltin,
		builtinProviderConfigs(),
	)
}

// NewConfigSource creates the operator config model source.
func NewConfigSource(providers map[string]aghconfig.ProviderConfig) *ProviderConfigSource {
	return newProviderConfigSource(SourceIDConfig, SourceKindConfig, PriorityConfig, providers)
}

func newProviderConfigSource(
	id string,
	kind SourceKind,
	priority int,
	providers map[string]aghconfig.ProviderConfig,
) *ProviderConfigSource {
	return &ProviderConfigSource{
		id:        id,
		kind:      kind,
		priority:  priority,
		providers: aghconfig.CloneProviderConfigs(providers),
	}
}

func (s *ProviderConfigSource) ID() string {
	return s.id
}

func (s *ProviderConfigSource) Kind() SourceKind {
	return s.kind
}

func (s *ProviderConfigSource) Priority() int {
	return s.priority
}

func (s *ProviderConfigSource) ProviderIDs() []string {
	s.mu.RLock()
	providers := providerConfigIDs(s.providers)
	s.mu.RUnlock()
	return providers
}

func (s *ProviderConfigSource) ListModels(
	_ context.Context,
	opts ListOptions,
) ([]ModelRow, error) {
	now := defaultNow(opts.Now)
	s.mu.RLock()
	providersSnapshot := aghconfig.CloneProviderConfigs(s.providers)
	s.mu.RUnlock()
	providers := providerConfigIDs(providersSnapshot)
	rows := make([]ModelRow, 0)
	for _, providerID := range providers {
		if opts.ProviderID != "" && opts.ProviderID != providerID {
			continue
		}
		provider := providersSnapshot[providerID]
		providerRows, err := providerModelRows(providerID, provider.Models, s.id, s.kind, s.priority, now)
		if err != nil {
			return nil, fmt.Errorf("model catalog: project provider config %q: %w", providerID, err)
		}
		rows = append(rows, providerRows...)
	}
	return rows, nil
}

func providerConfigIDs(providers map[string]aghconfig.ProviderConfig) []string {
	ids := make([]string, 0, len(providers))
	for providerID := range providers {
		ids = append(ids, providerID)
	}
	sort.Strings(ids)
	return ids
}

// ReplaceProviders atomically replaces the source snapshot used by subsequent refreshes.
func (s *ProviderConfigSource) ReplaceProviders(providers map[string]aghconfig.ProviderConfig) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.providers = aghconfig.CloneProviderConfigs(providers)
	s.mu.Unlock()
}

func providerModelRows(
	providerID string,
	models aghconfig.ProviderModelsConfig,
	sourceID string,
	kind SourceKind,
	priority int,
	now time.Time,
) ([]ModelRow, error) {
	byID := make(map[string]ModelRow)
	order := make([]string, 0, len(models.Curated)+1)
	addModel := func(modelID string) ModelRow {
		trimmed := strings.TrimSpace(modelID)
		row, ok := byID[trimmed]
		if ok {
			return row
		}
		row = ModelRow{
			ProviderID:  providerID,
			ModelID:     trimmed,
			SourceID:    sourceID,
			SourceKind:  kind,
			Priority:    priority,
			RefreshedAt: now,
		}
		byID[trimmed] = row
		order = append(order, trimmed)
		return row
	}
	if defaultModel := strings.TrimSpace(models.Default); defaultModel != "" {
		addModel(defaultModel)
	}
	for _, curated := range models.Curated {
		modelID := strings.TrimSpace(curated.ID)
		if modelID == "" {
			continue
		}
		row := addModel(modelID)
		row.ExplicitlyCurated = true
		if err := enrichRowFromProviderModel(&row, curated); err != nil {
			return nil, err
		}
		byID[modelID] = row
	}
	rows := make([]ModelRow, 0, len(order))
	for _, modelID := range order {
		rows = append(rows, byID[modelID])
	}
	return rows, nil
}

func enrichRowFromProviderModel(row *ModelRow, model aghconfig.ProviderModelConfig) error {
	row.DisplayName = strings.TrimSpace(model.DisplayName)
	row.ContextWindow = model.ContextWindow
	row.MaxInputTokens = model.MaxInputTokens
	row.MaxOutputTokens = model.MaxOutputTokens
	row.SupportsTools = model.SupportsTools
	row.SupportsReasoning = model.SupportsReasoning
	row.CostInputPerMillion = model.CostInputPerMillion
	row.CostOutputPerMillion = model.CostOutputPerMillion
	row.CostCacheReadPerMillion = model.CostCacheReadPerMillion
	row.CostCacheWritePerMillion = model.CostCacheWritePerMillion
	row.CostReasoningPerMillion = model.CostReasoningPerMillion
	row.Deprecated = cloneBoolPtr(model.Deprecated)
	row.Hidden = cloneBoolPtr(model.Hidden)
	row.Featured = cloneBoolPtr(model.Featured)
	if releaseDate, err := NormalizeReleaseDate(model.ReleaseDate); err == nil {
		row.ReleaseDate = releaseDate
	} else {
		row.LastError = RedactString(err.Error())
	}
	if len(model.ReasoningEfforts) > 0 {
		row.ReasoningEfforts = make([]ReasoningEffort, 0, len(model.ReasoningEfforts))
		for index, effort := range model.ReasoningEfforts {
			trimmed := strings.TrimSpace(effort)
			if !IsValidEffort(trimmed) {
				return &InvalidReasoningEffortError{
					ProviderID: row.ProviderID,
					ModelID:    row.ModelID,
					Field:      fmt.Sprintf("reasoning_efforts[%d]", index),
					Effort:     trimmed,
				}
			}
			row.ReasoningEfforts = append(row.ReasoningEfforts, ReasoningEffort(trimmed))
		}
	}
	if effort := strings.TrimSpace(model.DefaultReasoningEffort); effort != "" {
		if !IsValidEffort(effort) {
			return &InvalidReasoningEffortError{
				ProviderID: row.ProviderID,
				ModelID:    row.ModelID,
				Field:      defaultReasoningEffortField,
				Effort:     effort,
			}
		}
		defaultEffort := ReasoningEffort(effort)
		if len(row.ReasoningEfforts) > 0 && !slices.Contains(row.ReasoningEfforts, defaultEffort) {
			return &InvalidReasoningEffortError{
				ProviderID: row.ProviderID,
				ModelID:    row.ModelID,
				Field:      defaultReasoningEffortField,
				Effort:     effort,
			}
		}
		row.DefaultReasoningEffort = &defaultEffort
	}
	return nil
}
