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
	entries []indexedEntry
	// emptyOrder lists every entry in empty-query order (depth, then path), computed once per build.
	emptyOrder []*indexedEntry
	truncated  bool
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
	snap := &snapshot{entries: entries, truncated: truncated}
	snap.emptyOrder = make([]*indexedEntry, len(snap.entries))
	for i := range snap.entries {
		snap.emptyOrder[i] = &snap.entries[i]
	}
	slices.SortFunc(snap.emptyOrder, func(a, b *indexedEntry) int {
		return cmp.Or(cmp.Compare(a.depth, b.depth), cmp.Compare(a.entry.Path, b.entry.Path))
	})
	return snap
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
//
// The empty-query order is precomputed per snapshot; a query keeps only the best
// limit matches in a bounded max-heap and sorts just those, which yields the
// same entries as sorting every match because the order is total (paths are unique).
func (s *snapshot) rank(query string, limit int) []Entry {
	if limit <= 0 {
		return []Entry{}
	}
	normalized := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(query, "\\", "/")))
	if normalized == "" {
		ordered := s.emptyOrder[:min(limit, len(s.emptyOrder))]
		result := make([]Entry, 0, len(ordered))
		for _, item := range ordered {
			result = append(result, item.entry)
		}
		return result
	}
	nameQuery := strings.TrimSuffix(normalized, "/")
	best := make([]rankedEntry, 0, min(limit, len(s.entries)))
	for i := range s.entries {
		item := &s.entries[i]
		tier := matchTier(item, normalized, nameQuery)
		if tier == tierNoMatch {
			continue
		}
		candidate := rankedEntry{item: item, tier: tier}
		switch {
		case len(best) < limit:
			best = append(best, candidate)
			siftUpWorst(best, len(best)-1)
		case compareRanked(candidate, best[0]) < 0:
			best[0] = candidate
			siftDownWorst(best, 0)
		}
	}
	slices.SortFunc(best, compareRanked)
	result := make([]Entry, 0, len(best))
	for _, candidate := range best {
		result = append(result, candidate.item.entry)
	}
	return result
}

// compareRanked orders query matches by tier, then depth, then path length, then path.
func compareRanked(a, b rankedEntry) int {
	return cmp.Or(
		cmp.Compare(a.tier, b.tier),
		cmp.Compare(a.item.depth, b.item.depth),
		cmp.Compare(len(a.item.entry.Path), len(b.item.entry.Path)),
		cmp.Compare(a.item.entry.Path, b.item.entry.Path),
	)
}

// siftUpWorst and siftDownWorst keep heap[0] the worst-ranked of the kept matches.
func siftUpWorst(heap []rankedEntry, child int) {
	for child > 0 {
		parent := (child - 1) / 2
		if compareRanked(heap[child], heap[parent]) <= 0 {
			return
		}
		heap[child], heap[parent] = heap[parent], heap[child]
		child = parent
	}
}

func siftDownWorst(heap []rankedEntry, parent int) {
	for {
		worst := parent
		if left := 2*parent + 1; left < len(heap) && compareRanked(heap[left], heap[worst]) > 0 {
			worst = left
		}
		if right := 2*parent + 2; right < len(heap) && compareRanked(heap[right], heap[worst]) > 0 {
			worst = right
		}
		if worst == parent {
			return
		}
		heap[parent], heap[worst] = heap[worst], heap[parent]
		parent = worst
	}
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
