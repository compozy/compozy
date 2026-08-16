package loop

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func collectDependenciesReady(
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
	fanOutID dsl.NodeID,
	collect GenerationOutput,
) bool {
	outputMap := generationOutputMap(outputs)
	for _, dependency := range topology.dependencies[dsl.NodeID(collect.NodeID)] {
		if _, inScope := topology.fanOutScopes[fanOutID].body[dependency]; inScope {
			continue
		}
		output, ok := outputMap[generationOutputKey{nodeID: string(dependency), itemIndex: 0}]
		if !ok || output.Status != generationOutputSucceeded {
			return false
		}
	}
	settlement, ok := fanOutJoinSettlement(graph, topology, outputs, fanOutID, dsl.NodeID(collect.NodeID))
	return ok && settlement.State != joinSettlementPending
}

func fanOutJoinSettlement(
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
	fanOutID dsl.NodeID,
	collectID dsl.NodeID,
) (joinSettlement, bool) {
	branchCount, ok := fanOutBranchCount(outputs, fanOutID)
	if !ok {
		return joinSettlement{}, false
	}
	fanOut, ok := graphNode(graph, fanOutID)
	if !ok {
		return joinSettlement{}, false
	}
	strategy := dsl.StrategySpec{Kind: dsl.StrategyWaitAll}
	if fanOut.Strategy != nil {
		strategy = *fanOut.Strategy
	}
	lanes := make([]joinLaneState, 0, branchCount)
	for itemIndex := range branchCount {
		lanes = append(lanes, collectLaneState(topology, outputs, fanOutID, collectID, itemIndex))
	}
	return settleJoin(strategy, branchCount, lanes, nil), true
}

func collectLaneState(
	topology controlTopology,
	outputs []GenerationOutput,
	fanOutID dsl.NodeID,
	collectID dsl.NodeID,
	itemIndex int,
) joinLaneState {
	outputMap := generationOutputMap(outputs)
	state := joinLaneState{ItemIndex: itemIndex, Status: generationOutputSucceeded, Definitive: true}
	found := false
	for _, dependency := range topology.dependencies[collectID] {
		if _, inScope := topology.fanOutScopes[fanOutID].body[dependency]; !inScope {
			continue
		}
		output, ok := outputMap[generationOutputKey{nodeID: string(dependency), itemIndex: itemIndex}]
		if !ok {
			return joinLaneState{ItemIndex: itemIndex, Status: generationOutputPending}
		}
		found = true
		switch output.Status {
		case generationOutputFailed:
			return joinLaneState{ItemIndex: itemIndex, Status: generationOutputFailed, Definitive: true}
		case generationOutputCanceled:
			state.Status = generationOutputCanceled
		case generationOutputSucceeded:
			if state.OutputRef == "" {
				state.OutputRef = output.OutputRef
			}
		default:
			if state.Status != generationOutputCanceled {
				state.Status = output.Status
				state.Definitive = false
			}
		}
	}
	if !found {
		return joinLaneState{ItemIndex: itemIndex, Status: generationOutputPending}
	}
	return state
}

func evaluateCollectNode(
	eval *controlEvalContext,
	plan *task.CoordinatorCompletionPlan,
	output GenerationOutput,
	node dsl.Node,
	outputs *[]GenerationOutput,
	outputBlobs *[]GenerationOutputBlob,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	fanOutID, ok := eval.topology.isCollectForFanOut(node.ID)
	if !ok {
		output.Status = generationOutputSucceeded
		return output, nil, nil
	}
	settlement, ok := fanOutJoinSettlement(
		eval.resolved.Definition.Graph, eval.topology, *outputs, fanOutID, node.ID,
	)
	if !ok || settlement.State == joinSettlementPending {
		return output, nil, nil
	}
	payload, err := json.Marshal(settlement.Coverage)
	if err != nil {
		return output, nil, fmt.Errorf("loop: marshal collect coverage: %w", err)
	}
	storedRef, runtimePayload, err := generationOutputRefForPayload(payload, outputBlobs, eval.now)
	if err != nil {
		return output, nil, err
	}
	if settlement.WinnerItem != nil && strings.TrimSpace(settlement.WinnerRef) != "" {
		storedRef = settlement.WinnerRef
		runtimePayload = nil
	}
	output.Status = string(settlement.State)
	setGenerationOutputRef(&output, storedRef)
	output.runtimePayload = runtimePayload
	if len(settlement.CancelItems) > 0 {
		if err := applyStrategyLaneCancellations(eval, plan, fanOutID, settlement.CancelItems, outputs); err != nil {
			return output, nil, err
		}
	}
	return output, nil, nil
}

