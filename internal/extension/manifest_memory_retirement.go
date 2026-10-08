package extensionpkg

import (
	"fmt"
	"strings"
)

// Remove this boundary filter in v0.6.0 after the retired-entry migration window.
func dropRetiredMemoryManifestEntries(manifest *Manifest) (dropped []string) {
	filter := func(field string, entries []string) []string {
		start := len(dropped)
		kept := make([]string, 0, len(entries))
		for _, entry := range entries {
			retired := false
			switch field {
			case "capabilities.provides":
				retired = strings.TrimSpace(entry) == "memory.backend"
			case "permissions.requires":
				switch strings.TrimSpace(entry) {
				case "memory/recall", "memory/store", "memory/forget", "memory.read", "memory.write":
					retired = true
				}
			}
			if retired {
				dropped = append(dropped, field+"="+entry)
			} else {
				kept = append(kept, entry)
			}
		}
		if len(dropped) == start {
			return entries
		}
		return kept
	}
	manifest.Capabilities.Provides = filter("capabilities.provides", manifest.Capabilities.Provides)
	manifest.Permissions.Requires = filter("permissions.requires", manifest.Permissions.Requires)
	for index := range manifest.Resources.Hooks {
		matcher := &manifest.Resources.Hooks[index].Matcher
		if matcher.CompactionReason != "" {
			dropped = append(dropped, fmt.Sprintf("hooks[%d].compaction_reason", index))
			matcher.CompactionReason = ""
		}
		if matcher.CompactionStrategy != "" {
			dropped = append(dropped, fmt.Sprintf("hooks[%d].compaction_strategy", index))
			matcher.CompactionStrategy = ""
		}
	}
	return dropped
}
