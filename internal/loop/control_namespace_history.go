package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/compozy/compozy/internal/loop/gate"
)

// GenerationHistory is the read-only generation-boundary context for a namespace.
type GenerationHistory struct {
	Previous *PreviousGeneration
	Best     *BestGeneration
}

// PreviousGeneration projects the immediately preceding generation.
type PreviousGeneration struct {
	Generation  int64
	Nodes       map[string]map[int]NodeProjection
	Verdicts    map[string]map[int]VerdictProjection
	RouteCauses []GateInstanceProjection
}

// GateInstanceProjection identifies one materialized gate in a fan-out-aware history.
type GateInstanceProjection struct {
	GateID    string
	ItemIndex int
}

// NodeProjection is one previous-generation node's observable state.
type NodeProjection struct {
	Status      string
	Output      any
	Failure     *ClassifiedFailure
	Disposition AttemptDisposition
}

// VerdictProjection is one persisted machine gate verdict.
type VerdictProjection struct {
	Outcome        gate.VerdictOutcome
	Score          *float64
	BlockingIssues any
	Criteria       any
}

// BestNodeProjection exposes only the best generation node output.
type BestNodeProjection struct {
	Output any
}

// BestGeneration projects the run's ratchet baseline.
type BestGeneration struct {
	Generation int64
	Score      float64
	Nodes      map[string]map[int]BestNodeProjection
}

// GenerationHistoryReader provides all durable records needed for history projection.
type GenerationHistoryReader interface {
	GenerationOutputReader
	gate.VerdictReader
}

// ReadGenerationHistory loads the previous and best generation projections for one namespace.
func ReadGenerationHistory(
	ctx context.Context,
	reader GenerationHistoryReader,
	run Run,
	generation int,
) (GenerationHistory, error) {
	if generation < 1 {
		return GenerationHistory{}, fmt.Errorf("%w: generation must be positive", ErrValidation)
	}
	if generation == 1 {
		return GenerationHistory{}, nil
	}
	if reader == nil {
		return GenerationHistory{}, fmt.Errorf("%w: generation history reader is required", ErrValidation)
	}

	previousGeneration := int64(generation - 1)
	previousOutputs, err := reader.ListGenerationOutputs(
		ctx,
		run.WorkspaceID,
		run.ID,
		int(previousGeneration),
	)
	if err != nil {
		return GenerationHistory{}, fmt.Errorf("read previous generation outputs: %w", err)
	}
	previousOutputs, err = generationOutputRuntimeView(ctx, reader, generationOutputRuntimeScope{
		workspaceID: run.WorkspaceID,
		runID:       run.ID,
		generation:  int(previousGeneration),
	}, previousOutputs)
	if err != nil {
		return GenerationHistory{}, fmt.Errorf("read previous generation output payloads: %w", err)
	}
	previousVerdicts, err := reader.ListGateVerdicts(
		ctx,
		string(run.WorkspaceID),
		string(run.ID),
		previousGeneration,
	)
	if err != nil {
		return GenerationHistory{}, fmt.Errorf("read previous generation verdicts: %w", err)
	}
	routeCauses, err := reader.ListRouteCausingVerdicts(
		ctx,
		string(run.WorkspaceID),
		string(run.ID),
		previousGeneration,
	)
	if err != nil {
		return GenerationHistory{}, fmt.Errorf("read previous generation route causes: %w", err)
	}
	attempts, err := readGenerationAttempts(ctx, reader, run)
	if err != nil {
		return GenerationHistory{}, err
	}
	bestOutputs, err := readBestGenerationOutputs(ctx, reader, run, previousGeneration, previousOutputs)
	if err != nil {
		return GenerationHistory{}, err
	}

	return projectGenerationHistoryWithAttempts(
		generation,
		run,
		previousOutputs,
		previousVerdicts,
		routeCauses,
		bestOutputs,
		attempts,
	)
}

func readGenerationAttempts(
	ctx context.Context,
	reader GenerationHistoryReader,
	run Run,
) ([]NodeAttempt, error) {
	attemptReader, ok := reader.(NodeAttemptReader)
	if !ok {
		return nil, nil
	}
	attempts, err := attemptReader.ListNodeAttempts(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, fmt.Errorf("read previous generation attempts: %w", err)
	}
	return attempts, nil
}

