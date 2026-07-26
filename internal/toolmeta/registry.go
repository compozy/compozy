package toolmeta

import "strings"

// NativeEntry returns curated progress metadata for AGH's native tool namespace.
func NativeEntry(toolID string) (Entry, bool) {
	entry, ok := nativeEntries[strings.TrimSpace(toolID)]
	return entry, ok
}
