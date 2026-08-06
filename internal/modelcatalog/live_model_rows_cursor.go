package modelcatalog

import (
	"fmt"
	"strings"
	"time"
)

func parseCursorModelRows(providerID string, output string, now time.Time) ([]ModelRow, error) {
	rows := make([]ModelRow, 0)
	seen := make(map[string]struct{})
	for rawLine := range strings.SplitSeq(output, "\n") {
		modelID, displayName, ok := strings.Cut(strings.TrimSpace(rawLine), " - ")
		modelID = strings.TrimSpace(modelID)
		displayName = strings.TrimSpace(displayName)
		if !ok || modelID == "" || displayName == "" {
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
			DisplayName: displayName,
			SourceID:    SourceKindProviderLiveID(providerID),
			SourceKind:  SourceKindProviderLive,
			Priority:    PriorityProviderLive,
			Available:   &available,
			RefreshedAt: now,
		})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("model catalog: cursor model command returned no model rows")
	}
	sortModelRowsByID(rows)
	return rows, nil
}
