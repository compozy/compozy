package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveMatchesWorkspaceBySameFilesystemRoot(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve a registered alternate path to the same root", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		alternateRoot := symlinkWorkspaceRootForTest(t, root)
		ws := Workspace{ID: "ws_same_root", RootDir: alternateRoot, Name: "repo"}
		store := newMockWorkspaceStore(ws)
		loader := &countingConfigLoader{cfg: validConfig(homePaths)}
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader(loader.Load),
		)

		resolved, err := resolver.Resolve(ctx, root)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if got, want := resolved.ID, ws.ID; got != want {
			t.Fatalf("Resolve() ID = %q, want %q", got, want)
		}
		if store.listCalls == 0 {
			t.Fatal("ListWorkspaces() calls = 0, want same-root fallback")
		}
	})

	t.Run("Should classify a missing target as workspace not found before listing registrations", func(t *testing.T) {
		t.Parallel()

		store := newMockWorkspaceStore()
		resolver := newTestResolver(t, store)
		missing := filepath.Join(t.TempDir(), "missing")

		_, err := resolver.lookupWorkspaceBySameRoot(context.Background(), missing)
		if !errors.Is(err, ErrWorkspaceNotFound) {
			t.Fatalf("lookupWorkspaceBySameRoot(missing) error = %v, want ErrWorkspaceNotFound", err)
		}
		if store.listCalls != 0 {
			t.Fatalf("ListWorkspaces() calls = %d, want 0", store.listCalls)
		}
	})
}

func TestRegisterRejectsSameFilesystemRoot(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an alternate path to an existing root", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		alternateRoot := symlinkWorkspaceRootForTest(t, root)
		existing := Workspace{ID: "ws_existing", RootDir: alternateRoot, Name: "repo"}
		store := newMockWorkspaceStore(existing)
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
			WithIDGenerator(func(_ string) (string, error) { return "ws_duplicate", nil }),
		)

		_, err := resolver.Register(ctx, RegisterOptions{RootDir: root, Name: "repo-duplicate"})
		if !errors.Is(err, ErrWorkspacePathTaken) {
			t.Fatalf("Register() error = %v, want %v", err, ErrWorkspacePathTaken)
		}
	})
}

func TestResolveMissingRootReturnsErrWorkspaceRootMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := filepath.Join(t.TempDir(), "gone")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll(root) error = %v", err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("RemoveAll(root) error = %v", err)
	}

	store := newMockWorkspaceStore(Workspace{ID: "ws_missing", RootDir: root, Name: "repo"})
	loader := &countingConfigLoader{cfg: validConfig(homePaths)}
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(loader.Load),
	)

	if _, err := resolver.Resolve(ctx, "ws_missing"); !errors.Is(err, ErrWorkspaceRootMissing) {
		t.Fatalf("Resolve() error = %v, want %v", err, ErrWorkspaceRootMissing)
	}
}

func TestResolveCreatesAndLoadsStableWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	ws := Workspace{ID: "ws_identity", RootDir: root, Name: "repo"}
	store := newMockWorkspaceStore(ws)
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
	)

	first, err := resolver.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(first) error = %v", err)
	}
	if !IsWorkspaceID(first.WorkspaceID) {
		t.Fatalf("Resolve(first).WorkspaceID = %q, want workspace ULID", first.WorkspaceID)
	}
	identityPath := filepath.Join(first.RootDir, ".compozy", "workspace.toml")
	identity, err := loadIdentityFile(identityPath)
	if err != nil {
		t.Fatalf("loadIdentityFile(%q) error = %v", identityPath, err)
	}
	if identity.WorkspaceID != first.WorkspaceID {
		t.Fatalf("identity.WorkspaceID = %q, want %q", identity.WorkspaceID, first.WorkspaceID)
	}

	restarted := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
	)
	second, err := restarted.Resolve(ctx, ws.ID)
	if err != nil {
		t.Fatalf("Resolve(after restart) error = %v", err)
	}
	if second.WorkspaceID != first.WorkspaceID {
		t.Fatalf("Resolve(after restart).WorkspaceID = %q, want stable %q", second.WorkspaceID, first.WorkspaceID)
	}
}

func TestResolveMatchesWorkspaceByStableWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve a registered workspace by stable identity", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		root := t.TempDir()
		canonical, err := canonicalRoot(root)
		if err != nil {
			t.Fatalf("canonicalRoot(%q) error = %v", root, err)
		}
		if _, err := ensureIdentity(
			ctx,
			canonical,
			func() time.Time { return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC) },
			func() (string, error) { return testWorkspaceULID, nil },
		); err != nil {
			t.Fatalf("ensureIdentity(%q) error = %v", canonical, err)
		}

		ws := Workspace{ID: "ws_registered", RootDir: canonical, Name: "repo"}
		store := newMockWorkspaceStore(ws)
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
		)

		resolved, err := resolver.Resolve(ctx, testWorkspaceULID)
		if err != nil {
			t.Fatalf("Resolve(stable identity) error = %v", err)
		}
		if resolved.ID != ws.ID {
			t.Fatalf("Resolve(stable identity).ID = %q, want %q", resolved.ID, ws.ID)
		}
		if resolved.WorkspaceID != testWorkspaceULID {
			t.Fatalf("Resolve(stable identity).WorkspaceID = %q, want %q", resolved.WorkspaceID, testWorkspaceULID)
		}
		if got := len(store.getByNameCalls); got != 0 {
			t.Fatalf("GetWorkspaceByName() calls = %d, want 0", got)
		}
		if got := store.listCalls; got != 1 {
			t.Fatalf("ListWorkspaces() calls = %d, want 1", got)
		}
	})

	t.Run("Should reject an unknown stable identity", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := newTestHomePaths(t)
		store := newMockWorkspaceStore()
		resolver := newTestResolver(t, store,
			WithHomePaths(homePaths),
			WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
		)

		_, err := resolver.Resolve(ctx, testWorkspaceULID)
		if !errors.Is(err, ErrWorkspaceNotFound) {
			t.Fatalf("Resolve(unknown stable identity) error = %v, want %v", err, ErrWorkspaceNotFound)
		}
		if got := len(store.getByNameCalls); got != 0 {
			t.Fatalf("GetWorkspaceByName() calls = %d, want 0", got)
		}
		if got := store.listCalls; got != 1 {
			t.Fatalf("ListWorkspaces() calls = %d, want 1", got)
		}
	})
}

func TestResolveFailsClosedForInvalidWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".compozy", "workspace.toml"), `workspace_id = "invalid"
created_at = "2026-05-05T12:00:00Z"
realpath_at_creation = "/tmp/repo"
`)
	store := newMockWorkspaceStore(Workspace{ID: "ws_invalid_identity", RootDir: root, Name: "repo"})
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
	)

	_, err := resolver.Resolve(ctx, "ws_invalid_identity")
	if !errors.Is(err, ErrWorkspaceIdentityInvalid) {
		t.Fatalf("Resolve() error = %v, want %v", err, ErrWorkspaceIdentityInvalid)
	}
}

func TestResolveFailsClosedForPermissionDeniedWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("permission-denied identity test is not reliable as root")
	}

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	root := t.TempDir()
	identityPath := filepath.Join(root, ".compozy", "workspace.toml")
	writeFile(t, identityPath, `workspace_id = "`+testWorkspaceULID+`"
created_at = "2026-05-05T12:00:00Z"
realpath_at_creation = "/tmp/repo"
`)
	if err := os.Chmod(identityPath, 0); err != nil {
		t.Fatalf("os.Chmod(identityPath) error = %v", err)
	}
	t.Cleanup(func() {
		if chmodErr := os.Chmod(identityPath, workspaceIdentityFilePerm); chmodErr != nil {
			t.Errorf("os.Chmod(identityPath, workspaceIdentityFilePerm) cleanup error = %v", chmodErr)
		}
	})

	store := newMockWorkspaceStore(Workspace{ID: "ws_permission_identity", RootDir: root, Name: "repo"})
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader((&countingConfigLoader{cfg: validConfig(homePaths)}).Load),
	)

	_, err := resolver.Resolve(ctx, "ws_permission_identity")
	if !errors.Is(err, ErrWorkspaceIdentityPermissionDenied) {
		t.Fatalf("Resolve() error = %v, want %v", err, ErrWorkspaceIdentityPermissionDenied)
	}
}

func TestResolveSymlinkChangedUpdatesStoredRootDir(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homePaths := newTestHomePaths(t)
	parent := t.TempDir()
	targetOne := filepath.Join(parent, "target-one")
	targetTwo := filepath.Join(parent, "target-two")
	link := filepath.Join(parent, "workspace-link")

	if err := os.MkdirAll(targetOne, 0o755); err != nil {
		t.Fatalf("MkdirAll(targetOne) error = %v", err)
	}
	if err := os.MkdirAll(targetTwo, 0o755); err != nil {
		t.Fatalf("MkdirAll(targetTwo) error = %v", err)
	}
	createSymlink(t, targetOne, link)
	createSymlink(t, targetTwo, link)

	store := newMockWorkspaceStore(Workspace{ID: "ws_symlink", RootDir: link, Name: "repo"})
	loader := &countingConfigLoader{cfg: validConfig(homePaths)}
	resolver := newTestResolver(t, store,
		WithHomePaths(homePaths),
		WithConfigLoader(loader.Load),
	)

	resolved, err := resolver.Resolve(ctx, "ws_symlink")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	canonicalTargetTwo := mustCanonicalRoot(t, targetTwo)
	if resolved.RootDir != canonicalTargetTwo {
		t.Fatalf("Resolve() RootDir = %q, want %q", resolved.RootDir, canonicalTargetTwo)
	}
	if got := loader.LastRoot(); got != canonicalTargetTwo {
		t.Fatalf("loadConfig root = %q, want %q", got, canonicalTargetTwo)
	}
	if updated := store.mustWorkspace("ws_symlink"); updated.RootDir != canonicalTargetTwo {
		t.Fatalf("updated store root = %q, want %q", updated.RootDir, canonicalTargetTwo)
	}
}
