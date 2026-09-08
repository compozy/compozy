package session

import "time"

// AttentionClass groups badges by the operator action they require.
type AttentionClass string

const (
	AttentionNeedsYou AttentionClass = "needs-you"
	AttentionFinished AttentionClass = "finished"
	AttentionNone     AttentionClass = "none"
)

// AttentionEvent is the post-commit edge emitted by the canonical attention store.
type AttentionEvent struct {
	SessionID   string
	ProfileID   string
	WorkspaceID string
	From        Badge
	To          Badge
	Class       AttentionClass
	At          time.Time
}

// ClassForBadge returns the stable attention class shared by every surface.
func ClassForBadge(badge Badge) AttentionClass {
	switch badge {
	case BadgeWaitingForAuth, BadgeWaitingForInput, BadgeFailed, BadgeNeedsAttention:
		return AttentionNeedsYou
	case BadgeDone:
		return AttentionFinished
	default:
		return AttentionNone
	}
}
