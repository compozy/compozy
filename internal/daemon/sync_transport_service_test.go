package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apicore "github.com/compozy/compozy/internal/api/core"
)

func TestSyncTransportServiceResolvesTargetsAndUnavailableBranches(t *testing.T) {
	env := newRunManagerTestEnv(t, runManagerTestDeps{})
	env.writeWorkflowFile(t, env.workflowSlug, "task_01.md", daemonTaskBody("pending", "Sync task"))

	service := newTransportSyncService(env.globalDB)

	byPath, err := service.Sync(context.Background(), apicore.SyncRequest{
		Path: env.workflowDir(env.workflowSlug),
	})
	if err != nil {
		t.Fatalf("Sync(path) error = %v", err)
	}
	if byPath.WorkflowSlug != env.workflowSlug || byPath.TaskItemsUpserted != 1 {
		t.Fatalf("unexpected Sync(path) result: %#v", byPath)
	}

	byWorkspace, err := service.Sync(context.Background(), apicore.SyncRequest{
		Workspace:    env.workspaceRoot,
		WorkflowSlug: env.workflowSlug,
	})
	if err != nil {
		t.Fatalf("Sync(workspace) error = %v", err)
	}
	if byWorkspace.WorkflowSlug != env.workflowSlug || byWorkspace.WorkspaceID == "" {
		t.Fatalf("unexpected Sync(workspace) result: %#v", byWorkspace)
	}

	if _, err := service.Sync(context.Background(), apicore.SyncRequest{}); err == nil ||
		!strings.Contains(err.Error(), "workspace or path is required") {
		t.Fatalf("Sync(missing target) error = %v, want validation problem", err)
	}

	nilService := newTransportSyncService(nil)
	if _, err := nilService.Sync(context.Background(), apicore.SyncRequest{Workspace: env.workspaceRoot}); err == nil ||
		!strings.Contains(err.Error(), "workflow sync is not available") {
		t.Fatalf("nil Sync() error = %v, want unavailable", err)
	}

	workspace, err := env.globalDB.ResolveOrRegister(context.Background(), env.workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister() error = %v", err)
	}
	if row, err := resolveWorkspaceReference(
		context.Background(),
		env.globalDB,
		workspace.ID,
	); err != nil ||
		row.ID != workspace.ID {
		t.Fatalf("resolveWorkspaceReference(id) = %#v, %v; want workspace id %q", row, err, workspace.ID)
	}
	if row, err := resolveWorkspaceReference(
		context.Background(),
		env.globalDB,
		env.workspaceRoot,
	); err != nil ||
		row.ID != workspace.ID {
		t.Fatalf("resolveWorkspaceReference(path) = %#v, %v; want workspace id %q", row, err, workspace.ID)
	}
	if _, err := resolveWorkspaceReference(context.Background(), nil, env.workspaceRoot); err == nil ||
		!strings.Contains(err.Error(), "workspace registry is unavailable") {
		t.Fatalf("resolveWorkspaceReference(nil db) error = %v, want unavailable", err)
	}
}

func TestLooksLikeWorkflowDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if looksLikeWorkflowDir(root) {
		t.Fatalf("looksLikeWorkflowDir(%q) = true, want false", root)
	}

	workflowDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workflowDir, "task_01.md"), []byte("# Task 01\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(task_01.md) error = %v", err)
	}
	if !looksLikeWorkflowDir(workflowDir) {
		t.Fatalf("looksLikeWorkflowDir(%q) = false, want true", workflowDir)
	}

	reviewDir := filepath.Join(t.TempDir(), "reviews-001")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(reviews dir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "task_01.md"), []byte("# Task 01\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(reviews task_01.md) error = %v", err)
	}
	if looksLikeWorkflowDir(reviewDir) {
		t.Fatalf("looksLikeWorkflowDir(%q) = true, want false for review dir", reviewDir)
	}
}
