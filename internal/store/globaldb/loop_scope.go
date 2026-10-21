package globaldb

import (
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func loopStateProfileID(workspaceID looppkg.WorkspaceID, profileID string) string {
	if strings.TrimSpace(string(workspaceID)) != "" {
		return ""
	}
	return strings.TrimSpace(profileID)
}

func validateLoopStateScope(workspaceID looppkg.WorkspaceID, profileID string) error {
	if strings.TrimSpace(string(workspaceID)) == "" && strings.TrimSpace(profileID) == "" {
		return fmt.Errorf("%w: User scope requires profile_id", looppkg.ErrValidation)
	}
	return nil
}
