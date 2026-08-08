package store_test

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	atlasschema "ariga.io/atlas/sql/schema"
	atlassqlite "ariga.io/atlas/sql/sqlite"
	"github.com/compozy/compozy/internal/memory"
	memoryschema "github.com/compozy/compozy/internal/memory/schema"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	globalschema "github.com/compozy/compozy/internal/store/globaldb/schema"
	"github.com/compozy/compozy/internal/store/sessiondb"
	sessionschema "github.com/compozy/compozy/internal/store/sessiondb/schema"
	"github.com/compozy/compozy/internal/store/workspacedb"
	workspaceschema "github.com/compozy/compozy/internal/store/workspacedb/schema"
	"github.com/compozy/compozy/internal/testutil"
	_ "modernc.org/sqlite"
)

type productionMigrationStream struct {
	name              string
	stream            store.MigrationStream
	schemaFS          fs.FS
	declarativeSource string
}

func productionMigrationStreams() []productionMigrationStream {
	return []productionMigrationStream{
		{
			name:              "global",
			stream:            globaldb.MigrationStream(),
			schemaFS:          globalschema.Files,
			declarativeSource: "definitions",
		},
		{
			name:              "session",
			stream:            sessiondb.MigrationStream(),
			schemaFS:          sessionschema.Files,
			declarativeSource: "schema.sql",
		},
		{
			name:              "memory",
			stream:            memory.MigrationStream(),
			schemaFS:          memoryschema.Files,
			declarativeSource: "schema.sql",
		},
		{
			name:              "workspace",
			stream:            workspacedb.MigrationStream(),
			schemaFS:          workspaceschema.Files,
			declarativeSource: "schema.sql",
		},
	}
}

func TestProductionMigrationStreams(t *testing.T) {
	t.Run("Should embed four distinct sequential baseline streams", func(t *testing.T) {
		t.Parallel()

		seenTables := make(map[string]string)
		for _, item := range productionMigrationStreams() {
			if item.stream.Name != item.name {
				t.Fatalf("stream name = %q, want %q", item.stream.Name, item.name)
			}
			if owner, exists := seenTables[item.stream.VersionTable]; exists {
				t.Fatalf("version table %q shared by %s and %s", item.stream.VersionTable, owner, item.name)
			}
			seenTables[item.stream.VersionTable] = item.name
			versions := embeddedMigrationVersions(t, item.stream)
			foundBaseline := false
			for _, version := range versions {
				if version == 1 {
					foundBaseline = true
				}
			}
			sort.Ints(versions)
			if !foundBaseline || len(versions) == 0 {
				t.Fatalf("%s migrations have no 00001_baseline.sql", item.name)
			}
			for index, version := range versions {
				if want := index + 1; version != want {
					t.Fatalf("%s migration versions = %v, want sequential versions from 1", item.name, versions)
				}
			}
			if _, err := fs.ReadFile(item.stream.FS, item.stream.Dir+"/atlas.sum"); err != nil {
				t.Fatalf("read %s atlas.sum: %v", item.name, err)
			}
		}
	})

	t.Run("Should keep global and memory domain table ownership disjoint", func(t *testing.T) {
		t.Parallel()

		globalTables := schemaOwnedTables(t, globalschema.Files, "definitions")
		memoryTables := schemaOwnedTables(t, memoryschema.Files, "schema.sql")
		for table := range globalTables {
			if memoryTables[table] {
				t.Fatalf("table %q is owned by both global and memory baselines", table)
			}
		}
		if globalTables["memory_events"] {
			t.Fatal("global baseline owns memory_events, want memory stream ownership")
		}
		if !memoryTables["memory_events"] {
			t.Fatal("memory baseline does not own memory_events")
		}
	})

	t.Run("Should apply global and memory baselines to one physical database", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openStreamTestDB(t, "shared-compozy.db")
		globalStream := globaldb.MigrationStream()
		memoryStream := memory.MigrationStream()
		if err := store.Apply(ctx, db, globalStream); err != nil {
			t.Fatalf("Apply(global) error = %v", err)
		}
		if err := store.Apply(ctx, db, memoryStream); err != nil {
			t.Fatalf("Apply(memory) error = %v", err)
		}
		for _, stream := range []store.MigrationStream{globalStream, memoryStream} {
			status, err := store.Status(ctx, db, stream)
			if err != nil {
				t.Fatalf("Status(%s) error = %v", stream.Name, err)
			}
			versions := embeddedMigrationVersions(t, stream)
			wantVersion := int64(versions[len(versions)-1])
			if status.Version != wantVersion || status.AppliedCount != len(versions) {
				t.Fatalf(
					"Status(%s) = %#v, want version %d with %d applied migrations",
					stream.Name,
					status,
					wantVersion,
					len(versions),
				)
			}
		}
		if !sqliteTableExists(t, db, "memory_events") {
			t.Fatal("memory_events missing after shared-file baseline application")
		}
	})
}

