package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const globalMigrationTestTimeout = 3 * time.Minute

var globalMigrationTestSerialMu sync.Mutex

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
	globalMigrationTestSerialMu.Lock()
	defer globalMigrationTestSerialMu.Unlock()
	if _, err := os.Stat(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err := copyGlobalMigrationTemplate(path, stream); err != nil {
			return nil, err
		}
	}

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
	globalMigrationTestSerialMu.Lock()
	defer globalMigrationTestSerialMu.Unlock()

	return store.Apply(globalMigrationTestContext(t), db, stream)
}

func openGlobalMigrationUpgrade(t *testing.T, path string) (*GlobalDB, error) {
	return openGlobalMigrationUpgradeWithOptions(t, path)
}

func openGlobalMigrationUpgradeWithOptions(
	t *testing.T,
	path string,
	options ...OpenOption,
) (*GlobalDB, error) {
	t.Helper()
	globalMigrationTestSerialMu.Lock()
	defer globalMigrationTestSerialMu.Unlock()

	return OpenGlobalDB(globalMigrationTestContext(t), path, options...)
}
