package globaldb

import (
	"context"
	"strings"

	"github.com/compozy/agh/internal/loop/goal"
)

func validateGoalLivePromptOwner(
	ctx context.Context,
	exec taskSQLExecutor,
	checkpoint goal.Checkpoint,
	row *goalPromptRow,
	key goal.TurnKey,
	controlEpoch int64,
	bindingEpoch int64,
	taskRunID string,
	queueEntryID string,
	promptID string,
	sessionID string,
	bindingHandle string,
	allowTerminalPhase bool,
) error {
	normalizedTaskRunID := strings.TrimSpace(taskRunID)
	normalizedQueueEntryID := strings.TrimSpace(queueEntryID)
	normalizedPromptID := strings.TrimSpace(promptID)
	normalizedSessionID := strings.TrimSpace(sessionID)
	normalizedBindingHandle := strings.TrimSpace(bindingHandle)
	phaseValid := checkpoint.Phase == goalCheckpointPhasePrompting ||
		checkpoint.Phase == goalCheckpointPhaseCompacting
	if allowTerminalPhase {
		phaseValid = phaseValid || checkpoint.Phase == goalCheckpointPhaseJudging ||
			checkpoint.Phase == goalCheckpointPhasePersisting
	}
	if !phaseValid || checkpoint.Status != goalStatusActive ||
		checkpoint.ControlEpoch != controlEpoch || checkpoint.BindingEpoch != bindingEpoch ||
		checkpoint.TaskRunID != normalizedTaskRunID || checkpoint.QueueEntryID != normalizedQueueEntryID ||
		checkpoint.PromptID != normalizedPromptID || checkpoint.SessionID != normalizedSessionID ||
		checkpoint.BindingHandle != normalizedBindingHandle {
		return goalControlStaleError("Goal live prompt checkpoint owner changed")
	}
	if err := requireGoalPromptOwner(row, key, normalizedTaskRunID, normalizedQueueEntryID, bindingEpoch); err != nil {
		return err
	}
	if !row.ownerEpoch.Valid || row.ownerEpoch.Int64 != controlEpoch ||
		!row.promptID.Valid || row.promptID.String != normalizedPromptID ||
		row.sessionID != normalizedSessionID {
		return goalControlStaleError("Goal live prompt queue owner changed")
	}
	return validateActiveGoalPromptBinding(
		ctx,
		exec,
		key,
		normalizedBindingHandle,
		bindingEpoch,
		normalizedSessionID,
	)
}
