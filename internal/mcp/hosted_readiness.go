package mcp

import (
	"context"
	"time"
)

// WaitForBind confirms the initial projection before the launch window expires.
func (s *HostedService) WaitForBind(ctx context.Context, sessionID string) error {
	if err := ctxErr(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	launch := s.launches[sessionID]
	if launch == nil || !launch.armed {
		s.mu.Unlock()
		return ErrHostedNonceInvalid
	}
	remaining := launch.expiresAt.Sub(s.now().UTC())
	s.mu.Unlock()
	timer := time.NewTimer(max(remaining, 0))
	defer timer.Stop()
	for {
		changed := launch.readiness.wait()
		s.mu.Lock()
		current := s.launches[sessionID] == launch
		ready := current && launch.established && launch.readyBindID == launch.bindID && s.binds[launch.bindID] != nil
		s.mu.Unlock()
		if !current {
			return ErrHostedBindNotFound
		}
		if ready {
			return nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return ErrHostedBindTimeout
		}
	}
}
