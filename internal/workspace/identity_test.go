package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testWorkspaceULID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

func TestEnsureIdentityCreatesLoadsAndValidatesWorkspaceToml(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	canonical, err := canonicalRoot(root)
	if err != nil {
		t.Fatalf("canonicalRoot(%q) error = %v", root, err)
	}
	createdAt := time.Date(2026, 5, 5, 12, 0, 0, 123, time.UTC)

	created, err := ensureIdentity(
		ctx,
		root,
		func() time.Time { return createdAt },
		func() string { return testWorkspaceULID },
	)
	if err != nil {
		t.Fatalf("ensureIdentity(create) error = %v", err)
	}
	if got, want := created.WorkspaceID, testWorkspaceULID; got != want {
		t.Fatalf("created.WorkspaceID = %q, want %q", got, want)
	}
	if !created.CreatedAt.Equal(createdAt) {
		t.Fatalf("created.CreatedAt = %s, want %s", created.CreatedAt, createdAt)
	}
	if got, want := created.Path, filepath.Join(canonical, ".agh", "workspace.toml"); got != want {
		t.Fatalf("created.Path = %q, want %q", got, want)
	}

	loaded, err := ensureIdentity(
		ctx,
		root,
		func() time.Time { return createdAt.Add(time.Hour) },
		func() string { return "01BX5ZZKBKACTAV9WEVGEMMVRZ" },
	)
	if err != nil {
		t.Fatalf("ensureIdentity(load) error = %v", err)
	}
	if loaded.WorkspaceID != created.WorkspaceID {
		t.Fatalf("loaded.WorkspaceID = %q, want stable %q", loaded.WorkspaceID, created.WorkspaceID)
	}
	if !loaded.CreatedAt.Equal(createdAt) {
		t.Fatalf("loaded.CreatedAt = %s, want %s", loaded.CreatedAt, createdAt)
	}

	content, err := os.ReadFile(created.Path)
	if err != nil {
		t.Fatalf("os.ReadFile(workspace.toml) error = %v", err)
	}
	for _, want := range []string{
		`workspace_id = "` + testWorkspaceULID + `"`,
		`created_at = "` + createdAt.Format(time.RFC3339Nano) + `"`,
		`realpath_at_creation = "` + canonical + `"`,
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("workspace.toml = %q, want substring %q", string(content), want)
		}
	}
}

func TestEnsureIdentityFailsClosedForInvalidWorkspaceToml(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	identityPath := filepath.Join(root, ".agh", "workspace.toml")
	writeFile(t, identityPath, `workspace_id = "not-a-ulid"
created_at = "2026-05-05T12:00:00Z"
realpath_at_creation = "/tmp/workspace"
`)

	_, err := EnsureIdentity(context.Background(), root)
	if !errors.Is(err, ErrWorkspaceIdentityInvalid) {
		t.Fatalf("EnsureIdentity() error = %v, want %v", err, ErrWorkspaceIdentityInvalid)
	}
}

func TestEnsureIdentityFailsClosedForPermissionDeniedWorkspaceToml(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("permission-denied identity test is not reliable as root")
	}

	root := t.TempDir()
	identityPath := filepath.Join(root, ".agh", "workspace.toml")
	writeFile(t, identityPath, `workspace_id = "`+testWorkspaceULID+`"
created_at = "2026-05-05T12:00:00Z"
realpath_at_creation = "/tmp/workspace"
`)
	if err := os.Chmod(identityPath, 0); err != nil {
		t.Fatalf("os.Chmod(identityPath) error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(identityPath, workspaceIdentityFilePerm)
	})

	_, err := EnsureIdentity(context.Background(), root)
	if !errors.Is(err, ErrWorkspaceIdentityPermissionDenied) {
		t.Fatalf("EnsureIdentity() error = %v, want %v", err, ErrWorkspaceIdentityPermissionDenied)
	}
}
