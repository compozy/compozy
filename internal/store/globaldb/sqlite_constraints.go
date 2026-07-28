package globaldb

import (
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func sqliteConstraintCode(err error) (int, bool) {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return 0, false
	}
	return sqliteErr.Code(), true
}

func isSQLiteForeignKeyConstraint(err error) bool {
	code, ok := sqliteConstraintCode(err)
	return ok && code == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
}

func isSQLiteUniqueConstraint(err error) bool {
	code, ok := sqliteConstraintCode(err)
	return ok && code == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}

func isSQLitePrimaryKeyConstraint(err error) bool {
	code, ok := sqliteConstraintCode(err)
	return ok && code == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY
}
