package shared

import (
	"time"

	"github.com/compozy/compozy/engine/task"
)

// BuildProgressContext transforms ProgressState into template context variables
// This function creates a map of progress-related variables that can be used
// in collection and parallel task templates for conditional logic and monitoring
func BuildProgressContext(state *task.ProgressState) map[string]any {
	// Calculate elapsed time, handling zero time case
	var elapsedSeconds float64
	if !state.StartTime.IsZero() {
		elapsedSeconds = time.Since(state.StartTime).Seconds()
	}

	return map[string]any{
		"total":          state.TotalChildren,
		"completed":      state.CompletedCount,
		"failed":         state.FailedCount,
		"running":        state.RunningCount,
		"pending":        state.PendingCount,
		"completionRate": state.CompletionRate(),
		"failureRate":    state.FailureRate(),
		"overallStatus":  state.OverallStatusString(), // Use string version for template compatibility
		"statusType":     state.OverallStatus(),       // Provide core.StatusType version as well
		"elapsedSeconds": elapsedSeconds,
	}
}
