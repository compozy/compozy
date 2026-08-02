package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/store"
)

const (
	automaticSessionTitleMaxRunes = 64
	automaticSessionTitleMaxWords = 8
)

type sessionTitleClaim struct {
	title             string
	previousUpdatedAt time.Time
}

// ApplyAutomaticSessionTitle persists one generated title when the user session
// is still unnamed. Explicit names and an earlier successful generation win.
func (m *Manager) ApplyAutomaticSessionTitle(
	ctx context.Context,
	sessionID string,
	title string,
) (bool, error) {
	if m == nil {
		return false, errors.New("session: manager is required")
	}
	if ctx == nil {
		return false, errors.New("session: automatic title context is required")
	}
	session, ok := m.Get(sessionID)
	if !ok {
		return false, nil
	}
	session.mu.Lock()
	claim, ok := session.claimAutomaticTitleLocked(title, m.now())
	if !ok {
		session.mu.Unlock()
		return false, nil
	}
	metaPath := session.metaPath
	meta := session.metaLocked()
	info := session.infoLocked()
	if err := m.persistSessionIdentitySnapshot(ctx, metaPath, meta, info); err != nil {
		session.rollbackAutomaticTitleLocked(claim)
		rollbackErr := store.WriteSessionMeta(metaPath, session.metaLocked())
		session.mu.Unlock()
		if rollbackErr != nil {
			return false, errors.Join(err, fmt.Errorf("session: roll back automatic title: %w", rollbackErr))
		}
		return false, err
	}
	session.mu.Unlock()
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
	return true, nil
}

func (s *Session) claimAutomaticTitleLocked(candidate string, now time.Time) (sessionTitleClaim, bool) {
	if s == nil {
		return sessionTitleClaim{}, false
	}
	title := normalizeAutomaticSessionTitle(candidate)
	if title == "" {
		return sessionTitleClaim{}, false
	}

	if normalizeSessionType(s.Type) != SessionTypeUser || strings.TrimSpace(s.Name) != "" {
		return sessionTitleClaim{}, false
	}
	claim := sessionTitleClaim{title: title, previousUpdatedAt: s.UpdatedAt}
	s.Name = title
	s.UpdatedAt = now.UTC()
	return claim, true
}

func (s *Session) rollbackAutomaticTitleLocked(claim sessionTitleClaim) {
	if s == nil {
		return
	}
	if s.Name != claim.title {
		return
	}
	s.Name = ""
	s.UpdatedAt = claim.previousUpdatedAt
}

func normalizeAutomaticSessionTitle(candidate string) string {
	words := strings.Fields(candidate)
	if len(words) == 0 {
		return ""
	}

	truncatedByWords := len(words) > automaticSessionTitleMaxWords
	if truncatedByWords {
		words = words[:automaticSessionTitleMaxWords]
	}
	title := strings.Trim(strings.Join(words, " "), " \t\n\r#>*_`-.,;:!?")
	if title == "" {
		return ""
	}
	truncated := truncatedByWords || utf8.RuneCountInString(title) > automaticSessionTitleMaxRunes
	if truncated {
		runes := []rune(title)
		if len(runes) >= automaticSessionTitleMaxRunes {
			title = strings.TrimSpace(string(runes[:automaticSessionTitleMaxRunes-1]))
		}
		title = strings.TrimRight(title, " \t\n\r.,;:!?") + "…"
	}
	return title
}
