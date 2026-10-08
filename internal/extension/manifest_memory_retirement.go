package extensionpkg

import "strings"

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
	return dropped
}
