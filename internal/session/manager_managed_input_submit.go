package session

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
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

type managedInput struct {
	id            string
	sessionID     string
	text          string
	runtime       store.SessionInputRuntime
	taskRunID     string
	runGeneration *int64
	loopRunID     string
	ownerKind     string
	ownerEpoch    *int64
	bindingEpoch  *int64
	promptID      string
	promptKind    string
	promptAttempt int
}

func managedInputFromQueueEntry(entry *store.SessionInputQueueEntry) managedInput {
	return managedInput{
		id:            entry.ID,
		sessionID:     entry.SessionID,
		text:          entry.Text,
		runtime:       entry.Runtime,
		taskRunID:     entry.TaskRunID,
		runGeneration: entry.RunGeneration,
		loopRunID:     entry.LoopRunID,
		ownerKind:     entry.OwnerKind,
		ownerEpoch:    entry.OwnerEpoch,
		bindingEpoch:  entry.BindingEpoch,
		promptID:      entry.PromptID,
		promptKind:    entry.PromptKind,
		promptAttempt: entry.PromptAttempt,
	}
}

func (m *Manager) startManagedInputPrompt(session *Session, entry managedInput) {
	if m == nil || session == nil {
		return
	}
	lifecycle := m.currentManagedInputLifecycle()
	if lifecycle == nil {
		m.sessionLogger(session).Error(
			"session: managed input lifecycle is unavailable",
			"entry_id", entry.id,
		)
		return
	}
	owner, err := managedInputOwnerFromEntry(entry)
	if err != nil {
		m.sessionLogger(session).Error("session: invalid managed input owner", "entry_id", entry.id, "error", err)
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
	proc, err = m.ensurePromptRuntime(leaseCtx, session, req.runtime, proc)
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

func managedInputOwnerFromEntry(entry managedInput) (ManagedInputOwner, error) {
	if entry.runGeneration == nil || entry.ownerEpoch == nil || entry.bindingEpoch == nil ||
		strings.TrimSpace(entry.id) == "" || strings.TrimSpace(entry.sessionID) == "" ||
		entry.ownerKind != managedInputOwnerGoal || strings.TrimSpace(entry.loopRunID) == "" ||
		strings.TrimSpace(entry.taskRunID) == "" || strings.TrimSpace(entry.promptID) == "" ||
		strings.TrimSpace(entry.promptKind) == "" || entry.promptAttempt < 0 {
		return ManagedInputOwner{}, errors.New("session: managed input queue owner is incomplete")
	}
	return ManagedInputOwner{
		QueueEntryID:  entry.id,
		SessionID:     entry.sessionID,
		OwnerKind:     entry.ownerKind,
		LoopRunID:     entry.loopRunID,
		TaskRunID:     entry.taskRunID,
		RunGeneration: int(*entry.runGeneration),
		PromptAttempt: entry.promptAttempt,
		ControlEpoch:  *entry.ownerEpoch,
		BindingEpoch:  *entry.bindingEpoch,
		PromptID:      entry.promptID,
		PromptKind:    entry.promptKind,
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
	entry managedInput,
	submission ManagedInputSubmission,
) (promptRequest, error) {
	goalMeta, err := goalPromptMetaFromManagedInput(submission.PromptMeta)
	if err != nil {
		return promptRequest{}, err
	}
	return promptRequest{
		turnID:          submission.PromptMeta.PromptID,
		target:          entry.sessionID,
		message:         entry.text,
		authoredMessage: entry.text,
		turnSource:      TurnSourceSynthetic,
		runtime:         runtimeSelectionFromStore(entry.runtime),
		meta: acp.PromptMeta{
			TurnSource: string(TurnSourceSynthetic),
			Synthetic: &acp.PromptSyntheticMeta{
				TaskRunID:  entry.taskRunID,
				WorkflowID: entry.loopRunID,
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
		promptEvidenceQueueEntryIDKey, owner.QueueEntryID,
		"prompt_id", owner.PromptID,
	)
}
