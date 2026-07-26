package modelcatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
)

const (
	modelsdevClaudeKey     = "claude"
	modelsdevCodexKey      = "codex"
	modelsdevMoonshotKey   = "moonshot"
	modelsdevOpenrouterKey = "openrouter"
)

const maxModelsDevPayloadBytes = 16 << 20

var defaultModelsDevProviderMapping = map[string]string{
	"anthropic":            modelsdevClaudeKey,
	modelsdevClaudeKey:     modelsdevClaudeKey,
	"google":               string(liveAuthGemini),
	string(liveAuthGemini): string(liveAuthGemini),
	liveSourcesOpenaiKey:   modelsdevCodexKey,
	modelsdevCodexKey:      modelsdevCodexKey,
	modelsdevOpenrouterKey: modelsdevOpenrouterKey,
	modelsdevMoonshotKey:   modelsdevMoonshotKey,
	"kimi":                 modelsdevMoonshotKey,
	"xai":                  "xai",
	"mistral":              "mistral",
	"groq":                 "groq",
	"minimax":              "minimax",
}

type modelsDevCache struct {
	expiresAt time.Time
	rows      []ModelRow
}

// ModelsDevSource fetches catalog enrichment from models.dev.
type ModelsDevSource struct {
	endpoint    string
	enabled     bool
	ttl         time.Duration
	timeout     time.Duration
	client      *http.Client
	providerIDs map[string]struct{}
	providerMap map[string]string
	mu          sync.Mutex
	cache       *modelsDevCache
}

var _ Source = (*ModelsDevSource)(nil)

// ModelsDevSourceOption customizes models.dev source construction.
type ModelsDevSourceOption func(*ModelsDevSource)

// WithModelsDevHTTPClient injects the explicit-timeout HTTP client used for models.dev fetches.
func WithModelsDevHTTPClient(client *http.Client) ModelsDevSourceOption {
	return func(source *ModelsDevSource) {
		if source != nil && client != nil {
			source.client = client
		}
	}
}

// NewModelsDevSource creates a configured models.dev source.
func NewModelsDevSource(
	providers map[string]aghconfig.ProviderConfig,
	cfg aghconfig.ModelsDevSourceConfig,
	options ...ModelsDevSourceOption,
) (*ModelsDevSource, error) {
	ttl, err := time.ParseDuration(cfg.EffectiveTTL())
	if err != nil {
		return nil, fmt.Errorf("model catalog: parse models.dev ttl: %w", err)
	}
	timeout, err := time.ParseDuration(cfg.EffectiveTimeout())
	if err != nil {
		return nil, fmt.Errorf("model catalog: parse models.dev timeout: %w", err)
	}
	source := &ModelsDevSource{
		endpoint:    strings.TrimSpace(cfg.EffectiveEndpoint()),
		enabled:     cfg.EffectiveEnabled(),
		ttl:         ttl,
		timeout:     timeout,
		client:      &http.Client{Timeout: timeout},
		providerIDs: knownProviderIDs(providers),
		providerMap: cloneProviderMapping(defaultModelsDevProviderMapping),
	}
	for _, option := range options {
		if option != nil {
			option(source)
		}
	}
	if source.client == nil || source.client.Timeout <= 0 {
		return nil, fmt.Errorf("model catalog: models.dev client timeout must be positive")
	}
	return source, nil
}

func (s *ModelsDevSource) ID() string {
	return SourceIDModelsDev
}

func (s *ModelsDevSource) Kind() SourceKind {
	return SourceKindModelsDev
}

func (s *ModelsDevSource) Priority() int {
	return PriorityModelsDev
}

func (s *ModelsDevSource) TTL() time.Duration {
	return s.ttl
}

// Timeout returns the explicit HTTP timeout used by the source.
func (s *ModelsDevSource) Timeout() time.Duration {
	return s.timeout
}

func (s *ModelsDevSource) ProviderIDs() []string {
	providers := make([]string, 0, len(s.providerIDs))
	for providerID := range s.providerIDs {
		providers = append(providers, providerID)
	}
	sort.Strings(providers)
	return providers
}

func (s *ModelsDevSource) ListModels(ctx context.Context, opts ListOptions) ([]ModelRow, error) {
	if ctx == nil {
		return nil, fmt.Errorf("model catalog: models.dev context is required")
	}
	if !s.enabled {
		return nil, ErrSourceDisabled
	}
	now := defaultNow(opts.Now)
	if !opts.Refresh {
		if rows, ok := s.cachedRows(now, opts.ProviderID, false, ""); ok {
			return rows, nil
		}
	}
	rows, err := s.fetchRows(ctx, now)
	if err != nil {
		if cached, ok := s.cachedRows(now, opts.ProviderID, true, sourceErrorText(err)); ok {
			return cached, &StaleFallbackError{SourceID: s.ID(), Err: err}
		}
		return nil, err
	}
	s.mu.Lock()
	s.cache = &modelsDevCache{
		expiresAt: now.Add(s.ttl),
		rows:      cloneModelRows(rows),
	}
	s.mu.Unlock()
	return filterRowsByProvider(rows, opts.ProviderID), nil
}

