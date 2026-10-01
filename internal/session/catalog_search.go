package session

import "strings"

// sessionCatalogDisplayTitle mirrors the visible session identity at the read
// boundary, including the title used when no meaningful name has been persisted.
func sessionCatalogDisplayTitle(info *Info) string {
	name := strings.TrimSpace(info.Name)
	if name != "" && name != info.ID && name != strings.TrimSpace(info.AgentName) {
		return name
	}
	return "New session"
}
