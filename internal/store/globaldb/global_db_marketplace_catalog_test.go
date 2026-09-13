package globaldb

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
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
	t.Parallel()
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
		ctx := globalMigrationTestContext(t)
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
			Kind:         marketplace.KindExtension,
			EntryID:      "migration-fixture",
			Name:         "Migration fixture",
			Description:  "Proves the catalog projection survives restart",
			InstallSlug:  "compozy/migration-fixture",
			Version:      "1.0.0",
			DigestSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Tier:         "official",
			Payload:      json.RawMessage(`{"entry_id":"migration-fixture"}`),
		}},
	}
	if err := store.ReplaceSource(testutil.Context(t), marketplace.CompozyCatalogSource, 0, document); err != nil {
		t.Fatalf("ReplaceSource() error = %v", err)
	}
}

func assertMarketplaceMigrationProjection(t *testing.T, store *marketplace.SQLiteStore) {
	t.Helper()
	ctx := testutil.Context(t)
	page, err := store.BrowseSource(ctx, marketplace.CompozyCatalogSource, "", 0, 10)
	if err != nil {
		t.Fatalf("BrowseSource() error = %v", err)
	}
	if got, want := len(page.Entries), 1; got != want || page.Entries[0].EntryID != "migration-fixture" {
		t.Fatalf("BrowseSource() = %#v, want persisted migration fixture", page)
	}
	state, err := store.SourceState(ctx, marketplace.CompozyCatalogSource)
	if err != nil {
		t.Fatalf("SourceState() error = %v", err)
	}
	if state.EntryCount != 1 || state.Stale {
		t.Fatalf("SourceState() = %#v, want one fresh persisted row", state)
	}
}

// Invariant: source re-key preserves extension data and classifies provenance only from catalog evidence.
// Owner: global database upgrade; canonical suite: global_db_marketplace_catalog_test.go.
func TestMarketplaceCatalogSourceMigration(t *testing.T) {
	t.Parallel()
	t.Run("Should upgrade a v108 catalog losslessly and preserve unclassified provenance", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		previous, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00109_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := previous.Close(); err != nil {
				t.Errorf("close previous: %v", err)
			}
		})
		for _, kind := range []string{"extension", "mcp", "skill"} {
			_, err := previous.ExecContext(ctx, `INSERT INTO marketplace_catalog_entries
 (kind, entry_id, name, description, version, published_at, updated_at, digest_sha256, tier, install_slug, payload_json, fetched_at)
 VALUES (?, 'same-id', ' Original name ', 'Original description', '1.2.3', NULL, '2026-09-01T00:00:00Z', 'digest', 'official', 'compozy/original', '{ "name": "original" }', '2026-09-02T00:00:00Z')`, kind)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := previous.ExecContext(ctx, `INSERT INTO marketplace_catalog_state
 (kind, manifest_version, generated_at, fetched_at, stale, last_error)
 VALUES (?, 2, '2026-09-01T00:00:00Z', '2026-09-02T00:00:00Z', 1, 'cached failure')`, kind); err != nil {
				t.Fatal(err)
			}
		}
		originals := map[string]string{
			"classified":     `{ "installed_from": "marketplace_registry", "catalog_entry_id": "same-id", "checksum_sha256": "tree", "custom": {"retained": true} }`,
			"sideload":       `{ "installed_from": "github", "catalog_entry_id": "same-id", "source_url": "https://github.com/compozy/catalog" }`,
			"no-evidence":    `{ "installed_from": "marketplace_registry", "slug": "compozy/same-id" }`,
			"empty-evidence": `{ "installed_from": "marketplace_registry", "catalog_entry_id": " " }`,
		}
		for name, raw := range originals {
			if _, err := previous.ExecContext(ctx, `INSERT INTO extensions
 (name, version, source, manifest_path, installed_at, checksum, provenance_json)
 VALUES (?, '1.0.0', 'marketplace', '/fixture/extension.toml', '2026-09-01T00:00:00Z', 'tree', ?)`, name, raw); err != nil {
				t.Fatal(err)
			}
		}
		const projection = `json_array(kind,entry_id,name,description,version,published_at,updated_at,digest_sha256,tier,install_slug,payload_json,fetched_at)`
		var before string
		if err := previous.QueryRowContext(ctx, "SELECT "+projection+" FROM marketplace_catalog_entries WHERE kind = 'extension'").
			Scan(&before); err != nil {
			t.Fatal(err)
		}
		if err := previous.Close(); err != nil {
			t.Fatal(err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		if err := upgraded.Close(ctx); err != nil {
			t.Fatal(err)
		}
		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("close reopened: %v", err)
			}
		})
		var after, source string
		if err := reopened.db.QueryRowContext(ctx, "SELECT source,"+projection+" FROM marketplace_catalog_entries").
			Scan(&source, &after); err != nil {
			t.Fatal(err)
		}
		if before != after || source != "compozy-catalog" {
			t.Fatalf("projection source=%q before=%s after=%s", source, before, after)
		}
		var count int
		if err := reopened.db.QueryRowContext(ctx, "SELECT count(*) FROM marketplace_catalog_entries").
			Scan(&count); err != nil ||
			count != 1 {
			t.Fatalf("entries count=%d err=%v", count, err)
		}
		if err := reopened.db.QueryRowContext(ctx, "SELECT count(*) FROM marketplace_catalog_state").
			Scan(&count); err != nil ||
			count != 1 {
			t.Fatalf("states count=%d err=%v", count, err)
		}
		var state string
		if err := reopened.db.QueryRowContext(ctx, `SELECT json_array(source,manifest_version,generated_at,fetched_at,stale,last_error,plugins,installable,generation) FROM marketplace_catalog_state`).
			Scan(&state); err != nil {
			t.Fatal(err)
		}
		if want := `["compozy-catalog",2,"2026-09-01T00:00:00Z","2026-09-02T00:00:00Z",1,"cached failure",1,1,0]`; state != want {
			t.Fatalf("state=%s want=%s", state, want)
		}
		for name, raw := range originals {
			var got string
			if err := reopened.db.QueryRowContext(ctx, "SELECT provenance_json FROM extensions WHERE name = ?", name).
				Scan(&got); err != nil {
				t.Fatal(err)
			}
			if name != "classified" {
				if got != raw {
					t.Fatalf("unclassified %s changed: %s", name, got)
				}
				continue
			}
			var wantMap, gotMap map[string]any
			if err := json.Unmarshal([]byte(raw), &wantMap); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(got), &gotMap); err != nil {
				t.Fatal(err)
			}
			wantMap["source_name"] = "compozy-catalog"
			wantMap["source_ref"] = "catalog:compozy"
			wantMap["entry_id"] = "same-id"
			if !reflect.DeepEqual(gotMap, wantMap) {
				t.Fatalf("classified=%#v want=%#v", gotMap, wantMap)
			}
		}
		status, err := store.Status(ctx, reopened.db, MigrationStream())
		if err != nil {
			t.Fatal(err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
	})
}

