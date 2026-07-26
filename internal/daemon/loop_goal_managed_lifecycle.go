package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

type loopGoalManagedRuntimeStore interface {
	loopGoalPromptRuntimeStore
	goalpkg.BudgetGuard
	goalpkg.PreparedPromptRejectionStore
	GetTaskRun(context.Context, string) (taskpkg.Run, error)
}

type loopGoalManagedInputLifecycle struct {
	store  loopGoalManagedRuntimeStore
	binder *loopActionSessionBinder
	now    func() time.Time
}

var _ session.ManagedInputLifecycle = (*loopGoalManagedInputLifecycle)(nil)

type managedGoalTaskRunMetadata struct {
	Generation       int    `json:"generation"`
	NodeID           string `json:"node_id"`
	ItemIndex        int    `json:"item_index"`
	GoalSegmentEpoch int64  `json:"goal_segment_epoch"`
}

type managedGoalOperation struct {
	key        goalpkg.TurnKey
	entry      store.SessionInputQueueEntry
	checkpoint goalpkg.Checkpoint
	taskRun    taskpkg.Run
}

func (l *loopGoalManagedInputLifecycle) BeginSubmission(
	ctx context.Context,
	claim session.ManagedInputClaim,
) (session.ManagedInputSubmission, error) {
	operation, err := l.loadOperation(ctx, claim.Owner)
	if err != nil {
		return session.ManagedInputSubmission{}, l.effectError(claim.Owner, session.EffectKnownFalse, err)
	}
	base, reported := managedGoalUsageBase(operation.entry.OperationUsageBaseTokens)
	boundary, turn := managedGoalReadyBoundary(&operation)
	decision, err := l.store.FlushAndCheck(ctx, goalpkg.BudgetBoundarySnapshot{
		Key:                  operation.key,
		TaskRunID:            operation.taskRun.ID,
		ExpectedControlEpoch: operation.checkpoint.ControlEpoch,
		ExpectedBindingEpoch: operation.checkpoint.BindingEpoch,
		ExpectedPhase:        operation.checkpoint.Phase,
		ExpectedQueueEntryID: operation.checkpoint.QueueEntryID,
		ExpectedPromptID:     operation.checkpoint.PromptID,
		Boundary:             boundary,
		Phase:                loopManagedReadySlotPhase,
		Turn:                 int64(turn),
		OperationID:          operation.entry.PromptID,
		OperationBaseTokens:  base,
		LiveTokensUsed:       base,
		TokensReported:       reported,
	})
	if err != nil {
		return session.ManagedInputSubmission{}, l.effectError(claim.Owner, session.EffectKnownFalse, err)
	}
	if !decision.Allowed {
		if err := l.store.FencePreparedPrompt(ctx, goalpkg.FencePreparedPromptRequest{
			Key:                  operation.key,
			ExpectedControlEpoch: claim.Owner.ControlEpoch,
			ExpectedBindingEpoch: claim.Owner.BindingEpoch,
			TaskRunID:            claim.Owner.TaskRunID,
			QueueEntryID:         claim.Owner.QueueEntryID,
			PromptID:             claim.Owner.PromptID,
			Outcome:              looppkg.ActionPromptOutcomeBudgetFenced,
			Disposition:          decision.Disposition,
			Cause:                decision.Cause,
			BudgetDecision:       decision,
		}); err != nil {
			return session.ManagedInputSubmission{}, l.effectError(claim.Owner, session.EffectUnknown, err)
		}
		l.clearUsageReporter(claim.Owner.QueueEntryID)
		return session.ManagedInputSubmission{}, &session.ManagedInputEffectError{
			Owner:      claim.Owner,
			Certainty:  session.EffectKnownFalse,
			ReasonCode: string(decision.Cause),
			Finalized:  true,
			Err:        errors.New("daemon: managed Goal prompt fenced before submission"),
		}
	}
	claimed, err := l.claimPreparedPrompt(ctx, &operation, decision, base)
	if err != nil {
		return l.reconcileClaimError(ctx, &operation, claimed, err)
	}
	return l.submission(&operation, claimed, decision.BudgetVersion), nil
}

func (l *loopGoalManagedInputLifecycle) RecordDriverAttached(
	ctx context.Context,
	receipt session.ManagedInputReceipt,
) error {
	operation, err := l.loadOperation(ctx, receipt.Owner)
	if err != nil {
		return err
	}
	return l.store.RecordGoalDriverAttached(ctx, goalpkg.DriverAttachmentRequest{
		Key:                  operation.key,
		ExpectedControlEpoch: receipt.Owner.ControlEpoch,
		ExpectedBindingEpoch: receipt.Owner.BindingEpoch,
		TaskRunID:            receipt.Owner.TaskRunID,
		QueueEntryID:         receipt.Owner.QueueEntryID,
		PromptID:             receipt.Owner.PromptID,
		SessionID:            receipt.Owner.SessionID,
		BindingHandle:        operation.checkpoint.BindingHandle,
		DispatchToken:        receipt.DispatchToken,
		DriverTurnID:         receipt.DriverTurnID,
		AttachedAt:           l.currentTime(),
	})
}

