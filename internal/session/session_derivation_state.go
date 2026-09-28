package session

import (
	"time"

	"github.com/compozy/compozy/internal/store"
)

// importedContextSnapshot returns an isolated copy of the child's carried context.
func (s *Session) importedContextSnapshot() *store.SessionImportedContext {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return store.CloneSessionImportedContext(s.importedContext)
}

// hasImportedContext reports whether a rebuild of this session must carry an imported context.
func (s *Session) hasImportedContext() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.importedContext != nil
}

// pendingRouteSnapshot returns the declared route chosen at derive time, if still pending.
func (s *Session) pendingRouteSnapshot() *store.SessionPendingRoute {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.derivation == nil {
		return nil
	}
	return store.CloneSessionPendingRoute(s.derivation.PendingRoute)
}

// clearPendingRoute drops the pending route once it bound or an explicit runtime replaced it.
func (s *Session) clearPendingRoute() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.derivation == nil || s.derivation.PendingRoute == nil {
		return false
	}
	s.derivation.PendingRoute = nil
	return true
}

// markImportedContextConsumed records the first dispatch that carried the imported
// context; later dispatches leave the record unchanged.
func (s *Session) markImportedContextConsumed(admissionKey string, messageID string, at time.Time) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.importedContext == nil || s.importedContext.Consumed != nil {
		return false
	}
	s.importedContext.Consumed = &store.SessionImportedContextConsumption{
		AdmissionKey: admissionKey,
		MessageID:    messageID,
		At:           at.UTC(),
	}
	return true
}

// takePendingDeriveReceipt returns the derive receipt once, for the child's commit.
func (s *Session) takePendingDeriveReceipt() *store.SessionDerivationReceipt {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	receipt := s.pendingDeriveReceipt
	s.pendingDeriveReceipt = nil
	return receipt
}
