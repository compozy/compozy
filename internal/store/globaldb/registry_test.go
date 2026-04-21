package globaldb

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func TestRegisterSameWorkspaceThroughEquivalentPathsReturnsOneLogicalRow(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir workflow marker: %v", err)
	}

	linkPath := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(workspaceRoot, linkPath); err != nil {
		t.Fatalf("symlink workspace root: %v", err)
	}

	first, err := db.Register(context.Background(), workspaceRoot, "demo")
	if err != nil {
		t.Fatalf("Register(real path): %v", err)
	}
	second, err := db.Register(context.Background(), linkPath, "demo")
	if err != nil {
		t.Fatalf("Register(symlink path): %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("workspace ids differ\nfirst:  %#v\nsecond: %#v", first, second)
	}
	if first.RootDir != second.RootDir {
		t.Fatalf("workspace roots differ\nfirst: %q\nsecond: %q", first.RootDir, second.RootDir)
	}

	workspaces, err := db.List(context.Background())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("List() returned %d rows, want 1", len(workspaces))
	}
}

func TestResolveOrRegisterUsesDefaultNameAndReturnsExistingRow(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspaceRoot := filepath.Join(t.TempDir(), "demo-workspace")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir workflow marker: %v", err)
	}

	first, err := db.ResolveOrRegister(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister(first): %v", err)
	}
	second, err := db.ResolveOrRegister(context.Background(), workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister(second): %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("ResolveOrRegister() ids differ\nfirst: %#v\nsecond: %#v", first, second)
	}
	if first.Name != "demo-workspace" {
		t.Fatalf("workspace default name = %q, want demo-workspace", first.Name)
	}
}

