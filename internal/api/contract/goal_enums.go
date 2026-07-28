package contract

// GoalCommandOutcomeValues returns the closed structured Goal result vocabulary.
func GoalCommandOutcomeValues() []string {
	return []string{
		string(GoalOutcomeStarted), string(GoalOutcomeReplaced), string(GoalOutcomeStatus),
		string(GoalOutcomePaused), string(GoalOutcomeResumed), string(GoalOutcomeCleared),
		string(GoalOutcomeError),
	}
}

// GoalStatusValues returns the closed public Goal lifecycle vocabulary.
func GoalStatusValues() []string {
	return []string{
		string(GoalStatusActive), string(GoalStatusPaused), string(GoalStatusBlocked),
		string(GoalStatusUsageLimited), string(GoalStatusBudgetLimited), string(GoalStatusComplete),
	}
}

// GoalTurnResultStatusValues returns the closed Goal turn result vocabulary.
func GoalTurnResultStatusValues() []string {
	return []string{
		string(GoalTurnResultCompleted), string(GoalTurnResultInvalidResult),
		string(GoalTurnResultFailed), string(GoalTurnResultAmbiguous),
	}
}

// GoalVerdictOutcomeValues returns the closed Goal judge outcome vocabulary.
func GoalVerdictOutcomeValues() []string {
	return []string{
		string(GoalVerdictApproved), string(GoalVerdictRejected),
		string(GoalVerdictAwaitingApproval), string(GoalVerdictBlocked),
		string(GoalVerdictError), string(GoalVerdictTimeout), string(GoalVerdictInvalidOutput),
	}
}

// GoalContextStateValues returns the closed context freshness vocabulary.
func GoalContextStateValues() []string {
	return []string{string(GoalContextKnown), string(GoalContextUnknown), string(GoalContextPending)}
}

// GoalPromptKindValues returns the closed transcript Goal prompt vocabulary.
func GoalPromptKindValues() []string {
	return []string{
		string(GoalPromptWork), string(GoalPromptContinuation), string(GoalPromptCompaction),
	}
}

// GoalSnapshotChangeCauseValues returns the closed session projection cause vocabulary.
func GoalSnapshotChangeCauseValues() []string {
	return []string{
		string(GoalSnapshotCauseStart), string(GoalSnapshotCauseReplace), string(GoalSnapshotCauseStatus),
		string(GoalSnapshotCauseClear), string(GoalSnapshotCauseReseed),
	}
}

// GoalReasonCodeValues returns the stable public Goal reason vocabulary.
func GoalReasonCodeValues() []string {
	return []string{
		string(GoalReasonNotActive), string(GoalReasonReplaceRequired), string(GoalReasonReplaceStale),
		string(GoalReasonObjectiveRequired), string(GoalReasonObjectiveTooLarge),
		string(GoalReasonContractClauseEmpty), string(GoalReasonCommandInvalid),
		string(GoalReasonDraftRequiresIdle), string(GoalReasonEvidenceTooLarge),
		string(GoalReasonReportConflict), string(GoalReasonSessionBusyQueueFull),
		string(GoalReasonTurnFilterInvalid), string(GoalReasonJudgeUnavailable),
		string(GoalReasonJudgeBroken), string(GoalReasonJudgeOutcomeInvalid),
		string(GoalReasonActionControlInvalid), string(GoalReasonStopReasonInvalid),
		string(GoalReasonPromptRequestFailed), string(GoalReasonAgentRefused),
		string(GoalReasonCompactionCancelled), string(GoalReasonRecoveryAmbiguous),
		string(GoalReasonControlRevokedInFlight), string(GoalReasonReseedConfirmationRequired),
		string(GoalReasonPresubmitRetriesExhausted), string(GoalReasonBudgetFenced),
		string(GoalReasonOriginInvalid), string(GoalReasonOriginWorkspaceMismatch),
		string(GoalReasonOriginProfileUnavailable), string(GoalReasonControlStale),
		string(GoalReasonPromptFenced), string(GoalReasonSessionCreationMismatch),
		string(GoalReasonContinuousBindingMismatch),
	}
}
