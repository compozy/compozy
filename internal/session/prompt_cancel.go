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

type PromptCancelCause string

const (
	PromptCancelUser               PromptCancelCause = "user"
	PromptCancelAgent              PromptCancelCause = "agent"
	PromptCancelSyntheticAdmission PromptCancelCause = "synthetic_admission"
	PromptCancelSteerFallback      PromptCancelCause = "steer_fallback"
)

// CancelPrompt cancels prompt setup/execution for a known session.
func (m *Manager) CancelPrompt(ctx context.Context, id string) (PromptCancelResult, error) {
	cause := PromptCancelUser
	if actingSessionID(ctx) != "" {
		cause = PromptCancelAgent
	}
	return m.CancelPromptWithCause(ctx, id, cause)
}

func (m *Manager) CancelPromptWithCause(
	ctx context.Context,
	id string,
	cause PromptCancelCause,
) (PromptCancelResult, error) {
	switch cause {
	case PromptCancelUser, PromptCancelAgent, PromptCancelSyntheticAdmission, PromptCancelSteerFallback:
	default:
		return PromptCancelResult{}, errors.New("session: invalid prompt cancellation cause")
	}
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
	// Finalization can retire a known session between lookup and the stop claim.
	run, err := m.requestTurnStop(ctx, targetSession.ID, "", CauseUserRequested)
	if errors.Is(err, ErrPromptNotInProgress) || errors.Is(err, ErrSessionNotActive) ||
		errors.Is(err, ErrSessionNotFound) {
		return PromptCancelResult{Outcome: PromptCancelOutcomeNothingInFlight}, nil
	}
	if err != nil {
		return PromptCancelResult{}, err
	}
	if cause == PromptCancelUser || cause == PromptCancelAgent {
		if service := m.subagentService(); service != nil {
			if err := service.OnParentTurnInterrupted(ctx, target, run.turnID); err != nil {
				return PromptCancelResult{}, err
			}
		}
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
