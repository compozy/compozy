package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/transcript"
)

// PromptCancelOutcome is the stable result of one idempotent cancellation request.
type PromptCancelOutcome string

const (
	PromptCancelOutcomeCanceled        PromptCancelOutcome = "canceled"
	PromptCancelOutcomeNothingInFlight PromptCancelOutcome = "nothing-in-flight"
)

// PromptCancelResult identifies the canceled turn when one was active.
type PromptCancelResult struct {
	Outcome PromptCancelOutcome
	TurnID  string
}

// CancelPrompt cancels prompt setup/execution for a known session.
func (m *Manager) CancelPrompt(ctx context.Context, id string) (PromptCancelResult, error) {
	if m == nil {
		return PromptCancelResult{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return PromptCancelResult{}, errors.New("session: cancel prompt context is required")
	}

	target := strings.TrimSpace(id)
	if target == "" {
		return PromptCancelResult{}, errors.New("session: session id is required")
	}

	targetSession, ok := m.Get(target)
	if !ok {
		if _, err := m.readMetaWithContext(ctx, target); err != nil {
			return PromptCancelResult{}, err
		}
		return PromptCancelResult{Outcome: PromptCancelOutcomeNothingInFlight}, nil
	}
	turnID, proc, cancelPrompt, prompting, alreadyRequested := targetSession.requestCurrentPromptCancellation()
	if !prompting {
		return PromptCancelResult{Outcome: PromptCancelOutcomeNothingInFlight}, nil
	}
	result := PromptCancelResult{Outcome: PromptCancelOutcomeCanceled, TurnID: turnID}
	if alreadyRequested {
		return result, nil
	}
	if cancelPrompt != nil {
		cancelPrompt()
	}

	if proc == nil {
		m.emitPromptCancelMarker(ctx, targetSession, turnID)
		return result, nil
	}

	cancelErr := m.driver.Cancel(ctx, proc)
	if cancelErr != nil {
		if isProcessDone(proc) {
			return result, nil
		}
		targetSession.retryCurrentPromptCancellation(turnID)
		return PromptCancelResult{}, fmt.Errorf("session: cancel prompt for %q: %w", target, cancelErr)
	}
	if scoped, ok := m.driver.(ScopedInterrupter); ok && strings.TrimSpace(turnID) != "" {
		if _, err := scoped.Interrupt(ctx, target, turnID); err != nil &&
			!errors.Is(err, ErrScopedInterruptNotFound) {
			targetSession.retryCurrentPromptCancellation(turnID)
			return PromptCancelResult{}, fmt.Errorf("session: interrupt scoped tools for %q: %w", target, err)
		}
	}
	m.emitPromptCancelMarker(ctx, targetSession, turnID)
	return result, nil
}

func (m *Manager) emitPromptCancelMarker(ctx context.Context, target *Session, turnID string) {
	evidence := map[string]any{transcriptMarkerEvidenceSourceKey: "cancel_prompt"}
	if actorID := actingSessionID(ctx); actorID != "" {
		evidence["actor_kind"] = actingSessionActorKind
		evidence["actor_id"] = actorID
	}
	m.emitTranscriptMarker(
		ctx,
		target,
		turnID,
		transcript.MarkerPromptCancel,
		"Prompt canceled by operator.",
		evidence,
	)
}
