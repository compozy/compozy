package contract

import "time"

// GoalReasonCode is the stable public Goal error and transition vocabulary.
type GoalReasonCode string

// GoalCommandOutcome is the structured Goal command result vocabulary.
type GoalCommandOutcome string

// GoalStatus is the closed public Goal lifecycle vocabulary.
type GoalStatus string

// GoalTurnResultStatus is the closed terminal classification for one Goal turn.
type GoalTurnResultStatus string

// GoalVerdictOutcome is the closed Goal judge outcome vocabulary.
type GoalVerdictOutcome string

const (
	GoalOutcomeStarted  GoalCommandOutcome = "started"
	GoalOutcomeReplaced GoalCommandOutcome = "replaced"
	GoalOutcomeStatus   GoalCommandOutcome = "status"
	GoalOutcomePaused   GoalCommandOutcome = "paused"
	GoalOutcomeResumed  GoalCommandOutcome = "resumed"
	GoalOutcomeCleared  GoalCommandOutcome = "cleared"
	GoalOutcomeError    GoalCommandOutcome = "error"
)

const (
	GoalStatusActive        GoalStatus = "active"
	GoalStatusPaused        GoalStatus = "paused"
	GoalStatusBlocked       GoalStatus = "blocked"
	GoalStatusUsageLimited  GoalStatus = "usage-limited"
	GoalStatusBudgetLimited GoalStatus = "budget-limited"
	GoalStatusComplete      GoalStatus = "complete"
)

const (
	GoalTurnResultCompleted     GoalTurnResultStatus = "completed"
	GoalTurnResultInvalidResult GoalTurnResultStatus = "invalid-result"
	GoalTurnResultFailed        GoalTurnResultStatus = "failed"
	GoalTurnResultAmbiguous     GoalTurnResultStatus = "ambiguous"
)

const (
	GoalVerdictApproved         GoalVerdictOutcome = "approved"
	GoalVerdictRejected         GoalVerdictOutcome = "rejected"
	GoalVerdictAwaitingApproval GoalVerdictOutcome = "awaiting_approval"
	GoalVerdictBlocked          GoalVerdictOutcome = "blocked"
	GoalVerdictError            GoalVerdictOutcome = "error"
	GoalVerdictTimeout          GoalVerdictOutcome = "timeout"
	GoalVerdictInvalidOutput    GoalVerdictOutcome = "invalid_output"
)

