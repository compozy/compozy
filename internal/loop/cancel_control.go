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
	Kind        RunCancelKind
	Reason      string
	Actor       task.ActorContext
	RequestedAt time.Time
	Effects     []RenderedEffectIntent
}

// Validate rejects cancellation commands that cannot preserve ownership and provenance.
func (m CancellationMutation) Validate(nodeRequired bool) error {
	if strings.TrimSpace(string(m.WorkspaceID)) == "" || strings.TrimSpace(string(m.RunID)) == "" ||
		(nodeRequired && strings.TrimSpace(string(m.NodeID)) == "") ||
		(m.Kind != RunCancelCancel && m.Kind != RunCancelKill) || m.RequestedAt.IsZero() {
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
	AdvanceRunCancellation(context.Context, CancellationMutation, CancelState) (CancellationResult, error)
	RequestNodeCancellation(context.Context, CancellationMutation) (CancellationResult, error)
	AdvanceNodeCancellation(context.Context, CancellationMutation, CancelState) (CancellationResult, error)
}

// CancellationSessionController performs post-commit prompt cancellation or immediate process stop.
type CancellationSessionController interface {
	CancelLoopSession(context.Context, string, string) error
	KillLoopSession(context.Context, string, string) error
}

// CancellationSessionControllerFuncs adapts cancellation callbacks.
type CancellationSessionControllerFuncs struct {
	Cancel func(context.Context, string, string) error
	Kill   func(context.Context, string, string) error
}

// CancelLoopSession implements CancellationSessionController.
func (f CancellationSessionControllerFuncs) CancelLoopSession(ctx context.Context, id, reason string) error {
	if f.Cancel == nil {
		return nil
	}
	return f.Cancel(ctx, id, reason)
}

// KillLoopSession implements CancellationSessionController.
func (f CancellationSessionControllerFuncs) KillLoopSession(ctx context.Context, id, reason string) error {
	if f.Kill == nil {
		return nil
	}
	return f.Kill(ctx, id, reason)
}
