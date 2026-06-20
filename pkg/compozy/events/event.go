package events

import (
	"encoding/json"
	"time"
)

// SchemaVersion identifies the current event schema emitted by Compozy.
const SchemaVersion = "1.0"

// Event carries one versioned event envelope.
type Event struct {
	SchemaVersion string          `json:"schema_version"`
	RunID         string          `json:"run_id"`
	Seq           uint64          `json:"seq"`
	Timestamp     time.Time       `json:"ts"`
	Kind          EventKind       `json:"kind"`
	Payload       json.RawMessage `json:"payload"`
}

// EventKind identifies one emitted event kind.
type EventKind string

const (
	// Run lifecycle events.
	EventKindRunQueued    EventKind = "run.queued"
	EventKindRunStarted   EventKind = "run.started"
	EventKindRunCrashed   EventKind = "run.crashed"
	EventKindRunCompleted EventKind = "run.completed"
	EventKindRunFailed    EventKind = "run.failed"
	EventKindRunCancelled EventKind = "run.cancelled"

	EventKindRunRecoveryStarted    EventKind = "run.recovery_started"
	EventKindRunRecoveryRestarting EventKind = "run.recovery_restarting"
	EventKindRunRecovered          EventKind = "run.recovered"
	EventKindRunRecoveryExhausted  EventKind = "run.recovery_exhausted"

	// Job lifecycle events.
	EventKindJobQueued          EventKind = "job.queued"
	EventKindJobStarted         EventKind = "job.started"
	EventKindJobAttemptStarted  EventKind = "job.attempt_started"
	EventKindJobAttemptFinished EventKind = "job.attempt_finished"
	EventKindJobRetryScheduled  EventKind = "job.retry_scheduled"
	EventKindJobPausing         EventKind = "job.pausing"
	EventKindJobPaused          EventKind = "job.paused"
	EventKindJobResumed         EventKind = "job.resumed"
	EventKindJobCompleted       EventKind = "job.completed"
	EventKindJobFailed          EventKind = "job.failed"
	EventKindJobCancelled       EventKind = "job.cancelled"

	// Session events.
	EventKindSessionStarted   EventKind = "session.started"
	EventKindSessionUpdate    EventKind = "session.update"
	EventKindSessionCompleted EventKind = "session.completed"
	EventKindSessionFailed    EventKind = "session.failed"

	// Reusable-agent lifecycle events.
	EventKindReusableAgentLifecycle EventKind = "reusable_agent.lifecycle"

	// Tool call events.
	EventKindToolCallStarted EventKind = "tool_call.started"
	EventKindToolCallUpdated EventKind = "tool_call.updated"
	EventKindToolCallFailed  EventKind = "tool_call.failed"

	// Usage events.
	EventKindUsageUpdated    EventKind = "usage.updated"
	EventKindUsageAggregated EventKind = "usage.aggregated"

	// Task mutation events.
	EventKindTaskFileUpdated               EventKind = "task.file_updated"
	EventKindTaskFileSkipped               EventKind = "task.file_skipped"
	EventKindTaskMetadataRefreshed         EventKind = "task.metadata_refreshed"
	EventKindTaskMemoryUpdated             EventKind = "task.memory_updated"
	EventKindTaskRunMultipleStarted        EventKind = "task.multi.started"
	EventKindTaskRunMultipleItemQueued     EventKind = "task.multi.item_queued"
	EventKindTaskRunMultipleChildStarted   EventKind = "task.multi.child_started"
	EventKindTaskRunMultipleChildCompleted EventKind = "task.multi.child_completed"
	EventKindTaskRunMultipleChildFailed    EventKind = "task.multi.child_failed"
	EventKindTaskRunMultipleItemCanceled   EventKind = "task.multi.item_canceled"
	EventKindTaskRunMultipleQueueCanceled  EventKind = "task.multi.queue_canceled"
	EventKindTaskRunMultipleQueueCompleted EventKind = "task.multi.queue_completed"

	// Parallel task execution events emitted by the ParallelExecutionOrchestrator.
	EventKindTaskParallelPlanStarted       EventKind = "task.parallel.plan_started"
	EventKindTaskParallelWaveStarted       EventKind = "task.parallel.wave_started"
	EventKindTaskParallelWaveCompleted     EventKind = "task.parallel.wave_completed"
	EventKindTaskParallelMergeStarted      EventKind = "task.parallel.merge_started"
	EventKindTaskParallelConflictDetected  EventKind = "task.parallel.conflict_detected"
	EventKindTaskParallelConflictResolving EventKind = "task.parallel.conflict_resolving"
	EventKindTaskParallelMerged            EventKind = "task.parallel.merged"
	EventKindTaskParallelRolledBack        EventKind = "task.parallel.rolled_back"

	// Artifact and extension events.
	EventKindArtifactUpdated EventKind = "artifact.updated"
	EventKindExtensionLoaded EventKind = "extension.loaded"
	EventKindExtensionReady  EventKind = "extension.ready"
	EventKindExtensionFailed EventKind = "extension.failed"
	EventKindExtensionEvent  EventKind = "extension.event"

	// Review mutation events.
	EventKindReviewStatusFinalized    EventKind = "review.status_finalized"
	EventKindReviewRoundRefreshed     EventKind = "review.round_refreshed"
	EventKindReviewIssueResolved      EventKind = "review.issue_resolved"
	EventKindReviewWatchStarted       EventKind = "review.watch_started"
	EventKindReviewWatchWaiting       EventKind = "review.watch_waiting"
	EventKindReviewWatchRoundFetched  EventKind = "review.watch_round_fetched"
	EventKindReviewWatchFixStarted    EventKind = "review.watch_fix_started"
	EventKindReviewWatchFixCompleted  EventKind = "review.watch_fix_completed"
	EventKindReviewWatchPushStarted   EventKind = "review.watch_push_started"
	EventKindReviewWatchPushCompleted EventKind = "review.watch_push_completed"
	EventKindReviewWatchPushFailed    EventKind = "review.watch_push_failed"
	EventKindReviewWatchClean         EventKind = "review.watch_clean"
	EventKindReviewWatchMaxRounds     EventKind = "review.watch_max_rounds"

	// Provider I/O events.
	EventKindProviderCallStarted   EventKind = "provider.call_started"
	EventKindProviderCallCompleted EventKind = "provider.call_completed"
	EventKindProviderCallFailed    EventKind = "provider.call_failed"

	// Shutdown events.
	EventKindShutdownRequested  EventKind = "shutdown.requested"
	EventKindShutdownDraining   EventKind = "shutdown.draining"
	EventKindShutdownTerminated EventKind = "shutdown.terminated"
)

// SubID identifies one bus subscription.
type SubID uint64