func (l *loopGoalManagedInputLifecycle) RecordRejected(
	ctx context.Context,
	rejection session.ManagedInputRejection,
) error {
	if !rejection.EffectKnownFalse {
		return errors.New("daemon: managed Goal rejection lacks effect-known-false proof")
	}
	operation, err := l.loadOperation(ctx, rejection.Owner)
	if err != nil {
		return err
	}
	err = l.store.RejectPreparedGoalPrompt(ctx, goalpkg.RejectPreparedPromptRequest{
		Key:                  operation.key,
		ExpectedControlEpoch: rejection.Owner.ControlEpoch,
		ExpectedBindingEpoch: rejection.Owner.BindingEpoch,
		TaskRunID:            rejection.Owner.TaskRunID,
		QueueEntryID:         rejection.Owner.QueueEntryID,
		PromptID:             rejection.Owner.PromptID,
		ReasonCode:           managedGoalReasonCode(rejection.ReasonCode),
	})
	if err == nil {
		l.clearUsageReporter(rejection.Owner.QueueEntryID)
	}
	return err
}

func (l *loopGoalManagedInputLifecycle) RecordAmbiguous(
	ctx context.Context,
	receipt session.ManagedInputReceipt,
) error {
	operation, err := l.loadOperation(ctx, receipt.Owner)
	if err != nil {
		entry, lookupErr := l.store.GetSessionInputQueueEntryByID(ctx, receipt.Owner.QueueEntryID)
		if lookupErr == nil && managedGoalEntrySettled(entry) {
			return nil
		}
		return errors.Join(err, lookupErr)
	}
	err = l.store.MarkAmbiguous(ctx, goalpkg.AmbiguousRequest{
		Key:                  operation.key,
		ExpectedControlEpoch: receipt.Owner.ControlEpoch,
		ExpectedBindingEpoch: receipt.Owner.BindingEpoch,
		TaskRunID:            receipt.Owner.TaskRunID,
		QueueEntryID:         receipt.Owner.QueueEntryID,
		PromptID:             receipt.Owner.PromptID,
		Cause:                looppkg.ReasonCodeGoalRecoveryAmbiguous,
	})
	if err == nil {
		l.clearUsageReporter(receipt.Owner.QueueEntryID)
	}
	return err
}

func (l *loopGoalManagedInputLifecycle) RecordTerminal(
	ctx context.Context,
	terminal session.ManagedInputTerminal,
) error {
	operation, err := l.loadOperation(ctx, terminal.Owner)
	if err != nil {
		return err
	}
	result := looppkg.ActionPromptResult{
		PromptID:       terminal.Owner.PromptID,
		Outcome:        looppkg.ActionPromptOutcome(strings.TrimSpace(terminal.Outcome)),
		Text:           terminal.Text,
		Structured:     append(json.RawMessage(nil), terminal.Structured...),
		EventStartSeq:  terminal.EventStartSeq,
		EventEndSeq:    terminal.EventEndSeq,
		TokensUsed:     terminal.TokensUsed,
		TokensReported: terminal.TokensReported,
		StopReason:     looppkg.ActionStopReason(strings.TrimSpace(string(terminal.StopReason))),
		ReasonCode:     managedGoalReasonCode(terminal.ReasonCode),
	}
	err = l.store.FinalizeGoalPrompt(ctx, goalpkg.FinalizePromptRequest{
		Key:                  operation.key,
		ExpectedControlEpoch: terminal.Owner.ControlEpoch,
		ExpectedBindingEpoch: terminal.Owner.BindingEpoch,
		TaskRunID:            terminal.Owner.TaskRunID,
		QueueEntryID:         terminal.Owner.QueueEntryID,
		PromptID:             terminal.Owner.PromptID,
		SessionID:            terminal.Owner.SessionID,
		BindingHandle:        operation.checkpoint.BindingHandle,
		DispatchToken:        terminal.DispatchToken,
		Result:               result,
		TerminalAt:           terminal.TerminalAt,
	})
	if err == nil {
		l.clearUsageReporter(terminal.Owner.QueueEntryID)
	}
	return err
}

