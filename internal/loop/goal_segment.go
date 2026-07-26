package loop

import (
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
)

const initialGoalSegmentEpoch int64 = 1

// GoalNodeTaskID returns the stable task identity shared by every segment of one Goal node instance.
func GoalNodeTaskID(loopRunID RunID, generation int, nodeID dsl.NodeID, itemIndex int) string {
	return coordinatorNodeTaskID(loopRunID, generation, nodeID, itemIndex)
}

// GoalSegmentRunID returns the epoch-scoped worker run identity for one Goal segment.
func GoalSegmentRunID(
	loopRunID RunID,
	generation int,
	nodeID dsl.NodeID,
	itemIndex int,
	controlEpoch int64,
) string {
	return fmt.Sprintf(
		"%s:goal-segment:%d",
		coordinatorNodeRunID(loopRunID, generation, nodeID, itemIndex),
		controlEpoch,
	)
}

// GoalSegmentIdempotencyKey returns the epoch-scoped deduplication key for one Goal segment.
func GoalSegmentIdempotencyKey(
	loopRunID RunID,
	generation int,
	nodeID dsl.NodeID,
	itemIndex int,
	controlEpoch int64,
) string {
	return fmt.Sprintf(
		"%s:goal-segment:%d",
		coordinatorNodeIdempotencyKey(loopRunID, generation, nodeID, itemIndex),
		controlEpoch,
	)
}
