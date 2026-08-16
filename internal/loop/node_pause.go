package loop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/task"
)

// NodePauseMode controls what happens to an in-flight node attempt.
type NodePauseMode string

const (
	// NodePauseDrain lets an already-running attempt settle while parking future work.
	NodePauseDrain NodePauseMode = "drain"
	// NodePauseCancel requests cancellation of an already-running attempt after commit.
	NodePauseCancel NodePauseMode = "cancel"
)

// NodeResumeMode controls how parked retry state is restored.
type NodeResumeMode string

const (
	// NodeResumePlain restores the schedule and attempt counter that existed at pause time.
	NodeResumePlain NodeResumeMode = "plain"
	// NodeResumeResetAttempts clears retry delay and restarts the attempt counter.
	NodeResumeResetAttempts NodeResumeMode = "reset_attempts"
	// NodeResumeImmediate preserves the attempt counter but clears any retry delay.
	NodeResumeImmediate NodeResumeMode = "immediate"
)

// NodePauseMutation is the first-writer command for one authored node.
type NodePauseMutation struct {
	WorkspaceID      WorkspaceID
	RunID            RunID
	NodeID           NodeID
	ItemIndex        *int
	Mode             NodePauseMode
	Reason           string
	RuleID           string
	ExpectedRevision *int64
	Actor            task.ActorContext
	RequestedAt      time.Time
}

// NodeResumeMutation releases one paused node episode.
type NodeResumeMutation struct {
	WorkspaceID      WorkspaceID
	RunID            RunID
	NodeID           NodeID
	ItemIndex        *int
	Mode             NodeResumeMode
	ExpectedRevision *int64
	Actor            task.ActorContext
	RequestedAt      time.Time
}

// NodePauseResult reports committed truth and exact post-commit cancellation targets.
type NodePauseResult struct {
	Control    NodeControl
	SessionIDs []string
	Applied    bool
}

// NodeResumeResult reports committed truth and the coordinator wake, when one was needed.
type NodeResumeResult struct {
	Control     NodeControl
	Coordinator *task.Run
	Applied     bool
}

// NodePauseStore owns pause/resume CAS, cell fencing, events, and coordinator reservation.
type NodePauseStore interface {
	PauseNode(context.Context, NodePauseMutation) (NodePauseResult, error)
	ResumeNode(context.Context, NodeResumeMutation) (NodeResumeResult, error)
}

// Validate rejects pause requests that cannot preserve scope and provenance.
func (m NodePauseMutation) Validate() error {
	if strings.TrimSpace(string(m.WorkspaceID)) == "" || strings.TrimSpace(string(m.RunID)) == "" ||
		strings.TrimSpace(string(m.NodeID)) == "" || m.RequestedAt.IsZero() {
		return fmt.Errorf("%w: node pause identity is incomplete", ErrValidation)
	}
	if m.ItemIndex != nil && *m.ItemIndex < 0 {
		return fmt.Errorf("%w: node pause item_index must be non-negative", ErrValidation)
	}
	if m.Mode != NodePauseDrain && m.Mode != NodePauseCancel {
		return fmt.Errorf("%w: node pause mode is invalid: %q", ErrValidation, m.Mode)
	}
	if err := m.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: node pause actor: %w", ErrValidation, err)
	}
	if m.ExpectedRevision != nil && *m.ExpectedRevision < 0 {
		return fmt.Errorf("%w: expected node pause revision must be non-negative", ErrValidation)
	}
	return nil
}

// Validate rejects resume requests that cannot preserve scope and provenance.
func (m NodeResumeMutation) Validate() error {
	if strings.TrimSpace(string(m.WorkspaceID)) == "" || strings.TrimSpace(string(m.RunID)) == "" ||
		strings.TrimSpace(string(m.NodeID)) == "" || m.RequestedAt.IsZero() {
		return fmt.Errorf("%w: node resume identity is incomplete", ErrValidation)
	}
	if m.ItemIndex != nil && *m.ItemIndex < 0 {
		return fmt.Errorf("%w: node resume item_index must be non-negative", ErrValidation)
	}
	if m.Mode != NodeResumePlain && m.Mode != NodeResumeResetAttempts && m.Mode != NodeResumeImmediate {
		return fmt.Errorf("%w: node resume mode is invalid: %q", ErrValidation, m.Mode)
	}
	if err := m.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: node resume actor: %w", ErrValidation, err)
	}
	if m.ExpectedRevision != nil && *m.ExpectedRevision < 0 {
		return fmt.Errorf("%w: expected node resume revision must be non-negative", ErrValidation)
	}
	return nil
}
