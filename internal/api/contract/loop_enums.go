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
	LoopRunEventPredicateDiagnostic  LoopRunEventKind = "predicate_diagnostic"
	LoopRunEventNodeRetryScheduled   LoopRunEventKind = "node_retry_scheduled"
	LoopRunEventNodePaused           LoopRunEventKind = "node_paused"
	LoopRunEventNodeResumed          LoopRunEventKind = "node_resumed"
	LoopRunEventNodeCanceled         LoopRunEventKind = "node_canceled"
	LoopRunEventNodeKilled           LoopRunEventKind = "node_killed"
	LoopRunEventNodeQuarantined      LoopRunEventKind = "node_quarantined"
	LoopRunEventNodeRequeued         LoopRunEventKind = "node_requeued"
	LoopRunEventNodeWaitStarted      LoopRunEventKind = "node_wait_started"
	LoopRunEventNodeWaitResumed      LoopRunEventKind = "node_wait_resumed"
	LoopRunEventNodeAttentionFlagged LoopRunEventKind = "node_attention_flagged"
	LoopRunEventNodeAttentionCleared LoopRunEventKind = "node_attention_cleared"
	LoopRunEventEffectResults        LoopRunEventKind = "effect_results"
	LoopRunEventCustomEvent          LoopRunEventKind = "custom_event"
	LoopRunEventDuplicateSuppressed  LoopRunEventKind = "duplicate_suppressed"
	LoopRunEventTargetBreaker        LoopRunEventKind = "target_breaker_transition"
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
		string(LoopRunStatusCanceled),
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
		string(LoopRunStatusCanceled),
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
		string(LoopRunEventPredicateDiagnostic),
		string(LoopRunEventNodeRetryScheduled),
		string(LoopRunEventNodePaused),
		string(LoopRunEventNodeResumed),
		string(LoopRunEventNodeCanceled),
		string(LoopRunEventNodeKilled),
		string(LoopRunEventNodeQuarantined),
		string(LoopRunEventNodeRequeued),
		string(LoopRunEventNodeWaitStarted),
		string(LoopRunEventNodeWaitResumed),
		string(LoopRunEventNodeAttentionFlagged),
		string(LoopRunEventNodeAttentionCleared),
		string(LoopRunEventEffectResults),
		string(LoopRunEventCustomEvent),
		string(LoopRunEventDuplicateSuppressed),
		string(LoopRunEventTargetBreaker),
		string(LoopRunEventStaleScheduleDropped),
		string(LoopRunEventLateArrival),
	}
}

// LoopRunTransitionCauseValues returns the closed public transition-cause vocabulary.
func LoopRunTransitionCauseValues() []string {
	return []string{
		string(LoopRunTransitionCauseStart),
		string(LoopRunTransitionCausePromote),
		string(LoopRunTransitionCauseOperatorCancel),
		string(LoopRunTransitionCauseOperatorKill),
		string(LoopRunTransitionCauseGoalReplace),
		string(LoopRunTransitionCauseGoalClear),
		string(LoopRunTransitionCausePauseBoundary),
		string(LoopRunTransitionCauseOperatorResume),
		string(LoopRunTransitionCauseApproval),
		string(LoopRunTransitionCauseWaitExpired),
		string(LoopRunTransitionCauseGateRejected),
		string(LoopRunTransitionCauseContract),
		string(LoopRunTransitionCauseBudget),
		string(LoopRunTransitionCauseIterationCap),
		string(LoopRunTransitionCauseNoProgress),
		string(LoopRunTransitionCauseWatchPoll),
		string(LoopRunTransitionCauseWatchEvents),
		string(LoopRunTransitionCauseCoordinatorFailure),
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
		string(LoopGenerationOriginRequeue),
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
		string(LoopRunEventNodePaused),
		string(LoopRunEventNodeResumed),
		string(LoopRunEventNodeCanceled),
		string(LoopRunEventNodeKilled),
		string(LoopRunEventNodeQuarantined),
		string(LoopRunEventNodeRequeued),
		string(LoopRunEventNodeWaitStarted),
		string(LoopRunEventNodeWaitResumed),
		string(LoopRunEventNodeAttentionFlagged),
		string(LoopRunEventNodeAttentionCleared),
		string(LoopRunEventEffectResults),
		string(LoopRunEventCustomEvent),
		string(LoopRunEventDuplicateSuppressed),
		string(LoopRunEventTargetBreaker),
	}
}
