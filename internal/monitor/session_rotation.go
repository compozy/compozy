package monitor

import "errors"

var ErrSessionHasMonitors = errors.New("monitor: session has live monitors")

// RetireSession serializes publication with monitor creation and refuses later monitors for the predecessor.
func (s *Service) RetireSession(sessionID string, publish func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return ErrStopped
	}
	for _, item := range s.sessions[sessionID] {
		if item.occupies() {
			return ErrSessionHasMonitors
		}
	}
	if err := publish(); err != nil {
		return err
	}
	if s.retired == nil {
		s.retired = make(map[string]struct{})
	}
	s.retired[sessionID] = struct{}{}
	return nil
}
