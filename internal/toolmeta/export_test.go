package toolmeta

import "maps"

func NativeEntriesForTest() map[string]Entry {
	return maps.Clone(nativeEntries)
}