func TestGetByPathPrefersResolvedCanonicalWorkspaceRow(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspaceRoot := filepath.Join(t.TempDir(), "demo-workspace")
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir workflow marker: %v", err)
	}

	aliasRoot := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(workspaceRoot, aliasRoot); err != nil {
		t.Fatalf("symlink workspace root: %v", err)
	}

	canonical, err := db.Register(context.Background(), workspaceRoot, "demo")
	if err != nil {
		t.Fatalf("Register(canonical): %v", err)
	}

	aliasNow := db.now()
	if _, err := db.db.ExecContext(
		context.Background(),
		`INSERT INTO workspaces (id, root_dir, name, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		db.newID("ws"),
		aliasRoot,
		"demo-alias",
		store.FormatTimestamp(aliasNow),
		store.FormatTimestamp(aliasNow),
	); err != nil {
		t.Fatalf("insert alias workspace row: %v", err)
	}

	got, err := db.Get(context.Background(), aliasRoot)
	if err != nil {
		t.Fatalf("Get(alias path): %v", err)
	}
	if got.ID != canonical.ID {
		t.Fatalf("Get(alias path) = %#v, want canonical workspace %#v", got, canonical)
	}
}

func TestGetByIDOrPathAndUnregisterSuccess(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspaceRoot := t.TempDir()
	workspace, err := db.Register(context.Background(), workspaceRoot, "lookup-workspace")
	if err != nil {
		t.Fatalf("Register(): %v", err)
	}

	byID, err := db.Get(context.Background(), workspace.ID)
	if err != nil {
		t.Fatalf("Get(by id): %v", err)
	}
	byPath, err := db.Get(context.Background(), workspace.RootDir)
	if err != nil {
		t.Fatalf("Get(by path): %v", err)
	}
	if byID.ID != workspace.ID || byPath.ID != workspace.ID {
		t.Fatalf("workspace lookup mismatch\nbyID: %#v\nbyPath: %#v\nwant: %#v", byID, byPath, workspace)
	}

	if err := db.Unregister(context.Background(), workspace.RootDir); err != nil {
		t.Fatalf("Unregister(): %v", err)
	}
	if _, err := db.Get(context.Background(), workspace.ID); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("Get(after unregister) error = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestGetReturnsNotFoundForMissingWorkspace(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	if _, err := db.Get(
		context.Background(),
		filepath.Join(t.TempDir(), "missing"),
	); !errors.Is(
		err,
		ErrWorkspaceNotFound,
	) {
		t.Fatalf("Get(missing) error = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestRegistryValidationBranches(t *testing.T) {
	t.Parallel()

	var nilDB *GlobalDB
	var nilCtx context.Context
	if got := nilDB.Path(); got != "" {
		t.Fatalf("nil GlobalDB Path() = %q, want empty string", got)
	}
	if err := nilDB.Close(); err != nil {
		t.Fatalf("nil GlobalDB Close() error = %v, want nil", err)
	}
	if _, err := Open(nilCtx, filepath.Join(t.TempDir(), "invalid.db")); err == nil {
		t.Fatal("Open(nil, path) error = nil, want non-nil")
	}

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	filePath := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(filePath, []byte("file"), 0o600); err != nil {
		t.Fatalf("write file path: %v", err)
	}

	if _, err := db.Register(context.Background(), filePath, ""); err == nil {
		t.Fatal("Register(file path) error = nil, want non-nil")
	}
	if _, err := db.Get(context.Background(), " "); err == nil {
		t.Fatal("Get(empty) error = nil, want non-nil")
	}
	if _, err := db.ListWorkflows(context.Background(), ListWorkflowsOptions{}); err == nil {
		t.Fatal("ListWorkflows(missing workspace id) error = nil, want non-nil")
	}
	if _, err := db.PutWorkflow(context.Background(), Workflow{ID: "wf-update"}); err == nil {
		t.Fatal("PutWorkflow(invalid update) error = nil, want non-nil")
	}
	if _, err := db.PutRun(context.Background(), Run{}); err == nil {
		t.Fatal("PutRun(invalid) error = nil, want non-nil")
	}
	if err := db.Unregister(context.Background(), "missing-workspace"); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("Unregister(missing) error = %v, want ErrWorkspaceNotFound", err)
	}
	if got, err := normalizeWorkspaceRoot(filePath); err == nil || got != "" {
		t.Fatalf("normalizeWorkspaceRoot(file) = %q, %v; want empty string and error", got, err)
	}
}

func TestCanonicalizeExistingPathCaseWithUsesOnDiskNames(t *testing.T) {
	t.Parallel()

	root := string(filepath.Separator)
	usersDir := filepath.Join(root, "Users")
	homeDir := filepath.Join(usersDir, "pedronauck")
	devDir := filepath.Join(homeDir, "Dev")
	compozyDir := filepath.Join(devDir, "compozy")
	want := filepath.Join(compozyDir, "agh")
	input := filepath.Join(homeDir, "dev", "compozy", "agh")

	dirs := map[string][]os.DirEntry{
		root:       {fakeDirEntry{name: "Users"}},
		usersDir:   {fakeDirEntry{name: "pedronauck"}},
		homeDir:    {fakeDirEntry{name: "Dev"}},
		devDir:     {fakeDirEntry{name: "compozy"}},
		compozyDir: {fakeDirEntry{name: "agh"}},
	}

	got, err := canonicalizeExistingPathCaseWith(input, func(path string) ([]os.DirEntry, error) {
		entries, ok := dirs[path]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return entries, nil
	})
	if err != nil {
		t.Fatalf("canonicalizeExistingPathCaseWith() error = %v", err)
	}
	if got != want {
		t.Fatalf("canonicalizeExistingPathCaseWith() = %q, want %q", got, want)
	}
}

func TestMethodsRejectNilContextAndNilDatabase(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspaceRoot := t.TempDir()
	var nilCtx context.Context
	if _, err := db.Register(nilCtx, workspaceRoot, ""); err == nil {
		t.Fatal("Register(nil, ...) error = nil, want non-nil")
	}
	if _, err := db.List(nilCtx); err == nil {
		t.Fatal("List(nil) error = nil, want non-nil")
	}
	if _, err := db.GetWorkflow(nilCtx, "wf"); err == nil {
		t.Fatal("GetWorkflow(nil, ...) error = nil, want non-nil")
	}
	if _, err := db.GetRun(nilCtx, "run"); err == nil {
		t.Fatal("GetRun(nil, ...) error = nil, want non-nil")
	}

	var zeroDB GlobalDB
	if _, err := zeroDB.List(context.Background()); err == nil {
		t.Fatal("zeroDB.List(ctx) error = nil, want non-nil")
	}
}

func TestUnregisterWorkspaceWithActiveRunsReturnsConflict(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspaceRoot := t.TempDir()
	workspace, err := db.Register(context.Background(), workspaceRoot, "conflict-workspace")
	if err != nil {
		t.Fatalf("Register(): %v", err)
	}

	_, err = db.PutRun(context.Background(), Run{
		RunID:            "run-active",
		WorkspaceID:      workspace.ID,
		Mode:             "tasks",
		Status:           "running",
		PresentationMode: "ui",
		StartedAt:        time.Date(2026, 4, 17, 18, 5, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("PutRun(): %v", err)
	}

	err = db.Unregister(context.Background(), workspace.ID)
	if !errors.Is(err, ErrWorkspaceHasActiveRuns) {
		t.Fatalf("Unregister() error = %v, want ErrWorkspaceHasActiveRuns", err)
	}

	var activeRunsErr ActiveRunsError
	if !errors.As(err, &activeRunsErr) {
		t.Fatalf("Unregister() error = %v, want ActiveRunsError details", err)
	}
	if activeRunsErr.ActiveRuns != 1 {
		t.Fatalf("ActiveRunsError.ActiveRuns = %d, want 1", activeRunsErr.ActiveRuns)
	}

	if _, err := db.Get(context.Background(), workspace.ID); err != nil {
		t.Fatalf("workspace should remain registered after conflict: %v", err)
	}

	var typedErr ActiveRunsError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected ActiveRunsError details, got %v", err)
	}
	if got := typedErr.Error(); got == "" {
		t.Fatal("ActiveRunsError.Error() returned an empty message")
	}
}

func TestArchivedAndActiveWorkflowRowsKeepDistinctQueryBehavior(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace, err := db.Register(context.Background(), t.TempDir(), "workflow-catalog")
	if err != nil {
		t.Fatalf("Register(): %v", err)
	}

	first, err := db.PutWorkflow(context.Background(), Workflow{
		WorkspaceID: workspace.ID,
		Slug:        "demo",
	})
	if err != nil {
		t.Fatalf("PutWorkflow(first active): %v", err)
	}

	if _, err := db.PutWorkflow(context.Background(), Workflow{
		WorkspaceID: workspace.ID,
		Slug:        "demo",
	}); !errors.Is(err, ErrWorkflowSlugConflict) {
		t.Fatalf("PutWorkflow(duplicate active) error = %v, want ErrWorkflowSlugConflict", err)
	}

	archivedAt := time.Date(2026, 4, 17, 18, 10, 0, 0, time.UTC)
	first.ArchivedAt = &archivedAt
	first, err = db.PutWorkflow(context.Background(), first)
	if err != nil {
		t.Fatalf("PutWorkflow(archive existing): %v", err)
	}

	second, err := db.PutWorkflow(context.Background(), Workflow{
		WorkspaceID: workspace.ID,
		Slug:        "demo",
	})
	if err != nil {
		t.Fatalf("PutWorkflow(second active): %v", err)
	}

	active, err := db.GetActiveWorkflowBySlug(context.Background(), workspace.ID, "demo")
	if err != nil {
		t.Fatalf("GetActiveWorkflowBySlug(): %v", err)
	}
	if active.ID != second.ID {
		t.Fatalf("active workflow id = %q, want %q", active.ID, second.ID)
	}

	activeOnly, err := db.ListWorkflows(context.Background(), ListWorkflowsOptions{
		WorkspaceID: workspace.ID,
	})
	if err != nil {
		t.Fatalf("ListWorkflows(active only): %v", err)
	}
	if len(activeOnly) != 1 || activeOnly[0].ID != second.ID {
		t.Fatalf("ListWorkflows(active only) = %#v, want only current active workflow", activeOnly)
	}

	allRows, err := db.ListWorkflows(context.Background(), ListWorkflowsOptions{
		WorkspaceID:     workspace.ID,
		IncludeArchived: true,
	})
	if err != nil {
		t.Fatalf("ListWorkflows(include archived): %v", err)
	}
	if len(allRows) != 2 {
		t.Fatalf("ListWorkflows(include archived) returned %d rows, want 2", len(allRows))
	}
	if allRows[0].ID != first.ID || allRows[1].ID != second.ID {
		t.Fatalf("workflow list ordering/content = %#v, want archived then active rows", allRows)
	}

	if _, err := db.GetActiveWorkflowBySlug(
		context.Background(),
		workspace.ID,
		"missing",
	); !errors.Is(
		err,
		ErrWorkflowNotFound,
	) {
		t.Fatalf("GetActiveWorkflowBySlug(missing) error = %v, want ErrWorkflowNotFound", err)
	}
}

func TestGetWorkflowAndRunNotFoundAndRunDuplicates(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	workspace, err := db.Register(context.Background(), t.TempDir(), "run-workspace")
	if err != nil {
		t.Fatalf("Register(): %v", err)
	}

	archivedAt := time.Date(2026, 4, 17, 18, 20, 0, 0, time.UTC)
	workflow, err := db.PutWorkflow(context.Background(), Workflow{
		WorkspaceID: workspace.ID,
		Slug:        "linked",
		ArchivedAt:  &archivedAt,
	})
	if err != nil {
		t.Fatalf("PutWorkflow(linked): %v", err)
	}

	endedAt := time.Date(2026, 4, 17, 18, 30, 0, 0, time.UTC)
	run, err := db.PutRun(context.Background(), Run{
		RunID:            "run-finished",
		WorkspaceID:      workspace.ID,
		WorkflowID:       &workflow.ID,
		Mode:             "tasks",
		Status:           "completed",
		PresentationMode: "stream",
		StartedAt:        time.Date(2026, 4, 17, 18, 25, 0, 0, time.UTC),
		EndedAt:          &endedAt,
		ErrorText:        "none",
		RequestID:        "req-123",
	})
	if err != nil {
		t.Fatalf("PutRun(): %v", err)
	}
	if run.WorkflowID == nil || *run.WorkflowID != workflow.ID {
		t.Fatalf("run workflow link = %#v, want %q", run.WorkflowID, workflow.ID)
	}
	if run.EndedAt == nil || !run.EndedAt.Equal(endedAt) {
		t.Fatalf("run ended_at = %#v, want %v", run.EndedAt, endedAt)
	}

	if _, err := db.PutRun(context.Background(), Run{
		RunID:            "run-finished",
		WorkspaceID:      workspace.ID,
		Mode:             "tasks",
		Status:           "completed",
		PresentationMode: "stream",
		StartedAt:        time.Date(2026, 4, 17, 18, 25, 0, 0, time.UTC),
	}); !errors.Is(err, ErrRunAlreadyExists) {
		t.Fatalf("PutRun(duplicate) error = %v, want ErrRunAlreadyExists", err)
	}

	if _, err := db.GetWorkflow(context.Background(), "missing-workflow"); !errors.Is(err, ErrWorkflowNotFound) {
		t.Fatalf("GetWorkflow(missing) error = %v, want ErrWorkflowNotFound", err)
	}
	if _, err := db.GetRun(context.Background(), "missing-run"); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("GetRun(missing) error = %v, want ErrRunNotFound", err)
	}
}

func TestPutRunValidationAndWorkflowUpdateNotFoundBranches(t *testing.T) {
	t.Parallel()

	db := openTestGlobalDB(t)
	defer func() {
		_ = db.Close()
	}()

	invalidRuns := []Run{
		{WorkspaceID: "ws", Mode: "tasks", Status: "running", PresentationMode: "ui"},
		{RunID: "run-1", Mode: "tasks", Status: "running", PresentationMode: "ui"},
		{RunID: "run-1", WorkspaceID: "ws", Status: "running", PresentationMode: "ui"},
		{RunID: "run-1", WorkspaceID: "ws", Mode: "tasks", PresentationMode: "ui"},
		{RunID: "run-1", WorkspaceID: "ws", Mode: "tasks", Status: "running"},
	}
	for _, candidate := range invalidRuns {
		if _, err := db.PutRun(context.Background(), candidate); err == nil {
			t.Fatalf("PutRun(%#v) error = nil, want non-nil", candidate)
		}
	}

	if _, err := db.PutWorkflow(context.Background(), Workflow{
		ID:          "wf-missing",
		WorkspaceID: "ws-missing",
		Slug:        "demo",
	}); !errors.Is(err, ErrWorkflowNotFound) {
		t.Fatalf("PutWorkflow(update missing) error = %v, want ErrWorkflowNotFound", err)
	}
}

type fakeDirEntry struct {
	name string
}

func (e fakeDirEntry) Name() string               { return e.name }
func (e fakeDirEntry) IsDir() bool                { return true }
func (e fakeDirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (e fakeDirEntry) Info() (fs.FileInfo, error) { return fakeFileInfo(e), nil }

type fakeFileInfo struct {
	name string
}

func (i fakeFileInfo) Name() string       { return i.name }
func (i fakeFileInfo) Size() int64        { return 0 }
func (i fakeFileInfo) Mode() fs.FileMode  { return fs.ModeDir }
func (i fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (i fakeFileInfo) IsDir() bool        { return true }
func (i fakeFileInfo) Sys() any           { return nil }