const (
	GoalReasonNotActive                  GoalReasonCode = "goal_not_active"
	GoalReasonReplaceRequired            GoalReasonCode = "goal_replace_required"
	GoalReasonReplaceStale               GoalReasonCode = "goal_replace_stale"
	GoalReasonObjectiveRequired          GoalReasonCode = "goal_objective_required"
	GoalReasonObjectiveTooLarge          GoalReasonCode = "goal_objective_too_large"
	GoalReasonContractClauseEmpty        GoalReasonCode = "goal_contract_clause_empty"
	GoalReasonCommandInvalid             GoalReasonCode = "goal_command_invalid"
	GoalReasonDraftRequiresIdle          GoalReasonCode = "goal_draft_requires_idle"
	GoalReasonEvidenceTooLarge           GoalReasonCode = "goal_evidence_too_large"
	GoalReasonReportConflict             GoalReasonCode = "goal_report_conflict"
	GoalReasonSessionBusyQueueFull       GoalReasonCode = "goal_session_busy_queue_full"
	GoalReasonTurnFilterInvalid          GoalReasonCode = "goal_turn_filter_invalid"
	GoalReasonJudgeUnavailable           GoalReasonCode = "goal_judge_unavailable"
	GoalReasonJudgeBroken                GoalReasonCode = "goal_judge_broken"
	GoalReasonJudgeOutcomeInvalid        GoalReasonCode = "goal_judge_outcome_invalid"
	GoalReasonActionControlInvalid       GoalReasonCode = "goal_action_control_invalid"
	GoalReasonStopReasonInvalid          GoalReasonCode = "goal_stop_reason_invalid"
	GoalReasonPromptRequestFailed        GoalReasonCode = "goal_prompt_request_failed"
	GoalReasonAgentRefused               GoalReasonCode = "goal_agent_refused"
	GoalReasonCompactionCancelled        GoalReasonCode = "goal_compaction_cancel" + "led"
	GoalReasonRecoveryAmbiguous          GoalReasonCode = "goal_recovery_ambiguous"
	GoalReasonControlRevokedInFlight     GoalReasonCode = "goal_control_revoked_in_flight"
	GoalReasonReseedConfirmationRequired GoalReasonCode = "goal_reseed_confirmation_required"
	GoalReasonPresubmitRetriesExhausted  GoalReasonCode = "goal_presubmit_retries_exhausted"
	GoalReasonBudgetFenced               GoalReasonCode = "goal_budget_fenced"
	GoalReasonOriginInvalid              GoalReasonCode = "goal_origin_invalid"
	GoalReasonOriginWorkspaceMismatch    GoalReasonCode = "goal_origin_workspace_mismatch"
	GoalReasonOriginProfileUnavailable   GoalReasonCode = "goal_origin_profile_unavailable"
	GoalReasonControlStale               GoalReasonCode = "goal_control_stale"
	GoalReasonPromptFenced               GoalReasonCode = "goal_prompt_fenced"
	GoalReasonSessionCreationMismatch    GoalReasonCode = "session_creation_identity_mismatch"
	GoalReasonContinuousBindingMismatch  GoalReasonCode = "continuous_binding_mismatch"
)

// GoalContextState is the closed public context freshness vocabulary.
type GoalContextState string

// GoalPromptKind is the public transcript classification for engine-owned Goal prompts.
type GoalPromptKind string

const (
	GoalPromptWork         GoalPromptKind = "goal-work"
	GoalPromptContinuation GoalPromptKind = "goal-continuation"
	GoalPromptCompaction   GoalPromptKind = "goal-compaction"
)

// GoalSnapshotChangeCause classifies session Goal discovery signals.
type GoalSnapshotChangeCause string

const (
	GoalSnapshotCauseStart   GoalSnapshotChangeCause = "start"
	GoalSnapshotCauseReplace GoalSnapshotChangeCause = "replace"
	GoalSnapshotCauseStatus  GoalSnapshotChangeCause = "status"
	GoalSnapshotCauseClear   GoalSnapshotChangeCause = "clear"
	GoalSnapshotCauseReseed  GoalSnapshotChangeCause = "reseed"
)

const (
	GoalContextKnown   GoalContextState = "known"
	GoalContextUnknown GoalContextState = "unknown"
	GoalContextPending GoalContextState = "pending"
)

// GoalBlockingIssue is one public verdict blocker.
type GoalBlockingIssue struct {
	ID   string `json:"id"`
	Note string `json:"note"`
}

// GoalContextSnapshot is checkpoint-pinned context telemetry.
type GoalContextSnapshot struct {
	State      GoalContextState `json:"state"`
	Used       *int64           `json:"used"`
	Size       *int64           `json:"size"`
	Ratio      *float64         `json:"ratio"`
	NudgeRatio float64          `json:"nudge_ratio"`
	ReportedAt *time.Time       `json:"reported_at"`
}

// GoalVerdictSummary is the latest evaluated Goal verdict.
type GoalVerdictSummary struct {
	Outcome        GoalVerdictOutcome  `json:"outcome"`
	BlockingIssues []GoalBlockingIssue `json:"blocking_issues"`
	EvidenceRef    *string             `json:"evidence_ref"`
	EvaluatedAt    time.Time           `json:"evaluated_at"`
}

