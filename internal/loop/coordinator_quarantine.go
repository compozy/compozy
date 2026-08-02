package loop

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func (r *CoordinatorRunner) repeatedFailureNodes(
	ctx context.Context,
	run Run,
	generation int,
	outputs []GenerationOutput,
) ([]string, error) {
	history, err := r.generationFailureHistory(
		ctx, run.WorkspaceID, run.ID, generation, outputs, LoopFailureBreakerLimit,
	)
	if err != nil {
		return nil, err
	}
	return repeatedFailedNodeIDs(history), nil
}

func repeatedFailedNodeIDs(history [][]GenerationOutput) []string {
	if len(history) < LoopFailureBreakerLimit {
		return nil
	}
	failing := failedNodeIDs(history[0])
	for _, outputs := range history[1:LoopFailureBreakerLimit] {
		previous := failedNodeIDs(outputs)
		for nodeID := range failing {
			if _, ok := previous[nodeID]; !ok {
				delete(failing, nodeID)
			}
		}
	}
	ids := make([]string, 0, len(failing))
	for nodeID := range failing {
		ids = append(ids, nodeID)
	}
	sort.Strings(ids)
	return ids
}

func (r *CoordinatorRunner) applyQuarantineToPlan(
	ctx context.Context,
	run Run,
	generation int,
	graph dsl.Graph,
	plan task.CoordinatorCompletionPlan,
	outputs []GenerationOutput,
	nodeIDs []string,
) (task.CoordinatorCompletionPlan, []GenerationOutput, error) {
	if len(nodeIDs) == 0 {
		return plan, outputs, nil
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, nil, err
	}
	controls, err := r.listNodeControls(ctx, run)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, nil, err
	}
	allAttempts, err := r.listNodeAttempts(ctx, run)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, nil, err
	}
	controlByNode := nodeControlsByID(controls)
	quarantined := make(map[string]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		quarantined[strings.TrimSpace(nodeID)] = struct{}{}
	}
	for index := range outputs {
		if _, found := quarantined[outputs[index].NodeID]; !found {
			continue
		}
		outputs[index].Status = generationOutputQuarantined
		outputs[index].NextAttemptAt = nil
		outputs[index].Epoch++
	}
	payload.Outputs = cloneGenerationOutputs(outputs)
	for index := range payload.Attempts {
		if _, found := quarantined[string(payload.Attempts[index].NodeID)]; found &&
			payload.Attempts[index].Disposition == AttemptEscalated {
			payload.Attempts[index].Disposition = AttemptQuarantined
		}
	}
	allAttempts = append(allAttempts, payload.Attempts...)
	for _, nodeID := range nodeIDs {
		entry, mutation, event, buildErr := r.buildNodeQuarantine(
			run,
			generation,
			graph,
			NodeID(nodeID),
			allAttempts,
			controlByNode[NodeID(nodeID)],
		)
		if buildErr != nil {
			return task.CoordinatorCompletionPlan{}, nil, buildErr
		}
		mutation.QuarantineEntry = entry
		payload.Controls = append(payload.Controls, mutation)
		payload.Events = append(payload.Events, event)
	}
	plan.Snapshot.Payload = payload
	return plan, outputs, nil
}

func (r *CoordinatorRunner) buildNodeQuarantine(
	run Run,
	generation int,
	graph dsl.Graph,
	nodeID NodeID,
	allAttempts []NodeAttempt,
	control NodeControl,
) (jsonEntry []byte, mutation NodeControlMutation, event GenerationLifecycleEventIntent, err error) {
	if _, found := graphNode(graph, nodeID); !found {
		return nil, NodeControlMutation{}, GenerationLifecycleEventIntent{},
			fmt.Errorf("%w: quarantine node %q is absent from the graph", ErrValidation, nodeID)
	}
	entry, err := decodeQuarantineEntry(control.QuarantineEntry)
	if err != nil {
		return nil, NodeControlMutation{}, GenerationLifecycleEventIntent{}, err
	}
	chain := quarantineAttemptChain(allAttempts, nodeID, generation)
	if len(chain) == 0 {
		return nil, NodeControlMutation{}, GenerationLifecycleEventIntent{},
			fmt.Errorf("%w: quarantine node %q has no attempt evidence", ErrValidation, nodeID)
	}
	entry.NodeID = nodeID
	entry.InputRef = quarantineInputRef(run, nodeID)
	entry.Target = latestAttemptTarget(chain)
	entry.Episodes = append(entry.Episodes, QuarantineEpisode{
		Generation: generation, QuarantinedAt: r.now().UTC(), Attempts: chain,
	})
	encoded, err := encodeQuarantineEntry(entry)
	if err != nil {
		return nil, NodeControlMutation{}, GenerationLifecycleEventIntent{}, err
	}
	attempt, found := latestQuarantineAttempt(chain, generation)
	if !found {
		return nil, NodeControlMutation{}, GenerationLifecycleEventIntent{},
			fmt.Errorf("%w: quarantine node %q has no current attempt", ErrValidation, nodeID)
	}
	failure := classifiedFailureFromAttempt(attempt)
	mutation = NodeControlMutation{
		Kind: NodeControlMutationQuarantine, NodeID: nodeID,
		ExpectedRevision: control.Revision, ExpectExisting: control.NodeID != "", At: r.now().UTC(),
	}
	event = GenerationLifecycleEventIntent{
		Kind: GenerationLifecycleEventNodeQuarantined, NodeID: string(nodeID),
		ItemIndex: attempt.ItemIndex, Attempt: attempt.Attempt, Failure: &failure,
		Disposition: AttemptQuarantined, QuarantineEntry: encoded,
	}
	return encoded, mutation, event, nil
}

