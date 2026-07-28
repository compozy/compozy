package modelcatalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type livePayloadEnvelope struct {
	Data   []liveRawModel `json:"data"`
	Models []liveRawModel `json:"models"`
}

type liveRawModel struct {
	ID                         string          `json:"id"`
	Name                       string          `json:"name"`
	Model                      string          `json:"model"`
	Value                      string          `json:"value"`
	Label                      string          `json:"label"`
	DisplayName                string          `json:"display_name"`
	DisplayNameCamel           string          `json:"displayName"`
	ContextWindow              *int64          `json:"context_window"`
	ContextLength              *int64          `json:"context_length"`
	MaxTokens                  *int64          `json:"max_tokens"`
	MaxInputTokens             *int64          `json:"max_input_tokens"`
	MaxInputTokensCamel        *int64          `json:"maxInputTokens"`
	MaxOutputTokens            *int64          `json:"max_output_tokens"`
	MaxOutputTokensCamel       *int64          `json:"maxOutputTokens"`
	InputTokenLimit            *int64          `json:"inputTokenLimit"`
	OutputTokenLimit           *int64          `json:"outputTokenLimit"`
	SupportedGenerationMethods []string        `json:"supportedGenerationMethods"`
	SupportedParameters        []string        `json:"supported_parameters"`
	SupportsTools              *bool           `json:"supports_tools"`
	SupportsToolsCamel         *bool           `json:"supportsTools"`
	ToolCall                   *bool           `json:"tool_call"`
	SupportsReasoning          *bool           `json:"supports_reasoning"`
	SupportsReasoningCamel     *bool           `json:"supportsReasoning"`
	SupportsEffort             *bool           `json:"supportsEffort"`
	ReasoningEfforts           []string        `json:"reasoning_efforts"`
	SupportedEffortLevels      []string        `json:"supportedEffortLevels"`
	DefaultReasoningEffort     string          `json:"default_reasoning_effort"`
	Pricing                    liveRawPricing  `json:"pricing"`
	Cost                       liveRawPricing  `json:"cost"`
	Raw                        json.RawMessage `json:"-"`
}

type liveRawPricing struct {
	Input      json.RawMessage `json:"input"`
	Output     json.RawMessage `json:"output"`
	Prompt     json.RawMessage `json:"prompt"`
	Completion json.RawMessage `json:"completion"`
	CacheRead  json.RawMessage `json:"cache_read"`
	CacheWrite json.RawMessage `json:"cache_write"`
	Reasoning  json.RawMessage `json:"reasoning"`
}

func parseLiveModelPayload(providerID string, payload []byte, now time.Time) ([]ModelRow, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("model catalog: live discovery for %q returned empty payload", providerID)
	}
	rawModels, err := decodeLiveRawModels(trimmed)
	if err != nil {
		return nil, err
	}
	rows := make([]ModelRow, 0, len(rawModels))
	seen := make(map[string]struct{}, len(rawModels))
	for index := range rawModels {
		row, ok := liveModelRow(providerID, &rawModels[index], now)
		if !ok {
			continue
		}
		if _, exists := seen[row.ModelID]; exists {
			continue
		}
		seen[row.ModelID] = struct{}{}
		rows = append(rows, row)
	}
	sortModelRowsByID(rows)
	return rows, nil
}

