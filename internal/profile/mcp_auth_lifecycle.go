package profile

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb"
)

var mcpAuthLifecycleTables = []string{"mcp_auth_tokens", "mcp_oauth_registrations"}

func rewriteMCPAuthProfileName(
	ctx context.Context,
	exec globaldb.ProfileWriteExecutor,
	oldName string,
	newName string,
) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	oldSuffix := "@pf:" + oldName
	newSuffix := "@pf:" + newName
	for _, table := range mcpAuthLifecycleTables {
		if _, err := exec.ExecContext(
			ctx,
			`UPDATE `+table+`
			 SET workspace_id = CASE
				WHEN scope = 'profile' THEN ?
				ELSE SUBSTR(workspace_id, 1, LENGTH(workspace_id) - LENGTH(?)) || ?
			 END
			 WHERE (scope = 'profile' AND workspace_id = ?)
			    OR (scope = 'workspace_profile' AND workspace_id LIKE ?)`,
			newName,
			oldSuffix,
			newSuffix,
			oldName,
			"%"+oldSuffix,
		); err != nil {
			return fmt.Errorf("profile: rewrite %s profile keys from %q to %q: %w", table, oldName, newName, err)
		}
	}
	return nil
}

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
			    OR (scope = 'workspace_profile' AND workspace_id LIKE ?)`,
			profileName,
			"%"+suffix,
		); err != nil {
			return fmt.Errorf("profile: remove %s for %q: %w", table, profileName, err)
		}
	}
	return nil
}
