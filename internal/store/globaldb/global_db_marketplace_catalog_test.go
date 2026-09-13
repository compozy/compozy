package globaldb

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/marketplace/pluginsource"
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
		empty := &marketplace.Document{
			ManifestVersion: marketplace.ManifestVersion,
			FetchedAt:       time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC),
			SourceRef:       "github:example/empty", SourceKind: "custom", DocumentDigest: "document-digest",
			DocumentPath: ".claude-plugin/marketplace.json", Owner: "Example",
			Diagnostics: []pluginsource.Diagnostic{{Code: "invalid_plugin", Message: "Missing name"}},
		}
		catalog := openMarketplaceMigrationStore(t, first)
		generation, err := catalog.ConfigureSources(ctx, []marketplace.ResolvedSource{
			{
				Name:    marketplace.CompozyCatalogSource,
				Ref:     marketplace.CompozyCatalogRef,
				Kind:    marketplace.SourceKindFeed,
				Enabled: true,
			},
			{Name: "empty", Ref: empty.SourceRef, Kind: marketplace.SourceKindCustom, Enabled: true},
		}, "empty-fixture")
		if err != nil {
			t.Fatal(err)
		}
		if err := catalog.ReplaceSource(ctx, "empty", generation.Generation, empty); err != nil {
			t.Fatal(err)
		}
		before, err := catalog.SourceState(ctx, "empty")
		if err != nil {
			t.Fatal(err)
		}
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
		after, err := openMarketplaceMigrationStore(t, second).SourceState(ctx, "empty")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("empty source changed across restart: before=%#v after=%#v", before, after)
		}
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
	generation, err := store.ConfigureSources(
		t.Context(),
		[]marketplace.ResolvedSource{
			{
				Name:    marketplace.CompozyCatalogSource,
				Ref:     marketplace.CompozyCatalogRef,
				Kind:    marketplace.SourceKindFeed,
				Enabled: true,
			},
		},
		"feed-fixture",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceSource(
		testutil.Context(t),
		marketplace.CompozyCatalogSource,
		generation.Generation,
		document,
	); err != nil {
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

	// Invariant: source-state upgrades preserve rows and replace error prefixes with durable fields once.
	// Owner: global database upgrade; canonical suite: TestMarketplaceCatalogSourceMigration.
	t.Run("Should preserve v114 sources and separate error metadata across restart", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		previous, err := openGlobalMigrationPrefixDatabase(t, path,
			globalMigrationPrefixBefore(t, "00115_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := previous.Close(); err != nil {
				t.Errorf("close previous: %v", err)
			}
		})
		if _, err := previous.ExecContext(ctx, `INSERT INTO marketplace_catalog_state
(source,manifest_version,fetched_at,stale,last_error,kind_of_source,enabled,plugins,installable,error_class,document_path,owner,revision,generation)
VALUES ('compozy-catalog',3,'2026-09-13T12:00:00Z',1,'[network] unavailable','feed',1,0,0,'','','','feed-rev',4),
       ('team',3,'2026-09-13T12:00:00Z',1,'[remote] missing','custom',0,0,0,'validation','.claude-plugin/marketplace.json','Team','team-rev',7)`); err != nil {
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
		catalog := openMarketplaceMigrationStore(t, reopened)
		for _, want := range []struct {
			name, ref, kind, class, message, path, owner, revision string
			enabled                                                bool
			generation                                             int64
		}{
			{"compozy-catalog", marketplace.CompozyCatalogRef, "feed", "network", "unavailable", "", "", "feed-rev", true, 4},
			{"team", "", "custom", "validation", "[remote] missing", ".claude-plugin/marketplace.json", "Team", "team-rev", false, 7},
		} {
			state, err := catalog.SourceState(ctx, want.name)
			if err != nil {
				t.Fatal(err)
			}
			if state.SourceRef != want.ref || state.Kind != want.kind || state.Enabled != want.enabled ||
				state.ErrorClass != want.class || state.LastError != want.message || state.DocumentPath != want.path ||
				state.Owner != want.owner || state.Revision != want.revision || state.Generation != want.generation ||
				state.ManifestVersion != 3 || !state.Stale || state.FetchedAt.Format(time.RFC3339) != "2026-09-13T12:00:00Z" ||
				state.DocumentDigest != "" || state.Diagnostics == nil || len(state.Diagnostics) != 0 ||
				state.EntryCount != 0 || state.Installable != 0 {
				t.Fatalf("source %q after upgrade and reopen = %#v", want.name, state)
			}
		}
		status, err := store.Status(ctx, reopened.db, MigrationStream())
		if err != nil {
			t.Fatal(err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
	})

	// Invariant: removing the fixed discriminator preserves every variable catalog and source-state value.
	// Owner: global database upgrade; canonical suite: TestMarketplaceCatalogSourceMigration.
	t.Run("Should preserve complete v113 projections while removing the fixed discriminator", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		databasePath := filepath.Join(t.TempDir(), GlobalDatabaseName)
		previous, err := openGlobalMigrationPrefixDatabase(t, databasePath,
			globalMigrationPrefixBefore(t, "00114_marketplace_source_identity.sql"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := previous.Close(); err != nil {
				t.Errorf("close previous: %v", err)
			}
		})
		const entryProjection = "json_array(source,entry_id,name,description,version,published_at,updated_at,digest_sha256,tier,install_slug,payload_json,fetched_at,layout,icon,installable,install_blocker,resolved_ref)"
		const stateProjection = "json_array(source,manifest_version,generated_at,fetched_at,stale,last_error,kind_of_source,enabled,plugins,installable,error_class,document_path,owner,revision,generation)"
		beforeEntries, beforeStates := map[string]string{}, map[string]string{}
		for _, source := range []string{"compozy-catalog", "partner"} {
			if _, err := previous.ExecContext(ctx, `INSERT INTO marketplace_catalog_entries
(source,kind,entry_id,name,description,version,published_at,updated_at,digest_sha256,tier,install_slug,payload_json,fetched_at,layout,icon,installable,install_blocker,resolved_ref)
VALUES (?, 'extension', 'same', ' Original ', 'Résumé', '1.2.3', NULL, '2026-09-01T00:00:00Z', 'digest', 'community', 'owner/same', '{ "entry_id": "same" }', '2026-09-02T00:00:00Z', 'flat', 'https://example.test/icon.png', 0, 'load_failed', 'commit-123')`, source); err != nil {
				t.Fatal(err)
			}
			if _, err := previous.ExecContext(ctx, `INSERT INTO marketplace_catalog_state
(source,manifest_version,generated_at,fetched_at,stale,last_error,kind_of_source,enabled,plugins,installable,error_class,document_path,owner,revision,generation)
VALUES (?,3,'2026-09-01T00:00:00Z','2026-09-02T00:00:00Z',1,'cached error','custom',0,1,0,'validation','/fixture/marketplace.json','owner','revision-a',7)`, source); err != nil {
				t.Fatal(err)
			}
			var entry, state string
			if err := previous.QueryRowContext(ctx, "SELECT "+entryProjection+" FROM marketplace_catalog_entries WHERE source = ?", source).
				Scan(&entry); err != nil {
				t.Fatal(err)
			}
			if err := previous.QueryRowContext(ctx, "SELECT "+stateProjection+" FROM marketplace_catalog_state WHERE source = ?", source).
				Scan(&state); err != nil {
				t.Fatal(err)
			}
			beforeEntries[source], beforeStates[source] = entry, state
		}
		if err := previous.Close(); err != nil {
			t.Fatal(err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, databasePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := upgraded.Close(ctx); err != nil {
			t.Fatal(err)
		}
		reopened, err := OpenGlobalDB(ctx, databasePath)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("close upgraded: %v", err)
			}
		})
		for source, before := range beforeEntries {
			var entry, state string
			if err := reopened.db.QueryRowContext(ctx, "SELECT "+entryProjection+" FROM marketplace_catalog_entries WHERE source = ?", source).
				Scan(&entry); err != nil {
				t.Fatal(err)
			}
			if err := reopened.db.QueryRowContext(ctx, "SELECT "+stateProjection+" FROM marketplace_catalog_state WHERE source = ?", source).
				Scan(&state); err != nil {
				t.Fatal(err)
			}
			if entry != before || state != beforeStates[source] {
				t.Fatalf("source %q changed: entry=%s state=%s", source, entry, state)
			}
		}
		status, err := store.Status(ctx, reopened.db, MigrationStream())
		if err != nil {
			t.Fatal(err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
	})
	// Invariant: the complete upgrade preserves installed state and credentials while replacing only retired catalog rows.
	// Owner: global database migration; canonical suite: TestMarketplaceCatalogSourceMigration, IT-016.
	t.Run("Should preserve the complete v109 marketplace upgrade across repeated boots [IT-016]", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		previous, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00110_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := previous.Close(); err != nil {
				t.Errorf("close previous: %v", err)
			}
		})
		for _, family := range []struct {
			kind  string
			count int
		}{{"extension", 3}, {"mcp", 17}, {"skill", 1}} {
			kind := family.kind
			for i := range family.count {
				id := "same-id"
				if i > 0 {
					id = fmt.Sprintf("%s-%02d", kind, i)
				}
				_, err := previous.ExecContext(ctx, `INSERT INTO marketplace_catalog_entries
 (kind, entry_id, name, description, version, published_at, updated_at, digest_sha256, tier, install_slug, payload_json, fetched_at)
 VALUES (?, ?, ' Original name ', 'Original description', '1.2.3', NULL, '2026-09-01T00:00:00Z', 'digest', 'official', 'compozy/original', '{ "name": "original" }', '2026-09-02T00:00:00Z')`, kind, id)
				if err != nil {
					t.Fatal(err)
				}
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
		packageBytes := make(map[string]string, len(originals))
		for name, raw := range originals {
			manifestPath := filepath.Join(filepath.Dir(path), name+".toml")
			contents := fmt.Sprintf("name = %q\nversion = \"1.0.0\"\n", name)
			if err := os.WriteFile(manifestPath, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			packageBytes[manifestPath] = contents
			if _, err := previous.ExecContext(ctx, `INSERT INTO extensions
 (name, version, source, manifest_path, installed_at, checksum, provenance_json)
 VALUES (?, '1.0.0', 'marketplace', ?, '2026-09-01T00:00:00Z', 'tree', ?)`, name, manifestPath, raw); err != nil {
				t.Fatal(err)
			}
		}
		preservedQueries := seedMarketplaceUpgradeState(t, previous)
		preservedBefore := captureMarketplaceUpgradeState(t, previous, preservedQueries)
		const projection = `json_array(entry_id,name,description,version,published_at,updated_at,digest_sha256,tier,install_slug,payload_json,fetched_at)`
		var before string
		if err := previous.QueryRowContext(ctx, "SELECT json_group_array(json("+projection+")) FROM (SELECT * FROM marketplace_catalog_entries WHERE kind = 'extension' ORDER BY entry_id)").
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
		for boot := range 2 {
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
			if err := reopened.db.QueryRowContext(ctx, "SELECT source, json_group_array(json("+projection+")) FROM (SELECT * FROM marketplace_catalog_entries ORDER BY entry_id)").
				Scan(&source, &after); err != nil {
				t.Fatal(err)
			}
			if before != after || source != "compozy-catalog" {
				t.Fatalf("projection source=%q before=%s after=%s", source, before, after)
			}
			var count int
			if err := reopened.db.QueryRowContext(ctx, "SELECT count(*) FROM marketplace_catalog_entries").
				Scan(&count); err != nil ||
				count != 3 {
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
			if want := `["compozy-catalog",2,"2026-09-01T00:00:00Z","2026-09-02T00:00:00Z",1,"cached failure",3,3,0]`; state != want {
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
			afterState := captureMarketplaceUpgradeState(t, reopened.db, preservedQueries)
			if !reflect.DeepEqual(afterState, preservedBefore) {
				t.Fatalf("upgrade changed installed state on boot %d", boot)
			}
			assertMarketplaceUpgradeBackfills(t, reopened.db, len(originals))
			if boot == 1 {
				refreshMarketplaceUpgradeCatalog(t, reopened)
				afterRefresh := captureMarketplaceUpgradeState(t, reopened.db, preservedQueries)
				if !reflect.DeepEqual(afterRefresh, preservedBefore) {
					t.Fatal("catalog refresh changed installed state")
				}
			}
			for manifestPath, before := range packageBytes {
				after, err := os.ReadFile(manifestPath)
				if err != nil || string(after) != before {
					t.Fatalf("upgrade changed installed manifest %s: %v", manifestPath, err)
				}
			}
			if err := reopened.Close(ctx); err != nil {
				t.Fatal(err)
			}
		}
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
		generation, err := catalog.ConfigureSources(ctx, []marketplace.ResolvedSource{
			{
				Name:    marketplace.CompozyCatalogSource,
				Ref:     marketplace.CompozyCatalogRef,
				Kind:    marketplace.SourceKindFeed,
				Enabled: true,
			},
			{Name: "team-plugins", Kind: marketplace.SourceKindCustom, Enabled: true},
		}, "concurrent-fixture")
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		failures := make(chan error, 2)
		for _, source := range []string{"compozy-catalog", "team-plugins"} {
			wg.Go(func() {
				failures <- catalog.ReplaceSource(ctx, source, generation.Generation, &marketplace.Document{
					ManifestVersion: marketplace.ManifestVersion, GeneratedAt: at, FetchedAt: at,
					Entries: []marketplace.Entry{{EntryID: "same", Name: source, Description: "preserved", InstallSlug: source + "/same", Payload: json.RawMessage(`{"entry_id":"same"}`)}},
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
		configuration, err := db.ConfigureMarketplaceSources(ctx, []store.MarketplaceSourceDefinition{
			{Name: "compozy-catalog", Ref: marketplace.CompozyCatalogRef, Kind: marketplace.SourceKindFeed,
				Enabled: true, ConfigurationRevision: "changed-acquisition"},
			{Name: "team-plugins", Kind: marketplace.SourceKindCustom, Enabled: true},
		}, "changed-configuration")
		next := configuration.SourceGenerations["compozy-catalog"]
		if err != nil || next != state.Generation+1 {
			t.Fatalf("next=%d err=%v", next, err)
		}
		replacement := store.MarketplaceCatalogReplacement{
			Source:          "compozy-catalog",
			SourceRef:       marketplace.CompozyCatalogRef,
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
		configuration, err := db.ConfigureMarketplaceSources(
			ctx,
			[]store.MarketplaceSourceDefinition{
				{Name: "compozy-catalog", Ref: marketplace.CompozyCatalogRef, Kind: "feed", Enabled: true},
			},
			"snapshot-fixture",
		)
		if err != nil {
			t.Fatal(err)
		}
		replacement := func(revision string) store.MarketplaceCatalogReplacement {
			return store.MarketplaceCatalogReplacement{
				Source:    "compozy-catalog",
				SourceRef: marketplace.CompozyCatalogRef, Generation: configuration.Generation,
				Revision:        revision,
				ManifestVersion: 2,
				FetchedAt:       at,
				GeneratedAt:     at,
				Entries: []store.MarketplaceCatalogEntry{
					{Source: "compozy-catalog", EntryID: revision,
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

func seedMarketplaceUpgradeState(t *testing.T, db *sql.DB) []string {
	t.Helper()
	ctx := t.Context()
	statements := []string{
		`INSERT INTO workspaces(id, root_dir, name, created_at, updated_at)
 VALUES ('upgrade-workspace', '/upgrade', 'Upgrade', '2026-09-01T00:00:00Z', '2026-09-02T00:00:00Z')`,
		`INSERT INTO extension_profile_enablement(extension_name, profile_id, enabled)
 VALUES ('classified', '00000000000000000000000000', 0)`,
		`INSERT INTO extension_env_bindings
 (extension_name, profile_id, workspace_id, env_name, secret_ref, mcp_server, header_name, kind, created_at, updated_at)
 VALUES ('classified', '', '', 'API_KEY', 'vault:extensions/global/classified/env/API_KEY', '', '',
 'extension_env', '2026-09-01T00:00:00Z', '2026-09-02T00:00:00Z')`,
		`INSERT INTO extension_env_bindings
 (extension_name, profile_id, workspace_id, env_name, secret_ref, mcp_server, header_name, kind, created_at, updated_at)
 VALUES ('classified', '00000000000000000000000000', 'upgrade-workspace', 'HEADER_TOKEN',
 'vault:extensions/ws/upgrade-workspace/classified/env/HEADER_TOKEN', 'remote', 'Authorization',
 'extension_env', '2026-09-01T00:00:00Z', '2026-09-03T00:00:00Z')`,
		`INSERT INTO vault_secrets(ref, kind, encrypted_value, created_at, updated_at)
 SELECT secret_ref, 'extension_env', 'ciphertext:' || env_name, created_at, updated_at FROM extension_env_bindings`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	queries := seedMCPOwnerMigrationFixture(t, db)
	return append(queries,
		`SELECT json_group_array(json_array(name,version,source,manifest_path,format,ingest_diagnostics_json,
 installed_at,provides_json,permissions_json,checksum,lifecycle_token,registry_slug,registry_name,remote_version,
 network_requirement_digest,network_confirmed_by,network_confirmed_at)) FROM (SELECT * FROM extensions ORDER BY name)`,
		`SELECT json_group_array(json_array(extension_name,profile_id,enabled))
 FROM (SELECT * FROM extension_profile_enablement ORDER BY extension_name,profile_id)`,
		`SELECT json_group_array(json_array(extension_name,profile_id,workspace_id,env_name,secret_ref,
 mcp_server,header_name,kind,created_at,updated_at))
 FROM (SELECT * FROM extension_env_bindings ORDER BY extension_name,profile_id,workspace_id,env_name)`,
	)
}

func captureMarketplaceUpgradeState(t *testing.T, db *sql.DB, queries []string) []string {
	t.Helper()
	values := make([]string, len(queries))
	for i, query := range queries {
		if err := db.QueryRowContext(t.Context(), query).Scan(&values[i]); err != nil {
			t.Fatal(err)
		}
	}
	return values
}

func assertMarketplaceUpgradeBackfills(t *testing.T, db *sql.DB, installations int) {
	t.Helper()
	for _, expected := range []struct {
		query string
		count int
	}{
		{`SELECT count(*) FROM extension_inputs`, 0},
		{`SELECT count(*) FROM extension_mcp_overrides`, 0},
		{`SELECT count(*) FROM extension_env_bindings WHERE input_id = '' AND active = 1`, 2},
		{`SELECT count(*) FROM mcp_auth_tokens WHERE owner = 'manual'`, 1},
		{`SELECT count(*) FROM mcp_oauth_registrations WHERE owner = 'manual'`, 1},
		{`SELECT count(*) FROM extension_installations`, installations},
		{`SELECT count(*) FROM extension_installations i JOIN extensions e ON e.name = i.extension_name
 WHERE i.profile_id = '' AND i.workspace_id = '' AND i.created_at = e.installed_at`, installations},
	} {
		var count int
		if err := db.QueryRowContext(t.Context(), expected.query).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != expected.count {
			t.Fatalf("upgrade invariant %s = %d, want %d", expected.query, count, expected.count)
		}
	}
}

func refreshMarketplaceUpgradeCatalog(t *testing.T, db *GlobalDB) {
	t.Helper()
	ctx := t.Context()
	const provenanceQuery = `SELECT json_group_array(json_array(name,provenance_json)) FROM (SELECT * FROM extensions ORDER BY name)`
	before := captureMarketplaceUpgradeState(t, db.db, []string{provenanceQuery})
	server := httptest.NewServer(http.StripPrefix("/custom-base", http.FileServer(http.Dir("../../../catalog"))))
	t.Cleanup(server.Close)
	client := server.Client()
	client.Timeout = 10 * time.Second
	source, err := marketplace.NewHTTPSource(server.URL+"/custom-base", client)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := marketplace.NewSQLiteStore(db)
	if err != nil {
		t.Fatal(err)
	}
	service, err := marketplace.NewService(
		ctx,
		catalog,
		[]marketplace.SourceBinding{
			{
				Config: marketplace.ResolvedSource{
					Name:    marketplace.CompozyCatalogSource,
					Ref:     marketplace.CompozyCatalogRef,
					Kind:    marketplace.SourceKindFeed,
					Enabled: true,
				},
				Fetcher: source,
			},
		},
		time.Hour,
		10*time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Close(testutil.Context(t)); err != nil {
			t.Error(err)
		}
	})
	if _, err := service.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	page, err := catalog.BrowseSource(ctx, marketplace.CompozyCatalogSource, "", 0, 100)
	if err != nil || page.Total != 20 || len(page.Entries) != 20 {
		t.Fatalf("v3 refresh after upgrade = %d/%d entries: %v", page.Total, len(page.Entries), err)
	}
	if after := captureMarketplaceUpgradeState(t, db.db, []string{provenanceQuery}); !reflect.DeepEqual(before, after) {
		t.Fatal("catalog refresh reclassified existing installations")
	}
	if err := service.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

// Invariant: membership, identity fences and retained installation names change atomically.
// Owner: global marketplace transactions; canonical suite: global_db_marketplace_catalog_test.go.
func TestMarketplaceCatalogSourceConfiguration(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should preserve disabled rows and prevent removed or replaced sources from being recreated by a late writer",
		func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db := openFreshTestGlobalDB(t)
			sources := []store.MarketplaceSourceDefinition{
				{Name: "compozy-catalog", Ref: marketplace.CompozyCatalogRef, Kind: "feed", Enabled: true},
				{Name: "team", Ref: "github:team/plugins", Kind: "custom", Enabled: true},
			}
			first, err := db.ConfigureMarketplaceSources(ctx, sources, "first")
			if err != nil {
				t.Fatal(err)
			}
			at := store.FormatTimestamp(time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC))
			projection := store.MarketplaceCatalogReplacement{
				Source:          "team",
				SourceRef:       sources[1].Ref,
				Kind:            "custom",
				Generation:      first.SourceGenerations["team"],
				ManifestVersion: 3,
				FetchedAt:       at,
				Entries: []store.MarketplaceCatalogEntry{
					{
						Source:      "team",
						EntryID:     "tool",
						Name:        "Tool",
						PayloadJSON: "{}",
						FetchedAt:   at,
						Installable: true,
					},
				},
			}
			if err := db.ReplaceMarketplaceCatalog(ctx, projection); err != nil {
				t.Fatal(err)
			}
			sources[1].Enabled = false
			disabled, err := db.ConfigureMarketplaceSources(ctx, sources, "disabled")
			if err != nil || disabled.Generation <= first.Generation ||
				disabled.SourceGenerations["compozy-catalog"] != first.SourceGenerations["compozy-catalog"] {
				t.Fatalf("disabled configuration = %+v, %v", disabled, err)
			}
			rows, err := db.ListMarketplaceCatalogEntries(ctx, "team", 10)
			if err != nil || len(rows) != 1 {
				t.Fatalf("disabled rows were lost: %+v, %v", rows, err)
			}
			snapshot, err := db.ReadMarketplaceCatalogSources(ctx, []string{"compozy-catalog", "team"}, 10)
			if err != nil || snapshot.Generation != disabled.Generation || len(snapshot.Sources[1].Entries) != 0 ||
				snapshot.Sources[1].State.EntryCount != 1 {
				t.Fatalf("disabled read snapshot = %+v, %v", snapshot, err)
			}
			projection.Generation = disabled.SourceGenerations["team"]
			if err := db.ReplaceMarketplaceCatalog(
				ctx,
				projection,
			); !errors.Is(
				err,
				store.ErrMarketplaceCatalogGenerationStale,
			) {
				t.Fatalf("disabled source accepted a refresh: %v", err)
			}
			removed, err := db.ConfigureMarketplaceSources(ctx, sources[:1], "removed")
			if err != nil {
				t.Fatal(err)
			}
			if err := db.ReplaceMarketplaceCatalog(
				ctx,
				projection,
			); !errors.Is(
				err,
				store.ErrMarketplaceCatalogGenerationStale,
			) {
				t.Fatalf("removed source was recreated: %v", err)
			}
			if err := db.MarkMarketplaceCatalogStale(
				ctx,
				"team",
				projection.Generation,
				"network",
				"old error",
			); !errors.Is(
				err,
				store.ErrMarketplaceCatalogGenerationStale,
			) {
				t.Fatalf("removed error state was recreated: %v", err)
			}
			sources[1].Enabled = true
			readded, err := db.ConfigureMarketplaceSources(ctx, sources, "readded")
			if err != nil || readded.Generation <= removed.Generation ||
				readded.SourceGenerations["team"] <= projection.Generation {
				t.Fatalf("readded fence = %+v, %v", readded, err)
			}
			if err := db.ReplaceMarketplaceCatalog(
				ctx,
				projection,
			); !errors.Is(
				err,
				store.ErrMarketplaceCatalogGenerationStale,
			) {
				t.Fatalf("old incarnation overwrote the new source: %v", err)
			}
			projection.Generation = readded.SourceGenerations["team"]
			projection.SourceRef = "github:other/plugins"
			if err := db.ReplaceMarketplaceCatalog(
				ctx,
				projection,
			); !errors.Is(
				err,
				store.ErrMarketplaceCatalogGenerationStale,
			) {
				t.Fatalf("wrong origin accepted: %v", err)
			}
			unchanged, err := db.ConfigureMarketplaceSources(ctx, sources, "readded")
			if err != nil || !reflect.DeepEqual(unchanged, readded) {
				t.Fatalf("no-op changed configuration: %+v, %v", unchanged, err)
			}
		},
	)
	t.Run("Should roll back a retained-name conflict and allow the same origin under a new name", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openFreshTestGlobalDB(t)
		source := store.MarketplaceSourceDefinition{
			Name:    "team",
			Ref:     "github:team/plugins",
			Kind:    "custom",
			Enabled: true,
		}
		first, err := db.ConfigureMarketplaceSources(ctx, []store.MarketplaceSourceDefinition{source}, "first")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(
			ctx,
			`INSERT INTO extensions (name,version,source,manifest_path,installed_at,checksum,provenance_json)
VALUES ('tool','1.0.0','marketplace','/managed/tool','2026-09-13T12:00:00Z','checksum','{"source_name":"team","source_ref":"github:team/plugins","entry_id":"tool"}')`,
		); err != nil {
			t.Fatal(err)
		}
		other := source
		other.Ref = "github:other/plugins"
		_, err = db.ConfigureMarketplaceSources(ctx, []store.MarketplaceSourceDefinition{other}, "conflict")
		retained, ok := errors.AsType[*store.MarketplaceSourceNameRetainedError](err)
		if !ok || !errors.Is(err, store.ErrMarketplaceSourceNameRetained) || retained.Source != "team" ||
			!reflect.DeepEqual(retained.RetainedBy, []string{"tool"}) {
			t.Fatalf("retained-name error = %#v", err)
		}
		preserved, err := db.GetMarketplaceCatalogState(ctx, "team")
		if err != nil || preserved.SourceRef != source.Ref || preserved.Generation != first.SourceGenerations["team"] {
			t.Fatalf("failed configuration changed source: %+v, %v", preserved, err)
		}
		current, err := db.ConfigureMarketplaceSources(ctx, []store.MarketplaceSourceDefinition{source}, "first")
		if err != nil || current.Generation != first.Generation {
			t.Fatalf("failed configuration advanced the committed generation: %+v, %v", current, err)
		}
		source.Name = "renamed"
		if _, err := db.ConfigureMarketplaceSources(
			ctx,
			[]store.MarketplaceSourceDefinition{source},
			"renamed",
		); err != nil {
			t.Fatal(err)
		}
		var provenance string
		if err := db.db.QueryRowContext(ctx, "SELECT provenance_json FROM extensions WHERE name = 'tool'").
			Scan(&provenance); err != nil {
			t.Fatal(err)
		}
		if provenance != `{"source_name":"team","source_ref":"github:team/plugins","entry_id":"tool"}` {
			t.Fatalf("source mutation rewrote installed provenance: %s", provenance)
		}
		if _, err := db.ConfigureMarketplaceSources(
			ctx,
			[]store.MarketplaceSourceDefinition{source, other},
			"reuse-old-name",
		); !errors.Is(
			err,
			store.ErrMarketplaceSourceNameRetained,
		) {
			t.Fatalf("removed name was reassigned while retained: %v", err)
		}
	})
}
