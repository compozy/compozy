package loop

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
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
	// GenerationLifecycleEventNodeRetryScheduled records one durable retry schedule.
	GenerationLifecycleEventNodeRetryScheduled GenerationLifecycleEventKind = "node_retry_scheduled"
	// GenerationLifecycleEventNodeSucceeded records an effect-bearing successful node outcome.
	GenerationLifecycleEventNodeSucceeded GenerationLifecycleEventKind = "node_succeeded"
	// GenerationLifecycleEventNodeFailed records an effect-bearing handled or terminal node failure.
	GenerationLifecycleEventNodeFailed GenerationLifecycleEventKind = "node_failed"
	// GenerationLifecycleEventNodeCanceled records one cooperatively drained node attempt.
	GenerationLifecycleEventNodeCanceled GenerationLifecycleEventKind = "node_canceled"
	// GenerationLifecycleEventNodeQuarantined records one durable quarantine episode.
	GenerationLifecycleEventNodeQuarantined GenerationLifecycleEventKind = "node_quarantined"
	// GenerationLifecycleEventNodeAttentionFlagged records one dependency-parked attention flag.
	GenerationLifecycleEventNodeAttentionFlagged GenerationLifecycleEventKind = "node_attention_flagged"
	// GenerationLifecycleEventNodePaused records one automatic pause episode.
	GenerationLifecycleEventNodePaused GenerationLifecycleEventKind = "node_paused"
	// GenerationLifecycleEventNodeWaitStarted records one durable wait entry.
	GenerationLifecycleEventNodeWaitStarted GenerationLifecycleEventKind = "node_wait_started"
	// GenerationLifecycleEventNodeWaitResumed records an ahead-arrival wait resolution.
	GenerationLifecycleEventNodeWaitResumed GenerationLifecycleEventKind = "node_wait_resumed"
	// GenerationLifecycleEventTargetBreakerTransition records a loop-target breaker state change.
	GenerationLifecycleEventTargetBreakerTransition GenerationLifecycleEventKind = "target_breaker_transition"
	// GenerationLifecycleEventPredicateDiagnostic records predicate cost or evaluation diagnostics.
	GenerationLifecycleEventPredicateDiagnostic GenerationLifecycleEventKind = "predicate_diagnostic"
	// GenerationLifecycleEventRouteTaken records one exclusive routing decision.
	GenerationLifecycleEventRouteTaken GenerationLifecycleEventKind = "route_taken"
	// GenerationLifecycleEventBranchPruned records one bounded strategy-cancel aggregate.
	GenerationLifecycleEventBranchPruned GenerationLifecycleEventKind = "branch_pruned"
)

// GenerationLifecycleEventIntent requests one durable generation lifecycle event.
type GenerationLifecycleEventIntent struct {
	Kind            GenerationLifecycleEventKind `json:"kind"`
	GateID          string                       `json:"gate_id,omitempty"`
	ItemIndex       int                          `json:"item_index,omitempty"`
	Route           gate.RouteAction             `json:"route,omitempty"`
	Reason          string                       `json:"reason,omitempty"`
	BestGeneration  *int64                       `json:"best_generation,omitempty"`
	NodeID          string                       `json:"node_id,omitempty"`
	Attempt         int                          `json:"attempt,omitempty"`
	IssuedEpoch     int64                        `json:"issued_epoch,omitempty"`
	NextAttemptAt   *time.Time                   `json:"next_attempt_at,omitempty"`
	FailureClass    FailureClass                 `json:"failure_class,omitempty"`
	Failure         *ClassifiedFailure           `json:"failure,omitempty"`
	Disposition     AttemptDisposition           `json:"disposition,omitempty"`
	QuarantineEntry json.RawMessage              `json:"quarantine_entry,omitempty"`
	AttentionFlag   string                       `json:"attention_flag,omitempty"`
	// AttentionProducerNodeID names the parked producer behind a dependency attention flag.
	AttentionProducerNodeID string                 `json:"attention_producer_node_id,omitempty"`
	TargetFamily            string                 `json:"target_family,omitempty"`
	Target                  string                 `json:"target,omitempty"`
	BreakerState            string                 `json:"breaker_state,omitempty"`
	ActorKind               string                 `json:"actor_kind,omitempty"`
	ActorID                 string                 `json:"actor_id,omitempty"`
	RuleID                  string                 `json:"rule_id,omitempty"`
	WaitKind                string                 `json:"wait_kind,omitempty"`
	AheadArrival            string                 `json:"ahead_arrival,omitempty"`
	AheadCursors            map[string]int64       `json:"ahead_cursors,omitempty"`
	Effects                 []RenderedEffectIntent `json:"effects,omitempty"`
	Predicate               string                 `json:"predicate,omitempty"`
	DiagnosticCode          string                 `json:"diagnostic_code,omitempty"`
	Cost                    uint64                 `json:"cost,omitempty"`
	CostLimit               uint64                 `json:"cost_limit,omitempty"`
	Warning                 bool                   `json:"warning,omitempty"`
	SelectedRoute           string                 `json:"selected_route,omitempty"`
	MatchedWhen             string                 `json:"matched_when,omitempty"`
	DefaultRoute            bool                   `json:"default_route,omitempty"`
	ItemIndexes             []int                  `json:"item_indexes,omitempty"`
}

