package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"
)

const testWorkspaceULID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

func TestNewWorkspaceID(t *testing.T) {
	t.Parallel()

	t.Run("Should return the entropy error without panicking or substituting an ID", func(t *testing.T) {
		t.Parallel()

		entropyErr := errors.New("entropy unavailable")
		id, err := newWorkspaceID(
			time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
			iotest.ErrReader(entropyErr),
		)
		if !errors.Is(err, entropyErr) {
			t.Fatalf("newWorkspaceID() error = %v, want %v", err, entropyErr)
		}
		if id != "" {
			t.Fatalf("newWorkspaceID() ID = %q, want empty", id)
		}
	})

	t.Run("Should reject a nil entropy source without substituting an ID", func(t *testing.T) {
		t.Parallel()

		id, err := newWorkspaceID(time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC), nil)
		if err == nil {
			t.Fatal("newWorkspaceID() error = nil, want entropy source error")
		}
		if id != "" {
			t.Fatalf("newWorkspaceID() ID = %q, want empty", id)
		}
	})
}

func TestEnsureIdentityCreatesLoadsAndValidatesWorkspaceToml(t *testing.T) {
	t.Parallel()

	t.Run("Should create and reload a stable workspace identity", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
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
			func() (string, error) { return testWorkspaceULID, nil },
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
		if got, want := created.Path, filepath.Join(canonical, ".compozy", "workspace.toml"); got != want {
			t.Fatalf("created.Path = %q, want %q", got, want)
		}

		loaded, err := ensureIdentity(
			ctx,
			root,
			func() time.Time { return createdAt.Add(time.Hour) },
			func() (string, error) { return "01BX5ZZKBKACTAV9WEVGEMMVRZ", nil },
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
	})
}

func TestEnsureIdentityPropagatesIDGenerationFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should leave no identity artifacts when entropy fails", func(t *testing.T) {
		t.Parallel()

		entropyErr := errors.New("entropy unavailable")
		root := t.TempDir()
		_, err := ensureIdentity(
			t.Context(),
			root,
			func() time.Time { return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC) },
			func() (string, error) { return "", entropyErr },
		)
		if !errors.Is(err, entropyErr) {
			t.Fatalf("ensureIdentity() error = %v, want %v", err, entropyErr)
		}
		if _, statErr := os.Stat(filepath.Join(root, ".compozy")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(.compozy) error = %v, want %v", statErr, os.ErrNotExist)
		}
	})
}

func TestEnsureIdentityFailsClosedForInvalidWorkspaceToml(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a malformed workspace identity file", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		identityPath := filepath.Join(root, ".compozy", "workspace.toml")
		writeFile(t, identityPath, `workspace_id = "not-a-ulid"
created_at = "2026-05-05T12:00:00Z"
realpath_at_creation = "/tmp/workspace"
`)

		_, err := EnsureIdentity(t.Context(), root)
		if !errors.Is(err, ErrWorkspaceIdentityInvalid) {
			t.Fatalf("EnsureIdentity() error = %v, want %v", err, ErrWorkspaceIdentityInvalid)
		}
	})
}

func TestEnsureIdentityFailsClosedForPermissionDeniedWorkspaceToml(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an unreadable workspace identity file", func(t *testing.T) {
		t.Parallel()

		if os.Geteuid() == 0 {
			t.Skip("permission-denied identity test is not reliable as root")
		}

		root := t.TempDir()
		identityPath := filepath.Join(root, ".compozy", "workspace.toml")
		writeFile(t, identityPath, `workspace_id = "`+testWorkspaceULID+`"
created_at = "2026-05-05T12:00:00Z"
realpath_at_creation = "/tmp/workspace"
`)
		if err := os.Chmod(identityPath, 0); err != nil {
			t.Fatalf("os.Chmod(identityPath) error = %v", err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(identityPath, workspaceIdentityFilePerm); err != nil {
				t.Errorf("os.Chmod(identityPath, workspaceIdentityFilePerm) error = %v", err)
			}
		})

		_, err := EnsureIdentity(t.Context(), root)
		if !errors.Is(err, ErrWorkspaceIdentityPermissionDenied) {
			t.Fatalf("EnsureIdentity() error = %v, want %v", err, ErrWorkspaceIdentityPermissionDenied)
		}
	})
}
