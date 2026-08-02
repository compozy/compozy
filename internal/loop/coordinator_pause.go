package loop

import (
	"context"

	"github.com/compozy/compozy/internal/task"
)

func (r *CoordinatorRunner) applyPausedNodeControls(
	ctx context.Context,
	run Run,
	plan task.CoordinatorCompletionPlan,
) (task.CoordinatorCompletionPlan, error) {
	controls, err := r.listNodeControls(ctx, run)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	paused := make(map[NodeID]struct{})
	for _, control := range controls {
		if control.Paused {
			paused[control.NodeID] = struct{}{}
		}
	}
	if len(paused) == 0 {
		return plan, nil
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	parked := false
	for index := range payload.Outputs {
		if _, found := paused[NodeID(payload.Outputs[index].NodeID)]; !found {
			continue
		}
		parked = true
		switch payload.Outputs[index].Status {
		case generationOutputPending, generationOutputEnqueued, generationOutputRetrying:
			payload.Outputs[index].Status = generationOutputPaused
			payload.Outputs[index].TaskRunID = ""
			payload.Outputs[index].Epoch++
		}
	}
	if !parked {
		return plan, nil
	}
	plan.Snapshot.Payload = payload
	plan.NodeRuns, err = nodeRunsWithoutPausedNodes(plan.NodeRuns, paused)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	plan.GenerationInFlight = true
	if len(plan.NodeRuns) == 0 {
		if plan.Terminal != nil && plan.Terminal.ReasonCode == noReadyNodesReasonCode {
			plan.Terminal = nil
		}
		if plan.Terminal == nil {
			plan.Yield = true
		}
	}
	return plan, nil
}

func nodeRunsWithoutPausedNodes(runs []task.EnqueueSpec, paused map[NodeID]struct{}) ([]task.EnqueueSpec, error) {
	filtered := make([]task.EnqueueSpec, 0, len(runs))
	for _, run := range runs {
		metadata, err := parseCoordinatorActionRunMetadata(run.Metadata)
		if err != nil {
			return nil, err
		}
		if _, found := paused[NodeID(metadata.NodeID)]; found {
			continue
		}
		filtered = append(filtered, run)
	}
	return filtered, nil
}
