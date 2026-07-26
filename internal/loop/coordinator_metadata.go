package loop

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
)

func coordinatorNodeMetadataWithFanOutItem(
	run Run,
	generation int,
	node dsl.Node,
	itemIndex int,
	item any,
	hasItem bool,
) (json.RawMessage, error) {
	payload := map[string]any{
		metadataLoopRunIDKey:  string(run.ID),
		"loop_name":           run.LoopName,
		"workspace_id":        string(run.WorkspaceID),
		metadataGenerationKey: generation,
		"node_id":             string(node.ID),
		"node_class":          string(node.Class),
		"node_kind":           node.Kind,
		"item_index":          itemIndex,
		"index":               itemIndex,
	}
	if hasItem {
		payload["item"] = item
	}
	if dsl.ActionKind(node.Kind) == dsl.ActionGoal {
		payload["goal_segment_epoch"] = initialGoalSegmentEpoch
	}
	metadata, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("loop: marshal node metadata for %s: %w", node.ID, err)
	}
	return metadata, nil
}
