package task

import (
	"fmt"
	"time"
)

func (c TerminalRunCommand) terminalMutation(current Run) (TerminalRunMutation, error) {
	if !c.fence.Matches(current) {
		return TerminalRunMutation{}, fmt.Errorf(
			"%w: task run %q changed after terminal command admission",
			ErrInvalidStatusTransition,
			c.RunID(),
		)
	}
	next := current
	next.LeaseUntil = time.Time{}
	next.HeartbeatAt = time.Time{}
	next.EndedAt = c.commandAt
	switch c.kind {
	case TerminalRunCommandCompleted:
		next.Status = TaskRunStatusCompleted
		next.Result = cloneRawJSON(c.intent.Result)
		next.Error = ""
	case TerminalRunCommandFailed:
		if c.intent.Failure == nil {
			return TerminalRunMutation{}, fmt.Errorf("%w: terminal failure intent is required", ErrValidation)
		}
		next.Status = TaskRunStatusFailed
		next.Result = nil
		next.Error = c.intent.Failure.Error
	case TerminalRunCommandCanceled:
		next.Status = TaskRunStatusCanceled
		next.Result = nil
		next.Error = ""
	default:
		return TerminalRunMutation{}, fmt.Errorf(
			"%w: terminal run command %q does not produce a terminal mutation",
			ErrValidation,
			c.kind,
		)
	}
	return NewTerminalRunMutation(current, next), nil
}

func (c TerminalRunCommand) failure() RunFailure {
	if c.intent.Failure == nil {
		return RunFailure{}
	}
	failure := *c.intent.Failure
	failure.Metadata = cloneRawJSON(failure.Metadata)
	return failure
}

func (c TerminalRunCommand) cancellation() (CancelRun, cancelRunOptions, error) {
	if c.intent.Cancellation == nil {
		return CancelRun{}, cancelRunOptions{}, fmt.Errorf(
			"%w: terminal cancellation intent is required",
			ErrValidation,
		)
	}
	req := c.intent.Cancellation.Request
	req.Metadata = cloneRawJSON(req.Metadata)
	return req, cancelRunOptions{
		propagatedFromTaskID: c.intent.Cancellation.PropagatedFromTaskID,
		reconcileTask:        c.intent.Cancellation.ReconcileTask,
	}, nil
}

func (c TerminalRunCommand) diagnostic() string {
	return c.intent.Diagnostic
}

func (c TerminalRunCommand) recoveryAudit() *bootRecoveryAudit {
	if c.intent.RecoveryAudit == nil {
		return nil
	}
	return &bootRecoveryAudit{
		recovery:          c.intent.RecoveryAudit.Recovery,
		previousStatus:    c.intent.RecoveryAudit.PreviousStatus,
		previousSessionID: c.intent.RecoveryAudit.PreviousSessionID,
	}
}

func (c TerminalRunCommand) requiredSettlementPhase() TerminalRunCommandPhase {
	if c.StopRequired() {
		return TerminalRunCommandPhaseStopConfirmed
	}
	return TerminalRunCommandPhaseAdmitted
}
