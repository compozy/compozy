package globaldb

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/modelcatalog"
)

func normalizeModelCatalogRowReasoning(row *modelcatalog.ModelRow) error {
	if row.DefaultReasoningEffort != nil {
		effort := modelcatalog.ReasoningEffort(strings.TrimSpace(string(*row.DefaultReasoningEffort)))
		switch {
		case effort == "":
			row.DefaultReasoningEffort = nil
		case !modelcatalog.IsValidEffort(string(effort)):
			return fmt.Errorf("default reasoning effort %q is unsupported", effort)
		default:
			row.DefaultReasoningEffort = &effort
		}
	}
	for index, effort := range row.ReasoningEfforts {
		trimmed := modelcatalog.ReasoningEffort(strings.TrimSpace(string(effort)))
		if trimmed == "" {
			return fmt.Errorf("reasoning effort %d is required", index)
		}
		if !modelcatalog.IsValidEffort(string(trimmed)) {
			return fmt.Errorf("reasoning effort %d value %q is unsupported", index, trimmed)
		}
		row.ReasoningEfforts[index] = trimmed
	}
	return nil
}
