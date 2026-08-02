package contract

// LoopRunEventKind is the public loop run event stream vocabulary.
type LoopRunEventKind string

const (
	LoopRunEventNodeRunning          LoopRunEventKind = "node_running"
	LoopRunEventNodeSucceeded        LoopRunEventKind = "node_succeeded"
	LoopRunEventNodeFailed           LoopRunEventKind = "node_failed"
	LoopRunEventGateVerdict          LoopRunEventKind = "gate_verdict"
	LoopRunEventGenerationStarted    LoopRunEventKind = "generation_started"
	LoopRunEventChannelMsg           LoopRunEventKind = "channel_msg"
	LoopRunEventTokenTick            LoopRunEventKind = "token_tick"
	LoopRunEventNeedsApproval        LoopRunEventKind = "needs_approval"
	LoopRunEventStatusChanged        LoopRunEventKind = "status_changed"
	LoopRunEventGoalTurnStarted      LoopRunEventKind = "goal_turn_started"
	LoopRunEventGoalTurnCompleted    LoopRunEventKind = "goal_turn_completed"
	LoopRunEventGoalStatusChanged    LoopRunEventKind = "goal_status_changed"
	LoopRunEventRuntimeApplied       LoopRunEventKind = "runtime_applied"
	LoopRunEventNodeRetryScheduled   LoopRunEventKind = "node_retry_scheduled"
	LoopRunEventStaleScheduleDropped LoopRunEventKind = "stale_schedule_dropped"
	LoopRunEventLateArrival          LoopRunEventKind = "late_arrival"
)

// LoopRunStatusValues returns the closed public loop run status vocabulary.
func LoopRunStatusValues() []string {
	return []string{
		string(LoopRunStatusQueued),
		string(LoopRunStatusRunning),
		string(LoopRunStatusWatching),
		string(LoopRunStatusNeedsApproval),
		string(LoopRunStatusPaused),
		string(LoopRunStatusDone),
		string(LoopRunStatusNoOp),
		string(LoopRunStatusBlocked),
		string(LoopRunStatusFailed),
		string(LoopRunStatusExhausted),
		string(LoopRunStatusStalled),
	}
}

// LoopRunLiveStatusValues returns the non-terminal loop run statuses.
func LoopRunLiveStatusValues() []string {
	return []string{
		string(LoopRunStatusQueued),
		string(LoopRunStatusRunning),
		string(LoopRunStatusWatching),
		string(LoopRunStatusNeedsApproval),
		string(LoopRunStatusPaused),
	}
}

// LoopRunTerminalStatusValues returns the terminal loop run statuses.
func LoopRunTerminalStatusValues() []string {
	return []string{
		string(LoopRunStatusDone),
		string(LoopRunStatusNoOp),
		string(LoopRunStatusBlocked),
		string(LoopRunStatusFailed),
		string(LoopRunStatusExhausted),
		string(LoopRunStatusStalled),
	}
}

// LoopRunEventKindValues returns the closed public loop run event vocabulary.
func LoopRunEventKindValues() []string {
	return []string{
		string(LoopRunEventNodeRunning),
		string(LoopRunEventNodeSucceeded),
		string(LoopRunEventNodeFailed),
		string(LoopRunEventGateVerdict),
		string(LoopRunEventGenerationStarted),
		string(LoopRunEventChannelMsg),
		string(LoopRunEventTokenTick),
		string(LoopRunEventNeedsApproval),
		string(LoopRunEventStatusChanged),
		string(LoopRunEventGoalTurnStarted),
		string(LoopRunEventGoalTurnCompleted),
		string(LoopRunEventGoalStatusChanged),
		string(LoopRunEventRuntimeApplied),
		string(LoopRunEventNodeRetryScheduled),
		string(LoopRunEventStaleScheduleDropped),
		string(LoopRunEventLateArrival),
	}
}

// LoopGenerationOriginValues returns the closed generation provenance vocabulary.
func LoopGenerationOriginValues() []string {
	return []string{
		string(LoopGenerationOriginInitial),
		string(LoopGenerationOriginStopWhen),
		string(LoopGenerationOriginReattempt),
		string(LoopGenerationOriginGateRevise),
		string(LoopGenerationOriginGateNextGeneration),
		string(LoopGenerationOriginDoDRetry),
		string(LoopGenerationOriginRatchetRestore),
	}
}

// LoopGateVerdictOutcomeValues returns the closed machine-verdict vocabulary.
func LoopGateVerdictOutcomeValues() []string {
	return []string{
		string(LoopGateVerdictApproved),
		string(LoopGateVerdictRejected),
		string(LoopGateVerdictAwaitingApproval),
		string(LoopGateVerdictBlocked),
		string(LoopGateVerdictError),
		string(LoopGateVerdictTimeout),
		string(LoopGateVerdictInvalidOutput),
	}
}

// LoopMetricDirectionValues returns the closed gate metric direction vocabulary.
func LoopMetricDirectionValues() []string {
	return []string{
		string(LoopMetricMaximize),
		string(LoopMetricMinimize),
	}
}

// LoopRunLifecycleEventKindValues returns event kinds that mutate durable run state.
func LoopRunLifecycleEventKindValues() []string {
	return []string{
		string(LoopRunEventStatusChanged),
		string(LoopRunEventNodeRunning),
		string(LoopRunEventNodeSucceeded),
		string(LoopRunEventNodeFailed),
		string(LoopRunEventGateVerdict),
		string(LoopRunEventGenerationStarted),
		string(LoopRunEventNeedsApproval),
		string(LoopRunEventGoalStatusChanged),
		string(LoopRunEventRuntimeApplied),
		string(LoopRunEventNodeRetryScheduled),
	}
}
