package daemon

import (
	"fmt"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
)

// loopWatchEventsReadModel projects the parked watch-events node state for a loop-run
// detail response. It returns a value only while the run's current generation is dormant
// at a watch-events source node (its output_ref carries the durable pending park state);
// otherwise it returns nil so clients render nothing (truthful UI). The read-model is an
// explicit contract projection of the loop domain park state — no internal DSL/watch
// types cross the wire (ADR-024 §8).
func loopWatchEventsReadModel(
	run looppkg.Run,
	executedDefinition *contract.LoopDefinitionDocument,
	generations []contract.LoopGenerationPayload,
) (*contract.LoopWatchEventsState, error) {
	if executedDefinition == nil || run.Generation <= 0 {
		return nil, nil
	}
	watchNodes := watchEventsNodeIDs(*executedDefinition)
	if len(watchNodes) == 0 {
		return nil, nil
	}
	for _, output := range currentGenerationOutputs(run.Generation, generations) {
		if _, ok := watchNodes[output.NodeID]; !ok {
			continue
		}
		state, ok, err := watchpkg.EventsPendingFromOutputRef(output.OutputRef)
		if err != nil {
			return nil, fmt.Errorf(
				"daemon: decode watch-events park state for node %s: %w",
				output.NodeID,
				err,
			)
		}
		if !ok {
			continue
		}
		return watchEventsStatePayload(state, run.LastProgressAt), nil
	}
	return nil, nil
}

// watchEventsNodeIDs collects the ids of every watch-events source node in the executed
// definition graph — the only nodes whose output_ref carries watch-events park state.
func watchEventsNodeIDs(doc contract.LoopDefinitionDocument) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, node := range doc.Graph.Nodes {
		if node.Class == contract.LoopNodeClassSource && node.Kind == string(dsl.SourceWatchEvents) {
			ids[node.ID] = struct{}{}
		}
	}
	return ids
}

// currentGenerationOutputs returns the per-node outputs of the run's current generation.
func currentGenerationOutputs(
	generation int,
	generations []contract.LoopGenerationPayload,
) []contract.LoopGenerationOutput {
	for _, payload := range generations {
		if payload.Generation == generation {
			return payload.Outputs
		}
	}
	return nil
}

func watchEventsStatePayload(
	state watchpkg.EventsPendingState,
	lastProgressAt time.Time,
) *contract.LoopWatchEventsState {
	subscriptions := make([]contract.LoopWatchEventSubscription, 0, len(state.Subscriptions))
	for _, subscription := range state.Subscriptions {
		subscriptions = append(subscriptions, contract.LoopWatchEventSubscription{
			Kind:   contract.LoopWatchEventKind(subscription.Kind),
			Filter: subscription.Filter,
		})
	}
	out := &contract.LoopWatchEventsState{
		Subscriptions: subscriptions,
		Cursors:       state.Cursors,
	}
	if !lastProgressAt.IsZero() {
		at := lastProgressAt.UTC()
		out.LastWakeAt = &at
	}
	return out
}
