package globaldb

import (
	"errors"
	"fmt"
	"github.com/compozy/compozy/internal/notifications"
	"path/filepath"
	"reflect"
	"strings"
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

// Invariant: receipts are durable, actor/profile scoped and atomic over the captured population.
// Owner: SQLite notification repository. Canonical suite: profile attention persistence.
func TestAttentionAcknowledgements(t *testing.T) {
	t.Parallel()
	scope := notifications.AttentionScope{ProfileID: store.DefaultProfileID, ActorKind: "human", ActorID: "operator", Population: "bell"}
	t.Run("Should acknowledge a complete snapshot without consuming a racing occurrence", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openAttentionTestDB(t)
		var repository notifications.AttentionStore = db
		ids := make([]string, 251)
		for i := range ids {
			ids[i] = fmt.Sprintf("occurrence-%03d", i)
		}
		snapshot, unread, err := repository.CaptureAttentionSnapshot(ctx, scope, ids)
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := len(unread); got != len(ids) {
			t.Fatalf("length = %d, want %d", got, len(ids))
		}
		if err := repository.AcknowledgeAttentionSnapshot(ctx, scope, snapshot, ids[0]); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, unread, err = repository.CaptureAttentionSnapshot(ctx, scope, ids)
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := len(unread); got != len(ids)-1 {
			t.Fatalf("length = %d, want %d", got, len(ids)-1)
		}
		ids = append(ids, "new-occurrence")
		if err := repository.AcknowledgeAttentionSnapshot(ctx, scope, snapshot, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := repository.AcknowledgeAttentionSnapshot(ctx, scope, snapshot, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, unread, err = repository.CaptureAttentionSnapshot(ctx, scope, ids)
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := unread, []string{"new-occurrence"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
		// A second SQLite connection models reload/reconnect, without sharing a browser cache.
		reopened, err := OpenGlobalDB(ctx, db.Path())
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("close: %v", err)
			}
		})
		_, unread, err = reopened.CaptureAttentionSnapshot(ctx, scope, ids)
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := unread, []string{"new-occurrence"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})
	t.Run("Should reject foreign snapshots and share receipts only within the same operator profile", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openAttentionTestDB(t)
		insertAttentionTestProfile(t, db, "01K34OTHERPROFILE000000000", "other")
		snapshot, _, err := db.CaptureAttentionSnapshot(ctx, scope, []string{"one"})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, change := range []notifications.AttentionScope{
			{ProfileID: "01K34OTHERPROFILE000000000", ActorKind: scope.ActorKind, ActorID: scope.ActorID, Population: scope.Population},
			{ProfileID: scope.ProfileID, ActorKind: scope.ActorKind, ActorID: "other", Population: scope.Population},
			{ProfileID: scope.ProfileID, ActorKind: scope.ActorKind, ActorID: scope.ActorID, Population: "home-other-workspace"},
		} {
			if err := db.AcknowledgeAttentionSnapshot(ctx, change, snapshot, ""); !errors.Is(err, notifications.ErrAttentionSnapshotUnavailable) {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		if err := db.AcknowledgeAttentionSnapshot(ctx, scope, snapshot, "outside"); !errors.Is(err, notifications.ErrAttentionSnapshotUnavailable) {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := db.AcknowledgeAttentionSnapshot(ctx, scope, snapshot, "one"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		home := scope
		home.Population = "home-selected-workspace"
		_, unread, err := db.CaptureAttentionSnapshot(ctx, home, []string{"one"})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(unread) != 0 {
			t.Fatalf("expected empty, got %v", unread)
		}
		other := scope
		other.ActorID = "other"
		_, unread, err = db.CaptureAttentionSnapshot(ctx, other, []string{"one"})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := unread, []string{"one"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
		other = scope
		other.ProfileID = "01K34OTHERPROFILE000000000"
		_, unread, err = db.CaptureAttentionSnapshot(ctx, other, []string{"one"})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := unread, []string{"one"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})
	t.Run("Should retain every row after a partial bulk write failure and reject expired snapshots", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openAttentionTestDB(t)
		now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
		db.now = func() time.Time { return now }
		snapshot, _, err := db.CaptureAttentionSnapshot(ctx, scope, []string{"a", "b"})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = db.db.ExecContext(ctx, `CREATE TRIGGER fail_attention_receipt BEFORE INSERT ON attention_acknowledgements WHEN NEW.occurrence_id = 'b' BEGIN SELECT RAISE(ABORT, 'injected storage failure'); END`)
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := db.AcknowledgeAttentionSnapshot(ctx, scope, snapshot, ""); err == nil || !strings.Contains(err.Error(), "injected storage failure") {
			t.Fatalf("unexpected error: %v", err)
		}
		_, unread, err := db.CaptureAttentionSnapshot(ctx, scope, []string{"a", "b"})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got, want := unread, []string{"a", "b"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
		now = now.Add(attentionSnapshotLifetime + time.Second)
		err = db.AcknowledgeAttentionSnapshot(ctx, scope, snapshot, "")
		if !(errors.Is(err, notifications.ErrAttentionSnapshotUnavailable)) {
			t.Fatal("expected true: errors.Is(err, notifications.ErrAttentionSnapshotUnavailable)")
		}
	})
}
