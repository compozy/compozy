package globaldb

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveThenExplicitRegisterYieldsOneStableWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspaceRoot := t.TempDir()
	nestedPath := filepath.Join(workspaceRoot, "pkg", "feature", "subdir")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir workflow marker: %v", err)
	}
	if err := os.MkdirAll(nestedPath, 0o755); err != nil {
		t.Fatalf("mkdir nested path: %v", err)
	}

	resolved, err := db.Resolve(context.Background(), nestedPath)
	if err != nil {
		t.Fatalf("Resolve(): %v", err)
	}
	registered, err := db.Register(context.Background(), workspaceRoot, "stable-workspace")
	if err != nil {
		t.Fatalf("Register(): %v", err)
	}

	if resolved.ID != registered.ID {
		t.Fatalf("workspace ids differ after resolve/register\nresolved:   %#v\nregistered: %#v", resolved, registered)
	}

	listed, err := db.List(context.Background())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List() returned %d rows, want 1", len(listed))
	}
}
