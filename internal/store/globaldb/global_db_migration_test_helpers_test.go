package globaldb

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

var globalMigrationTestMu sync.Mutex

func openGlobalMigrationPrefixDatabase(
	t *testing.T,
	path string,
	stream store.MigrationStream,
) (*sql.DB, error) {
	t.Helper()
	globalMigrationTestMu.Lock()
	defer globalMigrationTestMu.Unlock()

	return store.OpenSQLiteDatabase(
		testutil.Context(t),
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

	return store.Apply(testutil.Context(t), db, stream)
}

func openGlobalMigrationUpgrade(t *testing.T, path string) (*GlobalDB, error) {
	t.Helper()
	globalMigrationTestMu.Lock()
	defer globalMigrationTestMu.Unlock()

	return OpenGlobalDB(testutil.Context(t), path)
}
