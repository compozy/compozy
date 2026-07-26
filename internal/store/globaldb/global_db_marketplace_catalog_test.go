package globaldb

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/marketplace"
	"github.com/compozy/agh/internal/testutil"
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
