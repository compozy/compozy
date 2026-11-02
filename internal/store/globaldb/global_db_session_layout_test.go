package globaldb

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestSessionLayoutFolders(t *testing.T) {
	t.Parallel()

	t.Run("Should create folders at the top, rename, and reorder them", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db, scope := openSessionLayoutTestDB(t, "folders")

		first := createLayoutFolder(ctx, t, db, scope, "sfold-a", "Alpha")
		createLayoutFolder(ctx, t, db, scope, "sfold-b", "Beta")
		if first.Position != 0 || first.Name != "Alpha" {
			t.Fatalf("first folder = %#v, want position 0 named Alpha", first)
		}
		assertFolderOrder(ctx, t, db, scope, "sfold-b", "sfold-a")

		renamed, err := db.RenameSessionFolder(ctx, scope, "sfold-a", "  Release  ")
		if err != nil {
			t.Fatalf("RenameSessionFolder() error = %v", err)
		}
		if renamed.Name != "Release" {
			t.Fatalf("renamed folder name = %q, want Release", renamed.Name)
		}

		reordered, err := db.ReorderSessionFolders(ctx, scope, []string{"sfold-a", "sfold-b"})
		if err != nil {
			t.Fatalf("ReorderSessionFolders() error = %v", err)
		}
		if got := folderIDs(reordered); !reflect.DeepEqual(got, []string{"sfold-a", "sfold-b"}) {
			t.Fatalf("reordered folders = %v", got)
		}
		_, err = db.ReorderSessionFolders(ctx, scope, []string{"sfold-a"})
		if !errors.Is(err, store.ErrSessionLayoutConflict) {
			t.Fatalf("partial folder order error = %v, want ErrSessionLayoutConflict", err)
		}
	})

	t.Run("Should reject blank and oversized folder names", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db, scope := openSessionLayoutTestDB(t, "names")
		multibyte := strings.Repeat("é", store.MaxSessionFolderNameLength)
		if _, err := db.CreateSessionFolder(ctx, scope, "sfold-wide", multibyte); err != nil {
			t.Fatalf("CreateSessionFolder(%d multibyte characters) error = %v", store.MaxSessionFolderNameLength, err)
		}
		for _, name := range []string{"   ", strings.Repeat("x", store.MaxSessionFolderNameLength+1)} {
			if _, err := db.CreateSessionFolder(ctx, scope, "sfold-bad", name); !errors.Is(
				err,
				store.ErrSessionLayoutInvalid,
			) {
				t.Fatalf("CreateSessionFolder(%q) error = %v, want ErrSessionLayoutInvalid", name, err)
			}
		}
	})

	t.Run("Should return member sessions to the root when a folder is deleted", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db, scope := openSessionLayoutTestDB(t, "delete")
		registerLayoutSession(ctx, t, db, scope.WorkspaceID, "sess-a")
		createLayoutFolder(ctx, t, db, scope, "sfold-a", "Alpha")
		if err := db.MoveSession(ctx, scope, "sess-a", "sfold-a"); err != nil {
			t.Fatalf("MoveSession() error = %v", err)
		}

		if err := db.DeleteSessionFolder(ctx, scope, "sfold-a"); err != nil {
			t.Fatalf("DeleteSessionFolder() error = %v", err)
		}
		placements := readPlacements(ctx, t, db, "sess-a")
		if _, ok := placements["sess-a"]; ok {
			t.Fatalf("placement after folder delete = %#v, want root (no placement)", placements["sess-a"])
		}
		page := pageManual(ctx, t, db, scope.WorkspaceID, nil)
		if len(page.Sessions) != 1 || page.Sessions[0].ID != "sess-a" || page.Sessions[0].ArchivedAt != nil {
			t.Fatalf("sessions after folder delete = %#v, want sess-a kept unarchived", page.Sessions)
		}
		if err := db.DeleteSessionFolder(ctx, scope, "sfold-a"); !errors.Is(err, store.ErrSessionFolderNotFound) {
			t.Fatalf("second delete error = %v, want ErrSessionFolderNotFound", err)
		}
	})
}