func readBestGenerationOutputs(
	ctx context.Context,
	reader GenerationHistoryReader,
	run Run,
	previousGeneration int64,
	previousOutputs []GenerationOutput,
) ([]GenerationOutput, error) {
	if run.BestGeneration == nil {
		return nil, nil
	}
	bestGeneration, err := bestGenerationValue(run)
	if err != nil {
		return nil, err
	}
	if bestGeneration == previousGeneration {
		return previousOutputs, nil
	}
	outputs, err := reader.ListGenerationOutputs(ctx, run.WorkspaceID, run.ID, int(bestGeneration))
	if err != nil {
		return nil, fmt.Errorf("read best generation outputs: %w", err)
	}
	outputs, err = generationOutputRuntimeView(ctx, reader, generationOutputRuntimeScope{
		workspaceID: run.WorkspaceID,
		runID:       run.ID,
		generation:  int(bestGeneration),
	}, outputs)
	if err != nil {
		return nil, fmt.Errorf("read best generation output payloads: %w", err)
	}
	return outputs, nil
}

// ProjectGenerationHistory creates a generation history from reader records without IO.
func ProjectGenerationHistory(
	generation int,
	run Run,
	previousOutputs []GenerationOutput,
	previousVerdicts []gate.VerdictRecord,
	routeCauses []gate.VerdictRecord,
	bestOutputs []GenerationOutput,
) (GenerationHistory, error) {
	return projectGenerationHistoryWithAttempts(
		generation, run, previousOutputs, previousVerdicts, routeCauses, bestOutputs, nil,
	)
}

func projectGenerationHistoryWithAttempts(
	generation int,
	run Run,
	previousOutputs []GenerationOutput,
	previousVerdicts []gate.VerdictRecord,
	routeCauses []gate.VerdictRecord,
	bestOutputs []GenerationOutput,
	attempts []NodeAttempt,
) (GenerationHistory, error) {
	if generation < 1 {
		return GenerationHistory{}, fmt.Errorf("%w: generation must be positive", ErrValidation)
	}
	if generation == 1 {
		return GenerationHistory{}, nil
	}

	previous, err := projectPreviousGeneration(
		int64(generation-1), previousOutputs, previousVerdicts, routeCauses, attempts,
	)
	if err != nil {
		return GenerationHistory{}, err
	}
	history := GenerationHistory{Previous: &previous}
	if run.BestGeneration == nil {
		return history, nil
	}
	best, err := projectBestGeneration(run, bestOutputs)
	if err != nil {
		return GenerationHistory{}, err
	}
	history.Best = &best
	return history, nil
}

func projectPreviousGeneration(
	generation int64,
	outputs []GenerationOutput,
	verdicts []gate.VerdictRecord,
	routeCauses []gate.VerdictRecord,
	attempts []NodeAttempt,
) (PreviousGeneration, error) {
	nodes, err := projectPreviousNodes(generation, outputs, attempts)
	if err != nil {
		return PreviousGeneration{}, err
	}
	projectedVerdicts, err := projectVerdicts(verdicts)
	if err != nil {
		return PreviousGeneration{}, err
	}
	causes, err := projectRouteCauses(routeCauses)
	if err != nil {
		return PreviousGeneration{}, err
	}
	return PreviousGeneration{
		Generation:  generation,
		Nodes:       nodes,
		Verdicts:    projectedVerdicts,
		RouteCauses: causes,
	}, nil
}

func projectPreviousNodes(
	generation int64,
	outputs []GenerationOutput,
	attempts []NodeAttempt,
) (map[string]map[int]NodeProjection, error) {
	failures := previousGenerationFailures(generation, attempts)
	nodes := make(map[string]map[int]NodeProjection, len(outputs))
	for _, output := range outputs {
		nodeID := strings.TrimSpace(output.NodeID)
		if nodeID == "" {
			return nil, fmt.Errorf("%w: generation output node_id is required", ErrValidation)
		}
		if output.ItemIndex < 0 {
			return nil, fmt.Errorf("%w: generation output item_index must be non-negative", ErrValidation)
		}
		if nodes[nodeID] == nil {
			nodes[nodeID] = make(map[int]NodeProjection)
		}
		projection := NodeProjection{
			Status: output.Status,
			Output: generationOutputRuntimeValue(output),
		}
		if attempt, ok := failures[generationOutputKey{nodeID: nodeID, itemIndex: output.ItemIndex}]; ok {
			failure := classifiedFailureFromAttempt(attempt)
			projection.Failure = &failure
			projection.Disposition = attempt.Disposition
		}
		nodes[nodeID][output.ItemIndex] = projection
	}
	return nodes, nil
}

