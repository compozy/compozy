package session

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/acp"
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
	s.currentPromptCancelTurn = ""
	if s.currentTurnID != "" && s.currentPromptDone == nil {
		s.currentPromptDone = make(chan struct{})
	}
}

func (s *Session) currentPromptCompletion() <-chan struct{} {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentPromptDone
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
	s.promptCancelRequested = false
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

func (s *Session) requestCurrentPromptCancellation() (
	string,
	*AgentProcess,
	context.CancelFunc,
	bool,
	bool,
) {
	if s == nil {
		return "", nil, nil, false, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.promptSetupCount == 0 && s.currentTurnSource == "" && s.currentTurnID == "" {
		return "", nil, nil, false, false
	}
	turnID := strings.TrimSpace(s.currentTurnID)
	if s.promptCancelRequested {
		return turnID, s.process, nil, true, true
	}
	s.promptCancelRequested = true
	if turnID != "" {
		s.currentPromptCancelTurn = turnID
	}
	return turnID, s.process, s.currentPromptCancel, true, false
}

func (s *Session) retryCurrentPromptCancellation(turnID string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(turnID) == strings.TrimSpace(s.currentTurnID) {
		s.promptCancelRequested = false
	}
}

func (s *Session) promptCancellationRequested(turnID string) bool {
	if s == nil {
		return false
	}

	target := strings.TrimSpace(turnID)
	if target == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentPromptCancelTurn == target
}

func (s *Session) clearPromptCancellation(turnID string) {
	if s == nil {
		return
	}

	target := strings.TrimSpace(turnID)
	if target == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentPromptCancelTurn == target {
		s.currentPromptCancelTurn = ""
	}
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

func (s *Session) finishCurrentPromptCompletion() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentPromptDone != nil {
		close(s.currentPromptDone)
		s.currentPromptDone = nil
	}
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
	s.promptCancelRequested = false
}