func TestSessionLayoutPlacements(t *testing.T) {
	t.Parallel()

	t.Run("Should pin listed sessions first and keep unlisted members after them", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db, scope := openSessionLayoutTestDB(t, "reorder")
		for _, id := range []string{"sess-a", "sess-b", "sess-c"} {
			registerLayoutSession(ctx, t, db, scope.WorkspaceID, id)
		}
		createLayoutFolder(ctx, t, db, scope, "sfold-a", "Alpha")
		if err := db.ReorderSessions(ctx, scope, "sfold-a", []string{"sess-a", "sess-b", "sess-c"}); err != nil {
			t.Fatalf("ReorderSessions(full) error = %v", err)
		}
		if err := db.ReorderSessions(ctx, scope, "sfold-a", []string{"sess-c"}); err != nil {
			t.Fatalf("ReorderSessions(partial) error = %v", err)
		}
		placements := readPlacements(ctx, t, db, "sess-a", "sess-b", "sess-c")
		got := map[string]int{}
		for id, placement := range placements {
			if placement.FolderID != "sfold-a" {
				t.Fatalf("placement %s folder = %q, want sfold-a", id, placement.FolderID)
			}
			got[id] = placement.Position
		}
		want := map[string]int{"sess-c": 0, "sess-a": 1, "sess-b": 2}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("positions = %v, want %v", got, want)
		}
	})

	t.Run("Should append moves to a folder and unplace moves to the root", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db, scope := openSessionLayoutTestDB(t, "move")
		registerLayoutSession(ctx, t, db, scope.WorkspaceID, "sess-a")
		registerLayoutSession(ctx, t, db, scope.WorkspaceID, "sess-b")
		createLayoutFolder(ctx, t, db, scope, "sfold-a", "Alpha")
		for _, id := range []string{"sess-a", "sess-b", "sess-a"} {
			if err := db.MoveSession(ctx, scope, id, "sfold-a"); err != nil {
				t.Fatalf("MoveSession(%s) error = %v", id, err)
			}
		}
		placements := readPlacements(ctx, t, db, "sess-a", "sess-b")
		if placements["sess-a"].Position != 0 || placements["sess-b"].Position != 1 {
			t.Fatalf("placements = %#v, want sess-a=0 sess-b=1", placements)
		}
		if err := db.MoveSession(ctx, scope, "sess-a", ""); err != nil {
			t.Fatalf("MoveSession(root) error = %v", err)
		}
		if _, ok := readPlacements(ctx, t, db, "sess-a")["sess-a"]; ok {
			t.Fatal("sess-a placement survived a move to the root")
		}
	})

	t.Run("Should append after the last member when earlier moves left gaps", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db, scope := openSessionLayoutTestDB(t, "gaps")
		for _, id := range []string{"sess-a", "sess-b", "sess-c", "sess-d"} {
			registerLayoutSession(ctx, t, db, scope.WorkspaceID, id)
		}
		createLayoutFolder(ctx, t, db, scope, "sfold-a", "Alpha")
		for _, id := range []string{"sess-a", "sess-b", "sess-c"} {
			if err := db.MoveSession(ctx, scope, id, "sfold-a"); err != nil {
				t.Fatalf("MoveSession(%s) error = %v", id, err)
			}
		}
		if err := db.MoveSession(ctx, scope, "sess-b", ""); err != nil {
			t.Fatalf("MoveSession(sess-b, root) error = %v", err)
		}
		if err := db.MoveSession(ctx, scope, "sess-d", "sfold-a"); err != nil {
			t.Fatalf("MoveSession(sess-d) error = %v", err)
		}
		placements := readPlacements(ctx, t, db, "sess-a", "sess-c", "sess-d")
		if placements["sess-d"].Position <= placements["sess-c"].Position {
			t.Fatalf("placements = %#v, want sess-d after sess-c", placements)
		}
	})

	t.Run("Should isolate workspaces, profiles, and subagent sessions", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db, scope := openSessionLayoutTestDB(t, "isolation")
		other := registerWorkspaceForGlobalTests(t, db, "isolation-other", filepath.Join(t.TempDir(), "other"))
		otherScope := store.SessionLayoutScope{ProfileID: scope.ProfileID, WorkspaceID: other}
		registerLayoutSession(ctx, t, db, scope.WorkspaceID, "sess-here")
		registerLayoutSession(ctx, t, db, other, "sess-there")
		registerSubagentSession(t, db, scope.WorkspaceID, "sess-sub", "sess-here", store.SubagentSpawnRole, time.Now())
		createLayoutFolder(ctx, t, db, scope, "sfold-here", "Here")
		createLayoutFolder(ctx, t, db, otherScope, "sfold-there", "There")

		if err := db.MoveSession(ctx, scope, "sess-there", "sfold-here"); !errors.Is(err, store.ErrSessionNotFound) {
			t.Fatalf("cross-workspace session move error = %v, want ErrSessionNotFound", err)
		}
		if err := db.MoveSession(ctx, scope, "sess-here", "sfold-there"); !errors.Is(
			err,
			store.ErrSessionFolderNotFound,
		) {
			t.Fatalf("cross-workspace folder move error = %v, want ErrSessionFolderNotFound", err)
		}
		if err := db.MoveSession(
			ctx,
			scope,
			"sess-sub",
			"sfold-here",
		); !errors.Is(
			err,
			store.ErrSessionLayoutSubagent,
		) {
			t.Fatalf("subagent move error = %v, want ErrSessionLayoutSubagent", err)
		}

		secondProfile := insertLayoutTestProfile(ctx, t, db)
		profileScope := store.SessionLayoutScope{ProfileID: secondProfile, WorkspaceID: scope.WorkspaceID}
		if err := db.MoveSession(ctx, profileScope, "sess-here", ""); !errors.Is(err, store.ErrSessionNotFound) {
			t.Fatalf("cross-profile session move error = %v, want ErrSessionNotFound", err)
		}
		folders, err := db.ListSessionFolders(ctx, profileScope)
		if err != nil || len(folders) != 0 {
			t.Fatalf("other profile folders = %v, %v; want none", folders, err)
		}
	})

	t.Run("Should keep membership across archive and drop it when the session is deleted", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db, scope := openSessionLayoutTestDB(t, "archive")
		registerLayoutSession(ctx, t, db, scope.WorkspaceID, "sess-a")
		createLayoutFolder(ctx, t, db, scope, "sfold-a", "Alpha")
		if err := db.MoveSession(ctx, scope, "sess-a", "sfold-a"); err != nil {
			t.Fatalf("MoveSession() error = %v", err)
		}
		if _, err := db.SetSessionArchived(ctx, scope.WorkspaceID, "sess-a", true); err != nil {
			t.Fatalf("archive error = %v", err)
		}
		assertFolderCount(ctx, t, db, scope, 0)
		if _, err := db.SetSessionArchived(ctx, scope.WorkspaceID, "sess-a", false); err != nil {
			t.Fatalf("unarchive error = %v", err)
		}
		assertFolderCount(ctx, t, db, scope, 1)
		if readPlacements(ctx, t, db, "sess-a")["sess-a"].FolderID != "sfold-a" {
			t.Fatal("unarchived session lost its folder membership")
		}
		if err := db.DeleteSession(ctx, "sess-a"); err != nil {
			t.Fatalf("DeleteSession() error = %v", err)
		}
		if len(readPlacements(ctx, t, db, "sess-a")) != 0 {
			t.Fatal("placement survived session delete")
		}
		assertFolderCount(ctx, t, db, scope, 0)
	})
}

