package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func (s StubSessionManager) Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error) {
	if s.PromptFn != nil {
		return s.PromptFn(ctx, id, msg)
	}
	ch := make(chan acp.AgentEvent)
	close(ch)
	return ch, nil
}

func (s StubSessionManager) PromptWithOpts(
	ctx context.Context,
	id string,
	opts session.PromptOpts,
) (<-chan acp.AgentEvent, error) {
	if s.PromptWithOptsFn != nil {
		return s.PromptWithOptsFn(ctx, id, opts)
	}
	return s.Prompt(ctx, id, opts.Message)
}

func (s StubSessionManager) PromptSynthetic(
	ctx context.Context,
	id string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	if s.PromptSyntheticFn != nil {
		return s.PromptSyntheticFn(ctx, id, opts)
	}
	ch := make(chan acp.AgentEvent)
	close(ch)
	return ch, nil
}

func (s StubSessionManager) SendPrompt(
	ctx context.Context,
	id string,
	opts session.SendPromptOpts,
) (session.SendPromptResult, error) {
	if s.SendPromptFn != nil {
		return s.SendPromptFn(ctx, id, opts)
	}
	events, err := s.Prompt(ctx, id, opts.Message)
	if err != nil {
		return session.SendPromptResult{}, err
	}
	return session.SendPromptResult{Status: "accepted", Events: events}, nil
}

func (s StubSessionManager) SteerPrompt(
	ctx context.Context,
	id string,
	opts session.SteerPromptOpts,
) (session.SendPromptResult, error) {
	if s.SteerFn != nil {
		return s.SteerFn(ctx, id, opts)
	}
	return session.SendPromptResult{
		Status: "steering", Delivery: store.SessionInputDeliveryInterruptThenPrompt,
	}, nil
}

func (s StubSessionManager) CancelQueuedPrompt(
	ctx context.Context,
	id string,
	queueEntryID string,
) (session.SendPromptResult, error) {
	if s.CancelQueuedFn != nil {
		return s.CancelQueuedFn(ctx, id, queueEntryID)
	}
	return session.SendPromptResult{Status: "canceled", QueueEntryID: queueEntryID}, nil
}

func (s StubSessionManager) ListPendingInputs(
	ctx context.Context,
	id string,
) ([]session.PendingInput, error) {
	if s.ListPendingInputsFn != nil {
		return s.ListPendingInputsFn(ctx, id)
	}
	return []session.PendingInput{}, nil
}

func (s StubSessionManager) ReplacePendingInput(
	ctx context.Context,
	id string,
	entryID string,
	opts session.ReplacePendingInputOpts,
) (session.PendingInput, error) {
	if s.ReplacePendingInputFn != nil {
		return s.ReplacePendingInputFn(ctx, id, entryID, opts)
	}
	return session.PendingInput{}, session.ErrSessionNotFound
}

func (s StubSessionManager) PromotePendingInputToSteer(
	ctx context.Context,
	id string,
	entryID string,
	opts session.PromotePendingInputOpts,
) (session.SendPromptResult, error) {
	if s.PromotePendingInputFn != nil {
		return s.PromotePendingInputFn(ctx, id, entryID, opts)
	}
	return session.SendPromptResult{}, session.ErrSessionNotFound
}

func (s StubSessionManager) CancelPrompt(ctx context.Context, id string) (session.PromptCancelResult, error) {
	if s.CancelPromptFn != nil {
		return s.CancelPromptFn(ctx, id)
	}
	return session.PromptCancelResult{Outcome: session.PromptCancelOutcomeNothingInFlight}, nil
}

func (s StubSessionManager) ApprovePermission(
	ctx context.Context,
	id string,
	req acp.ApproveRequest,
) (session.ApprovalResult, error) {
	if s.ApproveFn != nil {
		return s.ApproveFn(ctx, id, req)
	}
	return session.ApprovalResult{}, nil
}

func (s StubSessionManager) InputQueueSummary(ctx context.Context, id string) (session.InputQueueSummary, error) {
	if s.InputQueueFn != nil {
		return s.InputQueueFn(ctx, id)
	}
	return session.InputQueueSummary{}, nil
}
