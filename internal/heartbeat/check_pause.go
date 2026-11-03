package heartbeat

import (
	"context"

	"github.com/compozy/compozy/internal/agentinstance"
)

func (s *CheckService) SetPauseHandler(handler func(context.Context, agentinstance.Instance) error) {
	s.afterPause = handler
}

func (s *CheckService) finish(ctx context.Context, req FinishRequest) error {
	result, err := s.store.Finish(ctx, req)
	if err != nil {
		return err
	}
	if result.Paused == nil || s.afterPause == nil {
		return nil
	}
	// Instance maintenance retries effects from persisted instance state, including after restart.
	return s.afterPause(ctx, *result.Paused)
}