func openSessionLayoutTestDB(t *testing.T, name string) (*GlobalDB, store.SessionLayoutScope) {
	t.Helper()
	db := openTestGlobalDB(t)
	workspace := registerWorkspaceForGlobalTests(t, db, "layout-"+name, filepath.Join(t.TempDir(), "workspace"))
	return db, store.SessionLayoutScope{ProfileID: store.DefaultProfileID, WorkspaceID: workspace}
}

func registerLayoutSession(ctx context.Context, t *testing.T, db *GlobalDB, workspace string, id string) {
	t.Helper()
	info := sessionInfoForWorkspaceStateIndexTest(
		id,
		workspace,
		globalDBSessionStateStopped,
		time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC),
	)
	if err := db.RegisterSession(ctx, &(info)); err != nil {
		t.Fatalf("RegisterSession(%s) error = %v", id, err)
	}
}

func createLayoutFolder(
	ctx context.Context,
	t *testing.T,
	db *GlobalDB,
	scope store.SessionLayoutScope,
	id string,
	name string,
) store.SessionFolder {
	t.Helper()
	folder, err := db.CreateSessionFolder(ctx, scope, id, name)
	if err != nil {
		t.Fatalf("CreateSessionFolder(%s) error = %v", id, err)
	}
	return folder
}

