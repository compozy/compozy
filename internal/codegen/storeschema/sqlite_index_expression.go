package storeschema

import (
	"fmt"
	"regexp"
	"strings"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlite"
)

const unsupportedSQLiteIndexExpression = "<unsupported>"

var sqliteIndexOrderSuffix = regexp.MustCompile(`(?i)\s+(?:ASC|DESC)\s*$`)

var sqliteCreateIndexName = regexp.MustCompile(
	`(?i)^CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"([^"]+)"|` +
		"`([^`]+)`" + `|\[([^\]]+)\]|([^\s(]+))`,
)

// normalizeSQLiteIndexExpressions restores multiline index expressions that the Atlas SQLite
// inspector cannot currently extract from sqlite_master.
func normalizeSQLiteIndexExpressions(realm *schema.Realm) error {
	if realm == nil {
		return nil
	}
	for _, schemaObject := range realm.Schemas {
		for _, table := range schemaObject.Tables {
			for _, index := range table.Indexes {
				if !hasUnsupportedSQLiteIndexExpression(index) {
					continue
				}
				parts, err := sqliteCreateIndexParts(index)
				if err != nil {
					return fmt.Errorf(
						"codegen: recover SQLite expression index %q on table %q: %w",
						index.Name,
						table.Name,
						err,
					)
				}
				if len(parts) != len(index.Parts) {
					return fmt.Errorf(
						"codegen: recover SQLite expression index %q on table %q: parsed %d parts, want %d",
						index.Name,
						table.Name,
						len(parts),
						len(index.Parts),
					)
				}
				for partIndex, part := range index.Parts {
					raw, ok := part.X.(*schema.RawExpr)
					if !ok || raw.X != unsupportedSQLiteIndexExpression {
						continue
					}
					expression := strings.TrimSpace(sqliteIndexOrderSuffix.ReplaceAllString(parts[partIndex], ""))
					if expression == "" {
						return fmt.Errorf(
							"codegen: recover SQLite expression index %q on table %q: part %d is empty",
							index.Name,
							table.Name,
							partIndex,
						)
					}
					raw.X = expression
				}
			}
		}
	}
	return nil
}

func restoreSQLiteExpressionIndexStatements(plan *migrate.Plan, desired *schema.Realm) {
	if plan == nil || desired == nil {
		return
	}
	statements := make(map[string]string)
	for _, schemaObject := range desired.Schemas {
		for _, table := range schemaObject.Tables {
			for _, index := range table.Indexes {
				if !hasSQLiteIndexExpression(index) {
					continue
				}
				for _, attribute := range index.Attrs {
					if create, ok := attribute.(*sqlite.CreateStmt); ok {
						statements[index.Name] = strings.TrimSuffix(strings.TrimSpace(create.S), ";")
						break
					}
				}
			}
		}
	}
	for _, change := range plan.Changes {
		match := sqliteCreateIndexName.FindStringSubmatch(strings.TrimSpace(change.Cmd))
		if len(match) == 0 {
			continue
		}
		for _, name := range match[1:] {
			if statement := statements[name]; statement != "" {
				change.Cmd = statement
				break
			}
		}
	}
}

func hasSQLiteIndexExpression(index *schema.Index) bool {
	for _, part := range index.Parts {
		if part.X != nil {
			return true
		}
	}
	return false
}

func hasUnsupportedSQLiteIndexExpression(index *schema.Index) bool {
	for _, part := range index.Parts {
		if raw, ok := part.X.(*schema.RawExpr); ok && raw.X == unsupportedSQLiteIndexExpression {
			return true
		}
	}
	return false
}

func sqliteCreateIndexParts(index *schema.Index) ([]string, error) {
	var statement string
	for _, attribute := range index.Attrs {
		if create, ok := attribute.(*sqlite.CreateStmt); ok {
			statement = create.S
			break
		}
	}
	if statement == "" {
		return nil, fmt.Errorf("CREATE INDEX statement is missing")
	}
	open, err := sqliteIndexPartsOpen(statement, index.Table.Name)
	if err != nil {
		return nil, err
	}
	return splitSQLiteIndexParts(statement, open)
}

func sqliteIndexPartsOpen(statement, tableName string) (int, error) {
	quotedTable := regexp.QuoteMeta(tableName)
	pattern := regexp.MustCompile(
		`(?is)\bON\s+(?:"|` + "`" + `|\[)?` + quotedTable + `(?:"|` + "`" + `|\])?\s*\(`,
	)
	match := pattern.FindStringIndex(statement)
	if match == nil {
		return 0, fmt.Errorf("cannot find indexed table %q in CREATE INDEX statement", tableName)
	}
	relativeOpen := strings.LastIndex(statement[match[0]:match[1]], "(")
	if relativeOpen < 0 {
		return 0, fmt.Errorf("index part list is missing")
	}
	return match[0] + relativeOpen, nil
}

func splitSQLiteIndexParts(statement string, open int) ([]string, error) {
	parts := make([]string, 0)
	start := open + 1
	depth := 0
	var quote byte
	for position := start; position < len(statement); position++ {
		character := statement[position]
		if quote != 0 {
			if quote == '[' {
				if character == ']' {
					if position+1 < len(statement) && statement[position+1] == ']' {
						position++
						continue
					}
					quote = 0
				}
				continue
			}
			if character == quote {
				if position+1 < len(statement) && statement[position+1] == quote {
					position++
					continue
				}
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"', '`', '[':
			quote = character
		case '(':
			depth++
		case ')':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(statement[start:position]))
				return parts, nil
			}
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(statement[start:position]))
				start = position + 1
			}
		}
	}
	return nil, fmt.Errorf("index part list is not closed")
}
