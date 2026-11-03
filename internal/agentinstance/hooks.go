package agentinstance

import (
	"context"
	"log/slog"

	"github.com/compozy/compozy/internal/hooks"
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

func WithPauseEffects(effects func(context.Context, Instance) error) Option {
	return func(s *Service) { s.pauseEffects = effects }
}

func (s *Service) AfterPause(ctx context.Context, id string) error {
	instance, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.ApplyPause(ctx, instance)
}

func (s *Service) ApplyPause(ctx context.Context, instance Instance) error {
	if instance.State != "paused" || instance.ArchivedAt != nil {
		return nil
	}
	var effectsErr error
	if s.pauseEffects != nil {
		effectsErr = s.pauseEffects(ctx, instance)
	}
	s.notifyLifecycle(ctx, hooks.HookAgentInstancePaused, instance)
	return effectsErr
}
