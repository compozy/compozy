package sessiondb

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var errTransientOwnerGuard = errors.New("store: transient owner inspection failure; database integrity verified")

func verifySessionDBOwnerWithDriver(
	ctx context.Context,
	sqliteDriver driver.Driver,
	path string,
	owner store.SessionDBOwner,
) (string, error) {
	for attempt := 1; ; attempt++ {
		identity, err := verifySessionDBOwnerOnce(ctx, sqliteDriver, path, owner)
		if err == nil || attempt == 3 || (!store.IsSQLiteBusy(err) && !errors.Is(err, errTransientOwnerGuard)) {
			return identity, err
		}
		if waitErr := waitForSessionDBOpenRetry(ctx, path, attempt); waitErr != nil {
			return "", errors.Join(err, waitErr)
		}
	}
}

func classifyOwnerGuardFailure(ctx context.Context, connection sqlite.ExecQuerierContext, err error) error {
	sqliteErr, ok := errors.AsType[*sqlite.Error](err)
	if !ok || sqliteErr.Code()&0xff != sqlite3.SQLITE_CORRUPT {
		return err
	}
	values, checkErr := querySessionSQLiteConnectionRow(ctx, connection, "PRAGMA quick_check(1)", 1)
	if checkErr != nil {
		return fmt.Errorf("store: session database integrity could not be verified: %w", errors.Join(err, checkErr))
	}
	if len(values) == 1 && values[0] == "ok" {
		return errors.Join(errTransientOwnerGuard, err)
	}
	return fmt.Errorf("store: session database corruption confirmed by integrity check: %w", err)
}
