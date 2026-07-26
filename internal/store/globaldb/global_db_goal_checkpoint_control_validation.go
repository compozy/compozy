package globaldb

import (
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
)

func validateGoalControlCheckpointRequest(req goal.ControlCheckpointRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	if req.ExpectedControlEpoch < 1 || req.ExpectedBindingEpoch < 0 ||
		!goalCheckpointPhaseValid(strings.TrimSpace(req.ExpectedPhase)) ||
		!goalStatusValid(req.Status) ||
		strings.TrimSpace(req.ActorKind) == "" || strings.TrimSpace(req.ActorID) == "" {
		return fmt.Errorf("%w: goal control checkpoint identity is incomplete", looppkg.ErrValidation)
	}
	if !goalControlCheckpointDispositionMatchesStatus(req.Disposition, req.Status) {
		return fmt.Errorf(
			"%w: Goal disposition %q cannot persist status %q",
			looppkg.ErrValidation,
			req.Disposition,
			req.Status,
		)
	}
	if req.Disposition == looppkg.ActionDispositionSucceeded {
		if req.Cause != "" {
			return fmt.Errorf("%w: successful Goal control cannot persist a cause", looppkg.ErrValidation)
		}
		return nil
	}
	if strings.TrimSpace(string(req.Cause)) == "" {
		return fmt.Errorf("%w: non-success Goal control requires a cause", looppkg.ErrValidation)
	}
	return nil
}

func goalControlCheckpointDispositionMatchesStatus(
	disposition looppkg.ActionDisposition,
	status string,
) bool {
	switch disposition {
	case looppkg.ActionDispositionSucceeded:
		return status == goalStatusComplete
	case looppkg.ActionDispositionBlocked:
		return status == goalStatusBlocked
	case looppkg.ActionDispositionExhausted:
		return status == goalStatusBudgetLimited
	case looppkg.ActionDispositionPaused:
		return status == goalStatusPaused
	case looppkg.ActionDispositionNeedsApproval:
		return status == goalStatusPaused || status == goalStatusUsageLimited || status == goalStatusBudgetLimited
	default:
		return false
	}
}

func goalControlCheckpointTargetPhase(req goal.ControlCheckpointRequest) string {
	switch req.Disposition {
	case looppkg.ActionDispositionSucceeded,
		looppkg.ActionDispositionBlocked,
		looppkg.ActionDispositionExhausted:
		return goalCheckpointPhaseTerminal
	default:
		return goalCheckpointPhaseAwaitingControl
	}
}

func goalControlCheckpointOwnerMatches(
	checkpoint goal.Checkpoint,
	req goal.ControlCheckpointRequest,
) bool {
	return checkpoint.ControlEpoch == req.ExpectedControlEpoch &&
		checkpoint.BindingEpoch == req.ExpectedBindingEpoch &&
		checkpoint.Phase == strings.TrimSpace(req.ExpectedPhase) &&
		checkpoint.TaskRunID == strings.TrimSpace(req.TaskRunID) &&
		checkpoint.QueueEntryID == strings.TrimSpace(req.QueueEntryID) &&
		checkpoint.PromptID == strings.TrimSpace(req.PromptID)
}
