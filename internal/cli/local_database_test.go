package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
)

func TestOpenLocalGlobalDatabaseMigrationErrors(t *testing.T) {
	t.Run("Should preserve structured legacy errors for every direct-open surface", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
		seedLegacyCLIDatabase(t, path)
		canonicalPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatalf("EvalSymlinks(legacy database) error = %v", err)
		}

		for _, surface := range []string{"provider auth", "extension"} {
			t.Run("Should return "+surface+" refusal", func(t *testing.T) {
				t.Parallel()

				_, err := openLocalGlobalDatabase(ctx, path, surface)
				if !errors.Is(err, store.ErrLegacyDatabase) {
					t.Fatalf("openLocalGlobalDatabase(%s) error = %v, want ErrLegacyDatabase", surface, err)
				}
				var openErr *localDatabaseOpenError
				if !errors.As(err, &openErr) {
					t.Fatalf("openLocalGlobalDatabase(%s) error type = %T, want *localDatabaseOpenError", surface, err)
				}
				if openErr.Surface != surface || openErr.Path != canonicalPath {
					t.Fatalf("structured error = %#v, want surface %q path %q", openErr, surface, canonicalPath)
				}
				if !strings.Contains(err.Error(), canonicalPath) ||
					!strings.Contains(err.Error(), "complete AGH_HOME or workspace .agh directory") {
					t.Fatalf(
						"openLocalGlobalDatabase(%s) error = %q, want path and whole-family remediation",
						surface,
						err,
					)
				}
			})
		}
	})

	t.Run("Should preserve structured ahead-schema errors and remediation", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
		db, err := globaldb.OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(seed ahead) error = %v", err)
		}
		if _, err := db.DB().ExecContext(
			ctx,
			`INSERT INTO goose_db_version_global (version_id, is_applied) VALUES (99, 1)`,
		); err != nil {
			t.Fatalf("insert ahead migration version error = %v", err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatalf("GlobalDB.Close(seed ahead) error = %v", err)
		}

		_, err = openLocalGlobalDatabase(ctx, path, "provider auth")
		if !errors.Is(err, store.ErrSchemaAhead) {
			t.Fatalf("openLocalGlobalDatabase(ahead) error = %v, want ErrSchemaAhead", err)
		}
		var openErr *localDatabaseOpenError
		if !errors.As(err, &openErr) {
			t.Fatalf("openLocalGlobalDatabase(ahead) error type = %T, want *localDatabaseOpenError", err)
		}
		if !strings.Contains(err.Error(), "install a newer AGH binary") ||
			!strings.Contains(err.Error(), "complete AGH_HOME or workspace .agh directory") {
			t.Fatalf("openLocalGlobalDatabase(ahead) error = %q, want deterministic remediation", err)
		}
	})
}

