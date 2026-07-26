package task

import (
	"fmt"
	"maps"
	"strings"
	"time"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	diagnosticitems "github.com/compozy/agh/internal/diagnostics"
)

const (
	inspectDiagnosticsDocURL             = "/runtime/core/autonomy/task-runs-and-leases#task-inspect-diagnostics"
	inspectExpectedHeartbeatInterval     = time.Minute
	inspectQueuedStrandedAfter           = 5 * time.Minute
	inspectSessionFailedState            = "failed"
	inspectSessionMissingState           = "missing"
	inspectSessionStoppedState           = "stopped"
	inspectStuckHeartbeatThresholdFactor = 2
)

type inspectDiagnosticSnapshot struct {
	Task                  Summary
	CurrentRun            *InspectRunSummary
	BoundSession          *InspectSessionSummary
	RecentRuns            []InspectRunSummary
	Scheduler             InspectSchedulerState
	AsOf                  time.Time
	EligibleSessionCount  int
	SessionCatalogPresent bool
}

func detectInspectDiagnostics(snapshot *inspectDiagnosticSnapshot) []diagnosticcontract.DiagnosticItem {
	if snapshot == nil {
		return nil
	}
	if snapshot.CurrentRun == nil {
		return nil
	}

	diagnostics := make([]diagnosticcontract.DiagnosticItem, 0, 5)
	if item, ok := detectInspectStuckRun(snapshot); ok {
		diagnostics = append(diagnostics, item)
	}
	if item, ok := detectInspectStaleLease(snapshot); ok {
		diagnostics = append(diagnostics, item)
	}
	if item, ok := detectInspectOrphanRun(snapshot); ok {
		diagnostics = append(diagnostics, item)
	}
	if item, ok := detectInspectCrashedRun(snapshot); ok {
		diagnostics = append(diagnostics, item)
	}
	if item, ok := detectInspectStrandedRun(snapshot); ok {
		diagnostics = append(diagnostics, item)
	}
	return diagnostics
}

func noInspectDiagnosticItem() (diagnosticcontract.DiagnosticItem, bool) {
	var item diagnosticcontract.DiagnosticItem
	return item, false
}

func detectInspectStuckRun(snapshot *inspectDiagnosticSnapshot) (diagnosticcontract.DiagnosticItem, bool) {
	run := snapshot.CurrentRun
	if run.Status.Normalize() != TaskRunStatusClaimed || run.HeartbeatAgeSeconds == nil {
		return noInspectDiagnosticItem()
	}
	thresholdSeconds := int64((inspectExpectedHeartbeatInterval * inspectStuckHeartbeatThresholdFactor).Seconds())
	if *run.HeartbeatAgeSeconds <= thresholdSeconds {
		return noInspectDiagnosticItem()
	}
	return inspectDiagnosticItem(
		diagnosticcontract.CodeTaskRunStuck,
		run.RunID,
		"Task run heartbeat is stale",
		fmt.Sprintf("Run %s is claimed, but its heartbeat is older than the expected threshold.", run.RunID),
		diagnosticcontract.SeverityWarn,
		fmt.Sprintf("agh task release %s --reason \"stuck\"", run.RunID),
		inspectRunEvidence(snapshot, map[string]any{
			"heartbeat_age_seconds": *run.HeartbeatAgeSeconds,
			"threshold_seconds":     thresholdSeconds,
		}),
	), true
}

func detectInspectStaleLease(snapshot *inspectDiagnosticSnapshot) (diagnosticcontract.DiagnosticItem, bool) {
	run := snapshot.CurrentRun
	if run.Status.Normalize() != TaskRunStatusClaimed ||
		run.LeaseUntil.IsZero() ||
		!run.LeaseUntil.Before(snapshot.AsOf) {
		return noInspectDiagnosticItem()
	}
	return inspectDiagnosticItem(
		diagnosticcontract.CodeTaskRunStaleLease,
		run.RunID,
		"Task run lease is stale",
		fmt.Sprintf("Run %s still appears claimed after its lease expired.", run.RunID),
		diagnosticcontract.SeverityError,
		fmt.Sprintf("agh task release %s --reason \"stale lease\"", run.RunID),
		inspectRunEvidence(snapshot, map[string]any{
			"lease_until": run.LeaseUntil,
			"as_of":       snapshot.AsOf,
		}),
	), true
}

