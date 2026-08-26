package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// Invariant: the global stream persists one ownership record per owner/target,
// enforces the user/workspace shape, and preserves records across reopen.
// Owning layer: internal/store/globaldb. Canonical suite: TestGlobalDBSkillExposureRepository.
func TestGlobalDBSkillExposureRepository(t *testing.T) {
	t.Parallel()
	t.Run("Should persist owner-scoped records across reopen and workspace cleanup", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
		database, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}

		userRecord, err := database.CreateSkillExposure(ctx, store.SkillExposureRecord{
			SkillName: "review", CanonicalDir: "/canonical/review", TargetSlug: "agents",
			LinkPath: "/provider/agents/review", LinkTarget: "../../canonical/review",
			OwnerScope: store.SkillExposureOwnerUser,
		})
		if err != nil {
			t.Fatalf("CreateSkillExposure(user) error = %v", err)
		}
		workspaceRecord, err := database.CreateSkillExposure(ctx, store.SkillExposureRecord{
			SkillName: "review", CanonicalDir: "/workspace/review", TargetSlug: "agents",
			LinkPath: "/workspace/.agents/review", LinkTarget: "../../review",
			OwnerScope: store.SkillExposureOwnerWorkspace, WorkspaceID: "workspace-a",
		})
		if err != nil {
			t.Fatalf("CreateSkillExposure(workspace) error = %v", err)
		}
		if userRecord.ID == workspaceRecord.ID || userRecord.ID == 0 || workspaceRecord.ID == 0 {
			t.Fatalf(
				"record IDs are not distinct positive values: user=%d workspace=%d",
				userRecord.ID,
				workspaceRecord.ID,
			)
		}

		if _, err := database.CreateSkillExposure(ctx, store.SkillExposureRecord{
			SkillName: "review", CanonicalDir: "/other/review", TargetSlug: "agents",
			LinkPath: "/provider/agents/review-duplicate-owner", LinkTarget: "../../other/review",
			OwnerScope: store.SkillExposureOwnerUser,
		}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			t.Fatalf("duplicate user owner/target error = %v, want UNIQUE constraint", err)
		}
		assertSkillExposureCheckConstraint(t, database, store.SkillExposureRecord{
			SkillName: "invalid-user", CanonicalDir: "/canonical/invalid-user", TargetSlug: "agents",
			LinkPath: "/provider/agents/invalid-user", LinkTarget: "../../canonical/invalid-user",
			OwnerScope: store.SkillExposureOwnerUser, WorkspaceID: "workspace-a",
		})
		assertSkillExposureCheckConstraint(t, database, store.SkillExposureRecord{
			SkillName: "invalid-workspace", CanonicalDir: "/canonical/invalid-workspace", TargetSlug: "agents",
			LinkPath: "/provider/agents/invalid-workspace", LinkTarget: "../../canonical/invalid-workspace",
			OwnerScope: store.SkillExposureOwnerWorkspace,
		})

		if err := database.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}
		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(context.Background()); err != nil {
				t.Errorf("Close(reopened) error = %v", err)
			}
		})

		got, err := reopened.GetSkillExposureByOwnerTarget(
			ctx, "review", store.SkillExposureOwnerUser, "", "agents",
		)
		if err != nil {
			t.Fatalf("GetSkillExposureByOwnerTarget(reopen) error = %v", err)
		}
		if got.ID != userRecord.ID || got.LinkPath != userRecord.LinkPath || got.WorkspaceID != "" {
			t.Fatalf("reopened record = %#v, want %#v", got, userRecord)
		}
		byCanonical, err := reopened.ListSkillExposuresByCanonicalDir(ctx, "/workspace/review")
		if err != nil {
			t.Fatalf("ListSkillExposuresByCanonicalDir() error = %v", err)
		}
		if len(byCanonical) != 1 || byCanonical[0].ID != workspaceRecord.ID {
			t.Fatalf("canonical records = %#v", byCanonical)
		}
		if err := reopened.InsertWorkspace(ctx, workspacepkg.Workspace{
			ID: "workspace-a", RootDir: t.TempDir(), Name: "workspace-a",
		}); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		if err := reopened.DeleteWorkspace(ctx, "workspace-a"); err != nil {
			t.Fatalf("DeleteWorkspace() error = %v", err)
		}
		_, err = reopened.GetSkillExposureByOwnerTarget(
			ctx, "review", store.SkillExposureOwnerWorkspace, "workspace-a", "agents",
		)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("workspace cleanup exposure error = %v, want sql.ErrNoRows", err)
		}
		if err := reopened.DeleteSkillExposure(ctx, userRecord.ID); err != nil {
			t.Fatalf("DeleteSkillExposure() error = %v", err)
		}
		_, err = reopened.GetSkillExposureByOwnerTarget(
			ctx, "review", store.SkillExposureOwnerUser, "", "agents",
		)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("GetSkillExposureByOwnerTarget(deleted) error = %v, want sql.ErrNoRows", err)
		}
	})
}

func TestGlobalDBSkillExposureIndexMigration(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve exposure records while dropping the redundant name index", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00091_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open v90 migration prefix error = %v", err)
		}
		if _, err := prefixDB.ExecContext(t.Context(), `
			INSERT INTO skill_exposures (
				skill_name, canonical_dir, target_slug, link_path, link_target,
				owner_scope, workspace_id, created_at, updated_at
			) VALUES ('review', '/skills/review', 'agents', '/agents/review',
				'../../skills/review', 'user', NULL, '2026-08-25T12:00:00Z', '2026-08-25T12:00:00Z')
		`); err != nil {
			t.Fatalf("insert v90 exposure error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("close v90 prefix error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("open upgraded database error = %v", err)
		}
		t.Cleanup(func() {
			if err := upgraded.Close(context.Background()); err != nil {
				t.Errorf("Close(upgraded) error = %v", err)
			}
		})
		record, err := upgraded.GetSkillExposureByOwnerTarget(
			t.Context(), "review", store.SkillExposureOwnerUser, "", "agents",
		)
		if err != nil || record.CanonicalDir != "/skills/review" {
			t.Fatalf("upgraded exposure = %#v, error = %v", record, err)
		}
		rows, err := upgraded.DB().QueryContext(t.Context(), `PRAGMA index_list('skill_exposures')`)
		if err != nil {
			t.Fatalf("query skill exposure indexes error = %v", err)
		}
		defer func() {
			if err := rows.Close(); err != nil {
				t.Errorf("Close(index rows) error = %v", err)
			}
		}()
		for rows.Next() {
			var sequence int
			var name string
			var unique int
			var origin string
			var partial int
			if err := rows.Scan(&sequence, &name, &unique, &origin, &partial); err != nil {
				t.Fatalf("scan skill exposure index error = %v", err)
			}
			if name == "idx_skill_exposures_skill_name" {
				t.Fatalf("redundant index remains after migration: %q", name)
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate skill exposure indexes error = %v", err)
		}
	})
}

func assertSkillExposureCheckConstraint(t *testing.T, database *GlobalDB, record store.SkillExposureRecord) {
	t.Helper()
	if _, err := database.CreateSkillExposure(context.Background(), record); err == nil ||
		!strings.Contains(err.Error(), "CHECK constraint failed") {
		t.Fatalf(
			"CreateSkillExposure(%s/%s) error = %v, want CHECK constraint",
			record.OwnerScope,
			record.WorkspaceID,
			err,
		)
	}
}
