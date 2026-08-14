package storeschema

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"ariga.io/atlas/sql/migrate"
)

type sqliteTrigger struct {
	name      string
	tableName string
	createSQL string
}

var sqliteDropTable = regexp.MustCompile(`(?i)^DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?["` + "`" + `]?([^"` + "`" + `\s;]+)`)

func inspectSQLiteTriggers(ctx context.Context, db *sql.DB) (_ []sqliteTrigger, err error) {
	rows, err := db.QueryContext(ctx, `
		SELECT name, tbl_name, sql
		FROM sqlite_master
		WHERE type = 'trigger' AND sql IS NOT NULL
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("query SQLite triggers: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close SQLite trigger rows: %w", closeErr))
		}
	}()
	triggers := make([]sqliteTrigger, 0)
	for rows.Next() {
		var trigger sqliteTrigger
		if err := rows.Scan(&trigger.name, &trigger.tableName, &trigger.createSQL); err != nil {
			return nil, fmt.Errorf("scan SQLite trigger: %w", err)
		}
		triggers = append(triggers, trigger)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SQLite triggers: %w", err)
	}
	return triggers, nil
}

func appendSQLiteTriggerRecreations(plan *migrate.Plan, triggers []sqliteTrigger) {
	rebuilt := rebuiltSQLiteTables(plan)
	if len(rebuilt) == 0 {
		return
	}
	for _, trigger := range triggers {
		if !slices.Contains(rebuilt, trigger.tableName) {
			continue
		}
		plan.Changes = append(plan.Changes, &migrate.Change{
			Cmd:     strings.TrimSuffix(strings.TrimSpace(trigger.createSQL), ";"),
			Reverse: "DROP TRIGGER " + quoteSQLiteIdentifier(trigger.name),
			Comment: fmt.Sprintf("recreate trigger %q after rebuilding table %q", trigger.name, trigger.tableName),
		})
	}
}

func rebuiltSQLiteTables(plan *migrate.Plan) []string {
	tables := make([]string, 0)
	for _, change := range plan.Changes {
		match := sqliteDropTable.FindStringSubmatch(strings.TrimSpace(change.Cmd))
		if len(match) != 2 || slices.Contains(tables, match[1]) {
			continue
		}
		tables = append(tables, match[1])
	}
	return tables
}

func wrapGooseTriggerStatements(contents []byte, changes []*migrate.Change) ([]byte, error) {
	for _, change := range changes {
		command := strings.TrimSpace(change.Cmd)
		if !strings.HasPrefix(strings.ToUpper(command), "CREATE TRIGGER ") {
			continue
		}
		statement := []byte(command + ";")
		if bytes.Count(contents, statement) != 1 {
			return nil, fmt.Errorf("format goose migration: trigger %q statement occurrence is not unique", command)
		}
		wrapped := []byte("-- +goose StatementBegin\n" + command + ";\n-- +goose StatementEnd")
		contents = bytes.Replace(contents, statement, wrapped, 1)
	}
	return contents, nil
}

func quoteSQLiteIdentifier(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}
