package globaldb

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/extensionenv"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/vault"
)

func TestExtensionEnvironmentMigration(t *testing.T) {
	t.Run("Should append portable instance metadata and remote header mapping atomically", func(t *testing.T) {
		// Keep this full-history fixture serial: migration helpers share a process-wide lock.
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		legacy, err := sql.Open(sqliteDriverName, path)
		if err != nil {
			t.Fatalf("sql.Open(v64 fixture) error = %v", err)
		}
		legacyClosed := false
		t.Cleanup(func() {
			if legacyClosed {
				return
			}
			if closeErr := legacy.Close(); closeErr != nil {
				t.Errorf("Close(v64 fixture cleanup) error = %v", closeErr)
			}
		})
		if err := applyGlobalMigrationPrefix(
			t,
			legacy,
			globalMigrationPrefixBefore(t, "00065_schema.sql"),
		); err != nil {
			t.Fatalf("Apply(v64 prefix) error = %v", err)
		}
		statements := []string{
			`INSERT INTO workspaces (id, root_dir, name, created_at, updated_at)
			 VALUES ('ws-portable', '/tmp/ws-portable', 'portable',
			 '2026-08-15T12:00:00Z', '2026-08-15T12:00:00Z')`,
			`INSERT INTO extensions (
			 name, version, source, enabled, manifest_path, installed_at,
			 provides_json, permissions_json, checksum, provenance_json
			 ) VALUES (
			 'portable', '1.0.0', 'local', 0, '/tmp/portable/plugin.json',
			 '2026-08-15T12:01:00Z', '[]', '[]', 'checksum-portable', '{}'
			 )`,
			`INSERT INTO extension_dev_links (
			 extension_name, workspace_id, origin_path, bundle_generation, linked_at
			 ) VALUES (
			 'portable', 'ws-portable', '/tmp/portable-dev',
			 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
			 '2026-08-15T12:02:00Z'
			 )`,
			`INSERT INTO extension_env_bindings (
			 extension_name, workspace_id, env_name, secret_ref, kind, created_at, updated_at
			 ) VALUES (
			 'portable', '', 'API_KEY', 'vault:extensions/global/portable/env/API_KEY',
			 'extension_env', '2026-08-15T12:03:00Z', '2026-08-15T12:03:00Z'
			 )`,
			`INSERT INTO extension_env_bindings (
			 extension_name, workspace_id, env_name, secret_ref, kind, created_at, updated_at
			 ) VALUES (
			 'portable', 'ws-portable', 'WS_KEY',
			 'vault:extensions/ws/ws-portable/portable/env/WS_KEY',
			 'extension_env', '2026-08-15T12:04:00Z', '2026-08-15T12:04:00Z'
			 )`,
		}
		for idx, statement := range statements {
			if _, err := legacy.ExecContext(testutil.Context(t), statement); err != nil {
				t.Fatalf("seed v64 statement %d error = %v", idx+1, err)
			}
		}
		if err := legacy.Close(); err != nil {
			t.Fatalf("Close(v64 fixture) error = %v", err)
		}
		legacyClosed = true

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(v64 fixture) error = %v", err)
		}
		closed := false
		t.Cleanup(func() {
			if closed {
				return
			}
			if closeErr := upgraded.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(upgraded v64 fixture cleanup) error = %v", closeErr)
			}
		})
		assertTableHasColumns(t, upgraded.db, "extensions", []string{"format", "ingest_diagnostics_json"})
		assertTableHasColumns(
			t,
			upgraded.db,
			"extension_dev_links",
			[]string{"format", "ingest_diagnostics_json"},
		)
		assertTableHasColumns(t, upgraded.db, "extension_env_bindings", []string{"mcp_server", "header_name"})
		for table, where := range map[string]string{
			"extensions":          "name = 'portable'",
			"extension_dev_links": "extension_name = 'portable' AND workspace_id = 'ws-portable'",
		} {
			var format, diagnostics string
			if err := upgraded.db.QueryRowContext(
				testutil.Context(t),
				`SELECT format, ingest_diagnostics_json FROM `+table+` WHERE `+where,
			).Scan(&format, &diagnostics); err != nil {
				t.Fatalf("query %s portable defaults error = %v", table, err)
			}
			if format != "compozy" || diagnostics != "[]" {
				t.Fatalf("%s portable defaults = %q/%q, want compozy/[]", table, format, diagnostics)
			}
		}
		var server, header string
		if err := upgraded.db.QueryRowContext(
			testutil.Context(t),
			`SELECT mcp_server, header_name FROM extension_env_bindings
			 WHERE extension_name = 'portable' AND workspace_id = '' AND env_name = 'API_KEY'`,
		).Scan(&server, &header); err != nil {
			t.Fatalf("query binding mapping defaults error = %v", err)
		}
		if server != "" || header != "" {
			t.Fatalf("binding mapping defaults = %q/%q, want empty/empty", server, header)
		}
		baseInsert := `INSERT INTO extension_env_bindings (
		 extension_name, workspace_id, env_name, secret_ref, mcp_server, header_name,
		 kind, created_at, updated_at
		 ) VALUES ('portable', '', ?, ?, ?, ?, 'extension_env',
		 '2026-08-15T12:05:00Z', '2026-08-15T12:05:00Z')`
		for _, valid := range []struct {
			envName string
			server  string
			header  string
		}{
			{envName: "PLAIN", server: "", header: ""},
			{envName: "REMOTE", server: "api", header: "Authorization"},
		} {
			if _, err := upgraded.db.ExecContext(
				testutil.Context(t), baseInsert, valid.envName,
				"vault:extensions/global/portable/env/"+valid.envName, valid.server, valid.header,
			); err != nil {
				t.Fatalf("insert valid binding shape %#v error = %v", valid, err)
			}
		}
		for _, invalid := range []struct {
			envName string
			server  string
			header  string
		}{
			{envName: "SERVER_ONLY", server: "api"},
			{envName: "HEADER_ONLY", header: "Authorization"},
		} {
			if _, err := upgraded.db.ExecContext(
				testutil.Context(t), baseInsert, invalid.envName,
				"vault:extensions/global/portable/env/"+invalid.envName, invalid.server, invalid.header,
			); err == nil || !strings.Contains(err.Error(), "CHECK constraint failed") {
				t.Fatalf("insert partial binding shape %#v error = %v, want SQLite CHECK rejection", invalid, err)
			}
		}
		if _, err := upgraded.db.ExecContext(
			testutil.Context(t),
			`DELETE FROM workspaces WHERE id = 'ws-portable'`,
		); err != nil {
			t.Fatalf("delete workspace after binding table rebuild error = %v", err)
		}
		var workspaceBindingCount int
		if err := upgraded.db.QueryRowContext(
			testutil.Context(t),
			`SELECT COUNT(*) FROM extension_env_bindings WHERE workspace_id = 'ws-portable'`,
		).Scan(&workspaceBindingCount); err != nil {
			t.Fatalf("count workspace bindings after delete error = %v", err)
		}
		if workspaceBindingCount != 0 {
			t.Fatalf("workspace binding count after delete = %d, want zero", workspaceBindingCount)
		}
		status, err := store.Status(testutil.Context(t), upgraded.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(upgraded v64 fixture) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())

		if err := upgraded.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(upgraded v64 fixture) error = %v", err)
		}
		closed = true
		reopened, err := OpenGlobalDB(testutil.Context(t), path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen upgraded v64 fixture) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(reopened v64 fixture) error = %v", closeErr)
			}
		})
		assertTableHasColumns(t, reopened.db, "extensions", []string{"format", "ingest_diagnostics_json"})
		assertTableHasColumns(t, reopened.db, "extension_env_bindings", []string{"mcp_server", "header_name"})
	})

	t.Run("Should preserve valid extension environment bindings and enforce their ownership kind", func(t *testing.T) {
		// Keep this full-history fixture serial: migration helpers share a process-wide
		// lock, and parallel suite contention can exhaust its operation context under -race.

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		legacy, err := sql.Open(sqliteDriverName, path)
		if err != nil {
			t.Fatalf("sql.Open(v38 fixture) error = %v", err)
		}
		legacyClosed := false
		t.Cleanup(func() {
			if legacyClosed {
				return
			}
			if closeErr := legacy.Close(); closeErr != nil {
				t.Errorf("Close(v38 fixture cleanup) error = %v", closeErr)
			}
		})
		if err := applyGlobalMigrationPrefix(
			t,
			legacy,
			globalMigrationPrefixBefore(t, "00039_schema.sql"),
		); err != nil {
			t.Fatalf("Apply(v38 prefix) error = %v", err)
		}
		statements := []string{
			`INSERT INTO extensions (
			name, version, source, enabled, manifest_path, installed_at,
			provides_json, permissions_json, checksum, provenance_json
		) VALUES (
			'kit', '1.0.0', 'local', 0, '/tmp/kit/extension.toml',
			'2026-08-02T12:00:00Z', '[]', '[]', 'checksum-kit', '{}'
		)`,
			`INSERT INTO extension_dev_links (
			extension_name, workspace_id, origin_path, bundle_generation, linked_at
		) VALUES (
			'kit', 'ws-1', '/tmp/kit-dev',
			'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
			'2026-08-02T12:01:00Z'
		)`,
		}
		for index, statement := range statements {
			if _, err := legacy.ExecContext(testutil.Context(t), statement); err != nil {
				t.Fatalf("seed v38 statement %d error = %v", index+1, err)
			}
		}
		if err := legacy.Close(); err != nil {
			t.Fatalf("Close(v38 fixture) error = %v", err)
		}
		legacyClosed = true

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(v38 fixture) error = %v", err)
		}
		closed := false
		t.Cleanup(func() {
			if closed {
				return
			}
			if closeErr := upgraded.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(upgraded fixture cleanup) error = %v", closeErr)
			}
		})
		for table, where := range map[string]string{
			"extensions":          "name = 'kit'",
			"extension_dev_links": "extension_name = 'kit' AND workspace_id = 'ws-1'",
		} {
			var digest string
			var confirmedBy, confirmedAt sql.NullString
			query := `SELECT network_requirement_digest, network_confirmed_by, network_confirmed_at FROM ` +
				table + ` WHERE ` + where
			if err := upgraded.db.QueryRowContext(testutil.Context(t), query).Scan(
				&digest,
				&confirmedBy,
				&confirmedAt,
			); err != nil {
				t.Fatalf("query %s confirmation defaults error = %v", table, err)
			}
			if digest != "" || confirmedBy.Valid || confirmedAt.Valid {
				t.Fatalf(
					"%s confirmation defaults = digest:%q by:%#v at:%#v, want empty/null/null",
					table,
					digest,
					confirmedBy,
					confirmedAt,
				)
			}
		}
		insertBinding := `INSERT INTO extension_env_bindings (
		extension_name, workspace_id, env_name, secret_ref, kind, created_at, updated_at
	) VALUES ('kit', 'ws-1', 'API_KEY', 'vault:extensions/ws/ws-1/kit/env/API_KEY',
	'extension_env', '2026-08-02T12:02:00Z', '2026-08-02T12:02:00Z')`
		if _, err := upgraded.db.ExecContext(testutil.Context(t), insertBinding); err != nil {
			t.Fatalf("insert migrated binding error = %v", err)
		}
		if _, err := upgraded.db.ExecContext(testutil.Context(t), insertBinding); !isSQLitePrimaryKeyConstraint(err) {
			t.Fatalf("duplicate migrated binding error = %v, want composite primary-key constraint", err)
		}
		invalidKindBinding := `INSERT INTO extension_env_bindings (
		extension_name, workspace_id, env_name, secret_ref, kind, created_at, updated_at
	) VALUES ('kit', 'ws-1', 'OTHER_KEY', 'vault:extensions/ws/ws-1/kit/env/OTHER_KEY',
	'other', '2026-08-02T12:03:00Z', '2026-08-02T12:03:00Z')`
		if _, err := upgraded.db.ExecContext(testutil.Context(t), invalidKindBinding); err == nil {
			t.Fatal("invalid migrated binding kind error = nil, want CHECK rejection")
		}
		status, err := store.Status(testutil.Context(t), upgraded.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(upgraded fixture) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())

		if err := upgraded.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(upgraded fixture) error = %v", err)
		}
		closed = true
		reopened, err := OpenGlobalDB(testutil.Context(t), path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen upgraded fixture) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close(reopened fixture) error = %v", closeErr)
			}
		})
		bindings, err := reopened.ListEnvBindings(testutil.Context(t), "kit", "ws-1")
		if err != nil {
			t.Fatalf("ListEnvBindings(reopened) error = %v", err)
		}
		if len(bindings) != 1 || bindings[0].EnvName != "API_KEY" {
			t.Fatalf("reopened bindings = %#v, want preserved API_KEY", bindings)
		}
	})
}