func (i GenerationLifecycleEventIntent) normalized() GenerationLifecycleEventIntent {
	i.Kind = GenerationLifecycleEventKind(strings.TrimSpace(string(i.Kind)))
	i.GateID = strings.TrimSpace(i.GateID)
	i.Route = gate.RouteAction(strings.TrimSpace(string(i.Route)))
	i.Reason = strings.TrimSpace(i.Reason)
	i.NodeID = strings.TrimSpace(i.NodeID)
	i.AttentionFlag = strings.TrimSpace(i.AttentionFlag)
	i.AttentionProducerNodeID = strings.TrimSpace(i.AttentionProducerNodeID)
	i.TargetFamily = strings.TrimSpace(i.TargetFamily)
	i.Target = strings.TrimSpace(i.Target)
	i.BreakerState = strings.TrimSpace(i.BreakerState)
	i.ActorKind = strings.TrimSpace(i.ActorKind)
	i.ActorID = strings.TrimSpace(i.ActorID)
	i.RuleID = strings.TrimSpace(i.RuleID)
	i.WaitKind = strings.TrimSpace(i.WaitKind)
	i.AheadArrival = strings.TrimSpace(i.AheadArrival)
	i.Predicate = strings.TrimSpace(i.Predicate)
	i.DiagnosticCode = strings.TrimSpace(i.DiagnosticCode)
	i.SelectedRoute = strings.TrimSpace(i.SelectedRoute)
	i.MatchedWhen = strings.TrimSpace(i.MatchedWhen)
	i.ItemIndexes = append([]int(nil), i.ItemIndexes...)
	i.AheadCursors = cloneInt64Map(i.AheadCursors)
	if len(i.QuarantineEntry) > 0 {
		i.QuarantineEntry = append(json.RawMessage(nil), i.QuarantineEntry...)
	}
	if i.NextAttemptAt != nil {
		value := i.NextAttemptAt.UTC()
		i.NextAttemptAt = &value
	}
	if i.BestGeneration != nil {
		value := *i.BestGeneration
		i.BestGeneration = &value
	}
	if i.Failure != nil {
		failure := *i.Failure
		i.Failure = &failure
	}
	if len(i.Effects) > 0 {
		i.Effects = append([]RenderedEffectIntent(nil), i.Effects...)
	}
	return i
}

