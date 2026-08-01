package globaldb

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestMarketplaceCatalogFreshDB(t *testing.T) {
	t.Parallel()

	t.Run("Should install both marketplace catalog tables on a fresh database", func(t *testing.T) {
		t.Parallel()

		globalDB := openFreshTestGlobalDB(t)
		store := openMarketplaceMigrationStore(t, globalDB)
		seedMarketplaceMigrationProjection(t, store)
		assertMarketplaceMigrationProjection(t, store)
	})
}

func TestMarketplaceCatalogReopenAfterRestart(t *testing.T) {
	t.Parallel()

	t.Run("Should retain marketplace catalog entries and state after reopening", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		first, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}
		seedMarketplaceMigrationProjection(t, openMarketplaceMigrationStore(t, first))
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		second, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(second) error = %v", err)
		}
		t.Cleanup(func() {
			if err := second.Close(ctx); err != nil {
				t.Errorf("Close(second) error = %v", err)
			}
		})
		assertMarketplaceMigrationProjection(t, openMarketplaceMigrationStore(t, second))
	})
}

func TestMarketplaceCatalogManifestV2Migration(t *testing.T) {
	// Keep this full-history fixture serial: migration helpers share a process-wide
	// lock, and parallel suite contention previously exhausted its operation context.
	t.Run("Should discard every cached v1 catalog projection before the v2 reader opens", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
		legacy, err := sql.Open(sqliteDriverName, path)
		if err != nil {
			t.Fatalf("sql.Open(legacy) error = %v", err)
		}
		previousStream := globalMigrationPrefixBefore(t, "00032_marketplace_catalog_manifest_v2.sql")
		if err := applyGlobalMigrationPrefix(t, legacy, previousStream); err != nil {
			closeErr := legacy.Close()
			t.Fatalf("Apply(global through v31) error = %v; close error = %v", err, closeErr)
		}
		ctx := testutil.Context(t)
		if _, err := legacy.ExecContext(
			ctx,
			`INSERT INTO marketplace_catalog_state (kind, manifest_version, generated_at, fetched_at, stale, last_error)
			 VALUES ('mcp', 1, NULL, '2026-07-30T17:00:00Z', 0, '')`,
		); err != nil {
			closeErr := legacy.Close()
			t.Fatalf("seed v1 catalog state error = %v; close error = %v", err, closeErr)
		}
		if _, err := legacy.ExecContext(
			ctx,
			`INSERT INTO marketplace_catalog_entries (
				kind, entry_id, name, description, version, payload_json, fetched_at
			) VALUES ('mcp', 'legacy-mcp', 'Legacy MCP', 'must be removed', '1.0.0', '{}', '2026-07-30T17:00:00Z')`,
		); err != nil {
			closeErr := legacy.Close()
			t.Fatalf("seed v1 catalog entry error = %v; close error = %v", err, closeErr)
		}
		if err := legacy.Close(); err != nil {
			t.Fatalf("Close(legacy) error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(v2 migration) error = %v", err)
		}
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("Close(upgraded) error = %v", err)
		}

		reopenCtx := testutil.Context(t)
		reopened, err := OpenGlobalDB(reopenCtx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen v2 migration) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(reopenCtx); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})
		for _, table := range []string{"marketplace_catalog_entries", "marketplace_catalog_state"} {
			var count int
			if err := reopened.db.QueryRowContext(reopenCtx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
				t.Fatalf("count %s after v2 migration error = %v", table, err)
			}
			if count != 0 {
				t.Fatalf("%s count after v2 migration = %d, want 0", table, count)
			}
		}
	})
}

func openMarketplaceMigrationStore(t *testing.T, globalDB *GlobalDB) *marketplace.SQLiteStore {
	t.Helper()
	store, err := marketplace.NewSQLiteStore(globalDB)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	return store
}

func seedMarketplaceMigrationProjection(t *testing.T, store *marketplace.SQLiteStore) {
	t.Helper()
	fetchedAt := time.Date(2026, time.July, 13, 12, 0, 0, 0, time.UTC)
	document := &marketplace.Document{
		ManifestVersion: marketplace.ManifestVersion,
		GeneratedAt:     fetchedAt.Add(-time.Minute),
		FetchedAt:       fetchedAt,
		Entries: []marketplace.Entry{{
			Kind:        marketplace.KindSkill,
			EntryID:     "migration-fixture",
			Name:        "Migration fixture",
			Description: "Proves the catalog projection survives restart",
			InstallSlug: "compozy/migration-fixture",
			Payload:     json.RawMessage(`{"entry_id":"migration-fixture"}`),
		}},
	}
	if err := store.ReplaceKind(testutil.Context(t), marketplace.KindSkill, document); err != nil {
		t.Fatalf("ReplaceKind() error = %v", err)
	}
}

func assertMarketplaceMigrationProjection(t *testing.T, store *marketplace.SQLiteStore) {
	t.Helper()
	ctx := testutil.Context(t)
	page, err := store.ListKind(ctx, marketplace.KindSkill, "", 0, 10)
	if err != nil {
		t.Fatalf("ListKind() error = %v", err)
	}
	if got, want := len(page.Entries), 1; got != want || page.Entries[0].EntryID != "migration-fixture" {
		t.Fatalf("ListKind() = %#v, want persisted migration fixture", page)
	}
	state, err := store.KindState(ctx, marketplace.KindSkill)
	if err != nil {
		t.Fatalf("KindState() error = %v", err)
	}
	if state.EntryCount != 1 || state.Stale {
		t.Fatalf("KindState() = %#v, want one fresh persisted row", state)
	}
}
