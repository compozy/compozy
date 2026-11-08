package heartbeat

import (
	"context"
	"log/slog"

	"github.com/compozy/compozy/internal/agentinstance"
)

func (s *CheckService) SetPickHandler(handler func(context.Context, []agentinstance.Pick) error) {
	s.afterPicks = handler
}

func (s *CheckService) SetPauseHandler(handler func(context.Context, agentinstance.Instance) error) {
	s.afterPause = handler
}

func (s *CheckService) finish(ctx context.Context, req FinishRequest) error {
	result, err := s.store.Finish(ctx, req)
	if err != nil {
		return err
	}
	if len(result.Picks) > 0 && s.afterPicks != nil {
		if err := s.afterPicks(ctx, result.Picks); err != nil {
			slog.WarnContext(ctx, "agent.instance.pick.effects_failed", "instance_id", req.InstanceID, "error", err)
		}
	}
	if result.Paused == nil || s.afterPause == nil {
		return nil
	}
	// Instance maintenance retries effects from persisted instance state, including after restart.
	return s.afterPause(ctx, *result.Paused)
}
