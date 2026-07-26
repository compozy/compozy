package session

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

const (
	managedInputOwnerGoal               = "goal"
	managedInputReasonGoal              = "goal"
	managedInputReasonRecoveryAmbiguous = "goal_recovery_ambiguous"
	managedInputReasonBudgetFenced      = "goal_budget_fenced"
	managedInputReasonControlFenced     = "goal_control_fenced"
	managedInputPromptKindWork          = "work"
	managedInputPromptKindContinuation  = "continuation"
	managedInputPromptKindCompact       = "compact"
)

type managedInputExecution struct {
	lifecycle  ManagedInputLifecycle
	submission ManagedInputSubmission
}

func (m *Manager) startManagedInputPrompt(session *Session, entry store.SessionInputQueueEntry) {
	if m == nil || session == nil {
		return
	}
	lifecycle := m.currentManagedInputLifecycle()
	if lifecycle == nil {
		m.sessionLogger(session).Error(
			"session: managed input lifecycle is unavailable",
			"entry_id", entry.ID,
		)
		return
	}
	owner, err := managedInputOwnerFromEntry(entry)
	if err != nil {
		m.sessionLogger(session).Error("session: invalid managed input owner", "entry_id", entry.ID, "error", err)
		return
	}
	proc, err := session.beginExclusivePromptSetup()
	if err != nil {
		if !errors.Is(err, ErrPromptInProgress) {
			m.sessionLogger(session).Warn("session: reserve managed prompt slot failed", "error", err)
		}
		return
	}
	defer session.finishPromptSetup()
	leaseCtx, cancelLease := context.WithCancel(m.fallbackLifecycleContext())
	if err := m.registerManagedInputLease(owner, cancelLease); err != nil {
		cancelLease()
		m.sessionLogger(session).Warn("session: register managed input lease failed", "error", err)
		return
	}
	submission, err := lifecycle.BeginSubmission(leaseCtx, ManagedInputClaim{
		Owner:       owner,
		RequestedAt: m.now().UTC(),
	})
	if err != nil {
		m.handleManagedInputBeginError(leaseCtx, lifecycle, owner, err)
		m.releaseManagedInputLease(owner)
		return
	}
	if err := validateManagedInputSubmission(owner, submission); err != nil {
		m.recordManagedInputAmbiguous(leaseCtx, lifecycle, submission, managedInputReasonRecoveryAmbiguous, err)
		m.releaseManagedInputLease(owner)
		return
	}
	execution := &managedInputExecution{lifecycle: lifecycle, submission: submission}
	req, err := managedInputPromptRequest(entry, submission)
	if err != nil {
		m.recordManagedInputAmbiguous(leaseCtx, lifecycle, submission, managedInputReasonRecoveryAmbiguous, err)
		m.releaseManagedInputLease(owner)
		return
	}
	message, err := m.dispatchInputPreSubmit(leaseCtx, session, req.turnID, req.turnSource, req.message)
	if err != nil {
		m.recordManagedInputAmbiguous(leaseCtx, lifecycle, submission, managedInputReasonRecoveryAmbiguous, err)
		m.releaseManagedInputLease(owner)
		return
	}
	turnState := newPromptTurnDispatchState(session, req.turnID, req.turnSource, message)
	turnState.managed = execution
	if err := m.dispatchTurnStart(leaseCtx, turnState); err != nil {
		m.recordManagedInputAmbiguous(leaseCtx, lifecycle, submission, managedInputReasonRecoveryAmbiguous, err)
		m.releaseManagedInputLease(owner)
		return
	}
	events, err := m.submitPromptInReservedSlot(leaseCtx, session, proc, req, message, turnState)
	if err != nil {
		m.recordManagedInputAmbiguous(leaseCtx, lifecycle, submission, managedInputReasonRecoveryAmbiguous, err)
		m.releaseManagedInputLease(owner)
		return
	}
	go drainPromptSource(events)
}

func managedInputOwnerFromEntry(entry store.SessionInputQueueEntry) (ManagedInputOwner, error) {
	if entry.RunGeneration == nil || entry.OwnerEpoch == nil || entry.BindingEpoch == nil ||
		strings.TrimSpace(entry.ID) == "" || strings.TrimSpace(entry.SessionID) == "" ||
		entry.OwnerKind != managedInputOwnerGoal || strings.TrimSpace(entry.LoopRunID) == "" ||
		strings.TrimSpace(entry.TaskRunID) == "" || strings.TrimSpace(entry.PromptID) == "" ||
		strings.TrimSpace(entry.PromptKind) == "" || entry.PromptAttempt < 0 {
		return ManagedInputOwner{}, errors.New("session: managed input queue owner is incomplete")
	}
	return ManagedInputOwner{
		QueueEntryID:  entry.ID,
		SessionID:     entry.SessionID,
		OwnerKind:     entry.OwnerKind,
		LoopRunID:     entry.LoopRunID,
		TaskRunID:     entry.TaskRunID,
		RunGeneration: int(*entry.RunGeneration),
		PromptAttempt: entry.PromptAttempt,
		ControlEpoch:  *entry.OwnerEpoch,
		BindingEpoch:  *entry.BindingEpoch,
		PromptID:      entry.PromptID,
		PromptKind:    entry.PromptKind,
	}, nil
}

