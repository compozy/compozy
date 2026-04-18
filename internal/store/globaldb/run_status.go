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
