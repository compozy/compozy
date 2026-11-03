package agentinstance

import (
	"context"
	"log/slog"

	"github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
)

type LifecycleHooks interface {
	DispatchAgentInstance(context.Context, hooks.AgentInstancePayload) (hooks.AgentInstancePayload, error)
}

func WithLifecycleHooks(dispatcher LifecycleHooks) Option {
	return func(s *Service) { s.hooks = dispatcher }
}

func (s *Service) notifyLifecycle(ctx context.Context, event hooks.HookEvent, instance Instance) {
	if s.changed != nil {
		s.changed(instance)
	}
	if s.hooks == nil {
		return
	}
	_, err := s.hooks.DispatchAgentInstance(ctx, hooks.AgentInstancePayload{
		Event:         event,
		Timestamp:     s.now(),
		ProfileID:     instance.ProfileID,
		WorkspaceID:   instance.WorkspaceID,
		AgentName:     instance.AgentName,
		SessionID:     instance.MainSessionID,
		State:         instance.State,
		CreatedAt:     instance.CreatedAt,
		UpdatedAt:     instance.UpdatedAt,
		InstanceID:    instance.ID,
		MainSessionID: instance.MainSessionID,
		Reason:        instance.PauseReason,
	})
	if err != nil {
		slog.ErrorContext(
			ctx,
			"instance lifecycle hook failed",
			"instance_id",
			instance.ID,
			"hook_event",
			event,
			"error",
			err,
		)
	}
}

// WithStateEffects reconciles durable pause/resume obligations through their owning services.
func WithStateEffects(effects func(context.Context, Instance) error) Option {
	return func(s *Service) { s.stateEffects = effects }
}

func (s *Service) AfterPause(ctx context.Context, trip store.InstancePauseTrip) error {
	return s.ApplyPause(ctx, Instance{
		ID: trip.InstanceID, ProfileID: trip.ProfileID, WorkspaceID: trip.WorkspaceID,
		AgentName: trip.AgentName, MainSessionID: trip.MainSessionID, State: "paused", PauseReason: trip.Reason,
		CreatedAt: trip.CreatedAt, UpdatedAt: trip.PausedAt,
	})
}

func (s *Service) ApplyPause(ctx context.Context, instance Instance) error {
	if instance.State != "paused" || instance.ArchivedAt != nil {
		return nil
	}
	// Hooks describe the committed transition. Effects reconcile current state so a concurrent
	// resume cannot be undone by a delayed post-commit callback.
	s.notifyLifecycle(ctx, hooks.HookAgentInstancePaused, instance)
	return s.reconcileEffects(ctx, instance.ID)
}

func (s *Service) reconcileEffects(ctx context.Context, id string) error {
	if s.stateEffects == nil {
		return nil
	}
	current, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.stateEffects(ctx, current)
}
