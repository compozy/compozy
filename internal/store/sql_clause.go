package store

import (
	"fmt"
	"strings"
	"time"
)

const sqlFalsePredicate = "1 = 0"

// Clause represents an optional SQL filter clause plus its bound argument.
type Clause struct {
	sql    string
	arg    any
	ok     bool
	hasArg bool
}

// StringClause builds an equality clause when the value is non-empty.
func StringClause(column string, value string) Clause {
	value = strings.TrimSpace(value)
	if value == "" {
		return Clause{}
	}
	if _, err := NormalizeSQLiteIdentifier(column); err != nil {
		return alwaysFalseClause()
	}

	return Clause{sql: fmt.Sprintf("%s = ?", column), arg: value, ok: true, hasArg: true}
}

// NotStringClause builds an inequality clause when the value is non-empty.
func NotStringClause(column string, value string) Clause {
	value = strings.TrimSpace(value)
	if value == "" {
		return Clause{}
	}
	if _, err := NormalizeSQLiteIdentifier(column); err != nil {
		return alwaysFalseClause()
	}

	return Clause{sql: fmt.Sprintf("%s <> ?", column), arg: value, ok: true, hasArg: true}
}

// TimeClause builds a timestamp comparison clause when the value is non-zero.
func TimeClause(column string, op string, value time.Time) Clause {
	if value.IsZero() {
		return Clause{}
	}
	if _, err := NormalizeSQLiteIdentifier(column); err != nil || !isAllowedSQLOperator(op) {
		return alwaysFalseClause()
	}

	return Clause{sql: fmt.Sprintf("%s %s ?", column, op), arg: FormatTimestamp(value), ok: true, hasArg: true}
}

// Int64Clause builds a numeric comparison clause when the value is positive.
func Int64Clause(column string, op string, value int64) Clause {
	if value <= 0 {
		return Clause{}
	}
	if _, err := NormalizeSQLiteIdentifier(column); err != nil || !isAllowedSQLOperator(op) {
		return alwaysFalseClause()
	}

	return Clause{sql: fmt.Sprintf("%s %s ?", column, op), arg: value, ok: true, hasArg: true}
}

// BuildClauses compacts optional clauses into WHERE fragments and args.
func BuildClauses(input ...Clause) ([]string, []any) {
	where := make([]string, 0, len(input))
	args := make([]any, 0, len(input))
	for _, item := range input {
		if !item.ok {
			continue
		}
		where = append(where, item.sql)
		if item.hasArg {
			args = append(args, item.arg)
		}
	}
	return where, args
}

// AppendWhere appends a WHERE block when any clauses are present.
func AppendWhere(query string, where []string) string {
	if len(where) == 0 {
		return query
	}
	return query + " WHERE " + strings.Join(where, " AND ")
}

// AppendLimit appends a LIMIT clause when the limit is positive.
func AppendLimit(query string, args []any, limit int) (string, []any) {
	if limit <= 0 {
		return query, args
	}
	return query + " LIMIT ?", append(args, limit)
}

func alwaysFalseClause() Clause {
	return Clause{sql: sqlFalsePredicate, ok: true}
}

func isAllowedSQLOperator(value string) bool {
	switch strings.TrimSpace(value) {
	case "=", "!=", "<>", ">", ">=", "<", "<=":
		return true
	default:
		return false
	}
}
