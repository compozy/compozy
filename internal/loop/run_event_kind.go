package loop

type RunEventKind string

const runEventNodeKilled RunEventKind = "node_killed"

const (
	RunEventNodeRunning          RunEventKind = "node_running"
	RunEventNodeSucceeded        RunEventKind = "node_succeeded"
	RunEventNodeFailed           RunEventKind = "node_failed"
	RunEventGateVerdict          RunEventKind = "gate_verdict"
	RunEventGenerationStarted    RunEventKind = "generation_started"
	RunEventChannelMsg           RunEventKind = "channel_msg"
	RunEventTokenTick            RunEventKind = "token_tick"
	RunEventNeedsApproval        RunEventKind = "needs_approval"
	RunEventStatusChanged        RunEventKind = "status_changed"
	RunEventGoalTurnStarted      RunEventKind = "goal_turn_started"
	RunEventGoalTurnCompleted    RunEventKind = "goal_turn_completed"
	RunEventGoalStatusChanged    RunEventKind = "goal_status_changed"
	RunEventRuntimeApplied       RunEventKind = "runtime_applied"
	RunEventPredicateDiagnostic  RunEventKind = "predicate_diagnostic"
	RunEventRouteTaken           RunEventKind = "route_taken"
	RunEventNodeRetryScheduled   RunEventKind = "node_retry_scheduled"
	RunEventNodePaused           RunEventKind = "node_paused"
	RunEventNodeResumed          RunEventKind = "node_resumed"
	RunEventNodeCanceled         RunEventKind = "node_canceled"
	RunEventNodeQuarantined      RunEventKind = "node_quarantined"
	RunEventNodeRequeued         RunEventKind = "node_requeued"
	RunEventNodeWaitStarted      RunEventKind = "node_wait_started"
	RunEventNodeWaitResumed      RunEventKind = "node_wait_resumed"
	RunEventNodeAttentionFlagged RunEventKind = "node_attention_flagged"
	RunEventNodeAttentionCleared RunEventKind = "node_attention_cleared"
	RunEventEffectResults        RunEventKind = "effect_results"
	RunEventCustomEvent          RunEventKind = "custom_event"
	RunEventDuplicateSuppressed  RunEventKind = "duplicate_suppressed"
	RunEventTargetBreaker        RunEventKind = "target_breaker_transition"
	RunEventStaleScheduleDropped RunEventKind = "stale_schedule_dropped"
	RunEventLateArrival          RunEventKind = "late_arrival"
	RunEventRequestOpened        RunEventKind = "request_opened"
	RunEventRequestAnswered      RunEventKind = "request_answered"
	RunEventRequestExpired       RunEventKind = "request_expired"
	RunEventRequestCanceled      RunEventKind = "request_canceled"
	RunEventNodeAmended          RunEventKind = "node_amended"
	RunEventBranchPruned         RunEventKind = "branch_pruned"
	RunEventRunForked            RunEventKind = "run_forked"
)

func RunEventKindValues() []string {
	return []string{
		string(RunEventNodeRunning), string(RunEventNodeSucceeded), string(RunEventNodeFailed),
		string(RunEventGateVerdict), string(RunEventGenerationStarted), string(RunEventChannelMsg),
		string(RunEventTokenTick), string(RunEventNeedsApproval), string(RunEventStatusChanged),
		string(RunEventGoalTurnStarted), string(RunEventGoalTurnCompleted), string(RunEventGoalStatusChanged),
		string(RunEventRuntimeApplied), string(RunEventPredicateDiagnostic), string(RunEventRouteTaken),
		string(RunEventNodeRetryScheduled), string(RunEventNodePaused), string(RunEventNodeResumed),
		string(RunEventNodeCanceled), string(RunEventNodeQuarantined),
		string(RunEventNodeRequeued), string(RunEventNodeWaitStarted), string(RunEventNodeWaitResumed),
		string(RunEventNodeAttentionFlagged), string(RunEventNodeAttentionCleared), string(RunEventEffectResults),
		string(RunEventCustomEvent), string(RunEventDuplicateSuppressed), string(RunEventTargetBreaker),
		string(RunEventStaleScheduleDropped), string(RunEventLateArrival), string(RunEventRequestOpened),
		string(RunEventRequestAnswered), string(RunEventRequestExpired), string(RunEventRequestCanceled),
		string(RunEventNodeAmended), string(RunEventBranchPruned), string(RunEventRunForked),
	}
}

func projectedRunEventKind(value string) RunEventKind {
	kind := RunEventKind(value)
	if kind == runEventNodeKilled {
		return RunEventNodeCanceled
	}
	return kind
}
