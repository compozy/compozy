package globaldb

import (
	"context"
	"os"
	"path/filepath"
	"sync"
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

func TestConcurrentRegisterRequestsCollapseToOneDurableWorkspaceRow(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir workflow marker: %v", err)
	}

	const workers = 8

	type result struct {
		workspace Workspace
		err       error
	}

	start := make(chan struct{})
	results := make(chan result, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			workspace, err := db.Register(context.Background(), workspaceRoot, "concurrent-workspace")
			results <- result{workspace: workspace, err: err}
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	var first Workspace
	for item := range results {
		if item.err != nil {
			t.Fatalf("Register(concurrent) error = %v, want nil", item.err)
		}
		if first.ID == "" {
			first = item.workspace
			continue
		}
		if item.workspace.ID != first.ID {
			t.Fatalf("concurrent register returned workspace id %q, want %q", item.workspace.ID, first.ID)
		}
		if item.workspace.RootDir != first.RootDir {
			t.Fatalf("concurrent register returned root %q, want %q", item.workspace.RootDir, first.RootDir)
		}
	}

	listed, err := db.List(context.Background())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List() returned %d rows, want 1", len(listed))
	}
	if listed[0].ID != first.ID {
		t.Fatalf("List()[0].ID = %q, want %q", listed[0].ID, first.ID)
	}
}

func TestReopenPreservesWorkspaceWorkflowAndRunIdentity(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "global.db")
	db, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open(first): %v", err)
	}

	workspaceRoot := filepath.Join(t.TempDir(), "persisted-workspace")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir workflow marker: %v", err)
	}

	workspace, err := db.Register(context.Background(), workspaceRoot, "persisted-workspace")
	if err != nil {
		t.Fatalf("Register(): %v", err)
	}
	workflow, err := db.PutWorkflow(context.Background(), Workflow{
		WorkspaceID: workspace.ID,
		Slug:        "persisted-flow",
	})
	if err != nil {
		t.Fatalf("PutWorkflow(): %v", err)
	}
	run, err := db.PutRun(context.Background(), Run{
		RunID:            "run-persisted",
		WorkspaceID:      workspace.ID,
		WorkflowID:       &workflow.ID,
		Mode:             "tasks",
		Status:           "completed",
		PresentationMode: "stream",
	})
	if err != nil {
		t.Fatalf("PutRun(): %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close(first): %v", err)
	}

	reopened, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open(reopen): %v", err)
	}
	defer func() {
		_ = reopened.Close()
	}()

	workspaceByID, err := reopened.Get(context.Background(), workspace.ID)
	if err != nil {
		t.Fatalf("Get(workspace): %v", err)
	}
	if workspaceByID.RootDir != workspace.RootDir {
		t.Fatalf("workspace root after reopen = %q, want %q", workspaceByID.RootDir, workspace.RootDir)
	}

	activeWorkflow, err := reopened.GetActiveWorkflowBySlug(context.Background(), workspace.ID, workflow.Slug)
	if err != nil {
		t.Fatalf("GetActiveWorkflowBySlug(): %v", err)
	}
	if activeWorkflow.ID != workflow.ID {
		t.Fatalf("active workflow id after reopen = %q, want %q", activeWorkflow.ID, workflow.ID)
	}

	runByID, err := reopened.GetRun(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("GetRun(): %v", err)
	}
	if runByID.WorkspaceID != workspace.ID {
		t.Fatalf("run workspace id after reopen = %q, want %q", runByID.WorkspaceID, workspace.ID)
	}
	if runByID.WorkflowID == nil || *runByID.WorkflowID != workflow.ID {
		t.Fatalf("run workflow id after reopen = %#v, want %q", runByID.WorkflowID, workflow.ID)
	}

	listed, err := reopened.List(context.Background())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("List() after reopen returned %d rows, want 1", len(listed))
	}
}
