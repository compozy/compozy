package globaldb

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// Suite: profile attention workspace mutes.
// Invariant: one profile's mute set cannot alter another profile and workspace deletion cascades every owner row.
// Boundary IN: globaldb attention repository and SQLite constraints.
// Boundary OUT: settings/API profile selection, owned by internal/settings and internal/api/core.
func TestAttentionWorkspaceMutes(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate replacements by profile and cascade workspace deletion", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openAttentionTestDB(t)
		const marketingProfileID = "01K34MARKETINGPROFILE00000"
		insertAttentionTestProfile(t, db, marketingProfileID, "marketing")
		workspaceA := insertAttentionTestWorkspace(t, db, "ws_0123456789abcdef", "alpha")
		workspaceB := insertAttentionTestWorkspace(t, db, "ws_abcdef0123456789", "beta")

		if err := db.ReplaceAttentionWorkspaceMutes(ctx, store.DefaultProfileID, []string{workspaceA.ID}); err != nil {
			t.Fatalf("ReplaceAttentionWorkspaceMutes(default) error = %v", err)
		}
		if err := db.ReplaceAttentionWorkspaceMutes(ctx, marketingProfileID, []string{workspaceB.ID}); err != nil {
			t.Fatalf("ReplaceAttentionWorkspaceMutes(marketing) error = %v", err)
		}
		if err := db.ReplaceAttentionWorkspaceMutes(ctx, store.DefaultProfileID, []string{workspaceB.ID}); err != nil {
			t.Fatalf("ReplaceAttentionWorkspaceMutes(default second) error = %v", err)
		}
		marketing, err := db.ListAttentionWorkspaceMutes(ctx, marketingProfileID)
		if err != nil {
			t.Fatalf("ListAttentionWorkspaceMutes(marketing) error = %v", err)
		}
		if !reflect.DeepEqual(marketing, []string{workspaceB.ID}) {
			t.Fatalf("marketing mutes = %#v, want beta only", marketing)
		}

		muted, err := db.IsAttentionWorkspaceMuted(ctx, store.DefaultProfileID, workspaceB.ID)
		if err != nil || !muted {
			t.Fatalf("IsAttentionWorkspaceMuted(default, beta) = %t, error = %v, want true", muted, err)
		}
		if err := db.DeleteWorkspace(ctx, workspaceB.ID); err != nil {
			t.Fatalf("DeleteWorkspace(beta) error = %v", err)
		}
		for _, profileID := range []string{store.DefaultProfileID, marketingProfileID} {
			mutes, err := db.ListAttentionWorkspaceMutes(ctx, profileID)
			if err != nil {
				t.Fatalf("ListAttentionWorkspaceMutes(%s) error = %v", profileID, err)
			}
			if len(mutes) != 0 {
				t.Fatalf("mutes after workspace deletion for %s = %#v, want empty", profileID, mutes)
			}
		}
	})

	t.Run("Should roll back the complete replacement when one workspace is unknown", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openAttentionTestDB(t)
		workspace := insertAttentionTestWorkspace(t, db, "ws_0123456789abcdef", "alpha")
		if err := db.ReplaceAttentionWorkspaceMutes(ctx, store.DefaultProfileID, []string{workspace.ID}); err != nil {
			t.Fatalf("ReplaceAttentionWorkspaceMutes(seed) error = %v", err)
		}
		err := db.ReplaceAttentionWorkspaceMutes(
			ctx,
			store.DefaultProfileID,
			[]string{"ws_abcdef0123456789", "ws_1111111111111111"},
		)
		if err == nil {
			t.Fatal("ReplaceAttentionWorkspaceMutes(unknown) error = nil")
		}
		mutes, listErr := db.ListAttentionWorkspaceMutes(ctx, store.DefaultProfileID)
		if listErr != nil {
			t.Fatalf("ListAttentionWorkspaceMutes(after rollback) error = %v", listErr)
		}
		if !reflect.DeepEqual(mutes, []string{workspace.ID}) {
			t.Fatalf("mutes after rollback = %#v, want original workspace", mutes)
		}
	})
}

func openAttentionTestDB(t *testing.T) *GlobalDB {
	t.Helper()
	ctx := testutil.Context(t)
	db, err := OpenGlobalDB(ctx, filepath.Join(t.TempDir(), GlobalDatabaseName))
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return db
}

func insertAttentionTestProfile(t *testing.T, db *GlobalDB, id string, name string) {
	t.Helper()
	if _, err := db.db.ExecContext(testutil.Context(t), `
		INSERT INTO profiles (id, name, color, icon, state, created_at)
		VALUES (?, ?, '#E8572A', 'briefcase', 'active', ?)`, id, name, store.FormatTimestamp(time.Now().UTC())); err != nil {
		t.Fatalf("insert profile %q error = %v", name, err)
	}
}

func insertAttentionTestWorkspace(t *testing.T, db *GlobalDB, id string, name string) workspacepkg.Workspace {
	t.Helper()
	now := time.Now().UTC()
	workspace := workspacepkg.Workspace{
		ID: id, RootDir: filepath.Join(t.TempDir(), name), Name: name, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.InsertWorkspace(testutil.Context(t), workspace); err != nil {
		t.Fatalf("InsertWorkspace(%s) error = %v", name, err)
	}
	return workspace
}
