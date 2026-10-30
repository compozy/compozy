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
	if s == nil || !s.enabled {
		return ErrHostedDisabled
	}
	s.mu.Lock()
	launch := s.launches[sessionID]
	if launch == nil || !launch.armed {
		s.mu.Unlock()
		return ErrHostedNonceInvalid
	}
	s.mu.Unlock()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		changed := launch.readiness.wait()
		s.mu.Lock()
		current := s.launches[sessionID] == launch
		ready := current && launch.established && launch.readyBindID == launch.bindID && s.binds[launch.bindID] != nil
		remaining := launch.expiresAt.Sub(s.now().UTC())
		s.mu.Unlock()
		if !current {
			return ErrHostedBindNotFound
		}
		if ready {
			return nil
		}
		if remaining <= 0 {
			return ErrHostedBindTimeout
		}
		timer.Reset(remaining)
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			// Recheck readiness and the deadline; a released bind may renew it.
		}
	}
}

// acknowledgeHostedBind uses the proxy's authenticated stream request as receipt
// confirmation: it can only name the bind after receiving its initial projection.
func (s *HostedService) acknowledgeHostedBind(record *hostedBindRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	launch := s.launches[record.sessionID]
	if launch == nil || launch.bindID != record.bindID || !s.ownsBindRecord(record) ||
		!s.now().UTC().Before(launch.expiresAt) {
		return
	}
	launch.readyBindID = record.bindID
	launch.readiness.notify(nil)
}
