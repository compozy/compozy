package globaldb

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGlobalDBCallsMigration(t *testing.T) {
	t.Parallel()

	t.Run("Should backfill contracted task budgets and preserve them across reopen", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00099_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open prefix before 00099 error = %v", err)
		}
		ctx := globalMigrationTestContext(t)
		now := store.FormatTimestamp(time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC))
		digest := "sha256:" + strings.Repeat("0", 64)
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO contract_schemas (digest, schema, created_at)
			VALUES (?, '{"type":"object"}', ?)`, digest, now); err != nil {
			t.Fatalf("seed contract before 00099 error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO tasks (
			id, profile_id, scope, title, status, created_by_kind, created_by_ref,
			origin_kind, origin_ref, created_at, updated_at, expect_digest
		) VALUES (
			'task-calls-migration', ?, 'global', 'Contracted migration task', 'ready',
			'daemon', 'migration-test', 'daemon', 'migration-test', ?, ?, ?
		)`, store.DefaultProfileID, now, now, digest); err != nil {
			t.Fatalf("seed contracted task before 00099 error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("close prefix before 00099 error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("upgrade calls migration fixture error = %v", err)
		}
		upgradedClosed := false
		t.Cleanup(func() {
			if upgradedClosed {
				return
			}
			if closeErr := upgraded.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close upgraded calls migration fixture error = %v", closeErr)
			}
		})
		assertContractedTaskBudget(t, upgraded, digest)
		status, err := store.Status(ctx, upgraded.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(after calls migration) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("close upgraded calls migration fixture before reopen error = %v", err)
		}
		upgradedClosed = true

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("reopen calls migration fixture error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close reopened calls migration fixture error = %v", closeErr)
			}
		})
		assertContractedTaskBudget(t, reopened, digest)
	})
}

func assertContractedTaskBudget(t *testing.T, database *GlobalDB, wantDigest string) {
	t.Helper()

	var digest string
	var budget int64
	var overflow string
	if err := database.db.QueryRowContext(
		t.Context(),
		`SELECT expect_digest, result_budget_bytes, result_overflow FROM tasks WHERE id = 'task-calls-migration'`,
	).Scan(&digest, &budget, &overflow); err != nil {
		t.Fatalf("read migrated contracted task error = %v", err)
	}
	if digest != wantDigest || budget != 256<<10 || overflow != "store" {
		t.Fatalf("migrated contracted task = digest %q budget %d overflow %q", digest, budget, overflow)
	}
	var integrity string
	if err := database.db.QueryRowContext(t.Context(), "PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatalf("PRAGMA integrity_check error = %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("PRAGMA integrity_check = %q, want ok", integrity)
	}
}