func detectInspectOrphanRun(snapshot *inspectDiagnosticSnapshot) (diagnosticcontract.DiagnosticItem, bool) {
	run := snapshot.CurrentRun
	if IsTerminalRunStatus(run.Status) ||
		strings.TrimSpace(run.ClaimTokenHashTruncated) == "" ||
		!inspectSessionIsTerminal(snapshot.BoundSession) {
		return noInspectDiagnosticItem()
	}
	return inspectDiagnosticItem(
		diagnosticcontract.CodeTaskRunOrphan,
		run.RunID,
		"Task run is bound to a terminal session",
		fmt.Sprintf("Run %s still has an ownership token, but its bound session is no longer active.", run.RunID),
		diagnosticcontract.SeverityError,
		fmt.Sprintf("agh task release %s --reason \"orphaned\"", run.RunID),
		inspectRunEvidence(snapshot, map[string]any{
			"session_id":      inspectSessionID(snapshot.BoundSession),
			"session_state":   inspectSessionState(snapshot.BoundSession),
			"session_failure": inspectSessionFailure(snapshot.BoundSession),
		}),
	), true
}

func detectInspectCrashedRun(snapshot *inspectDiagnosticSnapshot) (diagnosticcontract.DiagnosticItem, bool) {
	run := snapshot.CurrentRun
	if run.Status.Normalize() != TaskRunStatusFailed || inspectHasRetryAfter(run, snapshot.RecentRuns) {
		return noInspectDiagnosticItem()
	}
	return inspectDiagnosticItem(
		diagnosticcontract.CodeTaskRunCrashed,
		run.RunID,
		"Task run failed without a queued retry",
		fmt.Sprintf("Run %s is failed and no later retry attempt is visible in the task snapshot.", run.RunID),
		diagnosticcontract.SeverityError,
		fmt.Sprintf("agh task run enqueue %s", run.TaskID),
		inspectRunEvidence(snapshot, map[string]any{
			"last_error_summary": run.LastErrorSummary,
			"failure_kind":       run.FailureKind,
		}),
	), true
}

func detectInspectStrandedRun(snapshot *inspectDiagnosticSnapshot) (diagnosticcontract.DiagnosticItem, bool) {
	run := snapshot.CurrentRun
	if run.Status.Normalize() != TaskRunStatusQueued || run.QueuedAt.IsZero() || snapshot.Scheduler.Paused {
		return noInspectDiagnosticItem()
	}
	if !snapshot.SessionCatalogPresent || snapshot.EligibleSessionCount > 0 {
		return noInspectDiagnosticItem()
	}
	queuedAge := snapshot.AsOf.Sub(run.QueuedAt)
	if queuedAge < inspectQueuedStrandedAfter {
		return noInspectDiagnosticItem()
	}
	return inspectDiagnosticItem(
		diagnosticcontract.CodeTaskRunStranded,
		run.RunID,
		"Queued task run has no eligible session",
		fmt.Sprintf(
			"Run %s has been queued while the scheduler is active, but no eligible session is visible.",
			run.RunID,
		),
		diagnosticcontract.SeverityWarn,
		"agh task next --wait",
		inspectRunEvidence(snapshot, map[string]any{
			"queued_age_seconds":     int64(queuedAge.Seconds()),
			"threshold_seconds":      int64(inspectQueuedStrandedAfter.Seconds()),
			"eligible_session_count": snapshot.EligibleSessionCount,
			"scheduler_paused":       snapshot.Scheduler.Paused,
		}),
	), true
}

func inspectDiagnosticItem(
	code string,
	runID string,
	title string,
	message string,
	severity string,
	suggestedCommand string,
	evidence map[string]any,
) diagnosticcontract.DiagnosticItem {
	return diagnosticitems.NewItem(
		"task.inspect."+code+"."+strings.TrimSpace(runID),
		code,
		diagnosticcontract.CategoryTask,
		title,
		message,
		severity,
		diagnosticcontract.FreshnessLive,
		diagnosticitems.WithSuggestedCommand(suggestedCommand),
		diagnosticitems.WithDocURL(inspectDiagnosticsDocURL),
		diagnosticitems.WithEvidence(evidence),
	)
}

