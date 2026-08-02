package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/store"
)

type sessionDBParentDirectory interface {
	Close() error
	OpenRegularFile(string) (*os.File, error)
	AtomicWriteFile(string, []byte, os.FileMode, bool) error
}

type sessionDBParentDirectoryOpener func(string, os.FileMode) (sessionDBParentDirectory, string, error)

type sessionDBDatabaseCloser func(*sql.DB) error

func openOwnedSessionSQLite(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) (*sql.DB, error) {
	return openOwnedSessionSQLiteWithVacuum(ctx, owner, path, vacuumSessionSQLite)
}

func openOwnedSessionSQLiteWithVacuum(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
	vacuumFn sessionVacuumFunc,
) (*sql.DB, error) {
	return openOwnedSessionSQLiteWithDirectory(
		ctx,
		owner,
		path,
		vacuumFn,
		openSessionDBParentDirectory,
		func(db *sql.DB) error { return db.Close() },
	)
}

func openOwnedSessionSQLiteWithDirectory(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
	vacuumFn sessionVacuumFunc,
	openParent sessionDBParentDirectoryOpener,
	closeDatabase sessionDBDatabaseCloser,
) (db *sql.DB, retErr error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, errors.New("store: session database path is required")
	}
	if openParent == nil {
		return nil, errors.New("store: session database parent opener is required")
	}
	if closeDatabase == nil {
		return nil, errors.New("store: session database closer is required")
	}
	familyLease, err := AcquireFamilyLease(ctx, cleanPath)
	if err != nil {
		return nil, err
	}
	defer familyLease.Release()

	directory, targetName, err := openParent(cleanPath, 0o755)
	if err != nil {
		return nil, fmt.Errorf("store: open session database parent for %q: %w", cleanPath, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("store: close session database parent: %w", closeErr))
			if db != nil {
				if dbCloseErr := closeDatabase(db); dbCloseErr != nil {
					retErr = errors.Join(
						retErr,
						fmt.Errorf("store: close session database after parent close failure: %w", dbCloseErr),
					)
				}
				db = nil
			}
		}
	}()

	existing, err := directory.OpenRegularFile(targetName)
	if err == nil {
		if closeErr := existing.Close(); closeErr != nil {
			return nil, fmt.Errorf("store: close inspected session database %q: %w", cleanPath, closeErr)
		}
		familyLease.Release()
		return openExistingOwnedSessionSQLite(ctx, owner, cleanPath, vacuumFn)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("store: inspect session database path %q: %w", cleanPath, err)
	}

	contents, err := buildSessionSQLiteCandidate(ctx, owner, vacuumFn)
	if err != nil {
		return nil, err
	}
	if err := directory.AtomicWriteFile(targetName, contents, 0o600, false); errors.Is(err, os.ErrExist) {
		familyLease.Release()
		return openExistingOwnedSessionSQLite(ctx, owner, cleanPath, vacuumFn)
	} else if err != nil {
		return nil, fmt.Errorf("store: publish complete session database %q: %w", cleanPath, err)
	}
	familyLease.Release()
	return openExistingOwnedSessionSQLiteWithRetry(ctx, owner, cleanPath, vacuumFn)
}

func openSessionDBParentDirectory(
	path string,
	perm os.FileMode,
) (sessionDBParentDirectory, string, error) {
	return fileutil.OpenOrCreateParentDirectory(path, perm)
}

func openExistingOwnedSessionSQLite(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
	vacuumFn sessionVacuumFunc,
) (*sql.DB, error) {
	return openExistingOwnedSessionSQLiteWithRetry(ctx, owner, path, vacuumFn)
}

func openExistingOwnedSessionSQLiteWithRetry(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
	vacuumFn sessionVacuumFunc,
) (*sql.DB, error) {
	const maxAttempts = 20
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		db, err := openSessionSQLiteForExistingOwnerWithVacuum(ctx, owner, path, vacuumFn)
		if err == nil {
			return db, nil
		}
		lastErr = err
		if !store.IsSQLiteBusy(err) || attempt == maxAttempts {
			return nil, err
		}
		if err := waitForSessionDBOpenRetry(ctx, path, attempt); err != nil {
			return nil, errors.Join(lastErr, err)
		}
	}
	return nil, lastErr
}

func waitForSessionDBOpenRetry(ctx context.Context, path string, attempt int) error {
	timer := time.NewTimer(time.Duration(attempt) * 10 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("store: wait for fresh session database owner %q: %w", path, ctx.Err())
	case <-timer.C:
		return nil
	}
}

func verifySessionDBOwnerReadOnly(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) (retErr error) {
	db, err := sql.Open("sqlite", sessionDBOwnerGuardDSN(path))
	if err != nil {
		return fmt.Errorf("store: open session database owner guard %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("store: close session database owner guard %q: %w", path, closeErr))
		}
	}()
	if err := verifySessionDBOwner(ctx, db, owner); err != nil {
		return fmt.Errorf("store: refuse session database %q before owned open: %w", path, err)
	}
	return nil
}

func sessionDBOwnerGuardDSN(path string) string {
	u := url.URL{Scheme: "file", Path: store.SqliteFilePath(path)}
	query := u.Query()
	query.Set("mode", "ro")
	query.Set("immutable", "1")
	u.RawQuery = query.Encode()
	return u.String()
}
