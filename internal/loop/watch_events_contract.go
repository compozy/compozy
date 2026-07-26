package loop

import "github.com/compozy/agh/internal/hooks"

const (
	watchEventsTaskStream       = "task_events"
	watchEventsLoopStream       = "loop_run_events"
	watchEventsAutomationStream = "automation_runs"
	watchEventsNetworkStream    = "network_timeline_log"
	watchEventsObserveStream    = "event_summaries"
	watchEventsSessionStream    = "session_events"
	watchEventsSessionSeparator = ":"

	loopRunEventLedgerStatusChanged = "status_changed"
	loopRunEventLedgerNodeSucceeded = "node_succeeded"
	loopRunEventLedgerNodeFailed    = "node_failed"

	watchEventsPayloadParentTaskID = "parent_task_id"
	watchEventsPayloadDetails      = "details"
	watchEventsPayloadError        = "error"
	watchEventsPayloadReason       = "reason"

	watchEventsPayloadAgentName   = "agent_name"
	watchEventsPayloadAttempt     = "attempt"
	watchEventsPayloadCausationID = "causation_id"
	watchEventsPayloadDirectID    = "direct_id"
	watchEventsPayloadDirection   = "direction"
	watchEventsPayloadDurationMS  = "duration_ms"
	watchEventsPayloadJobID       = "job_id"
	watchEventsPayloadMessageID   = "message_id"
	watchEventsPayloadPeerFrom    = "peer_from"
	watchEventsPayloadPeerTo      = "peer_to"
	watchEventsPayloadSurface     = "surface"
	watchEventsPayloadThreadID    = "thread_id"
	watchEventsPayloadTraceID     = "trace_id"
	watchEventsPayloadTriggerID   = "trigger_id"
	watchEventsPayloadWillRetry   = "will_retry"
	watchEventsPayloadWorkState   = "work_state"

	watchEventsPayloadCoordinatorSessionID         = "coordinator_session_id"
	watchEventsPayloadResolvedNetworkParticipation = "resolved_network_participation"
	watchEventsPayloadDecisionKind                 = "decision_kind"
	watchEventsPayloadDecision                     = "decision"
	watchEventsPayloadModel                        = "model"
	watchEventsPayloadProvider                     = "provider"
	watchEventsPayloadRecordType                   = "record_type"
	watchEventsPayloadSequence                     = "sequence"
	watchEventsPayloadStopReason                   = "stop_reason"
	watchEventsPayloadTurnID                       = "turn_id"
	watchEventsPayloadWorkflowID                   = "workflow_id"
)

const (
	// WatchEventsTaskStream is the task_events replay stream name.
	WatchEventsTaskStream = watchEventsTaskStream
	// WatchEventsLoopStream is the loop_run_events replay stream name.
	WatchEventsLoopStream = watchEventsLoopStream
	// WatchEventsAutomationStream is the automation_runs replay stream name.
	WatchEventsAutomationStream = watchEventsAutomationStream
	// WatchEventsNetworkStream is the network_timeline_log replay stream name.
	WatchEventsNetworkStream = watchEventsNetworkStream
	// WatchEventsObserveStream is the event_summaries replay stream name.
	WatchEventsObserveStream = watchEventsObserveStream
	// WatchEventsSessionStream is the per-session event replay stream base name.
	WatchEventsSessionStream = watchEventsSessionStream
)

// WatchEventsContract describes one supported watch-events family row.
type WatchEventsContract struct {
	Kind          hooks.HookEvent `json:"kind"`
	Stream        string          `json:"stream"`
	LedgerTypes   []string        `json:"ledger_types"`
	PayloadFields []string        `json:"payload_fields"`
	RequiredVars  []string        `json:"required_vars"`
}
