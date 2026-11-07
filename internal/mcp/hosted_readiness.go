package mcp

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/tools"
)

var ErrHostedProjectionFailed = errors.New("mcp: hosted tool projection failed")

var errHostedBindPending = errors.New("mcp: hosted MCP bind is not ready")

// WaitForBind confirms the initial projection before the launch window expires.
func (s *HostedService) WaitForBind(ctx context.Context, sessionID string) error {
	_, err := s.WaitForBindToolDigest(ctx, sessionID)
	return err
}

// WaitForBindToolDigest returns the exact initial tools atomically with readiness.
func (s *HostedService) WaitForBindToolDigest(ctx context.Context, sessionID string) (string, error) {
	if err := ctxErr(ctx); err != nil {
		return "", err
	}
	if s == nil || !s.enabled {
		return "", ErrHostedDisabled
	}
	s.mu.Lock()
	launch := s.launches[sessionID]
	if launch == nil || !launch.armed {
		s.mu.Unlock()
		return "", ErrHostedNonceInvalid
	}
	s.mu.Unlock()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		changed := launch.readiness.wait()
		s.mu.Lock()
		current := s.launches[sessionID] == launch
		ready := current && launch.established && launch.readyBindID == launch.bindID && s.binds[launch.bindID] != nil
		var digest string
		if ready {
			digest = s.binds[launch.bindID].initialToolDigest
		}
		failed := launch.projectionFailed
		remaining := launch.expiresAt.Sub(s.now().UTC())
		s.mu.Unlock()
		if !current {
			return "", ErrHostedBindNotFound
		}
		if failed {
			return "", ErrHostedProjectionFailed
		}
		if ready {
			return digest, nil
		}
		if remaining <= 0 {
			return "", ErrHostedBindTimeout
		}
		timer.Reset(remaining)
		select {
		case <-changed:
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timer.C:
			// Recheck readiness and the deadline; a released bind may renew it.
		}
	}
}

// acknowledgeHostedBind confirms receipt of the bootstrap and successful full projection.
func (s *HostedService) acknowledgeHostedBind(record *hostedBindRecord) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	launch := s.launches[record.sessionID]
	if launch == nil || launch.bindID != record.bindID || !s.ownsBindRecord(record) ||
		!s.now().UTC().Before(launch.expiresAt) {
		return false
	}
	first := launch.readyBindID != record.bindID
	launch.projectionFailed = false
	launch.readyBindID = record.bindID
	launch.readiness.notify(nil)
	return first
}

// Preserve only a safe failure marker: upstream errors may contain credentials.
func (s *HostedService) failHostedProjection(record *hostedBindRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if launch := s.launches[record.sessionID]; launch != nil && launch.bindID == record.bindID &&
		s.ownsBindRecord(record) && launch.readyBindID != record.bindID {
		launch.projectionFailed = true
		launch.readiness.notify(nil)
	}
}

// InitialToolDigest reads an established bind without waiting for hosts that connect on first prompt.
func (s *HostedService) InitialToolDigest(sessionID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	launch := s.launches[sessionID]
	if launch == nil || launch.readyBindID == "" || launch.readyBindID != launch.bindID {
		return "", false
	}
	record := s.binds[launch.bindID]
	if record == nil {
		return "", false
	}
	return record.initialToolDigest, true
}

// ToolDigest is an internal turn-boundary check for providers that cache tools.
// It hashes the projected MCP views, never credentials or native tool state.
func (s *HostedService) ToolDigest(ctx context.Context, sessionID string) (string, error) {
	s.mu.Lock()
	launch := s.launches[sessionID]
	var record *hostedBindRecord
	ready := false
	if launch != nil {
		record = s.binds[launch.bindID].clone()
		ready = launch.readyBindID == launch.bindID
	}
	s.mu.Unlock()
	if record == nil {
		return "", ErrHostedBindNotFound
	}
	if !ready {
		return "", errHostedBindPending
	}
	registry := s.currentRegistry()
	if registry == nil {
		return "", ErrHostedRegistryRequired
	}
	projection, err := s.projectionForGeneration(ctx, record, registry, buildHostedProjection)
	if err != nil {
		return "", ErrHostedProjectionFailed
	}
	return hostedExternalToolDigest(projection.Tools), nil
}

func hostedExternalToolDigest(views []tools.ToolView) string {
	var external []tools.ToolView
	for idx := range views {
		view := &views[idx]
		if view.Descriptor.Source.Kind == tools.SourceMCP {
			external = append(external, *view)
		}
	}
	return hostedProjectionDigest(external)
}
