package agentmemory

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/redact"
)

type SearchResult struct {
	SessionID string  `json:"session_id,omitempty"`
	Source    string  `json:"source"`
	File      string  `json:"file"`
	Line      int     `json:"line"`
	Text      string  `json:"text"`
	Score     float64 `json:"score"`
}

func (s *Store) Search(key Key, query string) ([]SearchResult, error) {
	defer s.lock(key)()
	dir, err := s.directory(key)
	if err != nil {
		return nil, err
	}
	terms := strings.Fields(strings.ToLower(query))
	results := make([]SearchResult, 0)
	if len(terms) == 0 {
		return results, nil
	}
	for _, file := range []string{"MEMORY.md", "archive.md"} {
		content, err := readFile(filepath.Join(dir, file))
		if err != nil {
			return nil, err
		}
		for i, line := range lines(redact.String(content)) {
			text := strings.ToLower(line)
			matched := 0
			for _, term := range terms {
				if strings.Contains(text, term) {
					matched++
				}
			}
			if matched > 0 {
				results = append(
					results,
					SearchResult{
						Source: "memory",
						File:   file,
						Line:   i + 1,
						Text:   line,
						Score:  float64(matched) / float64(len(terms)),
					},
				)
			}
		}
	}
	slices.SortStableFunc(results, func(a, b SearchResult) int {
		if a.Score > b.Score {
			return -1
		}
		if a.Score < b.Score {
			return 1
		}
		return 0
	})
	return results, nil
}

// Replace rewrites every matching line.
func (s *Store) Replace(key Key, match, text string) (Document, error) {
	return s.replace(key, match, text, false)
}

func (s *Store) replace(key Key, match, text string, guardRules bool) (Document, error) {
	if strings.TrimSpace(text) == "" || strings.ContainsAny(text, "\r\n") {
		return Document{}, errors.New("invalid memory entry")
	}
	if utf8.RuneCountInString(text) > MaxEntryCharacters {
		return Document{}, ErrEntryTooLarge
	}
	return s.update(key, func(old string) (string, error) {
		if match == "" {
			return "", errors.New("memory match is required")
		}
		all := lines(old)
		for i, line := range all {
			if strings.Contains(line, match) {
				replacement := redact.String(strings.ReplaceAll(line, match, text))
				if entryCharacters(replacement) > MaxEntryCharacters {
					return "", ErrEntryTooLarge
				}
				all[i] = replacement
			}
		}
		return join(all), nil
	}, true, guardRules)
}

// Remove deletes every matching line.
func (s *Store) Remove(key Key, match string) (Document, int, error) {
	return s.remove(key, match, false)
}

func (s *Store) remove(key Key, match string, guardRules bool) (Document, int, error) {
	removed := 0
	doc, err := s.update(key, func(old string) (string, error) {
		if match == "" {
			return "", errors.New("memory match is required")
		}
		kept := make([]string, 0)
		for _, line := range lines(old) {
			if strings.Contains(line, match) {
				removed++
				continue
			}
			kept = append(kept, line)
		}
		return join(kept), nil
	}, true, guardRules)
	if err != nil {
		return Document{}, 0, err
	}
	return doc, removed, nil
}

// AgentWriter is the store as an agent may write it. Every method keeps the
// operator rules exactly as they are and fails with ErrRuleProtected otherwise.
type AgentWriter struct{ store *Store }

// AsAgent returns the write surface for a caller that acts as the agent itself.
func (s *Store) AsAgent() AgentWriter { return AgentWriter{store: s} }

func (w AgentWriter) Add(key Key, now time.Time, scope, text, until string) (Document, error) {
	return w.store.Add(key, now, scope, text, until)
}

func (w AgentWriter) Replace(key Key, match, text string) (Document, error) {
	return w.store.replace(key, match, text, true)
}

func (w AgentWriter) Remove(key Key, match string) (Document, int, error) {
	return w.store.remove(key, match, true)
}

func (w AgentWriter) Write(key Key, content, expectedDigest string) (Document, error) {
	return w.store.write(key, content, expectedDigest, true)
}

func (w AgentWriter) Rollback(key Key, revision string) (Document, error) {
	return w.store.rollback(key, revision, true)
}