func TestProductionMigrationStreamsFreshReopenAndAhead(t *testing.T) {
	for _, item := range productionMigrationStreams() {
		name := "Should fresh-apply, reopen, and reject an ahead " + item.name + " stream"
		if item.name == "global" {
			name += " [UT-160]"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			stream := item.stream
			stream.Bootstrap = nil
			versions := embeddedMigrationVersions(t, stream)
			head := int64(versions[len(versions)-1])
			path := filepath.Join(t.TempDir(), item.name+"-reopen.db")
			first, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatalf("open fresh %s stream: %v", item.name, err)
			}
			firstOpen := true
			t.Cleanup(func() {
				if firstOpen {
					if err := first.Close(); err != nil {
						t.Errorf("close fresh %s stream after failure: %v", item.name, err)
					}
				}
			})
			if err := store.Apply(ctx, first, stream); err != nil {
				t.Fatalf("Apply(%s fresh) error = %v", item.name, err)
			}
			before, err := store.Status(ctx, first, stream)
			if err != nil {
				t.Fatalf("Status(%s fresh) error = %v", item.name, err)
			}
			if before.Version != head || before.AppliedCount != len(versions) {
				t.Fatalf(
					"Status(%s fresh) = %#v, want head %d with %d migrations",
					item.name,
					before,
					head,
					len(versions),
				)
			}
			if err := first.Close(); err != nil {
				t.Fatalf("close fresh %s stream: %v", item.name, err)
			}
			firstOpen = false

			reopened, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatalf("reopen %s stream: %v", item.name, err)
			}
			t.Cleanup(func() {
				if err := reopened.Close(); err != nil {
					t.Errorf("close reopened %s stream: %v", item.name, err)
				}
			})
			if err := store.Apply(ctx, reopened, stream); err != nil {
				t.Fatalf("Apply(%s reopen) error = %v", item.name, err)
			}
			after, err := store.Status(ctx, reopened, stream)
			if err != nil {
				t.Fatalf("Status(%s reopen) error = %v", item.name, err)
			}
			if after != before {
				t.Fatalf("Status(%s reopen) = %#v, want %#v", item.name, after, before)
			}

			aheadDB := openStreamTestDB(t, item.name+"-ahead.db")
			if _, err := aheadDB.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE %q (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				version_id INTEGER NOT NULL,
				is_applied INTEGER NOT NULL,
				tstamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`, stream.VersionTable)); err != nil {
				t.Fatalf("create %s version table: %v", item.name, err)
			}
			if _, err := aheadDB.ExecContext(
				ctx,
				fmt.Sprintf("INSERT INTO %q (version_id, is_applied) VALUES (?, 1)", stream.VersionTable),
				head+1,
			); err != nil {
				t.Fatalf("seed ahead %s version: %v", item.name, err)
			}
			if err := store.Apply(ctx, aheadDB, stream); !errors.Is(err, store.ErrSchemaAhead) {
				t.Fatalf("Apply(%s ahead) error = %v, want ErrSchemaAhead", item.name, err)
			}
		})
	}
}

func TestGlobalExtensionManifestV2Migration(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openStreamTestDB(t, "extension-manifest-v2.db")
	stream := globaldb.MigrationStream()
	if err := store.Apply(ctx, db, migrationPrefixStream(t, stream, 27)); err != nil {
		t.Fatalf("Apply(global prefix) error = %v", err)
	}
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO extensions (
			name, version, source, manifest_path, installed_at,
			capabilities, actions, checksum, provenance_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"fixture-ext",
		"0.1.0",
		"user",
		"/extensions/fixture-ext/extension.toml",
		"2026-07-28T00:00:00Z",
		`{"provides":["memory.backend","tool.provider"]}`,
		`{"requires":["memory/store","sessions/list"]}`,
		"sha256:fixture",
		`{"source_kind":"local"}`,
	); err != nil {
		t.Fatalf("seed manifest v1 extension: %v", err)
	}

	stream.Bootstrap = nil
	if err := store.Apply(ctx, db, stream); err != nil {
		t.Fatalf("Apply(global) error = %v", err)
	}

	var providesJSON string
	var permissionsJSON string
	if err := db.QueryRowContext(
		ctx,
		`SELECT provides_json, permissions_json FROM extensions WHERE name = ?`,
		"fixture-ext",
	).Scan(&providesJSON, &permissionsJSON); err != nil {
		t.Fatalf("query migrated extension: %v", err)
	}
	if got, want := providesJSON, `["memory.backend","tool.provider"]`; got != want {
		t.Fatalf("provides_json = %q, want %q", got, want)
	}
	if got, want := permissionsJSON, `["memory/store","sessions/list"]`; got != want {
		t.Fatalf("permissions_json = %q, want %q", got, want)
	}
	for _, table := range []string{"extensions", "extension_dev_links"} {
		if !sqliteTableExists(t, db, table) {
			t.Fatalf("table %q missing after manifest v2 migration", table)
		}
	}
	columns := sqliteColumns(t, db, "extensions")
	for _, removed := range []string{"capabilities", "actions"} {
		if columns[removed] {
			t.Fatalf("extensions column %q remains after manifest v2 migration", removed)
		}
	}
	for _, added := range []string{"provides_json", "permissions_json"} {
		if !columns[added] {
			t.Fatalf("extensions column %q missing after manifest v2 migration", added)
		}
	}
}

func migrationPrefixStream(t *testing.T, stream store.MigrationStream, head int) store.MigrationStream {
	t.Helper()

	files := fstest.MapFS{}
	directory := &atlasmigrate.MemDir{}
	entries, err := fs.ReadDir(stream.FS, stream.Dir)
	if err != nil {
		t.Fatalf("read %s migration directory: %v", stream.Name, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		separator := strings.IndexByte(entry.Name(), '_')
		if separator <= 0 {
			t.Fatalf("migration filename %q has no version prefix", entry.Name())
		}
		version, err := strconv.Atoi(entry.Name()[:separator])
		if err != nil {
			t.Fatalf("parse migration version from %q: %v", entry.Name(), err)
		}
		if version > head {
			continue
		}
		contents, err := fs.ReadFile(stream.FS, path.Join(stream.Dir, entry.Name()))
		if err != nil {
			t.Fatalf("read migration %q: %v", entry.Name(), err)
		}
		if err := directory.WriteFile(entry.Name(), contents); err != nil {
			t.Fatalf("write migration prefix %q: %v", entry.Name(), err)
		}
		files[path.Join(stream.Dir, entry.Name())] = &fstest.MapFile{Data: contents}
	}
	checksum, err := directory.Checksum()
	if err != nil {
		t.Fatalf("checksum %s migration prefix: %v", stream.Name, err)
	}
	checksumBytes, err := checksum.MarshalText()
	if err != nil {
		t.Fatalf("marshal %s migration prefix checksum: %v", stream.Name, err)
	}
	files[path.Join(stream.Dir, atlasmigrate.HashFileName)] = &fstest.MapFile{Data: checksumBytes}

	stream.FS = files
	stream.Bootstrap = nil
	return stream
}

func embeddedMigrationVersions(t *testing.T, stream store.MigrationStream) []int {
	t.Helper()

	entries, err := fs.ReadDir(stream.FS, stream.Dir)
	if err != nil {
		t.Fatalf("read %s migration directory: %v", stream.Name, err)
	}
	versions := make([]int, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		separator := strings.IndexByte(entry.Name(), '_')
		if separator <= 0 {
			t.Fatalf("%s migration filename %q has no version prefix", stream.Name, entry.Name())
		}
		version, err := strconv.Atoi(entry.Name()[:separator])
		if err != nil {
			t.Fatalf("parse %s migration version: %v", stream.Name, err)
		}
		if version == 1 && entry.Name() != "00001_baseline.sql" {
			t.Fatalf("%s first migration = %q, want 00001_baseline.sql", stream.Name, entry.Name())
		}
		versions = append(versions, version)
	}
	if len(versions) == 0 {
		t.Fatalf("%s migration directory contains no SQL migrations", stream.Name)
	}
	sort.Ints(versions)
	return versions
}

func TestMigrationSchemaEquivalence(t *testing.T) {
	for _, item := range productionMigrationStreams() {
		name := "Should match the declarative schema for the " + item.name + " stream"
		if item.name == "global" {
			name += " [UT-160]"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			replayStream := item.stream
			replayStream.Bootstrap = nil
			migrationDB := openStreamTestDB(t, item.name+"-migration.db")
			if err := store.Apply(ctx, migrationDB, replayStream); err != nil {
				t.Fatalf("Apply(%s) error = %v", item.name, err)
			}
			if _, err := migrationDB.ExecContext(ctx, "DROP TABLE "+item.stream.VersionTable); err != nil {
				t.Fatalf("drop %s migration version table: %v", item.name, err)
			}
			schemaDB := openStreamTestDB(t, item.name+"-schema.db")
			executeDeclarativeSchema(t, schemaDB, item.schemaFS, item.declarativeSource)
			assertMigrationSchemaEquivalent(t, item.name, migrationDB, schemaDB, item.stream.VersionTable)

			bootstrapDB := openStreamTestDB(t, item.name+"-bootstrap.db")
			ctx = testutil.Context(t)
			if err := store.Apply(ctx, bootstrapDB, item.stream); err != nil {
				t.Fatalf("Apply(%s bootstrap) error = %v", item.name, err)
			}
			assertMigrationSchemaEquivalent(
				t,
				item.name+" bootstrap",
				bootstrapDB,
				migrationDB,
				item.stream.VersionTable,
			)
			if got, want := normalizedSQLiteTableCounts(t, bootstrapDB, item.stream.VersionTable),
				normalizedSQLiteTableCounts(t, migrationDB, item.stream.VersionTable); got != want {
				t.Fatalf("%s bootstrap row counts = %q, want replay counts %q", item.name, got, want)
			}
		})
	}
}

func assertMigrationSchemaEquivalent(
	t *testing.T,
	streamName string,
	migrationDB *sql.DB,
	declarativeDB *sql.DB,
	versionTable string,
) {
	t.Helper()
	ctx := testutil.Context(t)
	migrationDriver, err := atlassqlite.Open(migrationDB)
	if err != nil {
		t.Fatalf("open %s migration schema inspector: %v", streamName, err)
	}
	declarativeDriver, err := atlassqlite.Open(declarativeDB)
	if err != nil {
		t.Fatalf("open %s declarative schema inspector: %v", streamName, err)
	}
	migrationRealm, err := migrationDriver.InspectRealm(ctx, nil)
	if err != nil {
		t.Fatalf("inspect %s migration schema: %v", streamName, err)
	}
	declarativeRealm, err := declarativeDriver.InspectRealm(ctx, nil)
	if err != nil {
		t.Fatalf("inspect %s declarative schema: %v", streamName, err)
	}
	removeSchemaTable(migrationRealm, versionTable)
	normalizeSQLiteAutoIndexes(migrationRealm)
	normalizeSQLiteAutoIndexes(declarativeRealm)
	changes, err := migrationDriver.RealmDiff(
		migrationRealm,
		declarativeRealm,
		atlasschema.DiffNormalized(),
	)
	if err != nil {
		t.Fatalf("diff %s migration and declarative schemas: %v", streamName, err)
	}
	if len(changes) != 0 {
		t.Fatalf(
			"%s migrations schema differs structurally from declarative schema\n--- migrations ---\n%s\n--- declarative ---\n%s",
			streamName,
			normalizedSQLiteSchema(t, migrationDB, versionTable),
			normalizedSQLiteSchema(t, declarativeDB, ""),
		)
	}

	gotNonAtlas := normalizedSQLiteObjects(t, migrationDB, "trigger", "view")
	wantNonAtlas := normalizedSQLiteObjects(t, declarativeDB, "trigger", "view")
	if gotNonAtlas != wantNonAtlas {
		t.Fatalf(
			"%s migration triggers/views differ from declarative schema\n--- migrations ---\n%s\n--- declarative ---\n%s",
			streamName,
			gotNonAtlas,
			wantNonAtlas,
		)
	}
}

func removeSchemaTable(realm *atlasschema.Realm, tableName string) {
	for _, schemaObject := range realm.Schemas {
		tables := schemaObject.Tables[:0]
		for _, table := range schemaObject.Tables {
			if table.Name != tableName {
				tables = append(tables, table)
			}
		}
		schemaObject.Tables = tables
	}
}

func normalizeSQLiteAutoIndexes(realm *atlasschema.Realm) {
	for _, schemaObject := range realm.Schemas {
		for _, table := range schemaObject.Tables {
			for _, index := range table.Indexes {
				if !strings.HasPrefix(index.Name, "sqlite_autoindex_") || sqliteIndexOrigin(index) != "u" {
					continue
				}
				parts := make([]string, 0, len(index.Parts)+1)
				parts = append(parts, table.Name)
				for _, part := range index.Parts {
					if part.C != nil {
						parts = append(parts, part.C.Name)
					}
				}
				if len(parts) > 1 {
					index.Name = strings.Join(parts, "_")
				}
			}
		}
	}
}

func sqliteIndexOrigin(index *atlasschema.Index) string {
	for _, attribute := range index.Attrs {
		if origin, ok := attribute.(*atlassqlite.IndexOrigin); ok {
			return origin.O
		}
	}
	return ""
}

func schemaOwnedTables(t *testing.T, schemaFS fs.FS, source string) map[string]bool {
	t.Helper()
	db := openStreamTestDB(t, "owned-tables.db")
	executeDeclarativeSchema(t, db, schemaFS, source)
	rows, err := db.QueryContext(
		testutil.Context(t),
		`SELECT name FROM pragma_table_list WHERE schema = 'main' AND type IN ('table', 'virtual')`,
	)
	if err != nil {
		t.Fatalf("query owned tables: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close owned table rows: %v", err)
		}
	}()
	tables := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan owned table: %v", err)
		}
		if !strings.HasPrefix(name, "sqlite_") {
			tables[name] = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate owned tables: %v", err)
	}
	return tables
}

func executeDeclarativeSchema(t *testing.T, db *sql.DB, schemaFS fs.FS, source string) {
	t.Helper()
	info, err := fs.Stat(schemaFS, source)
	if err != nil {
		t.Fatalf("stat declarative schema source %q: %v", source, err)
	}
	files := []string{source}
	if info.IsDir() {
		files = files[:0]
		entries, err := fs.ReadDir(schemaFS, source)
		if err != nil {
			t.Fatalf("read declarative schema directory %q: %v", source, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || path.Ext(entry.Name()) != ".sql" {
				continue
			}
			files = append(files, path.Join(source, entry.Name()))
		}
	}
	if len(files) == 0 {
		t.Fatalf("declarative schema source %q contains no SQL files", source)
	}
	for _, name := range files {
		contents, err := fs.ReadFile(schemaFS, name)
		if err != nil {
			t.Fatalf("read declarative schema file %q: %v", name, err)
		}
		if _, err := db.ExecContext(testutil.Context(t), string(contents)); err != nil {
			t.Fatalf("execute declarative schema file %q: %v", name, err)
		}
	}
}

func openStreamTestDB(t *testing.T, name string) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if err := db.PingContext(testutil.Context(t)); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("close failed database: %v", closeErr)
		}
		t.Fatalf("ping %s: %v", path, err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
			t.Errorf("close %s: %v", path, err)
		}
	})
	return db
}

func normalizedSQLiteSchema(t *testing.T, db *sql.DB, excludedVersionTable string) string {
	t.Helper()
	rows, err := db.QueryContext(testutil.Context(t), `SELECT type, name, sql FROM sqlite_master
		WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY type, name`)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close sqlite_master rows: %v", err)
		}
	}()
	objects := make([]string, 0)
	for rows.Next() {
		var kind string
		var name string
		var statement string
		if err := rows.Scan(&kind, &name, &statement); err != nil {
			t.Fatalf("scan sqlite_master: %v", err)
		}
		if name == excludedVersionTable {
			continue
		}
		objects = append(objects, fmt.Sprintf("%s|%s|%s", kind, name, normalizeSchemaSQL(statement)))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master: %v", err)
	}
	return strings.Join(objects, "\n")
}

func normalizedSQLiteTableCounts(t *testing.T, db *sql.DB, excludedVersionTable string) string {
	t.Helper()
	rows, err := db.QueryContext(testutil.Context(t), `SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name <> ? ORDER BY name`, excludedVersionTable)
	if err != nil {
		t.Fatalf("query sqlite table names: %v", err)
	}
	tableNames := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan sqlite table name: %v", err)
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite table names: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close sqlite table names: %v", err)
	}
	counts := make([]string, 0, len(tableNames))
	for _, name := range tableNames {
		var count int
		quotedName := `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
		if err := db.QueryRowContext(testutil.Context(t), "SELECT COUNT(*) FROM "+quotedName).Scan(&count); err != nil {
			t.Fatalf("count rows in %s: %v", name, err)
		}
		counts = append(counts, fmt.Sprintf("%s=%d", name, count))
	}
	return strings.Join(counts, ",")
}

