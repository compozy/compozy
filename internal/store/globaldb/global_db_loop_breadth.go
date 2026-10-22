package globaldb

import "github.com/compozy/compozy/internal/workspaceaccess"

func appendLoopWorkspaceFilter(
	where []string,
	args []any,
	column, workspace string,
	all bool,
	ids []string,
) ([]string, []any) {
	if !all {
		return append(where, column+" = ?"), append(args, workspace)
	}
	where = append(
		where,
		"("+column+" = '' OR EXISTS (SELECT 1 FROM workspaces AS registered WHERE registered.id = "+column+"))",
	)
	if len(ids) == 0 {
		return where, args
	}
	where = append(where, sqlInClause(column, len(ids)))
	for _, id := range ids {
		if id == workspaceaccess.UserSelector {
			id = ""
		}
		args = append(args, id)
	}
	return where, args
}
