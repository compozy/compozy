package agentmemory

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
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
				results = append(results, SearchResult{Source: "memory", File: file, Line: i + 1, Text: line, Score: float64(matched) / float64(len(terms))})
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

func (s *Store) Replace(key Key, match, text string) (Document, error) {
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
				replacement := strings.ReplaceAll(line, match, text)
				if entryCharacters(replacement) > MaxEntryCharacters {
					return "", ErrEntryTooLarge
				}
				all[i] = replacement
			}
		}
		return join(all), nil
	}, true)
}

func (s *Store) Remove(key Key, match string) (Document, int, error) {
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
	}, true)
	if err != nil {
		return Document{}, 0, err
	}
	return doc, removed, nil
}
