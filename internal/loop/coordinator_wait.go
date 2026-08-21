package loop

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/watch"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/task"
)

func evaluateWaitNode(
	eval *controlEvalContext,
	plan *task.CoordinatorCompletionPlan,
	output GenerationOutput,
	node dsl.Node,
	outputs []GenerationOutput,
	outputBlobs *[]GenerationOutputBlob,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	var params dsl.WaitParams
	if err := node.Params.Decode(&params); err != nil {
		return GenerationOutput{}, nil, fmt.Errorf("loop: decode wait node %q: %w", node.ID, err)
	}
	if eval == nil || eval.now.IsZero() {
		return GenerationOutput{}, nil, fmt.Errorf("%w: wait evaluation clock is required", ErrValidation)
	}
	if err := renderWaitUntil(eval, node, output, outputs, &params); err != nil {
		return GenerationOutput{}, nil, err
	}
	now := eval.now.UTC()
	var aheadRejectFence json.RawMessage
	var aheadRejectCursors map[string]int64
	if params.Event != nil && params.AheadArrival != dsl.WaitAheadReject {
		consumed, consumedOutput, err := consumeAheadWaitEvent(
			eval, plan, output, node, params, outputs, outputBlobs, now,
		)
		if err != nil {
			return GenerationOutput{}, nil, err
		}
		if consumed {
			return consumedOutput, nil, nil
		}
	} else if params.Event != nil {
		fence, cursors, captureErr := captureRejectedAheadWaitEvents(eval, params)
		if captureErr != nil {
			return GenerationOutput{}, nil, captureErr
		}
		aheadRejectFence, aheadRejectCursors = fence, cursors
	}
	intent, err := buildNodeWaitIntent(node, output, params, now)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	output.Status = generationOutputWaiting
	output.TaskRunID = ""
	output.NextAttemptAt = nil
	output.Epoch++
	intent.IssuedEpoch = output.Epoch
	intent.AheadPayload = aheadRejectFence
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	payload.Waits = append(payload.Waits, intent)
	payload.Events = append(payload.Events, GenerationLifecycleEventIntent{
		Kind: GenerationLifecycleEventNodeWaitStarted, NodeID: output.NodeID,
		ItemIndex: output.ItemIndex, Attempt: output.Attempt, IssuedEpoch: output.Epoch,
		WaitKind: intent.Kind, NextAttemptAt: intent.ResumeAt,
		AheadArrival: string(params.AheadArrival), AheadCursors: aheadRejectCursors,
	})
	plan.Snapshot.Payload = payload
	return output, nil, nil
}

func renderWaitUntil(
	eval *controlEvalContext,
	node dsl.Node,
	output GenerationOutput,
	outputs []GenerationOutput,
	params *dsl.WaitParams,
) error {
	if params == nil || strings.TrimSpace(params.Until) == "" {
		return nil
	}
	namespace, err := runtimeNamespaceWithHistory(
		eval.run,
		eval.generation,
		eval.resolved.Definition.Graph,
		eval.topology,
		outputs,
		eval.history,
		node.ID,
		output.ItemIndex,
	)
	if err != nil {
		return err
	}
	rendered, err := renderActionParam(
		fmt.Sprintf("nodes.%s.params.until", node.ID),
		params.Until,
		namespace,
	)
	if err != nil {
		return fmt.Errorf("loop: render wait node %q until: %w", node.ID, err)
	}
	until, ok := rendered.(string)
	if !ok {
		return fmt.Errorf("%w: wait node %q until must resolve to a string", ErrValidation, node.ID)
	}
	params.Until = until
	return nil
}

