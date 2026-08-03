package loop

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func coordinatorNodeMetadataWithFanOutItem(
	run Run,
	generation int,
	node dsl.Node,
	itemIndex int,
	attempt int,
	epoch int64,
	item any,
	hasItem bool,
) (json.RawMessage, error) {
	payload := map[string]any{
		metadataLoopRunIDKey:      string(run.ID),
		"loop_name":               run.LoopName,
		"workspace_id":            string(run.WorkspaceID),
		metadataGenerationKey:     generation,
		metadataNodeIDKey:         string(node.ID),
		"node_class":              string(node.Class),
		"node_kind":               node.Kind,
		metadataItemIndexKey:      itemIndex,
		"index":                   itemIndex,
		watchEventsPayloadAttempt: attempt,
		"epoch":                   epoch,
	}
	if hasItem {
		payload["item"] = item
	}
	actionKind := dsl.ActionKind(node.Kind)
	switch actionKind {
	case dsl.ActionRunAgent:
		handle := actionSessionHandle(node.Session)
		payload["session_handle"] = actionSessionSharedKey(generation, node.ID, itemIndex, handle)
	case dsl.ActionGoal:
		handle, err := dsl.DeriveGoalHandle(node.ID, actionSessionHandle(node.Session), itemIndex)
		if err != nil {
			return nil, fmt.Errorf("loop: derive Goal session handle for %s: %w", node.ID, err)
		}
		payload["session_handle"] = handle
	}
	if actionKind == dsl.ActionGoal {
		payload["goal_segment_epoch"] = initialGoalSegmentEpoch
	}
	metadata, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("loop: marshal node metadata for %s: %w", node.ID, err)
	}
	return metadata, nil
}
