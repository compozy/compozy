package globaldb

import (
	"errors"
	"fmt"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	taskpkg "github.com/compozy/compozy/internal/task"
)

const terminalRunCommandGuardMessage = "task run terminal command in progress"

func sqliteConstraintCode(err error) (int, bool) {
	sqliteErr, ok := errors.AsType[*sqlite.Error](err)
	if !ok || sqliteErr == nil {
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

// mapTerminalRunCommandGuardError translates the SQLite trigger ABI into the domain error once.
// SQLite exposes RAISE(ABORT) identity only as the trigger constraint code plus its exact message.
func mapTerminalRunCommandGuardError(err error) error {
	if err == nil {
		return nil
	}
	sqliteErr, ok := errors.AsType[*sqlite.Error](err)
	if !ok || sqliteErr == nil || sqliteErr.Code() != sqlite3.SQLITE_CONSTRAINT_TRIGGER {
		return err
	}
	wantSuffix := fmt.Sprintf(": %s (%d)", terminalRunCommandGuardMessage, sqlite3.SQLITE_CONSTRAINT_TRIGGER)
	if !strings.HasSuffix(sqliteErr.Error(), wantSuffix) {
		return err
	}
	return fmt.Errorf("%w: %w", taskpkg.ErrTerminalRunCommandInProgress, err)
}