func (l *loopGoalManagedInputLifecycle) loadOperation(
	ctx context.Context,
	owner session.ManagedInputOwner,
) (managedGoalOperation, error) {
	if l == nil || l.store == nil {
		return managedGoalOperation{}, errors.New("daemon: managed Goal lifecycle store is unavailable")
	}
	if err := validateManagedGoalOwner(owner); err != nil {
		return managedGoalOperation{}, err
	}
	entry, err := l.store.GetSessionInputQueueEntryByID(ctx, owner.QueueEntryID)
	if err != nil {
		return managedGoalOperation{}, err
	}
	if !managedGoalEntryMatchesOwner(entry, owner) {
		return managedGoalOperation{}, fmt.Errorf("%w: managed Goal queue owner changed", looppkg.ErrTransitionConflict)
	}
	taskRun, err := l.store.GetTaskRun(ctx, owner.TaskRunID)
	if err != nil {
		return managedGoalOperation{}, err
	}
	metadata, err := decodeManagedGoalTaskMetadata(taskRun.Metadata)
	if err != nil {
		return managedGoalOperation{}, err
	}
	if !managedGoalTaskMatchesOwner(taskRun, metadata, owner) {
		return managedGoalOperation{}, fmt.Errorf("%w: managed Goal task owner changed", looppkg.ErrTransitionConflict)
	}
	run, err := l.store.GetLoopRunByID(ctx, looppkg.RunID(owner.LoopRunID))
	if err != nil {
		return managedGoalOperation{}, err
	}
	key := goalpkg.TurnKey{
		WorkspaceID: run.WorkspaceID,
		LoopRunID:   run.ID,
		Generation:  metadata.Generation,
		NodeID:      looppkg.NodeID(metadata.NodeID),
		ItemIndex:   metadata.ItemIndex,
	}
	checkpoint, err := l.store.LoadCheckpoint(ctx, key)
	if err != nil {
		return managedGoalOperation{}, err
	}
	if !managedGoalCheckpointMatchesOwner(checkpoint, owner) {
		return managedGoalOperation{}, fmt.Errorf(
			"%w: managed Goal checkpoint owner changed",
			looppkg.ErrTransitionConflict,
		)
	}
	return managedGoalOperation{key: key, entry: entry, checkpoint: checkpoint, taskRun: taskRun}, nil
}

func (l *loopGoalManagedInputLifecycle) claimPreparedPrompt(
	ctx context.Context,
	operation *managedGoalOperation,
	decision goalpkg.BudgetDecision,
	usageBase int64,
) (goalpkg.ClaimedPrompt, error) {
	owner := operationOwner(operation)
	if operation.entry.PromptKind == loopManagedPromptKindCompact {
		return l.store.ClaimPreparedCompaction(ctx, goalpkg.ClaimPreparedCompactionRequest{
			Key: operation.key, ExpectedControlEpoch: owner.ControlEpoch, TaskRunID: owner.TaskRunID,
			QueueEntryID: owner.QueueEntryID, PromptID: owner.PromptID, SessionID: owner.SessionID,
			BindingHandle: operation.checkpoint.BindingHandle, BindingEpoch: owner.BindingEpoch,
			UsageBaseTokens: usageBase, UsageBaseReported: operation.entry.OperationUsageBaseTokens != nil,
			UsageSequence: operation.checkpoint.UsageSequence, BudgetDecision: decision,
		})
	}
	actorKind, actorID := managedGoalDispatchActor(operation.taskRun)
	return l.store.ClaimPreparedWorkPrompt(ctx, goalpkg.ClaimPreparedWorkPromptRequest{
		Key: operation.key, ExpectedControlEpoch: owner.ControlEpoch, TaskRunID: owner.TaskRunID,
		QueueEntryID: owner.QueueEntryID, PromptID: owner.PromptID, SessionID: owner.SessionID,
		BindingHandle: operation.checkpoint.BindingHandle, BindingEpoch: owner.BindingEpoch,
		UsageBaseTokens: usageBase, UsageBaseReported: operation.entry.OperationUsageBaseTokens != nil,
		BudgetDecision: decision, ActorKind: actorKind, ActorID: actorID,
	})
}

