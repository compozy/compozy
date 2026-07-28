package bridges

import (
	"fmt"
	"strings"
)

func normalizeBoundedBridgeInstanceIDs(input []string) ([]string, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("%w: at least one bridge instance id is required", ErrBridgeCatalogQueryInvalid)
	}
	if len(input) > MaxBridgeCatalogLimit {
		return nil, fmt.Errorf(
			"%w: bridge instance id batch cannot exceed %d",
			ErrBridgeCatalogQueryInvalid,
			MaxBridgeCatalogLimit,
		)
	}

	ids := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, raw := range input {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, fmt.Errorf("%w: bridge instance id is required", ErrBridgeCatalogQueryInvalid)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}
