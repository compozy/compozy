package memory

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	storepkg "github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
)

func TestCatalogMigrationStreams(t *testing.T) {
	t.Run("Should coexist with global migrations in one database with disjoint ownership", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		sharedPath := filepath.Join(baseDir, storepkg.GlobalDatabaseName)
		global, err := globaldb.OpenGlobalDB(testutil.Context(t), sharedPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(shared) error = %v", err)
		}
		t.Cleanup(func() {
			if err := global.Close(testutil.Context(t)); err != nil {
				t.Fatalf("GlobalDB.Close() error = %v", err)
			}
		})
		globalBefore, err := storepkg.Status(testutil.Context(t), global.DB(), globaldb.MigrationStream())
		if err != nil {
			t.Fatalf("Status(global before memory) error = %v", err)
		}

		catalog := openCatalogMigrationTestStore(t, sharedPath)
		memoryStatus, err := storepkg.Status(testutil.Context(t), catalog.catalog.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(memory) error = %v", err)
		}
		globalAfter, err := storepkg.Status(testutil.Context(t), global.DB(), globaldb.MigrationStream())
		if err != nil {
			t.Fatalf("Status(global after memory) error = %v", err)
		}
		if globalBefore != globalAfter {
			t.Fatalf("global status changed after memory apply: before=%#v after=%#v", globalBefore, globalAfter)
		}
		if memoryStatus.Version != 1 || memoryStatus.AppliedCount != 1 {
			t.Fatalf("shared memory status = %#v, want version/count 1", memoryStatus)
		}
		for _, table := range []string{globaldb.MigrationStream().VersionTable, MigrationStream().VersionTable} {
			if !catalogMigrationTableExists(t, catalog.catalog.db, table) {
				t.Fatalf("shared database missing version table %q", table)
			}
		}

		globalOnlyPath := filepath.Join(baseDir, "global-only.db")
		globalOnly, err := globaldb.OpenGlobalDB(testutil.Context(t), globalOnlyPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(global only) error = %v", err)
		}
		globalOnlyStatus, err := storepkg.Status(testutil.Context(t), globalOnly.DB(), globaldb.MigrationStream())
		if err != nil {
			t.Fatalf("Status(global only) error = %v", err)
		}
		globalTables := catalogDomainTables(t, globalOnly.DB())
		if err := globalOnly.Close(testutil.Context(t)); err != nil {
			t.Fatalf("GlobalDB.Close(global only) error = %v", err)
		}
		if globalAfter != globalOnlyStatus {
			t.Fatalf("shared global status = %#v, want global-only status %#v", globalAfter, globalOnlyStatus)
		}

		memoryOnly := openCatalogMigrationTestStore(t, filepath.Join(baseDir, "memory-only.db"))
		memoryOnlyStatus, err := storepkg.Status(testutil.Context(t), memoryOnly.catalog.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(memory only) error = %v", err)
		}
		if memoryStatus != memoryOnlyStatus {
			t.Fatalf("shared memory status = %#v, want memory-only status %#v", memoryStatus, memoryOnlyStatus)
		}
		memoryTables := catalogDomainTables(t, memoryOnly.catalog.db)
		for table := range globalTables {
			if _, overlaps := memoryTables[table]; overlaps {
				t.Fatalf("global and memory streams both own domain table %q", table)
			}
		}
	})

	t.Run("Should produce identical final schema when memory opens before global", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		globalFirstPath := filepath.Join(baseDir, "global-first.db")
		globalFirst, err := globaldb.OpenGlobalDB(testutil.Context(t), globalFirstPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(global first) error = %v", err)
		}
		globalFirstMemory := openCatalogMigrationTestStore(t, globalFirstPath)
		globalFirstStatuses := catalogMigrationStatuses(t, globalFirstMemory.catalog.db)
		globalFirstSchema := catalogSchemaObjects(t, globalFirstMemory.catalog.db)
		if err := globalFirst.Close(testutil.Context(t)); err != nil {
			t.Fatalf("GlobalDB.Close(global first) error = %v", err)
		}

		memoryFirstPath := filepath.Join(baseDir, "memory-first.db")
		memoryFirst := openCatalogMigrationTestStore(t, memoryFirstPath)
		globalSecond, err := globaldb.OpenGlobalDB(testutil.Context(t), memoryFirstPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(after memory) error = %v", err)
		}
		t.Cleanup(func() {
			if err := globalSecond.Close(testutil.Context(t)); err != nil {
				t.Fatalf("GlobalDB.Close(after memory) error = %v", err)
			}
		})
		memoryFirstSchema := catalogSchemaObjects(t, memoryFirst.catalog.db)
		if !reflect.DeepEqual(memoryFirstSchema, globalFirstSchema) {
			t.Fatalf(
				"schema differs by open order:\nglobal-first=%v\nmemory-first=%v",
				globalFirstSchema,
				memoryFirstSchema,
			)
		}
		memoryFirstStatuses := catalogMigrationStatuses(t, memoryFirst.catalog.db)
		if !reflect.DeepEqual(memoryFirstStatuses, globalFirstStatuses) {
			t.Fatalf(
				"migration statuses differ by open order:\nglobal-first=%#v\nmemory-first=%#v",
				globalFirstStatuses,
				memoryFirstStatuses,
			)
		}
	})

	t.Run("Should refuse either legacy marker through the opposite shared-file stream", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			legacyTable string
			open        func(t *testing.T, path string) error
		}{
			{
				name:        "Should refuse the memory legacy marker through global open",
				legacyTable: "memory_schema_migrations",
				open: func(t *testing.T, path string) error {
					t.Helper()
					globalDB, err := globaldb.OpenGlobalDB(testutil.Context(t), path)
					if err != nil {
						return err
					}
					return globalDB.Close(testutil.Context(t))
				},
			},
			{
				name:        "Should refuse the global legacy marker through memory open",
				legacyTable: "schema_migrations",
				open: func(t *testing.T, path string) error {
					t.Helper()
					store := NewStore(
						filepath.Join(t.TempDir(), "memory"),
						WithCatalogDatabasePath(path),
					)
					if err := store.OpenCatalog(testutil.Context(t)); err != nil {
						return err
					}
					return store.CloseCatalog(testutil.Context(t))
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				path := filepath.Join(t.TempDir(), storepkg.GlobalDatabaseName)
				seed, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatalf("sql.Open(seed) error = %v", err)
				}
				if _, err := seed.ExecContext(
					testutil.Context(t),
					`CREATE TABLE `+tt.legacyTable+` (version INTEGER PRIMARY KEY)`,
				); err != nil {
					if closeErr := seed.Close(); closeErr != nil {
						t.Fatalf("seed legacy table error = %v; close error = %v", err, closeErr)
					}
					t.Fatalf("seed legacy table error = %v", err)
				}
				if err := seed.Close(); err != nil {
					t.Fatalf("Close(seed) error = %v", err)
				}
				before := catalogDatabaseDigest(t, path)
				beforeWAL := catalogFilePresence(t, path+"-wal")
				beforeSHM := catalogFilePresence(t, path+"-shm")

				err = tt.open(t, path)
				if !errors.Is(err, storepkg.ErrLegacyDatabase) {
					t.Fatalf("open shared legacy database error = %v, want ErrLegacyDatabase", err)
				}
				if after := catalogDatabaseDigest(t, path); after != before {
					t.Fatalf("legacy database digest changed: got %x want %x", after, before)
				}
				if got := catalogFilePresence(t, path+"-wal"); got != beforeWAL {
					t.Fatalf("WAL presence after refusal = %t, want %t", got, beforeWAL)
				}
				if got := catalogFilePresence(t, path+"-shm"); got != beforeSHM {
					t.Fatalf("SHM presence after refusal = %t, want %t", got, beforeSHM)
				}
			})
		}
	})
}

