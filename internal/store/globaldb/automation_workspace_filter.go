package globaldb

import "strings"

func appendAutomationWorkspaceClauses(where []string, args []any, column string, ids []string) ([]string, []any) {
	if len(ids) == 0 {
		return where, args
	}
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		if id == "@user" {
			id = ""
		}
		placeholders[i] = "?"
		args = append(args, id)
	}
	where = append(where, column+" IN ("+strings.Join(placeholders, ",")+")")
	return where, args
}