func (l *loopGoalManagedInputLifecycle) reconcileClaimError(
	ctx context.Context,
	operation *managedGoalOperation,
	claimed goalpkg.ClaimedPrompt,
	cause error,
) (session.ManagedInputSubmission, error) {
	owner := operationOwner(operation)
	entry, err := l.store.GetSessionInputQueueEntryByID(ctx, operation.entry.ID)
	if err != nil {
		return session.ManagedInputSubmission{}, l.effectError(
			owner,
			session.EffectUnknown,
			errors.Join(cause, err),
		)
	}
	if claimed.DispatchToken != "" && entry.DispatchTokenHash == managedGoalDispatchTokenHash(claimed.DispatchToken) &&
		(entry.Status == store.SessionInputQueueStatusDispatching || entry.Status == store.SessionInputQueueStatusSent) {
		return l.submission(operation, claimed, 0), nil
	}
	if entry.Status == store.SessionInputQueueStatusQueued && entry.DispatchTokenHash == "" &&
		entry.TerminalAt == nil && entry.FenceKind == "" {
		return session.ManagedInputSubmission{}, l.effectError(
			owner,
			session.EffectKnownFalse,
			cause,
		)
	}
	if managedGoalEntrySettled(entry) {
		return session.ManagedInputSubmission{}, &session.ManagedInputEffectError{
			Owner:      owner,
			Certainty:  session.EffectKnownFalse,
			ReasonCode: firstNonEmptyManagedGoal(entry.TerminalReasonCode, entry.FenceReasonCode),
			Finalized:  true,
			Err:        cause,
		}
	}
	if markErr := l.store.MarkAmbiguous(ctx, goalpkg.AmbiguousRequest{
		Key:                  operation.key,
		ExpectedControlEpoch: owner.ControlEpoch,
		ExpectedBindingEpoch: owner.BindingEpoch,
		TaskRunID:            owner.TaskRunID,
		QueueEntryID:         owner.QueueEntryID,
		PromptID:             owner.PromptID,
		Cause:                looppkg.ReasonCodeGoalRecoveryAmbiguous,
	}); markErr != nil {
		return session.ManagedInputSubmission{}, l.effectError(
			owner,
			session.EffectUnknown,
			errors.Join(cause, markErr),
		)
	}
	return session.ManagedInputSubmission{}, &session.ManagedInputEffectError{
		Owner:      owner,
		Certainty:  session.EffectUnknown,
		ReasonCode: string(looppkg.ReasonCodeGoalRecoveryAmbiguous),
		Finalized:  true,
		Err:        cause,
	}
}

func (l *loopGoalManagedInputLifecycle) submission(
	operation *managedGoalOperation,
	claimed goalpkg.ClaimedPrompt,
	budgetVersion int64,
) session.ManagedInputSubmission {
	owner := operationOwner(operation)
	meta := session.ManagedInputPromptMeta{
		LoopRunID: owner.LoopRunID, NodeID: string(operation.key.NodeID), PromptID: owner.PromptID,
		Kind: owner.PromptKind, Generation: operation.key.Generation, ItemIndex: operation.key.ItemIndex,
		PromptAttempt: owner.PromptAttempt,
	}
	if owner.PromptKind != loopManagedPromptKindCompact {
		turn := claimed.Checkpoint.TurnsUsed
		meta.Turn = &turn
	}
	return session.ManagedInputSubmission{
		Owner: owner, PromptMeta: meta, DispatchToken: claimed.DispatchToken,
		BudgetVersion: budgetVersion, StartedAt: l.currentTime(),
		UsageReporter: l.usageReporter(owner.QueueEntryID),
	}
}

func (l *loopGoalManagedInputLifecycle) effectError(
	owner session.ManagedInputOwner,
	certainty session.EffectCertainty,
	err error,
) error {
	return &session.ManagedInputEffectError{
		Owner: owner, Certainty: certainty,
		ReasonCode: string(looppkg.ReasonCodeGoalPromptRequestFailed), Err: err,
	}
}

func (l *loopGoalManagedInputLifecycle) currentTime() time.Time {
	if l != nil && l.now != nil {
		return l.now().UTC()
	}
	return time.Now().UTC()
}

func managedGoalReadyBoundary(operation *managedGoalOperation) (goalpkg.BudgetBoundary, int) {
	if operation.entry.PromptKind == loopManagedPromptKindCompact {
		return goalpkg.BudgetBeforeCompact, operation.checkpoint.TurnsUsed
	}
	return goalpkg.BudgetBeforeWork, operation.checkpoint.TurnsUsed + 1
}

func managedGoalUsageBase(value *int64) (int64, bool) {
	if value == nil {
		return 0, false
	}
	return max(*value, 0), true
}

func managedGoalDispatchActor(run taskpkg.Run) (string, string) {
	if run.ClaimedBy != nil {
		kind := strings.TrimSpace(string(run.ClaimedBy.Kind.Normalize()))
		ref := strings.TrimSpace(run.ClaimedBy.Ref)
		if kind != "" && ref != "" {
			return kind, ref
		}
	}
	return string(taskpkg.ActorKindDaemon), loopActionRuntimeActorRef
}

func managedGoalEntrySettled(entry store.SessionInputQueueEntry) bool {
	return entry.TerminalAt != nil || strings.TrimSpace(entry.FenceKind) != ""
}

func managedGoalDispatchTokenHash(token string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(digest[:])
}

func managedGoalReasonCode(value string) looppkg.ReasonCode {
	return looppkg.ReasonCode(strings.TrimSpace(value))
}