func assertFolderOrder(
	ctx context.Context,
	t *testing.T,
	db *GlobalDB,
	scope store.SessionLayoutScope,
	want ...string,
) {
	t.Helper()
	folders, err := db.ListSessionFolders(ctx, scope)
	if err != nil {
		t.Fatalf("ListSessionFolders() error = %v", err)
	}
	if got := folderIDs(folders); !reflect.DeepEqual(got, want) {
		t.Fatalf("folder order = %v, want %v", got, want)
	}
}

func assertFolderCount(ctx context.Context, t *testing.T, db *GlobalDB, scope store.SessionLayoutScope, want int) {
	t.Helper()
	folders, err := db.ListSessionFolders(ctx, scope)
	if err != nil || len(folders) != 1 {
		t.Fatalf("ListSessionFolders() = %v, %v; want one folder", folders, err)
	}
	if folders[0].SessionCount != want {
		t.Fatalf("folder session_count = %d, want %d", folders[0].SessionCount, want)
	}
}

func folderIDs(folders []store.SessionFolder) []string {
	ids := make([]string, 0, len(folders))
	for _, folder := range folders {
		ids = append(ids, folder.ID)
	}
	return ids
}

func readPlacements(
	ctx context.Context,
	t *testing.T,
	db *GlobalDB,
	ids ...string,
) map[string]store.SessionPlacement {
	t.Helper()
	placements, err := db.SessionPlacements(ctx, ids)
	if err != nil {
		t.Fatalf("SessionPlacements() error = %v", err)
	}
	return placements
}

func pageManual(
	ctx context.Context,
	t *testing.T,
	db *GlobalDB,
	workspace string,
	after *store.SessionCatalogPosition,
) store.SessionCatalogPage {
	t.Helper()
	page, err := db.PageSessions(ctx, store.SessionCatalogPageQuery{
		ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
		WorkspaceID: workspace,
		Archive:     store.SessionArchiveInclude,
		Sort:        "manual",
		Limit:       50,
		After:       after,
	})
	if err != nil {
		t.Fatalf("PageSessions(manual) error = %v", err)
	}
	return page
}

