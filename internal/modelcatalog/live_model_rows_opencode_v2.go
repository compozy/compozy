package modelcatalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// openCodeV2Model decodes the minimal selectable subset of the native v2 model API.
type openCodeV2Model struct {
	ProviderID string `json:"providerID"`
	ID         string `json:"id"`
	Name       string `json:"name"`
	Enabled    *bool  `json:"enabled"`
	Limit      struct {
		Context *int64 `json:"context"`
		Input   *int64 `json:"input"`
		Output  *int64 `json:"output"`
	} `json:"limit"`
	Capabilities struct {
		Tools *bool `json:"tools"`
	} `json:"capabilities"`
	Variants []struct {
		ID string `json:"id"`
	} `json:"variants"`
	Status string `json:"status"`
}

// parseOpenCodeV2ModelRows decodes enabled v2 rows; malformed or empty generations fail wholly.
func parseOpenCodeV2ModelRows(providerID, output string, now time.Time) ([]ModelRow, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return nil, errors.New("model catalog: invalid OpenCode v2 model payload")
	}
	data, ok := envelope["data"]
	if !ok || strings.TrimSpace(string(data)) == "null" {
		return nil, errors.New("model catalog: invalid OpenCode v2 model payload")
	}
	var entries []openCodeV2Model
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, errors.New("model catalog: invalid OpenCode v2 model payload")
	}
	rows := make([]ModelRow, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for index := range entries {
		row, present, err := openCodeV2ModelRow(providerID, &entries[index], now)
		if err != nil {
			return nil, fmt.Errorf("model catalog: invalid OpenCode v2 model payload: entry %d %s", index, err)
		}
		if !present {
			continue
		}
		if _, exists := seen[row.ModelID]; exists {
			continue
		}
		seen[row.ModelID] = struct{}{}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, errors.New("model catalog: OpenCode v2 returned no enabled model rows")
	}
	sortModelRowsByID(rows)
	return rows, nil
}

func openCodeV2ModelRow(providerID string, entry *openCodeV2Model, now time.Time) (ModelRow, bool, error) {
	if entry == nil {
		return ModelRow{}, false, errors.New("is missing")
	}
	nativeProvider := strings.TrimSpace(entry.ProviderID)
	nativeID := strings.TrimSpace(entry.ID)
	if nativeProvider == "" || nativeID == "" ||
		strings.ContainsAny(nativeProvider, "/#") || strings.Contains(nativeID, "#") {
		return ModelRow{}, false, errors.New("has invalid selectable identity")
	}
	if entry.Enabled == nil {
		return ModelRow{}, false, errors.New("is missing the required enabled flag")
	}
	if !*entry.Enabled {
		return ModelRow{}, false, nil
	}
	contextWindow, err := openCodeV2TokenLimit(entry.Limit.Context)
	if err != nil {
		return ModelRow{}, false, err
	}
	maxInput, err := openCodeV2TokenLimit(entry.Limit.Input)
	if err != nil {
		return ModelRow{}, false, err
	}
	maxOutput, err := openCodeV2TokenLimit(entry.Limit.Output)
	if err != nil {
		return ModelRow{}, false, err
	}
	efforts := make([]ReasoningEffort, 0, len(entry.Variants))
	seen := make(map[ReasoningEffort]struct{}, len(entry.Variants))
	for _, variant := range entry.Variants {
		effort := ReasoningEffort(strings.TrimSpace(variant.ID))
		if !IsValidEffort(string(effort)) {
			return ModelRow{}, false, errors.New("has invalid variant identifier")
		}
		if _, exists := seen[effort]; exists {
			continue
		}
		seen[effort] = struct{}{}
		efforts = append(efforts, effort)
	}
	slices.SortFunc(efforts, compareReasoningEfforts)
	available := true
	row := ModelRow{
		ProviderID:       providerID,
		ModelID:          nativeProvider + "/" + nativeID,
		DisplayName:      entry.Name,
		SourceID:         SourceKindProviderLiveID(providerID),
		SourceKind:       SourceKindProviderLive,
		Priority:         PriorityProviderLive,
		Available:        &available,
		RefreshedAt:      now,
		ContextWindow:    contextWindow,
		MaxInputTokens:   maxInput,
		MaxOutputTokens:  maxOutput,
		SupportsTools:    entry.Capabilities.Tools,
		ReasoningEfforts: efforts,
	}
	if len(efforts) > 0 {
		row.SupportsReasoning = new(true)
	}
	if strings.TrimSpace(entry.Status) == "deprecated" {
		row.Deprecated = new(true)
	}
	return row, true, nil
}

// openCodeV2TokenLimit keeps positive limits, treats zero as unknown, and rejects negatives.
func openCodeV2TokenLimit(value *int64) (*int64, error) {
	if value == nil || *value == 0 {
		return nil, nil
	}
	if *value < 0 {
		return nil, errors.New("has invalid token limit")
	}
	return value, nil
}
