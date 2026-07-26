package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

const managedGoalPromptPollInterval = 20 * time.Millisecond

type loopGoalPromptRuntimeStore interface {
	goalpkg.TurnStore
	GetSessionInputQueueEntryByID(context.Context, string) (store.SessionInputQueueEntry, error)
	GetLoopRunByID(context.Context, looppkg.RunID) (looppkg.Run, error)
}

type loopManagedInputSessionManager interface {
	NotifyManagedInputReady(string)
	CancelManagedInputLease(session.ManagedInputOwner) bool
	WaitManagedInputLease(context.Context, session.ManagedInputOwner) error
}

type loopSessionEventReader interface {
	Events(context.Context, string, store.EventQuery) ([]store.SessionEvent, error)
}

func (b *loopActionSessionBinder) PrepareActionPrompt(
	ctx context.Context,
	binding looppkg.ActionSessionBinding,
	req looppkg.ActionPromptRequest,
) (looppkg.ActionPromptTicket, error) {
	if b == nil || b.prompts == nil || b.managedInputs == nil {
		return looppkg.ActionPromptTicket{}, fmt.Errorf(
			"%w: managed Goal prompt runtime is unavailable",
			looppkg.ErrActionDependencyMissing,
		)
	}
	key, err := managedGoalPromptKey(binding, req)
	if err != nil {
		return looppkg.ActionPromptTicket{}, err
	}
	queueEntryID := managedGoalQueueEntryID(key, req.PromptID)
	rollbackReporter := b.usageReporters.install(queueEntryID, req.UsageReporter)
	ticket, err := b.prompts.PrepareGoalPrompt(ctx, goalpkg.PreparePromptRequest{
		Key:                  key,
		ExpectedControlEpoch: req.Owner.ControlEpoch,
		TaskRunID:            strings.TrimSpace(req.Owner.TaskRunID),
		QueueEntryID:         queueEntryID,
		PromptID:             strings.TrimSpace(req.PromptID),
		PromptKind:           strings.TrimSpace(req.Kind),
		PromptAttempt:        req.Owner.PromptAttempt,
		SessionID:            strings.TrimSpace(binding.SessionID),
		BindingHandle:        strings.TrimSpace(binding.Handle),
		BindingEpoch:         req.Owner.BindingEpoch,
		UsageBaseTokens:      req.UsageBaseTokens,
		UsageBaseReported:    req.UsageBaseReported,
		ContextUsageSequence: req.ContextUsageSequence,
		ContextUsageUsed:     req.ContextUsageUsed,
		Message:              req.Message,
		PreparedAt:           b.promptNow(),
	})
	if err != nil {
		rollbackReporter()
		return looppkg.ActionPromptTicket{}, err
	}
	b.managedInputs.NotifyManagedInputReady(binding.SessionID)
	return looppkg.ActionPromptTicket{
		QueueEntryID: ticket.QueueEntryID,
		PromptID:     ticket.PromptID,
	}, nil
}

