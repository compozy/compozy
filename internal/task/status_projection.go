package task

import (
	"strings"

	eventspkg "github.com/compozy/agh/internal/events"
)

const (
	taskHookRunStarted   = "task.run.started"
	taskHookRunCompleted = "task.run.completed"
	taskHookRunFailed    = "task.run.failed"
	taskHookRunCanceled  = "task.run.canceled"
)

// StatusProjectionEventType returns the canonical transition type included in
// durable Network task-status projections. Empty means the event is not a
// status transition.
func StatusProjectionEventType(eventType string) string {
	switch strings.TrimSpace(eventType) {
	case eventspkg.TaskCanceled,
		eventspkg.TaskNeedsAttention,
		eventspkg.TaskRecovered,
		eventspkg.TaskPaused,
		eventspkg.TaskResumed,
		eventspkg.TaskRunEnqueued,
		eventspkg.TaskRunClaimed,
		eventspkg.TaskRunStarting,
		eventspkg.TaskRunSessionBound,
		eventspkg.TaskRunStarted,
		eventspkg.TaskRunCompleted,
		eventspkg.TaskRunFailed,
		eventspkg.TaskRunCanceled,
		eventspkg.TaskRunForceStopped,
		eventspkg.TaskRunRecovered,
		eventspkg.TaskRunRejected,
		eventspkg.TaskRunLeaseExpired,
		eventspkg.TaskRunReleased,
		eventspkg.TaskRunOperatorForcedFail,
		eventspkg.TaskRunOperatorRetry,
		eventspkg.TaskRunRecoveredFromAttention,
		eventspkg.TaskRunNeedsAttention,
		eventspkg.TaskRunReviewRetryEnqueued:
		return strings.TrimSpace(eventType)
	case taskHookRunStarted:
		return taskEventRunStarted
	case taskHookRunCompleted:
		return taskEventRunCompleted
	case taskHookRunFailed:
		return taskEventRunFailed
	case taskHookRunCanceled:
		return taskEventRunCanceled
	default:
		return ""
	}
}
