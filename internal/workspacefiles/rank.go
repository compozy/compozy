package workspacefiles

import (
	"cmp"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

// Match tiers, best first.
const (
	tierExactName = iota
	tierNamePrefix
	tierNameSubstring
	tierNameSubsequence
	tierPathSubstring
	tierPathSubsequence
	tierNoMatch
)

// indexedEntry is an entry plus its precomputed ranking keys.
type indexedEntry struct {
	entry     Entry
	lowerName string
	lowerPath string
	depth     int
}

// snapshot is an immutable index of one root.
type snapshot struct {
	entries   []indexedEntry
	truncated bool
}

// newSnapshot builds file entries and derives one folder entry per distinct parent directory.
// A listed path ending in "/" (a symlink to a directory inside the root) is a folder entry.
func newSnapshot(files []string, truncated bool) *snapshot {
	entries := make([]indexedEntry, 0, len(files)+len(files)/4)
	folders := make(map[string]struct{})
	for _, file := range files {
		if folder, ok := strings.CutSuffix(file, "/"); ok {
			if _, seen := folders[folder]; !seen {
				folders[folder] = struct{}{}
				entries = append(entries, newIndexedEntry(file, path.Base(folder), KindFolder))
			}
			file = folder
		} else {
			entries = append(entries, newIndexedEntry(file, path.Base(file), KindFile))
		}
		for dir := path.Dir(file); dir != "." && dir != "/"; dir = path.Dir(dir) {
			if _, ok := folders[dir]; ok {
				break
			}
			folders[dir] = struct{}{}
			entries = append(entries, newIndexedEntry(dir+"/", path.Base(dir), KindFolder))
		}
	}
	return &snapshot{entries: entries, truncated: truncated}
}

func newIndexedEntry(entryPath string, name string, kind EntryKind) indexedEntry {
	trimmed := strings.TrimSuffix(entryPath, "/")
	return indexedEntry{
		entry:     Entry{Path: entryPath, Name: name, Kind: kind},
		lowerName: strings.ToLower(name),
		lowerPath: strings.ToLower(entryPath),
		depth:     strings.Count(trimmed, "/"),
	}
}

type rankedEntry struct {
	item *indexedEntry
	tier int
}

// rank returns at most limit entries ordered by tier, then depth, then path
// length, then path. An empty query orders every entry by depth, then path.
func (s *snapshot) rank(query string, limit int) []Entry {
	if limit <= 0 {
		return []Entry{}
	}
	normalized := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(query, "\\", "/")))
	nameQuery := strings.TrimSuffix(normalized, "/")
	ranked := make([]rankedEntry, 0, min(len(s.entries), 1024))
	for i := range s.entries {
		item := &s.entries[i]
		tier := tierExactName
		if normalized != "" {
			tier = matchTier(item, normalized, nameQuery)
		}
		if tier != tierNoMatch {
			ranked = append(ranked, rankedEntry{item: item, tier: tier})
		}
	}
	emptyQuery := normalized == ""
	slices.SortFunc(ranked, func(a, b rankedEntry) int {
		if order := cmp.Compare(a.tier, b.tier); order != 0 {
			return order
		}
		if order := cmp.Compare(a.item.depth, b.item.depth); order != 0 {
			return order
		}
		if !emptyQuery {
			if order := cmp.Compare(len(a.item.entry.Path), len(b.item.entry.Path)); order != 0 {
				return order
			}
		}
		return cmp.Compare(a.item.entry.Path, b.item.entry.Path)
	})
	result := make([]Entry, 0, min(limit, len(ranked)))
	for _, candidate := range ranked[:min(limit, len(ranked))] {
		result = append(result, candidate.item.entry)
	}
	return result
}

func matchTier(item *indexedEntry, query string, nameQuery string) int {
	switch {
	case nameQuery != "" && item.lowerName == nameQuery:
		return tierExactName
	case nameQuery != "" && strings.HasPrefix(item.lowerName, nameQuery):
		return tierNamePrefix
	case nameQuery != "" && strings.Contains(item.lowerName, nameQuery):
		return tierNameSubstring
	case nameQuery != "" && isSubsequence(item.lowerName, nameQuery):
		return tierNameSubsequence
	case strings.Contains(item.lowerPath, query):
		return tierPathSubstring
	case isSubsequence(item.lowerPath, query):
		return tierPathSubsequence
	default:
		return tierNoMatch
	}
}

// isSubsequence reports whether every rune of query appears in value in order.
func isSubsequence(value string, query string) bool {
	remaining := query
	for _, r := range value {
		if remaining == "" {
			return true
		}
		head, size := utf8.DecodeRuneInString(remaining)
		if r == head {
			remaining = remaining[size:]
		}
	}
	return remaining == ""
}