func validateManagedInputSubmission(owner ManagedInputOwner, submission ManagedInputSubmission) error {
	if !managedInputOwnersEqual(owner, submission.Owner) ||
		strings.TrimSpace(submission.DispatchToken) == "" || submission.StartedAt.IsZero() ||
		strings.TrimSpace(submission.PromptMeta.PromptID) != owner.PromptID ||
		strings.TrimSpace(submission.PromptMeta.LoopRunID) != owner.LoopRunID ||
		strings.TrimSpace(submission.PromptMeta.Kind) != owner.PromptKind {
		return errors.New("session: managed input submission identity changed")
	}
	_, err := goalPromptMetaFromManagedInput(submission.PromptMeta)
	return err
}

func managedInputPromptRequest(
	entry store.SessionInputQueueEntry,
	submission ManagedInputSubmission,
) (promptRequest, error) {
	goalMeta, err := goalPromptMetaFromManagedInput(submission.PromptMeta)
	if err != nil {
		return promptRequest{}, err
	}
	return promptRequest{
		turnID:          submission.PromptMeta.PromptID,
		target:          entry.SessionID,
		message:         entry.Text,
		authoredMessage: entry.Text,
		turnSource:      TurnSourceSynthetic,
		meta: acp.PromptMeta{
			TurnSource: string(TurnSourceSynthetic),
			Synthetic: &acp.PromptSyntheticMeta{
				TaskRunID:  entry.TaskRunID,
				WorkflowID: entry.LoopRunID,
				Reason:     managedInputReasonGoal,
				Goal:       goalMeta,
			},
		},
	}, nil
}

func (m *Manager) handleManagedInputBeginError(
	ctx context.Context,
	lifecycle ManagedInputLifecycle,
	owner ManagedInputOwner,
	cause error,
) {
	var effectErr *ManagedInputEffectError
	if !errors.As(cause, &effectErr) || effectErr.Finalized {
		return
	}
	switch effectErr.Certainty {
	case EffectKnownFalse:
		if err := lifecycle.RecordRejected(ctx, ManagedInputRejection{
			Owner:            owner,
			ReasonCode:       effectErr.ReasonCode,
			EffectKnownFalse: true,
		}); err != nil {
			m.logManagedInputError("session: record managed input rejection failed", err, owner)
			return
		}
	case EffectUnknown:
		m.recordManagedInputAmbiguous(ctx, lifecycle, ManagedInputSubmission{Owner: owner}, effectErr.ReasonCode, cause)
	}
}

func (m *Manager) recordManagedDriverAttached(
	ctx context.Context,
	execution *managedInputExecution,
	driverTurnID string,
) error {
	if execution == nil || execution.lifecycle == nil {
		return errors.New("session: managed input execution is unavailable")
	}
	if strings.TrimSpace(driverTurnID) != execution.submission.Owner.PromptID {
		return errors.New("session: managed input driver turn identity changed")
	}
	return execution.lifecycle.RecordDriverAttached(ctx, ManagedInputReceipt{
		Owner:         execution.submission.Owner,
		DispatchToken: execution.submission.DispatchToken,
		DriverTurnID:  driverTurnID,
	})
}

func (m *Manager) recordManagedInputAmbiguous(
	ctx context.Context,
	lifecycle ManagedInputLifecycle,
	submission ManagedInputSubmission,
	reasonCode string,
	cause error,
) {
	if lifecycle == nil {
		return
	}
	reason := strings.TrimSpace(reasonCode)
	if reason == "" || reason == managedInputReasonBudgetFenced || reason == managedInputReasonControlFenced {
		reason = managedInputReasonRecoveryAmbiguous
	}
	if err := lifecycle.RecordAmbiguous(ctx, ManagedInputReceipt{
		Owner:         submission.Owner,
		DispatchToken: submission.DispatchToken,
		DriverTurnID:  submission.Owner.PromptID,
		ReasonCode:    reason,
	}); err != nil {
		m.logManagedInputError(
			"session: record managed input ambiguity failed",
			errors.Join(cause, err),
			submission.Owner,
		)
	}
}

func (m *Manager) logManagedInputError(msg string, err error, owner ManagedInputOwner) {
	logger := slog.Default()
	if m != nil && m.logger != nil {
		logger = m.logger
	}
	logger.Error(
		msg,
		"error", err,
		"queue_entry_id", owner.QueueEntryID,
		"prompt_id", owner.PromptID,
	)
}
