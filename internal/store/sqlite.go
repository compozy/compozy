package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"modernc.org/sqlite"
)

// OpenSQLiteDatabase opens a SQLite database and applies shared configuration.
func OpenSQLiteDatabase(
	ctx context.Context,
	path string,
	initialize func(context.Context, *sql.DB) error,
) (*sql.DB, error) {
	return openSQLiteDatabaseWithPragmas(ctx, path, nil, initialize)
}

// OpenSQLiteDatabaseWithPragmas opens a SQLite database with additional driver-level pragmas.
func OpenSQLiteDatabaseWithPragmas(
	ctx context.Context,
	path string,
	pragmas []string,
	initialize func(context.Context, *sql.DB) error,
) (*sql.DB, error) {
	return openSQLiteDatabaseWithPragmas(ctx, path, pragmas, initialize)
}

// OpenSQLiteReadPool reserves read-only connections independently of writer admission and capacity.
func OpenSQLiteReadPool(ctx context.Context, path string, initializers ...SQLiteReadInitializer) (*sql.DB, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, errors.New("store: database path is required")
	}
	if len(initializers) > 0 {
		return openInitializedSQLiteReadPool(ctx, cleanPath, initializers)
	}
	return openSQLiteHandle(ctx, cleanPath, sqliteQueryOnlyDSN(cleanPath))
}

func openSQLiteDatabaseWithPragmas(
	ctx context.Context,
	path string,
	pragmas []string,
	initialize func(context.Context, *sql.DB) error,
) (*sql.DB, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, errors.New("store: database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return nil, fmt.Errorf("store: create database directory for %q: %w", cleanPath, err)
	}
	if err := probeExistingSQLiteDatabase(ctx, cleanPath); err != nil {
		return nil, classifySQLiteOpenError(cleanPath, err)
	}

	db, err := openSQLiteDatabaseOnce(ctx, cleanPath, initialize, pragmas...)
	if err != nil {
		return nil, classifySQLiteOpenError(cleanPath, err)
	}
	return db, nil
}

func openSQLiteDatabaseOnce(
	ctx context.Context,
	path string,
	initialize func(context.Context, *sql.DB) error,
	extraPragmas ...string,
) (*sql.DB, error) {
	if initialize != nil {
		initializationDB, err := openSQLiteHandle(ctx, path, sqliteInitializationDSN(path))
		if err != nil {
			return nil, err
		}
		if err := initialize(ctx, initializationDB); err != nil {
			primaryErr := fmt.Errorf("store: initialize sqlite database %q: %w", path, err)
			return nil, closeSQLiteAfterError(
				initializationDB,
				primaryErr,
				fmt.Sprintf("initialized database %q after initialization failure", path),
			)
		}
		if err := initializationDB.Close(); err != nil {
			return nil, fmt.Errorf("store: close initialized sqlite database %q: %w", path, err)
		}
	}

	db, err := openSQLiteHandle(ctx, path, sqliteDSN(path, extraPragmas...))
	if err != nil {
		return nil, err
	}
	if err := configureSQLite(ctx, db); err != nil {
		primaryErr := fmt.Errorf("store: configure sqlite database %q: %w", path, err)
		return nil, closeSQLiteAfterError(
			db,
			primaryErr,
			fmt.Sprintf("database %q after configuration failure", path),
		)
	}

	return db, nil
}

func openSQLiteHandle(ctx context.Context, path string, dsn string) (*sql.DB, error) {
	if err := sqliteUnicodeSearchRegistrationErr; err != nil {
		return nil, fmt.Errorf("store: register SQLite Unicode search: %w", err)
	}
	connector, err := sqlite.NewConnector(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open sqlite database %q: %w", path, err)
	}
	identity := &sqliteFileDriver{Driver: connector.Driver()}
	db := sql.OpenDB(&sqliteFileConnector{Connector: connector, identity: identity})
	db.SetMaxOpenConns(defaultMaxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns)
	if err := db.PingContext(ctx); err != nil {
		primaryErr := fmt.Errorf("store: ping sqlite database %q: %w", path, err)
		return nil, closeSQLiteAfterError(
			db,
			primaryErr,
			fmt.Sprintf("database %q after ping failure", path),
		)
	}
	identity.info, err = os.Stat(path)
	if err != nil {
		return nil, closeSQLiteAfterError(
			db,
			fmt.Errorf("store: identify sqlite database %q: %w", path, err),
			"database after identity failure",
		)
	}
	return db, nil
}
