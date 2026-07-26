package globaldb

const (
	goalCheckpointPhaseIdle            = "idle"
	goalCheckpointPhasePreparing       = "preparing"
	goalCheckpointPhaseQueued          = "queued"
	goalCheckpointPhasePrompting       = "prompting"
	goalCheckpointPhaseCompacting      = "compacting"
	goalCheckpointPhaseJudging         = "judging"
	goalCheckpointPhasePersisting      = "persisting"
	goalCheckpointPhaseAwaitingControl = "awaiting_control"
	goalCheckpointPhaseTerminal        = "terminal"

	goalStatusActive        = globalDBSessionStateActive
	goalStatusPaused        = "paused"
	goalStatusBlocked       = "blocked"
	goalStatusUsageLimited  = "usage-limited"
	goalStatusBudgetLimited = "budget-limited"
	goalStatusComplete      = "complete"

	goalContextStateKnown   = "known"
	goalContextStateUnknown = "unknown"
	goalContextStatePending = "pending"

	goalPromptKindWork         = "work"
	goalPromptKindContinuation = "continuation"
	goalPromptKindCompact      = "compact"

	goalJudgeStatusRunning   = "running"
	goalJudgeStatusCompleted = "completed"
	goalJudgeStatusAmbiguous = "ambiguous"

	goalReportStatusComplete = "complete"
	goalReportStatusBlocked  = "blocked"

	goalGenerationOutputEnqueued = "enqueued"
	goalRunOriginKindSession     = "session"

	goalBindingCheckpointPredicate = ` AND generation = ? AND node_id = ? AND item_index = ?`
)