func captureRejectedAheadWaitEvents(
	eval *controlEvalContext,
	params dsl.WaitParams,
) (json.RawMessage, map[string]int64, error) {
	if eval == nil || eval.watchEventsRuntime.ledger == nil || params.Event == nil {
		return nil, nil, fmt.Errorf("%w: reject ahead_arrival requires the watch-events ledger", ErrValidation)
	}
	reader, ok := eval.watchEventsRuntime.ledger.(WatchEventsCursorReader)
	if !ok {
		return nil, nil, fmt.Errorf("%w: reject ahead_arrival requires watch-events cursors", ErrValidation)
	}
	subscriptions := []watch.EventSubscriptionRef{{
		Kind: strings.TrimSpace(params.Event.Kind), Filter: strings.TrimSpace(params.Event.Filter),
	}}
	streams, kinds, err := watchEventsInitialStreamsAndKinds(
		subscriptions, eval.run.Inputs, eval.resolved.WatchEventsContracts,
	)
	if err != nil {
		return nil, nil, err
	}
	cursors, err := reader.ReadCursors(eval.ctx, WatchEventsQuery{
		WorkspaceID: string(eval.run.WorkspaceID),
		ReadScope:   store.ReadScope{ProfileID: strings.TrimSpace(eval.run.ProfileID)},
		Streams:     streams,
		Kinds:       kinds,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("loop: read reject ahead-arrival cursors: %w", err)
	}
	for stream, cursor := range streams {
		if _, found := cursors[stream]; !found {
			cursors[stream] = cursor
		}
	}
	fence, err := encodeWaitAheadRejectFence(cursors)
	if err != nil {
		return nil, nil, err
	}
	return fence, cloneInt64Map(cursors), nil
}

func consumeAheadWaitEvent(
	eval *controlEvalContext,
	plan *task.CoordinatorCompletionPlan,
	output GenerationOutput,
	node dsl.Node,
	params dsl.WaitParams,
	outputs []GenerationOutput,
	outputBlobs *[]GenerationOutputBlob,
	now time.Time,
) (bool, GenerationOutput, error) {
	runtime := eval.watchEventsRuntime
	if runtime.ledger == nil || params.Event == nil {
		return false, output, nil
	}
	subscriptions := []watch.EventSubscriptionRef{{
		Kind: strings.TrimSpace(params.Event.Kind), Filter: strings.TrimSpace(params.Event.Filter),
	}}
	streams, kinds, err := watchEventsInitialStreamsAndKinds(
		subscriptions,
		eval.run.Inputs,
		eval.resolved.WatchEventsContracts,
	)
	if err != nil {
		return false, output, err
	}
	rows, err := runtime.ledger.ReadMatches(eval.ctx, WatchEventsQuery{
		WorkspaceID: string(eval.run.WorkspaceID),
		ReadScope:   store.ReadScope{ProfileID: strings.TrimSpace(eval.run.ProfileID)},
		Streams:     streams,
		Kinds:       kinds,
		Limit:       LoopWatchEventPageLimit,
	})
	if err != nil {
		return false, output, fmt.Errorf("loop: read ahead wait events for node %q: %w", node.ID, err)
	}
	state := watch.EventsPendingState{Subscriptions: subscriptions, Cursors: streams}
	matches, _, _, terminal, err := filterWatchEventsRows(&watchEventsEvaluationContext{
		control: *eval, plan: plan, outputs: outputs,
	}, output, node, state, rows)
	if err != nil {
		return false, output, err
	}
	if terminal != nil {
		return false, output, fmt.Errorf("%w: wait event filter cannot exit the loop", ErrValidation)
	}
	if len(matches) == 0 {
		return false, output, nil
	}
	payload, err := json.Marshal(matches[0].EventMap())
	if err != nil {
		return false, output, fmt.Errorf("loop: marshal ahead wait event: %w", err)
	}
	var expect json.RawMessage
	if len(params.Expect) > 0 {
		expect, err = json.Marshal(params.Expect)
		if err != nil {
			return false, output, fmt.Errorf("loop: marshal ahead wait expect: %w", err)
		}
	}
	if err := ValidateWaitPayload(expect, payload); err != nil {
		return false, output, err
	}
	updated, err := recordAheadWaitEvent(plan, output, node, params, matches[0], payload, outputBlobs, now)
	return err == nil, updated, err
}

func recordAheadWaitEvent(
	plan *task.CoordinatorCompletionPlan,
	output GenerationOutput,
	node dsl.Node,
	params dsl.WaitParams,
	match WatchEvent,
	payload json.RawMessage,
	outputBlobs *[]GenerationOutputBlob,
	now time.Time,
) (GenerationOutput, error) {
	storedRef, runtimePayload, err := generationOutputRefForPayload(payload, outputBlobs, now)
	if err != nil {
		return output, fmt.Errorf("loop: persist ahead wait event output: %w", err)
	}
	output.Status = generationOutputSucceeded
	output.TaskRunID = ""
	output.NextAttemptAt = nil
	setGenerationOutputRef(&output, storedRef)
	output.runtimePayload = runtimePayload
	output.Epoch++
	claimedAt := now.UTC()
	intent, err := buildNodeWaitIntent(node, output, params, now)
	if err != nil {
		return output, err
	}
	intent.IssuedEpoch = output.Epoch
	intent.ClaimState = WaitClaimResumed
	intent.ClaimedByKind = "ahead_event"
	intent.ClaimedByID = fmt.Sprintf("%s:%d", strings.TrimSpace(match.Stream), match.Seq)
	intent.ClaimedAt = &claimedAt
	intent.AheadPayload = payload
	snapshot, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return output, err
	}
	snapshot.Waits = append(snapshot.Waits, intent)
	snapshot.Events = append(snapshot.Events,
		GenerationLifecycleEventIntent{
			Kind: GenerationLifecycleEventNodeWaitStarted, NodeID: output.NodeID,
			ItemIndex: output.ItemIndex, Attempt: output.Attempt, IssuedEpoch: output.Epoch,
			WaitKind: intent.Kind,
		},
		GenerationLifecycleEventIntent{
			Kind: GenerationLifecycleEventNodeWaitResumed, NodeID: output.NodeID,
			ItemIndex: output.ItemIndex, Attempt: output.Attempt, IssuedEpoch: output.Epoch,
			WaitKind: intent.Kind, ActorKind: intent.ClaimedByKind, ActorID: intent.ClaimedByID,
		},
	)
	plan.Snapshot.Payload = snapshot
	return output, nil
}