func inspectRunEvidence(snapshot *inspectDiagnosticSnapshot, extra map[string]any) map[string]any {
	run := snapshot.CurrentRun
	evidence := map[string]any{
		taskEvidenceIDKey:            snapshot.Task.ID,
		runEvidenceIDKey:             run.RunID,
		leaseStatusKey:               run.Status.String(),
		runFieldAttempt:              run.Attempt,
		"claim_token_hash_truncated": run.ClaimTokenHashTruncated,
		"lease_until":                run.LeaseUntil,
		"heartbeat_at":               run.HeartbeatAt,
		"as_of":                      snapshot.AsOf,
	}
	maps.Copy(evidence, extra)
	return evidence
}

func inspectSessionIsTerminal(session *InspectSessionSummary) bool {
	if session == nil {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(session.State))
	switch state {
	case inspectSessionMissingState, inspectSessionStoppedState, inspectSessionFailedState, "crashed":
		return true
	}
	return strings.TrimSpace(session.FailureKind) != "" || strings.TrimSpace(session.StopReason) != ""
}

func inspectSessionID(session *InspectSessionSummary) string {
	if session == nil {
		return ""
	}
	return session.SessionID
}

func inspectSessionState(session *InspectSessionSummary) string {
	if session == nil {
		return ""
	}
	return session.State
}

func inspectSessionFailure(session *InspectSessionSummary) string {
	if session == nil {
		return ""
	}
	return session.FailureKind
}

func inspectHasRetryAfter(run *InspectRunSummary, recentRuns []InspectRunSummary) bool {
	for _, candidate := range recentRuns {
		if candidate.TaskID != run.TaskID || candidate.Attempt <= run.Attempt || candidate.RunID == run.RunID {
			continue
		}
		if candidate.Status.Normalize() != TaskRunStatusCanceled {
			return true
		}
	}
	return false
}

func inspectNextAction(view *InspectView) InspectNextAction {
	if view == nil {
		return InspectNextActionWaitingForSession
	}
	if inspectHasRecoveryDiagnostic(view.Diagnostics) {
		return InspectNextActionRecoveryRequired
	}
	if view.CurrentRun == nil {
		if isTerminalTaskStatus(view.Task.Status) {
			return InspectNextActionTerminal
		}
		return InspectNextActionWaitingForSession
	}
	switch view.CurrentRun.Status.Normalize() {
	case TaskRunStatusQueued:
		if inspectHasDiagnostic(view.Diagnostics, diagnosticcontract.CodeTaskRunStranded) {
			return InspectNextActionStranded
		}
		if view.Scheduler.Paused {
			return InspectNextActionWaitingForSession
		}
		if view.SessionCatalogPresent && view.EligibleSessionCount > 0 {
			return InspectNextActionClaimAvailable
		}
		return InspectNextActionWaitingForSession
	case TaskRunStatusClaimed, TaskRunStatusStarting, TaskRunStatusRunning:
		return InspectNextActionRunning
	case TaskRunStatusCompleted, TaskRunStatusFailed, TaskRunStatusCanceled:
		return InspectNextActionTerminal
	default:
		return InspectNextActionWaitingForSession
	}
}

func inspectHasRecoveryDiagnostic(items []diagnosticcontract.DiagnosticItem) bool {
	for _, code := range []string{
		diagnosticcontract.CodeTaskRunStuck,
		diagnosticcontract.CodeTaskRunStaleLease,
		diagnosticcontract.CodeTaskRunOrphan,
		diagnosticcontract.CodeTaskRunCrashed,
	} {
		if inspectHasDiagnostic(items, code) {
			return true
		}
	}
	return false
}

func inspectHasDiagnostic(items []diagnosticcontract.DiagnosticItem, code string) bool {
	for _, item := range items {
		if strings.TrimSpace(item.Code) == code {
			return true
		}
	}
	return false
}
