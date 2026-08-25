package storeschema

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

var (
	gooseTriggerBlock       = regexp.MustCompile(`(?is)--\s*\+goose\s+StatementBegin\s*(.*?)\s*--\s*\+goose\s+StatementEnd`)
	sqliteCreateTriggerName = regexp.MustCompile(`(?i)^CREATE\s+TRIGGER\s+(?:IF\s+NOT\s+EXISTS\s+)?["` + "`" + `\[]?([^"` + "`" + `\]\s]+)`)
	sqliteTriggerTable      = regexp.MustCompile(`(?is)\b(?:BEFORE|AFTER|INSTEAD\s+OF)\b.*?\bON\s+["` + "`" + `\[]?([^"` + "`" + `\]\s]+)`)
	sqliteDropTrigger       = regexp.MustCompile(`(?im)^\s*DROP\s+TRIGGER\s+(?:IF\s+EXISTS\s+)?["` + "`" + `\[]?([^"` + "`" + `\]\s;]+)[^\n]*`)
)

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

func readMigrationSQLiteTriggers(_ context.Context, descriptor stream) ([]sqliteTrigger, error) {
	entries, err := os.ReadDir(descriptor.migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read %s migrations for triggers: %w", descriptor.name, err)
	}
	current := make(map[string]sqliteTrigger)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != sqlFileExtension {
			continue
		}
		path := filepath.Join(descriptor.migrationsDir, entry.Name())
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s migration %q for triggers: %w", descriptor.name, path, err)
		}
		up := strings.SplitN(string(contents), "-- +goose Down", 2)[0]
		events, err := migrationTriggerEvents(up)
		if err != nil {
			return nil, fmt.Errorf("read %s migration %q triggers: %w", descriptor.name, path, err)
		}
		for _, event := range events {
			if event.drop {
				delete(current, event.trigger.name)
				continue
			}
			current[event.trigger.name] = event.trigger
		}
	}
	triggers := make([]sqliteTrigger, 0, len(current))
	for _, trigger := range current {
		triggers = append(triggers, trigger)
	}
	slices.SortFunc(triggers, func(left, right sqliteTrigger) int {
		return strings.Compare(left.name, right.name)
	})
	return triggers, nil
}

type migrationTriggerEvent struct {
	offset  int
	drop    bool
	trigger sqliteTrigger
}

func migrationTriggerEvents(contents string) ([]migrationTriggerEvent, error) {
	events := make([]migrationTriggerEvent, 0)
	for _, match := range gooseTriggerBlock.FindAllStringSubmatchIndex(contents, -1) {
		statement := strings.TrimSpace(contents[match[2]:match[3]])
		nameMatch := sqliteCreateTriggerName.FindStringSubmatch(statement)
		if len(nameMatch) != 2 {
			continue
		}
		tableMatch := sqliteTriggerTable.FindStringSubmatch(statement)
		if len(tableMatch) != 2 {
			return nil, fmt.Errorf("trigger %q has no owning table", nameMatch[1])
		}
		events = append(events, migrationTriggerEvent{
			offset: match[0],
			trigger: sqliteTrigger{
				name: nameMatch[1], tableName: tableMatch[1], createSQL: statement,
			},
		})
	}
	for _, match := range sqliteDropTrigger.FindAllStringSubmatchIndex(contents, -1) {
		events = append(events, migrationTriggerEvent{
			offset:  match[0],
			drop:    true,
			trigger: sqliteTrigger{name: contents[match[2]:match[3]]},
		})
	}
	slices.SortFunc(events, func(left, right migrationTriggerEvent) int {
		return left.offset - right.offset
	})
	return events, nil
}

func appendSQLiteTriggerChanges(plan *migrate.Plan, current, desired []sqliteTrigger) {
	rebuilt := rebuiltSQLiteTables(plan)
	currentByName := make(map[string]sqliteTrigger, len(current))
	desiredByName := make(map[string]sqliteTrigger, len(desired))
	for _, trigger := range current {
		currentByName[trigger.name] = trigger
	}
	for _, trigger := range desired {
		desiredByName[trigger.name] = trigger
	}

	dropNames := make([]string, 0)
	creates := make([]sqliteTrigger, 0)
	for _, trigger := range current {
		desiredTrigger, retained := desiredByName[trigger.name]
		changed := retained && normalizeSQLiteTriggerSQL(trigger.createSQL) != normalizeSQLiteTriggerSQL(desiredTrigger.createSQL)
		if !retained || changed || triggerTouchesRebuiltTable(trigger, rebuilt) {
			dropNames = append(dropNames, trigger.name)
		}
		if retained && (changed || triggerTouchesRebuiltTable(trigger, rebuilt)) {
			creates = append(creates, desiredTrigger)
		}
	}
	for _, trigger := range desired {
		_, exists := currentByName[trigger.name]
		if !exists {
			creates = append(creates, trigger)
		}
	}

	drops := make([]*migrate.Change, 0, len(dropNames))
	for _, name := range dropNames {
		drops = append(drops, &migrate.Change{
			Cmd:     "DROP TRIGGER IF EXISTS " + quoteSQLiteIdentifier(name),
			Comment: fmt.Sprintf("drop trigger %q before applying its declarative change", name),
		})
	}
	plan.Changes = slices.Insert(plan.Changes, firstSQLiteTableDrop(plan), drops...)
	for _, trigger := range creates {
		plan.Changes = append(plan.Changes, &migrate.Change{
			Cmd:     strings.TrimSuffix(strings.TrimSpace(trigger.createSQL), ";"),
			Reverse: "DROP TRIGGER " + quoteSQLiteIdentifier(trigger.name),
			Comment: fmt.Sprintf("apply declarative trigger %q on table %q", trigger.name, trigger.tableName),
		})
	}
}

func normalizeSQLiteTriggerSQL(statement string) string {
	return strings.Join(strings.Fields(strings.TrimSuffix(strings.TrimSpace(statement), ";")), " ")
}

func triggerTouchesRebuiltTable(trigger sqliteTrigger, rebuilt []string) bool {
	if slices.Contains(rebuilt, trigger.tableName) {
		return true
	}
	for _, tableName := range rebuilt {
		identifier := regexp.QuoteMeta(tableName)
		pattern := `(?i)(^|[^[:alnum:]_])(?:["` + "`" + `\[])?` + identifier + `(?:["` + "`" + `\]])?([^[:alnum:]_]|$)`
		if regexp.MustCompile(pattern).MatchString(trigger.createSQL) {
			return true
		}
	}
	return false
}

func firstSQLiteTableDrop(plan *migrate.Plan) int {
	for index, change := range plan.Changes {
		if sqliteDropTable.MatchString(strings.TrimSpace(change.Cmd)) {
			return index
		}
	}
	return len(plan.Changes)
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
