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
	if err := s.afterPause(ctx, *result.Paused); err != nil {
		s.mu.Lock()
		s.paused[result.Paused.ID] = result.Paused
		s.mu.Unlock()
		return err
	}
	return nil
}

func (s *CheckService) flushPauses(ctx context.Context) error {
	s.mu.Lock()
	paused := make([]*agentinstance.Instance, 0, len(s.paused))
	for _, instance := range s.paused {
		paused = append(paused, instance)
	}
	s.mu.Unlock()
	for _, instance := range paused {
		if err := s.afterPause(ctx, *instance); err != nil {
			return err
		}
		s.mu.Lock()
		if s.paused[instance.ID] == instance {
			delete(s.paused, instance.ID)
		}
		s.mu.Unlock()
	}
	return nil
}
