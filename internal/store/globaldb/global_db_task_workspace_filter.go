package globaldb

import (
	"strings"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

func appendTaskWorkspaceIDs(where []string, args []any, ids []string) ([]string, []any) {
	if len(ids) == 0 {
		return where, args
	}
	branches := make([]string, 0, 2)
	projects := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == workspaceaccess.UserSelector {
			branches = append(branches, "(scope = 'global' AND workspace_id IS NULL)")
		} else {
			projects = append(projects, id)
		}
	}
	if len(projects) != 0 {
		branches = append(branches, sqlInClause("workspace_id", len(projects)))
		for _, id := range projects {
			args = append(args, id)
		}
	}
	return append(where, "("+strings.Join(branches, " OR ")+")"), args
}