// Invariant: one source replacement cannot prune another source or commit an obsolete generation.
// Owner: catalog repository transactions; canonical suite: global_db_marketplace_catalog_test.go (UT-008).
func TestMarketplaceCatalogSourceReplacement(t *testing.T) {
	t.Parallel()
	t.Run("Should replace concurrent sources and reject stale or invalid transactions", func(t *testing.T) {
		t.Parallel()
		db := openFreshTestGlobalDB(t)
		ctx := t.Context()
		at := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
		catalog := openMarketplaceMigrationStore(t, db)
		var wg sync.WaitGroup
		failures := make(chan error, 2)
		for _, source := range []string{"compozy-catalog", "team-plugins"} {
			wg.Go(func() {
				failures <- catalog.ReplaceSource(ctx, source, 0, &marketplace.Document{
					ManifestVersion: marketplace.ManifestVersion, GeneratedAt: at, FetchedAt: at,
					Entries: []marketplace.Entry{{Kind: marketplace.KindExtension, EntryID: "same", Name: source, Description: "preserved", InstallSlug: source + "/same", Payload: json.RawMessage(`{"entry_id":"same"}`)}},
				})
			})
		}
		wg.Wait()
		close(failures)
		for err := range failures {
			if err != nil {
				t.Fatal(err)
			}
		}
		rowsBefore, err := db.ListMarketplaceCatalogEntries(ctx, "team-plugins", 10)
		if err != nil || len(rowsBefore) != 1 {
			t.Fatalf("team rows=%#v err=%v", rowsBefore, err)
		}
		state, err := db.GetMarketplaceCatalogState(ctx, "compozy-catalog")
		if err != nil {
			t.Fatal(err)
		}
		next, err := db.AdvanceMarketplaceCatalogGeneration(ctx, "compozy-catalog")
		if err != nil || next != state.Generation+1 {
			t.Fatalf("next=%d err=%v", next, err)
		}
		replacement := store.MarketplaceCatalogReplacement{
			Source:          "compozy-catalog",
			Generation:      state.Generation,
			ManifestVersion: 2,
			GeneratedAt:     store.FormatTimestamp(at),
			FetchedAt:       store.FormatTimestamp(at),
		}
		if err := db.ReplaceMarketplaceCatalog(
			ctx,
			replacement,
		); !errors.Is(
			err,
			store.ErrMarketplaceCatalogGenerationStale,
		) {
			t.Fatalf("obsolete replacement err=%v", err)
		}
		replacement.Generation = next
		replacement.Entries = []store.MarketplaceCatalogEntry{
			{
				Source:      "compozy-catalog",
				Kind:        "extension",
				EntryID:     "invalid",
				Name:        "Invalid",
				Description: "missing JSON",
				PayloadJSON: "not json",
				FetchedAt:   store.FormatTimestamp(at),
			},
		}
		if err := db.ReplaceMarketplaceCatalog(ctx, replacement); err == nil {
			t.Fatal("invalid transaction succeeded")
		}
		current, err := db.ListMarketplaceCatalogEntries(ctx, "compozy-catalog", 10)
		if err != nil || len(current) != 1 || current[0].EntryID != "same" {
			t.Fatalf("rollback rows=%#v err=%v", current, err)
		}
		other, err := db.ListMarketplaceCatalogEntries(ctx, "team-plugins", 10)
		if err != nil || !reflect.DeepEqual(other, rowsBefore) {
			t.Fatalf("other source=%#v err=%v", other, err)
		}
		replacement.Entries = nil
		replacement.Revision = "empty"
		if err := db.ReplaceMarketplaceCatalog(ctx, replacement); err != nil {
			t.Fatal(err)
		}
		current, err = db.ListMarketplaceCatalogEntries(ctx, "compozy-catalog", 10)
		if err != nil || len(current) != 0 {
			t.Fatalf("pruned source=%#v err=%v", current, err)
		}
		other, err = db.ListMarketplaceCatalogEntries(ctx, "team-plugins", 10)
		if err != nil || !reflect.DeepEqual(other, rowsBefore) {
			t.Fatalf("other source after prune=%#v err=%v", other, err)
		}
	})
}

