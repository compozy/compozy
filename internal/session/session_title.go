package session

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/store"
)

const (
	automaticSessionTitleMaxRunes = 64
	automaticSessionTitleMaxWords = 8
)

var (
	// automaticTitleBlockPrefix matches Markdown block markers before a title.
	automaticTitleBlockPrefix = regexp.MustCompile(`^(?:#{1,6}\s+|>\s*|[-*+]\s+|\d+[.)]\s+)+`)
	// automaticTitleLabel matches the label a model may put before its title.
	automaticTitleLabel = regexp.MustCompile("(?i)^[*_`\"']*title[*_`\"']*\\s*:[*_`\"']*\\s*")
	// automaticTitleNoticeLine matches a line that opens as a notice, not a title.
	automaticTitleNoticeLine = regexp.MustCompile(
		"(?i)^[*_`~]*\\s*(?:notice|note|reminder|warning|disclaimer)\\s*[*_`~]*\\s*:",
	)
	// automaticTitleAppendix matches where text glued after a title begins: an
	// emphasized label, bold opened against a word, or a plain notice label after
	// a sentence end or against a word.
	automaticTitleAppendix = regexp.MustCompile(
		`(\*{1,2}[^*]+:\*{1,2}|\*\*[^*]+\*\*:|_{1,2}[^_]+:_{1,2})` +
			`|[\p{L}\p{N}](\*\*\p{Lu})` +
			`|(?:[\p{Ll}\p{N}]|[.!?]\s+)((?:Notice|Note|Reminder|Warning|Disclaimer)\s*:)`,
	)
	// automaticTitleFence matches an opening or closing code fence line.
	automaticTitleFence = regexp.MustCompile("^(?:`{3,}|~{3,})[\\w+-]*$")
)

const (
	automaticTitleEdgeMarkup = "*_`~"
	automaticTitleEdgeQuotes = "\"'“”‘’«»"
)

type sessionTitleClaim struct {
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
	var info *Info
	applied, err := func() (bool, error) {
		session.mu.Lock()
		defer session.mu.Unlock()

		claim, ok := session.claimAutomaticTitleLocked(title, m.now())
		if !ok {
			return false, nil
		}
		metaPath := session.metaPath
		meta := session.metaLocked()
		info = session.infoLocked()
		if err := m.persistSessionIdentitySnapshot(ctx, metaPath, &meta, info); err != nil {
			session.rollbackAutomaticTitleLocked(claim)
			rollbackMeta := session.metaLocked()
			if rollbackErr := store.WriteSessionMeta(metaPath, &rollbackMeta); rollbackErr != nil {
				return false, errors.Join(err, fmt.Errorf("session: roll back automatic title: %w", rollbackErr))
			}
			return false, err
		}
		return true, nil
	}()
	if err != nil || !applied {
		return applied, err
	}
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
	return true, nil
}

func (s *Session) claimAutomaticTitleLocked(candidate string, now time.Time) (sessionTitleClaim, bool) {
	if s == nil {
		return sessionTitleClaim{}, false
	}
	title := CleanAutomaticSessionTitle(candidate)
	if title == "" {
		return sessionTitleClaim{}, false
	}

	if normalizeSessionType(s.Type) != SessionTypeUser || strings.TrimSpace(s.Name) != "" {
		return sessionTitleClaim{}, false
	}
	claim := sessionTitleClaim{previousUpdatedAt: s.UpdatedAt}
	s.Name = title
	s.UpdatedAt = now.UTC()
	return claim, true
}

func (s *Session) rollbackAutomaticTitleLocked(claim sessionTitleClaim) {
	if s == nil {
		return
	}
	s.Name = ""
	s.UpdatedAt = claim.previousUpdatedAt
}

// CleanAutomaticSessionTitle reduces generator output to one stored title. It
// keeps the first line that holds title text, drops notices and text glued after
// the title, removes Markdown, and applies the title bounds. It returns "" when
// no line is usable, so the caller keeps the session's current name.
func CleanAutomaticSessionTitle(output string) string {
	lines := strings.FieldsFunc(output, func(r rune) bool { return r == '\n' || r == '\r' })
	for _, line := range lines {
		if title := normalizeAutomaticSessionTitle(cleanAutomaticTitleLine(line)); title != "" {
			return title
		}
	}
	return ""
}

// cleanAutomaticTitleLine repeats its pass until the line stops changing, so a
// title accepted by the generator cannot be rejected when the session applies it.
func cleanAutomaticTitleLine(line string) string {
	candidate := strings.TrimSpace(line)
	if automaticTitleFence.MatchString(candidate) {
		return ""
	}
	for {
		next := cleanAutomaticTitlePass(candidate)
		if next == candidate {
			return candidate
		}
		candidate = next
	}
}

func cleanAutomaticTitlePass(line string) string {
	candidate := automaticTitleBlockPrefix.ReplaceAllString(line, "")
	candidate = automaticTitleLabel.ReplaceAllString(candidate, "")
	candidate = strings.Trim(candidate, automaticTitleEdgeQuotes+" \t")
	if automaticTitleNoticeLine.MatchString(candidate) {
		return ""
	}
	if match := automaticTitleAppendix.FindStringSubmatchIndex(candidate); match != nil {
		for group := 1; group < len(match)/2; group++ {
			if start := match[2*group]; start >= 0 {
				candidate = candidate[:start]
				break
			}
		}
	}
	candidate = stripAutomaticTitleMarkup(candidate)
	if strings.HasSuffix(candidate, ":") {
		// A trailing colon introduces the answer on a later line.
		return ""
	}
	return candidate
}

// stripAutomaticTitleMarkup removes emphasis and code markers at word edges, so
// identifiers such as snake_case and arithmetic such as 5*3 survive.
func stripAutomaticTitleMarkup(candidate string) string {
	words := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(candidate, "**", ""), "`", ""))
	kept := words[:0]
	for _, word := range words {
		if word = strings.Trim(word, automaticTitleEdgeMarkup+automaticTitleEdgeQuotes); word != "" {
			kept = append(kept, word)
		}
	}
	return strings.Join(kept, " ")
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
	if !strings.ContainsFunc(title, isAutomaticTitleContent) {
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

func isAutomaticTitleContent(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}
