package loop

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/compozy/compozy/internal/loop/gate"
)

// GenerationLifecycleEventKind identifies a durable event emitted while a
// generation snapshot is finalized.
type GenerationLifecycleEventKind string

const (
	// GenerationLifecycleEventGenerationStarted records the start of a generation.
	GenerationLifecycleEventGenerationStarted GenerationLifecycleEventKind = "generation_started"
	// GenerationLifecycleEventGateVerdict records a sanitized gate verdict.
	GenerationLifecycleEventGateVerdict GenerationLifecycleEventKind = "gate_verdict"
)

// GenerationLifecycleEventIntent requests one durable generation lifecycle event.
type GenerationLifecycleEventIntent struct {
	Kind           GenerationLifecycleEventKind `json:"kind"`
	GateID         string                       `json:"gate_id,omitempty"`
	ItemIndex      int                          `json:"item_index,omitempty"`
	Route          gate.RouteAction             `json:"route,omitempty"`
	Reason         string                       `json:"reason,omitempty"`
	BestGeneration *int64                       `json:"best_generation,omitempty"`
}

func (i GenerationLifecycleEventIntent) normalized() GenerationLifecycleEventIntent {
	i.Kind = GenerationLifecycleEventKind(strings.TrimSpace(string(i.Kind)))
	i.GateID = strings.TrimSpace(i.GateID)
	i.Route = gate.RouteAction(strings.TrimSpace(string(i.Route)))
	i.Reason = strings.TrimSpace(i.Reason)
	if i.BestGeneration != nil {
		value := *i.BestGeneration
		i.BestGeneration = &value
	}
	return i
}

func (i GenerationLifecycleEventIntent) validate() error {
	switch i.Kind {
	case GenerationLifecycleEventGenerationStarted:
		if i.GateID != "" || i.ItemIndex != 0 || i.Route != "" {
			return fmt.Errorf("%w: generation_started event cannot name a gate instance", ErrValidation)
		}
	case GenerationLifecycleEventGateVerdict:
		if i.GateID == "" {
			return fmt.Errorf("%w: gate_verdict event gate_id is required", ErrValidation)
		}
		if !generationLifecycleRouteValid(i.Route) {
			return fmt.Errorf("%w: gate_verdict event route is invalid: %q", ErrValidation, i.Route)
		}
		if i.ItemIndex < 0 {
			return fmt.Errorf("%w: gate_verdict event item_index must be non-negative", ErrValidation)
		}
		if i.BestGeneration != nil && *i.BestGeneration < 1 {
			return fmt.Errorf("%w: gate_verdict event best_generation must be positive", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: generation lifecycle event kind is invalid: %q", ErrValidation, i.Kind)
	}
	return nil
}

func generationLifecycleRouteValid(route gate.RouteAction) bool {
	switch route {
	case gate.RouteContinue,
		gate.RouteRevise,
		gate.RouteBranch,
		gate.RouteHalt,
		gate.RouteEscalate,
		gate.RouteDone,
		gate.RouteNextGeneration:
		return true
	default:
		return false
	}
}

func normalizeGenerationSnapshotIntents(payload GenerationSnapshotPayload) (GenerationSnapshotPayload, error) {
	if payload.GenerationProvenance != nil {
		provenance := *payload.GenerationProvenance
		if err := provenance.Validate(); err != nil {
			return GenerationSnapshotPayload{}, err
		}
		payload.GenerationProvenance = &provenance
	}
	if payload.BestUpdate != nil {
		best := *payload.BestUpdate
		if best.Generation < 1 || math.IsNaN(best.Score) || math.IsInf(best.Score, 0) {
			return GenerationSnapshotPayload{}, fmt.Errorf("%w: generation best update is invalid", ErrValidation)
		}
		payload.BestUpdate = &best
	}
	if len(payload.Verdicts) > 0 {
		verdicts := make([]gate.VerdictIntent, len(payload.Verdicts))
		for index, verdict := range payload.Verdicts {
			normalized, err := normalizeGenerationVerdictIntent(verdict)
			if err != nil {
				return GenerationSnapshotPayload{}, err
			}
			verdicts[index] = normalized
		}
		payload.Verdicts = verdicts
	}
	if len(payload.Events) > 0 {
		events := make([]GenerationLifecycleEventIntent, len(payload.Events))
		for index, event := range payload.Events {
			normalized := event.normalized()
			if err := normalized.validate(); err != nil {
				return GenerationSnapshotPayload{}, err
			}
			events[index] = normalized
		}
		payload.Events = events
	}
	return payload, nil
}

func normalizeGenerationVerdictIntent(intent gate.VerdictIntent) (gate.VerdictIntent, error) {
	intent.GateID = strings.TrimSpace(intent.GateID)
	if intent.GateID == "" || !generationVerdictOutcomeValid(intent.Outcome) {
		return gate.VerdictIntent{}, fmt.Errorf("%w: generation gate verdict is invalid", ErrValidation)
	}
	if intent.ItemIndex < 0 {
		return gate.VerdictIntent{}, fmt.Errorf(
			"%w: generation gate verdict item_index must be non-negative",
			ErrValidation,
		)
	}
	if intent.Score != nil && (math.IsNaN(*intent.Score) || math.IsInf(*intent.Score, 0)) {
		return gate.VerdictIntent{}, fmt.Errorf("%w: generation gate verdict score must be finite", ErrValidation)
	}
	if intent.RouteCauseRank != nil && *intent.RouteCauseRank < 0 {
		return gate.VerdictIntent{}, fmt.Errorf(
			"%w: generation gate verdict route cause rank must be non-negative",
			ErrValidation,
		)
	}
	if !json.Valid(intent.BlockingIssues) || !json.Valid(intent.Criteria) {
		return gate.VerdictIntent{}, fmt.Errorf("%w: generation gate verdict diagnostics must be JSON", ErrValidation)
	}
	if intent.Score != nil {
		score := *intent.Score
		intent.Score = &score
	}
	if intent.RouteCauseRank != nil {
		rank := *intent.RouteCauseRank
		intent.RouteCauseRank = &rank
	}
	intent.BlockingIssues = append(json.RawMessage(nil), intent.BlockingIssues...)
	intent.Criteria = append(json.RawMessage(nil), intent.Criteria...)
	return intent, nil
}

func generationVerdictOutcomeValid(outcome gate.VerdictOutcome) bool {
	switch outcome {
	case gate.VerdictOutcomeApproved,
		gate.VerdictOutcomeRejected,
		gate.VerdictOutcomeAwaitingApproval,
		gate.VerdictOutcomeBlocked,
		gate.VerdictOutcomeError,
		gate.VerdictOutcomeTimeout,
		gate.VerdictOutcomeInvalidOutput:
		return true
	default:
		return false
	}
}