func decodeLiveRawModels(payload []byte) ([]liveRawModel, error) {
	var array []liveRawModel
	if err := json.Unmarshal(payload, &array); err == nil && len(array) > 0 {
		return array, nil
	}
	var envelope livePayloadEnvelope
	if err := json.Unmarshal(payload, &envelope); err == nil {
		if len(envelope.Data) > 0 {
			return envelope.Data, nil
		}
		if len(envelope.Models) > 0 {
			return envelope.Models, nil
		}
	}
	var objectMap map[string]liveRawModel
	if err := json.Unmarshal(payload, &objectMap); err == nil && len(objectMap) > 0 {
		keys := make([]string, 0, len(objectMap))
		for key := range objectMap {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		models := make([]liveRawModel, 0, len(keys))
		for _, key := range keys {
			model := objectMap[key]
			if strings.TrimSpace(model.ID) == "" {
				model.ID = key
			}
			models = append(models, model)
		}
		return models, nil
	}
	return nil, fmt.Errorf("model catalog: live discovery payload did not contain model rows")
}

func liveModelRow(providerID string, raw *liveRawModel, now time.Time) (ModelRow, bool) {
	if raw == nil {
		return ModelRow{}, false
	}
	modelID := firstNonBlank(raw.ID, raw.Model, raw.Value, raw.Name)
	modelID = strings.TrimPrefix(modelID, "models/")
	if modelID == "" {
		return ModelRow{}, false
	}
	available := true
	row := ModelRow{
		ProviderID:     providerID,
		ModelID:        modelID,
		DisplayName:    firstNonBlank(raw.DisplayName, raw.DisplayNameCamel, raw.Label, raw.Name),
		SourceID:       SourceKindProviderLiveID(providerID),
		SourceKind:     SourceKindProviderLive,
		Priority:       PriorityProviderLive,
		Available:      &available,
		RefreshedAt:    now,
		ContextWindow:  firstInt64(raw.ContextWindow, raw.ContextLength),
		MaxInputTokens: firstInt64(raw.MaxInputTokens, raw.MaxInputTokensCamel, raw.InputTokenLimit),
		MaxOutputTokens: firstInt64(
			raw.MaxOutputTokens,
			raw.MaxOutputTokensCamel,
			raw.MaxTokens,
			raw.OutputTokenLimit,
		),
		SupportsTools:            liveSupportsTools(raw),
		SupportsReasoning:        firstBool(raw.SupportsReasoning, raw.SupportsReasoningCamel, raw.SupportsEffort),
		ReasoningEfforts:         normalizedReasoningEfforts(raw.ReasoningEfforts, raw.SupportedEffortLevels),
		CostInputPerMillion:      livePricePerMillion(raw.Cost.Input, raw.Pricing.Input, raw.Pricing.Prompt),
		CostOutputPerMillion:     livePricePerMillion(raw.Cost.Output, raw.Pricing.Output, raw.Pricing.Completion),
		CostCacheReadPerMillion:  livePricePerMillion(raw.Cost.CacheRead, raw.Pricing.CacheRead),
		CostCacheWritePerMillion: livePricePerMillion(raw.Cost.CacheWrite, raw.Pricing.CacheWrite),
		CostReasoningPerMillion:  livePricePerMillion(raw.Cost.Reasoning, raw.Pricing.Reasoning),
		DefaultReasoningEffort:   normalizedDefaultReasoningEffort(raw.DefaultReasoningEffort),
	}
	if row.SupportsReasoning == nil && len(row.ReasoningEfforts) > 0 {
		value := true
		row.SupportsReasoning = &value
	}
	return row, true
}

func parseLineModelRows(providerID string, stdout string, now time.Time) []ModelRow {
	lines := strings.Split(stdout, "\n")
	rows := make([]ModelRow, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		firstToken := strings.Fields(line)[0]
		if strings.EqualFold(firstToken, "id") || strings.EqualFold(firstToken, "model") {
			continue
		}
		if !strings.Contains(firstToken, "/") && providerID == liveSourcesOpencodeKey {
			continue
		}
		modelID := strings.TrimSpace(firstToken)
		if modelID == "" {
			continue
		}
		if _, exists := seen[modelID]; exists {
			continue
		}
		seen[modelID] = struct{}{}
		available := true
		rows = append(rows, ModelRow{
			ProviderID:  providerID,
			ModelID:     modelID,
			DisplayName: modelID,
			SourceID:    SourceKindProviderLiveID(providerID),
			SourceKind:  SourceKindProviderLive,
			Priority:    PriorityProviderLive,
			Available:   &available,
			RefreshedAt: now,
		})
	}
	sortModelRowsByID(rows)
	return rows
}

func liveSupportsTools(raw *liveRawModel) *bool {
	if raw == nil {
		return nil
	}
	if value := firstBool(raw.SupportsTools, raw.SupportsToolsCamel, raw.ToolCall); value != nil {
		return value
	}
	for _, parameter := range raw.SupportedParameters {
		normalized := strings.ToLower(strings.TrimSpace(parameter))
		if normalized == "tools" || normalized == "tool_choice" {
			value := true
			return &value
		}
	}
	for _, method := range raw.SupportedGenerationMethods {
		if strings.EqualFold(strings.TrimSpace(method), "generateContent") {
			value := true
			return &value
		}
	}
	return nil
}

func normalizedReasoningEfforts(groups ...[]string) []ReasoningEffort {
	efforts := make([]ReasoningEffort, 0)
	seen := make(map[ReasoningEffort]struct{})
	for _, group := range groups {
		for _, raw := range group {
			effort, ok := normalizeReasoningEffort(raw)
			if !ok {
				continue
			}
			if _, exists := seen[effort]; exists {
				continue
			}
			seen[effort] = struct{}{}
			efforts = append(efforts, effort)
		}
	}
	return efforts
}

func normalizedDefaultReasoningEffort(raw string) *ReasoningEffort {
	effort, ok := normalizeReasoningEffort(raw)
	if !ok {
		return nil
	}
	return &effort
}

func normalizeReasoningEffort(raw string) (ReasoningEffort, bool) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if !IsValidEffort(normalized) {
		return "", false
	}
	return ReasoningEffort(normalized), true
}

func livePricePerMillion(values ...json.RawMessage) *float64 {
	for _, raw := range values {
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		value, ok := parseJSONFloat(raw)
		if !ok {
			continue
		}
		perMillion := value * 1_000_000
		return &perMillion
	}
	return nil
}

func parseJSONFloat(raw json.RawMessage) (float64, bool) {
	var number float64
	if err := json.Unmarshal(raw, &number); err == nil {
		return number, true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func sortModelRowsByID(rows []ModelRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].ModelID < rows[j].ModelID
	})
}