func (b *loopActionSessionBinder) AwaitActionPrompt(
	ctx context.Context,
	ticket looppkg.ActionPromptTicket,
) (looppkg.ActionPromptResult, error) {
	if b == nil || b.prompts == nil {
		return looppkg.ActionPromptResult{}, fmt.Errorf(
			"%w: managed Goal prompt store is unavailable",
			looppkg.ErrActionDependencyMissing,
		)
	}
	if ctx == nil {
		return looppkg.ActionPromptResult{}, errors.New("daemon: managed Goal prompt context is required")
	}
	queueEntryID := strings.TrimSpace(ticket.QueueEntryID)
	promptID := strings.TrimSpace(ticket.PromptID)
	if queueEntryID == "" || promptID == "" {
		return looppkg.ActionPromptResult{}, fmt.Errorf(
			"%w: managed Goal prompt ticket is incomplete",
			looppkg.ErrValidation,
		)
	}
	ticker := time.NewTicker(managedGoalPromptPollInterval)
	defer ticker.Stop()
	for {
		entry, err := b.prompts.GetSessionInputQueueEntryByID(ctx, queueEntryID)
		if err != nil {
			return looppkg.ActionPromptResult{}, err
		}
		if entry.PromptID != promptID || entry.OwnerKind != loopManagedGoalKind {
			return looppkg.ActionPromptResult{}, fmt.Errorf(
				"%w: managed Goal prompt ticket identity changed",
				looppkg.ErrTransitionConflict,
			)
		}
		if managedGoalPromptSettled(entry) {
			result, resultErr := b.managedGoalPromptResult(ctx, entry)
			if resultErr == nil {
				b.usageReporters.remove(queueEntryID)
			}
			return result, resultErr
		}
		select {
		case <-ctx.Done():
			return looppkg.ActionPromptResult{}, fmt.Errorf("daemon: await managed Goal prompt: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (b *loopActionSessionBinder) CancelActionPrompts(
	ctx context.Context,
	owner looppkg.ActionPromptOwner,
) error {
	if b == nil || b.prompts == nil || b.managedInputs == nil {
		return fmt.Errorf("%w: managed Goal cancellation is unavailable", looppkg.ErrActionDependencyMissing)
	}
	run, err := b.prompts.GetLoopRunByID(ctx, owner.LoopRunID)
	if err != nil {
		return err
	}
	key := goalpkg.TurnKey{
		WorkspaceID: run.WorkspaceID,
		LoopRunID:   owner.LoopRunID,
		Generation:  owner.Generation,
		NodeID:      owner.NodeID,
		ItemIndex:   owner.ItemIndex,
	}
	checkpoint, err := b.prompts.LoadCheckpoint(ctx, key)
	if err != nil {
		return err
	}
	if checkpoint.TaskRunID != strings.TrimSpace(owner.TaskRunID) ||
		checkpoint.ControlEpoch != owner.ControlEpoch || checkpoint.BindingEpoch != owner.BindingEpoch ||
		checkpoint.PromptAttempt != owner.PromptAttempt {
		return fmt.Errorf("%w: managed Goal cancellation owner changed", looppkg.ErrTransitionConflict)
	}
	managedOwner := managedInputOwner(checkpoint, owner.Generation)
	b.managedInputs.CancelManagedInputLease(managedOwner)
	if err := b.managedInputs.WaitManagedInputLease(ctx, managedOwner); err != nil {
		return fmt.Errorf("daemon: drain managed Goal prompt: %w", err)
	}
	return nil
}

func managedGoalPromptKey(
	binding looppkg.ActionSessionBinding,
	req looppkg.ActionPromptRequest,
) (goalpkg.TurnKey, error) {
	key := goalpkg.TurnKey{
		WorkspaceID: binding.WorkspaceID,
		LoopRunID:   req.Owner.LoopRunID,
		Generation:  req.Owner.Generation,
		NodeID:      req.Owner.NodeID,
		ItemIndex:   req.Owner.ItemIndex,
	}
	if err := key.Validate(); err != nil {
		return goalpkg.TurnKey{}, err
	}
	if binding.LoopRunID != req.Owner.LoopRunID || binding.ControlEpoch != req.Owner.ControlEpoch ||
		binding.BindingEpoch != req.Owner.BindingEpoch || strings.TrimSpace(binding.SessionID) == "" ||
		strings.TrimSpace(binding.Handle) == "" || strings.TrimSpace(req.Owner.TaskRunID) == "" ||
		strings.TrimSpace(req.PromptID) == "" || strings.TrimSpace(req.Kind) == "" ||
		strings.TrimSpace(req.Message) == "" || req.Owner.Turn < 0 || req.Owner.PromptAttempt < 0 ||
		req.UsageBaseTokens < 0 || (!req.UsageBaseReported && req.UsageBaseTokens != 0) ||
		(req.ContextUsageSequence == nil) != (req.ContextUsageUsed == nil) ||
		(req.ContextUsageSequence != nil && (*req.ContextUsageSequence < 0 || *req.ContextUsageUsed < 0)) {
		return goalpkg.TurnKey{}, fmt.Errorf("%w: managed Goal prompt identity is incomplete", looppkg.ErrValidation)
	}
	return key, nil
}

func managedGoalQueueEntryID(key goalpkg.TurnKey, promptID string) string {
	material := fmt.Sprintf(
		"%s\x00%s\x00%d\x00%s\x00%d\x00%s",
		key.WorkspaceID,
		key.LoopRunID,
		key.Generation,
		key.NodeID,
		key.ItemIndex,
		strings.TrimSpace(promptID),
	)
	return "goalq_" + shortSHA256(material)
}

func (b *loopActionSessionBinder) promptNow() time.Time {
	if b != nil && b.now != nil {
		return b.now().UTC()
	}
	return time.Now().UTC()
}

func managedInputOwner(checkpoint goalpkg.Checkpoint, generation int) session.ManagedInputOwner {
	return session.ManagedInputOwner{
		QueueEntryID:  checkpoint.QueueEntryID,
		SessionID:     checkpoint.SessionID,
		OwnerKind:     loopManagedGoalKind,
		LoopRunID:     string(checkpoint.Key.LoopRunID),
		TaskRunID:     checkpoint.TaskRunID,
		RunGeneration: generation,
		PromptAttempt: checkpoint.PromptAttempt,
		ControlEpoch:  checkpoint.ControlEpoch,
		BindingEpoch:  checkpoint.BindingEpoch,
		PromptID:      checkpoint.PromptID,
		PromptKind:    checkpoint.PromptKind,
	}
}

func managedGoalPromptSettled(entry store.SessionInputQueueEntry) bool {
	return entry.TerminalAt != nil || strings.TrimSpace(entry.FenceKind) != ""
}

func (b *loopActionSessionBinder) managedGoalPromptResult(
	ctx context.Context,
	entry store.SessionInputQueueEntry,
) (looppkg.ActionPromptResult, error) {
	disposition := looppkg.ActionDisposition(firstNonEmptyManagedGoal(
		entry.TerminalDisposition,
		entry.FenceDisposition,
	))
	result := looppkg.ActionPromptResult{
		PromptID:         entry.PromptID,
		Outcome:          looppkg.ActionPromptOutcome(firstNonEmptyManagedGoal(entry.TerminalKind, entry.FenceKind)),
		StopReason:       looppkg.ActionStopReason(entry.TerminalStopReason),
		FenceDisposition: disposition,
		ReasonCode:       looppkg.ReasonCode(firstNonEmptyManagedGoal(entry.TerminalReasonCode, entry.FenceReasonCode)),
		TokensReported:   entry.TerminalTokensReported,
	}
	if entry.TerminalEventStartSeq != nil {
		result.EventStartSeq = *entry.TerminalEventStartSeq
	}
	if entry.TerminalEventEndSeq != nil {
		result.EventEndSeq = *entry.TerminalEventEndSeq
	}
	if entry.TerminalTokensUsed != nil {
		result.TokensUsed = *entry.TerminalTokensUsed
	}
	if result.EventEndSeq > 0 {
		reader, ok := b.sessions.(loopSessionEventReader)
		if !ok {
			return looppkg.ActionPromptResult{}, fmt.Errorf(
				"%w: managed Goal session event reader is unavailable",
				looppkg.ErrActionDependencyMissing,
			)
		}
		text, structured, err := readManagedGoalPromptOutput(ctx, reader, entry, result)
		if err != nil {
			return looppkg.ActionPromptResult{}, err
		}
		result.Text = text
		result.Structured = structured
	}
	return result, nil
}

func firstNonEmptyManagedGoal(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
