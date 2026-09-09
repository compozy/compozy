package modelcatalog

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// OpenCode's verbose CLI pairs each provider/model heading with its JSON model.
// The exact variant keys also form its ACP thought_level options.
func parseOpenCodeModelRows(providerID, output string, now time.Time) ([]ModelRow, error) {
	var rows []ModelRow
	var id string
	var body strings.Builder
	flush := func() error {
		if id == "" {
			return nil
		}
		row := ModelRow{
			ProviderID: providerID, ModelID: id, SourceID: SourceKindProviderLiveID(providerID),
			SourceKind: SourceKindProviderLive, Priority: PriorityProviderLive,
			Available: new(true), RefreshedAt: now,
		}
		if strings.TrimSpace(body.String()) != "" {
			var metadata struct {
				Name         string `json:"name"`
				Capabilities struct {
					Reasoning *bool `json:"reasoning"`
					Toolcall  *bool `json:"toolcall"`
				} `json:"capabilities"`
				Limit struct {
					Context *int64 `json:"context"`
				} `json:"limit"`
				Variants map[string]json.RawMessage `json:"variants"`
			}
			if err := json.Unmarshal([]byte(body.String()), &metadata); err != nil {
				return fmt.Errorf("model catalog: invalid OpenCode metadata for %q: %w", id, err)
			}
			row.DisplayName = metadata.Name
			row.SupportsReasoning = metadata.Capabilities.Reasoning
			row.SupportsTools = metadata.Capabilities.Toolcall
			row.ContextWindow = metadata.Limit.Context
			for variant := range metadata.Variants {
				if !IsValidEffort(variant) {
					return fmt.Errorf("model catalog: invalid OpenCode effort identifier for %q", id)
				}
				row.ReasoningEfforts = append(row.ReasoningEfforts, ReasoningEffort(variant))
			}
			slices.SortFunc(row.ReasoningEfforts, compareReasoningEfforts)
			if len(row.ReasoningEfforts) > 0 {
				row.SupportsReasoning = new(true)
			}
		}
		rows = append(rows, row)
		body.Reset()
		return nil
	}
	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.ContainsAny(trimmed, "\"{}[]") && strings.Contains(trimmed, "/") &&
			len(strings.Fields(trimmed)) == 1 {
			if err := flush(); err != nil {
				return nil, err
			}
			id = trimmed
		} else if id != "" {
			body.WriteString(line)
			body.WriteByte('\n')
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("model catalog: OpenCode command returned no model rows")
	}
	sortModelRowsByID(rows)
	return rows, nil
}
