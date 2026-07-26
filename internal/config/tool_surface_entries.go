package config

import "strings"

// EntryByPath returns one flattened entry.
func EntryByPath(entries []Entry, path string) (Entry, bool) {
	trimmed := strings.TrimSpace(path)
	for _, entry := range entries {
		if entry.Path == trimmed {
			return entry, true
		}
	}
	return Entry{}, false
}