func normalizedSQLiteObjects(t *testing.T, db *sql.DB, objectTypes ...string) string {
	t.Helper()
	wanted := make(map[string]bool, len(objectTypes))
	for _, objectType := range objectTypes {
		wanted[objectType] = true
	}
	rows, err := db.QueryContext(testutil.Context(t), `SELECT type, name, sql FROM sqlite_master
		WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite_%' ORDER BY type, name`)
	if err != nil {
		t.Fatalf("query sqlite_master objects: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close sqlite_master object rows: %v", err)
		}
	}()
	objects := make([]string, 0)
	for rows.Next() {
		var kind string
		var name string
		var statement string
		if err := rows.Scan(&kind, &name, &statement); err != nil {
			t.Fatalf("scan sqlite_master object: %v", err)
		}
		if wanted[kind] {
			objects = append(objects, fmt.Sprintf("%s|%s|%s", kind, name, normalizeSchemaSQL(statement)))
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master objects: %v", err)
	}
	return strings.Join(objects, "\n")
}

func sqliteTableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var exists bool
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?)`,
		table,
	).Scan(&exists); err != nil {
		t.Fatalf("query table %s existence: %v", table, err)
	}
	return exists
}

func sqliteColumns(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.QueryContext(testutil.Context(t), `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatalf("query table %s columns: %v", table, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close table %s column rows: %v", table, err)
		}
	}()

	columns := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table %s column: %v", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table %s columns: %v", table, err)
	}
	return columns
}

func normalizeSchemaSQL(statement string) string {
	normalized := strings.Join(strings.Fields(strings.ReplaceAll(statement, "`", "")), " ")
	normalized = strings.ReplaceAll(normalized, "( ", "(")
	return strings.ReplaceAll(normalized, " )", ")")
}
