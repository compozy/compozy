package session

import (
	"context"
	"strings"

	"github.com/compozy/agh/internal/acp"
)

// CurrentTurnSource reports the provenance of the currently active prompt turn.
func (s *Session) CurrentTurnSource() TurnSource {
	if s == nil {
		return ""
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentTurnSource
}

// CurrentTurnID reports the active prompt turn identifier.
func (s *Session) CurrentTurnID() string {
	if s == nil {
		return ""
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentTurnID
}

// CurrentPromptMeta reports the normalized metadata for the currently active prompt turn.
func (s *Session) CurrentPromptMeta() acp.PromptMeta {
	if s == nil {
		return acp.PromptMeta{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentPromptMeta.Normalize()
}

// CurrentPromptMessage reports the authored text of the active prompt turn.
func (s *Session) CurrentPromptMessage() string {
	if s == nil {
		return ""
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentPromptMessage
}

// IsPrompting reports whether the session currently has prompt setup or turn
// execution in flight.
func (s *Session) IsPrompting() bool {
	if s == nil {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.promptSetupCount > 0 || s.currentTurnSource != "" || s.currentTurnID != ""
}

func (s *Session) isCurrentPromptAgentWaiting() bool {
	if s == nil {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.promptSetupCount > 0 || s.currentTurnSource == "" || s.currentTurnID == "" {
		return false
	}
	return s.Liveness != nil &&
		s.Liveness.Activity != nil &&
		strings.TrimSpace(s.Liveness.Activity.LastActivityKind) == runtimeActivityKindAgentWaiting
}

func (s *Session) setCurrentTurnID(turnID string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentTurnID = strings.TrimSpace(turnID)
}

func (s *Session) setCurrentTurnSource(source TurnSource) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentTurnSource = normalizeTurnSource(source)
}

func (s *Session) setCurrentPromptMeta(meta acp.PromptMeta) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentPromptMeta = meta.Normalize()
}

func (s *Session) setCurrentPromptMessage(message string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentPromptMessage = strings.TrimSpace(message)
}

func (s *Session) setCurrentPromptCancel(cancel context.CancelFunc) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentPromptCancel = cancel
}

func (s *Session) cancelCurrentPrompt() bool {
	if s == nil {
		return false
	}

	s.mu.RLock()
	cancel := s.currentPromptCancel
	s.mu.RUnlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (s *Session) clearCurrentTurnSource() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentTurnSource = ""
}

func (s *Session) clearCurrentTurnID() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentTurnID = ""
}

func (s *Session) clearCurrentPromptMeta() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentPromptMeta = acp.PromptMeta{}
}

func (s *Session) clearCurrentPromptMessage() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentPromptMessage = ""
}

func (s *Session) clearCurrentPromptCancel() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentPromptCancel = nil
}
