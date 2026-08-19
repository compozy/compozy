package loop

import (
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/task"
)

func buildReattemptCoordinatorPlan(
	taskRun task.Run,
	run Run,
	currentGeneration int,
	nextGeneration int,
	graph dsl.Graph,
	currentOutputs []GenerationOutput,
	loopStops []task.CoordinatorStopSpec,
) (task.CoordinatorCompletionPlan, error) {
	rerun, err := reattemptRerunSet(run.ReattemptStrategy, graph, currentOutputs)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	nextOutputs := reattemptGenerationOutputs(graph, currentOutputs, rerun, nextGeneration)
	return buildNextGenerationCoordinatorPlan(
		taskRun,
		run,
		currentGeneration,
		nextGeneration,
		graph,
		false,
		currentOutputs,
		nextOutputs,
		loopStops,
	)
}

func buildFreshGenerationCoordinatorPlan(
	taskRun task.Run,
	run Run,
	currentGeneration int,
	nextGeneration int,
	graph dsl.Graph,
	gatesEnabled bool,
	currentOutputs []GenerationOutput,
	loopStops []task.CoordinatorStopSpec,
) (task.CoordinatorCompletionPlan, error) {
	topology := newControlTopology(graph)
	nextOutputs := initialGenerationOutputs(graph, topology, nextGeneration)
	return buildNextGenerationCoordinatorPlan(
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
}

func buildNextGenerationCoordinatorPlan(
	taskRun task.Run,
	run Run,
	currentGeneration int,
	nextGeneration int,
	graph dsl.Graph,
	gatesEnabled bool,
	currentOutputs []GenerationOutput,
	nextOutputs []GenerationOutput,
	loopStops []task.CoordinatorStopSpec,
) (task.CoordinatorCompletionPlan, error) {
	topology := newControlTopology(graph)
	plan := coordinatorFinisherPlan(run, currentGeneration, currentOutputs, loopStops)
	plan.NodeTasks = make([]task.CoordinatorTaskSpec, 0, len(nextOutputs))
	plan.Dependencies = make([]task.CoordinatorDependencySpec, 0, len(graph.Edges))
	if err := appendCoordinatorTasksForOutputs(
		&plan,
		run,
		nextGeneration,
		graph,
		topology,
		gatesEnabled,
		nextOutputs,
	); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	appendCoordinatorDependenciesForOutputs(
		&plan,
		run,
		nextGeneration,
		graph,
		topology,
		gatesEnabled,
		nextOutputs,
	)
	plan.PostReserveSnapshot = &task.GenerationSnapshot{
		LoopRunID:  string(run.ID),
		Generation: nextGeneration,
		Payload:    GenerationSnapshotPayload{Outputs: nextOutputs},
	}
	plan.NextCoordinator = &task.EnqueueSpec{
		TaskID:                       strings.TrimSpace(taskRun.TaskID),
		RunID:                        coordinatorRunID(run.ID, nextGeneration),
		RunKind:                      task.RunKindCoordinator,
		LoopRunID:                    string(run.ID),
		IdempotencyKey:               coordinatorIdempotencyKey(run.ID, nextGeneration),
		ResolvedNetworkParticipation: participation.CloneSpec(run.NetworkSpecSnapshot()),
	}
	return plan, nil
}

func reattemptRerunSet(
	strategy ReattemptStrategy,
	graph dsl.Graph,
	outputs []GenerationOutput,
) (map[generationOutputKey]struct{}, error) {
	normalizedStrategy := normalizeReattemptStrategy(strategy)
	rerun := make(map[generationOutputKey]struct{}, len(outputs))
	switch normalizedStrategy {
	case ReattemptFullBody:
		for _, output := range outputs {
			if GenerationOutputStatusParked(output.Status) {
				continue
			}
			rerun[generationOutputKey{nodeID: output.NodeID, itemIndex: output.ItemIndex}] = struct{}{}
		}
		return rerun, nil
	case ReattemptFailedOnly:
	default:
		return nil, fmt.Errorf("%w: reattempt_strategy is invalid: %q", ErrValidation, strategy)
	}
	for _, output := range outputs {
		if GenerationOutputStatusParked(output.Status) {
			continue
		}
		if output.Status == generationOutputFailed || output.Status == generationOutputPending {
			rerun[generationOutputKey{nodeID: output.NodeID, itemIndex: output.ItemIndex}] = struct{}{}
		}
	}
	addTransitiveDependents(graph, outputs, rerun)
	return rerun, nil
}

func normalizeReattemptStrategy(strategy ReattemptStrategy) ReattemptStrategy {
	if strategy == "" {
		return ReattemptFailedOnly
	}
	return strategy
}

func addTransitiveDependents(
	graph dsl.Graph,
	outputs []GenerationOutput,
	rerun map[generationOutputKey]struct{},
) {
	topology := newControlTopology(graph)
	queue := make([]generationOutputKey, 0, len(rerun))
	for key := range rerun {
		queue = append(queue, key)
	}
	for len(queue) > 0 {
		key := queue[0]
		queue = queue[1:]
		for _, dependent := range dependentOutputKeys(topology, outputs, key) {
			if _, ok := rerun[dependent]; ok {
				continue
			}
			if outputStatusForKey(outputs, dependent, GenerationOutputStatusParked) {
				continue
			}
			rerun[dependent] = struct{}{}
			queue = append(queue, dependent)
		}
	}
}

func outputStatusForKey(
	outputs []GenerationOutput,
	key generationOutputKey,
	predicate func(string) bool,
) bool {
	for _, output := range outputs {
		if output.NodeID == key.nodeID && output.ItemIndex == key.itemIndex {
			return predicate(output.Status)
		}
	}
	return false
}

func dependentOutputKeys(
	topology controlTopology,
	outputs []GenerationOutput,
	key generationOutputKey,
) []generationOutputKey {
	dependents := topology.dependents[dsl.NodeID(key.nodeID)]
	keys := make([]generationOutputKey, 0, len(dependents))
	for _, dependent := range dependents {
		for _, output := range outputs {
			if output.NodeID != string(dependent) {
				continue
			}
			if topology.sameFanOutBody(dsl.NodeID(key.nodeID), dependent) &&
				output.ItemIndex != key.itemIndex {
				continue
			}
			if _, inBody := topology.inFanOutBody(dependent); !inBody && output.ItemIndex != 0 {
				continue
			}
			keys = append(keys, generationOutputKey{
				nodeID:    output.NodeID,
				itemIndex: output.ItemIndex,
			})
		}
	}
	return keys
}

func reattemptGenerationOutputs(
	graph dsl.Graph,
	currentOutputs []GenerationOutput,
	rerun map[generationOutputKey]struct{},
	nextGeneration int,
) []GenerationOutput {
	next := make([]GenerationOutput, 0, len(currentOutputs))
	for _, current := range currentOutputs {
		node, ok := graphNode(graph, dsl.NodeID(current.NodeID))
		if !ok {
			continue
		}
		key := generationOutputKey{nodeID: current.NodeID, itemIndex: current.ItemIndex}
		if _, shouldRerun := rerun[key]; shouldRerun {
			next = append(next, reattemptPendingOutput(node, current, nextGeneration))
			continue
		}
		carry := current
		carry.Generation = nextGeneration
		if !GenerationOutputStatusParked(carry.Status) {
			carry.Status = generationOutputSucceeded
		}
		carry.ExpectedEpoch = nil
		next = append(next, carry)
	}
	sort.Slice(next, func(i, j int) bool {
		if next[i].NodeID == next[j].NodeID {
			return next[i].ItemIndex < next[j].ItemIndex
		}
		return next[i].NodeID < next[j].NodeID
	})
	return next
}

func reattemptPendingOutput(
	node dsl.Node,
	current GenerationOutput,
	nextGeneration int,
) GenerationOutput {
	return GenerationOutput{
		Generation: nextGeneration,
		NodeID:     string(node.ID),
		ItemIndex:  current.ItemIndex,
		Status:     generationOutputPending,
		Attempt:    1,
	}
}
