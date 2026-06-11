package globaldb

import "strings"

const (
	runStatusStarting  = "starting"
	runStatusRunning   = "running"
	runStatusCompleted = "completed"
	runStatusFailed    = "failed"
	runStatusCanceled  = "canceled"
	runStatusCrashed   = "crashed"
)

func normalizeRunStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "canceled" {
		return runStatusCanceled
	}
	return normalized
}

func normalizeRunStatusFilters(status string, statuses []string) []string {
	seen := make(map[string]struct{}, len(statuses)+1)
	filters := make([]string, 0, len(statuses)+1)
	appendStatus := func(raw string) {
		for _, candidate := range strings.Split(raw, ",") {
			normalized := normalizeRunStatus(candidate)
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			filters = append(filters, normalized)
		}
	}
	appendStatus(status)
	for _, candidate := range statuses {
		appendStatus(candidate)
	}
	return filters
}