// Invariant: concurrent replacements never pair a revision with another transaction's entries.
// Owner: catalog repository read transactions; canonical suite: global_db_marketplace_catalog_test.go (UT-068).
func TestMarketplaceCatalogSourceSnapshot(t *testing.T) {
	t.Parallel()
	t.Run("Should bind every concurrent read to one committed content revision", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openFreshTestGlobalDB(t)
		at := store.FormatTimestamp(time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC))
		replacement := func(revision string) store.MarketplaceCatalogReplacement {
			return store.MarketplaceCatalogReplacement{
				Source:          "compozy-catalog",
				Revision:        revision,
				ManifestVersion: 2,
				FetchedAt:       at,
				GeneratedAt:     at,
				Entries: []store.MarketplaceCatalogEntry{
					{Source: "compozy-catalog", Kind: "extension", EntryID: revision,
						Name: revision, Description: "Snapshot", PayloadJSON: "{}", FetchedAt: at},
				},
			}
		}
		if err := db.ReplaceMarketplaceCatalog(ctx, replacement("initial")); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		failures := make(chan error, 2)
		wg.Go(func() {
			for i := range 100 {
				if err := db.ReplaceMarketplaceCatalog(ctx, replacement(fmt.Sprintf("revision-%d", i))); err != nil {
					failures <- err
					return
				}
			}
		})
		wg.Go(func() {
			for range 100 {
				snapshot, err := db.ReadMarketplaceCatalogSnapshot(ctx, "compozy-catalog", 100)
				if err != nil {
					failures <- err
					return
				}
				if len(snapshot.Entries) != 1 || snapshot.Entries[0].EntryID != snapshot.State.Revision ||
					snapshot.State.EntryCount != 1 {
					failures <- fmt.Errorf("mixed snapshot: %#v", snapshot)
					return
				}
			}
		})
		wg.Wait()
		close(failures)
		for err := range failures {
			t.Error(err)
		}
	})
}
