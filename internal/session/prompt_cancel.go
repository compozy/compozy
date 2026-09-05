package session

import (
	"context"
	"errors"
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
	run, err := m.requestTurnStop(ctx, targetSession.ID, "", CauseUserRequested)
	if errors.Is(err, ErrPromptNotInProgress) {
		return PromptCancelResult{Outcome: PromptCancelOutcomeNothingInFlight}, nil
	}
	if err != nil {
		return PromptCancelResult{}, err
	}
	return PromptCancelResult{Outcome: PromptCancelOutcomeCanceled, TurnID: run.turnID}, nil
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
