package globaldb

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const globalMigrationTestTimeout = 3 * time.Minute

var globalMigrationTestMu sync.Mutex

func globalMigrationTestContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), globalMigrationTestTimeout)
	t.Cleanup(cancel)
	return ctx
}

func openGlobalMigrationPrefixDatabase(
	t *testing.T,
	path string,
	stream store.MigrationStream,
) (*sql.DB, error) {
	t.Helper()
	globalMigrationTestMu.Lock()
	defer globalMigrationTestMu.Unlock()

	return store.OpenSQLiteDatabase(
		globalMigrationTestContext(t),
		path,
		func(ctx context.Context, db *sql.DB) error {
			return store.Apply(ctx, db, stream)
		},
	)
}

func applyGlobalMigrationPrefix(
	t *testing.T,
	db *sql.DB,
	stream store.MigrationStream,
) error {
	t.Helper()
	globalMigrationTestMu.Lock()
	defer globalMigrationTestMu.Unlock()

	return store.Apply(globalMigrationTestContext(t), db, stream)
}

func openGlobalMigrationUpgrade(t *testing.T, path string) (*GlobalDB, error) {
	t.Helper()
	globalMigrationTestMu.Lock()
	defer globalMigrationTestMu.Unlock()

	return OpenGlobalDB(globalMigrationTestContext(t), path)
}
