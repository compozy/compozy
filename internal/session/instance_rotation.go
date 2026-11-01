package session

import (
	"context"

	"github.com/compozy/compozy/internal/store"
)

// PublishInstanceRotation holds the same lifecycle and turn locks as session admission.
func (m *Manager) PublishInstanceRotation(ctx context.Context, id string, publish func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	target, ok := m.Get(id)
	if !ok {
		return ErrSessionNotFound
	}
	return target.publishInstanceRotation(publish)
}

func (s *Session) publishInstanceRotation(publish func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rotationSuperseded {
		return store.ErrSessionSuperseded
	}
	if s.State != StateActive {
		return ErrSessionNotActive
	}
	if _, available := ResolveCompactionCommand(s.AdvertisedCommands); available {
		return ErrCompactionUnsupported
	}
	if s.isPromptingLocked() || len(s.PendingInteractions) > 0 {
		return ErrPromptInProgress
	}
	if err := publish(); err != nil {
		return err
	}
	s.rotationSuperseded = true
	return nil
}
