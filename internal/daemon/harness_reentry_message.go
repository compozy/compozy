package daemon

import (
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/session"

	taskpkg "github.com/compozy/agh/internal/task"
)

func detachedHarnessReentryProcessed(state *detachedHarnessReentry) bool {
	return state != nil && !state.ProcessedAt.IsZero()
}

func isDetachedHarnessTerminalTaskEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case harnessTaskEventRunCompleted, harnessTaskEventRunFailed, harnessTaskEventRunCanceled:
		return true
	default:
		return false
	}
}

func isDetachedHarnessTerminalRun(status taskpkg.RunStatus) bool {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusCompleted, taskpkg.TaskRunStatusFailed, taskpkg.TaskRunStatusCanceled:
		return true
	default:
		return false
	}
}

func syntheticReasonForTerminalRun(status taskpkg.RunStatus) (string, string) {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusFailed:
		return harnessReentryReasonFailed, harnessTaskEventRunFailed
	case taskpkg.TaskRunStatusCanceled:
		return harnessReentryReasonCanceled, harnessTaskEventRunCanceled
	default:
		return harnessReentryReasonCompleted, harnessTaskEventRunCompleted
	}
}

func buildDetachedHarnessSyntheticMessage(task taskpkg.Task, run taskpkg.Run, summary string) string {
	switch run.Status.Normalize() {
	case taskpkg.TaskRunStatusFailed:
		return fmt.Sprintf(
			"Detached harness work failed for %q (task %s, run %s): %s",
			summary,
			task.ID,
			run.ID,
			strings.TrimSpace(run.Error),
		)
	case taskpkg.TaskRunStatusCanceled:
		return fmt.Sprintf(
			"Detached harness work was canceled for %q (task %s, run %s).",
			summary,
			task.ID,
			run.ID,
		)
	default:
		return fmt.Sprintf(
			"Detached harness work completed for %q (task %s, run %s).",
			summary,
			task.ID,
			run.ID,
		)
	}
}

func detachedCompletionSummary(taskID string, run taskpkg.Run, metadata detachedHarnessRunMetadata) string {
	return fmt.Sprintf(
		"task=%s run=%s status=%s target_session=%s summary=%q",
		strings.TrimSpace(taskID),
		strings.TrimSpace(run.ID),
		run.Status.Normalize(),
		strings.TrimSpace(metadata.WakeTarget.SessionID),
		detachedHarnessSummary(metadata.Summary),
	)
}

func syntheticOutcomeSummary(taskID string, runID string, outcome string, reason string) string {
	return fmt.Sprintf(
		"task=%s run=%s disposition=%s reason=%s",
		strings.TrimSpace(taskID),
		strings.TrimSpace(runID),
		strings.TrimSpace(outcome),
		strings.TrimSpace(reason),
	)
}

func inactiveTargetReason(state session.State) string {
	trimmed := strings.TrimSpace(string(state))
	if trimmed == "" {
		return harnessReentryReasonTargetInactivePrefix
	}
	return harnessReentryReasonTargetInactivePrefix + ":" + trimmed
}

func classifySyntheticPromptError(err error) string {
	switch {
	case errors.Is(err, session.ErrSessionNotFound):
		return harnessReentryReasonTargetMissing
	case errors.Is(err, session.ErrSessionNotActive):
		return harnessReentryReasonTargetInactivePrefix
	default:
		return harnessReentryReasonDispatchFailed
	}
}