func insertLayoutTestProfile(ctx context.Context, t *testing.T, db *GlobalDB) string {
	t.Helper()
	id := strings.Repeat("c", 26)
	if _, err := db.db.ExecContext(
		ctx,
		`INSERT INTO profiles (id, name, color, icon, state, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, "layout-secondary", "#8E8EB5", "circle", "active",
		store.FormatTimestamp(time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)),
	); err != nil {
		t.Fatalf("insert secondary profile: %v", err)
	}
	return id
}

func TestSessionLayoutUserScope(t *testing.T) {
	t.Run("Should keep User folders placements and catalog isolated by profile", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := t.Context()
		other := insertLayoutTestProfile(ctx, t, db)
		scope := store.SessionLayoutScope{ProfileID: store.DefaultProfileID}
		projectID := registerWorkspaceForGlobalTests(t, db, "project", filepath.Join(t.TempDir(), "project"))
		registerLayoutSession(ctx, t, db, projectID, "sess-project")
		registerLayoutSession(ctx, t, db, "", "sess-user")
		folder := createLayoutFolder(ctx, t, db, scope, "folder-user", "Personal")
		if err := db.MoveSession(ctx, scope, "sess-user", folder.ID); err != nil {
			t.Fatal(err)
		}
		foreign := sessionInfoForWorkspaceStateIndexTest(
			"sess-user-other",
			"",
			globalDBSessionStateStopped,
			time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC),
		)
		foreign.ProfileID = other
		if err := db.RegisterSession(ctx, &(foreign)); err != nil {
			t.Fatal(err)
		}
		sessions, err := db.ListSessions(
			ctx,
			store.SessionListQuery{
				ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
				WorkspaceID: workspaceaccess.UserSelector,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(sessions) != 1 || sessions[0].ID != "sess-user" {
			t.Fatalf("sessions=%+v", sessions)
		}
		folders, err := db.ListSessionFolders(ctx, store.SessionLayoutScope{ProfileID: other})
		if err != nil || len(folders) != 0 {
			t.Fatalf("foreign folders=%+v err=%v", folders, err)
		}
		all, err := db.ListSessions(
			ctx,
			store.SessionListQuery{
				ReadScope:   store.ReadScope{AllProfiles: true},
				WorkspaceID: workspaceaccess.UserSelector,
			},
		)
		if err != nil || len(all) != 2 {
			t.Fatalf("all profiles=%+v err=%v", all, err)
		}
	})
}

func TestWorkspaceLayout(t *testing.T) {
	t.Parallel()

	t.Run("Should order folders and placements and keep unlisted members after a partial reorder", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		profile := store.DefaultProfileID
		alpha := registerWorkspaceForGlobalTests(t, db, "alpha", filepath.Join(t.TempDir(), "alpha"))
		beta := registerWorkspaceForGlobalTests(t, db, "beta", filepath.Join(t.TempDir(), "beta"))
		gamma := registerWorkspaceForGlobalTests(t, db, "gamma", filepath.Join(t.TempDir(), "gamma"))

		createWorkspaceLayoutFolder(ctx, t, db, profile, "wfold-a", "Clients")
		createWorkspaceLayoutFolder(ctx, t, db, profile, "wfold-b", "Internal")
		for _, move := range []struct{ workspace, folder string }{
			{alpha, "wfold-a"}, {beta, "wfold-a"}, {gamma, ""}, {"", ""},
		} {
			if err := db.MoveWorkspace(ctx, profile, move.workspace, move.folder); err != nil {
				t.Fatalf("MoveWorkspace(%q, %q) error = %v", move.workspace, move.folder, err)
			}
		}
		if err := db.ReorderWorkspaces(ctx, profile, "wfold-a", []string{beta}); err != nil {
			t.Fatalf("ReorderWorkspaces() error = %v", err)
		}
		if err := db.ReorderWorkspaceFolders(ctx, profile, []string{"wfold-a", "wfold-b"}); err != nil {
			t.Fatalf("ReorderWorkspaceFolders() error = %v", err)
		}
		err := db.ReorderWorkspaceFolders(ctx, profile, []string{"wfold-a"})
		if !errors.Is(err, store.ErrWorkspaceLayoutConflict) {
			t.Fatalf("partial folder order error = %v, want ErrWorkspaceLayoutConflict", err)
		}

		layout := readWorkspaceLayoutForTest(ctx, t, db, profile)
		if got := workspaceFolderIDs(layout.Folders); !reflect.DeepEqual(got, []string{"wfold-a", "wfold-b"}) {
			t.Fatalf("folders = %v", got)
		}
		if layout.Folders[0].WorkspaceCount != 2 || layout.Folders[1].WorkspaceCount != 0 {
			t.Fatalf("folder counts = %+v", layout.Folders)
		}
		want := []store.WorkspacePlacement{
			{WorkspaceID: beta, FolderID: "wfold-a", Position: 0},
			{WorkspaceID: alpha, FolderID: "wfold-a", Position: 1},
			{WorkspaceID: gamma, Position: 0},
			{WorkspaceID: "", Position: 1},
		}
		if !reflect.DeepEqual(layout.Placements, want) {
			t.Fatalf("placements = %+v, want %+v", layout.Placements, want)
		}
	})

	t.Run("Should return workspaces to the root when their folder or workspace is deleted", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		profile := store.DefaultProfileID
		alpha := registerWorkspaceForGlobalTests(t, db, "alpha", filepath.Join(t.TempDir(), "alpha"))
		beta := registerWorkspaceForGlobalTests(t, db, "beta", filepath.Join(t.TempDir(), "beta"))
		createWorkspaceLayoutFolder(ctx, t, db, profile, "wfold-a", "Clients")
		for _, workspace := range []string{alpha, beta} {
			if err := db.MoveWorkspace(ctx, profile, workspace, "wfold-a"); err != nil {
				t.Fatalf("MoveWorkspace(%s) error = %v", workspace, err)
			}
		}
		if err := db.DeleteWorkspace(ctx, beta); err != nil {
			t.Fatalf("DeleteWorkspace() error = %v", err)
		}
		if got := readWorkspaceLayoutForTest(
			ctx,
			t,
			db,
			profile,
		).Placements; len(got) != 1 ||
			got[0].WorkspaceID != alpha {
			t.Fatalf("placements after workspace delete = %+v", got)
		}
		if err := db.DeleteWorkspaceFolder(ctx, profile, "wfold-a"); err != nil {
			t.Fatalf("DeleteWorkspaceFolder() error = %v", err)
		}
		layout := readWorkspaceLayoutForTest(ctx, t, db, profile)
		if len(layout.Folders) != 0 || len(layout.Placements) != 0 {
			t.Fatalf("layout after folder delete = %+v", layout)
		}
		if _, err := db.GetWorkspace(ctx, alpha); err != nil {
			t.Fatalf("GetWorkspace(alpha) after folder delete error = %v", err)
		}
		if err := db.DeleteWorkspaceFolder(ctx, profile, "wfold-a"); !errors.Is(err, store.ErrWorkspaceFolderNotFound) {
			t.Fatalf("second delete error = %v, want ErrWorkspaceFolderNotFound", err)
		}
	})

	t.Run("Should isolate folders by profile and reject unknown targets", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		other := insertLayoutTestProfile(ctx, t, db)
		alpha := registerWorkspaceForGlobalTests(t, db, "alpha", filepath.Join(t.TempDir(), "alpha"))
		createWorkspaceLayoutFolder(ctx, t, db, store.DefaultProfileID, "wfold-a", "Clients")

		if err := db.MoveWorkspace(ctx, other, alpha, "wfold-a"); !errors.Is(err, store.ErrWorkspaceFolderNotFound) {
			t.Fatalf("foreign folder move error = %v, want ErrWorkspaceFolderNotFound", err)
		}
		if _, err := db.RenameWorkspaceFolder(ctx, other, "wfold-a", "Mine"); !errors.Is(
			err,
			store.ErrWorkspaceFolderNotFound,
		) {
			t.Fatalf("foreign rename error = %v, want ErrWorkspaceFolderNotFound", err)
		}
		if err := db.MoveWorkspace(ctx, other, "ws-missing", ""); !errors.Is(
			err,
			store.ErrWorkspaceLayoutTargetNotFound,
		) {
			t.Fatalf("unknown workspace move error = %v, want ErrWorkspaceLayoutTargetNotFound", err)
		}
		if _, err := db.CreateWorkspaceFolder(ctx, other, "wfold-x", "   "); !errors.Is(
			err,
			store.ErrWorkspaceLayoutInvalid,
		) {
			t.Fatalf("blank name error = %v, want ErrWorkspaceLayoutInvalid", err)
		}
		if layout := readWorkspaceLayoutForTest(ctx, t, db, other); len(layout.Folders) != 0 {
			t.Fatalf("foreign profile layout = %+v", layout)
		}
	})
}

func createWorkspaceLayoutFolder(
	ctx context.Context,
	t *testing.T,
	db *GlobalDB,
	profile string,
	id string,
	name string,
) {
	t.Helper()
	if _, err := db.CreateWorkspaceFolder(ctx, profile, id, name); err != nil {
		t.Fatalf("CreateWorkspaceFolder(%s) error = %v", id, err)
	}
}

func readWorkspaceLayoutForTest(ctx context.Context, t *testing.T, db *GlobalDB, profile string) store.WorkspaceLayout {
	t.Helper()
	layout, err := db.WorkspaceLayout(ctx, profile)
	if err != nil {
		t.Fatalf("WorkspaceLayout() error = %v", err)
	}
	return layout
}

func workspaceFolderIDs(folders []store.WorkspaceFolder) []string {
	ids := make([]string, 0, len(folders))
	for _, folder := range folders {
		ids = append(ids, folder.ID)
	}
	return ids
}