func buildNodeWaitIntent(
	node dsl.Node,
	output GenerationOutput,
	params dsl.WaitParams,
	now time.Time,
) (NodeWaitIntent, error) {
	intent := NodeWaitIntent{
		NodeID: NodeID(output.NodeID), ItemIndex: output.ItemIndex,
		CreatedAt: now.UTC(),
	}
	if len(params.Expect) > 0 {
		expect, err := json.Marshal(params.Expect)
		if err != nil {
			return NodeWaitIntent{}, fmt.Errorf("loop: marshal wait expect for %q: %w", node.ID, err)
		}
		intent.Expect = expect
	}
	switch {
	case strings.TrimSpace(params.For) != "":
		duration, err := time.ParseDuration(strings.TrimSpace(params.For))
		if err != nil {
			return NodeWaitIntent{}, fmt.Errorf("%w: wait node %q for: %v", ErrValidation, node.ID, err)
		}
		resumeAt := now.UTC().Add(duration)
		intent.Kind = NodeWaitKindTimer
		intent.ResumeAt = &resumeAt
	case strings.TrimSpace(params.Until) != "":
		resumeAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(params.Until))
		if err != nil {
			return NodeWaitIntent{}, fmt.Errorf(
				"%w: wait node %q until must be RFC3339: %v",
				ErrValidation,
				node.ID,
				err,
			)
		}
		resumeAt = resumeAt.UTC()
		intent.Kind = NodeWaitKindTimer
		intent.ResumeAt = &resumeAt
	case params.Event != nil:
		intent.Kind = NodeWaitKindEvent
	default:
		return NodeWaitIntent{}, fmt.Errorf("%w: wait node %q has no discriminator", ErrValidation, node.ID)
	}
	if params.Expires != nil {
		duration, err := time.ParseDuration(strings.TrimSpace(params.Expires.After))
		if err != nil {
			return NodeWaitIntent{}, fmt.Errorf("%w: wait node %q expiry: %v", ErrValidation, node.ID, err)
		}
		next := now.UTC().Add(duration)
		intent.NextEscalationAt = &next
	}
	return intent, nil
}
