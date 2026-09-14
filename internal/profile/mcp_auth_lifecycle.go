package profile

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb"
)

var mcpAuthLifecycleTables = []string{"mcp_auth_tokens", "mcp_oauth_registrations"}

func deleteMCPAuthProfileRecords(
	ctx context.Context,
	exec globaldb.ProfileWriteExecutor,
	profileName string,
) error {
	profileName = strings.TrimSpace(profileName)
	suffix := "@pf:" + profileName
	for _, table := range mcpAuthLifecycleTables {
		if _, err := exec.ExecContext(
			ctx,
			`DELETE FROM `+table+`
			 WHERE (scope = 'profile' AND workspace_id = ?)
			    OR (scope = 'workspace_profile' AND SUBSTR(workspace_id, -LENGTH(?)) = ?)`,
			profileName,
			suffix, suffix,
		); err != nil {
			return fmt.Errorf("profile: remove %s for %q: %w", table, profileName, err)
		}
	}
	return nil
}
