package loop

import (
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
	Status string
	Output any
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

	var bestOutputs []GenerationOutput
	if run.BestGeneration != nil {
		bestGeneration, err := bestGenerationValue(run)
		if err != nil {
			return GenerationHistory{}, err
		}
		if bestGeneration == previousGeneration {
			bestOutputs = previousOutputs
		} else {
			bestOutputs, err = reader.ListGenerationOutputs(
				ctx,
				run.WorkspaceID,
				run.ID,
				int(bestGeneration),
			)
			if err != nil {
				return GenerationHistory{}, fmt.Errorf("read best generation outputs: %w", err)
			}
		}
	}

	return ProjectGenerationHistory(
		generation,
		run,
		previousOutputs,
		previousVerdicts,
		routeCauses,
		bestOutputs,
	)
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
	if generation < 1 {
		return GenerationHistory{}, fmt.Errorf("%w: generation must be positive", ErrValidation)
	}
	if generation == 1 {
		return GenerationHistory{}, nil
	}

	previous, err := projectPreviousGeneration(int64(generation-1), previousOutputs, previousVerdicts, routeCauses)
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
) (PreviousGeneration, error) {
	nodes, err := projectPreviousNodes(outputs)
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

func projectPreviousNodes(outputs []GenerationOutput) (map[string]map[int]NodeProjection, error) {
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
		nodes[nodeID][output.ItemIndex] = NodeProjection{
			Status: output.Status,
			Output: outputValue(output.OutputRef),
		}
	}
	return nodes, nil
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
		nodes[nodeID][output.ItemIndex] = BestNodeProjection{Output: outputValue(output.OutputRef)}
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
	if err := decodeSingleJSONValue(strings.NewReader(string(raw)), &value); err != nil {
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
