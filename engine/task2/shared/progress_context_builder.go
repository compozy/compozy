package shared

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/task"
)

// BuildProgressContext transforms ProgressState into template context variables
// This function creates a map of progress-related variables that can be used
// in collection and parallel task templates for conditional logic and monitoring
func BuildProgressContext(_ context.Context, state *task.ProgressState) map[string]any {
	if state == nil {
		// NOTE: Provide zeroed progress context so templates don't nil panic.
		return map[string]any{
			"total":          0,
			"completed":      0,
			"success":        0,
			"failed":         0,
			"canceled":       0,
			"timedOut":       0,
			"terminal":       0,
			"running":        0,
			"pending":        0,
			"completionRate": 0.0,
			"failureRate":    0.0,
			"overallStatus":  "unknown",
			"statusType":     nil,
			"elapsedSeconds": 0.0,
		}
	}
	var elapsedSeconds float64
	if !state.StartTime.IsZero() {
		elapsedSeconds = time.Since(state.StartTime).Seconds()
	}
	return map[string]any{
		"total":          state.TotalChildren,
		"completed":      state.SuccessCount, // Keep "completed" key for template compatibility
		"success":        state.SuccessCount, // New explicit success key
		"failed":         state.FailedCount,
		"canceled":       state.CanceledCount,
		"timedOut":       state.TimedOutCount,
		"terminal":       state.TerminalCount,
		"running":        state.RunningCount,
		"pending":        state.PendingCount,
		"completionRate": state.CompletionRate(),
		"failureRate":    state.FailureRate(),
		"overallStatus":  state.OverallStatusString(), // Use string version for template compatibility
		"statusType":     state.OverallStatus(),       // Provide core.StatusType version as well
		"elapsedSeconds": elapsedSeconds,
	}
}