func (i GenerationLifecycleEventIntent) validate() error {
	var err error
	switch i.Kind {
	case GenerationLifecycleEventGenerationStarted:
		err = i.validateGenerationStarted()
	case GenerationLifecycleEventGateVerdict:
		err = i.validateGateVerdict()
	case GenerationLifecycleEventNodeRetryScheduled:
		err = i.validateNodeRetryScheduled()
	case GenerationLifecycleEventNodeSucceeded:
		err = i.validateNodeSucceeded()
	case GenerationLifecycleEventNodeFailed:
		err = i.validateNodeFailed()
	case GenerationLifecycleEventNodeCanceled:
		err = i.validateNodeCanceled()
	case GenerationLifecycleEventNodeQuarantined:
		err = i.validateNodeQuarantined()
	case GenerationLifecycleEventNodeAttentionFlagged:
		err = i.validateNodeAttentionFlagged()
	case GenerationLifecycleEventNodePaused:
		err = i.validateNodePaused()
	case GenerationLifecycleEventNodeWaitStarted:
		err = i.validateNodeWaitStarted()
	case GenerationLifecycleEventNodeWaitResumed:
		err = i.validateNodeWaitResumed()
	case GenerationLifecycleEventTargetBreakerTransition:
		err = i.validateTargetBreakerTransition()
	case GenerationLifecycleEventPredicateDiagnostic:
		err = i.validatePredicateDiagnostic()
	case GenerationLifecycleEventRouteTaken:
		err = i.validateRouteTaken()
	case GenerationLifecycleEventBranchPruned:
		err = i.validateBranchPruned()
	default:
		return fmt.Errorf("%w: generation lifecycle event kind is invalid: %q", ErrValidation, i.Kind)
	}
	if err != nil {
		return err
	}
	for _, effect := range i.Effects {
		if err := effect.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (i GenerationLifecycleEventIntent) validateBranchPruned() error {
	if i.NodeID == "" || i.Reason == "" || len(i.ItemIndexes) == 0 {
		return fmt.Errorf("%w: branch_pruned event has incomplete strategy identity", ErrValidation)
	}
	for _, itemIndex := range i.ItemIndexes {
		if itemIndex < 0 {
			return fmt.Errorf("%w: branch_pruned item index is invalid", ErrValidation)
		}
	}
	return nil
}

func (i GenerationLifecycleEventIntent) validateRouteTaken() error {
	if i.NodeID == "" || i.ItemIndex < 0 || i.SelectedRoute == "" || i.Reason == "" {
		return fmt.Errorf("%w: route_taken event has incomplete routing identity", ErrValidation)
	}
	if i.DefaultRoute && i.MatchedWhen != "" {
		return fmt.Errorf("%w: route_taken default cannot carry matched_when", ErrValidation)
	}
	return nil
}

func (i GenerationLifecycleEventIntent) validatePredicateDiagnostic() error {
	if i.Predicate == "" || i.DiagnosticCode == "" {
		return fmt.Errorf("%w: predicate diagnostic identity is incomplete", ErrValidation)
	}
	if i.CostLimit > 0 && i.Cost > i.CostLimit && i.Warning {
		return fmt.Errorf("%w: predicate warning cost exceeds the configured limit", ErrValidation)
	}
	return nil
}

func (i GenerationLifecycleEventIntent) validateNodeWaitResumed() error {
	if err := i.validateNodeWaitStarted(); err != nil {
		return err
	}
	if i.ActorKind != "" && i.ActorID != "" {
		return nil
	}
	return fmt.Errorf("%w: node_wait_resumed event has incomplete provenance", ErrValidation)
}

func (i GenerationLifecycleEventIntent) validateNodeWaitStarted() error {
	if i.AheadArrival != "" && i.AheadArrival != string(dsl.WaitAheadConsumeOnEntry) &&
		i.AheadArrival != string(dsl.WaitAheadReject) {
		return fmt.Errorf("%w: node_wait_started ahead_arrival is invalid", ErrValidation)
	}
	for stream, cursor := range i.AheadCursors {
		if strings.TrimSpace(stream) == "" || cursor < 0 {
			return fmt.Errorf("%w: node_wait_started ahead cursor is invalid", ErrValidation)
		}
	}
	if i.NodeID != "" && i.ItemIndex >= 0 && i.Attempt >= 1 && i.IssuedEpoch >= 1 &&
		(i.WaitKind == NodeWaitKindTimer || i.WaitKind == NodeWaitKindEvent ||
			i.WaitKind == NodeWaitKindApprovalEscalation || i.WaitKind == NodeWaitKindRequest) {
		return nil
	}
	return fmt.Errorf("%w: node_wait_started event has incomplete wait identity", ErrValidation)
}

func (i GenerationLifecycleEventIntent) validateNodeAttentionFlagged() error {
	if i.NodeID != "" && i.ItemIndex >= 0 && i.AttentionFlag != "" &&
		i.AttentionProducerNodeID != "" && i.Reason != "" {
		return nil
	}
	return fmt.Errorf("%w: node_attention_flagged event has incomplete provenance", ErrValidation)
}

func (i GenerationLifecycleEventIntent) validateNodePaused() error {
	if i.NodeID != "" && i.ItemIndex >= 0 && i.Attempt >= 1 && i.ActorKind != "" &&
		i.ActorID != "" && i.RuleID != "" && i.Reason != "" {
		return nil
	}
	return fmt.Errorf("%w: node_paused event has incomplete provenance", ErrValidation)
}

func (i GenerationLifecycleEventIntent) validateGenerationStarted() error {
	if i.GateID == "" && i.ItemIndex == 0 && i.Route == "" {
		return nil
	}
	return fmt.Errorf(
		"%w: generation_started event cannot carry gate_id, item_index, or route",
		ErrValidation,
	)
}

func (i GenerationLifecycleEventIntent) validateGateVerdict() error {
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
	return nil
}

func (i GenerationLifecycleEventIntent) validateNodeRetryScheduled() error {
	if i.NodeID != "" && i.ItemIndex >= 0 && i.Attempt >= 1 && i.IssuedEpoch >= 1 &&
		i.NextAttemptAt != nil && IsKnownFailureClass(i.FailureClass) {
		return nil
	}
	return fmt.Errorf("%w: node_retry_scheduled event has incomplete retry identity", ErrValidation)
}

func (i GenerationLifecycleEventIntent) validateNodeSucceeded() error {
	if i.NodeID != "" && i.ItemIndex >= 0 && i.Attempt >= 1 && len(i.Effects) > 0 {
		return nil
	}
	return fmt.Errorf("%w: node_succeeded effect event has incomplete identity", ErrValidation)
}

func (i GenerationLifecycleEventIntent) validateNodeFailed() error {
	if i.NodeID != "" && i.ItemIndex >= 0 && i.Attempt >= 1 && i.Failure != nil &&
		i.Disposition != "" && len(i.Effects) > 0 {
		return nil
	}
	return fmt.Errorf("%w: node_failed effect event has incomplete failure identity", ErrValidation)
}

func (i GenerationLifecycleEventIntent) validateNodeCanceled() error {
	if i.NodeID != "" && i.ItemIndex >= 0 && i.Attempt >= 1 && i.Failure != nil &&
		i.Disposition == AttemptCanceled {
		return nil
	}
	return fmt.Errorf("%w: node_canceled event has incomplete cancellation identity", ErrValidation)
}

func (i GenerationLifecycleEventIntent) validateNodeQuarantined() error {
	if i.NodeID != "" && i.ItemIndex >= 0 && i.Attempt >= 1 && i.Failure != nil &&
		i.Disposition == AttemptQuarantined && len(i.QuarantineEntry) > 0 && json.Valid(i.QuarantineEntry) {
		return nil
	}
	return fmt.Errorf("%w: node_quarantined event has incomplete quarantine identity", ErrValidation)
}

func (i GenerationLifecycleEventIntent) validateTargetBreakerTransition() error {
	stateValid := i.BreakerState == targetBreakerStateOpen ||
		i.BreakerState == targetBreakerStateHalfOpen ||
		i.BreakerState == targetBreakerStateClosed
	if i.TargetFamily != "" && i.Target != "" && stateValid {
		return nil
	}
	return fmt.Errorf("%w: target_breaker_transition event is incomplete", ErrValidation)
}

func generationLifecycleRouteValid(route gate.RouteAction) bool {
	switch route {
	case gate.RouteContinue,
		gate.RouteRevise,
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
	waits, err := normalizeNodeWaitIntents(payload.Waits)
	if err != nil {
		return GenerationSnapshotPayload{}, err
	}
	payload.Waits = waits
	requests := make([]RequestIntent, len(payload.Requests))
	for index, request := range payload.Requests {
		normalized := request.normalized()
		if err := normalized.validate(); err != nil {
			return GenerationSnapshotPayload{}, err
		}
		requests[index] = normalized
	}
	payload.Requests = requests
	payload.StrategyCancellations, err = normalizeStrategyCancellationIntents(payload.StrategyCancellations)
	if err != nil {
		return GenerationSnapshotPayload{}, err
	}
	controls, err := normalizeNodeControlMutations(payload.Controls)
	if err != nil {
		return GenerationSnapshotPayload{}, err
	}
	payload.Controls = controls
	if err := normalizeGenerationSnapshotMetadata(&payload); err != nil {
		return GenerationSnapshotPayload{}, err
	}
	payload.Verdicts, err = normalizeGenerationVerdictIntents(payload.Verdicts)
	if err != nil {
		return GenerationSnapshotPayload{}, err
	}
	payload.Events, err = normalizeGenerationLifecycleEventIntents(payload.Events)
	if err != nil {
		return GenerationSnapshotPayload{}, err
	}
	payload.BoundaryEffects, err = normalizeBoundaryEffectIntents(payload.BoundaryEffects)
	if err != nil {
		return GenerationSnapshotPayload{}, err
	}
	return payload, nil
}

func normalizeNodeControlMutations(controls []NodeControlMutation) ([]NodeControlMutation, error) {
	if len(controls) == 0 {
		return controls, nil
	}
	normalizedControls := make([]NodeControlMutation, len(controls))
	for index, control := range controls {
		normalized := control.normalized()
		if err := normalized.validate(); err != nil {
			return nil, err
		}
		normalizedControls[index] = normalized
	}
	return normalizedControls, nil
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