func (s *ModelsDevSource) cachedRows(
	now time.Time,
	providerID string,
	stale bool,
	lastError string,
) ([]ModelRow, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil {
		return nil, false
	}
	if !stale && !s.cache.expiresAt.IsZero() && !s.cache.expiresAt.After(now) {
		return nil, false
	}
	rows := cloneModelRows(s.cache.rows)
	for index := range rows {
		rows[index].Stale = stale
		rows[index].LastError = RedactString(lastError)
	}
	return filterRowsByProvider(rows, providerID), true
}

func (s *ModelsDevSource) fetchRows(ctx context.Context, now time.Time) (rows []ModelRow, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("model catalog: create models.dev request: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("model catalog: fetch models.dev catalog: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("model catalog: close models.dev response: %w", closeErr)
		}
	}()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("model catalog: models.dev returned HTTP %d", resp.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxModelsDevPayloadBytes))
	var payload modelsDevPayload
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("model catalog: decode models.dev catalog: %w", err)
	}
	return s.parsePayload(payload, now), nil
}

func (s *ModelsDevSource) parsePayload(payload modelsDevPayload, now time.Time) []ModelRow {
	rows := make([]ModelRow, 0)
	for providerKey, provider := range payload {
		rawProviderID := provider.ID
		if strings.TrimSpace(rawProviderID) == "" {
			rawProviderID = providerKey
		}
		providerID := s.resolveProviderID(rawProviderID)
		if providerID == "" || len(provider.Models) == 0 {
			continue
		}
		modelKeys := make([]string, 0, len(provider.Models))
		for modelKey := range provider.Models {
			modelKeys = append(modelKeys, modelKey)
		}
		sort.Strings(modelKeys)
		for _, modelKey := range modelKeys {
			row, ok := modelsDevRow(providerID, modelKey, provider.Models[modelKey], now)
			if ok {
				rows = append(rows, row)
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].ProviderID != rows[j].ProviderID {
			return rows[i].ProviderID < rows[j].ProviderID
		}
		return rows[i].ModelID < rows[j].ModelID
	})
	return rows
}

func (s *ModelsDevSource) resolveProviderID(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return ""
	}
	if mapped := s.providerMap[normalized]; mapped != "" {
		if _, ok := s.providerIDs[mapped]; ok {
			return mapped
		}
		return ""
	}
	if _, ok := s.providerIDs[normalized]; ok {
		return normalized
	}
	return ""
}

type modelsDevPayload map[string]modelsDevProvider

type modelsDevProvider struct {
	ID     string                       `json:"id"`
	Models map[string]modelsDevRawModel `json:"models"`
}

type modelsDevRawModel struct {
	ID                      string         `json:"id"`
	Name                    string         `json:"name"`
	ReleaseDate             string         `json:"release_date"`
	Status                  string         `json:"status"`
	Reasoning               *bool          `json:"reasoning"`
	SupportsReasoning       *bool          `json:"supportsReasoning"`
	SupportsReasoningLegacy *bool          `json:"supports_reasoning"`
	ToolCall                *bool          `json:"tool_call"`
	SupportsTools           *bool          `json:"supportsTools"`
	SupportsToolsLegacy     *bool          `json:"supports_tools"`
	Limit                   modelsDevLimit `json:"limit"`
	ContextWindow           *int64         `json:"contextWindow"`
	MaxInputTokens          *int64         `json:"maxInputTokens"`
	MaxOutputTokens         *int64         `json:"maxOutputTokens"`
	Cost                    modelsDevCost  `json:"cost"`
	Pricing                 modelsDevCost  `json:"pricing"`
}

type modelsDevLimit struct {
	Context *int64 `json:"context"`
	Input   *int64 `json:"input"`
	Output  *int64 `json:"output"`
}

type modelsDevCost struct {
	Input      *float64 `json:"input"`
	Output     *float64 `json:"output"`
	CacheRead  *float64 `json:"cache_read"`
	CacheWrite *float64 `json:"cache_write"`
	Reasoning  *float64 `json:"reasoning"`
}