func catalogMigrationStatuses(t *testing.T, db *sql.DB) map[string]storepkg.StreamStatus {
	t.Helper()

	statuses := make(map[string]storepkg.StreamStatus, 2)
	for _, stream := range []storepkg.MigrationStream{globaldb.MigrationStream(), MigrationStream()} {
		status, err := storepkg.Status(testutil.Context(t), db, stream)
		if err != nil {
			t.Fatalf("Status(%s) error = %v", stream.Name, err)
		}
		statuses[stream.Name] = status
	}
	return statuses
}

func openCatalogMigrationTestStore(t *testing.T, databasePath string) *Store {
	t.Helper()
	store := NewStore(
		filepath.Join(t.TempDir(), "memory"),
		WithCatalogDatabasePath(databasePath),
	)
	if err := store.OpenCatalog(testutil.Context(t)); err != nil {
		t.Fatalf("Store.OpenCatalog(%q) error = %v", databasePath, err)
	}
	t.Cleanup(func() {
		if err := store.CloseCatalog(testutil.Context(t)); err != nil {
			t.Fatalf("Store.CloseCatalog(%q) error = %v", databasePath, err)
		}
	})
	return store
}

func catalogMigrationTableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var count int
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		table,
	).Scan(&count); err != nil {
		t.Fatalf("query table %q existence error = %v", table, err)
	}
	return count == 1
}

func catalogDomainTables(t *testing.T, db *sql.DB) map[string]struct{} {
	t.Helper()
	objects := catalogSchemaObjects(t, db)
	tables := make(map[string]struct{})
	for _, object := range objects {
		parts := strings.SplitN(object, ":", 3)
		if len(parts) == 3 && parts[0] == "table" {
			tables[parts[1]] = struct{}{}
		}
	}
	return tables
}

func catalogSchemaObjects(t *testing.T, db *sql.DB) (objects []string) {
	t.Helper()
	rows, err := db.QueryContext(
		testutil.Context(t),
		`SELECT type, name, COALESCE(sql, '') FROM sqlite_master
		 WHERE name NOT LIKE 'sqlite_%'
		   AND name NOT LIKE 'goose_db_version_%'
		 ORDER BY type, name`,
	)
	if err != nil {
		t.Fatalf("query sqlite schema objects error = %v", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Fatalf("close sqlite schema object rows error = %v", closeErr)
		}
	}()
	for rows.Next() {
		var objectType, name, definition string
		if err := rows.Scan(&objectType, &name, &definition); err != nil {
			t.Fatalf("scan sqlite schema object error = %v", err)
		}
		objects = append(
			objects,
			fmt.Sprintf("%s:%s:%s", objectType, name, strings.Join(strings.Fields(definition), " ")),
		)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite schema objects error = %v", err)
	}
	sort.Strings(objects)
	return objects
}

func catalogDatabaseDigest(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return sha256.Sum256(contents)
}

func catalogFilePresence(t *testing.T, path string) bool {
	t.Helper()

	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	t.Fatalf("Stat(%q) error = %v", path, err)
	return false
}
