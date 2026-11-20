package globaldb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb"
	"github.com/compozy/compozy/internal/testutil"
	"modernc.org/sqlite"
)

// TestGlobalDBSessionLineageMigration owns IT-017: migration 00122 adds the lineage
// kind columns and the derivation receipt table, and backfills pre-feature rows with
// the same rule as store.UpgradeSessionLineageKind. Ahead-version refusal stays owned
// by internal/store/migrate_test.go:TestApplyMigrationStream.
func TestGlobalDBSessionLineageMigration(t *testing.T) {
	t.Parallel()

	t.Run("Should backfill lineage kinds and preserve them across reopen", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00122_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open prefix before 00122 error = %v", err)
		}
		ctx := globalMigrationTestContext(t)
		now := store.FormatTimestamp(time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC))
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO workspaces (
			id, root_dir, add_dirs, name, created_at, updated_at
		) VALUES ('ws-lineage-migration', ?, '[]', 'lineage-migration', ?, ?)`,
			t.TempDir(), now, now); err != nil {
			t.Fatalf("seed workspace before 00122 error = %v", err)
		}
		seeds := []struct {
			id, sessionType, parent, root, role string
			depth                               int
		}{
			{id: "sess-root", sessionType: "user", root: "sess-root"},
			{id: "sess-provenance", sessionType: "user", parent: "sess-root", root: "sess-root", depth: 1},
			{id: "sess-spawned", sessionType: "spawned", parent: "sess-root", root: "sess-root", depth: 1},
			{id: "sess-coordinator", sessionType: "coordinator", root: "sess-coordinator", role: "coordinator"},
		}
		for _, seed := range seeds {
			if _, err := prefixDB.ExecContext(ctx, `INSERT INTO sessions (
				id, profile_id, agent_name, workspace_id, session_type, state,
				parent_session_id, root_session_id, spawn_depth, spawn_role, created_at, updated_at
			) VALUES (?, ?, 'coder', 'ws-lineage-migration', ?, 'stopped',
				NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?, ?)`,
				seed.id, store.DefaultProfileID, seed.sessionType,
				seed.parent, seed.root, seed.depth, seed.role, now, now); err != nil {
				t.Fatalf("seed session %q before 00122 error = %v", seed.id, err)
			}
		}
		columns, err := tableColumns(ctx, prefixDB, "sessions")
		if err != nil {
			t.Fatalf("tableColumns(sessions before 00122) error = %v", err)
		}
		if _, exists := columns["lineage_kind"]; exists {
			t.Fatal("sessions.lineage_kind exists before 00122")
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("close prefix before 00122 error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("upgrade through 00122 error = %v", err)
		}
		upgradedClosed := false
		t.Cleanup(func() {
			if upgradedClosed {
				return
			}
			if closeErr := upgraded.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close upgraded database error = %v", closeErr)
			}
		})
		assertTableHasColumns(t, upgraded.db, "session_derivations", []string{
			"workspace_id", "idempotency_key", "profile_id", "request_fingerprint", "source_session_id",
			"child_session_id", "kind", "outcome_json", "created_at", "child_deleted_at",
		})
		assertIndexesPresent(
			t,
			upgraded.db,
			"session_derivations",
			"idx_session_derivations_source",
			"idx_session_derivations_child",
		)
		want := map[string]store.LineageKind{
			"sess-root":        store.LineageKindRoot,
			"sess-provenance":  store.LineageKindProvenance,
			"sess-spawned":     store.LineageKindSpawn,
			"sess-coordinator": store.LineageKindSpawn,
		}
		assertMigratedLineageKinds(t, upgraded, want)
		status, err := store.Status(ctx, upgraded.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(after migration) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("close upgraded database before reopen error = %v", err)
		}
		upgradedClosed = true

		reopened, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("reopen migrated database error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close reopened database error = %v", closeErr)
			}
		})
		assertMigratedLineageKinds(t, reopened, want)
		provenance, err := reopened.ListSessions(ctx, SessionListQuery{
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID: "ws-lineage-migration",
			LineageKind: store.LineageKindProvenance,
		})
		if err != nil {
			t.Fatalf("ListSessions(lineage_kind=provenance) error = %v", err)
		}
		if len(provenance) != 1 || provenance[0].ID != "sess-provenance" {
			t.Fatalf("ListSessions(lineage_kind=provenance) = %#v, want sess-provenance only", provenance)
		}
	})
}

func assertMigratedLineageKinds(t *testing.T, db *GlobalDB, want map[string]store.LineageKind) {
	t.Helper()

	sessions, err := db.ListSessions(t.Context(), SessionListQuery{
		ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
		WorkspaceID: "ws-lineage-migration",
	})
	if err != nil {
		t.Fatalf("ListSessions(after migration) error = %v", err)
	}
	if len(sessions) != len(want) {
		t.Fatalf("ListSessions(after migration) count = %d, want %d", len(sessions), len(want))
	}
	for i := range sessions {
		info := &sessions[i]
		if info.Lineage == nil || info.Lineage.Kind != want[info.ID] {
			t.Fatalf("session %q lineage = %#v, want kind %q", info.ID, info.Lineage, want[info.ID])
		}
	}
}

const sideMigrationCrashPathEnv = "COMPOZY_TEST_SIDE_MIGRATION_CRASH_PATH"
const sideMigrationCrashMarker = "crashing before renaming new_sessions"

func TestSideLineageMigration(t *testing.T) {
	if path := os.Getenv(sideMigrationCrashPathEnv); path != "" {
		crashSideLineageUpgrade(t, path)
		return
	}
	t.Parallel()
	t.Run("Should preserve sessions and receipts across a crash and admit only the new side kind", func(t *testing.T) {
		t.Parallel()
		// IT-005
		ctx := globalMigrationTestContext(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefix := globalMigrationPrefixBefore(t, "00210_schema.sql")
		db, err := store.OpenSQLiteDatabase(ctx, path, func(ctx context.Context, db *sql.DB) error {
			return store.Apply(ctx, db, prefix)
		})
		if err != nil {
			t.Fatal(err)
		}
		at := store.FormatTimestamp(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
		if _, err := db.ExecContext(
			ctx,
			"INSERT INTO workspaces(id,root_dir,add_dirs,name,created_at,updated_at) VALUES('ws-side-upgrade',?,'[]','Side upgrade',?,?)",
			t.TempDir(),
			at,
			at,
		); err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"fork", "continue", "recovery"} {
			if _, err := db.ExecContext(
				ctx,
				`INSERT INTO sessions(id,profile_id,agent_name,workspace_id,session_type,state,lineage_kind,parent_session_id,root_session_id,spawn_depth,origin_agent_name,name,created_at,updated_at) VALUES(?,?,'coder','ws-side-upgrade','user','stopped',?,'source','source',1,'coder',?, ?,?)`,
				kind,
				store.DefaultProfileID,
				kind,
				"Preserve "+kind,
				at,
				at,
			); err != nil {
				t.Fatal(err)
			}
			if kind != "recovery" {
				if _, err := db.ExecContext(
					ctx,
					`INSERT INTO session_derivations(workspace_id,idempotency_key,profile_id,request_fingerprint,source_session_id,child_session_id,kind,outcome_json,created_at) VALUES('ws-side-upgrade',?,?,?,'source',?,?,?,?)`,
					kind,
					store.DefaultProfileID,
					"fingerprint-"+kind,
					kind,
					kind,
					`{"seed":"replay","first_prompt":"staged","through_turn_id":"turn-1"}`,
					at,
				); err != nil {
					t.Fatal(err)
				}
			}
		}
		// The prefix already records telemetry relocation; seed its required destination
		// exactly as the other post-relocation migration fixtures do.
		telemetry, err := telemetrydb.Open(ctx, telemetrydb.Path(path))
		if err != nil {
			t.Fatal(err)
		}
		if err := bindTelemetryDestination(ctx, db, telemetry.DB()); err != nil {
			t.Fatal(err)
		}
		if err := telemetry.Close(ctx); err != nil {
			t.Fatal(err)
		}
		beforeSessions := sideMigrationRows(ctx, t, db, "sessions")
		beforeReceipts := sideMigrationRows(ctx, t, db, "session_derivations")
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSideLineageMigration$")
		cmd.Env = append(cmd.Environ(), sideMigrationCrashPathEnv+"="+path)
		output, err := cmd.CombinedOutput()
		if _, exited := errors.AsType[*exec.ExitError](err); !exited || ctx.Err() != nil ||
			!bytes.Contains(output, []byte(sideMigrationCrashMarker)) || bytes.Contains(output, []byte("FAIL")) {
			t.Fatalf("expected migration subprocess to be killed before rename, got %v\n%s", err, output)
		}
		checkInterruptedSideLineageUpgrade(ctx, t, path, beforeSessions, beforeReceipts)

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := upgraded.Close(context.WithoutCancel(t.Context())); err != nil {
				t.Error(err)
			}
		})
		if !bytes.Equal(beforeSessions, sideMigrationRows(ctx, t, upgraded.db, "sessions")) ||
			!bytes.Equal(beforeReceipts, sideMigrationRows(ctx, t, upgraded.db, "session_derivations")) {
			t.Fatal("migration changed persisted session or receipt bytes")
		}
		if err := store.RequireCurrent(ctx, upgraded.db, MigrationStream()); err != nil {
			t.Fatalf("migration did not reach the current version: %v", err)
		}
		insertSession := `INSERT INTO sessions(id,profile_id,agent_name,workspace_id,session_type,state,lineage_kind,parent_session_id,root_session_id,spawn_depth,origin_agent_name,created_at,updated_at) VALUES(?,?,'coder','ws-side-upgrade','user','stopped',?,'source','source',1,'coder',?,?)`
		if _, err := upgraded.db.ExecContext(
			ctx,
			insertSession,
			"side",
			store.DefaultProfileID,
			"side",
			at,
			at,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := upgraded.db.ExecContext(
			ctx,
			insertSession,
			"unknown",
			store.DefaultProfileID,
			"unknown",
			at,
			at,
		); err == nil {
			t.Fatal("unknown lineage kind accepted")
		}
		insertReceipt := `INSERT INTO session_derivations(workspace_id,idempotency_key,profile_id,request_fingerprint,source_session_id,child_session_id,kind,outcome_json,created_at) VALUES('ws-side-upgrade',?,?,'side-fingerprint','source','side',?,'{}',?)`
		if _, err := upgraded.db.ExecContext(
			ctx,
			insertReceipt,
			"side",
			store.DefaultProfileID,
			"side",
			at,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := upgraded.db.ExecContext(
			ctx,
			insertReceipt,
			"unknown",
			store.DefaultProfileID,
			"unknown",
			at,
		); err == nil {
			t.Fatal("unknown receipt kind accepted")
		}
	})
}

func crashSideLineageUpgrade(t *testing.T, path string) {
	t.Helper()
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := process.Release(); err != nil {
			t.Error(err)
		}
	}()
	sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, _ string) error {
		registerer, ok := conn.(sqlite.AuthorizerRegisterer)
		if !ok {
			return errors.New("SQLite connection does not support the crash authorizer")
		}
		return registerer.RegisterAuthorizer(
			func(action sqlite.AuthorizerActionCode, _, table, _, _ string) sqlite.AuthorizerReturnCode {
				if action != sqlite.AuthAlterTable || table != "new_sessions" {
					return sqlite.AuthorizerOK
				}
				// Authorization runs before RENAME executes and after the preceding DROP.
				if _, err := fmt.Fprintln(os.Stdout, sideMigrationCrashMarker); err != nil {
					return sqlite.AuthorizerDeny
				}
				if err := process.Kill(); err != nil {
					return sqlite.AuthorizerDeny
				}
				return sqlite.AuthorizerDeny
			},
		)
	})
	upgraded, err := openGlobalMigrationUpgrade(t, path)
	if err != nil {
		t.Fatalf("migration subprocess exited without being killed: %v", err)
	}
	if err := upgraded.Close(t.Context()); err != nil {
		t.Error(err)
	}
	t.Fatal("migration subprocess finished without reaching the crash hook")
}

func checkInterruptedSideLineageUpgrade(ctx context.Context, t *testing.T, path string, sessions, receipts []byte) {
	t.Helper()
	db, err := store.OpenSQLiteDatabase(ctx, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	status, err := store.Status(ctx, db, MigrationStream())
	if err != nil || status.Version != 209 || status.AppliedCount != 209 {
		t.Fatalf("interrupted migration status = %+v, %v, want version 209 with 209 applied migrations", status, err)
	}
	if !bytes.Equal(sessions, sideMigrationRows(ctx, t, db, "sessions")) ||
		!bytes.Equal(receipts, sideMigrationRows(ctx, t, db, "session_derivations")) {
		t.Fatal("interrupted migration changed persisted session or receipt bytes")
	}
}

func sideMigrationRows(ctx context.Context, t *testing.T, db *sql.DB, table string) []byte {
	t.Helper()
	rows, err := db.QueryContext(ctx, "SELECT * FROM "+table+" ORDER BY 1,2")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	}()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	all := [][]any{}
	for rows.Next() {
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range dest {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatal(err)
		}
		all = append(all, values)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