func TestExtensionEnvRepoRoundTripAndInstanceIsolation(t *testing.T) {
	t.Parallel()

	t.Run("Should upsert one instance while preserving timestamps and other scopes", func(t *testing.T) {
		t.Parallel()

		db := openFreshTestGlobalDB(t)
		createdAt := time.Date(2026, 8, 2, 10, 0, 0, 123, time.UTC)
		updatedAt := createdAt.Add(time.Minute)
		global := extensionenv.Binding{
			ExtensionName: "kit", EnvName: "API_KEY",
			SecretRef: vault.ExtensionSecretRef("kit", "", "API_KEY"),
			MCPServer: "deployment-api", HeaderName: "X-Deployment-Key",
			Kind: extensionenv.BindingKind, CreatedAt: createdAt, UpdatedAt: createdAt,
		}
		workspace := extensionenv.Binding{
			ExtensionName: "kit", WorkspaceID: "ws-1", EnvName: "API_KEY",
			SecretRef: vault.ExtensionSecretRef("kit", "ws-1", "API_KEY"),
			Kind:      extensionenv.BindingKind, CreatedAt: createdAt, UpdatedAt: createdAt,
		}
		for _, binding := range []extensionenv.Binding{global, workspace} {
			if err := db.PutEnvBinding(testutil.Context(t), binding); err != nil {
				t.Fatalf("PutEnvBinding(%#v) error = %v", binding, err)
			}
		}

		global.SecretRef = "vault:extensions/global/kit/env/API_KEY_V2"
		global.UpdatedAt = updatedAt
		if err := db.PutEnvBinding(testutil.Context(t), global); err != nil {
			t.Fatalf("PutEnvBinding(upsert) error = %v", err)
		}
		gotGlobal, err := db.ListEnvBindings(testutil.Context(t), "kit", "")
		if err != nil {
			t.Fatalf("ListEnvBindings(global) error = %v", err)
		}
		if len(gotGlobal) != 1 || gotGlobal[0].SecretRef != global.SecretRef ||
			gotGlobal[0].MCPServer != global.MCPServer || gotGlobal[0].HeaderName != global.HeaderName ||
			!gotGlobal[0].CreatedAt.Equal(createdAt) || !gotGlobal[0].UpdatedAt.Equal(updatedAt) ||
			gotGlobal[0].Kind != extensionenv.BindingKind {
			t.Fatalf("global bindings = %#v, want upsert preserving created_at", gotGlobal)
		}
		gotWorkspace, err := db.ListEnvBindings(testutil.Context(t), "kit", "ws-1")
		if err != nil {
			t.Fatalf("ListEnvBindings(workspace) error = %v", err)
		}
		if !reflect.DeepEqual(gotWorkspace, []extensionenv.Binding{workspace}) {
			t.Fatalf("workspace bindings = %#v, want %#v", gotWorkspace, []extensionenv.Binding{workspace})
		}

		if err := db.DeleteEnvBinding(testutil.Context(t), "kit", "", "API_KEY"); err != nil {
			t.Fatalf("DeleteEnvBinding(global) error = %v", err)
		}
		gotGlobal, err = db.ListEnvBindings(testutil.Context(t), "kit", "")
		if err != nil {
			t.Fatalf("ListEnvBindings(global after delete) error = %v", err)
		}
		if len(gotGlobal) != 0 {
			t.Fatalf("global bindings after delete = %#v, want empty", gotGlobal)
		}
		gotWorkspace, err = db.ListEnvBindings(testutil.Context(t), "kit", "ws-1")
		if err != nil || len(gotWorkspace) != 1 {
			t.Fatalf("workspace bindings after global delete = %#v, %v", gotWorkspace, err)
		}
	})

	t.Run("Should list environment names in sorted order", func(t *testing.T) {
		t.Parallel()

		db := openFreshTestGlobalDB(t)
		for _, envName := range []string{"Z_TOKEN", "A_TOKEN", "M_TOKEN"} {
			if err := db.PutEnvBinding(testutil.Context(t), extensionenv.Binding{
				ExtensionName: "kit", EnvName: envName,
				SecretRef: vault.ExtensionSecretRef("kit", "", envName), Kind: extensionenv.BindingKind,
			}); err != nil {
				t.Fatalf("PutEnvBinding(%s) error = %v", envName, err)
			}
		}
		bindings, err := db.ListEnvBindings(testutil.Context(t), "kit", "")
		if err != nil {
			t.Fatalf("ListEnvBindings() error = %v", err)
		}
		got := make([]string, 0, len(bindings))
		for _, binding := range bindings {
			got = append(got, binding.EnvName)
		}
		if !reflect.DeepEqual(got, []string{"A_TOKEN", "M_TOKEN", "Z_TOKEN"}) {
			t.Fatalf("env names = %#v, want sorted", got)
		}
	})

	t.Run("Should reject refs owned by another extension or workspace", func(t *testing.T) {
		t.Parallel()

		db := openFreshTestGlobalDB(t)
		for _, testCase := range []struct {
			name string
			ref  string
		}{
			{name: "another extension", ref: vault.ExtensionSecretRef("other", "ws-1", "API_KEY")},
			{name: "another workspace", ref: vault.ExtensionSecretRef("kit", "ws-2", "API_KEY")},
			{name: "global scope", ref: vault.ExtensionSecretRef("kit", "", "API_KEY")},
		} {
			t.Run("Should reject "+testCase.name, func(t *testing.T) {
				t.Parallel()
				err := db.PutEnvBinding(testutil.Context(t), extensionenv.Binding{
					ExtensionName: "kit", WorkspaceID: "ws-1", EnvName: "API_KEY",
					SecretRef: testCase.ref, Kind: extensionenv.BindingKind,
				})
				if err == nil || !strings.Contains(err.Error(), "outside its instance namespace") {
					t.Fatalf("PutEnvBinding(%s) error = %v, want owner isolation failure", testCase.name, err)
				}
			})
		}
	})

	t.Run("Should reject binding ownership kinds outside the extension environment contract", func(t *testing.T) {
		t.Parallel()

		db := openFreshTestGlobalDB(t)
		err := db.PutEnvBinding(testutil.Context(t), extensionenv.Binding{
			ExtensionName: "kit",
			EnvName:       "API_KEY",
			SecretRef:     vault.ExtensionSecretRef("kit", "", "API_KEY"),
			Kind:          "other",
		})
		if err == nil || !strings.Contains(err.Error(), `kind must be "extension_env"`) {
			t.Fatalf("PutEnvBinding(invalid kind) error = %v, want ownership-kind rejection", err)
		}
	})
}
