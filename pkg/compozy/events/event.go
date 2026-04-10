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
	EventKindRunCompleted EventKind = "run.completed"
	EventKindRunFailed    EventKind = "run.failed"
	EventKindRunCancelled EventKind = "run.cancelled"

	// Job lifecycle events.
	EventKindJobQueued          EventKind = "job.queued"
	EventKindJobStarted         EventKind = "job.started"
	EventKindJobAttemptStarted  EventKind = "job.attempt_started"
	EventKindJobAttemptFinished EventKind = "job.attempt_finished"
	EventKindJobRetryScheduled  EventKind = "job.retry_scheduled"
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
	EventKindTaskFileUpdated       EventKind = "task.file_updated"
	EventKindTaskMetadataRefreshed EventKind = "task.metadata_refreshed"

	// Review mutation events.
	EventKindReviewStatusFinalized EventKind = "review.status_finalized"
	EventKindReviewRoundRefreshed  EventKind = "review.round_refreshed"
	EventKindReviewIssueResolved   EventKind = "review.issue_resolved"

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