func modelsDevRow(providerID string, modelKey string, raw modelsDevRawModel, now time.Time) (ModelRow, bool) {
	modelID := strings.TrimSpace(raw.ID)
	if modelID == "" {
		modelID = strings.TrimSpace(modelKey)
	}
	if modelID == "" {
		return ModelRow{}, false
	}
	releaseDate, releaseDateErr := NormalizeReleaseDate(raw.ReleaseDate)
	if releaseDateErr != nil {
		releaseDate = nil
	}
	row := ModelRow{
		ProviderID:               providerID,
		ModelID:                  modelID,
		DisplayName:              strings.TrimSpace(raw.Name),
		SourceID:                 SourceIDModelsDev,
		SourceKind:               SourceKindModelsDev,
		Priority:                 PriorityModelsDev,
		RefreshedAt:              now,
		ContextWindow:            firstInt64(raw.Limit.Context, raw.ContextWindow),
		MaxInputTokens:           firstInt64(raw.Limit.Input, raw.MaxInputTokens),
		MaxOutputTokens:          firstInt64(raw.Limit.Output, raw.MaxOutputTokens),
		SupportsTools:            firstBool(raw.ToolCall, raw.SupportsTools, raw.SupportsToolsLegacy),
		SupportsReasoning:        firstBool(raw.Reasoning, raw.SupportsReasoning, raw.SupportsReasoningLegacy),
		CostInputPerMillion:      firstFloat64(raw.Cost.Input, raw.Pricing.Input),
		CostOutputPerMillion:     firstFloat64(raw.Cost.Output, raw.Pricing.Output),
		CostCacheReadPerMillion:  firstFloat64(raw.Cost.CacheRead, raw.Pricing.CacheRead),
		CostCacheWritePerMillion: firstFloat64(raw.Cost.CacheWrite, raw.Pricing.CacheWrite),
		CostReasoningPerMillion:  firstFloat64(raw.Cost.Reasoning, raw.Pricing.Reasoning),
		Deprecated:               modelsDevDeprecated(raw.Status),
		ReleaseDate:              releaseDate,
	}
	return row, true
}

func modelsDevDeprecated(status string) *bool {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return nil
	}
	return new(strings.EqualFold(trimmed, "deprecated"))
}

func firstBool(values ...*bool) *bool {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstInt64(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstFloat64(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func knownProviderIDs(providers map[string]aghconfig.ProviderConfig) map[string]struct{} {
	known := make(map[string]struct{})
	for providerID := range aghconfig.BuiltinProviders() {
		known[providerID] = struct{}{}
	}
	for providerID := range providers {
		known[providerID] = struct{}{}
	}
	return known
}

func cloneProviderMapping(src map[string]string) map[string]string {
	cloned := make(map[string]string, len(src))
	maps.Copy(cloned, src)
	return cloned
}

func filterRowsByProvider(rows []ModelRow, providerID string) []ModelRow {
	trimmed := strings.TrimSpace(providerID)
	if trimmed == "" {
		return rows
	}
	filtered := make([]ModelRow, 0, len(rows))
	for _, row := range rows {
		if row.ProviderID == trimmed {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func cloneModelRows(rows []ModelRow) []ModelRow {
	cloned := make([]ModelRow, len(rows))
	for index, row := range rows {
		cloned[index] = row
		cloned[index].Available = cloneModelRowPointer(row.Available)
		cloned[index].ContextWindow = cloneModelRowPointer(row.ContextWindow)
		cloned[index].MaxInputTokens = cloneModelRowPointer(row.MaxInputTokens)
		cloned[index].MaxOutputTokens = cloneModelRowPointer(row.MaxOutputTokens)
		cloned[index].SupportsTools = cloneModelRowPointer(row.SupportsTools)
		cloned[index].SupportsReasoning = cloneModelRowPointer(row.SupportsReasoning)
		cloned[index].ReasoningEfforts = append([]ReasoningEffort(nil), row.ReasoningEfforts...)
		cloned[index].DefaultReasoningEffort = cloneModelRowPointer(row.DefaultReasoningEffort)
		cloned[index].CostInputPerMillion = cloneModelRowPointer(row.CostInputPerMillion)
		cloned[index].CostOutputPerMillion = cloneModelRowPointer(row.CostOutputPerMillion)
		cloned[index].CostCacheReadPerMillion = cloneModelRowPointer(row.CostCacheReadPerMillion)
		cloned[index].CostCacheWritePerMillion = cloneModelRowPointer(row.CostCacheWritePerMillion)
		cloned[index].CostReasoningPerMillion = cloneModelRowPointer(row.CostReasoningPerMillion)
		cloned[index].Deprecated = cloneModelRowPointer(row.Deprecated)
		cloned[index].Hidden = cloneModelRowPointer(row.Hidden)
		cloned[index].Featured = cloneModelRowPointer(row.Featured)
		cloned[index].ReleaseDate = cloneStringPtr(row.ReleaseDate)
	}
	return cloned
}

func cloneModelRowPointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
