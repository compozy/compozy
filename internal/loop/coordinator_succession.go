package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

func rejectionRerunSet(
	graph dsl.Graph,
	outputs []GenerationOutput,
	gateIDs []string,
) (map[generationOutputKey]struct{}, error) {
	if len(gateIDs) == 0 {
		return nil, fmt.Errorf("%w: rejection rerun set requires a route-causing gate", ErrValidation)
	}
	topology := newControlTopology(graph)
	routeNodes := make(map[dsl.NodeID]struct{})
	for _, rawID := range gateIDs {
		gateID := dsl.NodeID(strings.TrimSpace(rawID))
		node, ok := graphNode(graph, gateID)
		if !ok || !isControlKind(node, dsl.ControlGate) {
			return nil, fmt.Errorf("%w: route-causing gate %q is not in the graph", ErrValidation, gateID)
		}
		queue := []dsl.NodeID{gateID}
		for len(queue) > 0 {
			nodeID := queue[0]
			queue = queue[1:]
			if _, seen := routeNodes[nodeID]; seen {
				continue
			}
			routeNodes[nodeID] = struct{}{}
			queue = append(queue, topology.dependencies[nodeID]...)
		}
	}
	rerun := make(map[generationOutputKey]struct{})
	for _, output := range outputs {
		if GenerationOutputStatusParked(output.Status) {
			continue
		}
		if _, ok := routeNodes[dsl.NodeID(output.NodeID)]; !ok {
			continue
		}
		rerun[generationOutputKey{nodeID: output.NodeID, itemIndex: output.ItemIndex}] = struct{}{}
	}
	addTransitiveDependents(graph, outputs, rerun)
	return rerun, nil
}

func successionGenerationOutputs(
	graph dsl.Graph,
	seedOutputs []GenerationOutput,
	currentOutputs []GenerationOutput,
	rerun map[generationOutputKey]struct{},
	nextGeneration int,
) []GenerationOutput {
	seedByKey := generationOutputMap(seedOutputs)
	topology := newControlTopology(graph)
	next := make([]GenerationOutput, 0, len(currentOutputs))
	for _, current := range currentOutputs {
		node, ok := graphNode(graph, dsl.NodeID(current.NodeID))
		if !ok {
			continue
		}
		key := generationOutputKey{nodeID: current.NodeID, itemIndex: current.ItemIndex}
		if GenerationOutputStatusParked(current.Status) {
			current.Generation = nextGeneration
			current.ExpectedEpoch = nil
			next = append(next, current)
			continue
		}
		seed, hasSeed := seedByKey[key]
		_, shouldRerun := rerun[key]
		if shouldRerun {
			if _, inFanOutBody := topology.inFanOutBody(node.ID); inFanOutBody {
				// The fan-out control rematerializes exactly the successor collection width.
				// Carrying prior body slots would preserve branches that no longer have an item.
				continue
			}
		}
		if shouldRerun || !hasSeed {
			next = append(next, reattemptPendingOutput(node, current, nextGeneration))
			continue
		}
		seed.Generation = nextGeneration
		seed.Status = generationOutputSucceeded
		next = append(next, seed)
	}
	return sortedGenerationOutputs(next)
}

func firstFailedGenerationOutput(outputs []GenerationOutput) (GenerationOutput, bool) {
	for _, output := range sortedGenerationOutputs(outputs) {
		if output.Status == generationOutputFailed {
			return output, true
		}
	}
	return GenerationOutput{}, false
}

func (r *CoordinatorRunner) loadPersistedRouteCauses(
	ctx context.Context,
	run Run,
	generation int,
	graph dsl.Graph,
	outputs []GenerationOutput,
) (*gateEvaluationCollector, error) {
	collector := &gateEvaluationCollector{}
	if r.verdicts == nil {
		return collector, nil
	}
	records, err := r.verdicts.ListRouteCausingVerdicts(
		ctx,
		string(run.WorkspaceID),
		string(run.ID),
		int64(generation),
	)
	if err != nil {
		return nil, fmt.Errorf("load route-causing gate verdicts: %w", err)
	}
	outputs, err = generationOutputRuntimeView(ctx, r.outputs, generationOutputRuntimeScope{
		workspaceID: run.WorkspaceID,
		runID:       run.ID,
		generation:  generation,
	}, outputs)
	if err != nil {
		return nil, fmt.Errorf("load route-causing gate output payloads: %w", err)
	}
	outputByKey := generationOutputMap(outputs)
	for _, record := range records {
		node, ok := graphNode(graph, dsl.NodeID(record.GateID))
		if !ok || !isControlKind(node, dsl.ControlGate) {
			return nil, fmt.Errorf(
				"%w: persisted route-causing gate %q is not in the graph",
				ErrValidation,
				record.GateID,
			)
		}
		output, ok := outputByKey[generationOutputKey{
			nodeID:    string(node.ID),
			itemIndex: record.ItemIndex,
		}]
		if !ok {
			return nil, fmt.Errorf("%w: persisted route-causing gate %q has no output", ErrValidation, record.GateID)
		}
		if output.Status != generationOutputSucceeded && output.Status != generationOutputFailed {
			return nil, fmt.Errorf(
				"%w: persisted route-causing gate %q output status is %q, want a terminal gate status",
				ErrValidation,
				record.GateID,
				output.Status,
			)
		}
		runtimeRef := generationOutputRuntimePayload(output)
		if strings.TrimSpace(runtimeRef) == "" {
			return nil, fmt.Errorf(
				"%w: persisted route-causing gate %q finished without an output reference",
				ErrValidation,
				record.GateID,
			)
		}
		var verdict gate.Verdict
		if err := json.Unmarshal([]byte(runtimeRef), &verdict); err != nil {
			return nil, fmt.Errorf("decode persisted route-causing gate %q output: %w", record.GateID, err)
		}
		collector.record(gate.GateFromNode(node), record.ItemIndex, verdict)
	}
	return collector, nil
}

