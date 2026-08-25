package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
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
			t.Fatalf("record IDs are not distinct positive values: user=%d workspace=%d", userRecord.ID, workspaceRecord.ID)
		}

		if _, err := database.CreateSkillExposure(ctx, store.SkillExposureRecord{
			SkillName: "review", CanonicalDir: "/other/review", TargetSlug: "agents",
			LinkPath: "/provider/agents/review-duplicate-owner", LinkTarget: "../../other/review",
			OwnerScope: store.SkillExposureOwnerUser,
		}); err == nil {
			t.Fatal("duplicate user owner/target insert succeeded")
		}
		assertSkillExposureConstraint(t, database, store.SkillExposureRecord{
			SkillName: "invalid-user", CanonicalDir: "/canonical/invalid-user", TargetSlug: "agents",
			LinkPath: "/provider/agents/invalid-user", LinkTarget: "../../canonical/invalid-user",
			OwnerScope: store.SkillExposureOwnerUser, WorkspaceID: "workspace-a",
		})
		assertSkillExposureConstraint(t, database, store.SkillExposureRecord{
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

func assertSkillExposureConstraint(t *testing.T, database *GlobalDB, record store.SkillExposureRecord) {
	t.Helper()
	if _, err := database.CreateSkillExposure(context.Background(), record); err == nil {
		t.Fatalf("CreateSkillExposure(%s/%s) error = nil, want constraint failure", record.OwnerScope, record.WorkspaceID)
	}
}
