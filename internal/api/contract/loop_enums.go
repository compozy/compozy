package contract

import looppkg "github.com/compozy/compozy/internal/loop"

// LoopRunEventKind is the public loop run event stream vocabulary.
type LoopRunEventKind = looppkg.RunEventKind

const (
	LoopRunEventNodeRunning          = looppkg.RunEventNodeRunning
	LoopRunEventNodeSucceeded        = looppkg.RunEventNodeSucceeded
	LoopRunEventNodeFailed           = looppkg.RunEventNodeFailed
	LoopRunEventGateVerdict          = looppkg.RunEventGateVerdict
	LoopRunEventGenerationStarted    = looppkg.RunEventGenerationStarted
	LoopRunEventChannelMsg           = looppkg.RunEventChannelMsg
	LoopRunEventTokenTick            = looppkg.RunEventTokenTick
	LoopRunEventNeedsApproval        = looppkg.RunEventNeedsApproval
	LoopRunEventStatusChanged        = looppkg.RunEventStatusChanged
	LoopRunEventGoalTurnStarted      = looppkg.RunEventGoalTurnStarted
	LoopRunEventGoalTurnCompleted    = looppkg.RunEventGoalTurnCompleted
	LoopRunEventGoalStatusChanged    = looppkg.RunEventGoalStatusChanged
	LoopRunEventRuntimeApplied       = looppkg.RunEventRuntimeApplied
	LoopRunEventPredicateDiagnostic  = looppkg.RunEventPredicateDiagnostic
	LoopRunEventRouteTaken           = looppkg.RunEventRouteTaken
	LoopRunEventNodeRetryScheduled   = looppkg.RunEventNodeRetryScheduled
	LoopRunEventNodePaused           = looppkg.RunEventNodePaused
	LoopRunEventNodeResumed          = looppkg.RunEventNodeResumed
	LoopRunEventNodeCanceled         = looppkg.RunEventNodeCanceled
	LoopRunEventNodeKilled           = looppkg.RunEventNodeKilled
	LoopRunEventNodeQuarantined      = looppkg.RunEventNodeQuarantined
	LoopRunEventNodeRequeued         = looppkg.RunEventNodeRequeued
	LoopRunEventNodeWaitStarted      = looppkg.RunEventNodeWaitStarted
	LoopRunEventNodeWaitResumed      = looppkg.RunEventNodeWaitResumed
	LoopRunEventNodeAttentionFlagged = looppkg.RunEventNodeAttentionFlagged
	LoopRunEventNodeAttentionCleared = looppkg.RunEventNodeAttentionCleared
	LoopRunEventEffectResults        = looppkg.RunEventEffectResults
	LoopRunEventCustomEvent          = looppkg.RunEventCustomEvent
	LoopRunEventDuplicateSuppressed  = looppkg.RunEventDuplicateSuppressed
	LoopRunEventTargetBreaker        = looppkg.RunEventTargetBreaker
	LoopRunEventStaleScheduleDropped = looppkg.RunEventStaleScheduleDropped
	LoopRunEventLateArrival          = looppkg.RunEventLateArrival
	LoopRunEventRequestOpened        = looppkg.RunEventRequestOpened
	LoopRunEventRequestAnswered      = looppkg.RunEventRequestAnswered
	LoopRunEventRequestExpired       = looppkg.RunEventRequestExpired
	LoopRunEventRequestCanceled      = looppkg.RunEventRequestCanceled
	LoopRunEventNodeAmended          = looppkg.RunEventNodeAmended
	LoopRunEventBranchPruned         = looppkg.RunEventBranchPruned
	LoopRunEventRunForked            = looppkg.RunEventRunForked
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

// LoopCompletionStateValues returns the closed completion coverage vocabulary.
func LoopCompletionStateValues() []string {
	return []string{
		string(LoopCompletionStateComplete),
		string(LoopCompletionStatePartial),
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
	return looppkg.RunEventKindValues()
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
		string(LoopRunTransitionCauseOperatorRerun),
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
		string(LoopGenerationOriginOperatorRerun),
		string(LoopGenerationOriginForkSeed),
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
		string(LoopRunEventBranchPruned),
		string(LoopRunEventRunForked),
	}
}
