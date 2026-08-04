package loop

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/compozy/compozy/internal/loop/gate"
)

func normalizeGenerationSnapshotMetadata(payload *GenerationSnapshotPayload) error {
	if payload.GenerationProvenance != nil {
		provenance := *payload.GenerationProvenance
		if err := provenance.Validate(); err != nil {
			return err
		}
		payload.GenerationProvenance = &provenance
	}
	if payload.BestUpdate == nil {
		return nil
	}
	best := *payload.BestUpdate
	if best.Generation < 1 || math.IsNaN(best.Score) || math.IsInf(best.Score, 0) {
		return fmt.Errorf("%w: generation best update is invalid", ErrValidation)
	}
	payload.BestUpdate = &best
	return nil
}

func normalizeGenerationVerdictIntents(
	verdicts []gate.VerdictIntent,
) ([]gate.VerdictIntent, error) {
	if len(verdicts) == 0 {
		return verdicts, nil
	}
	normalizedVerdicts := make([]gate.VerdictIntent, len(verdicts))
	for index, verdict := range verdicts {
		normalized, err := normalizeGenerationVerdictIntent(verdict)
		if err != nil {
			return nil, err
		}
		normalizedVerdicts[index] = normalized
	}
	return normalizedVerdicts, nil
}

func normalizeGenerationLifecycleEventIntents(
	events []GenerationLifecycleEventIntent,
) ([]GenerationLifecycleEventIntent, error) {
	if len(events) == 0 {
		return events, nil
	}
	normalizedEvents := make([]GenerationLifecycleEventIntent, len(events))
	for index, event := range events {
		normalized := event.normalized()
		if err := normalized.validate(); err != nil {
			return nil, err
		}
		normalizedEvents[index] = normalized
	}
	return normalizedEvents, nil
}

func normalizeBoundaryEffectIntents(
	boundaryEffects map[Status][]RenderedEffectIntent,
) (map[Status][]RenderedEffectIntent, error) {
	if len(boundaryEffects) == 0 {
		return boundaryEffects, nil
	}
	normalizedEffects := make(map[Status][]RenderedEffectIntent, len(boundaryEffects))
	for status, effects := range boundaryEffects {
		if !status.Terminal() && status != StatusNeedsApproval {
			return nil, fmt.Errorf(
				"%w: effect boundary status is not terminal or approval-gated: %q",
				ErrValidation,
				status,
			)
		}
		cloned := make([]RenderedEffectIntent, len(effects))
		for index, effect := range effects {
			if err := effect.Validate(); err != nil {
				return nil, err
			}
			effect.Entry = append(json.RawMessage(nil), effect.Entry...)
			cloned[index] = effect
		}
		normalizedEffects[status] = cloned
	}
	return normalizedEffects, nil
}
