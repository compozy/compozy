package loop

import (
	"fmt"
	"time"
)

// RetryDueCell is one durable retry schedule selected by the scheduler backstop.
type RetryDueCell struct {
	WorkspaceID   WorkspaceID
	LoopRunID     RunID
	Generation    int
	NodeID        NodeID
	ItemIndex     int
	Attempt       int
	Epoch         int64
	NextAttemptAt time.Time
}

// RetryDueCursor is the stable composite cursor for retry due-time paging.
type RetryDueCursor struct {
	NextAttemptAt time.Time
	LoopRunID     RunID
	Generation    int
	NodeID        NodeID
	ItemIndex     int
}

// Empty reports whether the cursor is the initial page position.
func (c RetryDueCursor) Empty() bool {
	return c.NextAttemptAt.IsZero()
}

// RetryWakeIdempotencyKey is shared by the durable scan and opportunistic timer paths.
func RetryWakeIdempotencyKey(cell RetryDueCell) string {
	return fmt.Sprintf(
		"loop.retry.%s.%d.%s.%d.a%d.e%d",
		cell.LoopRunID,
		cell.Generation,
		cell.NodeID,
		cell.ItemIndex,
		cell.Attempt,
		cell.Epoch,
	)
}
