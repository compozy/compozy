package loop

import (
	"context"
	"time"
)

// WaitDueCursor is the stable composite cursor for timer-wait paging.
type WaitDueCursor struct {
	ResumeAt   time.Time
	LoopRunID  RunID
	Generation int
	NodeID     NodeID
	ItemIndex  int
}

// Empty reports whether this is the initial page position.
func (c WaitDueCursor) Empty() bool {
	return c.ResumeAt.IsZero()
}

// WaitEscalationCursor is the stable composite cursor for wait-ladder paging.
type WaitEscalationCursor struct {
	NextEscalationAt time.Time
	LoopRunID        RunID
	Generation       int
	NodeID           NodeID
	ItemIndex        int
}

// Empty reports whether this is the initial page position.
func (c WaitEscalationCursor) Empty() bool {
	return c.NextEscalationAt.IsZero()
}

// WaitEscalationStore owns due ladder steps and terminal expiry transitions.
type WaitEscalationStore interface {
	EscalateDueLoopWaitsPage(
		context.Context,
		time.Time,
		WaitEscalationCursor,
		int,
	) (WaitEscalationCursor, error)
}
