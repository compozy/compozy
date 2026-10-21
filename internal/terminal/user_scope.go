package terminal

import (
	"strings"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func terminalWorkspaceID(value string) string {
	if workspacepkg.ParseWorkspaceRef(value).Kind == workspacepkg.RefUser {
		return ""
	}
	return strings.TrimSpace(value)
}

// JournalWorkspaceID separates User journals without registering a project.
func JournalWorkspaceID(workspaceID, profileID string) string {
	if terminalWorkspaceID(workspaceID) == "" {
		return "user-" + strings.TrimSpace(profileID)
	}
	return workspaceID
}