// This test mutates AGH_HOME and the process-wide slog default, so its cases must stay serial.
func TestExecuteContextLocalDatabaseMigrationErrors(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		surface       string
		prepareConfig bool
	}{
		{
			name:          "provider auth",
			args:          []string{"provider", "auth", "status", "claude", "--no-probe", "-o", "json"},
			surface:       "provider auth",
			prepareConfig: true,
		},
		{name: "extension", args: []string{"extension", "list", "-o", "json"}, surface: "extension"},
	}
	states := []struct {
		name string
		code string
		seed func(t *testing.T, path string)
	}{
		{
			name: "legacy database",
			code: diagnosticcontract.CodeLegacyDatabase,
			seed: seedLegacyCLIDatabase,
		},
		{
			name: "ahead database",
			code: diagnosticcontract.CodeSchemaAhead,
			seed: seedAheadCLIDatabase,
		},
	}

	for _, state := range states {
		for _, tt := range tests {
			t.Run("Should emit one JSON envelope for "+tt.name+" against a "+state.name, func(t *testing.T) {
				home := t.TempDir()
				t.Setenv("AGH_HOME", home)
				if tt.prepareConfig {
					writeLocalDatabaseProviderConfig(t, home)
				}
				databasePath := filepath.Join(home, store.GlobalDatabaseName)
				state.seed(t, databasePath)
				canonicalPath, err := filepath.EvalSymlinks(databasePath)
				if err != nil {
					t.Fatalf("EvalSymlinks(database) error = %v", err)
				}

				var stdout bytes.Buffer
				var stderr bytes.Buffer
				previousLogger := slog.Default()
				slog.SetDefault(slog.New(slog.NewTextHandler(&stderr, nil)))
				t.Cleanup(func() {
					slog.SetDefault(previousLogger)
				})

				if exitCode := ExecuteContext(testutil.Context(t), tt.args, &stdout, &stderr); exitCode == 0 {
					t.Fatalf("ExecuteContext(%s) exit code = 0, want non-zero", tt.name)
				}
				if stdout.Len() != 0 {
					t.Fatalf("ExecuteContext(%s) stdout = %q, want empty", tt.name, stdout.String())
				}
				var payload contract.ErrorPayload
				if err := json.Unmarshal(bytes.TrimSpace(stderr.Bytes()), &payload); err != nil {
					t.Fatalf("json.Unmarshal(%s stderr) error = %v; stderr = %q", tt.name, err, stderr.String())
				}
				if payload.Diagnostic == nil || payload.Diagnostic.Code != state.code {
					t.Fatalf(
						"ExecuteContext(%s) diagnostic = %#v, want code %q",
						tt.name,
						payload.Diagnostic,
						state.code,
					)
				}
				if got := payload.Diagnostic.Evidence["surface"]; got != tt.surface {
					t.Fatalf("ExecuteContext(%s) surface = %#v, want %q", tt.name, got, tt.surface)
				}
				if got := payload.Diagnostic.Evidence["offending_path"]; got != canonicalPath {
					t.Fatalf("ExecuteContext(%s) path = %#v, want %q", tt.name, got, canonicalPath)
				}
				if !strings.Contains(payload.Diagnostic.Message, "complete AGH_HOME or workspace .agh directory") {
					t.Fatalf(
						"ExecuteContext(%s) message = %q, want whole-family remediation",
						tt.name,
						payload.Diagnostic.Message,
					)
				}
			})
		}
	}
}

func seedLegacyCLIDatabase(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open(legacy) error = %v", err)
	}
	if _, err := db.ExecContext(testutil.Context(t), `CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		checksum TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy schema_migrations error = %v", err)
	}
	if _, err := db.ExecContext(testutil.Context(t), `WITH RECURSIVE versions(version) AS (
		SELECT 1 UNION ALL SELECT version + 1 FROM versions WHERE version < 61
	) INSERT INTO schema_migrations (version, name, checksum, applied_at)
	SELECT version, printf('legacy_%02d', version), printf('checksum-%02d', version), '2026-07-10T00:00:00Z'
	FROM versions`); err != nil {
		t.Fatalf("seed legacy schema_migrations error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close(legacy) error = %v", err)
	}
}

func seedAheadCLIDatabase(t *testing.T, path string) {
	t.Helper()
	ctx := testutil.Context(t)
	db, err := globaldb.OpenGlobalDB(ctx, path)
	if err != nil {
		t.Fatalf("OpenGlobalDB(ahead seed) error = %v", err)
	}
	if _, err := db.DB().ExecContext(
		ctx,
		`INSERT INTO goose_db_version_global (version_id, is_applied) VALUES (99, 1)`,
	); err != nil {
		t.Fatalf("insert ahead migration version error = %v", err)
	}
	if err := db.Close(ctx); err != nil {
		t.Fatalf("GlobalDB.Close(ahead seed) error = %v", err)
	}
}

func writeLocalDatabaseProviderConfig(t *testing.T, home string) {
	t.Helper()
	contents := []byte(`[providers.claude]
auth_mode = "bound_secret"

[[providers.claude.credential_slots]]
name = "api_key"
target_env = "ANTHROPIC_KEY"
secret_ref = "vault:providers/claude/api-key"
kind = "api_key"
required = true
`)
	if err := os.WriteFile(filepath.Join(home, "config.toml"), contents, 0o600); err != nil {
		t.Fatalf("WriteFile(provider config) error = %v", err)
	}
}