func (r *CoordinatorRunner) buildGateSuccessionPlan(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	currentGeneration int,
	graph dsl.Graph,
	gatesEnabled bool,
	currentPlan task.CoordinatorCompletionPlan,
	currentOutputs []GenerationOutput,
	causes []gateEvaluation,
) (task.CoordinatorCompletionPlan, error) {
	nextGeneration := currentGeneration + 1
	if terminal := iterationCapTerminal(run, nextGeneration); terminal != nil {
		currentPlan.Terminal = terminal
		return currentPlan, nil
	}
	action := routeActionForCauses(causes)
	intent, err := r.gateSuccessionIntent(run, currentGeneration, nextGeneration, action, causes)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if denied, deniedPlan := r.dispatchGenerationPre(ctx, taskRun, run, intent); denied {
		deniedPlan.Snapshot = currentPlan.Snapshot
		return deniedPlan, nil
	}
	var nextPlan task.CoordinatorCompletionPlan
	switch action {
	case gate.RouteNextGeneration:
		nextPlan, err = buildFreshGenerationCoordinatorPlan(
			taskRun,
			run,
			currentGeneration,
			nextGeneration,
			graph,
			gatesEnabled,
			currentOutputs,
			currentPlan.RunStops,
		)
		if err == nil {
			err = applyGenerationIntent(&nextPlan, intent)
		}
	case gate.RouteRevise:
		nextPlan, err = r.buildReviseGenerationPlan(
			ctx,
			taskRun,
			run,
			currentGeneration,
			nextGeneration,
			graph,
			gatesEnabled,
			currentOutputs,
			currentPlan.RunStops,
			causes,
			intent,
		)
	default:
		err = fmt.Errorf("%w: unsupported gate succession action %q", ErrValidation, action)
	}
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	nextPlan.Snapshot = currentPlan.Snapshot
	r.dispatchGenerationPost(ctx, taskRun, run, intent)
	return nextPlan, nil
}

func (r *CoordinatorRunner) gateSuccessionIntent(
	run Run,
	currentGeneration int,
	nextGeneration int,
	action gate.RouteAction,
	causes []gateEvaluation,
) (GenerationIntent, error) {
	intent := GenerationIntent{
		Generation:       int64(nextGeneration),
		ParentGeneration: int64(currentGeneration),
	}
	switch action {
	case gate.RouteNextGeneration:
		intent.Origin = originForNextGeneration(causes)
	case gate.RouteRevise:
		intent.Origin = OriginGateRevise
		if run.BestGeneration != nil && causesMetricGate(causes) {
			if run.BestScore == nil {
				return GenerationIntent{}, fmt.Errorf(
					"%w: best generation and score must be set together",
					ErrValidation,
				)
			}
			if *run.BestGeneration < int64(currentGeneration) {
				intent.ParentGeneration = *run.BestGeneration
				intent.Origin = OriginRatchetRestore
			} else {
				r.logger.Warn(
					"loop: ignored inconsistent best generation for ratchet restore",
					"loop_run_id", run.ID,
					"best_generation", *run.BestGeneration,
					"current_generation", currentGeneration,
				)
			}
		}
	default:
		return GenerationIntent{}, fmt.Errorf("%w: unsupported gate succession action %q", ErrValidation, action)
	}
	if err := intent.Validate(); err != nil {
		return GenerationIntent{}, err
	}
	return intent, nil
}

func originForNextGeneration(causes []gateEvaluation) GenerationOrigin {
	for _, cause := range causes {
		if cause.runtime.ID == definitionOfDoneGateID {
			return OriginDoDRetry
		}
	}
	return OriginGateNextGeneration
}

func (r *CoordinatorRunner) buildReviseGenerationPlan(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	currentGeneration int,
	nextGeneration int,
	graph dsl.Graph,
	gatesEnabled bool,
	currentOutputs []GenerationOutput,
	loopStops []task.CoordinatorStopSpec,
	causes []gateEvaluation,
	intent GenerationIntent,
) (task.CoordinatorCompletionPlan, error) {
	rerun, err := rejectionRerunSet(graph, currentOutputs, routeCauseIDs(causes))
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	seed := currentOutputs
	if intent.Origin == OriginRatchetRestore {
		seed, err = r.outputs.ListGenerationOutputs(
			ctx,
			run.WorkspaceID,
			run.ID,
			int(intent.ParentGeneration),
		)
		if err != nil {
			return task.CoordinatorCompletionPlan{}, fmt.Errorf("load ratchet baseline outputs: %w", err)
		}
	}
	nextOutputs := successionGenerationOutputs(graph, seed, currentOutputs, rerun, nextGeneration)
	plan, err := buildNextGenerationCoordinatorPlan(
		taskRun,
		run,
		currentGeneration,
		nextGeneration,
		graph,
		gatesEnabled,
		currentOutputs,
		nextOutputs,
		loopStops,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if err := applyGenerationIntent(&plan, intent); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	return plan, nil
}

func causesMetricGate(causes []gateEvaluation) bool {
	for _, cause := range causes {
		for _, criterion := range cause.runtime.Criteria {
			if criterion.Metric != nil {
				return true
			}
		}
	}
	return false
}
