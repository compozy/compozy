package session

import (
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

func normalizeExpiredSessionAttach(info *Info, now time.Time) *Info {
	if info == nil {
		return nil
	}
	normalized := *info
	normalized.AttachExpiresAt = cloneSessionTimePtr(info.AttachExpiresAt)
	if strings.TrimSpace(normalized.AttachedTo) == "" || normalized.AttachExpiresAt == nil {
		normalized.AttachedTo = ""
		normalized.AttachExpiresAt = nil
		return &normalized
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !normalized.AttachExpiresAt.After(now.UTC()) {
		normalized.AttachedTo = ""
		normalized.AttachExpiresAt = nil
	}
	return &normalized
}

func (s *Session) applyAttach(attach store.SessionAttach) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AttachedTo = strings.TrimSpace(attach.AttachedTo)
	s.AttachExpiresAt = cloneSessionTimePtr(&attach.AttachExpiresAt)
	if attach.AttachedAt.After(s.UpdatedAt) {
		s.UpdatedAt = attach.AttachedAt.UTC()
	}
}
