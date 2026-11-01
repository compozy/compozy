// Package agentmemory owns bounded, explicit agent notes.
package agentmemory

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxLines           = 200
	MaxBytes           = 24 * 1024
	MaxEntryCharacters = 1000
	TruncationMarker   = "[Memory truncated: undated notes exceed the load budget.]"
)

var (
	ErrBudgetExceeded = errors.New("agent_memory_budget_exceeded")
	ErrEntryTooLarge  = errors.New("agent_memory_entry_too_large")
	ErrConflict       = errors.New("agent_memory_conflict")
)

type Budget struct {
	Lines    int `json:"lines"`
	MaxLines int `json:"max_lines"`
	Bytes    int `json:"bytes"`
	MaxBytes int `json:"max_bytes"`
}

func Measure(content string) Budget {
	return Budget{Lines: len(lines(content)), MaxLines: MaxLines, Bytes: len(content), MaxBytes: MaxBytes}
}

func lines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

func fits(content string) bool {
	b := Measure(content)
	return b.Lines <= MaxLines && b.Bytes <= MaxBytes
}

func FormatEntry(now time.Time, scope, text, until string) (string, error) {
	if utf8.RuneCountInString(text) > MaxEntryCharacters {
		return "", ErrEntryTooLarge
	}
	if strings.TrimSpace(text) == "" || strings.TrimSpace(scope) == "" || strings.ContainsAny(text+scope, "\r\n") || strings.Contains(scope, " · ") {
		return "", errors.New("invalid memory entry")
	}
	entry := "- " + now.Format(time.DateOnly) + " · " + scope + " · " + text
	if until != "" {
		if _, err := time.Parse(time.DateOnly, until); err != nil {
			return "", errors.New("invalid memory expiry")
		}
		entry += " · until " + until
	}
	return entry, nil
}

// Overflow returns an archive plan; callers persist the archive before replacing memory.
func Overflow(content string) (string, string, error) {
	all := lines(content)
	type dated struct {
		index int
		date  string
	}
	candidates := make([]dated, 0)
	undated := make([]string, 0)
	for i, line := range all {
		parts := strings.SplitN(line, " · ", 3)
		if len(parts) == 3 && strings.HasPrefix(parts[0], "- ") {
			date := strings.TrimPrefix(parts[0], "- ")
			if _, err := time.Parse(time.DateOnly, date); err == nil {
				candidates = append(candidates, dated{i, date})
				continue
			}
		}
		undated = append(undated, line)
	}
	if !fits(join(undated)) {
		return content, "", ErrBudgetExceeded
	}
	slices.SortStableFunc(candidates, func(a, b dated) int { return strings.Compare(a.date, b.date) })
	removed := make(map[int]bool)
	archive := make([]string, 0)
	current := content
	for _, candidate := range candidates {
		if fits(current) {
			break
		}
		removed[candidate.index] = true
		archive = append(archive, all[candidate.index])
		kept := make([]string, 0, len(all)-len(removed))
		for i, line := range all {
			if !removed[i] {
				kept = append(kept, line)
			}
		}
		current = join(kept)
	}
	return current, join(archive), nil
}

func join(value []string) string {
	if len(value) == 0 {
		return ""
	}
	return strings.Join(value, "\n") + "\n"
}

// Snapshot bounds startup data without changing the operator's file.
func Snapshot(content string) string {
	if fits(content) {
		return content
	}
	result := ""
	for _, line := range lines(content) {
		next := result + line + "\n"
		if !fits(next + TruncationMarker) {
			break
		}
		result = next
	}
	return result + TruncationMarker
}

func entryCharacters(line string) int {
	parts := strings.SplitN(line, " · ", 3)
	if len(parts) == 3 && strings.HasPrefix(parts[0], "- ") {
		if _, err := time.Parse(time.DateOnly, strings.TrimPrefix(parts[0], "- ")); err == nil {
			line = parts[2]
			if text, expiry, ok := strings.Cut(line, " · until "); ok {
				if _, err := time.Parse(time.DateOnly, expiry); err == nil {
					line = text
				}
			}
		}
	}
	return utf8.RuneCountInString(line)
}