func previousGenerationFailures(
	generation int64,
	attempts []NodeAttempt,
) map[generationOutputKey]NodeAttempt {
	failures := make(map[generationOutputKey]NodeAttempt)
	for _, attempt := range attempts {
		if int64(attempt.Generation) != generation || attempt.Disposition != AttemptEscalated ||
			attempt.FailureClass == nil {
			continue
		}
		key := generationOutputKey{nodeID: string(attempt.NodeID), itemIndex: attempt.ItemIndex}
		if current, ok := failures[key]; !ok || attempt.Attempt > current.Attempt {
			failures[key] = attempt
		}
	}
	return failures
}

func classifiedFailureFromAttempt(attempt NodeAttempt) ClassifiedFailure {
	failure := ClassifiedFailure{
		Code: attempt.FailureCode, Cause: attempt.Cause, Hint: attempt.Hint, Target: attempt.Target,
	}
	if attempt.FailureClass != nil {
		failure.Class = *attempt.FailureClass
		failure.RetryEligible = attempt.FailureClass.RetryEligible()
	}
	return failure
}

func projectVerdicts(records []gate.VerdictRecord) (map[string]map[int]VerdictProjection, error) {
	verdicts := make(map[string]map[int]VerdictProjection, len(records))
	for _, record := range records {
		gateID := strings.TrimSpace(record.GateID)
		if gateID == "" {
			return nil, fmt.Errorf("%w: gate verdict gate_id is required", ErrValidation)
		}
		blockingIssues, err := historyJSONValue(record.BlockingIssues)
		if err != nil {
			return nil, fmt.Errorf("project verdict %q blocking issues: %w", gateID, err)
		}
		criteria, err := historyJSONValue(record.Criteria)
		if err != nil {
			return nil, fmt.Errorf("project verdict %q criteria: %w", gateID, err)
		}
		if record.ItemIndex < 0 {
			return nil, fmt.Errorf("%w: gate verdict item_index must be non-negative", ErrValidation)
		}
		if verdicts[gateID] == nil {
			verdicts[gateID] = make(map[int]VerdictProjection)
		}
		verdicts[gateID][record.ItemIndex] = VerdictProjection{
			Outcome:        record.Outcome,
			Score:          cloneFloat64(record.Score),
			BlockingIssues: blockingIssues,
			Criteria:       criteria,
		}
	}
	return verdicts, nil
}

func projectRouteCauses(records []gate.VerdictRecord) ([]GateInstanceProjection, error) {
	causes := make([]GateInstanceProjection, 0, len(records))
	for _, record := range records {
		gateID := strings.TrimSpace(record.GateID)
		if gateID == "" {
			return nil, fmt.Errorf("%w: route-causing verdict gate_id is required", ErrValidation)
		}
		if record.ItemIndex < 0 {
			return nil, fmt.Errorf("%w: route-causing verdict item_index must be non-negative", ErrValidation)
		}
		causes = append(causes, GateInstanceProjection{GateID: gateID, ItemIndex: record.ItemIndex})
	}
	return causes, nil
}

func projectBestGeneration(run Run, outputs []GenerationOutput) (BestGeneration, error) {
	generation, err := bestGenerationValue(run)
	if err != nil {
		return BestGeneration{}, err
	}
	nodes := make(map[string]map[int]BestNodeProjection, len(outputs))
	for _, output := range outputs {
		nodeID := strings.TrimSpace(output.NodeID)
		if nodeID == "" {
			return BestGeneration{}, fmt.Errorf("%w: generation output node_id is required", ErrValidation)
		}
		if output.ItemIndex < 0 {
			return BestGeneration{}, fmt.Errorf("%w: generation output item_index must be non-negative", ErrValidation)
		}
		if nodes[nodeID] == nil {
			nodes[nodeID] = make(map[int]BestNodeProjection)
		}
		nodes[nodeID][output.ItemIndex] = BestNodeProjection{Output: generationOutputRuntimeValue(output)}
	}
	return BestGeneration{Generation: generation, Score: *run.BestScore, Nodes: nodes}, nil
}

func bestGenerationValue(run Run) (int64, error) {
	if run.BestGeneration == nil || run.BestScore == nil {
		return 0, fmt.Errorf("%w: best generation and score must be set together", ErrValidation)
	}
	if *run.BestGeneration < 1 {
		return 0, fmt.Errorf("%w: best generation must be positive", ErrValidation)
	}
	if math.IsNaN(*run.BestScore) || math.IsInf(*run.BestScore, 0) {
		return 0, fmt.Errorf("%w: best score must be finite", ErrValidation)
	}
	return *run.BestGeneration, nil
}

func historyJSONValue(raw json.RawMessage) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var value any
	if err := decodeSingleJSONValue(bytes.NewReader(raw), &value); err != nil {
		return nil, fmt.Errorf("decode JSON value: %w", err)
	}
	return value, nil
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