func (r *CoordinatorRunner) listNodeControls(ctx context.Context, run Run) ([]NodeControl, error) {
	if r == nil || r.controls == nil {
		return nil, nil
	}
	controls, err := r.controls.ListNodeControls(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return nil, fmt.Errorf("loop: list node controls: %w", err)
	}
	return controls, nil
}

func nodeControlsByID(controls []NodeControl) map[NodeID]NodeControl {
	result := make(map[NodeID]NodeControl, len(controls))
	for _, control := range controls {
		result[control.NodeID] = control
	}
	return result
}

func quarantineAttemptChain(attempts []NodeAttempt, nodeID NodeID, generation int) []NodeAttempt {
	chain := make([]NodeAttempt, 0)
	for _, attempt := range attempts {
		if attempt.NodeID != nodeID || attempt.Generation < generation-LoopFailureBreakerLimit+1 ||
			attempt.Generation > generation || attempt.FailureClass == nil {
			continue
		}
		chain = append(chain, attempt)
	}
	sort.Slice(chain, func(i, j int) bool {
		if chain[i].Generation != chain[j].Generation {
			return chain[i].Generation < chain[j].Generation
		}
		if chain[i].ItemIndex != chain[j].ItemIndex {
			return chain[i].ItemIndex < chain[j].ItemIndex
		}
		return chain[i].Attempt < chain[j].Attempt
	})
	return chain
}

func latestAttemptTarget(attempts []NodeAttempt) string {
	for _, attempt := range slices.Backward(attempts) {
		if target := strings.TrimSpace(attempt.Target); target != "" {
			return target
		}
	}
	return ""
}

func latestQuarantineAttempt(attempts []NodeAttempt, generation int) (NodeAttempt, bool) {
	for _, attempt := range slices.Backward(attempts) {
		if attempt.Generation == generation && attempt.FailureClass != nil {
			return attempt, true
		}
	}
	return NodeAttempt{}, false
}

func (r *CoordinatorRunner) applyParkedDependencyAttention(
	ctx context.Context,
	run Run,
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
	plan task.CoordinatorCompletionPlan,
) (task.CoordinatorCompletionPlan, bool, error) {
	if !generationHasParkedOutput(outputs) {
		return plan, false, nil
	}
	controls, err := r.listNodeControls(ctx, run)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, false, err
	}
	controlByNode := nodeControlsByID(controls)
	planned := make(map[NodeID]struct{})
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, false, err
	}
	for _, output := range outputs {
		if output.Status != generationOutputPending {
			continue
		}
		nodeID := NodeID(output.NodeID)
		if _, exists := planned[nodeID]; exists {
			continue
		}
		dependency := requiredParkedProducer(graph, topology, outputs, output)
		if dependency == "" {
			continue
		}
		reason := fmt.Sprintf("node %s requires parked producer %s", output.NodeID, dependency)
		control := controlByNode[nodeID]
		if control.AttentionFlag == attentionDependencyFlag && control.AttentionReason == reason {
			continue
		}
		payload.Controls = append(payload.Controls, NodeControlMutation{
			Kind: NodeControlMutationAttention, NodeID: nodeID,
			ExpectedRevision: control.Revision, ExpectExisting: control.NodeID != "",
			AttentionFlag: attentionDependencyFlag, AttentionReason: reason, At: r.now().UTC(),
		})
		planned[nodeID] = struct{}{}
	}
	plan.Snapshot.Payload = payload
	plan.Yield = true
	return plan, true, nil
}

func generationHasParkedOutput(outputs []GenerationOutput) bool {
	for _, output := range outputs {
		if GenerationOutputStatusParked(output.Status) {
			return true
		}
	}
	return false
}

func requiredParkedProducer(
	graph dsl.Graph,
	topology controlTopology,
	outputs []GenerationOutput,
	consumer GenerationOutput,
) string {
	outputMap := generationOutputMap(outputs)
	visited := make(map[generationOutputKey]struct{})
	var walk func(GenerationOutput) string
	walk = func(current GenerationOutput) string {
		key := generationOutputKey{nodeID: current.NodeID, itemIndex: current.ItemIndex}
		if _, seen := visited[key]; seen {
			return ""
		}
		visited[key] = struct{}{}
		for _, dependency := range topology.dependencies[dsl.NodeID(current.NodeID)] {
			upstream, ok := dependencyOutputForNode(
				topology, outputMap, dsl.NodeID(current.NodeID), dependency, current.ItemIndex,
			)
			if !ok {
				continue
			}
			if GenerationOutputStatusParked(upstream.Status) {
				return upstream.NodeID
			}
			if producer := walk(upstream); producer != "" {
				return producer
			}
		}
		return ""
	}
	if _, found := graphNode(graph, NodeID(consumer.NodeID)); !found {
		return ""
	}
	return walk(consumer)
}