// GoalSnapshot is the canonical newest session-origin Goal projection.
type GoalSnapshot struct {
	RunID           string              `json:"run_id"`
	NodeID          string              `json:"node_id"`
	Objective       string              `json:"objective"`
	OriginSessionID string              `json:"origin_session_id"`
	BoundSessionID  string              `json:"bound_session_id"`
	Status          GoalStatus          `json:"status"`
	RunStatus       LoopRunStatus       `json:"run_status"`
	Cause           *GoalReasonCode     `json:"cause"`
	TurnsUsed       int                 `json:"turns_used"`
	TurnLimit       int                 `json:"turn_limit"`
	Live            bool                `json:"live"`
	ContractSummary string              `json:"contract_summary"`
	LastVerdict     *GoalVerdictSummary `json:"last_verdict"`
	Context         GoalContextSnapshot `json:"context"`
}

// SessionGoalResponse preserves explicit null when no visible Goal exists.
type SessionGoalResponse struct {
	Goal *GoalSnapshot `json:"goal"`
}

// GoalCommandResult is the shared structured success/error envelope for Goal operations.
type GoalCommandResult struct {
	Outcome       GoalCommandOutcome `json:"outcome"`
	ReasonCode    *GoalReasonCode    `json:"reason_code"`
	Snapshot      *GoalSnapshot      `json:"snapshot"`
	ReplacedRunID *string            `json:"replaced_run_id"`
}

// GoalTurn is one immutable-start, optionally terminal Goal audit row.
type GoalTurn struct {
	Seq            int64                 `json:"seq"`
	Generation     int64                 `json:"generation"`
	NodeID         string                `json:"node_id"`
	ItemIndex      int                   `json:"item_index"`
	Turn           int                   `json:"turn"`
	PromptAttempt  int                   `json:"prompt_attempt"`
	SessionID      string                `json:"session_id"`
	BindingHandle  string                `json:"binding_handle"`
	BindingEpoch   int64                 `json:"binding_epoch"`
	PromptID       string                `json:"prompt_id"`
	ResultStatus   *GoalTurnResultStatus `json:"result_status"`
	ReasonCode     *GoalReasonCode       `json:"reason_code"`
	StopReason     *ACPPromptStopReason  `json:"stop_reason"`
	VerdictOutcome *GoalVerdictOutcome   `json:"verdict_outcome"`
	BlockingIssues []GoalBlockingIssue   `json:"blocking_issues"`
	EvidenceRef    *string               `json:"evidence_ref"`
	PromptRef      *string               `json:"prompt_ref"`
	TokensUsed     *int64                `json:"tokens_used"`
	ActorKind      string                `json:"actor_kind"`
	ActorID        string                `json:"actor_id"`
	StartedAt      time.Time             `json:"started_at"`
	EndedAt        *time.Time            `json:"ended_at"`
}

// GoalTurnPage is one run-wide monotonic cursor page.
type GoalTurnPage struct {
	Turns        []GoalTurn `json:"turns"`
	NextAfterSeq *int64     `json:"next_after_seq"`
}

// GoalPromptMeta classifies one engine-owned Goal prompt in persisted transcript events.
type GoalPromptMeta struct {
	Kind          GoalPromptKind `json:"kind"`
	RunID         string         `json:"run_id"`
	NodeID        string         `json:"node_id"`
	Generation    int64          `json:"generation"`
	ItemIndex     int            `json:"item_index"`
	Turn          *int           `json:"turn"`
	PromptAttempt int            `json:"prompt_attempt"`
	PromptID      string         `json:"prompt_id"`
}

// GoalSnapshotChangedPayload is the persisted session stream discovery signal.
type GoalSnapshotChangedPayload struct {
	SessionID      string                  `json:"session_id"`
	RunID          *string                 `json:"run_id"`
	BoundSessionID *string                 `json:"bound_session_id"`
	Revision       int64                   `json:"revision"`
	Cause          GoalSnapshotChangeCause `json:"cause"`
}
