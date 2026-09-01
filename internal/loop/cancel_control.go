package loop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/task"
)

// CancellationMutation is the durable first-writer command for a Run or node.
type CancellationMutation struct {
	WorkspaceID WorkspaceID
	RunID       RunID
	NodeID      NodeID
	ItemIndex   *int
	Reason      string
	Actor       task.ActorContext
	RequestedAt time.Time
	Effects     []RenderedEffectIntent
}

// Validate rejects cancellation commands that cannot preserve ownership and provenance.
func (m CancellationMutation) Validate(nodeRequired bool) error {
	if strings.TrimSpace(string(m.WorkspaceID)) == "" || strings.TrimSpace(string(m.RunID)) == "" ||
		(nodeRequired && strings.TrimSpace(string(m.NodeID)) == "") ||
		(m.ItemIndex != nil && *m.ItemIndex < 0) ||
		m.RequestedAt.IsZero() {
		return fmt.Errorf("%w: cancellation mutation is incomplete", ErrValidation)
	}
	if err := m.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: cancellation actor: %w", ErrValidation, err)
	}
	for _, effect := range m.Effects {
		if err := effect.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// CancellationResult returns committed truth plus exact post-commit process identities.
type CancellationResult struct {
	Run                 Run
	Coordinator         *task.Run
	SessionIDs          []string
	RevokedPromptLeases []GoalPromptLease
	Terminal            bool
	Applied             bool
}

// CancellationStore owns atomic cancellation state, fencing, Goal cleanup, and terminal truth.
type CancellationStore interface {
	RequestRunCancellation(context.Context, CancellationMutation) (CancellationResult, error)
	RequestNodeCancellation(context.Context, CancellationMutation) (CancellationResult, error)
}

// CancellationSessionController performs the post-commit process stop.
type CancellationSessionController interface {
	StopLoopSession(context.Context, string, string) error
}

// CancellationSessionControllerFuncs adapts cancellation callbacks.
type CancellationSessionControllerFuncs struct {
	Stop func(context.Context, string, string) error
}

// StopLoopSession implements CancellationSessionController.
func (f CancellationSessionControllerFuncs) StopLoopSession(ctx context.Context, id, reason string) error {
	if f.Stop == nil {
		return nil
	}
	return f.Stop(ctx, id, reason)
}
