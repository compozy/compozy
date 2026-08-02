package deadentity

import (
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// ForgetEntity retires one entity's in-memory state without changing durable state.
func (s *Service) ForgetEntity(key store.DeadEntityKey) {
	if s == nil {
		return
	}
	normalized := key.Normalize()
	if normalized.Validate() != nil {
		return
	}
	for {
		s.mu.Lock()
		state := s.states[normalized]
		s.mu.Unlock()
		if state == nil {
			return
		}

		state.mu.Lock()
		s.mu.Lock()
		if s.states[normalized] != state {
			s.mu.Unlock()
			state.mu.Unlock()
			continue
		}
		state.entityGeneration++
		delete(s.states, normalized)
		s.releaseWorkspaceStateIfUnusedLocked(normalized.WorkspaceID)
		s.mu.Unlock()
		state.mu.Unlock()
		return
	}
}

// ForgetWorkspace retires one workspace's in-memory dead-entity cache.
func (s *Service) ForgetWorkspace(workspaceID string) {
	if s == nil {
		return
	}
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" {
		return
	}
	s.mu.Lock()
	delete(s.workspaceStates, trimmed)
	for key := range s.states {
		if key.WorkspaceID == trimmed {
			delete(s.states, key)
		}
	}
	s.mu.Unlock()
}

func (s *Service) releaseWorkspaceStateIfUnusedLocked(workspaceID string) {
	for key := range s.states {
		if key.WorkspaceID == workspaceID {
			return
		}
	}
	delete(s.workspaceStates, workspaceID)
}
