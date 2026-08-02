package loop

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func parkGateApprovalWait(
	eval *controlEvalContext,
	plan *task.CoordinatorCompletionPlan,
	node dsl.Node,
	output GenerationOutput,
) (GenerationOutput, error) {
	if eval == nil || plan == nil || eval.now.IsZero() {
		return GenerationOutput{}, fmt.Errorf("%w: gate approval wait context is required", ErrValidation)
	}
	now := eval.now.UTC()
	output.Epoch++
	intent := NodeWaitIntent{
		NodeID: NodeID(output.NodeID), ItemIndex: output.ItemIndex,
		Kind: NodeWaitKindApprovalEscalation, IssuedEpoch: output.Epoch, CreatedAt: now,
	}
	if node.Expires != nil {
		duration, err := time.ParseDuration(strings.TrimSpace(node.Expires.After))
		if err != nil {
			return GenerationOutput{}, fmt.Errorf("%w: gate node %q expiry: %v", ErrValidation, node.ID, err)
		}
		next := now.Add(duration)
		intent.NextEscalationAt = &next
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return GenerationOutput{}, err
	}
	payload.Waits = append(payload.Waits, intent)
	payload.Events = append(payload.Events, GenerationLifecycleEventIntent{
		Kind: GenerationLifecycleEventNodeWaitStarted, NodeID: output.NodeID,
		ItemIndex: output.ItemIndex, Attempt: max(1, output.Attempt), IssuedEpoch: output.Epoch,
		WaitKind: intent.Kind, NextAttemptAt: intent.NextEscalationAt,
	})
	plan.Snapshot.Payload = payload
	return output, nil
}