func applyStrategyLaneCancellations(
	eval *controlEvalContext,
	plan *task.CoordinatorCompletionPlan,
	fanOutID dsl.NodeID,
	items []int,
	outputs *[]GenerationOutput,
) error {
	slices.Sort(items)
	itemSet := make(map[int]struct{}, len(items))
	for _, itemIndex := range items {
		itemSet[itemIndex] = struct{}{}
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return err
	}
	scope := eval.topology.fanOutScopes[fanOutID]
	materializedItems := make(map[int]bool, len(items))
	for index := range *outputs {
		cell := &(*outputs)[index]
		if _, cancel := itemSet[cell.ItemIndex]; !cancel {
			continue
		}
		if _, inScope := scope.body[dsl.NodeID(cell.NodeID)]; !inScope {
			continue
		}
		materializedItems[cell.ItemIndex] = true
		if generationOutputTerminal(cell.Status) {
			continue
		}
		expectedEpoch := cell.Epoch
		cell.ExpectedEpoch = &expectedEpoch
		cell.Epoch++
		cell.Status = generationOutputCanceled
		cell.OutputRef = strategyCanceledReasonCode
		cell.NextAttemptAt = nil
		cell.TaskRunID = ""
		payload.StrategyCancellations = append(payload.StrategyCancellations, StrategyCancellationIntent{
			NodeID: cell.NodeID, ItemIndex: cell.ItemIndex, ActorKind: "system", ActorID: "strategy",
			ReasonCode: strategyCanceledReasonCode, At: eval.now.UTC(),
		})
		if strings.TrimSpace(cell.SessionID) != "" {
			plan.LaneCancels = append(plan.LaneCancels, task.CoordinatorLaneCancelSpec{
				LoopRunID: string(eval.run.ID), NodeID: cell.NodeID, ItemIndex: cell.ItemIndex,
				SessionIDs: []string{cell.SessionID}, ReasonCode: strategyCanceledReasonCode,
			})
		}
	}
	materialized := make([]int, 0, len(items))
	neverStarted := make([]int, 0, len(items))
	for _, itemIndex := range items {
		if materializedItems[itemIndex] {
			materialized = append(materialized, itemIndex)
			continue
		}
		neverStarted = append(neverStarted, itemIndex)
		for _, node := range eval.resolved.Definition.Graph.Nodes {
			if _, inScope := scope.body[node.ID]; !inScope {
				continue
			}
			*outputs = append(*outputs, GenerationOutput{
				Generation: eval.generation, NodeID: string(node.ID), ItemIndex: itemIndex,
				Status: generationOutputCanceled, OutputRef: strategyNeverStartedReasonCode, Attempt: 1,
			})
		}
	}
	payload.Events = append(payload.Events,
		branchPrunedEventIntents(fanOutID, materialized, strategyCanceledReasonCode)...)
	payload.Events = append(payload.Events,
		branchPrunedEventIntents(fanOutID, neverStarted, strategyNeverStartedReasonCode)...)
	plan.Snapshot.Payload = payload
	return nil
}

func generationOutputTerminal(status string) bool {
	switch status {
	case generationOutputSucceeded, generationOutputPartial, generationOutputFailed,
		generationOutputCanceled, generationOutputQuarantined:
		return true
	default:
		return false
	}
}

func fanOutCollectSettled(
	topology controlTopology,
	outputs map[generationOutputKey]GenerationOutput,
	fanOutID dsl.NodeID,
) bool {
	scope := topology.fanOutScopes[fanOutID]
	if len(scope.collect) == 0 {
		return false
	}
	for collectID := range scope.collect {
		output, ok := outputs[generationOutputKey{nodeID: string(collectID), itemIndex: 0}]
		if !ok || output.Status != generationOutputSucceeded && output.Status != generationOutputPartial {
			return false
		}
	}
	return true
}
